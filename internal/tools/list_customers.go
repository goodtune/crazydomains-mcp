package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListCustomersInput is the input of list_customers.
type ListCustomersInput struct {
	StatusID int `json:"status_id,omitempty" jsonschema:"only customers with this status ID (1 = Active)"`
	Paging
}

// ListCustomersOutput is the output of list_customers.
type ListCustomersOutput struct {
	Customers []CustomerOut `json:"customers"`
	PageInfo
}

func addListCustomers(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_customers",
		Description: "List customers of the Crazy Domains (Dreamscape) reseller account: id, name, organisation, email, phone and created date.",
		Annotations: readOnly("List customers"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListCustomersInput) (*mcp.CallToolResult, ListCustomersOutput, error) {
		page, limit, err := in.normalise()
		if err != nil {
			return nil, ListCustomersOutput{}, err
		}
		cs, pg, err := d.Client.ListCustomers(ctx, in.StatusID, page, limit)
		if err != nil {
			return nil, ListCustomersOutput{}, err
		}
		out := ListCustomersOutput{Customers: []CustomerOut{}, PageInfo: pageInfo(page, limit, len(cs), pg)}
		for _, c := range cs {
			out.Customers = append(out.Customers, customerOut(c))
		}
		return nil, out, nil
	})
}
