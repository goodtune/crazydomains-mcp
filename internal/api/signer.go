package api

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

// Header names used by the Dreamscape reseller API for request signing.
const (
	HeaderRequestID = "Api-Request-Id"
	HeaderSignature = "Api-Signature"
)

// Sign returns hex(md5(requestID + apiKey)), the value of the Api-Signature
// header for a given request ID.
func Sign(requestID, apiKey string) string {
	sum := md5.Sum([]byte(requestID + apiKey))
	return hex.EncodeToString(sum[:])
}

// NewRequestID returns a fresh request ID: the hex MD5 of 16 random bytes.
func NewRequestID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate request id: %w", err)
	}
	sum := md5.Sum(b[:])
	return hex.EncodeToString(sum[:]), nil
}

// Signer is an http.RoundTripper that adds the Api-Request-Id and
// Api-Signature headers to every request. It never logs the key or the
// signature.
type Signer struct {
	// APIKey is the reseller API key.
	APIKey string
	// Base is the underlying transport; http.DefaultTransport if nil.
	Base http.RoundTripper
	// RequestID generates request IDs; NewRequestID if nil. Tests may fix it.
	RequestID func() (string, error)
}

// RoundTrip implements http.RoundTripper.
func (s *Signer) RoundTrip(req *http.Request) (*http.Response, error) {
	gen := s.RequestID
	if gen == nil {
		gen = NewRequestID
	}
	id, err := gen()
	if err != nil {
		return nil, err
	}
	// RoundTrippers must not modify the caller's request.
	r := req.Clone(req.Context())
	r.Header.Set(HeaderRequestID, id)
	r.Header.Set(HeaderSignature, Sign(id, s.APIKey))
	base := s.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}
