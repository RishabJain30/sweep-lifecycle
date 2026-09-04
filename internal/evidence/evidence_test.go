package evidence

import (
	"testing"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

func hasKind(items []Item, kind Kind) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}

	return false
}

func TestCollectIsDeterministic(t *testing.T) {
	input := Input{
		NameMatchesConvention: true,
		PullRequestCorrelated: true,
		PullRequestFound:      true,
		PullRequestState:      domain.PullRequestStateMerged,
		SourceBranchChecked:   true,
		SourceBranchExists:    false,
		ResourceCreatedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ResourceUpdatedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Now:                   time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
	}

	first := Collect(input)
	second := Collect(input)

	if len(first) != len(second) {
		t.Fatalf("item count = %d, want %d", len(second), len(first))
	}

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("item %d = %+v, want %+v", i, second[i], first[i])
		}
	}
}

func TestCollectProtectedResource(t *testing.T) {
	items := Collect(Input{ResourceProtected: true})

	if !hasKind(items, KindProtectedResource) {
		t.Fatalf("items = %+v, want %s", items, KindProtectedResource)
	}
}

func TestCollectPullRequestState(t *testing.T) {
	tests := []struct {
		name     string
		input    Input
		wantKind Kind
	}{
		{
			name:     "not correlated",
			input:    Input{PullRequestCorrelated: false},
			wantKind: KindPullRequestNotCorrelated,
		},
		{
			name: "correlated but not found",
			input: Input{
				PullRequestCorrelated: true,
				PullRequestFound:      false,
			},
			wantKind: KindPullRequestUnknown,
		},
		{
			name: "open",
			input: Input{
				PullRequestCorrelated: true,
				PullRequestFound:      true,
				PullRequestState:      domain.PullRequestStateOpen,
			},
			wantKind: KindPullRequestOpen,
		},
		{
			name: "closed",
			input: Input{
				PullRequestCorrelated: true,
				PullRequestFound:      true,
				PullRequestState:      domain.PullRequestStateClosed,
			},
			wantKind: KindPullRequestClosed,
		},
		{
			name: "merged",
			input: Input{
				PullRequestCorrelated: true,
				PullRequestFound:      true,
				PullRequestState:      domain.PullRequestStateMerged,
			},
			wantKind: KindPullRequestMerged,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := Collect(test.input)

			if !hasKind(items, test.wantKind) {
				t.Fatalf("items = %+v, want %s", items, test.wantKind)
			}
		})
	}
}

func TestCollectSourceBranchEvidence(t *testing.T) {
	tests := []struct {
		name     string
		input    Input
		wantKind Kind
	}{
		{
			name:     "unchecked",
			input:    Input{SourceBranchChecked: false},
			wantKind: KindSourceBranchUnchecked,
		},
		{
			name: "exists",
			input: Input{
				SourceBranchChecked: true,
				SourceBranchExists:  true,
			},
			wantKind: KindSourceBranchExists,
		},
		{
			name: "missing",
			input: Input{
				SourceBranchChecked: true,
				SourceBranchExists:  false,
			},
			wantKind: KindSourceBranchMissing,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := Collect(test.input)

			if !hasKind(items, test.wantKind) {
				t.Fatalf("items = %+v, want %s", items, test.wantKind)
			}
		})
	}
}

func TestCollectSourceRepositoryMissing(t *testing.T) {
	items := Collect(Input{SourceRepositoryMissing: true})

	if !hasKind(items, KindSourceRepositoryMissing) {
		t.Fatalf(
			"items = %+v, want %s",
			items,
			KindSourceRepositoryMissing,
		)
	}
}

func TestCollectResourceAge(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		updatedAt time.Time
		wantKind  Kind
	}{
		{
			name:      "recent",
			updatedAt: now.Add(-1 * time.Hour),
			wantKind:  KindResourceAgeRecent,
		},
		{
			name:      "just under threshold is recent",
			updatedAt: now.Add(-RecentResourceThreshold + time.Minute),
			wantKind:  KindResourceAgeRecent,
		},
		{
			name:      "at threshold is mature",
			updatedAt: now.Add(-RecentResourceThreshold),
			wantKind:  KindResourceAgeMature,
		},
		{
			name:      "mature",
			updatedAt: now.Add(-30 * 24 * time.Hour),
			wantKind:  KindResourceAgeMature,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := Collect(Input{
				ResourceUpdatedAt: test.updatedAt,
				Now:               now,
			})

			if !hasKind(items, test.wantKind) {
				t.Fatalf("items = %+v, want %s", items, test.wantKind)
			}
		})
	}
}

func TestCollectResourceAgeUsesLatestOfCreatedAndUpdated(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	items := Collect(Input{
		ResourceCreatedAt: now.Add(-30 * 24 * time.Hour),
		ResourceUpdatedAt: now.Add(-1 * time.Hour),
		Now:               now,
	})

	if !hasKind(items, KindResourceAgeRecent) {
		t.Fatalf(
			"items = %+v, want %s (most recent touch should win)",
			items,
			KindResourceAgeRecent,
		)
	}
}

func TestCollectSkipsAgeEvidenceWithoutTimestamps(t *testing.T) {
	items := Collect(Input{})

	if hasKind(items, KindResourceAgeRecent) ||
		hasKind(items, KindResourceAgeMature) {
		t.Fatalf(
			"items = %+v, want no age evidence without timestamps",
			items,
		)
	}
}

func TestCollectLookupIncomplete(t *testing.T) {
	items := Collect(Input{
		LookupIncomplete:       true,
		LookupIncompleteReason: "the GitHub branch check failed",
	})

	if !hasKind(items, KindLookupIncomplete) {
		t.Fatalf("items = %+v, want %s", items, KindLookupIncomplete)
	}
}
