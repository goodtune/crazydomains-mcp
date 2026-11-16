package api

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestSandbox runs read-only calls against the Dreamscape sandbox. It is
// skipped unless DREAMSCAPE_SANDBOX_API_KEY is set.
func TestSandbox(t *testing.T) {
	key := strings.TrimSpace(os.Getenv("DREAMSCAPE_SANDBOX_API_KEY"))
	if key == "" {
		t.Skip("DREAMSCAPE_SANDBOX_API_KEY not set")
	}
	c, err := New(SandboxBaseURL, key)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := c.GetBalance(ctx); err != nil {
		t.Errorf("GetBalance: %v", err)
	}
	if _, _, err := c.ListDomains(ctx, ListDomainsParams{Limit: 5}); err != nil {
		t.Errorf("ListDomains: %v", err)
	}
	if _, _, err := c.ListTLDs(ctx, ListTLDsParams{TLDs: []string{"com.au"}}); err != nil {
		t.Errorf("ListTLDs: %v", err)
	}
	if _, err := c.CheckAvailability(ctx, []string{"example.com"}, ""); err != nil {
		t.Errorf("CheckAvailability: %v", err)
	}
}
