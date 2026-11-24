package positions

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/EkLeRchEk/template/internal/model"
)

const enqueueResultQueued = "queued"

// Repository предоставляет данные об устаревших поездках.
type Repository interface {
	ListStaleTrips(
		ctx context.Context,
		staleAfter time.Duration,
	) ([]model.StaleTrip, error)
}

// PushClient отправляет запросы на получение координат.
type PushClient interface {
	SendPositionRequest(
		ctx context.Context,
		driverID string,
		tripID string,
	) error

	Close()
}

// Config содержит настройки сканера и пула.
type Config struct {
	CheckInterval      time.Duration
	StaleAfter         time.Duration
	Workers            int
	QueueSize          int
	PushRequestTimeout time.Duration
}

// Ticker позволяет подменять время в тестах.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// TickerFactory создаёт управляемый тикер.
type TickerFactory func(time.Duration) Ticker

type realTicker struct {
	ticker *time.Ticker
}

func (t realTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t realTicker) Stop() {
	t.ticker.Stop()
}

type Option func(*Worker)

// WithTickerFactory позволяет передать тестовый тикер.
func WithTickerFactory(factory TickerFactory) Option {
	return func(w *Worker) {
		w.newTicker = factory
	}
}

type task struct {
	trip     model.StaleTrip
	token    uint64
	scanSpan trace.SpanContext
}

type inFlight struct {
	token  uint64
	cancel context.CancelFunc
}

type metrics struct {
	staleChecks   metric.Int64Counter
	queueSize     metric.Int64UpDownCounter
	activeWorkers metric.Int64UpDownCounter
	requests      metric.Int64Counter
	duration      metric.Float64Histogram
}

func newMetrics() metrics {
	meter := otel.Meter("trip-service")

	staleChecks, _ := meter.Int64Counter(
		"tripgo.stale_checks",
		metric.WithDescription("Results of stale trip scanning"),
	)

	queueSize, _ := meter.Int64UpDownCounter(
		"tripgo.position_requests.queue.size",
		metric.WithDescription("Number of pending position requests"),
	)

	activeWorkers, _ := meter.Int64UpDownCounter(
		"tripgo.position_requests.workers.active",
		metric.WithDescription("Number of active position workers"),
	)

	requests, _ := meter.Int64Counter(
		"tripgo.position_requests",
		metric.WithDescription("Position requests sent to Push Service"),
	)

	duration, _ := meter.Float64Histogram(
		"tripgo.position_requests.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Position request processing duration"),
	)

	return metrics{
		staleChecks:   staleChecks,
		queueSize:     queueSize,
		activeWorkers: activeWorkers,
		requests:      requests,
		duration:      duration,
	}
}

type Worker struct {
	repo   Repository
	push   PushClient
	logger *slog.Logger
	config Config

	queue chan task

	mu        sync.Mutex
	queued    map[string]uint64
	inFlight  map[string]inFlight
	nextToken uint64
	stopping  bool
	started   bool

	newTicker TickerFactory

	scannerCancel context.CancelFunc
	scannerDone   chan struct{}

	workerCtx    context.Context
	workerCancel context.CancelFunc
	workers      sync.WaitGroup
	workersDone  chan struct{}

	stopOnce  sync.Once
	closeOnce sync.Once

	metrics metrics
}

func New(
	repo Repository,
	push PushClient,
	cfg Config,
	logger *slog.Logger,
	options ...Option,
) *Worker {
	if logger == nil {
		logger = slog.Default()
	}

	w := &Worker{
		repo:     repo,
		push:     push,
		logger:   logger,
		config:   cfg,
		queue:    make(chan task, cfg.QueueSize),
		queued:   make(map[string]uint64),
		inFlight: make(map[string]inFlight),
		newTicker: func(interval time.Duration) Ticker {
			return realTicker{
				ticker: time.NewTicker(interval),
			}
		},
		metrics: newMetrics(),
	}

	for _, option := range options {
		option(w)
	}

	return w
}

// Start запускает фиксированное количество воркеров и сканер.
func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()

	if w.started || w.stopping {
		w.mu.Unlock()
		return
	}

	w.started = true
	w.scannerDone = make(chan struct{})
	w.workersDone = make(chan struct{})

	w.workerCtx, w.workerCancel = context.WithCancel(
		context.WithoutCancel(ctx),
	)

	scannerCtx, scannerCancel := context.WithCancel(ctx)
	w.scannerCancel = scannerCancel

	for i := 0; i < w.config.Workers; i++ {
		w.workers.Add(1)
		go w.runWorker(w.workerCtx)
	}

	go func() {
		w.workers.Wait()
		close(w.workersDone)
	}()

	go w.runScanner(scannerCtx)

	w.mu.Unlock()

	w.logger.Info(
		"position worker pool started",
		"workers", w.config.Workers,
		"queue_size", w.config.QueueSize,
	)
}

// Stop сначала останавливает сканер, затем закрывает очередь
// и ожидает завершения воркеров.
func (w *Worker) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	w.stopOnce.Do(func() {
		w.mu.Lock()
		w.stopping = true

		started := w.started
		scannerCancel := w.scannerCancel
		scannerDone := w.scannerDone

		w.mu.Unlock()

		if scannerCancel != nil {
			scannerCancel()
		}

		if started {
			<-scannerDone
		}

		close(w.queue)
	})

	w.mu.Lock()
	started := w.started
	workersDone := w.workersDone
	workerCancel := w.workerCancel
	w.mu.Unlock()

	if !started {
		w.closePush()
		return nil
	}

	select {
	case <-workersDone:
		w.closePush()
		return nil

	case <-ctx.Done():
		// Прерываем оставшиеся HTTP-запросы,
		// если общий таймаут завершения исчерпан.
		workerCancel()
		<-workersDone

		w.closePush()
		return ctx.Err()
	}
}

