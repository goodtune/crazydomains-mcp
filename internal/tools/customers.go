package tools

import (
	"github.com/goodtune/crazydomains-mcp/internal/api"
)

// CustomerOut is a customer in list_customers / get_customer output.
// Usernames, passwords and login links are never exposed.
type CustomerOut struct {
	ID           int    `json:"id"`
	Status       string `json:"status"`
	StatusID     int    `json:"status_id"`
	Name         string `json:"name"`
	Organisation string `json:"organisation,omitempty"`
	AccountType  string `json:"account_type,omitempty"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Mobile       string `json:"mobile,omitempty"`
	Created      string `json:"created,omitempty" jsonschema:"date the customer was added (YYYY-MM-DD)"`
}

func customerOut(c api.Customer) CustomerOut {
	return CustomerOut{
		ID:           c.ID,
		Status:       statusName(customerStatuses, c.StatusID),
		StatusID:     c.StatusID,
		Name:         joinName(c.FirstName, c.LastName),
		Organisation: c.BusinessName,
		AccountType:  c.AccountType,
		Email:        c.Email,
		Phone:        c.Phone,
		Mobile:       c.Mobile,
		Created:      isoDate(c.DateAdded),
	}
}
