ALTER TABLE proxy_subscription_sources
  ADD COLUMN IF NOT EXISTS strategy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS sidecar_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS runtime VARCHAR(32) NOT NULL DEFAULT 'sing-box',
  ADD COLUMN IF NOT EXISTS port_start INT NOT NULL DEFAULT 31000,
  ADD COLUMN IF NOT EXISTS port_end INT NOT NULL DEFAULT 31999,
  ADD COLUMN IF NOT EXISTS scan_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN IF NOT EXISTS scan_interval_minutes INT NOT NULL DEFAULT 360,
  ADD COLUMN IF NOT EXISTS health_check_interval_minutes INT NOT NULL DEFAULT 20,
  ADD COLUMN IF NOT EXISTS reputation_provider VARCHAR(32) NOT NULL DEFAULT 'none',
  ADD COLUMN IF NOT EXISTS reputation_api_key_ref VARCHAR(128),
  ADD COLUMN IF NOT EXISTS last_scan_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_scan_result JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS proxy_subscription_nodes (
  id BIGSERIAL PRIMARY KEY,
  source_id BIGINT NOT NULL REFERENCES proxy_subscription_sources(id) ON DELETE CASCADE,
  node_key VARCHAR(128) NOT NULL,
  raw_uri TEXT NOT NULL,
  name VARCHAR(255) NOT NULL DEFAULT '',
  protocol VARCHAR(32) NOT NULL,
  server VARCHAR(255) NOT NULL DEFAULT '',
  port INT NOT NULL DEFAULT 0,
  username TEXT,
  country_hint VARCHAR(64),
  exit_ip VARCHAR(64),
  exit_country VARCHAR(64),
  exit_country_code VARCHAR(8),
  latency_ms INT,
  ip_clean_score INT,
  reputation_provider VARCHAR(32),
  reputation_checked_at TIMESTAMPTZ,
  reputation_raw JSONB NOT NULL DEFAULT '{}'::jsonb,
  score INT NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'candidate',
  failure_count INT NOT NULL DEFAULT 0,
  timeout_count INT NOT NULL DEFAULT 0,
  sleep_until TIMESTAMPTZ,
  last_scanned_at TIMESTAMPTZ,
  last_error TEXT,
  selected BOOLEAN NOT NULL DEFAULT FALSE,
  sidecar_required BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_proxy_subscription_nodes_source_key
  ON proxy_subscription_nodes(source_id, node_key)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_proxy_subscription_nodes_source_status
  ON proxy_subscription_nodes(source_id, status)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_proxy_subscription_nodes_selected
  ON proxy_subscription_nodes(source_id, selected, score DESC)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS proxy_sidecar_endpoints (
  id BIGSERIAL PRIMARY KEY,
  source_id BIGINT NOT NULL REFERENCES proxy_subscription_sources(id) ON DELETE CASCADE,
  node_id BIGINT NOT NULL REFERENCES proxy_subscription_nodes(id) ON DELETE CASCADE,
  proxy_id BIGINT REFERENCES proxies(id) ON DELETE SET NULL,
  runtime VARCHAR(32) NOT NULL DEFAULT 'sing-box',
  listen_host VARCHAR(64) NOT NULL DEFAULT '127.0.0.1',
  listen_port INT NOT NULL,
  protocol VARCHAR(32) NOT NULL DEFAULT 'socks5',
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  config_hash VARCHAR(128),
  last_started_at TIMESTAMPTZ,
  last_checked_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_proxy_sidecar_endpoints_source_port
  ON proxy_sidecar_endpoints(source_id, listen_port)
  WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_proxy_sidecar_endpoints_node
  ON proxy_sidecar_endpoints(node_id)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS proxy_ip_reputation_cache (
  ip VARCHAR(64) NOT NULL,
  provider VARCHAR(32) NOT NULL,
  clean_score INT NOT NULL,
  raw JSONB NOT NULL DEFAULT '{}'::jsonb,
  checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (ip, provider)
);

CREATE INDEX IF NOT EXISTS idx_proxy_ip_reputation_cache_expires
  ON proxy_ip_reputation_cache(expires_at);
