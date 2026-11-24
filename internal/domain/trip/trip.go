package trip

import "time"

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Trip struct {
	ID     string
	Status Status

	StartedAt  time.Time
	FinishedAt *time.Time
}

// Finish переводит активную поездку в завершённую.
func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}

	t.Status = StatusCompleted
	t.FinishedAt = &at

	return nil
}
