package trip

import "time"

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Trip struct {
	ID string

	UserID   string
	DriverID string

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price  int64
	Status Status

	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type StatusHistory struct {
	ID int64

	TripID string

	FromStatus *Status
	ToStatus   Status
	Reason     *string

	ChangedAt time.Time
}
