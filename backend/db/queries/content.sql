-- M5 content sqlc 查询源。
-- 约定:租户自有内容走 RLS;跨校共享库只通过受控特权事务读取 visibility=shared 的已发布内容。

-- name: CreateContentItem :one
INSERT INTO content_item (
    id, tenant_id, code, version, type, title, category_id, difficulty,
    tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 0, $15)
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at;

-- name: CreateContentBody :one
INSERT INTO content_body (item_id, tenant_id, body, sensitive_fields)
VALUES ($1, $2, $3, $4)
RETURNING item_id, tenant_id, body, sensitive_fields, created_at, updated_at;

-- name: GetContentItemByID :one
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at FROM content_item
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetContentByCodeVersion :one
SELECT
    ci.id, ci.tenant_id, ci.code, ci.version, ci.type, ci.title, ci.category_id,
    ci.difficulty, ci.tags, ci.knowledge_points, ci.author_id, ci.author_type,
    ci.visibility, ci.status, ci.usage_count, ci.body_hash, ci.deleted_at,
    ci.created_at, ci.updated_at,
    cb.body, cb.sensitive_fields
FROM content_item ci
JOIN content_body cb ON cb.item_id = ci.id
WHERE ci.code = $1 AND ci.version = $2 AND ci.deleted_at IS NULL;

-- name: GetSharedContentByCodeVersion :one
SELECT
    ci.id, ci.tenant_id, ci.code, ci.version, ci.type, ci.title, ci.category_id,
    ci.difficulty, ci.tags, ci.knowledge_points, ci.author_id, ci.author_type,
    ci.visibility, ci.status, ci.usage_count, ci.body_hash, ci.deleted_at,
    ci.created_at, ci.updated_at,
    cb.body, cb.sensitive_fields
FROM content_item ci
JOIN content_body cb ON cb.item_id = ci.id
WHERE ci.code = $1 AND ci.version = $2
  AND ci.visibility = 3 AND ci.status = 2 AND ci.deleted_at IS NULL;

-- name: ListContentItems :many
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at FROM content_item
WHERE deleted_at IS NULL
  AND (sqlc.narg('type')::SMALLINT IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('category_id')::BIGINT IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('difficulty')::SMALLINT IS NULL OR difficulty = sqlc.narg('difficulty'))
  AND (sqlc.narg('visibility')::SMALLINT IS NULL OR visibility = sqlc.narg('visibility'))
  AND (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('tag')::TEXT IS NULL OR tags @> ARRAY[sqlc.narg('tag')::TEXT])
  AND (sqlc.narg('kp')::TEXT IS NULL OR knowledge_points @> ARRAY[sqlc.narg('kp')::TEXT])
  AND (sqlc.narg('keyword')::TEXT IS NULL OR code ILIKE '%' || sqlc.narg('keyword') || '%' OR title ILIKE '%' || sqlc.narg('keyword') || '%')
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountContentItems :one
SELECT count(*) FROM content_item
WHERE deleted_at IS NULL
  AND (sqlc.narg('type')::SMALLINT IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('category_id')::BIGINT IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('difficulty')::SMALLINT IS NULL OR difficulty = sqlc.narg('difficulty'))
  AND (sqlc.narg('visibility')::SMALLINT IS NULL OR visibility = sqlc.narg('visibility'))
  AND (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('tag')::TEXT IS NULL OR tags @> ARRAY[sqlc.narg('tag')::TEXT])
  AND (sqlc.narg('kp')::TEXT IS NULL OR knowledge_points @> ARRAY[sqlc.narg('kp')::TEXT])
  AND (sqlc.narg('keyword')::TEXT IS NULL OR code ILIKE '%' || sqlc.narg('keyword') || '%' OR title ILIKE '%' || sqlc.narg('keyword') || '%');

