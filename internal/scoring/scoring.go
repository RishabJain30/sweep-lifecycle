// Package scoring implements Sweep's deterministic, versioned cleanup
// scoring policy. It contains no AI, heuristics, or probabilistic models:
// the same evidence always produces the same score, confidence, and
// recommendation. Only internal/evidence feeds this package, so scan and
// explain always evaluate resources through the exact same policy.
package scoring

import (
	"fmt"

	"github.com/RishabJain30/sweep-lifecycle/internal/evidence"
)

// PolicyVersion identifies the scoring rules below. Bump it whenever
// weights, thresholds, or the confidence/exclusion rules change, so past
// explanations remain attributable to the policy that produced them.
const PolicyVersion = "v1"

// Weights are conservative by design: the two strongest signals a preview
// resource has outlived its purpose - a finished pull request and a
// missing source branch - together must clear the HIGH-confidence
// threshold on their own. Every other signal is corroborating only and is
// deliberately too small to reach HIGH, or even MEDIUM, by itself. This is
// what stops a resource from being strongly recommended for cleanup merely
// because its name contains a PR number (weightNamingConventionMatch is
// far below thresholdMedium).
const (
	weightPullRequestMerged   = 30
	weightPullRequestClosed   = 20
	weightSourceBranchExists  = -60
	weightSourceBranchMissing = 35

	// Corroborating signals only. None of these alone (nor all of them
	// combined) can reach thresholdMedium.
	weightSourceRepositoryMissing = 5
	weightNamingConventionMatch   = 8
	weightResourceAgeMature       = 5
	weightResourceAgeRecent       = -15
)

const (
	scoreMin = 0
	scoreMax = 100

	// thresholdMedium/thresholdHigh bound the score bands used for
	// recommendation wording and gate the MEDIUM/HIGH confidence levels.
	thresholdMedium = 35
	thresholdHigh   = 65
)

// Confidence expresses how much a user should trust Score, independent of
// how high or low it is.
type Confidence string

const (
	ConfidenceLow    Confidence = "LOW"
	ConfidenceMedium Confidence = "MEDIUM"
	ConfidenceHigh   Confidence = "HIGH"
)

// Contribution explains how much a single evidence item added to or
// subtracted from Score. Descriptions come from the evidence item itself so
// explanations never drift from the evidence that produced them.
type Contribution struct {
	Kind        evidence.Kind
	Description string
	Points      int
}

// Result is the complete, self-explanatory outcome of evaluating one
// resource's evidence against the policy.
type Result struct {
	PolicyVersion string
	Score         int
	Confidence    Confidence

	// Excluded means this resource must never be recommended for cleanup
	// regardless of any other evidence (protected/default/production-like
	// resources, resources with an open pull request, or resources Sweep
	// could not correlate with any pull request at all).
	Excluded        bool
	ExclusionReason string

	Recommendation string
	Contributions  []Contribution
}

// Evaluate deterministically scores a resource from its evidence items.
// Identical evidence always produces an identical Result.
func Evaluate(items []evidence.Item) Result {
	kinds := indexByKind(items)

	result := Result{PolicyVersion: PolicyVersion}
	result.Excluded, result.ExclusionReason = exclusionFor(kinds)

	score := clamp(totalScore(items), scoreMin, scoreMax)
	if result.Excluded {
		score = scoreMin
	}

	result.Score = score
	result.Confidence = confidenceFor(kinds, score)
	result.Contributions = contributionsFor(items)
	result.Recommendation = recommendationFor(result)

	return result
}

func indexByKind(items []evidence.Item) map[evidence.Kind]bool {
	kinds := make(map[evidence.Kind]bool, len(items))
	for _, item := range items {
		kinds[item.Kind] = true
	}

	return kinds
}

func exclusionFor(kinds map[evidence.Kind]bool) (bool, string) {
	switch {
	case kinds[evidence.KindProtectedResource]:
		return true, "resource is marked default, protected, or " +
			"production-like"
	case kinds[evidence.KindPullRequestOpen]:
		return true, "the correlated pull request is still open"
	case kinds[evidence.KindPullRequestNotCorrelated]:
		return true, "no pull request could be correlated with this " +
			"resource"
	default:
		return false, ""
	}
}

func weightFor(kind evidence.Kind) int {
	switch kind {
	case evidence.KindPullRequestMerged:
		return weightPullRequestMerged
	case evidence.KindPullRequestClosed:
		return weightPullRequestClosed
	case evidence.KindSourceBranchExists:
		return weightSourceBranchExists
	case evidence.KindSourceBranchMissing:
		return weightSourceBranchMissing
	case evidence.KindSourceRepositoryMissing:
		return weightSourceRepositoryMissing
	case evidence.KindNamingConventionMatch:
		return weightNamingConventionMatch
	case evidence.KindResourceAgeMature:
		return weightResourceAgeMature
	case evidence.KindResourceAgeRecent:
		return weightResourceAgeRecent
	default:
		return 0
	}
}

func totalScore(items []evidence.Item) int {
	total := 0
	for _, item := range items {
		total += weightFor(item.Kind)
	}

	return total
}

func clamp(value, min, max int) int {
	switch {
	case value < min:
		return min
	case value > max:
		return max
	default:
		return value
	}
}

// confidenceFor requires strong, complete, non-recent lifecycle evidence
// before trusting a score: a finished pull request together with a missing
// source branch, clearing thresholdHigh, is the only path to HIGH. Recent
// or incomplete evidence always caps confidence at LOW, regardless of
// score.
func confidenceFor(kinds map[evidence.Kind]bool, score int) Confidence {
	if kinds[evidence.KindLookupIncomplete] ||
		kinds[evidence.KindPullRequestUnknown] ||
		kinds[evidence.KindResourceAgeRecent] {
		return ConfidenceLow
	}

	finishedPullRequest := kinds[evidence.KindPullRequestMerged] ||
		kinds[evidence.KindPullRequestClosed]
	sourceBranchMissing := kinds[evidence.KindSourceBranchMissing]

	switch {
	case score >= thresholdHigh &&
		finishedPullRequest &&
		sourceBranchMissing:
		return ConfidenceHigh
	case score >= thresholdMedium &&
		(finishedPullRequest || sourceBranchMissing):
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

func contributionsFor(items []evidence.Item) []Contribution {
	contributions := make([]Contribution, 0, len(items))

	for _, item := range items {
		contributions = append(contributions, Contribution{
			Kind:        item.Kind,
			Description: item.Description,
			Points:      weightFor(item.Kind),
		})
	}

	return contributions
}

func recommendationFor(result Result) string {
	if result.Excluded {
		return fmt.Sprintf("Do not clean up: %s.", result.ExclusionReason)
	}

	switch {
	case result.Score >= thresholdHigh:
		return "Strong cleanup candidate: finished pull request and " +
			"missing source branch."
	case result.Score >= thresholdMedium:
		return "Possible cleanup candidate: review the evidence before " +
			"removing it."
	default:
		return "Insufficient evidence to recommend cleanup: keep this " +
			"resource."
	}
}
