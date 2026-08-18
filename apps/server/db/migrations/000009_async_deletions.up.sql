-- Large cascades (especially links/workspaces with years of click_events) must
-- not run in an HTTP request. Resources are made unavailable immediately and
-- their dependent rows are removed in bounded batches by the worker.
ALTER TABLE workspaces ADD COLUMN deletion_requested_at TIMESTAMPTZ;

CREATE TABLE deletion_jobs (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    resource_type VARCHAR(16) NOT NULL,
    resource_id  UUID        NOT NULL,
    workspace_id UUID        NOT NULL,
    not_before   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error   TEXT,
    attempts     INTEGER     NOT NULL DEFAULT 0,

    CONSTRAINT deletion_jobs_resource_type_valid
        CHECK (resource_type IN ('link', 'workspace')),
    UNIQUE (resource_type, resource_id)
);

CREATE INDEX deletion_jobs_ready_idx ON deletion_jobs (not_before, id);
