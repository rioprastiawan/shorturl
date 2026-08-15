-- name: CreateDomain :one
INSERT INTO domains (workspace_id, hostname, verification_token, verification_method, is_default)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetDomain :one
SELECT * FROM domains WHERE id = $1;

-- name: GetDomainInWorkspace :one
SELECT * FROM domains WHERE id = $1 AND workspace_id = $2;

-- name: GetDomainByHostname :one
SELECT * FROM domains WHERE LOWER(hostname) = LOWER(sqlc.arg(hostname)::text);

-- name: GetDomainByHostnameInWorkspace :one
SELECT * FROM domains
WHERE LOWER(hostname) = LOWER(sqlc.arg(hostname)::text)
  AND workspace_id = sqlc.arg(workspace_id);

-- name: ListDomains :many
SELECT * FROM domains WHERE workspace_id = $1 ORDER BY is_default DESC, created_at ASC;

-- name: ListActiveDomains :many
SELECT * FROM domains WHERE status = 'active' ORDER BY hostname ASC;

-- name: GetDefaultDomain :one
SELECT * FROM domains
WHERE workspace_id = $1 AND is_default AND status = 'active';

-- name: CountActiveDomains :one
SELECT count(*) FROM domains WHERE workspace_id = $1 AND status = 'active';

-- name: UpdateDomainVerification :one
UPDATE domains
SET status = $2,
    ssl_status = $3,
    verification_error = $4,
    verified_at = $5,
    last_checked_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateDomainSSLStatus :exec
UPDATE domains SET ssl_status = $2 WHERE id = $1;

-- name: ClearDefaultDomain :exec
UPDATE domains SET is_default = FALSE WHERE workspace_id = $1 AND is_default;

-- name: SetDefaultDomain :one
UPDATE domains SET is_default = TRUE WHERE id = $1 RETURNING *;

-- name: DeleteDomain :execrows
DELETE FROM domains WHERE id = $1 AND workspace_id = $2;

-- name: ListDomainHostnamesForWorkspace :many
SELECT hostname FROM domains WHERE workspace_id = $1;
