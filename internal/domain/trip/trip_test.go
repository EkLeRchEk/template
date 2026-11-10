package trip

import (
	"errors"
	"testing"
	"time"
)

func TestTripFinish(t *testing.T) {
	t.Run("active trip", func(t *testing.T) {
		tr := Trip{Status: StatusActive}
		now := time.Now().UTC()

		if err := tr.Finish(now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tr.Status != StatusCompleted {
			t.Fatalf("expected completed, got %s", tr.Status)
		}

		if tr.FinishedAt == nil || !tr.FinishedAt.Equal(now) {
			t.Fatal("incorrect finish time")
		}
	})

	t.Run("already completed", func(t *testing.T) {
		tr := Trip{Status: StatusCompleted}

		err := tr.Finish(time.Now())

		if !errors.Is(err, ErrTripCompleted) {
			t.Fatalf("expected ErrTripCompleted, got %v", err)
		}
	})
}