func (w *Worker) closePush() {
	w.closeOnce.Do(func() {
		w.push.Close()
	})
}

// Cancel отменяет ожидающий запрос по поездке.
// Задание, уже помещённое в канал, будет пропущено воркером.
func (w *Worker) Cancel(tripID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	token, exists := w.queued[tripID]
	if !exists {
		return
	}

	delete(w.queued, tripID)

	if current, ok := w.inFlight[tripID]; ok &&
		current.token == token {
		current.cancel()
		delete(w.inFlight, tripID)
	}
}

func (w *Worker) runScanner(ctx context.Context) {
	defer close(w.scannerDone)

	ticker := w.newTicker(w.config.CheckInterval)
	defer ticker.Stop()

	w.logger.Info(
		"position scanner started",
		"interval", w.config.CheckInterval,
		"stale_after", w.config.StaleAfter,
	)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("position scanner stopped")
			return

		case <-ticker.C():
			w.scan(ctx)
		}
	}
}

// scan выполняет один последовательный обход.
// Следующий обход не запускается, пока текущий не завершён.
func (w *Worker) scan(ctx context.Context) {
	ctx, span := otel.Tracer("trip-service/worker").Start(
		ctx,
		"Positions.Scan",
	)
	defer span.End()

	trips, err := w.repo.ListStaleTrips(
		ctx,
		w.config.StaleAfter,
	)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			span.RecordError(err)

			w.metrics.staleChecks.Add(
				context.Background(),
				1,
				metric.WithAttributes(
					attribute.String("result", "error"),
				),
			)

			w.logger.ErrorContext(
				ctx,
				"failed to scan stale trips",
				"error", err,
			)
		}

		return
	}

	for _, trip := range trips {
		if ctx.Err() != nil {
			return
		}

		result := w.enqueue(task{
			trip:     trip,
			scanSpan: span.SpanContext(),
		})

		if result == "" {
			continue
		}

		w.metrics.staleChecks.Add(
			ctx,
			1,
			metric.WithAttributes(
				attribute.String("result", result),
			),
		)

		if result == "dropped" {
			w.logger.WarnContext(
				ctx,
				"position request dropped: queue is full",
				"trip_id", trip.ID,
				"stale_since", trip.StaleSince,
			)
		}
	}
}

// enqueue никогда не блокируется на заполненной очереди.
func (w *Worker) enqueue(t task) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopping {
		return ""
	}

	if _, exists := w.queued[t.trip.ID]; exists {
		return "duplicate"
	}

	w.nextToken++
	t.token = w.nextToken

	w.queued[t.trip.ID] = t.token

	select {
	case w.queue <- t:
		w.metrics.queueSize.Add(
			context.Background(),
			1,
		)

		return enqueueResultQueued

	default:
		delete(w.queued, t.trip.ID)
		return "dropped"
	}
}

func (w *Worker) runWorker(ctx context.Context) {
	defer w.workers.Done()

	w.metrics.activeWorkers.Add(
		ctx,
		1,
	)
	defer w.metrics.activeWorkers.Add(
		ctx,
		-1,
	)

	for t := range w.queue {
		w.metrics.queueSize.Add(
			ctx,
			-1,
		)

		if ctx.Err() != nil {
			return
		}

		w.process(t)
	}
}

func (w *Worker) process(t task) {
	w.mu.Lock()

	token, exists := w.queued[t.trip.ID]
	if !exists || token != t.token {
		w.mu.Unlock()
		return
	}

	ctx, cancel := context.WithTimeout(
		w.workerCtx,
		w.config.PushRequestTimeout,
	)

	w.inFlight[t.trip.ID] = inFlight{
		token:  t.token,
		cancel: cancel,
	}

	w.mu.Unlock()

	defer func() {
		cancel()

		w.mu.Lock()
		defer w.mu.Unlock()

		if current, ok := w.queued[t.trip.ID]; ok &&
			current == t.token {
			delete(w.queued, t.trip.ID)
		}

		if current, ok := w.inFlight[t.trip.ID]; ok &&
			current.token == t.token {
			delete(w.inFlight, t.trip.ID)
		}
	}()

	var options []trace.SpanStartOption

	if t.scanSpan.IsValid() {
		options = append(
			options,
			trace.WithLinks(trace.Link{
				SpanContext: t.scanSpan,
			}),
		)
	}

	ctx, span := otel.Tracer("trip-service/worker").Start(
		ctx,
		"Positions.Process",
		options...,
	)
	defer span.End()

	started := time.Now()

	err := w.push.SendPositionRequest(
		ctx,
		t.trip.DriverID,
		t.trip.ID,
	)

	duration := time.Since(started).Seconds()

	w.metrics.duration.Record(
		context.Background(),
		duration,
	)

	result := "success"

	if err != nil {
		result = "error"

		if errors.Is(err, context.Canceled) &&
			w.workerCtx.Err() == nil {
			result = "cancelled"
		}

		span.RecordError(err)

		w.logger.WarnContext(
			ctx,
			"position request failed",
			"trip_id", t.trip.ID,
			"driver_id", t.trip.DriverID,
			"error", err,
		)
	} else {
		w.logger.DebugContext(
			ctx,
			"position request sent",
			"trip_id", t.trip.ID,
		)
	}

	w.metrics.requests.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("result", result),
		),
	)
}
