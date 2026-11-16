package tools

import (
	"context"
	"strings"

	"github.com/goodtune/crazydomains-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListTLDsInput is the input of list_tlds.
type ListTLDsInput struct {
	TLDs     []string `json:"tlds,omitempty" jsonschema:"only these TLDs, e.g. [\"com.au\", \"net\"] (no leading dot)"`
	Currency string   `json:"currency,omitempty" jsonschema:"currency for prices, e.g. AUD (default: the account's currency)"`
	Paging
}

// TLDOut is one TLD in list_tlds output. Prices are per year, as decimal
// strings.
type TLDOut struct {
	TLD                 string `json:"tld"`
	MinPeriod           int    `json:"min_period"`
	MaxPeriod           int    `json:"max_period"`
	RegisterPrice       string `json:"register_price"`
	RenewPrice          string `json:"renew_price"`
	TransferPrice       string `json:"transfer_price"`
	EligibilityRequired bool   `json:"eligibility_required"`
	PrivacyAllowed      bool   `json:"privacy_allowed"`
}

// ListTLDsOutput is the output of list_tlds.
type ListTLDsOutput struct {
	TLDs []TLDOut `json:"tlds"`
	PageInfo
}

func addListTLDs(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_tlds",
		Description: "List TLDs available to the reseller account with register/renew/transfer prices, allowed registration periods and whether eligibility data is required.",
		Annotations: readOnly("List TLDs"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListTLDsInput) (*mcp.CallToolResult, ListTLDsOutput, error) {
		page, limit, err := in.normalise()
		if err != nil {
			return nil, ListTLDsOutput{}, err
		}
		var tlds []string
		for _, t := range in.TLDs {
			if t = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(t)), "."); t != "" {
				tlds = append(tlds, t)
			}
		}
		res, pg, err := d.Client.ListTLDs(ctx, api.ListTLDsParams{TLDs: tlds, Currency: in.Currency, Page: page, Limit: limit})
		if err != nil {
			return nil, ListTLDsOutput{}, err
		}
		out := ListTLDsOutput{TLDs: []TLDOut{}, PageInfo: pageInfo(page, limit, len(res), pg)}
		for _, t := range res {
			out.TLDs = append(out.TLDs, TLDOut{
				TLD:                 t.TLD,
				MinPeriod:           t.MinPeriod,
				MaxPeriod:           t.MaxPeriod,
				RegisterPrice:       string(t.Price.Register),
				RenewPrice:          string(t.Price.Renew),
				TransferPrice:       string(t.Price.Transfer),
				EligibilityRequired: t.EligibilityRequired,
				PrivacyAllowed:      t.PrivacyAllowed,
			})
		}
		return nil, out, nil
	})
}
