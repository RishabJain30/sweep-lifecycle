package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
	"github.com/RishabJain30/sweep-lifecycle/internal/evidence"
	scanservice "github.com/RishabJain30/sweep-lifecycle/internal/scan"
	"github.com/RishabJain30/sweep-lifecycle/internal/scoring"
)

func sampleResult() scanservice.Result {
	return scanservice.Result{
		ProviderStatuses: []scanservice.ProviderStatus{
			{Provider: "neon", Status: "ok", Detail: "discovered 1 branch(es)"},
			{Provider: "vercel", Status: "skipped", Detail: "Vercel is not configured"},
		},
		Candidates: []scanservice.Candidate{
			{
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
				Score: scoring.Result{
					PolicyVersion:  scoring.PolicyVersion,
					Score:          78,
					Confidence:     scoring.ConfidenceHigh,
					Recommended:    true,
					Recommendation: "Strong cleanup candidate: finished pull request and missing source branch.",
					Contributions: []scoring.Contribution{
						{
							Kind:        evidence.KindPullRequestMerged,
							Description: "The correlated pull request has been merged.",
							Points:      30,
						},
						{
							Kind:        evidence.KindSourceBranchMissing,
							Description: "The pull request's source branch no longer exists on GitHub.",
							Points:      35,
						},
					},
				},
			},
		},
		Uncertain: []scanservice.Candidate{
			{
				Provider:            "neon",
				ResourceID:          "br-uncertain",
				ResourceName:        "preview-pr-3",
				PullRequestFound:    false,
				SourceBranchChecked: false,
				Score: scoring.Result{
					PolicyVersion:  scoring.PolicyVersion,
					Score:          8,
					Confidence:     scoring.ConfidenceLow,
					Recommended:    false,
					Recommendation: "Insufficient evidence to recommend cleanup: keep this resource.",
					Contributions: []scoring.Contribution{
						{
							Kind:        evidence.KindNamingConventionMatch,
							Description: "Resource name matches Sweep's recognized preview-resource naming convention.",
							Points:      8,
						},
					},
				},
			},
		},
		Skipped: []scanservice.Skipped{
			{
				Provider:     "neon",
				ResourceID:   "br-prod",
				ResourceName: "production",
				Reason:       "resource is marked default, protected, or production-like",
			},
		},
		Warnings: []scanservice.Warning{
			{
				Provider: "neon",
				Resource: "br-flaky",
				Message:  "neon br-flaky: could not retrieve pull request #2: connection failed",
			},
		},
	}
}

