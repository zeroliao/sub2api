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

func TestRecordFailedProxySubscriptionScanRetiresPreviousResourcesAndPersistsFailure(t *testing.T) {
	svc, mock := newProxySubscriptionScanJobsTestService(t)

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, source_id, node_key, raw_uri, name, protocol, server, port, COALESCE(username, ''),
       COALESCE(country_hint, ''), COALESCE(exit_ip, ''), COALESCE(exit_country, ''),
       COALESCE(exit_country_code, ''), latency_ms, ip_clean_score, COALESCE(reputation_provider, ''),
       reputation_checked_at, score, status, failure_count, timeout_count, sleep_until,
	       last_scanned_at, COALESCE(last_error, ''), COALESCE(management_status, 'not_selected'),
	       COALESCE(management_error, ''), managed_proxy_id, selected, sidecar_required, created_at, updated_at
FROM proxy_subscription_nodes
WHERE source_id = $1 AND deleted_at IS NULL
ORDER BY selected DESC, score DESC, id ASC`)).
		WithArgs(int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(regexp.QuoteMeta(`
DELETE FROM proxy_sidecar_endpoints
WHERE source_id = $1`)).
		WithArgs(int64(21)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE proxy_subscription_nodes
SET status = 'inactive',
    selected = FALSE,
    last_error = NULLIF($2, ''),
    deleted_at = NOW(),
    updated_at = NOW()
WHERE source_id = $1 AND deleted_at IS NULL`)).
		WithArgs(int64(21), "scan failed").
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE proxy_subscription_sources
SET last_scan_at = NOW(), last_scan_result = $2::jsonb, last_error = $3, updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`)).
		WithArgs(int64(21), sqlmock.AnyArg(), "scan failed").
		WillReturnResult(sqlmock.NewResult(0, 1))

	scanErr := stdErrors.New("scan failed")
	result, err := svc.recordFailedProxySubscriptionScan(context.Background(), 21, defaultProxySubscriptionStrategy(), 3, 3, scanErr)

	require.ErrorIs(t, err, scanErr)
	require.Equal(t, 3, result.Total)
	require.Equal(t, 3, result.Parsed)
	require.Equal(t, []string{"scan failed"}, result.Errors)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRetireSidecarProxyForSubscriptionNodePhysicallyDeletesEndpointAndProxy(t *testing.T) {
	svc, mock := newProxySubscriptionScanJobsTestService(t)

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT proxy_id
FROM proxy_sidecar_endpoints
WHERE node_id = $1 AND deleted_at IS NULL
LIMIT 1`)).
		WithArgs(int64(31)).
		WillReturnRows(sqlmock.NewRows([]string{"proxy_id"}).AddRow(int64(47)))
	mock.ExpectExec(regexp.QuoteMeta(`
DELETE FROM proxy_sidecar_endpoints
WHERE node_id = $1`)).
		WithArgs(int64(31)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
DELETE FROM proxies
WHERE id = $1
  AND COALESCE(source, 'manual') = 'subscription'`)).
		WithArgs(int64(47)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, svc.retireSidecarProxyForSubscriptionNode(context.Background(), 31, "subscription node missing from latest scan"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindManagedSubscriptionProxyIDOnlyReturnsSubscriptionProxy(t *testing.T) {
	svc, mock := newProxySubscriptionScanJobsTestService(t)

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT p.id
FROM proxy_subscription_nodes n
JOIN proxies p ON p.id = n.managed_proxy_id
WHERE n.id = $1
  AND n.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND COALESCE(p.source, 'manual') = 'subscription'
LIMIT 1`)).
		WithArgs(int64(31)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	proxyID, err := svc.findManagedSubscriptionProxyID(context.Background(), 31)
	require.NoError(t, err)
	require.Nil(t, proxyID)
	require.NoError(t, mock.ExpectationsWereMet())
}
