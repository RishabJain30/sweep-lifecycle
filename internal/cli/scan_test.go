package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
	scanservice "github.com/RishabJain30/sweep-lifecycle/internal/scan"
)

func TestScanCommandPrintsCorrelatedResources(t *testing.T) {
	runner := scanRunner(func(
		_ context.Context,
		repository string,
		projectID string,
	) ([]scanservice.Match, error) {
		if repository != "RishabJain30/sweep-lifecycle" {
			t.Fatalf(
				"repository = %q, want %q",
				repository,
				"RishabJain30/sweep-lifecycle",
			)
		}

		if projectID != "test-project" {
			t.Fatalf(
				"project ID = %q, want %q",
				projectID,
				"test-project",
			)
		}

		return []scanservice.Match{
			{
				Branch: domain.DatabaseBranch{
					ID:   "br-preview",
					Name: "preview-pr-1",
				},
				PullRequest: domain.PullRequest{
					Number:         1,
					State:          domain.PullRequestStateMerged,
					HeadRepository: "RishabJain30/sweep-lifecycle",
					HeadBranch:     "feat/example",
				},
				SourceBranchExists: false,
			},
		}, nil
	})

	command := newScanCommand(runner)

	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"--repo", "RishabJain30/sweep-lifecycle",
		"--neon-project", "test-project",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	wantParts := []string{
		"Correlated preview resources",
		"Neon branch: preview-pr-1",
		"Resource ID: br-preview",
		"Associated PR: #1",
		"PR state: merged",
		"PR head repository: RishabJain30/sweep-lifecycle",
		"PR head branch: feat/example",
		"Source branch exists: false",
		"Default: false",
		"Protected: false",
	}

	for _, want := range wantParts {
		if !strings.Contains(output.String(), want) {
			t.Fatalf(
				"output does not contain %q:\n%s",
				want,
				output.String(),
			)
		}
	}
}

func TestScanCommandRequiresRepository(t *testing.T) {
	command := newScanCommand(func(
		context.Context,
		string,
		string,
	) ([]scanservice.Match, error) {
		t.Fatal("scan runner should not be called")
		return nil, nil
	})

	command.SetArgs([]string{
		"--neon-project", "test-project",
	})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "--repo is required") {
		t.Fatalf(
			"error = %q, want missing repository error",
			err,
		)
	}
}

func TestScanCommandRequiresNeonProject(t *testing.T) {
	command := newScanCommand(func(
		context.Context,
		string,
		string,
	) ([]scanservice.Match, error) {
		t.Fatal("scan runner should not be called")
		return nil, nil
	})

	command.SetArgs([]string{
		"--repo", "RishabJain30/sweep-lifecycle",
	})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "--neon-project is required") {
		t.Fatalf(
			"error = %q, want missing Neon project error",
			err,
		)
	}
}
