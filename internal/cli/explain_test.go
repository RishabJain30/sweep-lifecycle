package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
	"github.com/RishabJain30/sweep-lifecycle/internal/evidence"
	scanservice "github.com/RishabJain30/sweep-lifecycle/internal/scan"
	"github.com/RishabJain30/sweep-lifecycle/internal/scoring"
)

func TestParseExplainTarget(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		wantProvider   string
		wantResourceID string
		wantErr        bool
	}{
		{
			name:           "neon resource",
			raw:            "neon:br-preview",
			wantProvider:   "neon",
			wantResourceID: "br-preview",
		},
		{
			name:           "vercel resource",
			raw:            "vercel:dpl_123",
			wantProvider:   "vercel",
			wantResourceID: "dpl_123",
		},
		{
			name:    "missing separator",
			raw:     "br-preview",
			wantErr: true,
		},
		{
			name:    "unsupported provider",
			raw:     "aws:i-123",
			wantErr: true,
		},
		{
			name:    "empty resource id",
			raw:     "neon:",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, resourceID, err := parseExplainTarget(test.raw)

			if test.wantErr {
				if err == nil {
					t.Fatal("err = nil, want an error")
				}

				return
			}

			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}

			if provider != test.wantProvider {
				t.Fatalf(
					"provider = %q, want %q",
					provider,
					test.wantProvider,
				)
			}

			if resourceID != test.wantResourceID {
				t.Fatalf(
					"resourceID = %q, want %q",
					resourceID,
					test.wantResourceID,
				)
			}
		})
	}
}

func sampleCandidate() scanservice.Candidate {
	return scanservice.Candidate{
		Provider:     "neon",
		ResourceID:   "br-preview",
		ResourceName: "preview-pr-1",
		PullRequest: domain.PullRequest{
			Number:         1,
			State:          domain.PullRequestStateMerged,
			HeadRepository: "RishabJain30/sweep-lifecycle",
			HeadBranch:     "feat/example",
		},
		PullRequestFound:    true,
		SourceBranchChecked: true,
		SourceBranchExists:  false,
		Evidence: []evidence.Item{
			{
				Kind:        evidence.KindPullRequestMerged,
				Description: "The correlated pull request has been merged.",
			},
		},
		Score: scoring.Result{
			PolicyVersion:  scoring.PolicyVersion,
			Score:          78,
			Confidence:     scoring.ConfidenceHigh,
			Recommendation: "Strong cleanup candidate: finished pull request and missing source branch.",
			Contributions: []scoring.Contribution{
				{
					Kind:        evidence.KindPullRequestMerged,
					Description: "The correlated pull request has been merged.",
					Points:      30,
				},
			},
		},
	}
}

