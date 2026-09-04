package cli

import (
	"context"
	"strings"
	"testing"

	scanservice "github.com/RishabJain30/sweep-lifecycle/internal/scan"
)

// These tests exercise the real provider wiring's credential-validation
// paths without making network calls: NewClient validates its token before
// any request is sent, so an unset/empty credential fails fast with a
// clear, provider-specific error.

func TestRunScanRequiresGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("NEON_API_KEY", "irrelevant")
	t.Setenv("VERCEL_TOKEN", "")

	_, err := runScan(context.Background(), scanservice.Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err == nil {
		t.Fatal("runScan() error = nil, want a GitHub configuration error")
	}

	if !strings.Contains(err.Error(), "GitHub") {
		t.Fatalf("error = %q, want it to mention GitHub", err)
	}
}

func TestRunScanRequiresNeonAPIKey(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "irrelevant")
	t.Setenv("NEON_API_KEY", "")
	t.Setenv("VERCEL_TOKEN", "")

	_, err := runScan(context.Background(), scanservice.Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err == nil {
		t.Fatal("runScan() error = nil, want a Neon configuration error")
	}

	if !strings.Contains(err.Error(), "Neon") {
		t.Fatalf("error = %q, want it to mention Neon", err)
	}
}

func TestNewOptionalVercelClientSkipsWhenTokenUnset(t *testing.T) {
	t.Setenv("VERCEL_TOKEN", "")

	client, err := newOptionalVercelClient()
	if err != nil {
		t.Fatalf("newOptionalVercelClient() error = %v", err)
	}

	if client != nil {
		t.Fatalf("client = %v, want nil when VERCEL_TOKEN is unset", client)
	}
}

func TestNewOptionalVercelClientConfiguresWhenTokenSet(t *testing.T) {
	t.Setenv("VERCEL_TOKEN", "a-token")
	t.Setenv("VERCEL_TEAM_ID", "team_1")

	client, err := newOptionalVercelClient()
	if err != nil {
		t.Fatalf("newOptionalVercelClient() error = %v", err)
	}

	if client == nil {
		t.Fatal("client = nil, want a configured client when VERCEL_TOKEN is set")
	}
}

func TestRunExplainRequiresGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")

	_, err := runExplain(context.Background(), explainTarget{
		Provider:      "neon",
		ResourceID:    "br-preview",
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err == nil {
		t.Fatal("runExplain() error = nil, want a GitHub configuration error")
	}

	if !strings.Contains(err.Error(), "GitHub") {
		t.Fatalf("error = %q, want it to mention GitHub", err)
	}
}

func TestRunExplainRequiresVercelTokenForVercelResource(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "irrelevant")
	t.Setenv("VERCEL_TOKEN", "")

	_, err := runExplain(context.Background(), explainTarget{
		Provider:   "vercel",
		ResourceID: "dpl_123",
		Repository: "RishabJain30/sweep-lifecycle",
	})
	if err == nil {
		t.Fatal("runExplain() error = nil, want a VERCEL_TOKEN error")
	}

	if !strings.Contains(err.Error(), "VERCEL_TOKEN") {
		t.Fatalf("error = %q, want it to mention VERCEL_TOKEN", err)
	}
}
