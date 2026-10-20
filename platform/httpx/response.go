package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ego/platform/logger"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const (
	internalServerErrorMessage = "[ERROR] Internal server error"
	notFoundMessage            = "[ERROR] Resource not found"
)

var internalServerErrorBody, _ = json.Marshal(map[string]string{"error": internalServerErrorMessage})

func isErrorStatus(code int) bool {
	return code >= http.StatusBadRequest && code <= 599
}

// HTTPError carries the public client message and retains the underlying cause
// for inspection and server-side logging only.
type HTTPError struct {
	StatusCode int    `json:"-"`
	Message    string `json:"error"`
	cause      error
}

// NewError creates an HTTP error and retains err for internal logging and error
// inspection. Internal errors are never copied into the public response body.
func NewError(statusCode int, message string, err error) *HTTPError {
	if !isErrorStatus(statusCode) {
		invalidStatusErr := fmt.Errorf("invalid HTTP error status code: %d", statusCode)
		if err != nil {
			invalidStatusErr = fmt.Errorf("%w: %w", invalidStatusErr, err)
		}
		statusCode = http.StatusInternalServerError
		message = internalServerErrorMessage
		err = invalidStatusErr
	}

	if strings.TrimSpace(message) == "" {
		if statusCode >= http.StatusInternalServerError {
			message = internalServerErrorMessage
		} else {
			message = http.StatusText(statusCode)
		}
	}

	httpErr := &HTTPError{
		StatusCode: statusCode,
		Message:    message,
		cause:      err,
	}
	return httpErr
}

func (e *HTTPError) Error() string {
	if e == nil {
		return internalServerErrorMessage
	}
	return e.Message
}

func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// FromError converts an error coming out of a transaction, repository, helper or
// gRPC call into an HTTPError: an HTTPError anywhere in the chain is kept as is,
// a missing record becomes 404, client-facing gRPC codes keep their meaning, and
// anything else becomes a 500 with message.
func FromError(err error, message string) *HTTPError {
	if err == nil {
		return nil
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NewError(http.StatusNotFound, notFoundMessage, err)
	}
	if grpcStatus, ok := status.FromError(err); ok {
		switch grpcStatus.Code() {
		case codes.PermissionDenied:
			return NewError(http.StatusForbidden, publicStatusMessage(grpcStatus), err)
		case codes.NotFound:
			return NewError(http.StatusNotFound, publicStatusMessage(grpcStatus), err)
		case codes.InvalidArgument, codes.FailedPrecondition:
			return NewError(http.StatusBadRequest, publicStatusMessage(grpcStatus), err)
		case codes.AlreadyExists:
			return NewError(http.StatusConflict, publicStatusMessage(grpcStatus), err)
		case codes.Unauthenticated:
			return NewError(http.StatusUnauthorized, publicStatusMessage(grpcStatus), err)
		case codes.Unavailable, codes.DeadlineExceeded:
			return NewError(http.StatusServiceUnavailable, "[UNAVAILABLE] Service temporarily unavailable", err)
		}
	}
	return NewError(http.StatusInternalServerError, message, err)
}

// publicStatusMessage forwards only a message the producer explicitly marked as
// public (rpc.PublicError); the plain status message may be internal error text.
func publicStatusMessage(grpcStatus *status.Status) string {
	for _, detail := range grpcStatus.Details() {
		if localized, ok := detail.(*errdetails.LocalizedMessage); ok {
			return localized.GetMessage()
		}
	}
	return ""
}

// JSON writes a success response.
func JSON(w http.ResponseWriter, code int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		logger.Log.Error().Err(err).Msg("[INTERNAL_SERVER_ERROR] Error marshalling response")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(internalServerErrorBody)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(data)
}

// Error writes an HTTPError. Client errors retain their specific message. For
// server errors, the internal message and cause are logged; a 500 response is
// replaced with a generic message, while other 5xx keep their public message.
func Error(w http.ResponseWriter, httpErr *HTTPError) {
	if httpErr == nil {
		httpErr = NewError(http.StatusInternalServerError, internalServerErrorMessage, nil)
	} else {
		httpErr = NewError(httpErr.StatusCode, httpErr.Message, httpErr.cause)
	}

	clientMessage := httpErr.Message
	if httpErr.StatusCode >= http.StatusInternalServerError {
		logEvent := logger.Log.Error().Int("code", httpErr.StatusCode).Str("internal_error", httpErr.Message)
		if cause := errors.Unwrap(httpErr); cause != nil {
			logEvent = logEvent.Err(cause)
		}
		logEvent.Msg("Internal server error")
		if httpErr.StatusCode == http.StatusInternalServerError {
			clientMessage = internalServerErrorMessage
		}
	} else if httpErr.cause != nil {
		logger.Log.Warn().Int("code", httpErr.StatusCode).Str("error", httpErr.Message).Err(httpErr.cause).Msg("Client error")
	}

	JSON(w, httpErr.StatusCode, &HTTPError{
		StatusCode: httpErr.StatusCode,
		Message:    clientMessage,
	})
}
