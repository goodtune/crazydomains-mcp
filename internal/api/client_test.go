package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, testAPIKey, WithRetryBackoff(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

func TestGetDomainStripsAuthKey(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/42" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"status":true,"data":{"id":42,"domain_name":"example.com.au","auth_key":"EPP-SECRET","status_id":2,
			"name_servers":[{"host":"ns1.digitalocean.com","ip":""}],"nested":{"auth_key":"EPP-SECRET"}}}`))
	})
	d, err := c.GetDomain(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 42 || d.DomainName != "example.com.au" || len(d.NameServers) != 1 {
		t.Fatalf("unexpected domain: %+v", d)
	}
}

func TestRedactJSON(t *testing.T) {
	out, err := redactJSON([]byte(`{"a":1.50,"auth_key":"x","l":[{"auth_key":"y","b":12345678901234567890}]}`))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "auth_key") {
		t.Fatalf("auth_key not stripped: %s", s)
	}
	if !strings.Contains(s, "1.50") || !strings.Contains(s, "12345678901234567890") {
		t.Fatalf("numbers not preserved: %s", s)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{401, `{"message":"Invalid signature"}`, "authentication failed (check key / sandbox vs production): Invalid signature"},
		{404, `{"status":false,"message":"Domain not found"}`, "not found: Domain not found"},
		{402, `{"error":"Insufficient funds"}`, "insufficient reseller balance: Insufficient funds"},
		{400, `{"errors":{"domain_name":["is required"]}}`, "upstream error: HTTP 400: domain_name: is required"},
		{403, `plain text body`, "upstream error: HTTP 403: plain text body"},
		{400, `{"errors":"bad","auth_key":"EPP-SECRET"}`, "upstream error: HTTP 400: bad"},
		{418, `{"weird":"shape","auth_key":"EPP-SECRET"}`, `upstream error: HTTP 418: {"weird":"shape"}`},
	}
	for _, tc := range cases {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			w.Write([]byte(tc.body))
		})
		_, err := c.GetBalance(context.Background())
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.status {
			t.Fatalf("status %d: err = %v", tc.status, err)
		}
		if err.Error() != tc.want {
			t.Errorf("status %d: got %q, want %q", tc.status, err.Error(), tc.want)
		}
	}
}

func TestEnvelopeStatusFalse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":false,"message":"nope"}`))
	})
	_, err := c.GetBalance(context.Background())
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
}

func TestRetryOnce5xx(t *testing.T) {
	var calls atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte(`{"status":true,"data":{"balance":"12.50","currency":"AUD"}}`))
	})
	b, err := c.GetBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if b.Balance != "12.50" || calls.Load() != 2 {
		t.Fatalf("balance=%v calls=%d", b, calls.Load())
	}
}

func TestRetryGivesUpAfterOne(t *testing.T) {
	var calls atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(503)
	})
	_, err := c.GetBalance(context.Background())
	if err == nil || calls.Load() != 2 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}

func TestNoRetryOn4xx(t *testing.T) {
	for _, status := range []int{400, 401, 402, 403, 404} {
		var calls atomic.Int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(status)
		})
		if _, err := c.GetBalance(context.Background()); err == nil {
			t.Fatalf("%d: expected error", status)
		}
		if calls.Load() != 1 {
			t.Fatalf("%d: calls = %d, want 1", status, calls.Load())
		}
	}
}

func TestRetryOnNetworkError(t *testing.T) {
	var calls atomic.Int32
	c, err := New("http://example.invalid", testAPIKey,
		WithRetryBackoff(time.Millisecond),
		WithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if calls.Add(1) == 1 {
				return nil, errors.New("connection reset")
			}
			return &http.Response{
				StatusCode: 200,
				Body:       readCloser(`{"status":true,"data":{"balance":1,"currency":"AUD"}}`),
				Header:     http.Header{},
			}, nil
		})))
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.GetBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if b.Balance != "1" || calls.Load() != 2 {
		t.Fatalf("balance=%v calls=%d", b, calls.Load())
	}
}

func TestContextCancellation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.GetBalance(ctx); err == nil {
		t.Fatal("expected error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("did not respect context cancellation")
	}
}

func TestQueryEncoding(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q["tlds[]"]; len(got) != 2 || got[0] != "com.au" || got[1] != "com" {
			t.Errorf("tlds[] = %v", got)
		}
		if q.Get("domain_name") != "mets" || q.Get("customer_id") != "7" || q.Get("page") != "2" || q.Get("limit") != "10" {
			t.Errorf("query = %v", q)
		}
		w.Write([]byte(`{"status":true,"data":[],"pagination":{"total_items":0,"total_pages":0,"current_page":2}}`))
	})
	_, pg, err := c.ListDomains(context.Background(), ListDomainsParams{
		DomainName: "mets", CustomerID: 7, TLDs: []string{"com.au", "com"}, Page: 2, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pg == nil || pg.CurrentPage != 2 {
		t.Fatalf("pagination = %+v", pg)
	}
}

func TestDecimal(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":true,"data":[{"tld":"com.au","price":{"register":"12.95","renew":13.5,"transfer":null}}]}`))
	})
	tlds, _, err := c.ListTLDs(context.Background(), ListTLDsParams{})
	if err != nil {
		t.Fatal(err)
	}
	p := tlds[0].Price
	if p.Register != "12.95" || p.Renew != "13.5" || p.Transfer != "" {
		t.Fatalf("price = %+v", p)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type stringReadCloser struct{ *strings.Reader }

func (stringReadCloser) Close() error { return nil }

func readCloser(s string) stringReadCloser { return stringReadCloser{strings.NewReader(s)} }
