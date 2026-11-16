// Package httpauth provides static bearer-token authentication for the
// streamable HTTP transport.
package httpauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireBearer wraps next so that every request must carry
// "Authorization: Bearer <token>". Other requests get 401. token must be
// non-empty; RequireBearer panics otherwise so a misconfiguration can never
// expose next unauthenticated.
func RequireBearer(token string, next http.Handler) http.Handler {
	if token == "" {
		panic("httpauth: empty bearer token")
	}
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := bearerToken(r.Header.Get("Authorization"))
		// Compare fixed-length digests in constant time.
		gotSum := sha256.Sum256([]byte(got))
		if !ok || subtle.ConstantTimeCompare(gotSum[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="crazydomains-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(h string) (string, bool) {
	scheme, tok, ok := strings.Cut(strings.TrimSpace(h), " ")
	if !ok || !strings.EqualFold(scheme, "bearer") {
		return "", false
	}
	tok = strings.TrimSpace(tok)
	return tok, tok != ""
}
