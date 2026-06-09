-- M4 sim sqlc 查询源。
-- 约定:租户表依赖 RLS 透明过滤;平台级仿真包/审核通过 app 事务访问。

-- name: CreateSimPackage :one
INSERT INTO sim_package (
    id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
    backend_adapter, backend_config, author_type, author_id, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at;

-- name: GetSimPackageByID :one
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at FROM sim_package WHERE id = $1;

-- name: GetSimPackageByCodeVersion :one
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at FROM sim_package WHERE code = $1 AND version = $2;

-- name: ListSimPackages :many
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at FROM sim_package
WHERE (sqlc.narg('category')::VARCHAR IS NULL OR category = sqlc.narg('category'))
  AND (sqlc.narg('keyword')::VARCHAR IS NULL OR code ILIKE '%' || sqlc.narg('keyword') || '%' OR name ILIKE '%' || sqlc.narg('keyword') || '%')
  AND (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListSimPackageVersions :many
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at FROM sim_package
WHERE code = $1 AND status = 3
ORDER BY created_at DESC;

-- name: UpdateSimPackageDraft :one
UPDATE sim_package
SET name = $2,
    category = $3,
    scale_limit = $4,
    bundle_key = $5,
    bundle_hash = $6,
    backend_adapter = $7,
    backend_config = $8,
    status = 2
WHERE id = $1 AND status IN (1, 5)
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at;

-- name: UpdateSimPackageStatus :one
UPDATE sim_package
SET status = $2
WHERE id = $1
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at;

-- name: TransitionSimPackageStatus :one
UPDATE sim_package
SET status = $3
WHERE id = $1 AND status = $2
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at;

-- name: CreateSimPackageReview :one
INSERT INTO sim_package_review (id, package_id, submitter_id, preview_report, result)
VALUES ($1, $2, $3, $4, 1)
RETURNING id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at;

-- name: ListSimReviews :many
SELECT id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at FROM sim_package_review
WHERE (sqlc.narg('result')::SMALLINT IS NULL OR result = sqlc.narg('result'))
ORDER BY created_at ASC
LIMIT $1 OFFSET $2;

-- name: GetSimReviewByID :one
SELECT id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at FROM sim_package_review WHERE id = $1;

-- name: GetPendingSimReviewByPackageID :one
SELECT id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at FROM sim_package_review
WHERE package_id = $1 AND result = 1
ORDER BY created_at DESC
LIMIT 1;

-- name: CompleteSimReview :one
UPDATE sim_package_review
SET result = $2,
    reviewer_id = $3,
    comment = $4
WHERE id = $1 AND result = 1
RETURNING id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at;

-- name: UpdateSimReviewPreviewReport :one
UPDATE sim_package_review
SET preview_report = $2
WHERE id = (
    SELECT r.id FROM sim_package_review AS r
    WHERE r.package_id = $1 AND r.result = 1
    ORDER BY r.created_at DESC
    LIMIT 1
)
RETURNING id, package_id, submitter_id, preview_report, reviewer_id, result, comment, created_at, updated_at;

-- name: CreateSimSession :one
INSERT INTO sim_session (id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 2)
RETURNING id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at;

-- name: GetSimSessionByID :one
SELECT id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at FROM sim_session WHERE id = $1;

-- name: GetSimSessionWithPackage :one
SELECT
    ss.id, ss.tenant_id, ss.package_id, ss.source_ref, ss.owner_account_id, ss.seed, ss.init_params, ss.compute, ss.status, ss.created_at, ss.updated_at,
    sp.code AS package_code,
    sp.version AS package_version,
    sp.bundle_key AS package_bundle_key,
    sp.bundle_hash AS package_bundle_hash,
    sp.backend_adapter AS package_backend_adapter,
    sp.backend_config AS package_backend_config
FROM sim_session ss
JOIN sim_package sp ON sp.id = ss.package_id
WHERE ss.id = $1;

-- name: ArchiveSimSession :one
UPDATE sim_session
SET status = 5
WHERE id = $1
RETURNING id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at;

-- name: ArchiveSimSessionsBySourceRef :many
UPDATE sim_session
SET status = 5
WHERE source_ref = $1 AND status <> 5
RETURNING id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at;

-- name: GetLastSimAction :one
SELECT id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at FROM sim_action_log
WHERE session_id = $1
ORDER BY seq DESC
LIMIT 1;

-- name: GetSimActionBySeq :one
SELECT id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at FROM sim_action_log
WHERE session_id = $1 AND seq = $2;

-- name: CreateSimAction :one
INSERT INTO sim_action_log (id, tenant_id, session_id, seq, at_tick, event_type, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at;

-- name: ListSimActions :many
SELECT id, tenant_id, session_id, seq, at_tick, event_type, payload, created_at FROM sim_action_log
WHERE session_id = $1
ORDER BY seq ASC;

-- name: UpsertSimCheckpoint :one
INSERT INTO sim_checkpoint (id, tenant_id, session_id, checkpoint_id, answer, achieved)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (tenant_id, session_id, checkpoint_id) DO UPDATE
SET answer = EXCLUDED.answer,
    achieved = EXCLUDED.achieved,
    created_at = now()
RETURNING id, tenant_id, session_id, checkpoint_id, answer, achieved, created_at;

-- name: CreateSimShare :one
INSERT INTO sim_share (id, tenant_id, session_id, code, created_by, status, expire_at)
VALUES ($1, $2, $3, $4, $5, 1, $6)
RETURNING id, tenant_id, session_id, code, created_by, status, expire_at, created_at, updated_at;

-- name: GetSimShareByCode :one
SELECT id, tenant_id, session_id, code, created_by, status, expire_at, created_at, updated_at FROM sim_share
WHERE code = $1 AND status = 1 AND (expire_at IS NULL OR expire_at > now());
