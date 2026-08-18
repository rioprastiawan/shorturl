package analytics

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rioprastiawan/shorturl/apps/server/internal/cache"
	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

const (
	// maintenanceInterval spaces out the housekeeping pass. It shares the read
	// loop rather than running on its own goroutine so it can never overlap
	// with a batch write.
	maintenanceInterval = 5 * time.Minute

	// claimIdleAfter is how long an entry may sit unacknowledged in another
	// consumer's name before we assume that consumer died and take it over.
	claimIdleAfter = 5 * time.Minute

	// maxDeliveries bounds redelivery. An entry that has failed this many
	// times is not going to succeed — it references a deleted link, or trips a
	// constraint — and leaving it pending would stall the group forever, so it
	// is acknowledged and reported instead.
	maxDeliveries = 5

	// batchTimeout bounds the PostgreSQL work for one batch.
	batchTimeout = 30 * time.Second

	// Column widths from 000001_initial_schema. Values are clipped rather than
	// rejected: a truncated user agent is still useful, a failed batch is not.
	maxIPHash         = 64
	maxCountry        = 2
	maxDevice         = 16
	maxOSName         = 48
	maxBrowser        = 48
	maxReferrerHost   = 255
	maxReferrer       = 2048
	maxUTMValue       = 120
	maxDimensionValue = 255
)

// clickEventColumns matches the CopyFrom row order below. click_events.id is
// GENERATED ALWAYS AS IDENTITY and must not be supplied.
var clickEventColumns = []string{
	"link_id", "workspace_id", "clicked_at", "ip_hash", "country", "city",
	"device", "os", "browser", "referrer_host", "referrer",
	"utm_source", "utm_medium", "utm_campaign",
}

// Worker drains the click stream into PostgreSQL. Several instances can run
// concurrently: Redis consumer groups hand each entry to exactly one of them,
// and every worker takes a distinct consumer name so a crash is recoverable.
type Worker struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	rdb      *redis.Client
	cfg      config.Config
	logger   *slog.Logger
	consumer string
}

// NewWorker builds the consumer. Run does the work.
func NewWorker(pool *pgxpool.Pool, q *store.Queries, c *cache.Client, cfg config.Config, logger *slog.Logger) *Worker {
	return &Worker{
		pool:     pool,
		q:        q,
		rdb:      c.Redis(),
		cfg:      cfg,
		logger:   logger,
		consumer: consumerName(),
	}
}

// consumerName identifies this process within the group. Host and PID together
// stay stable across a reconnect but differ between containers, which is what
// lets a restarted worker reclaim the entries its predecessor left pending.
func consumerName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "worker"
	}
	return host + "-" + strconv.Itoa(os.Getpid())
}

// Run consumes the stream until ctx is cancelled, then finishes the batch in
// flight and returns nil. It returns an error only when it cannot start.
func (w *Worker) Run(ctx context.Context) error {
	// "0" so a group created after events already exist still sees them.
	if err := w.rdb.XGroupCreateMkStream(ctx, w.cfg.ClickStreamName, w.cfg.ClickConsumerGroup, "0").Err(); err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			return err
		}
	}

	w.logger.Info("analytics worker started",
		slog.String("stream", w.cfg.ClickStreamName),
		slog.String("group", w.cfg.ClickConsumerGroup),
		slog.String("consumer", w.consumer),
	)

	maintenance := time.NewTicker(maintenanceInterval)
	defer maintenance.Stop()

	// Consecutive read failures back off and stop repeating themselves in the
	// log. Redis being down is one condition, not one per second: without this
	// an overnight outage writes tens of thousands of identical ERROR lines and
	// buries whatever else went wrong.
	var readFailures int

	for {
		if ctx.Err() != nil {
			w.logger.Info("analytics worker stopped", slog.String("consumer", w.consumer))
			return nil
		}

		// Non-blocking: housekeeping runs between reads, never instead of them.
		select {
		case <-maintenance.C:
			w.maintain(ctx)
		default:
		}

		streams, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    w.cfg.ClickConsumerGroup,
			Consumer: w.consumer,
			Streams:  []string{w.cfg.ClickStreamName, ">"},
			Count:    int64(w.cfg.ClickBatchSize),
			Block:    w.cfg.ClickBlockDuration,
		}).Result()
		switch {
		case err == nil:
			if readFailures > 0 {
				w.logger.Info("click stream reachable again",
					slog.Int("failed_attempts", readFailures))
				readFailures = 0
			}
		case errors.Is(err, redis.Nil):
			// Block elapsed with nothing to read.
			readFailures = 0
			continue
		case ctx.Err() != nil:
			w.logger.Info("analytics worker stopped", slog.String("consumer", w.consumer))
			return nil
		default:
			readFailures++
			delay := readBackoff(readFailures)
			// Log the first few, then only on each backoff step, so a long
			// outage leaves a readable trail instead of a flood.
			if readFailures <= 3 || readFailures%10 == 0 {
				w.logger.Error("reading click stream",
					slog.String("error", err.Error()),
					slog.Int("consecutive_failures", readFailures),
					slog.Duration("retry_in", delay),
				)
			}
			w.pause(ctx, delay)
			continue
		}

		for _, stream := range streams {
			w.processBatch(ctx, stream.Messages)
		}
	}
}

