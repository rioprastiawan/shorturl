CREATE TABLE workspace_invitations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    token_hash   TEXT        NOT NULL UNIQUE,
    role         VARCHAR(20) NOT NULL,
    created_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    accepted_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT workspace_invitations_role_valid CHECK (role IN ('admin', 'member'))
);

CREATE INDEX workspace_invitations_workspace_idx
    ON workspace_invitations (workspace_id, created_at DESC);
CREATE INDEX workspace_invitations_token_idx
    ON workspace_invitations (token_hash) WHERE accepted_at IS NULL;
