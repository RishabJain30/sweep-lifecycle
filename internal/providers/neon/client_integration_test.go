//go:build integration

package neon

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestClientListBranchesIntegration(t *testing.T) {
	token := os.Getenv("NEON_API_KEY")
	if token == "" {
		t.Skip("NEON_API_KEY is not set")
	}

	projectID := os.Getenv("NEON_PROJECT_ID")
	if projectID == "" {
		t.Skip("NEON_PROJECT_ID is not set")
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

	branches, err := client.ListBranches(ctx, projectID)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}

	if len(branches) == 0 {
		t.Fatal("ListBranches() returned no branches")
	}

	var defaultBranchFound bool

	for _, branch := range branches {
		if !branch.Default {
			continue
		}

		defaultBranchFound = true

		if branch.Name != "production" {
			t.Fatalf(
				"default branch name = %q, want %q",
				branch.Name,
				"production",
			)
		}

		if !branch.IsProtected() {
			t.Fatal("default branch is not protected by Sweep")
		}
	}

	if !defaultBranchFound {
		t.Fatal("default branch was not found")
	}
}
