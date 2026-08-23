-- Sidecar listen ports are shared by the sing-box runtime, so uniqueness is
-- global for live endpoints rather than scoped to a subscription source.
-- Existing inactive-source endpoints are retired first, so stale sidecars do
-- not keep ports reserved or remain active in the proxy pool.
UPDATE account_proxy_bindings b
SET status = 'proxy_unavailable',
    last_failure_at = NOW(),
    last_failure_reason = 'subscription sidecar source is inactive',
    updated_at = NOW()
FROM proxy_sidecar_endpoints e
JOIN proxy_subscription_nodes n ON n.id = e.node_id
JOIN proxy_subscription_sources s ON s.id = n.source_id
WHERE b.proxy_id = e.proxy_id
  AND b.status = 'active'
  AND e.deleted_at IS NULL
  AND (s.deleted_at IS NOT NULL OR s.status <> 'active' OR n.deleted_at IS NOT NULL);

UPDATE proxies p
SET status = 'disabled', quality_status = 'failed', updated_at = NOW()
FROM proxy_sidecar_endpoints e
JOIN proxy_subscription_nodes n ON n.id = e.node_id
JOIN proxy_subscription_sources s ON s.id = n.source_id
WHERE p.id = e.proxy_id
  AND p.deleted_at IS NULL
  AND e.deleted_at IS NULL
  AND (s.deleted_at IS NOT NULL OR s.status <> 'active' OR n.deleted_at IS NOT NULL);

UPDATE proxy_sidecar_endpoints e
SET status = 'inactive',
    last_error = 'retired inactive subscription source during migration',
    deleted_at = NOW(),
    updated_at = NOW()
FROM proxy_subscription_nodes n
JOIN proxy_subscription_sources s ON s.id = n.source_id
WHERE e.node_id = n.id
  AND e.deleted_at IS NULL
  AND (s.deleted_at IS NOT NULL OR s.status <> 'active' OR n.deleted_at IS NOT NULL);

-- Remaining duplicates are retired and recreated by the next active scan; this
-- avoids preserving an ambiguous node-to-port mapping during migration.
WITH duplicate_endpoints AS (
  SELECT e.id, e.proxy_id
  FROM proxy_sidecar_endpoints e
  WHERE e.deleted_at IS NULL
    AND EXISTS (
      SELECT 1
      FROM proxy_sidecar_endpoints prior
      WHERE prior.deleted_at IS NULL
        AND prior.listen_host = e.listen_host
        AND prior.listen_port = e.listen_port
        AND prior.id < e.id
    )
)
UPDATE proxies p
SET status = 'disabled', quality_status = 'failed', updated_at = NOW()
FROM duplicate_endpoints d
WHERE p.id = d.proxy_id AND p.deleted_at IS NULL;

UPDATE proxy_sidecar_endpoints e
SET status = 'inactive',
    last_error = 'retired duplicate global sidecar port during migration',
    deleted_at = NOW(),
    updated_at = NOW()
WHERE e.deleted_at IS NULL
  AND EXISTS (
    SELECT 1
    FROM proxy_sidecar_endpoints prior
    WHERE prior.deleted_at IS NULL
      AND prior.listen_host = e.listen_host
      AND prior.listen_port = e.listen_port
      AND prior.id < e.id
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_proxy_sidecar_endpoints_global_port
  ON proxy_sidecar_endpoints(listen_host, listen_port)
  WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION prevent_proxy_sidecar_port_conflict()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.deleted_at IS NULL AND EXISTS (
    SELECT 1
    FROM proxy_sidecar_endpoints e
    WHERE e.deleted_at IS NULL
      AND e.listen_host = NEW.listen_host
      AND e.listen_port = NEW.listen_port
      AND e.id <> COALESCE(NEW.id, 0)
  ) THEN
    RAISE EXCEPTION 'sidecar listen port %:% is already allocated', NEW.listen_host, NEW.listen_port
      USING ERRCODE = 'unique_violation';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS proxy_sidecar_endpoint_port_guard ON proxy_sidecar_endpoints;
CREATE TRIGGER proxy_sidecar_endpoint_port_guard
BEFORE INSERT OR UPDATE OF listen_host, listen_port, deleted_at
ON proxy_sidecar_endpoints
FOR EACH ROW
EXECUTE FUNCTION prevent_proxy_sidecar_port_conflict();
