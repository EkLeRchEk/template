package model

import (
	"time"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
)

// Trip — модель поездки, передаваемая между слоями приложения.
type Trip struct {
	ID string

	UserID   string
	DriverID string

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price  int64
	Status domaintrip.Status

	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// StatusHistory — история изменения состояния поездки.
type StatusHistory struct {
	ID int64

	TripID string

	FromStatus *domaintrip.Status
	ToStatus   domaintrip.Status
	Reason     *string

	ChangedAt time.Time
}

// ToDomain преобразует модель поездки в доменный объект.
func (t Trip) ToDomain() domaintrip.Trip {
	return domaintrip.Trip{
		ID:         t.ID,
		Status:     t.Status,
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}
