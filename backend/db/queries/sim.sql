-- name: GetSimPackageByCodeVersion :one
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
       backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at
FROM sim_package
WHERE code = $1 AND version = $2;

-- name: GetSimPackageByID :one
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
       backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at
FROM sim_package
WHERE id = $1;

-- name: ListSimPackages :many
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
       backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at
FROM sim_package
WHERE ($1::smallint = 0 OR status = $1)
  AND ($2::text = '' OR category = $2)
  AND ($3::text = '' OR code ILIKE '%' || $3 || '%' OR name ILIKE '%' || $3 || '%')
  AND ($6::bigint = 0 OR (author_type = 2 AND author_id = $6))
ORDER BY updated_at DESC, id DESC
LIMIT $4 OFFSET $5;

-- name: CountSimPackages :one
SELECT COUNT(*)::bigint
FROM sim_package
WHERE ($1::smallint = 0 OR status = $1)
  AND ($2::text = '' OR category = $2)
  AND ($3::text = '' OR code ILIKE '%' || $3 || '%' OR name ILIKE '%' || $3 || '%')
  AND ($4::bigint = 0 OR (author_type = 2 AND author_id = $4));

-- name: ListSimPackageVersions :many
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
       backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at
FROM sim_package
WHERE code = $1
ORDER BY created_at DESC, id DESC;

-- name: CreateSimPackage :one
INSERT INTO sim_package (
    id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
    backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now(), now())
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
          backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at;

-- name: UpsertBuiltinSimPackage :one
-- 平台内置仿真包按 (code, version) 幂等入库。
-- 内置包不来自教师上传,而是平台随版本交付的标准库(见 docs/04-仿真可视化引擎/09-内置仿真包标准库.md),
-- 故直接落 author_type=1、compute=1(浏览器执行)、status=3(已上架):它不经审核流程,
-- 审核针对的是外部提交的包;entry/backend_adapter 恒为 NULL —— 内置包由 sim-sdk registry 按 code 装配。
-- 重跑部署 seed 时按 code+version 覆盖协议字段,不新建行、不改 created_at。
INSERT INTO sim_package (
    id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
    backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8, NULL, NULL, '{}'::jsonb, $9, $10, 1, NULL, 3, now(), now())
ON CONFLICT (code, version) DO UPDATE
SET name = EXCLUDED.name,
    category = EXCLUDED.category,
    scale_limit = EXCLUDED.scale_limit,
    bundle_key = EXCLUDED.bundle_key,
    bundle_hash = EXCLUDED.bundle_hash,
    interaction_schema = EXCLUDED.interaction_schema,
    code_trace = EXCLUDED.code_trace,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
          backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at;

-- name: ArchiveRetiredBuiltinSimPackages :many
-- 下架已从标准库移除的内置包。
-- 内置包被删掉版本时不能物理删除:已有实验定义与仿真会话按 (code, version) 引用它,
-- 删了会让历史实验取不到场景。改为 status=4(已下架),既让新建选不到、也保住旧引用可解释。
UPDATE sim_package
SET status = 4, updated_at = now()
WHERE author_type = 1
  AND status <> 4
  AND (code || '@' || version) <> ALL(sqlc.arg(live_keys)::text[])
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
          backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at;

-- name: UpdateSimPackageDraft :one
-- 更新草稿或被退回的包。compute、backend_adapter 与 author_type 不在可更新列中:
-- 它们由服务端按作者类型派生,更新一个包不改变它的作者,也就不该改变执行位置与运行能力。
UPDATE sim_package
SET name = $2,
    category = $3,
    scale_limit = $4,
    bundle_key = $5,
    bundle_hash = $6,
    entry = $7,
    interaction_schema = $8,
    code_trace = $9,
    status = $10,
    updated_at = now()
WHERE id = $1 AND status IN (1, 5)
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
          backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at;

-- name: UpdateSimPackageStatus :one
UPDATE sim_package
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, entry,
          backend_adapter, backend_config, interaction_schema, code_trace, author_type, author_id, status, created_at, updated_at;

-- name: CreateSimPackageReview :one
INSERT INTO sim_package_review (id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at)
VALUES ($1, $2, $3, $4, NULL, 1, NULL, now(), now())
RETURNING id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at;

-- name: GetSimReviewByID :one
SELECT id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at
FROM sim_package_review
WHERE id = $1;

-- name: GetLatestSimReviewForPackage :one
SELECT id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at
FROM sim_package_review
WHERE package_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListSimReviews :many
SELECT r.id, r.package_id, r.submitter_id, r.preview_report, r.reviewer_id, r.result, r.comment, r.created_at, r.updated_at,
       p.code, p.version, p.name, p.category, p.compute, p.status
FROM sim_package_review r
JOIN sim_package p ON p.id = r.package_id
WHERE ($1::smallint = 0 OR r.result = $1)
ORDER BY r.created_at ASC, r.id ASC
LIMIT $2 OFFSET $3;

-- name: CountSimReviews :one
SELECT COUNT(*)::bigint
FROM sim_package_review
WHERE ($1::smallint = 0 OR result = $1);

