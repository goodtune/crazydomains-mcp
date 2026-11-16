package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goodtune/crazydomains-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testAPIKey  = "abcdefghijklmnopqrstuvwxyz012345"
	eppSecret   = "EPP-SECRET-7f3a9c"
	fixedNowStr = "2026-11-16T09:00:00+11:00"
)

var fixedNow, _ = time.Parse(time.RFC3339, fixedNowStr)

// upstream is a fake Dreamscape API. Handlers are keyed by URL path.
type upstream struct {
	t        *testing.T
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
	calls    map[string]int
	queries  map[string][]string
	// override, if set, handles every request before the path handlers;
	// returning true means it wrote a response.
	override func(w http.ResponseWriter, r *http.Request, call int) bool
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.mu.Lock()
	u.calls[r.URL.Path]++
	n := u.calls[r.URL.Path]
	u.queries[r.URL.Path] = append(u.queries[r.URL.Path], r.URL.RawQuery)
	h := u.handlers[r.URL.Path]
	ov := u.override
	u.mu.Unlock()

	if r.Header.Get("Api-Request-Id") == "" || r.Header.Get("Api-Signature") != api.Sign(r.Header.Get("Api-Request-Id"), testAPIKey) {
		u.t.Errorf("%s: missing or bad signature headers", r.URL.Path)
	}
	if ov != nil && ov(w, r, n) {
		return
	}
	if h == nil {
		u.t.Errorf("unexpected upstream request: %s?%s", r.URL.Path, r.URL.RawQuery)
		http.NotFound(w, r)
		return
	}
	h(w, r)
}

func (u *upstream) callCount(path string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls[path]
}

func (u *upstream) lastQuery(path string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	q := u.queries[path]
	if len(q) == 0 {
		return ""
	}
	return q[len(q)-1]
}

// setup starts a fake upstream and an MCP client session connected in-memory
// to a server with all tools registered.
func setup(t *testing.T, handlers map[string]http.HandlerFunc) (*mcp.ClientSession, *upstream) {
	t.Helper()
	u := &upstream{t: t, handlers: handlers, calls: map[string]int{}, queries: map[string][]string{}}
	srv := httptest.NewServer(u)
	t.Cleanup(srv.Close)

	c, err := api.New(srv.URL, testAPIKey, api.WithRetryBackoff(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(&Deps{Client: c, Now: func() time.Time { return fixedNow }}, "test")

	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, u
}

// callResult is a decoded tool call.
type callResult struct {
	res  *mcp.CallToolResult
	raw  string         // the whole result as JSON
	text string         // concatenated text content
	data map[string]any // structured content
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) callResult {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error: %v", name, err)
	}
	raw, _ := json.Marshal(res)
	var text []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text = append(text, tc.Text)
		}
	}
	cr := callResult{res: res, raw: string(raw), text: strings.Join(text, "\n")}
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		_ = json.Unmarshal(b, &cr.data)
	}
	assertNoSecrets(t, name, cr.raw)
	return cr
}

func mustOK(t *testing.T, name string, cr callResult) {
	t.Helper()
	if cr.res.IsError {
		t.Fatalf("%s: unexpected tool error: %s", name, cr.text)
	}
}

func mustErr(t *testing.T, name string, cr callResult, want string) {
	t.Helper()
	if !cr.res.IsError {
		t.Fatalf("%s: expected tool error containing %q, got success: %s", name, want, cr.text)
	}
	if !strings.Contains(cr.text, want) {
		t.Fatalf("%s: error %q does not contain %q", name, cr.text, want)
	}
}

// assertNoSecrets checks that neither the EPP code, the auth_key field name
// nor the API key appear anywhere in a tool result.
func assertNoSecrets(t *testing.T, name, raw string) {
	t.Helper()
	for _, s := range []string{eppSecret, "auth_key", testAPIKey} {
		if strings.Contains(raw, s) {
			t.Errorf("%s: output contains %q: %s", name, s, raw)
		}
	}
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}
}

func get(m map[string]any, path ...string) any {
	var v any = m
	for _, p := range path {
		mm, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = mm[p]
	}
	return v
}

// Fixtures. Every domain carries an auth_key to exercise redaction.

func domainJSON(id int, name, expiry string) string {
	b, _ := json.Marshal(map[string]any{
		"id": id, "domain_name": name, "auth_key": eppSecret, "status_id": 2, "period": 2,
		"start_date": "2024-01-10T00:00:00+00:00", "expiry_date": expiry,
		"customer_id": 501, "registrant_id": 9001, "admin_contact_id": 11, "billing_contact_id": 12, "tech_contact_id": 13,
		"name_servers": []map[string]string{
			{"host": "ns1.digitalocean.com", "ip": ""}, {"host": "ns2.digitalocean.com", "ip": ""}, {"host": "ns3.digitalocean.com", "ip": ""},
		},
		"eligibility_data": []map[string]string{
			{"name": "Registrant Name", "value": "Sydney Mets Touch Association Incorporated"},
			{"name": "Registrant ID Type", "value": "ABN"},
		},
		"privacy": false, "is_locked": true,
	})
	return string(b)
}

func envelope(data string) string { return `{"status":true,"data":` + data + `}` }

func listEnvelope(data string, total, pages, current int) string {
	b, _ := json.Marshal(map[string]int{"total_items": total, "total_pages": pages, "current_page": current})
	return `{"status":true,"data":` + data + `,"pagination":` + string(b) + `}`
}

const customerJSON = `{"id":501,"status_id":1,"username":"smets","first_name":"Jane","last_name":"Citizen",
	"phone":"+61.299999999","mobile":"","email":"jane@example.org","currency":"AUD","account_type":"business",
	"business_name":"Sydney Mets Touch Association","date_added":"2019-03-04 10:11:12","auth_key":"` + eppSecret + `"}`

const registrantJSON = `{"id":9001,"domain_id":42,"customer_id":501,"name":"Sydney Mets Touch Association Incorporated",
	"first_name":"Jane","last_name":"Citizen","address":"1 Example St","city":"Sydney","country":"Australia","state":"NSW",
	"post_code":"2000","country_code":61,"phone":"+61.299999999","email":"registrant@example.org","account_type":"business",
	"business_name":"Sydney Mets Touch Association Incorporated","business_number_type":"ABN","business_number":"12345678901"}`
