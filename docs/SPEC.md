# Spec: Crazy Domains (Dreamscape) Reseller MCP server in Go

## Goal

Build an MCP server in Go that lets an AI assistant query Gary's Crazy Domains reseller account through the Dreamscape Reseller REST API. Phase 1 is **read-only**: answer questions like "who owns sydneymets.com.au, when does it expire, and which nameservers is it on?". Write operations (renewals, transfers, DNS edits) are out of scope for phase 1 and must not be reachable unless explicitly enabled later (see Phase 2).

## Upstream API

- Docs: https://doc-reseller-api.ds.network
- OpenAPI 3.0 spec: https://doc-reseller-api.ds.network/schema/json (182 operations)
- Base URLs:
  - Production: `https://reseller-api.ds.network`
  - Sandbox: `https://reseller-api.sandbox.ds.network` (isolated; no real registrations or charges; sandbox API key may differ from production)
- Responses are always `application/json`. Pagination max 100 items per page (`page`, `limit`).
- Status codes: 200, 201, 204, 400, 401, 402 (insufficient funds), 403, 404, 500. Rate limits are undocumented.

### Authentication (every request)

1. API key: 32 lowercase alphanumeric chars, from Reseller Console → Account Settings → API & WHMCS → API Setup.
2. `Api-Request-Id`: a hex MD5 string, unique per request (e.g. `md5(16 random bytes)`).
3. `Api-Signature`: `hex(md5(requestID + apiKey))`, plain string concatenation, no separator.
4. Send both as HTTP headers.

401 usually means: wrong key, stray whitespace in the key, or a sandbox/production key mismatch.

## Implementation requirements

### Language and libraries

- Go (current stable). Module path: `github.com/goodtune/crazydomains-mcp` (adjust if preferred).
- MCP: official Go SDK `github.com/modelcontextprotocol/go-sdk`.
- API client: hand-written, small and typed, covering only the phase 1 endpoints. Request/response shapes are in `docs/API_REFERENCE.md` (extracted from the upstream OpenAPI 3.0 spec; the full spec is not reachable from the build environment). Keep it structured so it can be swapped for an `oapi-codegen` client later.
- Auth: implement as an `http.RoundTripper` that sets the two headers on every request. Never log the API key or signature.

