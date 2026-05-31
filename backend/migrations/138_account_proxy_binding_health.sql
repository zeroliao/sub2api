ALTER TABLE account_proxy_bindings
  ADD COLUMN IF NOT EXISTS failure_count INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_failure_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_account_proxy_bindings_identity_proxy_status_failure
  ON account_proxy_bindings(identity_key, proxy_id, status, failure_count);