// processBatch writes one batch and acknowledges it only once the transaction
// has committed. A failure leaves the entries pending so they are redelivered;
// at-least-once delivery is the right trade here, because a rare double count
// is cheaper than a lost day of analytics.
func (w *Worker) processBatch(ctx context.Context, messages []redis.XMessage) {
	if len(messages) == 0 {
		return
	}

	// The batch outlives cancellation on purpose: shutdown finishes the work in
	// flight rather than abandoning entries mid-transaction.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), batchTimeout)
	defer cancel()

	events := make([]enriched, 0, len(messages))
	ids := make([]string, 0, len(messages))

	for _, msg := range messages {
		ids = append(ids, msg.ID)
		ev, err := ParseEvent(msg.Values)
		if err != nil {
			// Unparseable entries are acknowledged with the batch: retrying
			// them can only fail again.
			w.logger.Error("discarding malformed click event",
				slog.String("id", msg.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		events = append(events, enrich(ev))
	}

	if len(events) > 0 {
		if err := w.write(ctx, events); err != nil {
			w.logger.Error("writing click batch",
				slog.Int("events", len(events)),
				slog.String("error", err.Error()),
			)
			return // no ack: Redis redelivers via XAutoClaim
		}
	}

	if err := w.rdb.XAck(ctx, w.cfg.ClickStreamName, w.cfg.ClickConsumerGroup, ids...).Err(); err != nil {
		// The data is committed; a failed ack only costs a redelivery.
		w.logger.Warn("acknowledging click batch", slog.String("error", err.Error()))
	}
}

// write persists one batch: raw rows in bulk, then the pre-aggregated rollups,
// all in a single transaction so the raw log and the counters cannot diverge.
func (w *Worker) write(ctx context.Context, events []enriched) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows := make([][]any, 0, len(events))
	for _, ev := range events {
		rows = append(rows, []any{
			ev.LinkID,
			ev.WorkspaceID,
			ev.ClickedAt.UTC(),
			nullable(ev.IPHash, maxIPHash),
			nullable(ev.Country, maxCountry),
			nil, // city: reserved for GeoIP enrichment
			nullable(ev.Device, maxDevice),
			nullable(ev.OS, maxOSName),
			nullable(ev.Browser, maxBrowser),
			nullable(ev.ReferrerHost, maxReferrerHost),
			nullable(ev.Referrer, maxReferrer),
			nullable(ev.UTMSource, maxUTMValue),
			nullable(ev.UTMMedium, maxUTMValue),
			nullable(ev.UTMCampaign, maxUTMValue),
		})
	}

	// COPY rather than N inserts: the raw log is the only per-event write left
	// in the pipeline, so it is the one worth making cheap.
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"click_events"}, clickEventColumns, pgx.CopyFromRows(rows)); err != nil {
		return err
	}

	qtx := w.q.WithTx(tx)
	rollup := aggregate(events)

	for _, bucket := range rollup.Hourly {
		if err := qtx.UpsertClickHourly(ctx, store.UpsertClickHourlyParams{
			WorkspaceID: bucket.WorkspaceID,
			LinkID:      bucket.LinkID,
			Bucket:      bucket.Bucket,
			Clicks:      bucket.Clicks,
		}); err != nil {
			return err
		}
	}

	for _, bucket := range rollup.Dimensions {
		if err := qtx.UpsertClickDimensionDaily(ctx, store.UpsertClickDimensionDailyParams{
			WorkspaceID: bucket.WorkspaceID,
			Day:         bucket.Day,
			Dimension:   bucket.Dimension,
			Value:       bucket.Value,
			Clicks:      bucket.Clicks,
		}); err != nil {
			return err
		}
	}

	// links.click_count is the denormalised total the link list renders; the
	// rollup tables remain the source of truth for reporting.
	for _, link := range rollup.Links {
		if err := qtx.IncrementLinkClickCount(ctx, store.IncrementLinkClickCountParams{
			Delta: link.Clicks,
			ID:    link.LinkID,
		}); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// maintain runs the periodic housekeeping: rescue entries stranded by a dead
// worker, retire the ones that will never succeed, and expire old rows.
func (w *Worker) maintain(ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), batchTimeout)
	defer cancel()

	w.retirePoisonEntries(ctx)
	w.claimStrandedEntries(ctx)
	w.expireRows(ctx)
	w.cleanupDeletions(ctx)
}

