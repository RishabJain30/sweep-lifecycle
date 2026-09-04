// Package evidence builds a small, provider-independent set of lifecycle
// signals about a preview resource. Evidence items are pure facts translated
// from provider-specific data (Neon branches, Vercel deployments, GitHub
// pull requests) - they carry no score or recommendation. Scoring policy
// lives in internal/scoring and consumes these items deterministically.
package evidence

import (
	"fmt"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

// Kind identifies a distinct lifecycle signal.
type Kind string

const (
	// KindPullRequestNotCorrelated means Sweep could not associate the
	// resource with any pull request at all (for example, the resource
	// name does not match a recognized preview naming convention and no
	// other correlation metadata was available).
	KindPullRequestNotCorrelated Kind = "pull_request_not_correlated"
	// KindPullRequestUnknown means a pull request was correlated (by name
	// or metadata) but Sweep could not retrieve it from GitHub.
	KindPullRequestUnknown Kind = "pull_request_unknown"
	KindPullRequestOpen    Kind = "pull_request_open"
	KindPullRequestClosed  Kind = "pull_request_closed"
	KindPullRequestMerged  Kind = "pull_request_merged"

	KindSourceBranchExists  Kind = "source_branch_exists"
	KindSourceBranchMissing Kind = "source_branch_missing"
	// KindSourceBranchUnchecked means Sweep did not attempt to verify the
	// source branch, typically because the source repository itself is
	// missing.
	KindSourceBranchUnchecked Kind = "source_branch_unchecked"

	// KindSourceRepositoryMissing means GitHub no longer exposes the
	// repository that originally contained the pull request's source
	// branch (for example, a deleted fork).
	KindSourceRepositoryMissing Kind = "source_repository_missing"

	// KindNamingConventionMatch means the resource name follows Sweep's
	// recognized preview-resource naming convention. This is weak,
	// corroborating evidence only.
	KindNamingConventionMatch Kind = "naming_convention_match"

	// KindProtectedResource means the provider marks the resource as
	// default, protected, or production-like.
	KindProtectedResource Kind = "protected_resource"

	// KindResourceAgeMature/KindResourceAgeRecent describe how long ago the
	// resource was created or last updated by its provider. "Updated" is
	// reported as resource update time, not as a claim about database or
	// application activity, since providers do not uniformly define it
	// that way.
	KindResourceAgeMature Kind = "resource_age_mature"
	KindResourceAgeRecent Kind = "resource_age_recent"

	// KindLookupIncomplete means a provider lookup needed to fully evaluate
	// this resource failed. Whatever was learned before the failure is
	// still reported, but confidence must be reduced.
	KindLookupIncomplete Kind = "lookup_incomplete"
)

// RecentResourceThreshold is the conservative recency window: a resource
// created or updated more recently than this is still likely to be part of
// active review or rollout, so evidence flags it as recent rather than
// assuming inactivity.
const RecentResourceThreshold = 24 * time.Hour

// Item is a single self-explanatory lifecycle signal about a resource.
type Item struct {
	Kind        Kind
	Description string
}

// Input carries every raw signal Collect needs. It has no dependency on any
// provider package: callers translate provider-specific data into this
// shape before evidence is collected.
type Input struct {
	NameMatchesConvention bool

	// ResourceProtected reports whether the provider marks the resource as
	// default, protected, or production-like.
	ResourceProtected bool

	ResourceCreatedAt time.Time
	ResourceUpdatedAt time.Time
	// Now is the evaluation time, injected so evidence collection is
	// deterministic and independent of wall-clock time in tests.
	Now time.Time

	// PullRequestCorrelated reports whether Sweep associated a pull
	// request with this resource at all (by name or by metadata).
	PullRequestCorrelated bool
	// PullRequestFound reports whether the correlated pull request was
	// successfully retrieved from GitHub.
	PullRequestFound bool
	PullRequestState domain.PullRequestState

	// SourceBranchChecked reports whether Sweep attempted to verify the
	// pull request's source branch.
	SourceBranchChecked bool
	SourceBranchExists  bool

	// SourceRepositoryMissing reports whether GitHub no longer exposes the
	// repository that originally contained the pull request's source
	// branch.
	SourceRepositoryMissing bool

	// LookupIncomplete reports whether a provider lookup needed to fully
	// evaluate this resource failed.
	LookupIncomplete       bool
	LookupIncompleteReason string
}

// Collect deterministically translates an Input into an ordered list of
// evidence items. Equal inputs always produce equal output.
func Collect(input Input) []Item {
	var items []Item

	if input.ResourceProtected {
		items = append(items, Item{
			Kind: KindProtectedResource,
			Description: "Resource is marked default, protected, or " +
				"production-like.",
		})
	}

	if input.NameMatchesConvention {
		items = append(items, Item{
			Kind: KindNamingConventionMatch,
			Description: "Resource name matches Sweep's recognized " +
				"preview-resource naming convention.",
		})
	}

	items = append(items, pullRequestEvidence(input))

	if input.SourceRepositoryMissing {
		items = append(items, Item{
			Kind: KindSourceRepositoryMissing,
			Description: "GitHub no longer exposes the repository that " +
				"originally contained the pull request's source branch " +
				"(for example, a deleted fork).",
		})
	}

	items = append(items, sourceBranchEvidence(input))

	if age, ok := resourceAge(input); ok {
		items = append(items, resourceAgeEvidence(age))
	}

	if input.LookupIncomplete {
		reason := input.LookupIncompleteReason
		if reason == "" {
			reason = "a provider lookup failed"
		}

		items = append(items, Item{
			Kind: KindLookupIncomplete,
			Description: fmt.Sprintf(
				"Evidence is incomplete because %s; confidence is "+
					"reduced accordingly.",
				reason,
			),
		})
	}

	return items
}

func pullRequestEvidence(input Input) Item {
	if !input.PullRequestCorrelated {
		return Item{
			Kind: KindPullRequestNotCorrelated,
			Description: "Sweep could not correlate this resource with " +
				"any pull request.",
		}
	}

	if !input.PullRequestFound {
		return Item{
			Kind: KindPullRequestUnknown,
			Description: "Sweep correlated a pull request but could not " +
				"retrieve it from GitHub.",
		}
	}

	switch input.PullRequestState {
	case domain.PullRequestStateMerged:
		return Item{
			Kind:        KindPullRequestMerged,
			Description: "The correlated pull request has been merged.",
		}
	case domain.PullRequestStateClosed:
		return Item{
			Kind: KindPullRequestClosed,
			Description: "The correlated pull request was closed " +
				"without merging.",
		}
	default:
		return Item{
			Kind:        KindPullRequestOpen,
			Description: "The correlated pull request is still open.",
		}
	}
}

func sourceBranchEvidence(input Input) Item {
	if !input.SourceBranchChecked {
		return Item{
			Kind: KindSourceBranchUnchecked,
			Description: "Sweep did not verify whether the pull " +
				"request's source branch still exists.",
		}
	}

	if input.SourceBranchExists {
		return Item{
			Kind: KindSourceBranchExists,
			Description: "The pull request's source branch still " +
				"exists on GitHub.",
		}
	}

	return Item{
		Kind: KindSourceBranchMissing,
		Description: "The pull request's source branch no longer " +
			"exists on GitHub.",
	}
}

func resourceAge(input Input) (time.Duration, bool) {
	lastTouched := input.ResourceUpdatedAt
	if input.ResourceCreatedAt.After(lastTouched) {
		lastTouched = input.ResourceCreatedAt
	}

	if lastTouched.IsZero() || input.Now.IsZero() {
		return 0, false
	}

	age := input.Now.Sub(lastTouched)
	if age < 0 {
		age = 0
	}

	return age, true
}

func resourceAgeEvidence(age time.Duration) Item {
	if age < RecentResourceThreshold {
		return Item{
			Kind: KindResourceAgeRecent,
			Description: fmt.Sprintf(
				"Resource was created or updated %s ago, within "+
					"Sweep's %s recency window.",
				age.Round(time.Minute),
				RecentResourceThreshold,
			),
		}
	}

	return Item{
		Kind: KindResourceAgeMature,
		Description: fmt.Sprintf(
			"Resource has not been created or updated for %s, past "+
				"Sweep's %s recency window.",
			age.Round(time.Hour),
			RecentResourceThreshold,
		),
	}
}
