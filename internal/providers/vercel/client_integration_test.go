//go:build integration

package vercel

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestClientListDeploymentsIntegration(t *testing.T) {
	token := os.Getenv("VERCEL_TOKEN")
	if token == "" {
		t.Skip("VERCEL_TOKEN is not set")
	}

	projectID := os.Getenv("VERCEL_PROJECT_ID")
	if projectID == "" {
		t.Skip("VERCEL_PROJECT_ID is not set")
	}

	var opts []Option
	if teamID := os.Getenv("VERCEL_TEAM_ID"); teamID != "" {
		opts = append(opts, WithTeamID(teamID))
	}

	client, err := NewClient(token, opts...)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if _, err := client.ListDeployments(ctx, projectID); err != nil {
		t.Fatalf("ListDeployments() error = %v", err)
	}
}
