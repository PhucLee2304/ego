package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ego/platform/logger"
	platformRpc "ego/platform/rpc"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type errorBody struct {
	Error string `json:"error"`
}

func TestErrorHidesInternalDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, NewError(http.StatusInternalServerError, "[ERROR] Database request failed", errors.New("password authentication failed for database")))

	var response errorBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "[ERROR] Internal server error" {
		t.Fatalf("expected a sanitized error, got %q", response.Error)
	}
}

func TestErrorPreservesClientMessage(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, NewError(http.StatusBadRequest, "[ERROR] Invalid request", nil))

	var response errorBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "[ERROR] Invalid request" {
		t.Fatalf("expected the client error to be preserved, got %q", response.Error)
	}
}

func decodeErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var response errorBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestNewErrorKeepsCauseInternal(t *testing.T) {
	cause := errors.New("file too large")
	httpErr := NewError(http.StatusBadRequest, "[ERROR] Invalid avatar", cause)

	if httpErr.StatusCode != http.StatusBadRequest || httpErr.Message != "[ERROR] Invalid avatar" {
		t.Fatalf("unexpected HTTPError: %+v", httpErr)
	}
	if !errors.Is(httpErr, cause) {
		t.Fatal("expected the original error to be available through errors.Is")
	}
	if httpErr.Error() != "[ERROR] Invalid avatar" {
		t.Fatalf("unexpected error string: %q", httpErr.Error())
	}
}

func TestErrorClientErrorExposesSpecificMessage(t *testing.T) {
	recorder := httptest.NewRecorder()
	const validationMessage = "T\u00ean kh\u00f4ng \u0111\u01b0\u1ee3c \u0111\u1ec3 tr\u1ed1ng"
	Error(recorder, NewError(http.StatusBadRequest, validationMessage, errors.New("validation failed on the required tag")))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	response := decodeErrorResponse(t, recorder)
	if response.Error != validationMessage {
		t.Fatalf("unexpected response: %+v", response)
	}
	if strings.Contains(recorder.Body.String(), "validation failed") {
		t.Fatalf("client response exposed the internal cause: %s", recorder.Body.String())
	}
}

func TestErrorServerErrorHidesCauseAndWritesTraceLog(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := logger.Log
	logger.Log = zerolog.New(&logs)
	defer func() { logger.Log = previousLogger }()

	recorder := httptest.NewRecorder()
	Error(recorder, NewError(http.StatusInternalServerError, "[ERROR] Failed to login", errors.New("password authentication failed for database")))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
	response := decodeErrorResponse(t, recorder)
	if response.Error != internalServerErrorMessage {
		t.Fatalf("expected a sanitized server error, got %+v", response)
	}
	if strings.Contains(recorder.Body.String(), "Failed to login") || strings.Contains(recorder.Body.String(), "password authentication failed") {
		t.Fatalf("client response exposed internal information: %s", recorder.Body.String())
	}
	if logOutput := logs.String(); !strings.Contains(logOutput, "Failed to login") || !strings.Contains(logOutput, "password authentication failed") {
		t.Fatalf("expected internal message and cause in log, got %s", logOutput)
	}
}

func TestErrorNilWritesInternalError(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, nil)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}

func TestNewErrorNormalizesInvalidStatus(t *testing.T) {
	httpErr := NewError(http.StatusOK, "not an error", nil)
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, httpErr.StatusCode)
	}
	if httpErr.Message != internalServerErrorMessage {
		t.Fatalf("unexpected public message: %q", httpErr.Message)
	}
}

func TestErrorNormalizesDirectInvalidHTTPError(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, &HTTPError{StatusCode: http.StatusOK, Message: "unsafe"})

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
	response := decodeErrorResponse(t, recorder)
	if response.Error != internalServerErrorMessage {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestJSONMarshalFailureReturnsJSONError(t *testing.T) {
	recorder := httptest.NewRecorder()
	JSON(recorder, http.StatusOK, make(chan int))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json, got %q", contentType)
	}
	response := decodeErrorResponse(t, recorder)
	if response.Error != internalServerErrorMessage {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestFromError(t *testing.T) {
	if FromError(nil, "[ERROR] Failed") != nil {
		t.Fatal("expected nil for a nil error")
	}

	forbidden := NewError(http.StatusForbidden, "[FORBIDDEN] Denied", nil)
	if got := FromError(fmt.Errorf("in transaction: %w", forbidden), "[ERROR] Failed"); got != forbidden {
		t.Fatalf("expected the wrapped HTTPError to be kept, got %+v", got)
	}

	notFound := FromError(fmt.Errorf("lookup: %w", gorm.ErrRecordNotFound), "[ERROR] Failed")
	if notFound.StatusCode != http.StatusNotFound || notFound.Message != notFoundMessage || !errors.Is(notFound, gorm.ErrRecordNotFound) {
		t.Fatalf("unexpected not-found error: %+v", notFound)
	}

	denied := FromError(platformRpc.PublicError(codes.PermissionDenied, "[FORBIDDEN] Not a member"), "[ERROR] Failed")
	if denied.StatusCode != http.StatusForbidden || denied.Message != "[FORBIDDEN] Not a member" {
		t.Fatalf("unexpected gRPC permission error: %+v", denied)
	}
	if unavailable := FromError(status.Error(codes.Unavailable, "dial tcp"), "[ERROR] Failed"); unavailable.StatusCode != http.StatusServiceUnavailable || strings.Contains(unavailable.Message, "dial tcp") {
		t.Fatalf("expected a sanitized 503 for an unavailable gRPC service, got %+v", unavailable)
	}
	if hidden := FromError(status.Error(codes.PermissionDenied, "[FORBIDDEN] internal detail"), "[ERROR] Failed"); hidden.StatusCode != http.StatusForbidden || strings.Contains(hidden.Message, "internal detail") {
		t.Fatalf("expected a gRPC message not marked public to be hidden, got %+v", hidden)
	}

	cause := errors.New("connection refused")
	internal := FromError(cause, "[ERROR] Failed")
	if internal.StatusCode != http.StatusInternalServerError || internal.Message != "[ERROR] Failed" || !errors.Is(internal, cause) {
		t.Fatalf("unexpected internal error: %+v", internal)
	}
}
