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

-- name: GetRuntimeByIDForUpdate :one
-- 运行时更新必须在同一事务内比较旧执行契约并写入新状态,防止并发更新覆盖自检结论。
SELECT id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at
FROM runtime
WHERE id = $1
FOR UPDATE;

-- name: StartRuntimeSelftest :one
-- 自检启动时写入唯一批次并退回接入中,并发契约更新或停用可使旧批次自然失效。
UPDATE runtime
SET selftest_status = 1, selftest_detail = $2, status = 2, updated_at = now()
WHERE id = $1 AND status <> 3
RETURNING id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at;

-- name: FinishRuntimeSelftest :one
-- 只允许仍处于接入中的同一自检批次写回,防止旧结果覆盖并发配置更新或停用。
UPDATE runtime
SET selftest_status = $2, selftest_detail = $3, status = $4, updated_at = now()
WHERE id = $1
  AND selftest_status = 1
  AND status = 2
  AND selftest_detail->>'attempt_id' = sqlc.arg(attempt_id)::text
RETURNING id, code, name, eco, adapter_level, adapter_spec, capability_impl, plugin_ref, selftest_status, selftest_detail, status, created_at, updated_at;

-- name: GetRuntimeImageByID :one
SELECT id, runtime_id, image_url, version, status, genesis_baked, created_at
FROM runtime_image
WHERE id = $1 AND runtime_id = $2;

-- name: GetRuntimeImageByVersion :one
SELECT id, runtime_id, image_url, version, status, genesis_baked, created_at
FROM runtime_image
WHERE runtime_id = $1 AND version = $2 AND status = 1;

-- name: GetRuntimeImageByVersionForShare :one
SELECT id, runtime_id, image_url, version, status, genesis_baked, created_at
FROM runtime_image
WHERE runtime_id = $1 AND version = $2 AND status = 1
FOR SHARE;

-- name: ListRuntimeImages :many
SELECT id, runtime_id, image_url, version, status, genesis_baked, created_at
FROM runtime_image
WHERE runtime_id = $1
ORDER BY created_at DESC, id DESC;

-- name: CreateRuntimeImage :one
INSERT INTO runtime_image (id, runtime_id, image_url, version, status, genesis_baked, created_at)
VALUES ($1, $2, $3, $4, 1, $5, now())
RETURNING id, runtime_id, image_url, version, status, genesis_baked, created_at;

-- name: GetRuntimeImageByIDForUpdate :one
-- 开始预拉取前先锁定证明行;运行时或工具变更事务必须等本次闭包快照落为 running 后再执行失效。
SELECT id, runtime_id, image_url, version, status, genesis_baked, created_at
FROM runtime_image
WHERE id = $1 AND runtime_id = $2
FOR UPDATE;

-- name: GetPublishedCompositionSnapshot :one
SELECT composition_digest, runtime_id, runtime_image_id, snapshot, status, created_at, updated_at
FROM sandbox_composition
WHERE composition_digest = $1 AND status = 1;

-- name: UpsertPublishedCompositionSnapshot :exec
INSERT INTO sandbox_composition (composition_digest, runtime_id, runtime_image_id, snapshot, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, 1, now(), now())
ON CONFLICT (composition_digest) DO UPDATE
SET runtime_id = EXCLUDED.runtime_id,
    runtime_image_id = EXCLUDED.runtime_image_id,
    snapshot = EXCLUDED.snapshot,
    status = 1,
    updated_at = now();

-- name: DisableRuntimeImage :one
UPDATE runtime_image
SET status = 2
WHERE id = $1 AND runtime_id = $2
RETURNING id, runtime_id, image_url, version, status, genesis_baked, created_at;

-- name: GetCompositionPrepullForUpdate :one
SELECT id, runtime_image_id, composition_digest, status, attempt_id, daemonset_name, image_closure,
       desired_nodes, ready_nodes, detail, started_at, completed_at, created_at, updated_at
FROM sandbox_composition_prepull
WHERE runtime_image_id = $1 AND composition_digest = $2
FOR UPDATE;

-- name: StartCompositionPrepull :one
INSERT INTO sandbox_composition_prepull (
    id, runtime_image_id, composition_digest, status, attempt_id, daemonset_name, image_closure,
    desired_nodes, ready_nodes, detail, started_at, completed_at, created_at, updated_at
)
VALUES ($1, $2, $3, 4, $4, '', $5, 0, 0, $6, now(), NULL, now(), now())
ON CONFLICT (runtime_image_id, composition_digest) DO UPDATE
SET status = 4, attempt_id = EXCLUDED.attempt_id, daemonset_name = '', image_closure = EXCLUDED.image_closure,
    desired_nodes = 0, ready_nodes = 0, detail = EXCLUDED.detail, started_at = now(), completed_at = NULL, updated_at = now()
