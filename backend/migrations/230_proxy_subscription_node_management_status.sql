ALTER TABLE proxy_subscription_nodes
  ADD COLUMN IF NOT EXISTS management_status VARCHAR(32) NOT NULL DEFAULT 'not_selected',
  ADD COLUMN IF NOT EXISTS management_error TEXT,
  ADD COLUMN IF NOT EXISTS managed_proxy_id BIGINT REFERENCES proxies(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_proxy_subscription_nodes_management_status
  ON proxy_subscription_nodes(source_id, management_status)
  WHERE deleted_at IS NULL;
