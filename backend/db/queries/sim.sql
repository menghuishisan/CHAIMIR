-- name: GetSimPackageByCodeVersion :one
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
       backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at
FROM sim_package
WHERE code = $1 AND version = $2;

-- name: GetSimPackageByID :one
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
       backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at
FROM sim_package
WHERE id = $1;

-- name: ListSimPackages :many
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
       backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at
FROM sim_package
WHERE ($1::smallint = 0 OR status = $1)
  AND ($2::text = '' OR category = $2)
  AND ($3::text = '' OR code ILIKE '%' || $3 || '%' OR name ILIKE '%' || $3 || '%')
ORDER BY updated_at DESC, id DESC
LIMIT $4 OFFSET $5;

-- name: CountSimPackages :one
SELECT COUNT(*)::bigint
FROM sim_package
WHERE ($1::smallint = 0 OR status = $1)
  AND ($2::text = '' OR category = $2)
  AND ($3::text = '' OR code ILIKE '%' || $3 || '%' OR name ILIKE '%' || $3 || '%');

-- name: ListSimPackageVersions :many
SELECT id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
       backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at
FROM sim_package
WHERE code = $1
ORDER BY created_at DESC, id DESC;

-- name: CreateSimPackage :one
INSERT INTO sim_package (
    id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
    backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, now(), now())
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
          backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at;

-- name: UpdateSimPackageDraft :one
UPDATE sim_package
SET name = $2,
    category = $3,
    compute = $4,
    scale_limit = $5,
    bundle_key = $6,
    bundle_hash = $7,
    backend_adapter = $8,
    backend_config = $9,
    status = $10,
    updated_at = now()
WHERE id = $1 AND status IN (1, 5)
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
          backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at;

-- name: UpdateSimPackageStatus :one
UPDATE sim_package
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash,
          backend_adapter, backend_config, author_type, author_id, status, created_at, updated_at;

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
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 2, now(), now())
RETURNING id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at;

-- name: GetSimSession :one
SELECT id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at
FROM sim_session
WHERE tenant_id = $1 AND id = $2;

-- name: GetSimSessionWithPackage :one
SELECT s.id, s.tenant_id, s.package_id, s.source_ref, s.owner_account_id, s.seed, s.init_params, s.compute, s.status, s.created_at, s.updated_at,
       p.code, p.version, p.name, p.category, p.bundle_key, p.bundle_hash, p.backend_adapter, p.backend_config, p.status AS package_status
FROM sim_session s
JOIN sim_package p ON p.id = s.package_id
WHERE s.tenant_id = $1 AND s.id = $2;

-- name: ArchiveSimSession :one
UPDATE sim_session
SET status = 5, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status <> 5
RETURNING id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status, created_at, updated_at;

-- name: ArchiveSimSessionsBySourceRef :many
UPDATE sim_session
SET status = 5, updated_at = now()
WHERE tenant_id = $1 AND source_ref = $2 AND status <> 5
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
