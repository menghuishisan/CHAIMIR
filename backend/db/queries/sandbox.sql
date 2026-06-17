-- name: GetRuntimeByCode :one
SELECT id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at
FROM runtime
WHERE code = $1;

-- name: GetRuntimeByID :one
SELECT id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at
FROM runtime
WHERE id = $1;

-- name: ListRuntimes :many
SELECT id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at
FROM runtime
ORDER BY created_at DESC, id DESC;

-- name: UpsertRuntime :one
INSERT INTO runtime (id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now(), now())
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    eco = EXCLUDED.eco,
    adapter_level = EXCLUDED.adapter_level,
    adapter_spec = EXCLUDED.adapter_spec,
    capability_impl = EXCLUDED.capability_impl,
    plugin_ref = EXCLUDED.plugin_ref,
    selftest_status = EXCLUDED.selftest_status,
    selftest_detail = EXCLUDED.selftest_detail,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at;

-- name: UpdateRuntimeSelftest :one
UPDATE runtime
SET selftest_status = $2, selftest_detail = $3, status = $4, updated_at = now()
WHERE id = $1
RETURNING id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at;

-- name: GetRuntimeImageByID :one
SELECT id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default, created_at
FROM runtime_image
WHERE id = $1 AND runtime_id = $2;

-- name: GetRuntimeImageByVersion :one
SELECT id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default, created_at
FROM runtime_image
WHERE runtime_id = $1 AND version = $2 AND status = 1;

-- name: GetDefaultRuntimeImage :one
SELECT id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default, created_at
FROM runtime_image
WHERE runtime_id = $1 AND is_default = true AND status = 1;

-- name: ListRuntimeImages :many
SELECT id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default, created_at
FROM runtime_image
WHERE runtime_id = $1
ORDER BY created_at DESC, id DESC;

-- name: CreateRuntimeImage :one
INSERT INTO runtime_image (id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default, created_at)
VALUES ($1, $2, $3, $4, 1, false, 1, '{}'::jsonb, NULL, $5, $6, now())
RETURNING id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default, created_at;

-- name: MarkOtherRuntimeImagesNotDefault :exec
UPDATE runtime_image
SET is_default = false
WHERE runtime_id = $1 AND id <> $2;

-- name: UpdateRuntimeImagePrepull :one
UPDATE runtime_image
SET prepulled = $3, prepull_status = $4, prepull_detail = $5, prepulled_at = $6
WHERE id = $1 AND runtime_id = $2 AND status = 1
RETURNING id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default, created_at;

-- name: DisableRuntimeImage :one
UPDATE runtime_image
SET status = 2,
    prepulled = false,
    prepull_status = 1,
    prepull_detail = $3,
    prepulled_at = NULL,
    is_default = false
WHERE id = $1 AND runtime_id = $2
RETURNING id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default, created_at;

-- name: GetToolByCode :one
SELECT id, code, name, kind, image_url, port, eco_tags, resource_spec, status, created_at, updated_at
FROM tool
WHERE code = $1;

-- name: ListTools :many
SELECT id, code, name, kind, image_url, port, eco_tags, resource_spec, status, created_at, updated_at
FROM tool
ORDER BY created_at DESC, id DESC;

