package config

import (
	"strings"
	"testing"

	"github.com/goodtune/crazydomains-mcp/internal/api"
)

const key = "abcdefghijklmnopqrstuvwxyz012345"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"DREAMSCAPE_API_KEY": "  " + key + "\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.APIKey != key || c.Env != "production" || c.BaseURL != api.ProductionBaseURL || c.Transport != TransportStdio {
		t.Fatalf("config = %+v", c)
	}
}

func TestLoadSandboxAndOverride(t *testing.T) {
	c, err := Load(env(map[string]string{"DREAMSCAPE_API_KEY": key, "DREAMSCAPE_ENV": "Sandbox"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL != api.SandboxBaseURL {
		t.Errorf("BaseURL = %s", c.BaseURL)
	}
	c, err = Load(env(map[string]string{"DREAMSCAPE_API_KEY": key, "DREAMSCAPE_BASE_URL": "http://127.0.0.1:9999"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL != "http://127.0.0.1:9999" {
		t.Errorf("BaseURL = %s", c.BaseURL)
	}
}

func TestLoadHTTP(t *testing.T) {
	c, err := Load(env(map[string]string{"DREAMSCAPE_API_KEY": key, "MCP_TRANSPORT": "http", "MCP_HTTP_BEARER_TOKEN": "s3cret"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.BearerToken != "s3cret" {
		t.Errorf("config = %+v", c)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{}, "DREAMSCAPE_API_KEY is required"},
		{map[string]string{"DREAMSCAPE_API_KEY": "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"}, "32 lowercase"},
		{map[string]string{"DREAMSCAPE_API_KEY": key[:31]}, "32 lowercase"},
		{map[string]string{"DREAMSCAPE_API_KEY": key[:16] + " " + key[17:]}, "32 lowercase"},
		{map[string]string{"DREAMSCAPE_API_KEY": key, "DREAMSCAPE_ENV": "staging"}, "production or sandbox"},
		{map[string]string{"DREAMSCAPE_API_KEY": key, "MCP_TRANSPORT": "sse"}, "stdio or http"},
		{map[string]string{"DREAMSCAPE_API_KEY": key, "MCP_TRANSPORT": "http"}, "MCP_HTTP_BEARER_TOKEN is required"},
		{map[string]string{"DREAMSCAPE_API_KEY": key, "MCP_TRANSPORT": "http", "MCP_HTTP_BEARER_TOKEN": "   "}, "MCP_HTTP_BEARER_TOKEN is required"},
	}
	for _, tc := range cases {
		_, err := Load(env(tc.env))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("env %v: err = %v, want %q", tc.env, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), key[:20]) {
			t.Errorf("error leaks API key: %v", err)
		}
	}
}