-- name: ListSharedContentItems :many
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at FROM content_item
WHERE deleted_at IS NULL AND visibility = 3 AND status = 2
  AND (sqlc.narg('type')::SMALLINT IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('keyword')::TEXT IS NULL OR code ILIKE '%' || sqlc.narg('keyword') || '%' OR title ILIKE '%' || sqlc.narg('keyword') || '%')
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListContentVersions :many
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at FROM content_item
WHERE code = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateContentDraft :one
UPDATE content_item
SET title = $2,
    category_id = $3,
    difficulty = $4,
    tags = $5,
    knowledge_points = $6,
    visibility = $7,
    body_hash = $8
WHERE id = $1 AND status = 1 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at;

-- name: UpdateContentBody :one
UPDATE content_body
SET body = $2,
    sensitive_fields = $3
WHERE item_id = $1
RETURNING item_id, tenant_id, body, sensitive_fields, created_at, updated_at;

-- name: UpdateContentStatus :one
UPDATE content_item
SET status = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at;

-- name: UpdateContentVisibility :one
UPDATE content_item
SET visibility = $2
WHERE id = $1 AND status = 2 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at;

-- name: SoftDeleteContentItem :one
UPDATE content_item
SET deleted_at = now()
WHERE id = $1 AND status = 1 AND usage_count = 0 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at;

-- name: IncrementContentUsage :one
UPDATE content_item
SET usage_count = usage_count + 1
WHERE code = $1 AND version = $2 AND status = 2 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at;

-- name: CreateContentCategory :one
INSERT INTO content_category (id, tenant_id, parent_id, name, sort)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, tenant_id, parent_id, name, sort, deleted_at, created_at, updated_at;

-- name: ListContentCategories :many
SELECT id, tenant_id, parent_id, name, sort, deleted_at, created_at, updated_at FROM content_category
WHERE deleted_at IS NULL
ORDER BY parent_id NULLS FIRST, sort ASC, created_at ASC;

-- name: UpdateContentCategory :one
UPDATE content_category
SET parent_id = $2, name = $3, sort = $4
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, tenant_id, parent_id, name, sort, deleted_at, created_at, updated_at;

-- name: DeleteContentCategory :one
UPDATE content_category
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, tenant_id, parent_id, name, sort, deleted_at, created_at, updated_at;

-- name: CreatePaper :one
INSERT INTO paper (id, tenant_id, name, author_id, gen_mode, gen_criteria)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, tenant_id, name, author_id, gen_mode, gen_criteria, deleted_at, created_at, updated_at;

-- name: ListPapers :many
SELECT id, tenant_id, name, author_id, gen_mode, gen_criteria, deleted_at, created_at, updated_at FROM paper
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetPaperByID :one
SELECT id, tenant_id, name, author_id, gen_mode, gen_criteria, deleted_at, created_at, updated_at FROM paper
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeletePaperItems :exec
DELETE FROM paper_item WHERE paper_id = $1;

-- name: CreatePaperItem :one
INSERT INTO paper_item (id, tenant_id, paper_id, item_code, item_version, score, seq)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, tenant_id, paper_id, item_code, item_version, score, seq, created_at;

-- name: ListPaperItems :many
SELECT id, tenant_id, paper_id, item_code, item_version, score, seq, created_at FROM paper_item
WHERE paper_id = $1
ORDER BY seq ASC;

-- name: ListPublishedItemsForRandomPaper :many
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, body_hash, deleted_at, created_at, updated_at FROM content_item
WHERE deleted_at IS NULL AND status = 2
  AND (sqlc.narg('type')::SMALLINT IS NULL OR type = sqlc.narg('type'))
  AND (cardinality(sqlc.arg('difficulties')::SMALLINT[]) = 0 OR difficulty = ANY(sqlc.arg('difficulties')::SMALLINT[]))
  AND (cardinality(sqlc.arg('knowledge_points')::TEXT[]) = 0 OR knowledge_points && sqlc.arg('knowledge_points')::TEXT[])
ORDER BY random()
LIMIT $1;
