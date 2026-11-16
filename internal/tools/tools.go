// Package tools defines the phase 1 (read-only) MCP tools for the Dreamscape
// reseller API. Each tool lives in its own file with its input and output
// types; Register adds them all to a server.
package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goodtune/crazydomains-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deps are the dependencies shared by all tools.
type Deps struct {
	Client *api.Client
	// Now returns the current time; time.Now if nil. Used for
	// days_until_expiry and expiring_within_days.
	Now func() time.Time
}

func (d *Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// Instructions are sent to MCP clients on initialisation.
const Instructions = "Read-only access to a Crazy Domains (Dreamscape) reseller account: domains, expiry dates, " +
	"nameservers, eligibility data, customers, registrants, Dreamscape-hosted DNS records, availability, TLD pricing and " +
	"account balance. Dates are YYYY-MM-DD. EPP transfer codes are never returned. No tool here can change anything."

// NewServer returns an MCP server with all phase 1 tools registered.
func NewServer(d *Deps, version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "crazydomains-mcp",
		Title:   "Crazy Domains reseller (read-only)",
		Version: version,
	}, &mcp.ServerOptions{Instructions: Instructions})
	Register(s, d)
	return s
}

// Register adds every phase 1 tool to s. All are read-only. Phase 2 write
// tools are not registered here; they belong behind DREAMSCAPE_ENABLE_WRITES.
func Register(s *mcp.Server, d *Deps) {
	addListDomains(s, d)
	addGetDomain(s, d)
	addListCustomers(s, d)
	addGetCustomer(s, d)
	addGetRegistrant(s, d)
	addListDomainDNSRecords(s, d)
	addCheckDomainAvailability(s, d)
	addCheckTransferAvailability(s, d)
	addGetAccountBalance(s, d)
	addListTLDs(s, d)
}

// readOnly returns annotations for a read-only, idempotent tool.
func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:          title,
		ReadOnlyHint:   true,
		IdempotentHint: true,
	}
}

// Reference data from the API documentation.
var (
	domainStatuses = map[int]string{
		1: "Awaiting Registration",
		2: "Registered",
		3: "Awaiting Renewal",
		4: "Awaiting Deleted",
		5: "Deleted",
		6: "Expired",
	}
	customerStatuses = map[int]string{
		1: "Active",
	}
)

func statusName(m map[int]string, id int) string {
	if s, ok := m[id]; ok {
		return s
	}
	return fmt.Sprintf("unknown (%d)", id)
}

// dateLayouts are the formats the API may use for date-time fields.
var dateLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// parseDate returns the calendar date of an API date-time string as written
// (no timezone conversion).
func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

// isoDate formats an API date-time as YYYY-MM-DD, or returns it unchanged if
// it cannot be parsed.
func isoDate(s string) string {
	if t, ok := parseDate(s); ok {
		return t.Format("2006-01-02")
	}
	return s
}

// daysUntil returns the number of whole calendar days from now's date to the
// given date, or nil if the date cannot be parsed.
func daysUntil(s string, now time.Time) *int {
	t, ok := parseDate(s)
	if !ok {
		return nil
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	n := int(t.Sub(today).Hours() / 24)
	return &n
}

// Paging is the pagination input shared by list tools.
type Paging struct {
	Page  int `json:"page,omitempty" jsonschema:"page number, starting at 1 (default 1)"`
	Limit int `json:"limit,omitempty" jsonschema:"items per page, 1-100 (default 50)"`
}

const defaultLimit = 50

func (p Paging) normalise() (page, limit int, err error) {
	page, limit = p.Page, p.Limit
	if page < 0 || limit < 0 {
		return 0, 0, errors.New("page and limit must be positive")
	}
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit > api.MaxPageSize {
		limit = api.MaxPageSize
	}
	return page, limit, nil
}

// PageInfo is the pagination output shared by list tools.
type PageInfo struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Total   *int `json:"total,omitempty" jsonschema:"total number of items, when the API reports it"`
	HasMore bool `json:"has_more"`
}

func pageInfo(page, limit, got int, pg *api.Pagination) PageInfo {
	pi := PageInfo{Page: page, Limit: limit}
	if pg == nil {
		pi.HasMore = got >= limit
		return pi
	}
	total := pg.TotalItems
	pi.Total = &total
	if pg.CurrentPage > 0 {
		pi.Page = pg.CurrentPage
	}
	if pg.TotalPages > 0 {
		pi.HasMore = pi.Page < pg.TotalPages
	} else {
		pi.HasMore = pi.Page*limit < total
	}
	return pi
}

// normaliseDomainName lower-cases and trims a domain name.
func normaliseDomainName(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
}

// DomainRef identifies a domain by ID or name (exactly one).
type DomainRef struct {
	DomainID   int    `json:"domain_id,omitempty" jsonschema:"Dreamscape domain ID; give this or domain_name"`
	DomainName string `json:"domain_name,omitempty" jsonschema:"fully qualified domain name, e.g. example.com.au; give this or domain_id"`
}

// resolveDomainID returns the domain ID for ref. A name is looked up with
// GET /domains?domain_name= and must match exactly one domain exactly
// (case-insensitive); otherwise the error lists the candidates.
func resolveDomainID(ctx context.Context, c *api.Client, ref DomainRef) (int, error) {
	name := normaliseDomainName(ref.DomainName)
	switch {
	case ref.DomainID > 0 && name != "":
		return 0, errors.New("give either domain_id or domain_name, not both")
	case ref.DomainID > 0:
		return ref.DomainID, nil
	case ref.DomainID < 0:
		return 0, errors.New("domain_id must be positive")
	case name == "":
		return 0, errors.New("domain_id or domain_name is required")
	}
	domains, _, err := c.ListDomains(ctx, api.ListDomainsParams{DomainName: name, Limit: api.MaxPageSize})
	if err != nil {
		return 0, fmt.Errorf("looking up %q: %w", name, err)
	}
	var exact []api.Domain
	for _, d := range domains {
		if normaliseDomainName(d.DomainName) == name {
			exact = append(exact, d)
		}
	}
	if len(exact) == 1 {
		return exact[0].ID, nil
	}
	if len(domains) == 0 {
		return 0, fmt.Errorf("no domain named %q in this reseller account", name)
	}
	candidates := exact
	if len(candidates) == 0 {
		candidates = domains
	}
	var list []string
	for _, d := range candidates {
		list = append(list, fmt.Sprintf("%s (id %d)", d.DomainName, d.ID))
	}
	if len(exact) > 1 {
		return 0, fmt.Errorf("%d domains match %q exactly; use domain_id: %s", len(exact), name, strings.Join(list, ", "))
	}
	return 0, fmt.Errorf("no exact match for %q; candidates: %s", name, strings.Join(list, ", "))
}

// orEmpty replaces a nil slice with an empty one so it encodes as [].
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func joinName(first, last string) string {
	return strings.TrimSpace(first + " " + last)
}
