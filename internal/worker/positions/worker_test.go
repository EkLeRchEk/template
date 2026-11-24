package positions

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/EkLeRchEk/template/internal/model"
)

const (
	testTripID  = "trip-1"
	testTripID2 = "trip-2"
)

type testRepository struct{}

func (testRepository) ListStaleTrips(
	context.Context,
	time.Duration,
) ([]model.StaleTrip, error) {
	return nil, nil
}

type testPush struct {
	started   chan struct{}
	completed chan string
	cancelled chan struct{}
}

func (p *testPush) SendPositionRequest(
	ctx context.Context,
	_ string,
	tripID string,
) error {
	if p.started != nil {
		select {
		case p.started <- struct{}{}:
		default:
		}
	}

	select {
	case p.completed <- tripID:
		return nil
	case <-ctx.Done():
		if p.cancelled != nil {
			close(p.cancelled)
		}
		return ctx.Err()
	}
}

func (p *testPush) Close() {}

func newTestWorker(push *testPush, queueSize int) *Worker {
	return New(
		testRepository{},
		push,
		Config{
			CheckInterval:      time.Hour,
			StaleAfter:         time.Minute,
			Workers:            2,
			QueueSize:          queueSize,
			PushRequestTimeout: 5 * time.Second,
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func testTrip(id string) model.StaleTrip {
	return model.StaleTrip{
		ID:         id,
		DriverID:   "driver-" + id,
		StaleSince: time.Now().Add(-time.Minute),
	}
}

func stopTestWorker(t *testing.T, worker *Worker) {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)
	defer cancel()

	if err := worker.Stop(ctx); err != nil {
		t.Fatalf("stop worker: %v", err)
	}
}

func TestEnqueueDeduplicatesAndDropsWhenQueueFull(t *testing.T) {
	push := &testPush{}
	worker := newTestWorker(push, 1)
	defer stopTestWorker(t, worker)

	first := task{trip: testTrip(testTripID)}
	second := task{trip: testTrip("trip-2")}

	if got := worker.enqueue(first); got != enqueueResultQueued {
		t.Fatalf("first enqueue = %q, want queued", got)
	}

	if got := worker.enqueue(first); got != "duplicate" {
		t.Fatalf("duplicate enqueue = %q, want duplicate", got)
	}

	if got := worker.enqueue(second); got != "dropped" {
		t.Fatalf("enqueue into full queue = %q, want dropped", got)
	}

	if got := len(worker.queue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}
}

func TestCancelledQueuedTaskIsNotSent(t *testing.T) {
	push := &testPush{
		started:   make(chan struct{}, 1),
		completed: make(chan string, 1),
	}

	worker := newTestWorker(push, 10)

	if got := worker.enqueue(task{trip: testTrip(testTripID)}); got != enqueueResultQueued {
		t.Fatalf("enqueue = %q, want queued", got)
	}

	worker.Cancel(testTripID)
	worker.Start(context.Background())

	stopTestWorker(t, worker)

	select {
	case <-push.started:
		t.Fatal("cancelled task was sent to Push Service")
	default:
	}
}

func TestWorkerSendsQueuedTask(t *testing.T) {
	push := &testPush{
		started:   make(chan struct{}, 1),
		completed: make(chan string, 1),
	}

	worker := newTestWorker(push, 10)
	worker.Start(context.Background())

	if got := worker.enqueue(task{trip: testTrip(testTripID)}); got != enqueueResultQueued {
		t.Fatalf("enqueue = %q, want queued", got)
	}

	select {
	case tripID := <-push.completed:
		if tripID != testTripID {
			t.Fatalf("sent trip = %q, want trip-1", tripID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not send position request")
	}

	stopTestWorker(t, worker)
}

func TestCancelInterruptsInFlightRequest(t *testing.T) {
	push := &testPush{
		started:   make(chan struct{}, 1),
		cancelled: make(chan struct{}),
	}

	worker := newTestWorker(push, 10)
	worker.Start(context.Background())

	if got := worker.enqueue(task{trip: testTrip(testTripID)}); got != enqueueResultQueued {
		t.Fatalf("enqueue = %q, want queued", got)
	}

	select {
	case <-push.started:
	case <-time.After(2 * time.Second):
		t.Fatal("Push request did not start")
	}

	worker.Cancel(testTripID)

	select {
	case <-push.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight Push request was not cancelled")
	}

	stopTestWorker(t, worker)
}
