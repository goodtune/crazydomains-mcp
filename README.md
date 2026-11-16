# crazydomains-mcp

A read-only [MCP](https://modelcontextprotocol.io) server for a Crazy Domains reseller account, built on the
[Dreamscape Reseller REST API](https://doc-reseller-api.ds.network). It lets an AI assistant answer questions such as
"who owns sydneymets.com.au, when does it expire, and which nameservers is it on?".

This is phase 1: **read-only tools only**. Nothing here can register, renew, transfer or change DNS. See
[docs/SPEC.md](docs/SPEC.md) for the full spec and [docs/API_REFERENCE.md](docs/API_REFERENCE.md) for the upstream API
shapes used.

## Tools

All tools are annotated `readOnlyHint: true`. Dates are `YYYY-MM-DD`.

| Tool | Upstream | What it does |
|---|---|---|
| `list_domains` | `GET /domains` | List domains with status, start/expiry dates, customer, privacy and lock. Filters: `domain_name`, `customer_id`, `status_id`, `tlds`, `expiring_within_days` (scans every page, sorts by expiry). Paginated. |
| `get_domain` | `GET /domains/{id}` | One domain by `domain_id` or `domain_name` (the name must match exactly one domain): nameservers, eligibility data, contact IDs, `days_until_expiry`, and (by default) the customer and registrant summaries. |
| `list_customers` | `GET /customers` | Customers: id, name, organisation, email, phone, created date. Paginated. |
| `get_customer` | `GET /customers/{id}` | One customer. |
| `get_registrant` | `GET /domains/registrants/{id}` | One registrant: name, organisation, business number, contact details, address. |
| `list_domain_dns_records` | `GET /domains/{id}/dns` | Records hosted on Dreamscape DNS, plus the domain's nameservers. Often empty, because most domains are delegated to DigitalOcean DNS. |
| `check_domain_availability` | `GET /domains/availability` | Availability and prices for 1-20 names. No charge. |
| `check_transfer_availability` | `GET /domains/transfers/availability` | Whether a domain can be transferred in, with renewal requirements and price. No EPP code is needed or returned. |
| `get_account_balance` | `GET /finances/balance` | Reseller balance and currency. |
| `list_tlds` | `GET /domains/tlds` | TLDs with register/renew/transfer prices and period limits. Paginated. |

List tools take `page` (default 1) and `limit` (default 50, max 100) and return `page`, `limit`, `total` (when the API
reports it) and `has_more`.

## Security properties

- The EPP transfer code (`auth_key`) is stripped from every API response body, at any depth, before decoding, so no tool
  can return it.
- The API key and request signature are never logged or included in tool output or errors.
- Customer usernames, passwords and login links are not exposed; `POST /customers/{id}/login-link` is not implemented.
- HTTP mode refuses to start without a bearer token and rejects any request to `/mcp` without it.

## Setup

1. Get the API key from the Reseller Console: **Account Settings → API & WHMCS → API Setup**. It is 32 lowercase
   letters and digits. The sandbox may use a different key from production.
2. Build or install the binary (Go 1.25 or later):

   ```sh
   go install github.com/goodtune/crazydomains-mcp/cmd/crazydomains-mcp@latest
   # or, from a checkout:
   make build        # writes bin/crazydomains-mcp
   ```

## Configuration

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DREAMSCAPE_API_KEY` | yes | | Whitespace is trimmed. The server refuses to start unless it is 32 lowercase letters/digits. |
| `DREAMSCAPE_ENV` | no | `production` | `production` (`https://reseller-api.ds.network`) or `sandbox` (`https://reseller-api.sandbox.ds.network`). |
| `DREAMSCAPE_BASE_URL` | no | | Overrides the base URL (testing). |
| `MCP_TRANSPORT` | no | `stdio` | `stdio` or `http` (streamable HTTP). |
| `MCP_HTTP_ADDR` | no | `:8080` | Listen address in `http` mode. |
| `MCP_HTTP_BEARER_TOKEN` | in `http` mode | | Required `Authorization: Bearer <token>` value. Use a long random string, e.g. `openssl rand -hex 32`. |

A 401 from the API usually means a wrong key, stray characters in the key, or a sandbox key used against production
(or the reverse).

## Use with Claude Code (stdio)

```sh
claude mcp add \
  --env DREAMSCAPE_API_KEY=your32characterkeyxxxxxxxxxxxxxx \
  --scope user \
  crazydomains -- crazydomains-mcp
```

Add `--env DREAMSCAPE_ENV=sandbox` to point it at the sandbox. Then ask, for example, "when does sydneymets.com.au
expire and what nameservers is it on?" or "which domains expire in the next 90 days?".

## HTTP mode (remote connector)

```sh
DREAMSCAPE_API_KEY=... \
MCP_TRANSPORT=http \
MCP_HTTP_ADDR=:8080 \
MCP_HTTP_BEARER_TOKEN=$(openssl rand -hex 32) \
crazydomains-mcp
```

- The MCP endpoint is `/mcp` and needs `Authorization: Bearer <token>` on every request.
- `/healthz` is unauthenticated and returns only `ok`, for load balancer health checks.
- The server speaks plain HTTP. Put it behind TLS (a reverse proxy or a platform such as DigitalOcean App Platform)
  before exposing it.

Connect Claude Code to a hosted instance:

```sh
claude mcp add --transport http \
  --header "Authorization: Bearer $MCP_HTTP_BEARER_TOKEN" \
  crazydomains https://mcp.example.com/mcp
```

Any client that can send a custom `Authorization` header can connect the same way. This server does not implement
OAuth, so a client that only supports OAuth-based remote connectors needs an authenticating proxy in front of it.

## Development

```sh
make test     # go test ./...
make race     # go test -race ./...
make lint     # gofmt check, go vet, staticcheck and golangci-lint (when installed)
make build
```

Tests run entirely against `httptest` fake upstreams. An optional smoke test against the Dreamscape sandbox runs when
`DREAMSCAPE_SANDBOX_API_KEY` is set:

```sh
DREAMSCAPE_SANDBOX_API_KEY=... go test ./internal/api -run TestSandbox -v
```

### Layout

```
cmd/crazydomains-mcp/   entry point: config, transport selection, HTTP auth wiring
internal/api/           hand-written API client: signer, retries, error mapping, auth_key redaction
internal/tools/         one file per MCP tool, with input/output types
internal/config/        environment configuration
internal/httpauth/      bearer token middleware
docs/                   spec and upstream API reference
```

The API client is hand-written and covers only the phase 1 endpoints. It is small enough to swap for an
`oapi-codegen` client later.

### Phase 2 (not implemented)

Write tools (renewals, nameserver changes, DNS edits, transfers) are planned behind `DREAMSCAPE_ENABLE_WRITES=true`,
with `destructiveHint`, an explicit `confirm: true` argument and a sandbox default. This build has no write tools: if
`DREAMSCAPE_ENABLE_WRITES` is set, the server logs a warning and ignores it.
