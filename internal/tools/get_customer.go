package tools

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetCustomerInput is the input of get_customer.
type GetCustomerInput struct {
	CustomerID int `json:"customer_id" jsonschema:"customer ID"`
}

func addGetCustomer(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_customer",
		Description: "Get one reseller customer by ID: name, organisation, email, phone and created date.",
		Annotations: readOnly("Get customer"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetCustomerInput) (*mcp.CallToolResult, CustomerOut, error) {
		if in.CustomerID <= 0 {
			return nil, CustomerOut{}, errors.New("customer_id must be a positive integer")
		}
		c, err := d.Client.GetCustomer(ctx, in.CustomerID)
		if err != nil {
			return nil, CustomerOut{}, err
		}
		return nil, customerOut(*c), nil
	})
}
