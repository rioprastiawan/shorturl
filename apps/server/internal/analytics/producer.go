package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rioprastiawan/shorturl/apps/server/internal/cache"
	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
)

const (
	// queueCapacity buffers roughly a second of peak redirect traffic. Beyond
	// that Redis is either down or hopelessly behind, and the right answer is
	// to shed events rather than grow memory without bound.
	queueCapacity = 4096

	// publisherCount is enough concurrency to keep the queue drained without
	// consuming a meaningful share of the Redis connection pool.
	publisherCount = 4

	// xaddTimeout bounds a single XADD. The redirect has already been served by
	// the time this runs, so the only purpose is to free the goroutine when
	// Redis stops responding.
	xaddTimeout = 2 * time.Second

	// logInterval throttles the drop and failure warnings. A Redis outage
	// affects every redirect; without this the log becomes the bottleneck.
	logInterval = 30 * time.Second
)

// Producer publishes click events to a Redis stream without ever blocking the
// caller. Enqueue hands the event to a buffered channel and returns; a small
// pool of goroutines performs the XADD.
//
// It returns no error by design: an analytics event is worth strictly less
// than redirect latency, so a full buffer or a dead Redis drops the event and
// increments a counter instead of propagating a failure into the hot path.
type Producer struct {
	rdb    *redis.Client
	stream string
	maxLen int64
	logger *slog.Logger

	queue chan queued
	quit  chan struct{}
	stop  sync.Once
	wg    sync.WaitGroup

	dropped     atomic.Int64
	failed      atomic.Int64
	lastDropLog atomic.Int64
	lastFailLog atomic.Int64
}

// queued carries the caller's context alongside the event so request-scoped
// logging values survive into the publisher goroutine.
type queued struct {
	ctx context.Context
	ev  ClickEvent
}

// NewProducer starts the publisher goroutines. Call Close to drain them.
func NewProducer(c *cache.Client, cfg config.Config, logger *slog.Logger) *Producer {
	p := &Producer{
		rdb:    c.Redis(),
		stream: cfg.ClickStreamName,
		maxLen: cfg.ClickStreamMaxLen,
		logger: logger,
		queue:  make(chan queued, queueCapacity),
		quit:   make(chan struct{}),
	}

	p.wg.Add(publisherCount)
	for range publisherCount {
		go p.publishLoop()
	}
	return p
}

// Enqueue submits a click event. It never blocks, never fails, and never
// touches the network on the calling goroutine.
//
// The caller's context is detached with context.WithoutCancel: an HTTP
// request's context is cancelled the instant the redirect response is written,
// which would otherwise abort the XADD we are about to perform.
func (p *Producer) Enqueue(ctx context.Context, ev ClickEvent) {
	select {
	case p.queue <- queued{ctx: context.WithoutCancel(ctx), ev: ev}:
	default:
		p.noteDrop()
	}
}

// Close stops accepting work and waits for the buffered events to reach Redis,
// giving up when ctx expires so shutdown stays bounded.
func (p *Producer) Close(ctx context.Context) error {
	p.stop.Do(func() { close(p.quit) })

	drained := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("analytics producer: %d events still queued: %w", len(p.queue), ctx.Err())
	}
}

// Dropped reports how many events were discarded because the buffer was full.
func (p *Producer) Dropped() int64 { return p.dropped.Load() }

// Failed reports how many events reached a publisher but not Redis.
func (p *Producer) Failed() int64 { return p.failed.Load() }

func (p *Producer) publishLoop() {
	defer p.wg.Done()
	for {
		select {
		case item := <-p.queue:
			p.publish(item)
		case <-p.quit:
			// Shutting down: flush whatever is already buffered, then stop.
			// The queue is never closed, so a late Enqueue is simply dropped
			// rather than panicking on a send to a closed channel.
			for {
				select {
				case item := <-p.queue:
					p.publish(item)
				default:
					return
				}
			}
		}
	}
}

func (p *Producer) publish(item queued) {
	ctx, cancel := context.WithTimeout(item.ctx, xaddTimeout)
	defer cancel()

	err := p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		// Approximate trimming is bounded work per XADD; an exact MAXLEN would
		// make the write O(entries removed). The cap is what stops a stalled
		// worker from letting the stream exhaust Redis memory.
		MaxLen: p.maxLen,
		Approx: true,
		Values: item.ev.Fields(),
	}).Err()
	if err != nil {
		p.noteFailure(ctx, err)
	}
}

func (p *Producer) noteDrop() {
	total := p.dropped.Add(1)
	if !throttle(&p.lastDropLog) {
		return
	}
	p.logger.Warn("analytics events dropped: publish queue is full",
		slog.Int64("dropped_total", total),
		slog.Int("queue_capacity", queueCapacity),
	)
}

func (p *Producer) noteFailure(ctx context.Context, err error) {
	total := p.failed.Add(1)
	if !throttle(&p.lastFailLog) {
		return
	}
	p.logger.WarnContext(ctx, "analytics event not published",
		slog.Int64("failed_total", total),
		slog.String("error", err.Error()),
	)
}

// throttle reports whether enough time has passed since the last log, updating
// the timestamp when it wins the race. Losers stay silent.
func throttle(last *atomic.Int64) bool {
	now := time.Now().UnixNano()
	previous := last.Load()
	if now-previous < int64(logInterval) {
		return false
	}
	return last.CompareAndSwap(previous, now)
}