RETURNING id, runtime_image_id, composition_digest, status, attempt_id, daemonset_name, image_closure,
          desired_nodes, ready_nodes, detail, started_at, completed_at, created_at, updated_at;

-- name: FinishCompositionPrepull :one
UPDATE sandbox_composition_prepull
SET status = $3, daemonset_name = $4, desired_nodes = $5, ready_nodes = $6, detail = $7,
    completed_at = CASE WHEN $3 = 2 OR $3 = 3 THEN now() ELSE NULL END, updated_at = now()
WHERE runtime_image_id = $1 AND composition_digest = $2 AND status = 4 AND attempt_id = sqlc.arg(attempt_id)::text
RETURNING id, runtime_image_id, composition_digest, status, attempt_id, daemonset_name, image_closure,
          desired_nodes, ready_nodes, detail, started_at, completed_at, created_at, updated_at;

-- name: InvalidateCompositionPrepullByRuntime :exec
UPDATE sandbox_composition_prepull p
SET status = 1, detail = $2, completed_at = NULL, updated_at = now()
FROM runtime_image i
WHERE p.runtime_image_id = i.id AND i.runtime_id = $1 AND p.status <> 1;

-- name: DeleteCompositionPrepullByRuntimeImage :exec
DELETE FROM sandbox_composition_prepull WHERE runtime_image_id = $1;

-- name: GetToolByCode :one
SELECT id, code, name, category, kind, eco_tags, resource_spec, status, created_at, updated_at
FROM tool
WHERE code = $1;

-- name: ListTools :many
SELECT id, code, name, category, kind, eco_tags, resource_spec, status, created_at, updated_at
FROM tool
ORDER BY created_at DESC, id DESC;

-- name: ListCatalogRuntimes :many
-- 编排目录只取真正可调度的运行时与镜像版本,不把未完成自检或最新闭包预拉取的项暴露给教师。
-- 一次 JOIN 平铺取回后由 repo 按运行时分组,既避免按运行时逐个查镜像的 N+1,
-- 也不用 jsonb_agg —— 那会让生成的行类型退化成 interface{},把解码负担推给业务层。
SELECT r.code AS runtime_code, r.name AS runtime_name, r.eco, r.adapter_spec,
       i.version AS image_version
FROM runtime r
JOIN runtime_image i
  ON i.runtime_id = r.id
 AND i.status = 1
 AND i.genesis_baked = true
WHERE r.status = 1 AND r.selftest_status = 2
ORDER BY r.code, i.version DESC;

-- name: ListCatalogTools :many
-- eco_tags 只在服务端计算各运行时兼容组件,不会直接下发给编排端。
SELECT category, code, name, kind, eco_tags, resource_spec
FROM tool
WHERE status = 1
ORDER BY category, code;

-- name: UpsertTool :one
INSERT INTO tool (id, code, name, category, kind, eco_tags, resource_spec, status, created_at, updated_at)
VALUES ($1, $2, $3, 'tool', $4, $5, $6, $7, now(), now())
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    kind = EXCLUDED.kind,
    eco_tags = EXCLUDED.eco_tags,
    resource_spec = EXCLUDED.resource_spec,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING id, code, name, category, kind, eco_tags, resource_spec, status, created_at, updated_at;

-- name: GetTenantQuota :one
SELECT tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at
FROM tenant_quota
WHERE tenant_id = $1;

-- name: EnsureTenantQuota :one
WITH inserted AS (
    INSERT INTO tenant_quota (tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
    ON CONFLICT (tenant_id) DO NOTHING
    RETURNING tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at
)
SELECT tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at
FROM inserted
UNION ALL
SELECT tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb, idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min, updated_at
FROM tenant_quota
WHERE tenant_id = $1
LIMIT 1;

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
    id, tenant_id, runtime_id, image_id, namespace, source_ref, scope_ref, composition_digest, composition_snapshot, access_profile, owner_account_id, shared_account_ids, phase, status,
    keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains,
    snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
    $22, $23, $24, $25, now(), $26, now(), now()
)
RETURNING id, tenant_id, runtime_id, image_id, namespace, source_ref, scope_ref, composition_digest, composition_snapshot, access_profile, owner_account_id, shared_account_ids, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, workspace_revision, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at;

-- name: GetSandbox :one
SELECT id, tenant_id, runtime_id, image_id, namespace, source_ref, scope_ref, composition_digest, composition_snapshot, access_profile, owner_account_id, shared_account_ids, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, workspace_revision, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
FROM sandbox
WHERE tenant_id = $1 AND id = $2;

-- name: ClaimSandboxWorkspaceRevision :one
-- 用户写入必须携带最近一次读取到的 revision;不一致时不更新,由 service 返回冲突。
UPDATE sandbox
SET workspace_revision = workspace_revision + 1,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
  AND workspace_revision = sqlc.arg(expected_revision)::bigint
