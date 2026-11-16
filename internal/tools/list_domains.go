package tools

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/goodtune/crazydomains-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListDomainsInput is the input of list_domains.
type ListDomainsInput struct {
	DomainName         string   `json:"domain_name,omitempty" jsonschema:"filter by domain name (passed to the API's domain_name filter)"`
	CustomerID         int      `json:"customer_id,omitempty" jsonschema:"only domains owned by this customer ID"`
	StatusID           int      `json:"status_id,omitempty" jsonschema:"only domains with this status: 1 Awaiting Registration, 2 Registered, 3 Awaiting Renewal, 4 Awaiting Deleted, 5 Deleted, 6 Expired"`
	TLDs               []string `json:"tlds,omitempty" jsonschema:"only these TLDs, e.g. [\"com.au\", \"com\"]"`
	ExpiringWithinDays *int     `json:"expiring_within_days,omitempty" jsonschema:"only domains whose expiry date is between today and this many days from now (filtered client-side across all pages, sorted by expiry)"`
	Paging
}

// DomainSummary is one domain in list_domains output.
type DomainSummary struct {
	ID              int    `json:"id"`
	DomainName      string `json:"domain_name"`
	Status          string `json:"status"`
	StatusID        int    `json:"status_id"`
	StartDate       string `json:"start_date"`
	ExpiryDate      string `json:"expiry_date"`
	DaysUntilExpiry *int   `json:"days_until_expiry,omitempty"`
	CustomerID      int    `json:"customer_id"`
	Privacy         bool   `json:"privacy"`
	IsLocked        bool   `json:"is_locked"`
}

// ListDomainsOutput is the output of list_domains.
type ListDomainsOutput struct {
	Domains []DomainSummary `json:"domains"`
	PageInfo
}

// maxScanPages bounds how many upstream pages expiring_within_days reads.
const maxScanPages = 50

func summariseDomain(d api.Domain, now time.Time) DomainSummary {
	return DomainSummary{
		ID:              d.ID,
		DomainName:      d.DomainName,
		Status:          statusName(domainStatuses, d.StatusID),
		StatusID:        d.StatusID,
		StartDate:       isoDate(d.StartDate),
		ExpiryDate:      isoDate(d.ExpiryDate),
		DaysUntilExpiry: daysUntil(d.ExpiryDate, now),
		CustomerID:      d.CustomerID,
		Privacy:         d.Privacy,
		IsLocked:        d.IsLocked,
	}
}

func addListDomains(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "list_domains",
		Description: "List domains in the Crazy Domains (Dreamscape) reseller account, with status, start/expiry dates (YYYY-MM-DD) and owning customer. " +
			"Filter by name, customer, status or TLD. Use expiring_within_days to find upcoming renewals: the whole account is scanned and " +
			"matching domains are returned sorted by expiry date, paginated locally.",
		Annotations: readOnly("List domains"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListDomainsInput) (*mcp.CallToolResult, ListDomainsOutput, error) {
		page, limit, err := in.normalise()
		if err != nil {
			return nil, ListDomainsOutput{}, err
		}
		params := api.ListDomainsParams{
			DomainName: in.DomainName,
			CustomerID: in.CustomerID,
			StatusID:   in.StatusID,
			TLDs:       in.TLDs,
		}
		now := d.now()

		if in.ExpiringWithinDays == nil {
			params.Page, params.Limit = page, limit
			domains, pg, err := d.Client.ListDomains(ctx, params)
			if err != nil {
				return nil, ListDomainsOutput{}, err
			}
			out := ListDomainsOutput{Domains: []DomainSummary{}, PageInfo: pageInfo(page, limit, len(domains), pg)}
			for _, dom := range domains {
				out.Domains = append(out.Domains, summariseDomain(dom, now))
			}
			return nil, out, nil
		}

		window := *in.ExpiringWithinDays
		if window < 0 {
			return nil, ListDomainsOutput{}, errors.New("expiring_within_days must be zero or positive")
		}
		var matched []DomainSummary
		params.Limit = api.MaxPageSize
		for p := 1; p <= maxScanPages; p++ {
			params.Page = p
			domains, pg, err := d.Client.ListDomains(ctx, params)
			if err != nil {
				return nil, ListDomainsOutput{}, err
			}
			for _, dom := range domains {
				sum := summariseDomain(dom, now)
				if sum.DaysUntilExpiry != nil && *sum.DaysUntilExpiry >= 0 && *sum.DaysUntilExpiry <= window {
					matched = append(matched, sum)
				}
			}
			if pg != nil && pg.TotalPages > 0 {
				if p >= pg.TotalPages {
					break
				}
			} else if len(domains) < api.MaxPageSize {
				break
			}
		}
		sort.SliceStable(matched, func(i, j int) bool {
			return *matched[i].DaysUntilExpiry < *matched[j].DaysUntilExpiry
		})
		total := len(matched)
		start := min((page-1)*limit, total)
		end := min(start+limit, total)
		return nil, ListDomainsOutput{
			Domains:  orEmpty(matched[start:end]),
			PageInfo: PageInfo{Page: page, Limit: limit, Total: &total, HasMore: end < total},
		}, nil
	})
}
