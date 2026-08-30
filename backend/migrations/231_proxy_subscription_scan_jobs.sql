CREATE TABLE IF NOT EXISTS proxy_subscription_scan_jobs (
  id BIGSERIAL PRIMARY KEY,
  source_id BIGINT NOT NULL REFERENCES proxy_subscription_sources(id) ON DELETE CASCADE,
  status VARCHAR(16) NOT NULL DEFAULT 'running',
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  last_error TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT proxy_subscription_scan_jobs_status_check CHECK (status IN ('running', 'succeeded', 'failed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_proxy_subscription_scan_jobs_running
  ON proxy_subscription_scan_jobs ((status))
  WHERE status = 'running';

CREATE INDEX IF NOT EXISTS idx_proxy_subscription_scan_jobs_source_started
  ON proxy_subscription_scan_jobs (source_id, started_at DESC);
