package tools

import (
	"context"
	"fmt"

	"github.com/goodtune/crazydomains-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetDomainInput is the input of get_domain.
type GetDomainInput struct {
	DomainRef
	ResolveParties *bool `json:"resolve_parties,omitempty" jsonschema:"also fetch the customer and registrant summaries (default true)"`
}

// PartySummary is a short customer or registrant summary embedded in
// get_domain output.
type PartySummary struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Organisation string `json:"organisation,omitempty"`
	Email        string `json:"email,omitempty"`
}

// DomainDetail is the output of get_domain.
type DomainDetail struct {
	ID               int                 `json:"id"`
	DomainName       string              `json:"domain_name"`
	Status           string              `json:"status"`
	StatusID         int                 `json:"status_id"`
	Period           int                 `json:"period" jsonschema:"registration period in years"`
	StartDate        string              `json:"start_date"`
	ExpiryDate       string              `json:"expiry_date"`
	DaysUntilExpiry  *int                `json:"days_until_expiry,omitempty"`
	NameServers      []api.NameServer    `json:"name_servers"`
	EligibilityData  []api.EligibilityKV `json:"eligibility_data"`
	Privacy          bool                `json:"privacy"`
	IsLocked         bool                `json:"is_locked"`
	CustomerID       int                 `json:"customer_id"`
	RegistrantID     int                 `json:"registrant_id"`
	AdminContactID   int                 `json:"admin_contact_id"`
	BillingContactID int                 `json:"billing_contact_id"`
	TechContactID    int                 `json:"tech_contact_id"`
	Customer         *PartySummary       `json:"customer,omitempty"`
	Registrant       *PartySummary       `json:"registrant,omitempty"`
	Warnings         []string            `json:"warnings,omitempty"`
}

func addGetDomain(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_domain",
		Description: "Get one domain by domain_id or domain_name: status, period, start/expiry dates, days until expiry, nameservers, " +
			"registry eligibility data (e.g. .au registrant name and ID), privacy, lock and contact IDs. By default also embeds the owning " +
			"customer and the registrant (name, organisation, email). A domain_name must match exactly one domain in the account. " +
			"The EPP transfer code (auth_key) is never returned.",
		Annotations: readOnly("Get domain"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetDomainInput) (*mcp.CallToolResult, DomainDetail, error) {
		id, err := resolveDomainID(ctx, d.Client, in.DomainRef)
		if err != nil {
			return nil, DomainDetail{}, err
		}
		dom, err := d.Client.GetDomain(ctx, id)
		if err != nil {
			return nil, DomainDetail{}, err
		}
		out := DomainDetail{
			ID:               dom.ID,
			DomainName:       dom.DomainName,
			Status:           statusName(domainStatuses, dom.StatusID),
			StatusID:         dom.StatusID,
			Period:           dom.Period,
			StartDate:        isoDate(dom.StartDate),
			ExpiryDate:       isoDate(dom.ExpiryDate),
			DaysUntilExpiry:  daysUntil(dom.ExpiryDate, d.now()),
			NameServers:      orEmpty(dom.NameServers),
			EligibilityData:  orEmpty(dom.EligibilityData),
			Privacy:          dom.Privacy,
			IsLocked:         dom.IsLocked,
			CustomerID:       dom.CustomerID,
			RegistrantID:     dom.RegistrantID,
			AdminContactID:   dom.AdminContactID,
			BillingContactID: dom.BillingContactID,
			TechContactID:    dom.TechContactID,
		}
		if in.ResolveParties != nil && !*in.ResolveParties {
			return nil, out, nil
		}
		// Party lookups are best-effort: a failure is reported as a warning
		// rather than hiding the domain details.
		if dom.CustomerID > 0 {
			if c, err := d.Client.GetCustomer(ctx, dom.CustomerID); err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("customer %d: %v", dom.CustomerID, err))
			} else {
				out.Customer = &PartySummary{
					ID:           c.ID,
					Name:         joinName(c.FirstName, c.LastName),
					Organisation: c.BusinessName,
					Email:        c.Email,
				}
			}
		}
		if dom.RegistrantID > 0 {
			if r, err := d.Client.GetRegistrant(ctx, dom.RegistrantID); err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("registrant %d: %v", dom.RegistrantID, err))
			} else {
				name := r.Name
				if name == "" {
					name = joinName(r.FirstName, r.LastName)
				}
				out.Registrant = &PartySummary{
					ID:           r.ID,
					Name:         name,
					Organisation: r.BusinessName,
					Email:        r.Email,
				}
			}
		}
		return nil, out, nil
	})
}
