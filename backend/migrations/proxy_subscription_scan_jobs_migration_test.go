package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProxySubscriptionScanJobsMigrationIsIdempotentAndLeaseSafe(t *testing.T) {
	content, err := FS.ReadFile("231_proxy_subscription_scan_jobs.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS proxy_subscription_scan_jobs")
	require.Contains(t, sql, "REFERENCES proxy_subscription_sources(id) ON DELETE CASCADE")
	require.Contains(t, sql, "CHECK (status IN ('running', 'succeeded', 'failed'))")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS idx_proxy_subscription_scan_jobs_running")
	require.Contains(t, sql, "WHERE status = 'running'")
	require.NotContains(t, strings.ToUpper(sql), "DROP TABLE")
}
