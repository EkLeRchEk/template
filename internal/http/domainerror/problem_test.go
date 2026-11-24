package domainerror

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	api "github.com/EkLeRchEk/template/internal/generated"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "trip not found",
			err:        domaintrip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "trip completed",
			err:        domaintrip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
		{
			name:       "driver busy",
			err:        domaintrip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
		},
		{
			name:       "wrapped error",
			err:        fmt.Errorf("finish: %w", domaintrip.ErrTripCompleted),
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
		{
			name:       "internal error",
			err:        errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			Write(recorder, tt.err)

			if recorder.Code != tt.wantStatus {
				t.Fatalf(
					"expected status %d, got %d",
					tt.wantStatus,
					recorder.Code,
				)
			}

			if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Fatalf("unexpected content type: %s", got)
			}

			var problem api.Problem

			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if problem.Code != tt.wantCode {
				t.Fatalf(
					"expected code %s, got %s",
					tt.wantCode,
					problem.Code,
				)
			}
		})
	}
}
