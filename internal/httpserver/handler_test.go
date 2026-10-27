package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