### Configuration (environment variables)

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DREAMSCAPE_API_KEY` | yes | — | Trim whitespace; fail fast at startup if not 32 lowercase alphanumerics |
| `DREAMSCAPE_ENV` | no | `production` | `production` or `sandbox`; selects base URL |
| `DREAMSCAPE_BASE_URL` | no | — | Override base URL (for tests) |
| `MCP_TRANSPORT` | no | `stdio` | `stdio` or `http` (streamable HTTP) |
| `MCP_HTTP_ADDR` | no | `:8080` | Only for `http` transport |
| `MCP_HTTP_BEARER_TOKEN` | required if `http` | — | Require `Authorization: Bearer <token>` on every HTTP request; refuse to start in `http` mode without it |

### Transports

- `stdio` for local use (Claude Code / desktop).
- Streamable HTTP for hosting as a remote connector. Must be authenticated (bearer token above); never expose the reseller API unauthenticated.

### Behaviour common to all tools

- HTTP client timeout 20 s; respect context cancellation.
- Retry once on network errors and 5xx with a short backoff; never retry 4xx.
- Map upstream errors to clear tool errors: 401 → "authentication failed (check key / sandbox vs production)", 404 → "not found", 402 → "insufficient reseller balance", others → status + upstream message.
- **Redaction:** never return `auth_key` (EPP transfer code) in any tool output. Strip it at the client layer, not just in formatting. Never return the API key.
- Return structured JSON results (and a short human-readable summary is fine). Dates as ISO 8601 (`YYYY-MM-DD`).
- Pagination: tools accept `page` and `limit` (max 100) and return `page`, `limit`, `total` (if provided) and `has_more`.
- Map `status_id` to a readable `status` string where the API provides a reference list; otherwise return the raw ID.

## Phase 1 tools (read-only)

All annotated as read-only (`readOnlyHint: true`).

### `list_domains`
Wraps `GET /domains`.
- Inputs (all optional): `domain_name` (string; substring or exact as the API supports), `customer_id` (int), `status_id` (int), `tlds` (string[]), `expiring_within_days` (int, filtered client-side on `expiry_date`), `page`, `limit`.
- Output per domain: `id`, `domain_name`, `status`, `start_date`, `expiry_date`, `customer_id`, `privacy`, `is_locked`.

### `get_domain`
Wraps `GET /domains/{domain_id}`; accepts either `domain_id` or `domain_name` (resolve the name via `GET /domains?domain_name=` and require exactly one match, else error listing candidates).
- Output: `id`, `domain_name`, `status`, `period`, `start_date`, `expiry_date`, `days_until_expiry` (computed), `name_servers` (`host`, `ip`), `eligibility_data` (`name`/`value` pairs, e.g. .au registrant name/ID/type), `privacy`, `is_locked`, `customer_id`, `registrant_id`, contact IDs.
- Option `resolve_parties` (bool, default `true`): also fetch and embed the customer (`GET /customers/{id}`) and registrant (`GET /domains/registrants/{id}`) summaries (name, organisation, email).

### `list_customers` / `get_customer`
Wrap `GET /customers` and `GET /customers/{customer_id}`. Return id, name, organisation, email, phone, created date. Do not expose passwords or login links. **Do not** implement `POST /customers/{id}/login-link` in phase 1.

### `get_registrant`
Wraps `GET /domains/registrants/{registrant_id}`.

### `list_domain_dns_records`
Wraps `GET /domains/{domain_id}/dns` (accepts `domain_id` or `domain_name`). Note in the description that most of Gary's domains use DigitalOcean DNS, so this is often empty; include the domain's nameservers in the output so the caller can tell.

### `check_domain_availability`
Wraps `GET /domains/availability` with `domain_names[]` (1–20 names). Read-only, no charge.

### `check_transfer_availability`
Wraps `GET /domains/transfers/availability`. Read-only.

### `get_account_balance`
Wraps the balance endpoint under financial data (find it in the spec). Read-only.

### `list_tlds`
Wraps `GET /domains/tlds` (with pricing). Read-only.

## Phase 2 (do not build yet; design for it)

Write tools (`renew_domain` → `POST /domains/{id}/renewal`, `update_domain_nameservers` → `PATCH /domains/{id}`, DNS record create/update/delete, transfers) must:
- Only be registered when `DREAMSCAPE_ENABLE_WRITES=true`.
- Carry `destructiveHint: true`.
- Require a `confirm: true` argument and echo back exactly what will happen (domain, period, cost from `list_tlds` pricing) when `confirm` is absent, without calling the API.
- Default to sandbox unless `DREAMSCAPE_ENV=production` is set explicitly.

Keep the client layer generic enough that these are thin additions.

## Project layout

```
cmd/crazydomains-mcp/main.go   # config, transport selection, server start
internal/api/                  # generated client + auth editor + redaction + retries
internal/tools/                # one file per tool, input/output structs
docs/SPEC.md, docs/API_REFERENCE.md
Makefile                       # generate, test, lint, build
README.md                      # setup, env vars, connecting to Claude
```

## Testing

- Unit tests for the signer with a fixed request ID and key (assert exact header values).
- `httptest` server tests for each tool: happy path, pagination, 401, 404, 402, 500-then-retry, redaction of `auth_key` (assert it never appears in output).
- Name resolution tests for `get_domain` (0, 1, many matches).
- Optional integration tests against the sandbox, gated on `DREAMSCAPE_SANDBOX_API_KEY` being set; skipped otherwise.
- `go vet`, `staticcheck` and `golangci-lint` clean.

## Acceptance criteria

1. `go build ./...`, `go vet ./...` and `go test ./...` pass.
2. Started with stdio, the server lists the phase 1 tools only, all marked read-only.
3. Against production, `get_domain` with `domain_name: "sydneymets.com.au"` returns the expiry date, status, nameservers (expected: ns1/ns2/ns3.digitalocean.com), the eligibility data (registrant name: Sydney Mets Touch Association Incorporated), and the resolved customer. It includes no `auth_key`.
4. `list_domains` with `expiring_within_days: 90` returns only domains expiring in that window.
5. HTTP mode refuses to start without `MCP_HTTP_BEARER_TOKEN` and rejects requests without it.
6. README documents setup, env vars, and how to add it to Claude Code (`claude mcp add`) and as a remote connector.
