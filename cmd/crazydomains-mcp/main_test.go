package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodtune/crazydomains-mcp/internal/api"
	"github.com/goodtune/crazydomains-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testToken = "test-bearer-token-0123456789"

type headerTransport struct {
	token string
}

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+h.token)
	return http.DefaultTransport.RoundTrip(r)
}

func newTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":true,"data":{"balance":"10.00","currency":"AUD"}}`))
	}))
	t.Cleanup(upstream.Close)
	c, err := api.New(upstream.URL, "abcdefghijklmnopqrstuvwxyz012345")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newHTTPHandler(tools.NewServer(&tools.Deps{Client: c}, "test"), testToken))
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPRejectsMissingOrWrongToken(t *testing.T) {
	srv := newTestHTTPServer(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	for _, auth := range []string{"", "Bearer nope"} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("auth %q: status %d, want 401", auth, resp.StatusCode)
		}
	}
}

func TestHTTPWithToken(t *testing.T) {
	srv := newTestHTTPServer(t)
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             srv.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: headerTransport{token: testToken}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_account_balance", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %v", res.Content)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"10.00"`) {
		t.Errorf("content = %v", res.Content[0])
	}
}

func TestHealthz(t *testing.T) {
	srv := newTestHTTPServer(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status %d", resp.StatusCode)
	}
}
