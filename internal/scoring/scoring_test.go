package scoring

import (
	"testing"

	"github.com/RishabJain30/sweep-lifecycle/internal/evidence"
)

func TestEvaluateIsDeterministic(t *testing.T) {
	items := []evidence.Item{
		{Kind: evidence.KindPullRequestMerged},
		{Kind: evidence.KindSourceBranchMissing},
		{Kind: evidence.KindNamingConventionMatch},
	}

	first := Evaluate(items)
	second := Evaluate(items)

	if first.Score != second.Score {
		t.Fatalf("Score = %d, want %d", second.Score, first.Score)
	}

	if first.Confidence != second.Confidence {
		t.Fatalf(
			"Confidence = %s, want %s",
			second.Confidence,
			first.Confidence,
		)
	}

	if first.Excluded != second.Excluded {
		t.Fatalf("Excluded = %v, want %v", second.Excluded, first.Excluded)
	}
}

func TestEvaluateScoreIsBounded(t *testing.T) {
	tests := []struct {
		name  string
		items []evidence.Item
	}{
		{
			name: "many duplicate positive signals stay at or below max",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestMerged},
				{Kind: evidence.KindPullRequestMerged},
				{Kind: evidence.KindSourceBranchMissing},
				{Kind: evidence.KindSourceBranchMissing},
				{Kind: evidence.KindSourceBranchMissing},
				{Kind: evidence.KindNamingConventionMatch},
				{Kind: evidence.KindResourceAgeMature},
				{Kind: evidence.KindSourceRepositoryMissing},
			},
		},
		{
			name: "many duplicate negative signals stay at or above min",
			items: []evidence.Item{
				{Kind: evidence.KindSourceBranchExists},
				{Kind: evidence.KindSourceBranchExists},
				{Kind: evidence.KindSourceBranchExists},
				{Kind: evidence.KindResourceAgeRecent},
				{Kind: evidence.KindResourceAgeRecent},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Evaluate(test.items)

			if result.Score < scoreMin || result.Score > scoreMax {
				t.Fatalf(
					"Score = %d, want between %d and %d",
					result.Score,
					scoreMin,
					scoreMax,
				)
			}
		})
	}
}

func TestEvaluateExcludesProtectedResourceRegardlessOfPositiveSignals(t *testing.T) {
	result := Evaluate([]evidence.Item{
		{Kind: evidence.KindProtectedResource},
		{Kind: evidence.KindPullRequestMerged},
		{Kind: evidence.KindSourceBranchMissing},
		{Kind: evidence.KindNamingConventionMatch},
	})

	if !result.Excluded {
		t.Fatal("Excluded = false, want true for a protected resource")
	}

	if result.Score != scoreMin {
		t.Fatalf(
			"Score = %d, want %d for an excluded resource",
			result.Score,
			scoreMin,
		)
	}
}

func TestEvaluateExcludesOpenPullRequest(t *testing.T) {
	result := Evaluate([]evidence.Item{
		{Kind: evidence.KindPullRequestOpen},
		{Kind: evidence.KindSourceBranchExists},
	})

	if !result.Excluded {
		t.Fatal("Excluded = false, want true for an open pull request")
	}
}

func TestEvaluateExcludesUncorrelatedResource(t *testing.T) {
	result := Evaluate([]evidence.Item{
		{Kind: evidence.KindPullRequestNotCorrelated},
	})

	if !result.Excluded {
		t.Fatal(
			"Excluded = false, want true for an uncorrelated resource",
		)
	}
}

func TestEvaluateNamingAloneIsNotStronglyRecommended(t *testing.T) {
	result := Evaluate([]evidence.Item{
		{Kind: evidence.KindPullRequestUnknown},
		{Kind: evidence.KindNamingConventionMatch},
	})

	if result.Excluded {
		t.Fatal("Excluded = true, want false")
	}

	if result.Score >= thresholdMedium {
		t.Fatalf(
			"Score = %d, want below thresholdMedium (%d) for naming "+
				"evidence alone",
			result.Score,
			thresholdMedium,
		)
	}

	if result.Confidence == ConfidenceHigh {
		t.Fatal("Confidence = HIGH, want not HIGH for naming evidence alone")
	}
}

