DROP TABLE IF EXISTS deletion_jobs;
ALTER TABLE workspaces DROP COLUMN IF EXISTS deletion_requested_at;
