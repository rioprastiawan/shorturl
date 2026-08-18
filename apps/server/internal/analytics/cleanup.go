package analytics

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const deletionBatchSize = 50_000

type deletionJob struct {
	id           int64
	resourceType string
	resourceID   uuid.UUID
	workspaceID  uuid.UUID
}

// cleanupDeletions spends the maintenance time budget draining durable delete
// jobs. Every transaction removes at most deletionBatchSize rows, keeping row
// locks, WAL spikes, and replica lag bounded even for very old workspaces.
func (w *Worker) cleanupDeletions(ctx context.Context) {
	for ctx.Err() == nil {
		done, err := w.cleanupDeletionBatch(ctx)
		if err != nil {
			w.logger.Error("cleaning asynchronously deleted resource", slog.String("error", err.Error()))
			return
		}
		if done {
			return
		}
	}
}

// cleanupDeletionBatch returns true when there are currently no ready jobs.
func (w *Worker) cleanupDeletionBatch(ctx context.Context) (bool, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var job deletionJob
	err = tx.QueryRow(ctx, `
		SELECT id, resource_type, resource_id, workspace_id
		FROM deletion_jobs
		WHERE not_before <= now()
		ORDER BY id
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&job.id, &job.resourceType, &job.resourceID, &job.workspaceID)
	if err == pgx.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	deleted, err := deleteRawEvents(ctx, tx, job)
	if err != nil {
		return false, err
	}
	if deleted == 0 {
		deleted, err = deleteHourlyRows(ctx, tx, job)
	}
	if err == nil && deleted == 0 && job.resourceType == "workspace" {
		deleted, err = deleteWorkspaceDimensions(ctx, tx, job.workspaceID)
	}
	if err == nil && deleted == 0 && job.resourceType == "workspace" {
		deleted, err = deleteWorkspaceLinks(ctx, tx, job.workspaceID)
	}
	if err != nil {
		return false, err
	}

	if deleted == 0 {
		switch job.resourceType {
		case "link":
			_, err = tx.Exec(ctx, `DELETE FROM links WHERE id = $1 AND workspace_id = $2`, job.resourceID, job.workspaceID)
		case "workspace":
			_, err = tx.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, job.resourceID)
		default:
			err = fmt.Errorf("unknown deletion resource type %q", job.resourceType)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `DELETE FROM deletion_jobs WHERE id = $1`, job.id)
		}
	}
	if err != nil {
		return false, err
	}
	return false, tx.Commit(ctx)
}

func deleteRawEvents(ctx context.Context, tx pgx.Tx, job deletionJob) (int64, error) {
	column, id := "workspace_id", job.workspaceID
	if job.resourceType == "link" {
		column, id = "link_id", job.resourceID
	}
	return deleteLimited(ctx, tx, "click_events", column, id)
}

func deleteHourlyRows(ctx context.Context, tx pgx.Tx, job deletionJob) (int64, error) {
	column, id := "workspace_id", job.workspaceID
	if job.resourceType == "link" {
		column, id = "link_id", job.resourceID
	}
	return deleteLimited(ctx, tx, "click_hourly", column, id)
}

func deleteWorkspaceDimensions(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (int64, error) {
	return deleteLimited(ctx, tx, "click_dimension_daily", "workspace_id", workspaceID)
}

func deleteWorkspaceLinks(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (int64, error) {
	return deleteLimited(ctx, tx, "links", "workspace_id", workspaceID)
}

func deleteLimited(ctx context.Context, tx pgx.Tx, table, column string, id uuid.UUID) (int64, error) {
	// table and column only come from the fixed callers above, never user input.
	query := fmt.Sprintf(`DELETE FROM %s WHERE ctid IN (
		SELECT ctid FROM %s WHERE %s = $1 LIMIT %d
	)`, table, table, column, deletionBatchSize)
	tag, err := tx.Exec(ctx, query, id)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