func TestEvaluateHighConfidenceRequiresFinishedPRAndMissingBranch(t *testing.T) {
	tests := []struct {
		name     string
		items    []evidence.Item
		wantHigh bool
	}{
		{
			name: "merged and missing branch reaches HIGH",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestMerged},
				{Kind: evidence.KindSourceBranchMissing},
				{Kind: evidence.KindNamingConventionMatch},
				{Kind: evidence.KindResourceAgeMature},
			},
			wantHigh: true,
		},
		{
			name: "merged without missing branch stays below HIGH",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestMerged},
				{Kind: evidence.KindSourceBranchExists},
			},
			wantHigh: false,
		},
		{
			name: "missing branch without a finished PR stays below HIGH",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestUnknown},
				{Kind: evidence.KindSourceBranchMissing},
			},
			wantHigh: false,
		},
		{
			name: "closed (not merged) with missing branch reaches MEDIUM, not HIGH",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestClosed},
				{Kind: evidence.KindSourceBranchMissing},
			},
			wantHigh: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Evaluate(test.items)

			isHigh := result.Confidence == ConfidenceHigh
			if isHigh != test.wantHigh {
				t.Fatalf(
					"Confidence = %s (score %d), want HIGH = %v",
					result.Confidence,
					result.Score,
					test.wantHigh,
				)
			}
		})
	}
}

func TestEvaluateRecentEvidenceReducesConfidenceToLow(t *testing.T) {
	result := Evaluate([]evidence.Item{
		{Kind: evidence.KindPullRequestMerged},
		{Kind: evidence.KindSourceBranchMissing},
		{Kind: evidence.KindResourceAgeRecent},
	})

	if result.Confidence != ConfidenceLow {
		t.Fatalf(
			"Confidence = %s, want %s for recent evidence even with "+
				"strong lifecycle signals",
			result.Confidence,
			ConfidenceLow,
		)
	}
}

func TestEvaluateIncompleteEvidenceReducesConfidenceToLow(t *testing.T) {
	result := Evaluate([]evidence.Item{
		{Kind: evidence.KindPullRequestMerged},
		{Kind: evidence.KindSourceBranchMissing},
		{Kind: evidence.KindLookupIncomplete},
	})

	if result.Confidence != ConfidenceLow {
		t.Fatalf(
			"Confidence = %s, want %s when evidence is incomplete",
			result.Confidence,
			ConfidenceLow,
		)
	}
}

func TestEvaluateMediumConfidenceCandidate(t *testing.T) {
	result := Evaluate([]evidence.Item{
		{Kind: evidence.KindPullRequestClosed},
		{Kind: evidence.KindSourceBranchMissing},
		{Kind: evidence.KindNamingConventionMatch},
	})

	if result.Excluded {
		t.Fatal("Excluded = true, want false")
	}

	if result.Confidence != ConfidenceMedium {
		t.Fatalf(
			"Confidence = %s, want %s",
			result.Confidence,
			ConfidenceMedium,
		)
	}
}

func TestEvaluateContributionsMirrorEvidenceOrder(t *testing.T) {
	items := []evidence.Item{
		{Kind: evidence.KindPullRequestMerged, Description: "merged"},
		{Kind: evidence.KindSourceBranchMissing, Description: "missing"},
	}

	result := Evaluate(items)

	if len(result.Contributions) != len(items) {
		t.Fatalf(
			"contribution count = %d, want %d",
			len(result.Contributions),
			len(items),
		)
	}

	for i, item := range items {
		if result.Contributions[i].Kind != item.Kind {
			t.Fatalf(
				"contribution %d kind = %s, want %s",
				i,
				result.Contributions[i].Kind,
				item.Kind,
			)
		}

		if result.Contributions[i].Description != item.Description {
			t.Fatalf(
				"contribution %d description = %q, want %q",
				i,
				result.Contributions[i].Description,
				item.Description,
			)
		}
	}

	if result.Contributions[0].Points != weightPullRequestMerged {
		t.Fatalf(
			"contribution 0 points = %d, want %d",
			result.Contributions[0].Points,
			weightPullRequestMerged,
		)
	}

	if result.Contributions[1].Points != weightSourceBranchMissing {
		t.Fatalf(
			"contribution 1 points = %d, want %d",
			result.Contributions[1].Points,
			weightSourceBranchMissing,
		)
	}
}