-- name: UpsertTool :one
INSERT INTO tool (id, code, name, kind, image_url, port, eco_tags, resource_spec, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    kind = EXCLUDED.kind,
    image_url = EXCLUDED.image_url,
    port = EXCLUDED.port,
    eco_tags = EXCLUDED.eco_tags,
    resource_spec = EXCLUDED.resource_spec,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING id, code, name, kind, image_url, port, eco_tags, resource_spec, status, created_at, updated_at;

-- name: GetTenantQuota :one
SELECT tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at
FROM tenant_quota
WHERE tenant_id = $1;

-- name: GetTenantQuotaForUpdate :one
SELECT tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at
FROM tenant_quota
WHERE tenant_id = $1
FOR UPDATE;

-- name: UpsertTenantQuota :one
INSERT INTO tenant_quota (tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (tenant_id) DO UPDATE
SET max_concurrent_sandbox = EXCLUDED.max_concurrent_sandbox,
    max_cpu = EXCLUDED.max_cpu,
    max_memory_mb = EXCLUDED.max_memory_mb,
    idle_timeout_min = EXCLUDED.idle_timeout_min,
    max_lifetime_min = EXCLUDED.max_lifetime_min,
    max_keepalive_min = EXCLUDED.max_keepalive_min,
    max_snapshot_retention_min = EXCLUDED.max_snapshot_retention_min,
    updated_at = now()
RETURNING tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at;

-- name: CountActiveSandboxes :one
SELECT COUNT(*)::bigint
FROM sandbox
WHERE tenant_id = $1 AND status IN (1, 2, 3, 4, 7, 8);

-- name: CreateSandbox :one
INSERT INTO sandbox (
    id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status,
    keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains,
    snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    $10, $11, $12, $13, $14, $15, $16, $17,
    $18, $19, $20, now(), $21, now(), now()
)
RETURNING id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at;

-- name: GetSandbox :one
SELECT id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
FROM sandbox
WHERE tenant_id = $1 AND id = $2;

-- name: ListSandboxesBySourceRef :many
SELECT id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
FROM sandbox
WHERE tenant_id = $1 AND source_ref = $2 AND status <> 5
ORDER BY created_at DESC, id DESC;

-- name: ListRecycleCandidates :many
SELECT s.id, s.tenant_id, s.runtime_id, s.image_id, s.namespace, s.source_ref, s.owner_account_id, s.phase, s.status, s.keep_alive, s.snapshot_enabled, s.code_storage_key, s.code_hash, s.init_code_ref, s.init_script_ref, s.snapshot_ref, s.snapshot_domains, s.snapshot_created_at, s.snapshot_expire_at, s.keep_alive_until, s.last_active_at, s.expire_at, s.created_at, s.updated_at
FROM sandbox s
JOIN tenant_quota tq ON tq.tenant_id = s.tenant_id
WHERE s.status IN (4, 6)
   OR (s.status = 1 AND s.last_active_at <= $1)
   OR (s.status = 7 AND s.last_active_at <= $1)
   OR (s.status = 8 AND s.keep_alive = false AND s.last_active_at <= now() - make_interval(mins => tq.idle_timeout_min))
   OR (s.status IN (1, 2, 3, 7, 8) AND s.expire_at <= now())
   OR (s.status IN (1, 2, 3, 7, 8) AND s.keep_alive_until IS NOT NULL AND s.keep_alive_until <= now())
ORDER BY s.updated_at ASC, s.id ASC
LIMIT $2;

-- name: MarkIdleSandboxes :many
UPDATE sandbox s
SET status = 8, updated_at = now()
FROM tenant_quota tq
WHERE tq.tenant_id = s.tenant_id
  AND s.status = 2
  AND s.keep_alive = false
  AND s.last_active_at <= now() - make_interval(mins => tq.idle_timeout_min)
RETURNING s.id, s.tenant_id, s.runtime_id, s.image_id, s.namespace, s.source_ref, s.owner_account_id, s.phase, s.status, s.keep_alive, s.snapshot_enabled, s.code_storage_key, s.code_hash, s.init_code_ref, s.init_script_ref, s.snapshot_ref, s.snapshot_domains, s.snapshot_created_at, s.snapshot_expire_at, s.keep_alive_until, s.last_active_at, s.expire_at, s.created_at, s.updated_at;

-- name: ListSnapshotCleanupCandidates :many
SELECT id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
FROM sandbox
WHERE status = 5 AND snapshot_expire_at IS NOT NULL AND snapshot_expire_at <= now()
ORDER BY snapshot_expire_at ASC, id ASC
LIMIT $1;

-- name: UpdateSandboxPhaseStatus :one
UPDATE sandbox
SET phase = $3, status = $4, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at;

-- name: MarkSandboxActive :one
UPDATE sandbox
SET last_active_at = now(),
    status = CASE WHEN status IN (7, 8) THEN 2 ELSE status END,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
  AND status IN (2, 7, 8)
RETURNING id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at;

-- name: UpdateSandboxCode :one
UPDATE sandbox
SET code_storage_key = $3, code_hash = $4, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at;

-- name: UpdateSandboxSnapshot :one
UPDATE sandbox
SET snapshot_ref = $3, snapshot_domains = $4, snapshot_created_at = $5, snapshot_expire_at = $6, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at;

-- name: CreateSandboxTool :one
INSERT INTO sandbox_tool (id, tenant_id, sandbox_id, tool_id, access_endpoint, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, tenant_id, sandbox_id, tool_id, access_endpoint, status;

-- name: ListSandboxTools :many
SELECT st.id, st.tenant_id, st.sandbox_id, st.tool_id, st.access_endpoint, st.status,
       t.code, t.kind
FROM sandbox_tool st
JOIN tool t ON t.id = st.tool_id
WHERE st.tenant_id = $1 AND st.sandbox_id = $2
ORDER BY st.id;

-- name: UpdateSandboxToolStatus :one
UPDATE sandbox_tool
SET status = $4, access_endpoint = $5
WHERE tenant_id = $1 AND sandbox_id = $2 AND tool_id = $3
RETURNING id, tenant_id, sandbox_id, tool_id, access_endpoint, status;

-- name: CreateSandboxEvent :one
INSERT INTO sandbox_event (id, tenant_id, sandbox_id, event_type, detail, created_at)
VALUES ($1, $2, $3, $4, $5, now())
RETURNING id, tenant_id, sandbox_id, event_type, detail, created_at;

-- name: CreateSandboxRecycleOutbox :one
INSERT INTO sandbox_recycle_outbox (id, tenant_id, sandbox_id, source_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, 0, NULL, now(), now())
RETURNING id, tenant_id, sandbox_id, source_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at;

-- name: ClaimPendingSandboxRecycleOutbox :many
UPDATE sandbox_recycle_outbox
SET status = 2, retry_count = retry_count + 1, updated_at = now()
WHERE id IN (
    SELECT id
    FROM sandbox_recycle_outbox
    WHERE status IN (1, 4) OR (status = 2 AND updated_at <= @stale_before::timestamptz)
    ORDER BY created_at ASC, id ASC
    LIMIT @page_limit
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, sandbox_id, source_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at;

-- name: MarkSandboxRecycleOutboxPublished :one
UPDATE sandbox_recycle_outbox
SET status = 3, last_error = NULL, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, sandbox_id, source_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at;

-- name: MarkSandboxRecycleOutboxFailed :one
UPDATE sandbox_recycle_outbox
SET status = 4, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, sandbox_id, source_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at;

-- name: StatsByTenant :one
SELECT
  COUNT(*) FILTER (WHERE s.status IN (1, 2, 3, 4, 7, 8))::bigint AS active_sandbox_count,
  tq.max_concurrent_sandbox,
  tq.max_cpu,
  tq.max_memory_mb,
  tq.idle_timeout_min,
  tq.max_lifetime_min,
  tq.max_keepalive_min,
  tq.max_snapshot_retention_min
FROM tenant_quota tq
LEFT JOIN sandbox s ON s.tenant_id = tq.tenant_id
WHERE tq.tenant_id = $1
GROUP BY tq.max_concurrent_sandbox, tq.max_cpu, tq.max_memory_mb, tq.idle_timeout_min, tq.max_lifetime_min, tq.max_keepalive_min, tq.max_snapshot_retention_min;
