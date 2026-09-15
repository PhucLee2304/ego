package socket

import (
	"net/http/httptest"
	"testing"
)

func TestTokenFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		target     string
		want       string
	}{
		{
			name:       "bearer header",
			authHeader: "Bearer native-token",
			target:     "http://example.com/ws",
			want:       "native-token",
		},
		{
			name:   "web query fallback",
			target: "http://example.com/ws?access_token=web-token",
			want:   "web-token",
		},
		{
			name:       "header takes precedence",
			authHeader: "Bearer native-token",
			target:     "http://example.com/ws?access_token=web-token",
			want:       "native-token",
		},
		{
			name:   "query token is trimmed",
			target: "http://example.com/ws?access_token=%20web-token%20",
			want:   "web-token",
		},
		{
			name:   "missing token",
			target: "http://example.com/ws",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.target, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			if got := TokenFromRequest(req); got != tt.want {
				t.Fatalf("TokenFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}
