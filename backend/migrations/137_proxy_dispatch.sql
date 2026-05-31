ALTER TABLE proxies
  ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT 'manual',
  ADD COLUMN IF NOT EXISTS proxy_type VARCHAR(32) NOT NULL DEFAULT 'datacenter',
  ADD COLUMN IF NOT EXISTS provider VARCHAR(100),
  ADD COLUMN IF NOT EXISTS region VARCHAR(64),
  ADD COLUMN IF NOT EXISTS exit_ip VARCHAR(64),
  ADD COLUMN IF NOT EXISTS quality_status VARCHAR(32) NOT NULL DEFAULT 'healthy',
  ADD COLUMN IF NOT EXISTS max_bound_accounts INT,
  ADD COLUMN IF NOT EXISTS max_active_accounts INT,
  ADD COLUMN IF NOT EXISTS weight INT NOT NULL DEFAULT 100,
  ADD COLUMN IF NOT EXISTS last_checked_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS failure_count INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS account_proxy_bindings (
  id BIGSERIAL PRIMARY KEY,
  identity_key VARCHAR(128) NOT NULL,
  platform VARCHAR(50) NOT NULL,
  account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
  proxy_id BIGINT NOT NULL REFERENCES proxies(id) ON DELETE CASCADE,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  source VARCHAR(32) NOT NULL DEFAULT 'auto',
  first_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_success_at TIMESTAMPTZ,
  last_failure_at TIMESTAMPTZ,
  use_count BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_proxy_bindings_identity_status
  ON account_proxy_bindings(identity_key, status);

CREATE INDEX IF NOT EXISTS idx_account_proxy_bindings_account_id
  ON account_proxy_bindings(account_id);

CREATE INDEX IF NOT EXISTS idx_account_proxy_bindings_proxy_status
  ON account_proxy_bindings(proxy_id, status);

CREATE INDEX IF NOT EXISTS idx_account_proxy_bindings_platform
  ON account_proxy_bindings(platform);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_proxy_bindings_identity_proxy
  ON account_proxy_bindings(identity_key, proxy_id);

CREATE TABLE IF NOT EXISTS proxy_subscription_sources (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(120) NOT NULL,
  url TEXT NOT NULL,
  source_type VARCHAR(32) NOT NULL DEFAULT 'clash',
  provider VARCHAR(100),
  sync_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sync_interval_minutes INT NOT NULL DEFAULT 1440,
  last_synced_at TIMESTAMPTZ,
  last_error TEXT,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_proxy_subscription_sources_status
  ON proxy_subscription_sources(status)
  WHERE deleted_at IS NULL;

INSERT INTO settings (key, value, updated_at)
VALUES ('proxy_dispatch_settings', '{"direct_fallback_mode":"off","auto_assign_enabled":true}'::text, NOW())
ON CONFLICT (key) DO NOTHING;
