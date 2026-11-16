// Package config loads server configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/goodtune/crazydomains-mcp/internal/api"
)

// Transports.
const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

// Config is the server configuration.
type Config struct {
	APIKey      string
	Env         string // "production" or "sandbox"
	BaseURL     string // resolved from Env unless overridden
	Transport   string // "stdio" or "http"
	HTTPAddr    string
	BearerToken string
	// WritesRequested is true when DREAMSCAPE_ENABLE_WRITES=true. This build
	// has no write tools, so it only triggers a warning.
	WritesRequested bool
}

var apiKeyRE = regexp.MustCompile(`^[a-z0-9]{32}$`)

// Load reads the configuration using getenv (normally os.Getenv). Errors never
// include the API key or bearer token.
func Load(getenv func(string) string) (*Config, error) {
	c := &Config{
		APIKey:      strings.TrimSpace(getenv("DREAMSCAPE_API_KEY")),
		Env:         strings.ToLower(strings.TrimSpace(getenv("DREAMSCAPE_ENV"))),
		BaseURL:     strings.TrimSpace(getenv("DREAMSCAPE_BASE_URL")),
		Transport:   strings.ToLower(strings.TrimSpace(getenv("MCP_TRANSPORT"))),
		HTTPAddr:    strings.TrimSpace(getenv("MCP_HTTP_ADDR")),
		BearerToken: strings.TrimSpace(getenv("MCP_HTTP_BEARER_TOKEN")),
	}
	c.WritesRequested = strings.EqualFold(strings.TrimSpace(getenv("DREAMSCAPE_ENABLE_WRITES")), "true")

	switch {
	case c.APIKey == "":
		return nil, errors.New("DREAMSCAPE_API_KEY is required")
	case !apiKeyRE.MatchString(c.APIKey):
		return nil, fmt.Errorf("DREAMSCAPE_API_KEY must be 32 lowercase letters or digits (got %d characters)", len(c.APIKey))
	}

	switch c.Env {
	case "", "production":
		c.Env = "production"
		if c.BaseURL == "" {
			c.BaseURL = api.ProductionBaseURL
		}
	case "sandbox":
		if c.BaseURL == "" {
			c.BaseURL = api.SandboxBaseURL
		}
	default:
		return nil, fmt.Errorf("DREAMSCAPE_ENV must be production or sandbox (got %q)", c.Env)
	}

	switch c.Transport {
	case "", TransportStdio:
		c.Transport = TransportStdio
	case TransportHTTP:
		if c.BearerToken == "" {
			return nil, errors.New("MCP_HTTP_BEARER_TOKEN is required when MCP_TRANSPORT=http")
		}
		if c.HTTPAddr == "" {
			c.HTTPAddr = ":8080"
		}
	default:
		return nil, fmt.Errorf("MCP_TRANSPORT must be stdio or http (got %q)", c.Transport)
	}
	return c, nil
}
