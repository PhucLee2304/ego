package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorHidesInternalDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, http.StatusInternalServerError, "password authentication failed for database")

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "[ERROR] Internal server error" {
		t.Fatalf("expected a sanitized error, got %q", response.Error)
	}
}

func TestErrorPreservesClientMessage(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, http.StatusBadRequest, "[ERROR] Invalid request")

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "[ERROR] Invalid request" {
		t.Fatalf("expected the client error to be preserved, got %q", response.Error)
	}
}
