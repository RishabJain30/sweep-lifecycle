//go:build integration

package github

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

func TestClientGetPullRequestIntegration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN is not set")
	}

	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pr, err := client.GetPullRequest(ctx, "cli/cli", 14343)
	if err != nil {
		t.Fatalf("GetPullRequest() error = %v", err)
	}

	if pr.Repository != "cli/cli" {
		t.Fatalf("Repository = %q, want %q", pr.Repository, "cli/cli")
	}

	if pr.Number != 14343 {
		t.Fatalf("Number = %d, want %d", pr.Number, 14343)
	}

	if pr.State != domain.PullRequestStateMerged {
		t.Fatalf(
			"State = %q, want %q",
			pr.State,
			domain.PullRequestStateMerged,
		)
	}
}