func TestExplainCommandPrintsReport(t *testing.T) {
	runner := explainRunner(func(
		_ context.Context,
		target explainTarget,
	) (scanservice.Candidate, error) {
		if target.Provider != "neon" {
			t.Fatalf("provider = %q, want %q", target.Provider, "neon")
		}

		if target.ResourceID != "br-preview" {
			t.Fatalf(
				"resource ID = %q, want %q",
				target.ResourceID,
				"br-preview",
			)
		}

		if target.Repository != "RishabJain30/sweep-lifecycle" {
			t.Fatalf(
				"repository = %q, want %q",
				target.Repository,
				"RishabJain30/sweep-lifecycle",
			)
		}

		if target.NeonProjectID != "test-project" {
			t.Fatalf(
				"Neon project = %q, want %q",
				target.NeonProjectID,
				"test-project",
			)
		}

		return sampleCandidate(), nil
	})

	command := newExplainCommand(runner)

	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"neon:br-preview",
		"--repo", "RishabJain30/sweep-lifecycle",
		"--neon-project", "test-project",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	wantParts := []string{
		"Resource: neon br-preview (preview-pr-1)",
		"Pull request: #1 (merged)",
		"PR head repository: RishabJain30/sweep-lifecycle",
		"PR head branch: feat/example",
		"Source branch exists: false",
		"Score: 78/100",
		"Confidence: HIGH",
		"Recommendation: Strong cleanup candidate",
		"[+30] The correlated pull request has been merged.",
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

func TestExplainCommandShowsExclusionReason(t *testing.T) {
	runner := explainRunner(func(
		context.Context,
		explainTarget,
	) (scanservice.Candidate, error) {
		candidate := sampleCandidate()
		candidate.Score.Excluded = true
		candidate.Score.ExclusionReason = "resource is marked default, protected, or production-like"

		return candidate, nil
	})

	command := newExplainCommand(runner)

	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"neon:br-prod",
		"--repo", "RishabJain30/sweep-lifecycle",
		"--neon-project", "test-project",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(output.String(), "Excluded: resource is marked default, protected, or production-like") {
		t.Fatalf(
			"output does not contain the exclusion reason:\n%s",
			output.String(),
		)
	}
}

func TestExplainCommandRequiresRepository(t *testing.T) {
	command := newExplainCommand(func(
		context.Context,
		explainTarget,
	) (scanservice.Candidate, error) {
		t.Fatal("explain runner should not be called")
		return scanservice.Candidate{}, nil
	})

	command.SetArgs([]string{
		"neon:br-preview",
		"--neon-project", "test-project",
	})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "--repo is required") {
		t.Fatalf("error = %q, want missing repository error", err)
	}
}

func TestExplainCommandRejectsMalformedRepository(t *testing.T) {
	command := newExplainCommand(func(
		context.Context,
		explainTarget,
	) (scanservice.Candidate, error) {
		t.Fatal("explain runner should not be called")
		return scanservice.Candidate{}, nil
	})

	command.SetArgs([]string{
		"neon:br-preview",
		"--repo", "not-a-valid-repo",
		"--neon-project", "test-project",
	})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "owner/name") {
		t.Fatalf("error = %q, want an owner/name format error", err)
	}
}

func TestExplainCommandRequiresNeonProjectForNeonResource(t *testing.T) {
	command := newExplainCommand(func(
		context.Context,
		explainTarget,
	) (scanservice.Candidate, error) {
		t.Fatal("explain runner should not be called")
		return scanservice.Candidate{}, nil
	})

	command.SetArgs([]string{
		"neon:br-preview",
		"--repo", "RishabJain30/sweep-lifecycle",
	})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "--neon-project is required") {
		t.Fatalf("error = %q, want missing Neon project error", err)
	}
}

func TestExplainCommandDoesNotRequireNeonProjectForVercelResource(t *testing.T) {
	runner := explainRunner(func(
		context.Context,
		explainTarget,
	) (scanservice.Candidate, error) {
		candidate := sampleCandidate()
		candidate.Provider = "vercel"

		return candidate, nil
	})

	command := newExplainCommand(runner)

	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"vercel:dpl_123",
		"--repo", "RishabJain30/sweep-lifecycle",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want success", err)
	}
}

func TestExplainCommandRejectsMalformedTarget(t *testing.T) {
	command := newExplainCommand(func(
		context.Context,
		explainTarget,
	) (scanservice.Candidate, error) {
		t.Fatal("explain runner should not be called")
		return scanservice.Candidate{}, nil
	})

	command.SetArgs([]string{
		"br-preview",
		"--repo", "RishabJain30/sweep-lifecycle",
	})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "provider:resource-id") {
		t.Fatalf("error = %q, want a format error", err)
	}
}

func TestExplainCommandRequiresExactlyOneArgument(t *testing.T) {
	command := newExplainCommand(func(
		context.Context,
		explainTarget,
	) (scanservice.Candidate, error) {
		t.Fatal("explain runner should not be called")
		return scanservice.Candidate{}, nil
	})

	command.SetArgs([]string{})

	if err := command.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error for no arguments")
	}
}
