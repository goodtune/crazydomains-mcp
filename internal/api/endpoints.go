package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ListDomainsParams are the filters for GET /domains.
type ListDomainsParams struct {
	DomainName string
	CustomerID int
	StatusID   int
	TLDs       []string
	Page       int
	Limit      int
}

// ListDomains wraps GET /domains.
func (c *Client) ListDomains(ctx context.Context, p ListDomainsParams) ([]Domain, *Pagination, error) {
	q := url.Values{}
	if p.DomainName != "" {
		q.Set("domain_name", p.DomainName)
	}
	if p.CustomerID > 0 {
		q.Set("customer_id", strconv.Itoa(p.CustomerID))
	}
	if p.StatusID > 0 {
		q.Set("status_id", strconv.Itoa(p.StatusID))
	}
	for _, t := range p.TLDs {
		q.Add("tlds[]", t)
	}
	var out []Domain
	pg, err := c.get(ctx, "/domains", pageQuery(q, p.Page, p.Limit), &out)
	return out, pg, err
}

// GetDomain wraps GET /domains/{domain_id}.
func (c *Client) GetDomain(ctx context.Context, id int) (*Domain, error) {
	var out Domain
	if _, err := c.get(ctx, fmt.Sprintf("/domains/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDNSRecords wraps GET /domains/{domain_id}/dns. recordType is optional.
func (c *Client) ListDNSRecords(ctx context.Context, domainID int, recordType string) ([]DNSRecord, error) {
	q := url.Values{}
	if recordType != "" {
		q.Set("type", recordType)
	}
	var out []DNSRecord
	_, err := c.get(ctx, fmt.Sprintf("/domains/%d/dns", domainID), q, &out)
	return out, err
}

// GetRegistrant wraps GET /domains/registrants/{registrant_id}.
func (c *Client) GetRegistrant(ctx context.Context, id int) (*Registrant, error) {
	var out Registrant
	if _, err := c.get(ctx, fmt.Sprintf("/domains/registrants/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListCustomers wraps GET /customers.
func (c *Client) ListCustomers(ctx context.Context, statusID, page, limit int) ([]Customer, *Pagination, error) {
	q := url.Values{}
	if statusID > 0 {
		q.Set("status_id", strconv.Itoa(statusID))
	}
	var out []Customer
	pg, err := c.get(ctx, "/customers", pageQuery(q, page, limit), &out)
	return out, pg, err
}

// GetCustomer wraps GET /customers/{customer_id}.
func (c *Client) GetCustomer(ctx context.Context, id int) (*Customer, error) {
	var out Customer
	if _, err := c.get(ctx, fmt.Sprintf("/customers/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckAvailability wraps GET /domains/availability.
func (c *Client) CheckAvailability(ctx context.Context, names []string, currency string) ([]Availability, error) {
	q := url.Values{}
	for _, n := range names {
		q.Add("domain_names[]", n)
	}
	if currency != "" {
		q.Set("currency", currency)
	}
	var out []Availability
	_, err := c.get(ctx, "/domains/availability", q, &out)
	return out, err
}

// CheckTransferAvailability wraps GET /domains/transfers/availability.
// The optional auth_key query parameter is intentionally not supported.
func (c *Client) CheckTransferAvailability(ctx context.Context, name string, period int) (*TransferAvailability, error) {
	q := url.Values{}
	q.Set("domain_name", name)
	if period > 0 {
		q.Set("period", strconv.Itoa(period))
	}
	var out TransferAvailability
	if _, err := c.get(ctx, "/domains/transfers/availability", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListTLDsParams are the filters for GET /domains/tlds.
type ListTLDsParams struct {
	TLDs     []string
	Currency string
	Page     int
	Limit    int
}

// ListTLDs wraps GET /domains/tlds.
func (c *Client) ListTLDs(ctx context.Context, p ListTLDsParams) ([]TLD, *Pagination, error) {
	q := url.Values{}
	for _, t := range p.TLDs {
		q.Add("tlds[]", t)
	}
	if p.Currency != "" {
		q.Set("currency", p.Currency)
	}
	var out []TLD
	pg, err := c.get(ctx, "/domains/tlds", pageQuery(q, p.Page, p.Limit), &out)
	return out, pg, err
}

// GetBalance wraps GET /finances/balance.
func (c *Client) GetBalance(ctx context.Context) (*Balance, error) {
	var out Balance
	if _, err := c.get(ctx, "/finances/balance", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
