package tools

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
)

type toolCase struct {
	name     string
	args     map[string]any
	handlers func() map[string]http.HandlerFunc
	check    func(t *testing.T, cr callResult, u *upstream)
}

func toolCases() []toolCase {
	return []toolCase{
		{
			name: "list_domains",
			args: map[string]any{"domain_name": "mets", "customer_id": 501, "status_id": 2, "tlds": []string{"com.au"}},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{
					"/domains": jsonHandler(listEnvelope("["+domainJSON(42, "sydneymets.com.au", "2027-01-10T00:00:00+00:00")+","+
						domainJSON(43, "northmets.com.au", "2028-05-01 00:00:00")+"]", 2, 1, 1)),
				}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				q, _ := url.ParseQuery(u.lastQuery("/domains"))
				if q.Get("domain_name") != "mets" || q.Get("customer_id") != "501" || q.Get("status_id") != "2" || q.Get("tlds[]") != "com.au" {
					t.Errorf("upstream query = %v", q)
				}
				doms := cr.data["domains"].([]any)
				if len(doms) != 2 {
					t.Fatalf("domains = %v", doms)
				}
				d0 := doms[0].(map[string]any)
				want := map[string]any{"id": 42.0, "domain_name": "sydneymets.com.au", "status": "Registered", "start_date": "2024-01-10",
					"expiry_date": "2027-01-10", "customer_id": 501.0, "privacy": false, "is_locked": true, "days_until_expiry": 55.0}
				for k, v := range want {
					if d0[k] != v {
						t.Errorf("domains[0].%s = %v, want %v", k, d0[k], v)
					}
				}
				if doms[1].(map[string]any)["expiry_date"] != "2028-05-01" {
					t.Errorf("domains[1].expiry_date = %v", doms[1])
				}
				if cr.data["total"] != 2.0 || cr.data["has_more"] != false || cr.data["page"] != 1.0 || cr.data["limit"] != 50.0 {
					t.Errorf("page info = %v", cr.data)
				}
			},
		},
		{
			name: "get_domain",
			args: map[string]any{"domain_id": 42},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{
					"/domains/42":               jsonHandler(envelope(domainJSON(42, "sydneymets.com.au", "2027-01-10T00:00:00+00:00"))),
					"/customers/501":            jsonHandler(envelope(customerJSON)),
					"/domains/registrants/9001": jsonHandler(envelope(registrantJSON)),
				}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				checkSydneyMets(t, cr)
			},
		},
		{
			name: "list_customers",
			args: map[string]any{"status_id": 1},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{"/customers": jsonHandler(listEnvelope("["+customerJSON+"]", 1, 1, 1))}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				if q, _ := url.ParseQuery(u.lastQuery("/customers")); q.Get("status_id") != "1" {
					t.Errorf("query = %v", q)
				}
				cs := cr.data["customers"].([]any)
				c := cs[0].(map[string]any)
				checkCustomer(t, c)
			},
		},
		{
			name: "get_customer",
			args: map[string]any{"customer_id": 501},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{"/customers/501": jsonHandler(envelope(customerJSON))}
			},
			check: func(t *testing.T, cr callResult, u *upstream) { checkCustomer(t, cr.data) },
		},
		{
			name: "get_registrant",
			args: map[string]any{"registrant_id": 9001},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{"/domains/registrants/9001": jsonHandler(envelope(registrantJSON))}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				want := map[string]any{"id": 9001.0, "domain_id": 42.0, "name": "Sydney Mets Touch Association Incorporated",
					"organisation": "Sydney Mets Touch Association Incorporated", "business_number_type": "ABN",
					"business_number": "12345678901", "email": "registrant@example.org", "state": "NSW", "country_code": 61.0}
				for k, v := range want {
					if cr.data[k] != v {
						t.Errorf("%s = %v, want %v", k, cr.data[k], v)
					}
				}
			},
		},
		{
			name: "list_domain_dns_records",
			args: map[string]any{"domain_id": 42, "type": "mx"},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{
					"/domains/42": jsonHandler(envelope(domainJSON(42, "sydneymets.com.au", "2027-01-10T00:00:00+00:00"))),
					"/domains/42/dns": jsonHandler(envelope(`[{"id":1,"type":"MX","subdomain":"","content":"mx.example.org","priority":10},
						{"id":2,"type":"CAA","flag":0,"tag":"issue","value":"letsencrypt.org","auth_key":"` + eppSecret + `"}]`)),
				}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				if q, _ := url.ParseQuery(u.lastQuery("/domains/42/dns")); q.Get("type") != "MX" {
					t.Errorf("query = %v", q)
				}
				recs := cr.data["records"].([]any)
				if len(recs) != 2 || recs[0].(map[string]any)["content"] != "mx.example.org" || recs[1].(map[string]any)["tag"] != "issue" {
					t.Errorf("records = %v", recs)
				}
				if ns := cr.data["name_servers"].([]any); len(ns) != 3 {
					t.Errorf("name_servers = %v", ns)
				}
				if _, ok := cr.data["note"]; ok {
					t.Errorf("unexpected note: %v", cr.data["note"])
				}
			},
		},
		{
			name: "check_domain_availability",
			args: map[string]any{"domain_names": []string{"Example-New.com.au", "taken.com"}, "currency": "AUD"},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{"/domains/availability": jsonHandler(envelope(`[
					{"domain_name":"example-new.com.au","checking_error":"","is_available":true,"status":"regular","register_price":12.95,"renew_price":"12.95"},
					{"domain_name":"taken.com","checking_error":"","is_available":false,"status":null,"register_price":null,"renew_price":null}]`))}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				q, _ := url.ParseQuery(u.lastQuery("/domains/availability"))
				if got := q["domain_names[]"]; len(got) != 2 || got[0] != "example-new.com.au" || q.Get("currency") != "AUD" {
					t.Errorf("query = %v", q)
				}
				rs := cr.data["results"].([]any)
				r0 := rs[0].(map[string]any)
				if r0["is_available"] != true || r0["status"] != "regular" || r0["register_price"] != "12.95" {
					t.Errorf("results[0] = %v", r0)
				}
				if rs[1].(map[string]any)["is_available"] != false {
					t.Errorf("results[1] = %v", rs[1])
				}
			},
		},
		{
			name: "check_transfer_availability",
			args: map[string]any{"domain_name": "transferme.com", "period": 1},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{"/domains/transfers/availability": jsonHandler(envelope(`{"status":true,
					"eligible_for_renewal":true,"renewal_required":true,"is_premium":false,"renew_price":18.5,
					"expiry_date":"2027-03-01T00:00:00Z","allowed_renewal_periods":[1,2,3],"auth_key":"` + eppSecret + `"}`))}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				q, _ := url.ParseQuery(u.lastQuery("/domains/transfers/availability"))
				if q.Get("domain_name") != "transferme.com" || q.Get("period") != "1" || q.Has("auth_key") {
					t.Errorf("query = %v", q)
				}
				if cr.data["transferable"] != true || cr.data["renew_price"] != "18.5" || cr.data["expiry_date"] != "2027-03-01" {
					t.Errorf("output = %v", cr.data)
				}
				if p := cr.data["allowed_renewal_periods"].([]any); len(p) != 3 {
					t.Errorf("allowed_renewal_periods = %v", p)
				}
			},
		},
		{
			name: "get_account_balance",
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{"/finances/balance": jsonHandler(envelope(`{"balance":1234.50,"currency":"AUD"}`))}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				if cr.data["balance"] != "1234.50" || cr.data["currency"] != "AUD" {
					t.Errorf("output = %v", cr.data)
				}
			},
		},
		{
			name: "list_tlds",
			args: map[string]any{"tlds": []string{".COM.AU"}, "currency": "AUD"},
			handlers: func() map[string]http.HandlerFunc {
				return map[string]http.HandlerFunc{"/domains/tlds": jsonHandler(listEnvelope(`[{"tld":"com.au","min_period":1,"max_period":5,
					"price":{"register":"12.95","renew":"12.95","transfer":"0.00"},"eligibility_required":true,"privacy_allowed":false}]`, 1, 1, 1))}
			},
			check: func(t *testing.T, cr callResult, u *upstream) {
				q, _ := url.ParseQuery(u.lastQuery("/domains/tlds"))
				if q.Get("tlds[]") != "com.au" || q.Get("currency") != "AUD" {
					t.Errorf("query = %v", q)
				}
				tl := cr.data["tlds"].([]any)[0].(map[string]any)
				if tl["tld"] != "com.au" || tl["register_price"] != "12.95" || tl["eligibility_required"] != true || tl["max_period"] != 5.0 {
					t.Errorf("tld = %v", tl)
				}
			},
		},
	}
}

