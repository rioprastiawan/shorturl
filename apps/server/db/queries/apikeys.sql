-- name: CreateAPIKey :one
INSERT INTO api_keys (workspace_id, name, key_prefix, key_hash, scopes, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAPIKeyByPrefix :one
SELECT * FROM api_keys WHERE key_prefix = $1;

-- name: ListAPIKeys :many
SELECT * FROM api_keys WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: RevokeAPIKey :execrows
UPDATE api_keys
SET revoked_at = now()
WHERE id = $1 AND workspace_id = $2 AND revoked_at IS NULL;

-- name: TouchAPIKey :exec
UPDATE api_keys SET last_used_at = now() WHERE id = $1;


-- name: CreateIdempotencyRecord :one
INSERT INTO idempotency_keys (workspace_id, idempotency_key, request_hash, link_id, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIdempotencyRecord :one
SELECT * FROM idempotency_keys
WHERE workspace_id = $1 AND idempotency_key = $2 AND expires_at > now();

-- name: DeleteExpiredIdempotencyRecords :execrows
DELETE FROM idempotency_keys WHERE expires_at <= now();
