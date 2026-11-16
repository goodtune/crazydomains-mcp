package tools

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetRegistrantInput is the input of get_registrant.
type GetRegistrantInput struct {
	RegistrantID int `json:"registrant_id" jsonschema:"registrant ID (from get_domain's registrant_id)"`
}

// RegistrantOut is the output of get_registrant.
type RegistrantOut struct {
	ID                 int    `json:"id"`
	DomainID           int    `json:"domain_id"`
	CustomerID         int    `json:"customer_id"`
	Name               string `json:"name"`
	FirstName          string `json:"first_name,omitempty"`
	LastName           string `json:"last_name,omitempty"`
	Organisation       string `json:"organisation,omitempty"`
	AccountType        string `json:"account_type,omitempty"`
	BusinessNumberType string `json:"business_number_type,omitempty"`
	BusinessNumber     string `json:"business_number,omitempty"`
	Email              string `json:"email,omitempty"`
	Phone              string `json:"phone,omitempty"`
	Address            string `json:"address,omitempty"`
	City               string `json:"city,omitempty"`
	State              string `json:"state,omitempty"`
	PostCode           string `json:"post_code,omitempty"`
	Country            string `json:"country,omitempty"`
	CountryCode        int    `json:"country_code,omitempty"`
}

func addGetRegistrant(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_registrant",
		Description: "Get a domain registrant (the legal holder of a domain) by registrant ID: name, organisation, business number, contact details and address.",
		Annotations: readOnly("Get registrant"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetRegistrantInput) (*mcp.CallToolResult, RegistrantOut, error) {
		if in.RegistrantID <= 0 {
			return nil, RegistrantOut{}, errors.New("registrant_id must be a positive integer")
		}
		r, err := d.Client.GetRegistrant(ctx, in.RegistrantID)
		if err != nil {
			return nil, RegistrantOut{}, err
		}
		name := r.Name
		if name == "" {
			name = joinName(r.FirstName, r.LastName)
		}
		return nil, RegistrantOut{
			ID:                 r.ID,
			DomainID:           r.DomainID,
			CustomerID:         r.CustomerID,
			Name:               name,
			FirstName:          r.FirstName,
			LastName:           r.LastName,
			Organisation:       r.BusinessName,
			AccountType:        r.AccountType,
			BusinessNumberType: r.BusinessNumberType,
			BusinessNumber:     r.BusinessNumber,
			Email:              r.Email,
			Phone:              r.Phone,
			Address:            r.Address,
			City:               r.City,
			State:              r.State,
			PostCode:           r.PostCode,
			Country:            r.Country,
			CountryCode:        r.CountryCode,
		}, nil
	})
}
