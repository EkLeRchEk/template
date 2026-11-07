package httpserver

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/EkLeRchEk/template/internal/generated"
)

func TestWriteProblem(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeProblem(
		recorder,
		http.StatusNotFound,
		"trip_not_found",
		"Trip not found",
	)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status 404, got %d",
			recorder.Code,
		)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/problem+json" {
		t.Fatalf("unexpected content type: %s", contentType)
	}

	var response struct {
		Code   string `json:"code"`
		Status int    `json:"status"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != "trip_not_found" {
		t.Errorf("unexpected error code: %s", response.Code)
	}

	if response.Status != 404 {
		t.Errorf("unexpected status: %d", response.Status)
	}
}

func TestValidCoordinates(t *testing.T) {
	tests := []struct {
		name      string
		latitude  float64
		longitude float64
		want      bool
	}{
		{
			name:      "valid coordinates",
			latitude:  59.9398,
			longitude: 30.3146,
			want:      true,
		},
		{
			name:      "minimum latitude",
			latitude:  -90,
			longitude: 0,
			want:      true,
		},
		{
			name:      "maximum latitude",
			latitude:  90,
			longitude: 0,
			want:      true,
		},
		{
			name:      "minimum longitude",
			latitude:  0,
			longitude: -180,
			want:      true,
		},
		{
			name:      "maximum longitude",
			latitude:  0,
			longitude: 180,
			want:      true,
		},
		{
			name:      "latitude too high",
			latitude:  91,
			longitude: 30,
			want:      false,
		},
		{
			name:      "latitude too low",
			latitude:  -91,
			longitude: 30,
			want:      false,
		},
		{
			name:      "longitude too high",
			latitude:  50,
			longitude: 181,
			want:      false,
		},
		{
			name:      "longitude too low",
			latitude:  50,
			longitude: -181,
			want:      false,
		},
		{
			name:      "NaN latitude",
			latitude:  math.NaN(),
			longitude: 30,
			want:      false,
		},
		{
			name:      "infinite longitude",
			latitude:  50,
			longitude: math.Inf(1),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validCoordinates(tt.latitude, tt.longitude)

			if got != tt.want {
				t.Errorf("validCoordinates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateTripRejectsNegativePrice(t *testing.T) {
	handler := &Handler{}

	body := `{
		"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
		"driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5",
		"start_point": {
			"latitude": 59.9398,
			"longitude": 30.3146
		},
		"end_point": {
			"latitude": 59.929,
			"longitude": 30.3626
		},
		"price": -100
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	handler.CreateTrip(recorder, req, api.CreateTripParams{})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status 400, got %d",
			recorder.Code,
		)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/problem+json" {
		t.Fatalf("unexpected content type: %s", contentType)
	}
}

func TestCreateTripRejectsInvalidCoordinates(t *testing.T) {
	tests := []struct {
		name      string
		latitude  float64
		longitude float64
	}{
		{
			name:      "latitude too high",
			latitude:  91,
			longitude: 30,
		},
		{
			name:      "latitude too low",
			latitude:  -91,
			longitude: 30,
		},
		{
			name:      "longitude too high",
			latitude:  50,
			longitude: 181,
		},
		{
			name:      "longitude too low",
			latitude:  50,
			longitude: -181,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{}

			body := fmt.Sprintf(`{
				"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
				"driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5",
				"start_point": {
					"latitude": %f,
					"longitude": %f
				},
				"end_point": {
					"latitude": 59.929,
					"longitude": 30.3626
				},
				"price": 1450
			}`, tt.latitude, tt.longitude)

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/trips",
				strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()

			handler.CreateTrip(
				recorder,
				req,
				api.CreateTripParams{},
			)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status 400, got %d",
					recorder.Code,
				)
			}

			if contentType := recorder.Header().Get("Content-Type"); contentType != "application/problem+json" {
				t.Fatalf("unexpected content type: %s", contentType)
			}
		})
	}
}
