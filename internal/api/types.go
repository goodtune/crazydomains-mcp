package api

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Domain is a domain as returned by GET /domains and GET /domains/{id}.
// auth_key is deliberately absent: it is stripped from the raw response
// before decoding (see redact.go) and has no field to land in.
type Domain struct {
	ID               int             `json:"id"`
	DomainName       string          `json:"domain_name"`
	StatusID         int             `json:"status_id"`
	Period           int             `json:"period"`
	StartDate        string          `json:"start_date"`
	ExpiryDate       string          `json:"expiry_date"`
	CustomerID       int             `json:"customer_id"`
	RegistrantID     int             `json:"registrant_id"`
	AdminContactID   int             `json:"admin_contact_id"`
	BillingContactID int             `json:"billing_contact_id"`
	TechContactID    int             `json:"tech_contact_id"`
	NameServers      []NameServer    `json:"name_servers"`
	EligibilityData  []EligibilityKV `json:"eligibility_data"`
	Privacy          bool            `json:"privacy"`
	IsLocked         bool            `json:"is_locked"`
}

// NameServer is a delegated nameserver. IP may hold IPv4, IPv6 or both
// (comma separated).
type NameServer struct {
	Host string `json:"host"`
	IP   string `json:"ip"`
}

// EligibilityKV is a name/value pair of registry eligibility data (e.g. .au
// registrant name, ID and ID type).
type EligibilityKV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DNSRecord is a DNS record hosted on Dreamscape DNS. The upstream schema is
// loosely specified (fields vary by record type), so records are kept as
// generic JSON objects.
type DNSRecord map[string]any

// Registrant is a domain registrant as returned by
// GET /domains/registrants/{id}.
type Registrant struct {
	ID                 int    `json:"id"`
	DomainID           int    `json:"domain_id"`
	CustomerID         int    `json:"customer_id"`
	Name               string `json:"name"`
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	Address            string `json:"address"`
	City               string `json:"city"`
	Country            string `json:"country"`
	State              string `json:"state"`
	PostCode           string `json:"post_code"`
	CountryCode        int    `json:"country_code"`
	Phone              string `json:"phone"`
	Email              string `json:"email"`
	AccountType        string `json:"account_type"`
	BusinessName       string `json:"business_name"`
	BusinessNumberType string `json:"business_number_type"`
	BusinessNumber     string `json:"business_number"`
}

// Customer is a reseller customer as returned by GET /customers and
// GET /customers/{id}. Login credentials are not modelled.
type Customer struct {
	ID                 int    `json:"id"`
	StatusID           int    `json:"status_id"`
	Username           string `json:"username"`
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	Address            string `json:"address"`
	City               string `json:"city"`
	Country            string `json:"country"`
	State              string `json:"state"`
	PostCode           string `json:"post_code"`
	CountryCode        int    `json:"country_code"`
	Phone              string `json:"phone"`
	Mobile             string `json:"mobile"`
	Email              string `json:"email"`
	Currency           string `json:"currency"`
	AccountType        string `json:"account_type"`
	BusinessName       string `json:"business_name"`
	BusinessNumberType string `json:"business_number_type"`
	BusinessNumber     string `json:"business_number"`
	DateAdded          string `json:"date_added"`
}

// Availability is one result of GET /domains/availability.
type Availability struct {
	DomainName    string  `json:"domain_name"`
	CheckingError string  `json:"checking_error"`
	IsAvailable   bool    `json:"is_available"`
	Status        *string `json:"status"`
	RegisterPrice Decimal `json:"register_price"`
	RenewPrice    Decimal `json:"renew_price"`
}

// TransferAvailability is the result of GET /domains/transfers/availability.
type TransferAvailability struct {
	Status                bool    `json:"status"`
	EligibleForRenewal    bool    `json:"eligible_for_renewal"`
	RenewalRequired       bool    `json:"renewal_required"`
	IsPremium             bool    `json:"is_premium"`
	RenewPrice            Decimal `json:"renew_price"`
	ExpiryDate            string  `json:"expiry_date"`
	AllowedRenewalPeriods []int   `json:"allowed_renewal_periods"`
}

// TLD is one result of GET /domains/tlds.
type TLD struct {
	TLD                 string   `json:"tld"`
	MinPeriod           int      `json:"min_period"`
	MaxPeriod           int      `json:"max_period"`
	Price               TLDPrice `json:"price"`
	EligibilityRequired bool     `json:"eligibility_required"`
	PrivacyAllowed      bool     `json:"privacy_allowed"`
}

// TLDPrice holds per-year prices for a TLD.
type TLDPrice struct {
	Register Decimal `json:"register"`
	Renew    Decimal `json:"renew"`
	Transfer Decimal `json:"transfer"`
}

// Balance is the result of GET /finances/balance.
type Balance struct {
	Balance  Decimal `json:"balance"`
	Currency string  `json:"currency"`
}

// Decimal is a monetary amount. The API sends some amounts as JSON numbers
// and others as strings; both decode into the decimal text, which is kept
// verbatim (no float rounding) and re-encoded as a JSON string.
type Decimal string

// UnmarshalJSON accepts a JSON string, number or null.
func (d *Decimal) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		*d = ""
	case len(b) > 0 && b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*d = Decimal(s)
	default:
		var n json.Number
		if err := json.Unmarshal(b, &n); err != nil {
			return fmt.Errorf("decimal: %w", err)
		}
		*d = Decimal(n.String())
	}
	return nil
}
