-- name: CreateWorkspace :one
INSERT INTO workspaces (name, slug, owner_user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = $1 AND deletion_requested_at IS NULL;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspaces WHERE LOWER(slug) = LOWER(sqlc.arg(slug)::text)
  AND deletion_requested_at IS NULL;

-- name: ListWorkspacesForUser :many
SELECT
    sqlc.embed(workspaces),
    workspace_members.role
FROM workspaces
JOIN workspace_members ON workspace_members.workspace_id = workspaces.id
WHERE workspace_members.user_id = $1
  AND workspaces.deletion_requested_at IS NULL
ORDER BY workspaces.created_at ASC;

-- name: ListWorkspacesForUserPage :many
SELECT
    sqlc.embed(workspaces),
    workspace_members.role
FROM workspaces
JOIN workspace_members ON workspace_members.workspace_id = workspaces.id
WHERE workspace_members.user_id = sqlc.arg(user_id)
  AND workspaces.deletion_requested_at IS NULL
  AND (sqlc.arg(search)::text = '' OR workspaces.name ILIKE '%' || sqlc.arg(search)::text || '%')
  AND (
    NOT sqlc.arg(has_cursor)::boolean
    OR (workspaces.created_at, workspaces.id) > (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
  )
ORDER BY workspaces.created_at ASC, workspaces.id ASC
LIMIT sqlc.arg(page_limit);

-- name: UpdateWorkspace :one
UPDATE workspaces SET name = $2 WHERE id = $1 AND deletion_requested_at IS NULL RETURNING *;

-- name: RequestWorkspaceDeletion :execrows
WITH requested AS (
    UPDATE workspaces
    SET deletion_requested_at = now()
    WHERE workspaces.id = sqlc.arg(workspace_id) AND deletion_requested_at IS NULL
    RETURNING id
)
INSERT INTO deletion_jobs (resource_type, resource_id, workspace_id, not_before)
SELECT 'workspace', requested.id, requested.id, now() + interval '5 minutes' FROM requested
ON CONFLICT (resource_type, resource_id) DO NOTHING;

-- name: CountWorkspacesForUser :one
SELECT count(*) FROM workspace_members
JOIN workspaces ON workspaces.id = workspace_members.workspace_id
WHERE workspace_members.user_id = $1 AND workspaces.deletion_requested_at IS NULL;


-- name: AddWorkspaceMember :one
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWorkspaceMember :one
SELECT * FROM workspace_members WHERE workspace_id = $1 AND user_id = $2;

-- name: ListWorkspaceMembers :many
SELECT
    workspace_members.workspace_id,
    workspace_members.user_id,
    workspace_members.role,
    workspace_members.created_at,
    users.name,
    users.email,
    coalesce(user_two_factor.enabled, false)::boolean AS two_factor_enabled
FROM workspace_members
JOIN users ON users.id = workspace_members.user_id
LEFT JOIN user_two_factor ON user_two_factor.user_id = users.id
WHERE workspace_members.workspace_id = $1
ORDER BY
    CASE workspace_members.role
        WHEN 'owner'  THEN 0
        WHEN 'admin'  THEN 1
        ELSE 2
    END,
    users.name ASC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountWorkspaceMembers :one
SELECT count(*) FROM workspace_members WHERE workspace_id = $1;

-- name: UpdateWorkspaceMemberRole :one
UPDATE workspace_members
SET role = $3
WHERE workspace_id = $1 AND user_id = $2
RETURNING *;

-- name: RemoveWorkspaceMember :execrows
DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2;
