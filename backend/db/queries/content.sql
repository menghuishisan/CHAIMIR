-- name: CreateContentItem :one
INSERT INTO content_item (id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 0, $15, now(), now(), NULL)
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at;

-- name: CreateContentBody :one
INSERT INTO content_body (item_id, tenant_id, body, sensitive_fields, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
RETURNING item_id, tenant_id, body, sensitive_fields, created_at, updated_at;

-- name: UpdateDraftContentItem :one
UPDATE content_item
SET title = $3,
    category_id = $4,
    difficulty = $5,
    tags = $6,
    knowledge_points = $7,
    visibility = $8,
    version_hash = $9,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at;

-- name: UpdateContentBody :one
UPDATE content_body
SET body = $3,
    sensitive_fields = $4,
    updated_at = now()
WHERE tenant_id = $1 AND item_id = $2
RETURNING item_id, tenant_id, body, sensitive_fields, created_at, updated_at;

-- name: GetContentItemByID :one
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at
FROM content_item
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: GetContentItemByRef :one
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at
FROM content_item
WHERE code = $1 AND version = $2 AND deleted_at IS NULL AND (tenant_id = $3 OR visibility = 3)
ORDER BY CASE WHEN tenant_id = $3 THEN 0 ELSE 1 END
LIMIT 1;

-- name: GetContentItemWithBodyByRef :one
SELECT i.id, i.tenant_id, i.code, i.version, i.type, i.title, i.category_id, i.difficulty, i.tags, i.knowledge_points, i.author_id, i.author_type, i.visibility, i.status, i.usage_count, i.version_hash, i.created_at, i.updated_at, i.deleted_at,
       b.body, b.sensitive_fields
FROM content_item i
JOIN content_body b ON b.tenant_id = i.tenant_id AND b.item_id = i.id
WHERE i.code = $1 AND i.version = $2 AND i.deleted_at IS NULL AND (i.tenant_id = $3 OR i.visibility = 3)
ORDER BY CASE WHEN i.tenant_id = $3 THEN 0 ELSE 1 END
LIMIT 1;

-- name: GetContentItemWithBodyByID :one
SELECT i.id, i.tenant_id, i.code, i.version, i.type, i.title, i.category_id, i.difficulty, i.tags, i.knowledge_points, i.author_id, i.author_type, i.visibility, i.status, i.usage_count, i.version_hash, i.created_at, i.updated_at, i.deleted_at,
       b.body, b.sensitive_fields
FROM content_item i
JOIN content_body b ON b.tenant_id = i.tenant_id AND b.item_id = i.id
WHERE i.tenant_id = $1 AND i.id = $2 AND i.deleted_at IS NULL;

-- name: ListContentItems :many
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at
FROM content_item
WHERE deleted_at IS NULL
  AND (tenant_id = $1 OR visibility = 3)
  AND ($2::smallint = 0 OR type = $2)
  AND ($3::bigint = 0 OR category_id = $3)
  AND ($4::smallint = 0 OR difficulty = $4)
  AND ($5::text = '' OR tags @> ARRAY[$5]::text[])
  AND ($6::text = '' OR knowledge_points @> ARRAY[$6]::text[])
  AND ($7::text = '' OR title ILIKE '%' || $7 || '%' OR code ILIKE '%' || $7 || '%')
  AND ($8::smallint = 0 OR visibility = $8)
  AND ($9::smallint = 0 OR status = $9)
  AND ($10::bigint = 0 OR author_id = $10)
ORDER BY updated_at DESC, id DESC
LIMIT $11 OFFSET $12;

-- name: CountContentItems :one
SELECT COUNT(*)::bigint
FROM content_item
WHERE deleted_at IS NULL
  AND (tenant_id = $1 OR visibility = 3)
  AND ($2::smallint = 0 OR type = $2)
  AND ($3::bigint = 0 OR category_id = $3)
  AND ($4::smallint = 0 OR difficulty = $4)
  AND ($5::text = '' OR tags @> ARRAY[$5]::text[])
  AND ($6::text = '' OR knowledge_points @> ARRAY[$6]::text[])
  AND ($7::text = '' OR title ILIKE '%' || $7 || '%' OR code ILIKE '%' || $7 || '%')
  AND ($8::smallint = 0 OR visibility = $8)
  AND ($9::smallint = 0 OR status = $9)
  AND ($10::bigint = 0 OR author_id = $10);

-- name: ListContentVersions :many
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at
FROM content_item
WHERE code = $1 AND deleted_at IS NULL AND (tenant_id = $2 OR visibility = 3)
ORDER BY created_at DESC, id DESC;

-- name: PublishContentItem :one
UPDATE content_item
SET status = 2, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at;

-- name: DeprecateContentItem :one
UPDATE content_item
SET status = 3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at;

-- name: SoftDeleteDraftContentItem :one
UPDATE content_item
SET deleted_at = now(), updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1 AND usage_count = 0 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at;

-- name: SetContentVisibility :one
UPDATE content_item
SET visibility = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at;

-- name: IncrementContentUsage :one
UPDATE content_item
SET usage_count = usage_count + 1, updated_at = now()
WHERE code = $1 AND version = $2 AND tenant_id = $3 AND status = 2 AND deleted_at IS NULL
RETURNING id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at;

-- name: CreateContentCategory :one
INSERT INTO content_category (id, tenant_id, parent_id, name, sort, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, now(), now(), NULL)
RETURNING id, tenant_id, parent_id, name, sort, created_at, updated_at, deleted_at;

-- name: UpdateContentCategory :one
UPDATE content_category
SET parent_id = $3, name = $4, sort = $5, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, parent_id, name, sort, created_at, updated_at, deleted_at;

-- name: DeleteContentCategory :one
UPDATE content_category
SET deleted_at = now(), updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, parent_id, name, sort, created_at, updated_at, deleted_at;

-- name: ListContentCategories :many
SELECT id, tenant_id, parent_id, name, sort, created_at, updated_at, deleted_at
FROM content_category
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY parent_id NULLS FIRST, sort ASC, id ASC;

-- name: CreatePaper :one
INSERT INTO paper (id, tenant_id, name, author_id, gen_mode, gen_criteria, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, now(), now(), NULL)
RETURNING id, tenant_id, name, author_id, gen_mode, gen_criteria, created_at, updated_at, deleted_at;

-- name: DeletePaperItems :exec
DELETE FROM paper_item WHERE tenant_id = $1 AND paper_id = $2;

-- name: CreatePaperItem :one
INSERT INTO paper_item (id, tenant_id, paper_id, item_code, item_version, score, seq, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id, tenant_id, paper_id, item_code, item_version, score, seq, created_at;

-- name: GetPaper :one
SELECT id, tenant_id, name, author_id, gen_mode, gen_criteria, created_at, updated_at, deleted_at
FROM paper
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: ListPapers :many
SELECT id, tenant_id, name, author_id, gen_mode, gen_criteria, created_at, updated_at, deleted_at
FROM paper
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY updated_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountPapers :one
SELECT COUNT(*)::bigint FROM paper WHERE tenant_id = $1 AND deleted_at IS NULL;

-- name: ListPaperItems :many
SELECT id, tenant_id, paper_id, item_code, item_version, score, seq, created_at
FROM paper_item
WHERE tenant_id = $1 AND paper_id = $2
ORDER BY seq ASC;

-- name: RandomPickContentItems :many
SELECT id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash, created_at, updated_at, deleted_at
FROM content_item
WHERE tenant_id = $1 AND deleted_at IS NULL AND status = 2
  AND ($2::smallint = 0 OR type = $2)
  AND (cardinality($3::smallint[]) = 0 OR difficulty = ANY($3::smallint[]))
  AND (cardinality($4::text[]) = 0 OR knowledge_points && $4::text[])
ORDER BY random()
LIMIT $5;
