package logger

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		Log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("ip", r.RemoteAddr).
			Msg("incoming request")

		next.ServeHTTP(recorder, r)

		event := Log.Info().
			Int("status", recorder.statusCode).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("duration", time.Since(start))

		if message := recorder.message(); message != "" {
			event = event.Str("message", message)
		}

		event.Msg("request completed")
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.body.Len() < 4096 {
		remaining := 4096 - r.body.Len()
		if len(data) > remaining {
			r.body.Write(data[:remaining])
		} else {
			r.body.Write(data)
		}
	}
	return r.ResponseWriter.Write(data)
}

func (r *responseRecorder) message() string {
	if r.body.Len() == 0 {
		return http.StatusText(r.statusCode)
	}

	var payload map[string]any
	if err := json.Unmarshal(r.body.Bytes(), &payload); err != nil {
		return http.StatusText(r.statusCode)
	}

	if message, ok := payload["message"].(string); ok && message != "" {
		return message
	}
	if message, ok := payload["error"].(string); ok && message != "" {
		return message
	}

	return http.StatusText(r.statusCode)
}