-- name: MergeSimValidationReport :one
UPDATE sim_package_review
SET preview_report = preview_report || $2,
    updated_at = now()
WHERE package_id = $1 AND result = 1
RETURNING id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at;

-- name: ClaimSimPackagesForPreview :many
-- 认领待隔离预览的包:审核中、且报告里两项动态校验都还没有结论。
-- 四项审核门禁中 determinism_check 与 worker_preview 只能由隔离预览产出,
-- 没有这个认领查询就没有生产者,教师提交的包会永久停在待审(见 docs/04-仿真可视化引擎/06-业务流程与状态机.md §4)。
-- FOR UPDATE SKIP LOCKED:多副本部署时各副本认领互不重复,也不互相阻塞。
SELECT p.id, p.code, p.version, p.name, p.category, p.compute, p.scale_limit, p.bundle_key, p.bundle_hash, p.entry,
       p.backend_adapter, p.backend_config, p.interaction_schema, p.code_trace, p.author_type, p.author_id, p.status,
       p.created_at, p.updated_at
FROM sim_package p
JOIN sim_package_review r ON r.package_id = p.id AND r.result = 1
WHERE p.status = 2
  AND p.compute = 2
  AND NOT (r.preview_report ? 'determinism_check' AND r.preview_report ? 'worker_preview')
ORDER BY r.created_at ASC, p.id ASC
LIMIT $1
FOR UPDATE OF p SKIP LOCKED;

-- name: CompleteSimReview :one
UPDATE sim_package_review
SET result = $2,
    reviewer_id = $3,
    comment = $4,
    updated_at = now()
WHERE id = $1 AND result = 1
RETURNING id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at;

-- name: CreateSimSession :one
INSERT INTO sim_session (id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())
RETURNING id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at;

-- name: GetSimSession :one
SELECT id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at
FROM sim_session
WHERE tenant_id = $1 AND id = $2;

-- name: GetSimSessionWithPackage :one
SELECT s.id, s.tenant_id, s.package_id, s.source_ref, s.owner_account_id, s.seed, s.init_params, s.compute, s.status, s.created_at, s.updated_at,
       p.code, p.version, p.name, p.category, p.scale_limit, p.bundle_key, p.bundle_hash, p.entry, p.backend_adapter, p.backend_config,
       p.interaction_schema, p.status AS package_status
FROM sim_session s
JOIN sim_package p ON p.id = s.package_id
WHERE s.tenant_id = $1 AND s.id = $2;

-- name: CountActiveIsolatedSimSessions :one
-- 统计本租户当前占用集群资源的隔离执行会话数,供并发闸门
-- SIM_BACKEND_MAX_CONCURRENT_SESSIONS_PER_TENANT 使用。
-- 一个隔离会话一个 Pod,没有这道闸门循环建会话可耗尽节点;浏览器执行的会话不占集群资源故不计入。
SELECT COUNT(*)::bigint
FROM sim_session
WHERE tenant_id = $1 AND compute = 2 AND status IN (1, 2, 3);

-- name: UpdateSimSessionStatus :one
UPDATE sim_session
SET status = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
  AND status IN (1, 2, 3, 4)
  AND $3 IN (2, 3, 4, 5, 6)
RETURNING id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at;

-- name: ArchiveSimSessionsBySourceRef :many
UPDATE sim_session
SET status = 5, updated_at = now()
WHERE tenant_id = $1 AND source_ref = $2 AND status IN (1, 2, 3, 4)
RETURNING id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at;

-- name: GetLastSimAction :one
SELECT id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at
FROM sim_action_log
WHERE tenant_id = $1 AND session_id = $2
ORDER BY seq DESC
LIMIT 1;

-- name: GetSimActionBySeq :one
SELECT id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at
FROM sim_action_log
WHERE tenant_id = $1 AND session_id = $2 AND seq = $3;

-- name: CreateSimAction :one
INSERT INTO sim_action_log (id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at;

-- name: ListSimActions :many
SELECT id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at
FROM sim_action_log
WHERE tenant_id = $1 AND session_id = $2
ORDER BY seq ASC;

-- name: UpsertSimCheckpoint :one
INSERT INTO sim_checkpoint (id, tenant_id, session_id, checkpoint_id, answer, achieved, created_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (tenant_id, session_id, checkpoint_id) DO UPDATE
SET answer = EXCLUDED.answer,
    achieved = EXCLUDED.achieved,
    created_at = now()
RETURNING id, tenant_id, session_id, checkpoint_id, answer, achieved, created_at;

-- name: CreateSimShare :one
INSERT INTO sim_share (id, tenant_id, session_id, code, created_by, status, expire_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 1, $6, now(), now())
RETURNING id, tenant_id, session_id, code, created_by, status, expire_at, created_at, updated_at;

-- name: GetSimShareByCode :one
SELECT id, tenant_id, session_id, code, created_by, status, expire_at, created_at, updated_at
FROM sim_share
WHERE code = $1;
