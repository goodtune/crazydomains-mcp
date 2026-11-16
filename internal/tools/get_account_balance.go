package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetAccountBalanceInput is the (empty) input of get_account_balance.
type GetAccountBalanceInput struct{}

// GetAccountBalanceOutput is the output of get_account_balance.
type GetAccountBalanceOutput struct {
	Balance  string `json:"balance" jsonschema:"available reseller balance as a decimal string"`
	Currency string `json:"currency"`
}

func addGetAccountBalance(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_account_balance",
		Description: "Get the Crazy Domains (Dreamscape) reseller account's available balance and currency.",
		Annotations: readOnly("Get account balance"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ GetAccountBalanceInput) (*mcp.CallToolResult, GetAccountBalanceOutput, error) {
		b, err := d.Client.GetBalance(ctx)
		if err != nil {
			return nil, GetAccountBalanceOutput{}, err
		}
		return nil, GetAccountBalanceOutput{Balance: string(b.Balance), Currency: b.Currency}, nil
	})
}
