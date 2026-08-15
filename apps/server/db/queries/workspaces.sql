-- name: CreateWorkspace :one
INSERT INTO workspaces (name, slug, owner_user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = $1;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspaces WHERE LOWER(slug) = LOWER(sqlc.arg(slug)::text);

-- name: ListWorkspacesForUser :many
SELECT
    sqlc.embed(workspaces),
    workspace_members.role
FROM workspaces
JOIN workspace_members ON workspace_members.workspace_id = workspaces.id
WHERE workspace_members.user_id = $1
ORDER BY workspaces.created_at ASC;

-- name: UpdateWorkspace :one
UPDATE workspaces SET name = $2 WHERE id = $1 RETURNING *;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces WHERE id = $1;

-- name: CountWorkspacesForUser :one
SELECT count(*) FROM workspace_members WHERE user_id = $1;


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
    users.email
FROM workspace_members
JOIN users ON users.id = workspace_members.user_id
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
