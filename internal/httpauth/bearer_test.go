package httpauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireBearer(t *testing.T) {
	h := RequireBearer("s3cret-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	cases := []struct {
		header string
		want   int
	}{
		{"", http.StatusUnauthorized},
		{"Bearer", http.StatusUnauthorized},
		{"Bearer ", http.StatusUnauthorized},
		{"Bearer wrong", http.StatusUnauthorized},
		{"Basic s3cret-token", http.StatusUnauthorized},
		{"Bearer s3cret-token-extra", http.StatusUnauthorized},
		{"Bearer s3cret-token", http.StatusTeapot},
		{"bearer s3cret-token", http.StatusTeapot},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%q: status %d, want %d", tc.header, rec.Code, tc.want)
		}
		if rec.Code == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%q: missing WWW-Authenticate", tc.header)
		}
	}
}

func TestRequireBearerEmptyTokenPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	RequireBearer("", http.NotFoundHandler())
}