// TestEvaluateEmptyEvidenceNeverRecommendsCleanup documents behavior on an
// input Collect never actually produces (it always emits a pull-request
// evidence item, even KindPullRequestNotCorrelated). Evaluate must still
// fail safe: no evidence must never translate into a cleanup
// recommendation.
func TestEvaluateEmptyEvidenceNeverRecommendsCleanup(t *testing.T) {
	result := Evaluate(nil)

	if result.Score != scoreMin {
		t.Fatalf(
			"Score = %d, want %d with no evidence at all",
			result.Score,
			scoreMin,
		)
	}

	if result.Confidence != ConfidenceLow {
		t.Fatalf(
			"Confidence = %s, want %s with no evidence at all",
			result.Confidence,
			ConfidenceLow,
		)
	}
}

func TestEvaluateRecommendedRequiresClearingTheMediumThreshold(t *testing.T) {
	tests := []struct {
		name          string
		items         []evidence.Item
		wantRecommend bool
	}{
		{
			name: "merged and missing branch is recommended",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestMerged},
				{Kind: evidence.KindSourceBranchMissing},
			},
			wantRecommend: true,
		},
		{
			name: "merged PR with the branch still present is not recommended",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestMerged},
				{Kind: evidence.KindSourceBranchExists},
			},
			wantRecommend: false,
		},
		{
			name: "naming match alone is not recommended",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestUnknown},
				{Kind: evidence.KindNamingConventionMatch},
			},
			wantRecommend: false,
		},
		{
			name: "a failed lookup alone is not recommended",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestUnknown},
				{Kind: evidence.KindLookupIncomplete},
			},
			wantRecommend: false,
		},
		{
			name: "protected resource is never recommended despite strong evidence",
			items: []evidence.Item{
				{Kind: evidence.KindProtectedResource},
				{Kind: evidence.KindPullRequestMerged},
				{Kind: evidence.KindSourceBranchMissing},
			},
			wantRecommend: false,
		},
		{
			name: "open pull request is never recommended",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestOpen},
			},
			wantRecommend: false,
		},
		{
			// merged (+30) + naming (+8) + mature (+5) = 43, which alone
			// clears thresholdMedium (35) - but the branch check itself
			// failed, so this must not be recommended despite the score.
			name: "high score from unrelated signals is not recommended when the branch check failed",
			items: []evidence.Item{
				{Kind: evidence.KindPullRequestMerged},
				{Kind: evidence.KindNamingConventionMatch},
				{Kind: evidence.KindResourceAgeMature},
				{Kind: evidence.KindLookupIncomplete},
			},
			wantRecommend: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Evaluate(test.items)

			if result.Recommended != test.wantRecommend {
				t.Fatalf(
					"Recommended = %v (score %d), want %v",
					result.Recommended,
					result.Score,
					test.wantRecommend,
				)
			}
		})
	}
}

func TestPolicyVersionIsSet(t *testing.T) {
	result := Evaluate([]evidence.Item{
		{Kind: evidence.KindPullRequestOpen},
	})

	if result.PolicyVersion != PolicyVersion {
		t.Fatalf(
			"PolicyVersion = %q, want %q",
			result.PolicyVersion,
			PolicyVersion,
		)
	}

	if result.PolicyVersion == "" {
		t.Fatal("PolicyVersion is empty")
	}
}
