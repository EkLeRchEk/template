package positions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/EkLeRchEk/template/internal/model"
)

// Проверяем отсутствие утечек горутин после выполнения тестов.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type lifecycleRepository struct{}

func (lifecycleRepository) ListStaleTrips(
	context.Context,
	time.Duration,
) ([]model.StaleTrip, error) {
	return nil, nil
}

func newLifecycleWorker(
	push PushClient,
	workers int,
	queueSize int,
) *Worker {
	return New(
		lifecycleRepository{},
		push,
		Config{
			CheckInterval:      time.Hour,
			StaleAfter:         time.Minute,
			Workers:            workers,
			QueueSize:          queueSize,
			PushRequestTimeout: 5 * time.Second,
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func lifecycleTrip(id string) model.StaleTrip {
	return model.StaleTrip{
		ID:         id,
		DriverID:   "driver-" + id,
		StaleSince: time.Now().Add(-time.Minute),
	}
}

func waitStarted(t *testing.T, ch <-chan string) string {
	t.Helper()

	select {
	case id := <-ch:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start processing task")
		return ""
	}
}

func stopLifecycleWorker(t *testing.T, worker *Worker) {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	defer cancel()

	if err := worker.Stop(ctx); err != nil {
		t.Fatalf("stop worker: %v", err)
	}
}

// blockingPush удерживает запросы, позволяя проверить
// фактическое количество одновременно работающих воркеров.
type blockingPush struct {
	started chan string
	release chan struct{}

	mu        sync.Mutex
	active    int
	maxActive int
}

func (p *blockingPush) SendPositionRequest(
	ctx context.Context,
	_ string,
	tripID string,
) error {
	p.mu.Lock()
	p.active++

	if p.active > p.maxActive {
		p.maxActive = p.active
	}

	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
	}()

	p.started <- tripID

	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *blockingPush) Close() {}

// Проверяем, что пул использует ровно заданное количество воркеров.
func TestWorkerPoolUsesConfiguredNumberOfWorkers(t *testing.T) {
	push := &blockingPush{
		started: make(chan string, 5),
		release: make(chan struct{}),
	}

	worker := newLifecycleWorker(push, 2, 5)

	var releaseOnce sync.Once
	releaseAll := func() {
		releaseOnce.Do(func() {
			close(push.release)
		})
	}

	defer func() {
		releaseAll()
		stopLifecycleWorker(t, worker)
	}()

	for i := 1; i <= 5; i++ {
		if got := worker.enqueue(task{
			trip: lifecycleTrip(
				"trip-" + string(rune('0'+i)),
			),
		}); got != enqueueResultQueued {
			t.Fatalf("enqueue result = %q, want queued", got)
		}
	}

	worker.Start(context.Background())

	first := waitStarted(t, push.started)
	second := waitStarted(t, push.started)

	if first == second {
		t.Fatal("two workers started the same task")
	}

	// Пока первые два запроса заблокированы,
	// третий воркер не должен появиться.
	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()

	select {
	case id := <-push.started:
		t.Fatalf("unexpected third concurrent worker: %s", id)
	case <-timer.C:
	}

	releaseAll()

	seen := map[string]bool{
		first:  true,
		second: true,
	}

	for len(seen) < 5 {
		id := waitStarted(t, push.started)
		seen[id] = true
	}

	stopLifecycleWorker(t, worker)

	push.mu.Lock()
	maxActive := push.maxActive
	push.mu.Unlock()

	if maxActive != 2 {
		t.Fatalf("max concurrent workers = %d, want 2", maxActive)
	}

	if len(seen) != 5 {
		t.Fatalf("processed %d unique tasks, want 5", len(seen))
	}
}

// Первая ошибка Push Service не должна останавливать воркер.
type errorThenSuccessPush struct {
	calls     chan string
	completed chan string
	count     atomic.Int32
}

func (p *errorThenSuccessPush) SendPositionRequest(
	ctx context.Context,
	_ string,
	tripID string,
) error {
	select {
	case p.calls <- tripID:
	case <-ctx.Done():
		return ctx.Err()
	}

	if p.count.Add(1) == 1 {
		return errors.New("simulated Push Service failure")
	}

	select {
	case p.completed <- tripID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *errorThenSuccessPush) Close() {}

func TestWorkerContinuesAfterPushError(t *testing.T) {
	push := &errorThenSuccessPush{
		calls:     make(chan string, 2),
		completed: make(chan string, 1),
	}

	worker := newLifecycleWorker(push, 1, 2)
	defer stopLifecycleWorker(t, worker)

	for _, id := range []string{testTripID, testTripID2} {
		if got := worker.enqueue(task{
			trip: lifecycleTrip(id),
		}); got != enqueueResultQueued {
			t.Fatalf("enqueue result = %q, want queued", got)
		}
	}

	worker.Start(context.Background())

	select {
	case id := <-push.completed:
		if id != testTripID2 {
			t.Fatalf("completed task = %q, want trip-2", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not process task after Push Service error")
	}

	stopLifecycleWorker(t, worker)

	first := <-push.calls
	second := <-push.calls

	if first != testTripID || second != testTripID2 {
		t.Fatalf("calls = %q, %q; want trip-1, trip-2", first, second)
	}
}

// Проверяем обработку уже поставленных задач при штатной остановке.
type recordingPush struct {
	completed chan string
}

func (p *recordingPush) SendPositionRequest(
	ctx context.Context,
	_ string,
	tripID string,
) error {
	select {
	case p.completed <- tripID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *recordingPush) Close() {}

func TestWorkerStopDrainsQueuedTasks(t *testing.T) {
	push := &recordingPush{
		completed: make(chan string, 3),
	}

	worker := newLifecycleWorker(push, 2, 3)

	for i := 1; i <= 3; i++ {
		if got := worker.enqueue(task{
			trip: lifecycleTrip(
				"trip-" + string(rune('0'+i)),
			),
		}); got != enqueueResultQueued {
			t.Fatalf("enqueue result = %q, want queued", got)
		}
	}

	worker.Start(context.Background())

	// Stop должен дождаться обработки всех задач.
	stopLifecycleWorker(t, worker)

	processed := make(map[string]bool)

	for i := 0; i < 3; i++ {
		select {
		case id := <-push.completed:
			processed[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d tasks were processed", len(processed))
		}
	}

	if len(processed) != 3 {
		t.Fatalf("processed %d unique tasks, want 3", len(processed))
	}
}