RETURNING workspace_revision;

-- name: LockSandboxChain :exec
-- 沙箱 ID 是全局雪花 ID;事务级 advisory lock 贯穿真实链能力调用,跨副本串行化 deploy/tx/reset。
SELECT pg_advisory_xact_lock($1::bigint);

-- name: ListSandboxesBySourceRef :many
SELECT id, tenant_id, runtime_id, image_id, namespace, source_ref, scope_ref, composition_digest, composition_snapshot, access_profile, owner_account_id, shared_account_ids, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, workspace_revision, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
FROM sandbox
WHERE tenant_id = $1 AND source_ref = $2 AND status <> 5
ORDER BY created_at DESC, id DESC;

-- name: ListSandboxesByScopeRef :many
SELECT id, tenant_id, runtime_id, image_id, namespace, source_ref, scope_ref, composition_digest, composition_snapshot, access_profile, owner_account_id, shared_account_ids, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, workspace_revision, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
FROM sandbox
WHERE tenant_id = $1 AND scope_ref = $2 AND status <> 5
ORDER BY created_at DESC, id DESC;

-- name: ListRecycleCandidates :many
SELECT s.id, s.tenant_id, s.runtime_id, s.image_id, s.namespace, s.source_ref, s.scope_ref, s.composition_digest, s.composition_snapshot, s.access_profile, s.owner_account_id, s.shared_account_ids, s.phase, s.status, s.keep_alive, s.snapshot_enabled, s.code_storage_key, s.code_hash, s.workspace_revision, s.init_code_ref, s.init_script_ref, s.snapshot_ref, s.snapshot_domains, s.snapshot_created_at, s.snapshot_expire_at, s.keep_alive_until, s.last_active_at, s.expire_at, s.created_at, s.updated_at
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
RETURNING s.id, s.tenant_id, s.runtime_id, s.image_id, s.namespace, s.source_ref, s.scope_ref, s.composition_digest, s.composition_snapshot, s.access_profile, s.owner_account_id, s.shared_account_ids, s.phase, s.status, s.keep_alive, s.snapshot_enabled, s.code_storage_key, s.code_hash, s.workspace_revision, s.init_code_ref, s.init_script_ref, s.snapshot_ref, s.snapshot_domains, s.snapshot_created_at, s.snapshot_expire_at, s.keep_alive_until, s.last_active_at, s.expire_at, s.created_at, s.updated_at;

-- name: ListSnapshotCleanupCandidates :many
SELECT id, tenant_id, runtime_id, image_id, namespace, source_ref, scope_ref, composition_digest, composition_snapshot, access_profile, owner_account_id, shared_account_ids, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, workspace_revision, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at
FROM sandbox
WHERE status = 5 AND snapshot_expire_at IS NOT NULL AND snapshot_expire_at <= now()
ORDER BY snapshot_expire_at ASC, id ASC
LIMIT $1;

-- name: UpdateSandboxPhaseStatus :one
UPDATE sandbox
SET phase = $3, status = $4, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, runtime_id, image_id, namespace, source_ref, scope_ref, composition_digest, composition_snapshot, access_profile, owner_account_id, shared_account_ids, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, workspace_revision, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at;

-- name: MarkSandboxActive :one
UPDATE sandbox
SET last_active_at = now(),
    status = CASE WHEN status IN (7, 8) THEN 2 ELSE status END,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
  AND status IN (2, 7, 8)
RETURNING id, tenant_id, runtime_id, image_id, namespace, source_ref, scope_ref, composition_digest, composition_snapshot, access_profile, owner_account_id, shared_account_ids, phase, status, keep_alive, snapshot_enabled, code_storage_key, code_hash, workspace_revision, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains, snapshot_created_at, snapshot_expire_at, keep_alive_until, last_active_at, expire_at, created_at, updated_at;

-- name: UpdateSandboxCode :one
WITH updated AS (
    UPDATE sandbox SET code_storage_key = $3, code_hash = $4, updated_at = now()
    WHERE sandbox.tenant_id = $1 AND sandbox.id = $2 RETURNING sandbox.id
)
SELECT s.id, s.tenant_id, s.runtime_id, s.image_id, s.namespace, s.source_ref, s.scope_ref, s.composition_digest, s.composition_snapshot, s.access_profile, s.owner_account_id, s.shared_account_ids, s.phase, s.status, s.keep_alive, s.snapshot_enabled, s.code_storage_key, s.code_hash, s.workspace_revision, s.init_code_ref, s.init_script_ref, s.snapshot_ref, s.snapshot_domains, s.snapshot_created_at, s.snapshot_expire_at, s.keep_alive_until, s.last_active_at, s.expire_at, s.created_at, s.updated_at
FROM sandbox s JOIN updated u ON u.id = s.id;

