package api

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

const (
	testRequestID = "0123456789abcdef0123456789abcdef"
	testAPIKey    = "abcdefghijklmnopqrstuvwxyz012345"
	// Precomputed with: printf '%s' "$ID$KEY" | md5sum
	testSignature = "4ebe75a3dfd259ab3ffdcef81d26df3b"
)

func TestSign(t *testing.T) {
	// Independent computation of md5(requestID + apiKey).
	want := fmt.Sprintf("%x", md5.Sum([]byte(testRequestID+testAPIKey)))
	if want != testSignature {
		t.Fatalf("test fixture mismatch: %s != %s", want, testSignature)
	}
	if got := Sign(testRequestID, testAPIKey); got != testSignature {
		t.Fatalf("Sign = %q, want %q", got, testSignature)
	}
}

func TestSignerSetsHeaders(t *testing.T) {
	var gotID, gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("Api-Request-Id")
		gotSig = r.Header.Get("Api-Signature")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := &Signer{APIKey: testAPIKey, RequestID: func() (string, error) { return testRequestID, nil }}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := s.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if gotID != testRequestID {
		t.Errorf("Api-Request-Id = %q, want %q", gotID, testRequestID)
	}
	if gotSig != testSignature {
		t.Errorf("Api-Signature = %q, want %q", gotSig, testSignature)
	}
	if req.Header.Get("Api-Signature") != "" {
		t.Error("signer mutated the caller's request")
	}
}

func TestNewRequestIDUnique(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{32}$`)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := NewRequestID()
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(id) {
			t.Fatalf("request id %q is not hex md5", id)
		}
		if seen[id] {
			t.Fatalf("duplicate request id %q", id)
		}
		seen[id] = true
	}
}
