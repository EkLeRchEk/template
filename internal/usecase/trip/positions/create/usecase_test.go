package create

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EkLeRchEk/template/internal/model"
)

type testRepository struct {
	err error
}

func (r *testRepository) SavePosition(
	_ context.Context,
	position model.TripPosition,
) (model.TripPosition, error) {
	if r.err != nil {
		return model.TripPosition{}, r.err
	}

	return position, nil
}

type testTransaction struct {
	committed bool
	err       error
}

func (tx *testTransaction) Do(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	if tx.err != nil {
		return tx.err
	}

	if err := fn(ctx); err != nil {
		return err
	}

	tx.committed = true
	return nil
}

type testCanceller struct {
	tx          *testTransaction
	calls       []string
	afterCommit bool
}

func (c *testCanceller) Cancel(tripID string) {
	c.calls = append(c.calls, tripID)
	c.afterCommit = c.tx.committed
}

func TestExecuteCancelsRequestAfterCommit(t *testing.T) {
	tx := &testTransaction{}
	canceller := &testCanceller{tx: tx}

	uc := New(
		&testRepository{},
		tx,
		canceller,
	)

	input := Input{
		TripID:     "trip-1",
		Latitude:   55.7558,
		Longitude:  37.6173,
		RecordedAt: time.Now(),
	}

	result, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TripID != input.TripID {
		t.Fatalf("unexpected trip ID: %s", result.TripID)
	}

	if len(canceller.calls) != 1 {
		t.Fatalf("expected 1 cancellation, got %d", len(canceller.calls))
	}

	if canceller.calls[0] != input.TripID {
		t.Fatalf("unexpected cancelled trip: %s", canceller.calls[0])
	}

	if !canceller.afterCommit {
		t.Fatal("request cancelled before transaction commit")
	}
}

func TestExecuteDoesNotCancelWhenSaveFails(t *testing.T) {
	tx := &testTransaction{}
	canceller := &testCanceller{tx: tx}

	uc := New(
		&testRepository{err: errors.New("database error")},
		tx,
		canceller,
	)

	_, err := uc.Execute(context.Background(), Input{
		TripID: "trip-1",
	})

	if err == nil {
		t.Fatal("expected error")
	}

	if len(canceller.calls) != 0 {
		t.Fatal("request was cancelled despite failed save")
	}
}
