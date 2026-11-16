# Dreamscape Reseller REST API: phase 1 reference

Extracted from the upstream OpenAPI 3.0 spec (https://doc-reseller-api.ds.network/schema/json) and docs page on 2026-11-16. Only the endpoints the phase 1 tools need are listed.

## Servers
- Production: `https://reseller-api.ds.network`
- Sandbox: `https://reseller-api.sandbox.ds.network`

## Auth headers (every request)
- `Api-Request-Id`: hex MD5, unique per request
- `Api-Signature`: hex `md5(requestID + apiKey)` (plain concatenation)

## Envelope
Every 200 response is `{"status": bool, "data": <payload>}`; list endpoints add `"pagination": {"total_items": int, "total_pages": int, "current_page": int}`.
Common query params on lists: `page` (int, default 1), `limit` (int, default 50, max 100). Array query params use the `name[]=` form (e.g. `tlds[]=com.au&tlds[]=com`).
Error codes: 400, 401, 402 (insufficient funds), 403, 404, 500. Error body shape is not documented: parse JSON leniently and surface any `message`/`error`/`errors` field, else the raw body (truncated).

## Reference data
Domain statuses (`status_id`): 1 Awaiting Registration, 2 Registered, 3 Awaiting Renewal, 4 Awaiting Deleted, 5 Deleted, 6 Expired.
Customer statuses (`status_id`): 1 Active (others undocumented here; return raw id if unknown).

## Endpoints

### GET /domains
Query: `domain_name` (string), `customer_id` (int), `admin_contact_id` (int), `billing_contact_id` (int), `tech_contact_id` (int), `status_id` (int), `tlds[]` (string[]), `page`, `limit`.
Data: `[Domain]` + pagination.

### GET /domains/{domain_id}
Data: `Domain`.

`Domain`:
```
id                 int
domain_name        string
auth_key           string   // EPP transfer code: MUST be stripped, never returned by tools
status_id          int
period             int
start_date         string (date-time)
expiry_date        string (date-time)
customer_id        int
registrant_id      int
admin_contact_id   int
billing_contact_id int
tech_contact_id    int
name_servers       [{host string, ip string}]   // ip may be IPv4, IPv6, or both comma-separated
eligibility_data   [{name string, value string}] // null for TLDs without eligibility
privacy            bool
is_locked          bool
```

### GET /domains/{domain_id}/dns
Query: `type` (string, optional record type filter).
Data: `[DnsRecord]` (response schema not spelled out upstream; assume list in `data`). Each record has `id` (int), `type` (string) plus type-specific fields:
- A / AAAA / TXT / CNAME / ALIAS: `subdomain`, `content`
- MX: `subdomain`, `content`, `priority`
- CAA: `flag`, `tag`, `value`
- SRV: `subdomain`, `weight`, `port`, `target`, `priority`
- WEBFWD: `subdomain`, `forward_to`, `cloak`
- MAILFWD: `email`, `forward_to`
Decode into a struct with all optional fields (or `map[string]any`).

### GET /domains/availability
Query: `domain_names[]` (string[], required), `currency` (string), `customer_id` (int).
Data: `[{domain_name string, checking_error string, is_available bool, status "regular"|"premium"|"resale"|"reserved"|null, register_price number, renew_price number}]`.

### GET /domains/transfers/availability
Query: `domain_name` (string, required), `auth_key` (string), `period` (int).
Data: `{status bool, eligible_for_renewal bool, renewal_required bool, is_premium bool, renew_price number, expiry_date string, allowed_renewal_periods [int]}`.
Tool must not require or echo an `auth_key`.

### GET /domains/tlds
Query: `tlds[]` (string[]), `currency` (string), `customer_id` (int), `page`, `limit`.
Data: `[{tld string, min_period int, max_period int, price {register string, renew string, transfer string}, eligibility_required bool, privacy_allowed bool}]` + pagination. Prices are strings holding decimals.

### GET /domains/registrants/{registrant_id}
Data: `{id, domain_id, customer_id, name, first_name, last_name, address, city, country, state, post_code, country_code int, phone, email, account_type, business_name, business_number_type, business_number}`.

### GET /customers
Query: `status_id` (int), `page`, `limit`.
Data: `[Customer]` + pagination.

### GET /customers/{customer_id}
Data: `Customer`: `{id, status_id, username, first_name, last_name, address, city, country, state, post_code, country_code int, phone, mobile, email, currency, account_type, business_name, business_number_type, business_number, date_added (date-time)}`.

### GET /finances/balance
Data: `{balance number, currency string}`.
