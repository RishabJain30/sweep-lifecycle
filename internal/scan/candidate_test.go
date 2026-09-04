package scan

import (
	"context"
	"errors"
	"testing"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
	"github.com/RishabJain30/sweep-lifecycle/internal/evidence"
	"github.com/RishabJain30/sweep-lifecycle/internal/scoring"
)

func TestEvaluateResourceUncorrelated(t *testing.T) {
	sourceControl := &stubSourceControl{}

	candidate, warning := EvaluateResource(
		context.Background(),
		sourceControl,
		"RishabJain30/sweep-lifecycle",
		ResourceInput{
			Provider:     "neon",
			ResourceID:   "br-manual",
			ResourceName: "manual-test",
		},
		fixedNow,
	)

	if warning != "" {
		t.Fatalf("warning = %q, want none", warning)
	}

	if !candidate.Score.Excluded {
		t.Fatal("Excluded = false, want true for an uncorrelated resource")
	}

	if len(sourceControl.pullRequestCalls) != 0 {
		t.Fatalf(
			"GitHub PR calls = %v, want none for an uncorrelated resource",
			sourceControl.pullRequestCalls,
		)
	}
}

func TestEvaluateResourceOpenPullRequest(t *testing.T) {
	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			1: {Number: 1, State: domain.PullRequestStateOpen},
		},
	}

	candidate, warning := EvaluateResource(
		context.Background(),
		sourceControl,
		"RishabJain30/sweep-lifecycle",
		ResourceInput{
			Provider:          "neon",
			ResourceID:        "br-preview",
			ResourceName:      "preview-pr-1",
			PullRequestNumber: 1,
		},
		fixedNow,
	)

	if warning != "" {
		t.Fatalf("warning = %q, want none", warning)
	}

	if !candidate.Score.Excluded {
		t.Fatal("Excluded = false, want true for an open pull request")
	}

	if len(sourceControl.branchCalls) != 0 {
		t.Fatalf(
			"GitHub branch calls = %v, want none for an open PR",
			sourceControl.branchCalls,
		)
	}
}

func TestEvaluateResourcePullRequestLookupFailureIsIncompleteNotFatal(t *testing.T) {
	sourceControl := &stubSourceControl{
		pullRequestErr: errors.New("connection failed"),
	}

	candidate, warning := EvaluateResource(
		context.Background(),
		sourceControl,
		"RishabJain30/sweep-lifecycle",
		ResourceInput{
			Provider:          "neon",
			ResourceID:        "br-preview",
			ResourceName:      "preview-pr-1",
			PullRequestNumber: 1,
		},
		fixedNow,
	)

	if warning == "" {
		t.Fatal("warning is empty, want a warning describing the failure")
	}

	if candidate.PullRequestFound {
		t.Fatal("PullRequestFound = true, want false")
	}

	if candidate.Score.Confidence != scoring.ConfidenceLow {
		t.Fatalf(
			"Confidence = %s, want %s",
			candidate.Score.Confidence,
			scoring.ConfidenceLow,
		)
	}

	found := false
	for _, item := range candidate.Evidence {
		if item.Kind == evidence.KindLookupIncomplete {
			found = true
		}
	}

	if !found {
		t.Fatalf(
			"evidence = %+v, want a KindLookupIncomplete item",
			candidate.Evidence,
		)
	}
}

func TestEvaluateResourceProtectedNamedLikeAPreview(t *testing.T) {
	// A resource that is both protected and happens to match the naming
	// convention must still be excluded, and evidence must accurately
	// reflect that GitHub was actually queried (not silently skipped).
	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			1: {Number: 1, State: domain.PullRequestStateMerged},
		},
	}

	candidate, _ := EvaluateResource(
		context.Background(),
		sourceControl,
		"RishabJain30/sweep-lifecycle",
		ResourceInput{
			Provider:              "neon",
			ResourceID:            "br-odd",
			ResourceName:          "preview-pr-1",
			ResourceProtected:     true,
			NameMatchesConvention: true,
			PullRequestNumber:     1,
		},
		fixedNow,
	)

	if !candidate.Score.Excluded {
		t.Fatal("Excluded = false, want true for a protected resource")
	}

	if candidate.Score.ExclusionReason == "" ||
		candidate.Score.ExclusionReason[:9] != "resource " {
		t.Fatalf(
			"ExclusionReason = %q, want the protected-resource reason",
			candidate.Score.ExclusionReason,
		)
	}
}

func TestEvaluateResourceIsDeterministic(t *testing.T) {
	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			1: {
				Number:         1,
				State:          domain.PullRequestStateMerged,
				HeadRepository: "RishabJain30/sweep-lifecycle",
				HeadBranch:     "feat/example",
			},
		},
		branchExists: false,
	}

	input := ResourceInput{
		Provider:              "neon",
		ResourceID:            "br-preview",
		ResourceName:          "preview-pr-1",
		NameMatchesConvention: true,
		PullRequestNumber:     1,
		ResourceCreatedAt:     mature,
		ResourceUpdatedAt:     mature,
	}

	first, _ := EvaluateResource(
		context.Background(),
		sourceControl,
		"RishabJain30/sweep-lifecycle",
		input,
		fixedNow,
	)

	second, _ := EvaluateResource(
		context.Background(),
		sourceControl,
		"RishabJain30/sweep-lifecycle",
		input,
		fixedNow,
	)

	if first.Score.Score != second.Score.Score {
		t.Fatalf(
			"Score = %d, want %d (must be deterministic)",
			second.Score.Score,
			first.Score.Score,
		)
	}

	if first.Score.Confidence != second.Score.Confidence {
		t.Fatalf(
			"Confidence = %s, want %s (must be deterministic)",
			second.Score.Confidence,
			first.Score.Confidence,
		)
	}
}
