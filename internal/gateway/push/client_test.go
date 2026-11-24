package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendPositionRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("unexpected method: %s", r.Method)
			}

			if r.URL.Path != "/api/v1/push" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			var request pushRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}

			if request.RecipientID != "driver-id" {
				t.Errorf("unexpected recipient: %s", request.RecipientID)
			}

			if request.Kind != "REQUEST_POSITION" {
				t.Errorf("unexpected kind: %s", request.Kind)
			}

			if request.Data["trip_id"] != "trip-id" {
				t.Errorf("unexpected trip ID: %s", request.Data["trip_id"])
			}

			if request.RequestID == "" {
				t.Error("request_id is empty")
			}

			w.WriteHeader(http.StatusAccepted)
		},
	))
	defer server.Close()

	client, err := New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.SendPositionRequest(
		context.Background(),
		"driver-id",
		"trip-id",
	); err != nil {
		t.Fatal(err)
	}
}

func TestSendPositionRequestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		},
	))
	defer server.Close()

	client, err := New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	err = client.SendPositionRequest(
		context.Background(),
		"driver-id",
		"trip-id",
	)

	if err == nil {
		t.Fatal("expected error")
	}

	statusErr, ok := err.(*HTTPStatusError)
	if !ok {
		t.Fatalf("expected HTTPStatusError, got %T", err)
	}

	if statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("unexpected status: %d", statusErr.StatusCode)
	}
}
