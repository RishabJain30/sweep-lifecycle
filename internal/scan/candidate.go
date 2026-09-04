package scan

import (
	"context"
	"fmt"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
	"github.com/RishabJain30/sweep-lifecycle/internal/evidence"
	"github.com/RishabJain30/sweep-lifecycle/internal/scoring"
)

// SourceControl provides the GitHub lifecycle information needed to
// evaluate a resource.
type SourceControl interface {
	GetPullRequest(
		ctx context.Context,
		repository string,
		number int,
	) (domain.PullRequest, error)

	BranchExists(
		ctx context.Context,
		repository string,
		branch string,
	) (bool, error)
}

// ResourceInput is a provider-independent description of one discovered
// resource, translated by the caller from a Neon branch or a Vercel
// deployment.
type ResourceInput struct {
	Provider     string
	ResourceID   string
	ResourceName string

	ResourceProtected bool
	ResourceCreatedAt time.Time
	ResourceUpdatedAt time.Time

	// NameMatchesConvention reports whether the resource name follows
	// Sweep's recognized preview-resource naming convention. It is
	// corroborating evidence only, independent of PullRequestNumber.
	NameMatchesConvention bool

	// PullRequestNumber is the pull request Sweep correlated this
	// resource with. Zero means no correlation was possible.
	PullRequestNumber int
}

// Candidate is one resource Sweep evaluated: its identity, the pull request
// it correlated with (if any), the source-branch check outcome, and the
// deterministic evidence and score that resulted.
type Candidate struct {
	Provider     string
	ResourceID   string
	ResourceName string

	PullRequest      domain.PullRequest
	PullRequestFound bool

	SourceBranchChecked bool
	SourceBranchExists  bool

	Evidence []evidence.Item
	Score    scoring.Result
}

// EvaluateResource correlates a resource with GitHub and evaluates its
// evidence and score. scan.Service and the explain command both call this
// function so a resource is always evaluated identically regardless of
// which command asked about it. Provider lookup failures never abort
// evaluation: they are recorded as incomplete evidence and returned as a
// non-empty warning message, so the caller can surface a partial failure
// without losing everything else it learned about the resource.
func EvaluateResource(
	ctx context.Context,
	github SourceControl,
	repository string,
	input ResourceInput,
	now time.Time,
) (Candidate, string) {
	candidate := Candidate{
		Provider:     input.Provider,
		ResourceID:   input.ResourceID,
		ResourceName: input.ResourceName,
	}

	evidenceInput := evidence.Input{
		NameMatchesConvention: input.NameMatchesConvention,
		ResourceProtected:     input.ResourceProtected,
		ResourceCreatedAt:     input.ResourceCreatedAt,
		ResourceUpdatedAt:     input.ResourceUpdatedAt,
		Now:                   now,
		PullRequestCorrelated: input.PullRequestNumber > 0,
	}

	var warning string

	if evidenceInput.PullRequestCorrelated {
		warning = evaluatePullRequest(
			ctx,
			github,
			repository,
			input,
			&evidenceInput,
			&candidate,
		)
	}

	items := evidence.Collect(evidenceInput)
	candidate.Evidence = items
	candidate.Score = scoring.Evaluate(items)

	return candidate, warning
}

// evaluatePullRequest fetches the correlated pull request and, if found,
// checks whether its source branch still exists. It mutates evidenceInput
// and candidate in place and returns a non-empty warning message on
// partial failure.
func evaluatePullRequest(
	ctx context.Context,
	github SourceControl,
	repository string,
	input ResourceInput,
	evidenceInput *evidence.Input,
	candidate *Candidate,
) string {
	pr, err := github.GetPullRequest(ctx, repository, input.PullRequestNumber)
	if err != nil {
		evidenceInput.LookupIncomplete = true
		evidenceInput.LookupIncompleteReason = fmt.Sprintf(
			"retrieving pull request #%d failed: %s",
			input.PullRequestNumber,
			err,
		)

		return fmt.Sprintf(
			"%s %s: could not retrieve pull request #%d: %s",
			input.Provider,
			input.ResourceID,
			input.PullRequestNumber,
			err,
		)
	}

	evidenceInput.PullRequestFound = true
	evidenceInput.PullRequestState = pr.State
	evidenceInput.SourceRepositoryMissing = pr.HeadRepository == ""
	candidate.PullRequest = pr
	candidate.PullRequestFound = true

	if pr.HeadRepository == "" {
		return ""
	}

	exists, err := github.BranchExists(ctx, pr.HeadRepository, pr.HeadBranch)
	if err != nil {
		evidenceInput.LookupIncomplete = true
		evidenceInput.LookupIncompleteReason = fmt.Sprintf(
			"checking source branch %q failed: %s",
			pr.HeadBranch,
			err,
		)

		return fmt.Sprintf(
			"%s %s: could not verify source branch %q: %s",
			input.Provider,
			input.ResourceID,
			pr.HeadBranch,
			err,
		)
	}

	evidenceInput.SourceBranchChecked = true
	evidenceInput.SourceBranchExists = exists
	candidate.SourceBranchChecked = true
	candidate.SourceBranchExists = exists

	return ""
}
