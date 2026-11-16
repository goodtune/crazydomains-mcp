package tools

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CheckTransferAvailabilityInput is the input of check_transfer_availability.
type CheckTransferAvailabilityInput struct {
	DomainName string `json:"domain_name" jsonschema:"fully qualified domain name to check for transfer in"`
	Period     int    `json:"period,omitempty" jsonschema:"renewal period in years to price"`
}

// CheckTransferAvailabilityOutput is the output of check_transfer_availability.
type CheckTransferAvailabilityOutput struct {
	DomainName            string `json:"domain_name"`
	Transferable          bool   `json:"transferable" jsonschema:"the API's status flag for the transfer check"`
	EligibleForRenewal    bool   `json:"eligible_for_renewal"`
	RenewalRequired       bool   `json:"renewal_required"`
	IsPremium             bool   `json:"is_premium"`
	RenewPrice            string `json:"renew_price,omitempty"`
	ExpiryDate            string `json:"expiry_date,omitempty"`
	AllowedRenewalPeriods []int  `json:"allowed_renewal_periods"`
}

func addCheckTransferAvailability(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "check_transfer_availability",
		Description: "Check whether a domain can be transferred into the reseller account, with renewal requirements and price. " +
			"Read-only; no transfer is started and no EPP code is needed or returned.",
		Annotations: readOnly("Check transfer availability"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CheckTransferAvailabilityInput) (*mcp.CallToolResult, CheckTransferAvailabilityOutput, error) {
		name := normaliseDomainName(in.DomainName)
		if name == "" {
			return nil, CheckTransferAvailabilityOutput{}, errors.New("domain_name is required")
		}
		if in.Period < 0 {
			return nil, CheckTransferAvailabilityOutput{}, errors.New("period must be positive")
		}
		t, err := d.Client.CheckTransferAvailability(ctx, name, in.Period)
		if err != nil {
			return nil, CheckTransferAvailabilityOutput{}, err
		}
		return nil, CheckTransferAvailabilityOutput{
			DomainName:            name,
			Transferable:          t.Status,
			EligibleForRenewal:    t.EligibleForRenewal,
			RenewalRequired:       t.RenewalRequired,
			IsPremium:             t.IsPremium,
			RenewPrice:            string(t.RenewPrice),
			ExpiryDate:            isoDate(t.ExpiryDate),
			AllowedRenewalPeriods: orEmpty(t.AllowedRenewalPeriods),
		}, nil
	})
}