// retirePoisonEntries acknowledges entries that have been redelivered too many
// times. Without this one bad entry blocks its slot in the pending list
// forever, and every maintenance pass re-attempts the same doomed batch.
func (w *Worker) retirePoisonEntries(ctx context.Context) {
	pending, err := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.cfg.ClickStreamName,
		Group:  w.cfg.ClickConsumerGroup,
		Idle:   claimIdleAfter,
		Start:  "-",
		End:    "+",
		Count:  int64(w.cfg.ClickBatchSize),
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		w.logger.Error("inspecting pending click entries", slog.String("error", err.Error()))
		return
	}

	var poison []string
	for _, entry := range pending {
		if entry.RetryCount > maxDeliveries {
			poison = append(poison, entry.ID)
			w.logger.Error("discarding poison click event",
				slog.String("id", entry.ID),
				slog.Int64("deliveries", entry.RetryCount),
				slog.String("last_consumer", entry.Consumer),
			)
		}
	}
	if len(poison) == 0 {
		return
	}
	if err := w.rdb.XAck(ctx, w.cfg.ClickStreamName, w.cfg.ClickConsumerGroup, poison...).Err(); err != nil {
		w.logger.Error("acknowledging poison click events", slog.String("error", err.Error()))
	}
}

// claimStrandedEntries takes over entries left pending by a consumer that went
// away, and processes them here.
func (w *Worker) claimStrandedEntries(ctx context.Context) {
	messages, _, err := w.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   w.cfg.ClickStreamName,
		Group:    w.cfg.ClickConsumerGroup,
		MinIdle:  claimIdleAfter,
		Start:    "0-0",
		Count:    int64(w.cfg.ClickBatchSize),
		Consumer: w.consumer,
	}).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			w.logger.Error("claiming idle click entries", slog.String("error", err.Error()))
		}
		return
	}
	if len(messages) == 0 {
		return
	}

	w.logger.Info("reclaimed idle click entries", slog.Int("count", len(messages)))
	w.processBatch(ctx, messages)
}

// expireRows drops the rows that have outlived their purpose. Raw click events
// age out on the configured retention; the rollups are kept indefinitely
// because they are tiny.
func (w *Worker) expireRows(ctx context.Context) {
	if n, err := w.q.DeleteExpiredSessions(ctx); err != nil {
		w.logger.Error("deleting expired sessions", slog.String("error", err.Error()))
	} else if n > 0 {
		w.logger.Info("deleted expired sessions", slog.Int64("rows", n))
	}

	if n, err := w.q.DeleteExpiredIdempotencyRecords(ctx); err != nil {
		w.logger.Error("deleting expired idempotency records", slog.String("error", err.Error()))
	} else if n > 0 {
		w.logger.Info("deleted expired idempotency records", slog.Int64("rows", n))
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -w.cfg.ClickRetentionDays)
	if n, err := w.q.DeleteClickEventsBefore(ctx, cutoff); err != nil {
		w.logger.Error("deleting expired click events", slog.String("error", err.Error()))
	} else if n > 0 {
		w.logger.Info("deleted expired click events",
			slog.Int64("rows", n),
			slog.Time("before", cutoff),
		)
	}
	if n, err := w.q.DeleteVisitorDimensionsBefore(ctx, truncateDay(cutoff)); err != nil {
		w.logger.Error("deleting expired visitor rollups", slog.String("error", err.Error()))
	} else if n > 0 {
		w.logger.Info("deleted expired visitor rollups", slog.Int64("rows", n))
	}
}

// pause sleeps unless the worker is shutting down.
func (w *Worker) pause(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// maxReadBackoff caps the retry delay. Half a minute is long enough to keep an
// outage quiet, short enough that recovery is noticed promptly — click events
// survive in the stream meanwhile, so there is nothing to lose by waiting.
const maxReadBackoff = 30 * time.Second

// readBackoff doubles the delay per consecutive failure: 1s, 2s, 4s ... 30s.
func readBackoff(failures int) time.Duration {
	if failures < 1 {
		return time.Second
	}
	// Cap the shift before it overflows the duration.
	if failures > 6 {
		return maxReadBackoff
	}
	d := time.Second << (failures - 1)
	if d > maxReadBackoff {
		return maxReadBackoff
	}
	return d
}

// nullable clips a value to its column width and maps "" to SQL NULL, so an
// absent referrer is absent rather than an empty string.
func nullable(s string, max int) any {
	if s == "" {
		return nil
	}
	return clip(s, max)
}

// clip truncates by rune, matching how PostgreSQL counts VARCHAR length.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