func checkSydneyMets(t *testing.T, cr callResult) {
	t.Helper()
	want := map[string]any{"id": 42.0, "domain_name": "sydneymets.com.au", "status": "Registered", "status_id": 2.0, "period": 2.0,
		"start_date": "2024-01-10", "expiry_date": "2027-01-10", "days_until_expiry": 55.0, "customer_id": 501.0,
		"registrant_id": 9001.0, "admin_contact_id": 11.0, "billing_contact_id": 12.0, "tech_contact_id": 13.0,
		"privacy": false, "is_locked": true}
	for k, v := range want {
		if cr.data[k] != v {
			t.Errorf("%s = %v, want %v", k, cr.data[k], v)
		}
	}
	ns := cr.data["name_servers"].([]any)
	var hosts []string
	for _, n := range ns {
		hosts = append(hosts, n.(map[string]any)["host"].(string))
	}
	if strings.Join(hosts, ",") != "ns1.digitalocean.com,ns2.digitalocean.com,ns3.digitalocean.com" {
		t.Errorf("name servers = %v", hosts)
	}
	el := cr.data["eligibility_data"].([]any)[0].(map[string]any)
	if el["name"] != "Registrant Name" || el["value"] != "Sydney Mets Touch Association Incorporated" {
		t.Errorf("eligibility_data = %v", cr.data["eligibility_data"])
	}
	if get(cr.data, "customer", "name") != "Jane Citizen" || get(cr.data, "customer", "organisation") != "Sydney Mets Touch Association" ||
		get(cr.data, "customer", "email") != "jane@example.org" {
		t.Errorf("customer = %v", cr.data["customer"])
	}
	if get(cr.data, "registrant", "name") != "Sydney Mets Touch Association Incorporated" || get(cr.data, "registrant", "email") != "registrant@example.org" {
		t.Errorf("registrant = %v", cr.data["registrant"])
	}
	if _, ok := cr.data["warnings"]; ok {
		t.Errorf("unexpected warnings: %v", cr.data["warnings"])
	}
}