-- name: UpdateSandboxAuthorizedAccounts :one
WITH updated AS (
    UPDATE sandbox SET shared_account_ids = $3, updated_at = now()
    WHERE sandbox.tenant_id = $1 AND sandbox.id = $2 RETURNING sandbox.id
)
SELECT s.id, s.tenant_id, s.runtime_id, s.image_id, s.namespace, s.source_ref, s.scope_ref, s.composition_digest, s.composition_snapshot, s.access_profile, s.owner_account_id, s.shared_account_ids, s.phase, s.status, s.keep_alive, s.snapshot_enabled, s.code_storage_key, s.code_hash, s.workspace_revision, s.init_code_ref, s.init_script_ref, s.snapshot_ref, s.snapshot_domains, s.snapshot_created_at, s.snapshot_expire_at, s.keep_alive_until, s.last_active_at, s.expire_at, s.created_at, s.updated_at
FROM sandbox s JOIN updated u ON u.id = s.id;

-- name: UpdateSandboxSnapshot :one
WITH updated AS (
    UPDATE sandbox SET snapshot_ref = $3, snapshot_domains = $4, snapshot_created_at = $5, snapshot_expire_at = $6, updated_at = now()
    WHERE sandbox.tenant_id = $1 AND sandbox.id = $2 RETURNING sandbox.id
)
SELECT s.id, s.tenant_id, s.runtime_id, s.image_id, s.namespace, s.source_ref, s.scope_ref, s.composition_digest, s.composition_snapshot, s.access_profile, s.owner_account_id, s.shared_account_ids, s.phase, s.status, s.keep_alive, s.snapshot_enabled, s.code_storage_key, s.code_hash, s.workspace_revision, s.init_code_ref, s.init_script_ref, s.snapshot_ref, s.snapshot_domains, s.snapshot_created_at, s.snapshot_expire_at, s.keep_alive_until, s.last_active_at, s.expire_at, s.created_at, s.updated_at
FROM sandbox s JOIN updated u ON u.id = s.id;

-- name: CreateSandboxTool :one
INSERT INTO sandbox_tool (id, tenant_id, sandbox_id, tool_id, access_endpoint, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, tenant_id, sandbox_id, tool_id, access_endpoint, status;

-- name: ListSandboxTools :many
SELECT st.id, st.tenant_id, st.sandbox_id, st.tool_id, st.access_endpoint, st.status,
       t.code, t.kind, t.resource_spec
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
INSERT INTO sandbox_recycle_outbox (id, tenant_id, sandbox_id, source_ref, scope_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, 0, NULL, now(), now(), '', NULL)
RETURNING id, tenant_id, sandbox_id, source_ref, scope_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: ClaimPendingSandboxRecycleOutbox :many
WITH exhausted AS (
    UPDATE sandbox_recycle_outbox AS expired
    SET status = 4, last_error = 'recycle lease expired after retry limit', updated_at = now(), lease_token = '', lease_until = NULL
    WHERE expired.status = 2 AND expired.lease_until <= @stale_before::timestamptz AND expired.retry_count >= @max_attempts
    RETURNING expired.id
), candidates AS (
    SELECT o.id
    FROM sandbox_recycle_outbox o
    WHERE (o.status IN (1, 4) OR (o.status = 2 AND o.lease_until <= @stale_before::timestamptz))
      AND o.retry_count < @max_attempts
    ORDER BY o.created_at ASC, o.id ASC
    LIMIT @page_limit
    FOR UPDATE SKIP LOCKED
)
UPDATE sandbox_recycle_outbox AS outbox
SET status = 2, retry_count = outbox.retry_count + 1, updated_at = now(), lease_token = @lease_token, lease_until = @lease_until::timestamptz
FROM candidates
WHERE outbox.id = candidates.id
RETURNING outbox.id, outbox.tenant_id, outbox.sandbox_id, outbox.source_ref, outbox.scope_ref, outbox.owner_account_id, outbox.reason, outbox.trace_id, outbox.recycled_at, outbox.status, outbox.retry_count, outbox.last_error, outbox.created_at, outbox.updated_at, outbox.lease_token, outbox.lease_until;

-- name: MarkSandboxRecycleOutboxPublished :one
UPDATE sandbox_recycle_outbox
SET status = 3, last_error = NULL, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) AND status = 2 AND lease_token = sqlc.arg(lease_token) AND lease_until > now()
RETURNING id, tenant_id, sandbox_id, source_ref, scope_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: MarkSandboxRecycleOutboxFailed :one
UPDATE sandbox_recycle_outbox
SET status = 4, last_error = sqlc.arg(last_error), updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) AND status = 2 AND lease_token = sqlc.arg(lease_token) AND lease_until > now()
RETURNING id, tenant_id, sandbox_id, source_ref, scope_ref, owner_account_id, reason, trace_id, recycled_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

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
