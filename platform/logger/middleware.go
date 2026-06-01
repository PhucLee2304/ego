package logger

import (
	"net/http"
	"time"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		Log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("ip", r.RemoteAddr).
			Msg("incoming request")

		next.ServeHTTP(w, r)

		Log.Info().
			Dur("duration", time.Since(start)).
			Str("path", r.URL.Path).
			Msg("request completed")
	})
}
