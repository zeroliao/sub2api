//go:build unit

package service

import (
	"context"
	stdErrors "errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newProxySubscriptionScanJobsTestService(t *testing.T) (*adminServiceImpl, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return &adminServiceImpl{entClient: client}, mock
}

func expectExpireStaleProxySubscriptionScans(mock sqlmock.Sqlmock) {
	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE proxy_subscription_scan_jobs
SET status = 'failed', finished_at = NOW(), last_error = 'scan lease expired', updated_at = NOW()
WHERE status = 'running' AND heartbeat_at < NOW() - INTERVAL '2 minutes'`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func TestProxySubscriptionScanJobRejectsConcurrentRunAndRecoversAfterExpiry(t *testing.T) {
	svc, mock := newProxySubscriptionScanJobsTestService(t)

	expectExpireStaleProxySubscriptionScans(mock)
	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO proxy_subscription_scan_jobs (source_id, status, started_at, heartbeat_at, updated_at)
VALUES ($1, 'running', NOW(), NOW(), NOW())
ON CONFLICT DO NOTHING
RETURNING id, started_at`)).
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "started_at"}).AddRow(int64(41), time.Date(2026, 8, 30, 7, 0, 0, 0, time.UTC)))
	jobID, startedAt, err := svc.tryStartProxySubscriptionScan(context.Background(), 11)
	require.NoError(t, err)
	require.Equal(t, int64(41), jobID)
	require.Equal(t, time.Date(2026, 8, 30, 7, 0, 0, 0, time.UTC), startedAt)

	expectExpireStaleProxySubscriptionScans(mock)
	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO proxy_subscription_scan_jobs (source_id, status, started_at, heartbeat_at, updated_at)
VALUES ($1, 'running', NOW(), NOW(), NOW())
ON CONFLICT DO NOTHING
RETURNING id, started_at`)).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "started_at"}))
	_, _, err = svc.tryStartProxySubscriptionScan(context.Background(), 12)
	require.Equal(t, "PROXY_SUBSCRIPTION_SCAN_BUSY", errors.Reason(err))

	// A subsequent start can succeed once the database has reclaimed the stale lease.
	expectExpireStaleProxySubscriptionScans(mock)
	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO proxy_subscription_scan_jobs (source_id, status, started_at, heartbeat_at, updated_at)
VALUES ($1, 'running', NOW(), NOW(), NOW())
ON CONFLICT DO NOTHING
RETURNING id, started_at`)).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "started_at"}).AddRow(int64(42), time.Date(2026, 8, 30, 7, 3, 0, 0, time.UTC)))
	_, _, err = svc.tryStartProxySubscriptionScan(context.Background(), 12)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProxySubscriptionScanStatusReadsRunningLeaseFromDatabase(t *testing.T) {
	svc, mock := newProxySubscriptionScanJobsTestService(t)
	expectExpireStaleProxySubscriptionScans(mock)
	startedAt := time.Now().Add(-10 * time.Second).UTC()
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT j.source_id, s.name, j.started_at, COALESCE(s.strategy_json::text, '{}')
FROM proxy_subscription_scan_jobs j
JOIN proxy_subscription_sources s ON s.id = j.source_id
WHERE j.status = 'running' AND j.heartbeat_at >= NOW() - INTERVAL '2 minutes'
ORDER BY j.started_at DESC
LIMIT 1`)).
		WillReturnRows(sqlmock.NewRows([]string{"source_id", "name", "started_at", "strategy_json"}).
			AddRow(int64(11), "primary", startedAt, `{"scan_budget_minutes":7,"scan_budget_max_minutes":12}`))

	status, err := svc.GetProxySubscriptionScanStatus(context.Background())
	require.NoError(t, err)
	require.True(t, status.Active)
	require.Equal(t, int64(11), status.SourceID)
	require.Equal(t, "primary", status.SourceName)
	require.Equal(t, 7, status.ScanBudgetMinutes)
	require.Equal(t, 12, status.ScanBudgetMaxMinutes)
	require.GreaterOrEqual(t, status.ElapsedSeconds, 0)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFinishProxySubscriptionScanPersistsTerminalState(t *testing.T) {
	svc, mock := newProxySubscriptionScanJobsTestService(t)
	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE proxy_subscription_scan_jobs
SET status = $2, finished_at = NOW(), heartbeat_at = NOW(), last_error = NULLIF($3, ''), updated_at = NOW()
WHERE id = $1 AND status = 'running'`)).
		WithArgs(int64(41), "failed", "scan failed").
		WillReturnResult(sqlmock.NewResult(0, 1))

	svc.finishProxySubscriptionScan(41, stdErrors.New("scan failed"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProxySubscriptionScanStatusIsInactiveAfterTerminalJob(t *testing.T) {
	svc, mock := newProxySubscriptionScanJobsTestService(t)
	expectExpireStaleProxySubscriptionScans(mock)
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT j.source_id, s.name, j.started_at, COALESCE(s.strategy_json::text, '{}')
FROM proxy_subscription_scan_jobs j
JOIN proxy_subscription_sources s ON s.id = j.source_id
WHERE j.status = 'running' AND j.heartbeat_at >= NOW() - INTERVAL '2 minutes'
ORDER BY j.started_at DESC
LIMIT 1`)).
		WillReturnRows(sqlmock.NewRows([]string{"source_id", "name", "started_at", "strategy_json"}))

	status, err := svc.GetProxySubscriptionScanStatus(context.Background())
	require.NoError(t, err)
	require.False(t, status.Active)
	require.Zero(t, status.SourceID)
	require.NoError(t, mock.ExpectationsWereMet())
}
