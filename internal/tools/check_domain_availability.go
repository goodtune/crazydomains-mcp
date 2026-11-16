package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxAvailabilityNames = 20

// CheckDomainAvailabilityInput is the input of check_domain_availability.
type CheckDomainAvailabilityInput struct {
	DomainNames []string `json:"domain_names" jsonschema:"1 to 20 fully qualified domain names to check"`
	Currency    string   `json:"currency,omitempty" jsonschema:"currency for prices, e.g. AUD (default: the account's currency)"`
}

// AvailabilityOut is one result of check_domain_availability.
type AvailabilityOut struct {
	DomainName    string `json:"domain_name"`
	IsAvailable   bool   `json:"is_available"`
	Status        string `json:"status,omitempty" jsonschema:"regular, premium, resale or reserved"`
	RegisterPrice string `json:"register_price,omitempty"`
	RenewPrice    string `json:"renew_price,omitempty"`
	CheckingError string `json:"checking_error,omitempty"`
}

// CheckDomainAvailabilityOutput is the output of check_domain_availability.
type CheckDomainAvailabilityOutput struct {
	Results []AvailabilityOut `json:"results"`
}

func addCheckDomainAvailability(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "check_domain_availability",
		Description: "Check whether 1-20 domain names are available to register, with register/renew prices. Read-only; nothing is registered or charged.",
		Annotations: readOnly("Check domain availability"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CheckDomainAvailabilityInput) (*mcp.CallToolResult, CheckDomainAvailabilityOutput, error) {
		var names []string
		for _, n := range in.DomainNames {
			if n = normaliseDomainName(n); n != "" {
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			return nil, CheckDomainAvailabilityOutput{}, errors.New("domain_names must contain at least one name")
		}
		if len(names) > maxAvailabilityNames {
			return nil, CheckDomainAvailabilityOutput{}, fmt.Errorf("at most %d domain names per call (got %d)", maxAvailabilityNames, len(names))
		}
		res, err := d.Client.CheckAvailability(ctx, names, in.Currency)
		if err != nil {
			return nil, CheckDomainAvailabilityOutput{}, err
		}
		out := CheckDomainAvailabilityOutput{Results: []AvailabilityOut{}}
		for _, a := range res {
			o := AvailabilityOut{
				DomainName:    a.DomainName,
				IsAvailable:   a.IsAvailable,
				RegisterPrice: string(a.RegisterPrice),
				RenewPrice:    string(a.RenewPrice),
				CheckingError: a.CheckingError,
			}
			if a.Status != nil {
				o.Status = *a.Status
			}
			out.Results = append(out.Results, o)
		}
		return nil, out, nil
	})
}
