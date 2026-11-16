// Package api is a small, hand-written client for the Dreamscape (Crazy
// Domains) reseller REST API. It covers only the read-only endpoints needed by
// the phase 1 MCP tools. Request signing lives in Signer, redaction of
// sensitive fields (auth_key) in redact.go, and retry/error handling here.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Base URLs for the two Dreamscape environments.
const (
	ProductionBaseURL = "https://reseller-api.ds.network"
	SandboxBaseURL    = "https://reseller-api.sandbox.ds.network"
)

// MaxPageSize is the largest page size the API accepts.
const MaxPageSize = 100

const (
	defaultTimeout      = 20 * time.Second
	defaultRetryBackoff = 500 * time.Millisecond
	maxErrorBodyLen     = 300
	maxResponseBytes    = 16 << 20
)

// Client talks to the Dreamscape reseller API.
type Client struct {
	baseURL      *url.URL
	hc           *http.Client
	retryBackoff time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithRetryBackoff sets the delay before the single retry (default 500ms).
func WithRetryBackoff(d time.Duration) Option {
	return func(c *Client) { c.retryBackoff = d }
}

// WithTransport sets the base transport underneath the signer (tests).
func WithTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.hc.Transport.(*Signer).Base = rt }
}

// WithRequestIDFunc overrides request ID generation (tests).
func WithRequestIDFunc(f func() (string, error)) Option {
	return func(c *Client) { c.hc.Transport.(*Signer).RequestID = f }
}

// New returns a client for baseURL authenticated with apiKey.
func New(baseURL, apiKey string, opts ...Option) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid base URL %q: scheme must be http or https", baseURL)
	}
	c := &Client{
		baseURL: u,
		hc: &http.Client{
			Timeout:   defaultTimeout,
			Transport: &Signer{APIKey: apiKey},
		},
		retryBackoff: defaultRetryBackoff,
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// Pagination is the pagination block returned by list endpoints.
type Pagination struct {
	TotalItems  int `json:"total_items"`
	TotalPages  int `json:"total_pages"`
	CurrentPage int `json:"current_page"`
}

type envelope struct {
	Status     *bool           `json:"status"`
	Data       json.RawMessage `json:"data"`
	Pagination *Pagination     `json:"pagination"`
	Message    json.RawMessage `json:"message"`
	Error      json.RawMessage `json:"error"`
	Errors     json.RawMessage `json:"errors"`
}

// Error is returned for non-2xx responses (and 2xx responses whose envelope
// reports status=false).
type Error struct {
	StatusCode int
	Message    string
}

func (e *Error) Error() string {
	var s string
	switch e.StatusCode {
	case http.StatusUnauthorized:
		s = "authentication failed (check key / sandbox vs production)"
	case http.StatusNotFound:
		s = "not found"
	case http.StatusPaymentRequired:
		s = "insufficient reseller balance"
	default:
		s = fmt.Sprintf("upstream error: HTTP %d", e.StatusCode)
	}
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s
}

// IsNotFound reports whether err is an upstream 404.
func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.StatusCode == http.StatusNotFound
}

// get performs a GET and decodes the envelope's data into out.
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) (*Pagination, error) {
	return c.do(ctx, http.MethodGet, path, q, nil, out)
}

// do performs a request with one retry on network errors and 5xx for
// idempotent methods, strips auth_key from the response and decodes the
// envelope's data into out. body (if non-nil) is JSON-encoded.
func (c *Client) do(ctx context.Context, method, path string, q url.Values, body any, out any) (*Pagination, error) {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}
	u := *c.baseURL
	u.Path = c.baseURL.Path + path
	u.RawQuery = q.Encode()

	retryable := method == http.MethodGet || method == http.MethodHead
	var (
		status int
		raw    []byte
		err    error
	)
	for attempt := 0; ; attempt++ {
		status, raw, err = c.once(ctx, method, u.String(), payload)
		shouldRetry := err != nil || status >= 500
		if !retryable || attempt >= 1 || !shouldRetry || ctx.Err() != nil {
			break
		}
		t := time.NewTimer(c.retryBackoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
	if err != nil {
		return nil, err
	}

	redacted, jsonErr := redactJSON(raw)
	if status < 200 || status > 299 {
		return nil, &Error{StatusCode: status, Message: errorMessage(raw, redacted, jsonErr)}
	}
	if status == http.StatusNoContent || len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	if jsonErr != nil {
		return nil, fmt.Errorf("decode response: %w", jsonErr)
	}
	var env envelope
	if err := json.Unmarshal(redacted, &env); err != nil {
		return nil, fmt.Errorf("decode response envelope: %w", err)
	}
	if env.Status != nil && !*env.Status {
		return nil, &Error{StatusCode: status, Message: errorMessage(raw, redacted, nil)}
	}
	if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return nil, fmt.Errorf("decode response data: %w", err)
		}
	}
	return env.Pagination, nil
}

func (c *Client) once(ctx context.Context, method, u string, payload []byte) (int, []byte, error) {
	var rd io.Reader
	if payload != nil {
		rd = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", scrubURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, raw, nil
}

// scrubURLError drops the URL from *url.Error so that query strings never end
// up in tool output; the method and underlying cause are kept.
func scrubURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s: %w", ue.Op, ue.Err)
	}
	return err
}

// errorMessage extracts a human-readable message from an error body:
// message/error/errors from JSON if present, else the (truncated) raw body.
func errorMessage(raw, redacted []byte, jsonErr error) string {
	if jsonErr == nil && len(redacted) > 0 {
		var env envelope
		if json.Unmarshal(redacted, &env) == nil {
			var parts []string
			for _, f := range []json.RawMessage{env.Message, env.Error, env.Errors} {
				if s := flatten(f); s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				return truncate(strings.Join(parts, "; "))
			}
		}
		return truncate(string(redacted))
	}
	return truncate(strings.TrimSpace(string(raw)))
}

// flatten renders a JSON value as text: strings as-is, arrays joined, objects
// as "key: value" pairs, anything else as compact JSON.
func flatten(m json.RawMessage) string {
	if len(m) == 0 || string(m) == "null" {
		return ""
	}
	var v any
	if err := json.Unmarshal(m, &v); err != nil {
		return ""
	}
	return flattenValue(v)
}

func flattenValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var parts []string
		for _, e := range t {
			if s := flattenValue(e); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		var parts []string
		for _, k := range sortedKeys(t) {
			if s := flattenValue(t[k]); s != "" {
				parts = append(parts, k+": "+s)
			}
		}
		return strings.Join(parts, "; ")
	case nil:
		return ""
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxErrorBodyLen {
		return s[:maxErrorBodyLen] + "…"
	}
	return s
}

func pageQuery(q url.Values, page, limit int) url.Values {
	if q == nil {
		q = url.Values{}
	}
	if page > 0 {
		q.Set("page", fmt.Sprint(page))
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprint(limit))
	}
	return q
}