func TestScanCommandPrintsTextReport(t *testing.T) {
	runner := scanRunner(func(
		_ context.Context,
		cfg scanservice.Config,
	) (scanservice.Result, error) {
		if cfg.Repository != "RishabJain30/sweep-lifecycle" {
			t.Fatalf(
				"repository = %q, want %q",
				cfg.Repository,
				"RishabJain30/sweep-lifecycle",
			)
		}

		if cfg.NeonProjectID != "test-project" {
			t.Fatalf(
				"project ID = %q, want %q",
				cfg.NeonProjectID,
				"test-project",
			)
		}

		return sampleResult(), nil
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
		"Providers:",
		"neon: ok - discovered 1 branch(es)",
		"vercel: skipped - Vercel is not configured",
		"Cleanup candidates (1):",
		"neon br-preview (preview-pr-1)",
		"Pull request: #1 (merged)",
		"PR head repository: RishabJain30/sweep-lifecycle",
		"PR head branch: feat/example",
		"Source branch exists: false",
		"Score: 78/100",
		"Confidence: HIGH",
		"[+30] The correlated pull request has been merged.",
		"[+35] The pull request's source branch no longer exists on GitHub.",
		"Protected / skipped resources (1):",
		"neon br-prod (production): resource is marked default, protected, or production-like",
		"Warnings (1):",
		"could not retrieve pull request #2",
		"Uncertain / low-confidence resources (1):",
		"neon br-uncertain (preview-pr-3)",
		"Summary: 1 candidate(s) (1 high confidence), 1 uncertain, 1 skipped, 1 warning(s)",
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

func TestScanCommandPrintsJSONReport(t *testing.T) {
	runner := scanRunner(func(
		context.Context,
		scanservice.Config,
	) (scanservice.Result, error) {
		return sampleResult(), nil
	})

	command := newScanCommand(runner)

	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"--repo", "RishabJain30/sweep-lifecycle",
		"--neon-project", "test-project",
		"--format", "json",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var report jsonReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("Unmarshal() error = %v\noutput: %s", err, output.String())
	}

	if len(report.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want 1", report.Candidates)
	}

	if report.Candidates[0].ResourceID != "br-preview" {
		t.Fatalf(
			"resource ID = %q, want %q",
			report.Candidates[0].ResourceID,
			"br-preview",
		)
	}

	if report.Candidates[0].PullRequest == nil {
		t.Fatal("PullRequest is nil, want the merged PR")
	}

	if report.Candidates[0].Score.Value != 78 {
		t.Fatalf("score value = %d, want 78", report.Candidates[0].Score.Value)
	}

	if !report.Candidates[0].Score.Recommended {
		t.Fatal("candidate Recommended = false, want true")
	}

	if len(report.Uncertain) != 1 ||
		report.Uncertain[0].ResourceID != "br-uncertain" {
		t.Fatalf("uncertain = %+v, want br-uncertain", report.Uncertain)
	}

	if report.Uncertain[0].Score.Recommended {
		t.Fatal("uncertain Recommended = true, want false")
	}

	if report.Summary.CandidateCount != 1 ||
		report.Summary.HighConfidenceCount != 1 ||
		report.Summary.UncertainCount != 1 ||
		report.Summary.SkippedCount != 1 ||
		report.Summary.WarningCount != 1 {
		t.Fatalf("summary = %+v, want 1/1/1/1/1", report.Summary)
	}

	if len(report.Providers) != 2 {
		t.Fatalf("providers = %+v, want 2", report.Providers)
	}

	if report.Providers[0].Provider != "neon" ||
		report.Providers[0].Status != "ok" {
		t.Fatalf("providers[0] = %+v, want neon/ok", report.Providers[0])
	}

	if len(report.Skipped) != 1 ||
		report.Skipped[0].ResourceID != "br-prod" ||
		!strings.Contains(report.Skipped[0].Reason, "protected") {
		t.Fatalf("skipped = %+v, want br-prod/protected", report.Skipped)
	}

	if len(report.Warnings) != 1 ||
		report.Warnings[0].Resource != "br-flaky" {
		t.Fatalf("warnings = %+v, want br-flaky", report.Warnings)
	}

	if len(report.Candidates[0].Score.Evidence) != 2 {
		t.Fatalf(
			"evidence = %+v, want 2 contributions",
			report.Candidates[0].Score.Evidence,
		)
	}

	if strings.Contains(output.String(), "GITHUB_TOKEN") ||
		strings.Contains(output.String(), "token") {
		t.Fatalf(
			"JSON output must never mention credentials: %s",
			output.String(),
		)
	}
}

func TestScanCommandRejectsMalformedRepository(t *testing.T) {
	command := newScanCommand(func(
		context.Context,
		scanservice.Config,
	) (scanservice.Result, error) {
		t.Fatal("scan runner should not be called")
		return scanservice.Result{}, nil
	})

	command.SetArgs([]string{
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

func TestScanCommandRequiresRepository(t *testing.T) {
	command := newScanCommand(func(
		context.Context,
		scanservice.Config,
	) (scanservice.Result, error) {
		t.Fatal("scan runner should not be called")
		return scanservice.Result{}, nil
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
		scanservice.Config,
	) (scanservice.Result, error) {
		t.Fatal("scan runner should not be called")
		return scanservice.Result{}, nil
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

func TestScanCommandRejectsInvalidFormat(t *testing.T) {
	command := newScanCommand(func(
		context.Context,
		scanservice.Config,
	) (scanservice.Result, error) {
		t.Fatal("scan runner should not be called")
		return scanservice.Result{}, nil
	})

	command.SetArgs([]string{
		"--repo", "RishabJain30/sweep-lifecycle",
		"--neon-project", "test-project",
		"--format", "yaml",
	})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "--format") {
		t.Fatalf("error = %q, want a --format error", err)
	}
}

func TestScanCommandPrintsZeroCandidatesSuccessfully(t *testing.T) {
	runner := scanRunner(func(
		context.Context,
		scanservice.Config,
	) (scanservice.Result, error) {
		return scanservice.Result{
			ProviderStatuses: []scanservice.ProviderStatus{
				{Provider: "neon", Status: "ok", Detail: "discovered 0 branch(es)"},
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
		t.Fatalf("Execute() error = %v, want success with zero candidates", err)
	}

	if !strings.Contains(output.String(), "Cleanup candidates (0):") {
		t.Fatalf("output missing zero-candidate section:\n%s", output.String())
	}
}