func checkCustomer(t *testing.T, c map[string]any) {
	t.Helper()
	want := map[string]any{"id": 501.0, "status": "Active", "name": "Jane Citizen", "organisation": "Sydney Mets Touch Association",
		"email": "jane@example.org", "phone": "+61.299999999", "created": "2019-03-04"}
	for k, v := range want {
		if c[k] != v {
			t.Errorf("customer.%s = %v, want %v", k, c[k], v)
		}
	}
	if _, ok := c["username"]; ok {
		t.Error("customer output exposes username")
	}
}

func TestToolsHappyPath(t *testing.T) {
	for _, tc := range toolCases() {
		t.Run(tc.name, func(t *testing.T) {
			cs, u := setup(t, tc.handlers())
			cr := call(t, cs, tc.name, tc.args)
			mustOK(t, tc.name, cr)
			if cr.data == nil {
				t.Fatalf("no structured content: %s", cr.raw)
			}
			tc.check(t, cr, u)
		})
	}
}

func TestToolsUpstreamErrors(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{401, `{"message":"Invalid signature"}`, "authentication failed (check key / sandbox vs production)"},
		{404, `{"status":false,"message":"Not found"}`, "not found"},
		{402, `{"message":"Insufficient funds"}`, "insufficient reseller balance"},
		{400, `{"errors":{"limit":["too big"]},"auth_key":"` + eppSecret + `"}`, "upstream error: HTTP 400: limit: too big"},
	}
	for _, tc := range toolCases() {
		for _, ec := range cases {
			t.Run(tc.name+"/"+http.StatusText(ec.status), func(t *testing.T) {
				cs, u := setup(t, tc.handlers())
				u.override = func(w http.ResponseWriter, r *http.Request, n int) bool {
					w.WriteHeader(ec.status)
					w.Write([]byte(ec.body))
					return true
				}
				cr := call(t, cs, tc.name, tc.args)
				mustErr(t, tc.name, cr, ec.want)
				// 4xx is never retried.
				u.mu.Lock()
				defer u.mu.Unlock()
				if len(u.calls) == 0 {
					t.Error("no upstream calls")
				}
				for p, n := range u.calls {
					if n != 1 {
						t.Errorf("%s called %d times; 4xx must not be retried", p, n)
					}
				}
			})
		}
	}
}

func TestToolsRetryAfter500(t *testing.T) {
	for _, tc := range toolCases() {
		t.Run(tc.name, func(t *testing.T) {
			cs, u := setup(t, tc.handlers())
			u.override = func(w http.ResponseWriter, r *http.Request, n int) bool {
				if n == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"message":"boom"}`))
					return true
				}
				return false
			}
			cr := call(t, cs, tc.name, tc.args)
			mustOK(t, tc.name, cr)
			tc.check(t, cr, u)
			for p := range tc.handlers() {
				if n := u.callCount(p); n != 0 && n != 2 {
					t.Errorf("%s called %d times, want 2 (500 then retry)", p, n)
				}
			}
		})
	}
}

func TestToolsPersistent500(t *testing.T) {
	cs, u := setup(t, map[string]http.HandlerFunc{})
	u.override = func(w http.ResponseWriter, r *http.Request, n int) bool {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<html>bad gateway</html>"))
		return true
	}
	cr := call(t, cs, "get_account_balance", nil)
	mustErr(t, "get_account_balance", cr, "upstream error: HTTP 502: <html>bad gateway</html>")
	if u.callCount("/finances/balance") != 2 {
		t.Errorf("calls = %d, want 2", u.callCount("/finances/balance"))
	}
}

func TestListToolsReadOnly(t *testing.T) {
	cs, _ := setup(t, nil)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not annotated read-only", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("%s has no description", tool.Name)
		}
	}
	sort.Strings(names)
	want := []string{"check_domain_availability", "check_transfer_availability", "get_account_balance", "get_customer",
		"get_domain", "get_registrant", "list_customers", "list_domain_dns_records", "list_domains", "list_tlds"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("tools = %v, want %v", names, want)
	}
}
