-- name: CreateLink :one
INSERT INTO links (
    workspace_id, domain_id, slug, destination_url, title, status,
    redirect_type, password_hash, expires_at, max_clicks,
    external_reference, metadata, created_by, created_via
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetLink :one
SELECT * FROM links WHERE id = $1;

-- name: GetLinkInWorkspace :one
SELECT
    sqlc.embed(links),
    domains.hostname
FROM links
JOIN domains ON domains.id = links.domain_id
WHERE links.id = $1 AND links.workspace_id = $2;

-- name: GetLinkByExternalReference :one
SELECT
    sqlc.embed(links),
    domains.hostname
FROM links
JOIN domains ON domains.id = links.domain_id
WHERE links.workspace_id = $1 AND links.external_reference = $2;

-- Redirect hot path fallback: only used on a Redis cache miss.
-- name: ResolveLink :one
SELECT
    links.id,
    links.workspace_id,
    links.destination_url,
    links.redirect_type,
    links.status,
    links.expires_at,
    links.max_clicks,
    links.click_count,
    links.password_hash
FROM links
JOIN domains ON domains.id = links.domain_id
WHERE LOWER(domains.hostname) = LOWER(sqlc.arg(hostname)::text)
  AND links.slug = sqlc.arg(slug)::text
  AND domains.status = 'active';

-- name: SlugExistsOnDomain :one
SELECT EXISTS (SELECT 1 FROM links WHERE domain_id = $1 AND slug = $2);

-- name: UpdateLink :one
UPDATE links
SET destination_url = coalesce(sqlc.narg(destination_url), destination_url),
    title           = CASE WHEN sqlc.arg(set_title)::bool THEN sqlc.narg(title) ELSE title END,
    status          = coalesce(sqlc.narg(status), status),
    redirect_type   = coalesce(sqlc.narg(redirect_type), redirect_type),
    expires_at      = CASE WHEN sqlc.arg(set_expires_at)::bool THEN sqlc.narg(expires_at) ELSE expires_at END,
    max_clicks      = CASE WHEN sqlc.arg(set_max_clicks)::bool THEN sqlc.narg(max_clicks) ELSE max_clicks END,
    password_hash   = CASE WHEN sqlc.arg(set_password)::bool THEN sqlc.narg(password_hash) ELSE password_hash END,
    metadata        = CASE WHEN sqlc.arg(set_metadata)::bool THEN sqlc.narg(metadata) ELSE metadata END,
    slug            = coalesce(sqlc.narg(slug), slug),
    domain_id       = coalesce(sqlc.narg(domain_id), domain_id)
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id)
RETURNING *;

-- name: RequestLinkDeletion :execrows
WITH requested AS (
    UPDATE links
    SET status = 'archived'
    WHERE links.id = sqlc.arg(link_id) AND links.workspace_id = sqlc.arg(workspace_id)
    RETURNING id, workspace_id
)
INSERT INTO deletion_jobs (resource_type, resource_id, workspace_id, not_before)
SELECT 'link', requested.id, requested.workspace_id, now() + interval '5 minutes' FROM requested
ON CONFLICT (resource_type, resource_id) DO NOTHING;

-- name: CountLinksInWorkspace :one
SELECT
    count(*) AS total,
    count(*) FILTER (WHERE status = 'active') AS active
FROM links
WHERE workspace_id = $1;

-- name: IncrementLinkClickCount :exec
UPDATE links SET click_count = click_count + sqlc.arg(delta)::bigint
WHERE id = sqlc.arg(id);

-- name: ListLinksForDomain :many
SELECT links.slug, domains.hostname
FROM links
JOIN domains ON domains.id = links.domain_id
WHERE links.domain_id = $1;

-- name: CountLinksForDomain :one
SELECT count(*) FROM links WHERE domain_id = $1;

-- name: RecentLinks :many
SELECT
    sqlc.embed(links),
    domains.hostname
FROM links
JOIN domains ON domains.id = links.domain_id
WHERE links.workspace_id = $1
ORDER BY links.created_at DESC
LIMIT $2;
