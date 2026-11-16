package tools

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// pagedHandler serves 250 items across pages of the requested limit and
// reports pagination unless omitPagination is set.
func pagedHandler(t *testing.T, item func(i int) string, total int, omitPagination bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 || limit < 1 || limit > 100 {
			t.Errorf("bad page/limit: %s", r.URL.RawQuery)
		}
		var items []string
		for i := (page - 1) * limit; i < min(page*limit, total); i++ {
			items = append(items, item(i))
		}
		data := "[" + strings.Join(items, ",") + "]"
		if omitPagination {
			jsonHandler(envelope(data))(w, r)
			return
		}
		pages := (total + limit - 1) / limit
		jsonHandler(listEnvelope(data, total, pages, page))(w, r)
	}
}

func TestPagination(t *testing.T) {
	type listTool struct {
		tool, path, key string
		item            func(i int) string
	}
	tools := []listTool{
		{"list_domains", "/domains", "domains", func(i int) string {
			return domainJSON(1000+i, fmt.Sprintf("d%d.com.au", i), "2027-01-10T00:00:00+00:00")
		}},
		{"list_customers", "/customers", "customers", func(i int) string {
			return fmt.Sprintf(`{"id":%d,"status_id":1,"first_name":"C","last_name":"%d"}`, i+1, i)
		}},
		{"list_tlds", "/domains/tlds", "tlds", func(i int) string {
			return fmt.Sprintf(`{"tld":"t%d","price":{"register":"1.00","renew":"1.00","transfer":"1.00"}}`, i)
		}},
	}
	cases := []struct {
		page, limit         int
		omitPagination      bool
		wantPage, wantLimit int
		wantItems           int
		wantMore            bool
		wantTotal           any
	}{
		{page: 0, limit: 0, wantPage: 1, wantLimit: 50, wantItems: 50, wantMore: true, wantTotal: 250.0},
		{page: 2, limit: 100, wantPage: 2, wantLimit: 100, wantItems: 100, wantMore: true, wantTotal: 250.0},
		{page: 3, limit: 100, wantPage: 3, wantLimit: 100, wantItems: 50, wantMore: false, wantTotal: 250.0},
		{page: 1, limit: 500, wantPage: 1, wantLimit: 100, wantItems: 100, wantMore: true, wantTotal: 250.0}, // clamped
		{page: 2, limit: 100, omitPagination: true, wantPage: 2, wantLimit: 100, wantItems: 100, wantMore: true, wantTotal: nil},
		{page: 3, limit: 100, omitPagination: true, wantPage: 3, wantLimit: 100, wantItems: 50, wantMore: false, wantTotal: nil},
	}
	for _, lt := range tools {
		for _, c := range cases {
			t.Run(fmt.Sprintf("%s/page%d_limit%d_nopag%v", lt.tool, c.page, c.limit, c.omitPagination), func(t *testing.T) {
				cs, u := setup(t, map[string]http.HandlerFunc{lt.path: pagedHandler(t, lt.item, 250, c.omitPagination)})
				args := map[string]any{}
				if c.page > 0 {
					args["page"] = c.page
				}
				if c.limit > 0 {
					args["limit"] = c.limit
				}
				cr := call(t, cs, lt.tool, args)
				mustOK(t, lt.tool, cr)
				q, _ := url.ParseQuery(u.lastQuery(lt.path))
				if q.Get("page") != strconv.Itoa(c.wantPage) || q.Get("limit") != strconv.Itoa(c.wantLimit) {
					t.Errorf("upstream query = %v", q)
				}
				if n := len(cr.data[lt.key].([]any)); n != c.wantItems {
					t.Errorf("items = %d, want %d", n, c.wantItems)
				}
				if cr.data["page"] != float64(c.wantPage) || cr.data["limit"] != float64(c.wantLimit) ||
					cr.data["has_more"] != c.wantMore || cr.data["total"] != c.wantTotal {
					t.Errorf("page info = page:%v limit:%v has_more:%v total:%v", cr.data["page"], cr.data["limit"], cr.data["has_more"], cr.data["total"])
				}
			})
		}
	}
}

func TestPaginationRejectsNegative(t *testing.T) {
	cs, _ := setup(t, nil)
	for _, tool := range []string{"list_domains", "list_customers", "list_tlds"} {
		cr := call(t, cs, tool, map[string]any{"page": -1})
		mustErr(t, tool, cr, "page and limit must be positive")
	}
}

func TestListDomainsExpiringWithin(t *testing.T) {
	// fixedNow is 2026-11-16. 150 domains over two upstream pages.
	expiries := map[int]string{
		3:   "2026-11-16T00:00:00Z", // today: 0 days, included
		10:  "2027-02-14 00:00:00",  // 90 days, included (boundary)
		20:  "2027-02-15 00:00:00",  // 91 days, excluded
		30:  "2026-11-15T00:00:00Z", // already expired, excluded
		120: "2026-12-01T00:00:00Z", // 15 days, on page 2, included
		140: "",                     // unparseable, excluded
	}
	item := func(i int) string {
		e, ok := expiries[i]
		if !ok {
			e = "2029-01-01T00:00:00Z"
		}
		return domainJSON(i, fmt.Sprintf("d%d.com.au", i), e)
	}
	cs, u := setup(t, map[string]http.HandlerFunc{"/domains": pagedHandler(t, item, 150, false)})
	cr := call(t, cs, "list_domains", map[string]any{"expiring_within_days": 90, "customer_id": 501})
	mustOK(t, "list_domains", cr)
	if u.callCount("/domains") != 2 {
		t.Errorf("upstream pages fetched = %d, want 2", u.callCount("/domains"))
	}
	if q, _ := url.ParseQuery(u.lastQuery("/domains")); q.Get("limit") != "100" || q.Get("customer_id") != "501" {
		t.Errorf("upstream query = %v", q)
	}
	var got []string
	for _, d := range cr.data["domains"].([]any) {
		m := d.(map[string]any)
		got = append(got, fmt.Sprintf("%s:%v", m["domain_name"], m["days_until_expiry"]))
	}
	want := "d3.com.au:0,d120.com.au:15,d10.com.au:90"
	if strings.Join(got, ",") != want {
		t.Errorf("domains = %v, want %s", got, want)
	}
	if cr.data["total"] != 3.0 || cr.data["has_more"] != false {
		t.Errorf("page info = %v", cr.data)
	}

	// Local pagination over the filtered set.
	cr = call(t, cs, "list_domains", map[string]any{"expiring_within_days": 90, "limit": 2, "page": 2})
	mustOK(t, "list_domains", cr)
	doms := cr.data["domains"].([]any)
	if len(doms) != 1 || doms[0].(map[string]any)["domain_name"] != "d10.com.au" || cr.data["has_more"] != false || cr.data["total"] != 3.0 {
		t.Errorf("page 2 = %v", cr.data)
	}
	cr = call(t, cs, "list_domains", map[string]any{"expiring_within_days": 90, "limit": 2, "page": 1})
	if cr.data["has_more"] != true || len(cr.data["domains"].([]any)) != 2 {
		t.Errorf("page 1 = %v", cr.data)
	}

	cr = call(t, cs, "list_domains", map[string]any{"expiring_within_days": -1})
	mustErr(t, "list_domains", cr, "expiring_within_days must be zero or positive")
}
