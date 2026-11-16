package tools

import (
	"net/http"
	"net/url"
	"testing"
)

func sydneyMetsHandlers(list string) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/domains":                  jsonHandler(listEnvelope(list, 0, 1, 1)),
		"/domains/42":               jsonHandler(envelope(domainJSON(42, "sydneymets.com.au", "2027-01-10T00:00:00+00:00"))),
		"/customers/501":            jsonHandler(envelope(customerJSON)),
		"/domains/registrants/9001": jsonHandler(envelope(registrantJSON)),
	}
}

func TestGetDomainByNameOneMatch(t *testing.T) {
	// The API may match substrings; only the exact (case-insensitive) match counts.
	list := "[" + domainJSON(41, "notsydneymets.com.au", "2027-01-01T00:00:00Z") + "," +
		domainJSON(42, "SydneyMets.com.au", "2027-01-10T00:00:00Z") + "]"
	cs, u := setup(t, sydneyMetsHandlers(list))
	cr := call(t, cs, "get_domain", map[string]any{"domain_name": " SYDNEYMETS.com.au. "})
	mustOK(t, "get_domain", cr)
	checkSydneyMets(t, cr)
	if q, _ := url.ParseQuery(u.lastQuery("/domains")); q.Get("domain_name") != "sydneymets.com.au" {
		t.Errorf("lookup query = %v", q)
	}
	if u.callCount("/domains/42") != 1 {
		t.Error("domain detail not fetched")
	}
}

func TestGetDomainByNameNoMatch(t *testing.T) {
	cs, u := setup(t, sydneyMetsHandlers("[]"))
	cr := call(t, cs, "get_domain", map[string]any{"domain_name": "nosuch.com.au"})
	mustErr(t, "get_domain", cr, `no domain named "nosuch.com.au"`)
	if u.callCount("/domains/42") != 0 {
		t.Error("detail fetched despite no match")
	}
}

func TestGetDomainByNameManyMatches(t *testing.T) {
	list := "[" + domainJSON(42, "sydneymets.com.au", "2027-01-10T00:00:00Z") + "," +
		domainJSON(77, "sydneymets.com.au", "2025-01-10T00:00:00Z") + "]"
	cs, _ := setup(t, sydneyMetsHandlers(list))
	cr := call(t, cs, "get_domain", map[string]any{"domain_name": "sydneymets.com.au"})
	mustErr(t, "get_domain", cr, `2 domains match "sydneymets.com.au" exactly; use domain_id: sydneymets.com.au (id 42), sydneymets.com.au (id 77)`)
}

func TestGetDomainByNameOnlyPartialMatches(t *testing.T) {
	list := "[" + domainJSON(41, "notsydneymets.com.au", "2027-01-01T00:00:00Z") + "," +
		domainJSON(43, "sydneymets.com", "2027-01-01T00:00:00Z") + "]"
	cs, _ := setup(t, sydneyMetsHandlers(list))
	cr := call(t, cs, "get_domain", map[string]any{"domain_name": "sydneymets.com.au"})
	mustErr(t, "get_domain", cr, `no exact match for "sydneymets.com.au"; candidates: notsydneymets.com.au (id 41), sydneymets.com (id 43)`)
}

func TestGetDomainArgValidation(t *testing.T) {
	cs, _ := setup(t, nil)
	mustErr(t, "get_domain", call(t, cs, "get_domain", nil), "domain_id or domain_name is required")
	mustErr(t, "get_domain", call(t, cs, "get_domain", map[string]any{"domain_id": 1, "domain_name": "x.com"}), "not both")
	mustErr(t, "list_domain_dns_records", call(t, cs, "list_domain_dns_records", nil), "domain_id or domain_name is required")
}

func TestGetDomainWithoutParties(t *testing.T) {
	h := sydneyMetsHandlers("[]")
	cs, u := setup(t, h)
	cr := call(t, cs, "get_domain", map[string]any{"domain_id": 42, "resolve_parties": false})
	mustOK(t, "get_domain", cr)
	if _, ok := cr.data["customer"]; ok {
		t.Error("customer embedded despite resolve_parties=false")
	}
	if u.callCount("/customers/501") != 0 || u.callCount("/domains/registrants/9001") != 0 {
		t.Error("party endpoints called despite resolve_parties=false")
	}
}

func TestGetDomainPartyLookupFailureIsWarning(t *testing.T) {
	h := sydneyMetsHandlers("[]")
	h["/customers/501"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Customer not found"}`))
	}
	cs, _ := setup(t, h)
	cr := call(t, cs, "get_domain", map[string]any{"domain_id": 42})
	mustOK(t, "get_domain", cr)
	w, _ := cr.data["warnings"].([]any)
	if len(w) != 1 || w[0] != "customer 501: not found: Customer not found" {
		t.Errorf("warnings = %v", cr.data["warnings"])
	}
	if get(cr.data, "registrant", "id") != 9001.0 {
		t.Errorf("registrant = %v", cr.data["registrant"])
	}
}

func TestDNSRecordsEmptyNote(t *testing.T) {
	list := "[" + domainJSON(42, "sydneymets.com.au", "2027-01-10T00:00:00Z") + "]"
	h := sydneyMetsHandlers(list)
	h["/domains/42/dns"] = jsonHandler(envelope("[]"))
	cs, _ := setup(t, h)
	cr := call(t, cs, "list_domain_dns_records", map[string]any{"domain_name": "sydneymets.com.au"})
	mustOK(t, "list_domain_dns_records", cr)
	if len(cr.data["records"].([]any)) != 0 {
		t.Errorf("records = %v", cr.data["records"])
	}
	want := "No DNS records hosted at Dreamscape. The domain is delegated to: ns1.digitalocean.com, ns2.digitalocean.com, ns3.digitalocean.com"
	if cr.data["note"] != want {
		t.Errorf("note = %v", cr.data["note"])
	}
}

func TestInputValidation(t *testing.T) {
	cs, _ := setup(t, nil)
	mustErr(t, "get_customer", call(t, cs, "get_customer", map[string]any{"customer_id": 0}), "customer_id must be a positive integer")
	mustErr(t, "get_registrant", call(t, cs, "get_registrant", map[string]any{"registrant_id": -3}), "registrant_id must be a positive integer")
	mustErr(t, "check_domain_availability", call(t, cs, "check_domain_availability", map[string]any{"domain_names": []string{" "}}), "at least one name")
	many := make([]string, 21)
	for i := range many {
		many[i] = "x.com"
	}
	mustErr(t, "check_domain_availability", call(t, cs, "check_domain_availability", map[string]any{"domain_names": many}), "at most 20")
	mustErr(t, "check_transfer_availability", call(t, cs, "check_transfer_availability", map[string]any{"domain_name": ""}), "domain_name is required")
}
