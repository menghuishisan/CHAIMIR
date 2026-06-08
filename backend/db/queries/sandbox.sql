-- M2 sandbox sqlc 查询源。
-- 约定:租户表查询依赖 RLS 透明过滤;插入显式带 tenant_id(WITH CHECK)。

-- ============================================================
-- runtime / runtime_image / tool(全局配置)
-- ============================================================

-- name: CreateRuntime :one
INSERT INTO runtime (id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetRuntimeByID :one
SELECT * FROM runtime WHERE id = $1;

-- name: GetRuntimeByCode :one
SELECT * FROM runtime WHERE code = $1;

-- name: ListRuntimes :many
SELECT * FROM runtime
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateRuntime :one
UPDATE runtime
SET name = $2, eco = $3, adapter_level = $4, adapter_spec = $5,
    capability_impl = $6, plugin_ref = $7, status = $8
WHERE id = $1
RETURNING *;

-- name: UpdateRuntimeSelftest :one
UPDATE runtime SET selftest_status = $2, selftest_detail = $3, status = $4
WHERE id = $1
RETURNING *;

-- name: CreateRuntimeImage :one
INSERT INTO runtime_image (id, runtime_id, image_url, version, prepulled, prepull_status, prepull_detail, genesis_baked, is_default)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetDefaultRuntimeImage :one
SELECT * FROM runtime_image WHERE runtime_id = $1 AND is_default = true;

-- name: GetRuntimeImage :one
SELECT * FROM runtime_image WHERE id = $1 AND runtime_id = $2;

-- name: GetRuntimeImageByVersion :one
SELECT * FROM runtime_image WHERE runtime_id = $1 AND version = $2;

-- name: UpdateRuntimeImagePrepull :one
UPDATE runtime_image
SET prepulled = $3,
    prepull_status = $4,
    prepull_detail = $5,
    prepulled_at = $6
WHERE id = $1 AND runtime_id = $2
RETURNING *;

-- name: CreateTool :one
INSERT INTO tool (id, code, name, kind, image_url, port, eco_tags, resource_spec, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListTools :many
SELECT * FROM tool
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetToolByCode :one
SELECT * FROM tool WHERE code = $1 AND status = 1;

-- ============================================================
-- tenant_quota
-- ============================================================

-- name: UpsertTenantQuota :one
INSERT INTO tenant_quota (tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (tenant_id) DO UPDATE
SET max_concurrent_sandbox = EXCLUDED.max_concurrent_sandbox,
    max_cpu = EXCLUDED.max_cpu,
    max_memory_mb = EXCLUDED.max_memory_mb,
    idle_timeout_min = EXCLUDED.idle_timeout_min,
    max_lifetime_min = EXCLUDED.max_lifetime_min,
    max_keepalive_min = EXCLUDED.max_keepalive_min,
    max_snapshot_retention_min = EXCLUDED.max_snapshot_retention_min,
    updated_at = now()
RETURNING *;

-- name: GetTenantQuota :one
SELECT * FROM tenant_quota WHERE tenant_id = $1;

-- name: CountActiveSandboxes :one
SELECT count(*) FROM sandbox WHERE status IN (1,2,3,4);

-- name: ListActiveSandboxResourceSpecs :many
SELECT
    s.id AS sandbox_id,
    r.adapter_spec AS runtime_adapter_spec,
    t.id AS tool_id,
    t.code AS tool_code,
    t.name AS tool_name,
    t.kind AS tool_kind,
    t.image_url AS tool_image_url,
    t.port AS tool_port,
    t.eco_tags AS tool_eco_tags,
    t.resource_spec AS tool_resource_spec
FROM sandbox s
JOIN runtime r ON r.id = s.runtime_id
LEFT JOIN sandbox_tool st ON st.sandbox_id = s.id
LEFT JOIN tool t ON t.id = st.tool_id
WHERE s.status IN (1,2,3,4)
ORDER BY s.id;

-- ============================================================
-- sandbox / sandbox_tool / sandbox_event
-- ============================================================

-- name: CreateSandbox :one
INSERT INTO sandbox (
    id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id,
    phase, status, keep_alive, snapshot_enabled, code_storage_key, init_script_ref,
    keep_alive_until, snapshot_expire_at, last_active_at, expire_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, now(), $16)
RETURNING *;

-- name: GetSandboxByID :one
SELECT * FROM sandbox WHERE id = $1;

-- name: ListSandboxesBySourceRef :many
SELECT * FROM sandbox WHERE source_ref = $1 ORDER BY created_at DESC;

-- name: UpdateSandboxPhaseStatus :one
UPDATE sandbox SET phase = $2, status = $3 WHERE id = $1 RETURNING *;

-- name: MarkSandboxActive :exec
UPDATE sandbox SET status = 3, last_active_at = now() WHERE id = $1 AND status IN (2,3,4);

-- name: RecycleSandbox :one
UPDATE sandbox SET status = 5 WHERE id = $1 AND status NOT IN (5,6)
RETURNING *;

-- name: DestroySandbox :one
UPDATE sandbox SET status = 6 WHERE id = $1 AND status = 5
RETURNING *;

-- name: UpdateSandboxCodeHash :one
UPDATE sandbox SET code_hash = $2 WHERE id = $1 RETURNING *;

-- name: UpdateSandboxSnapshot :one
UPDATE sandbox
SET snapshot_ref = $2,
    snapshot_created_at = $3,
    snapshot_expire_at = $4
WHERE id = $1
RETURNING *;

-- name: CreateSandboxTool :one
INSERT INTO sandbox_tool (id, tenant_id, sandbox_id, tool_id, access_endpoint, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListSandboxTools :many
SELECT st.*, t.code AS tool_code, t.name AS tool_name, t.kind AS tool_kind
FROM sandbox_tool st
JOIN tool t ON t.id = st.tool_id
WHERE st.sandbox_id = $1
ORDER BY t.code;

-- name: GetSandboxToolForProxy :one
SELECT st.*, t.code AS tool_code, t.name AS tool_name, t.kind AS tool_kind
FROM sandbox_tool st
JOIN tool t ON t.id = st.tool_id
WHERE st.sandbox_id = $1 AND t.code = $2 AND st.status = 1 AND t.status = 1;

-- name: CreateSandboxEvent :exec
INSERT INTO sandbox_event (id, tenant_id, sandbox_id, event_type, detail)
VALUES ($1, $2, $3, $4, $5);

-- name: ListSandboxEvents :many
SELECT * FROM sandbox_event WHERE sandbox_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;
