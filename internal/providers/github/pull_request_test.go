package github

import (
	"encoding/json"
	"testing"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

func TestAPIPullRequestToDomain(t *testing.T) {
	const response = `{
		"number": 14343,
		"state": "closed",
		"closed_at": "2026-09-04T09:02:01Z",
		"merged_at": "2026-09-04T09:02:01Z",
		"head": {
			"ref": "issue-triage-permalinks",
			"sha": "92029a9e735475ab83b9daffeff500c92a9fe2ad"
		},
		"base": {
			"repo": {
				"full_name": "cli/cli"
			}
		}
	}`

	var apiPR apiPullRequest
	if err := json.Unmarshal([]byte(response), &apiPR); err != nil {
		t.Fatalf("decode API response: %v", err)
	}

	pr, err := apiPR.toDomain()
	if err != nil {
		t.Fatalf("convert API pull request: %v", err)
	}

	if pr.Repository != "cli/cli" {
		t.Fatalf("Repository = %q, want %q", pr.Repository, "cli/cli")
	}

	if pr.Number != 14343 {
		t.Fatalf("Number = %d, want %d", pr.Number, 14343)
	}

	if pr.State != domain.PullRequestStateMerged {
		t.Fatalf(
			"State = %q, want %q",
			pr.State,
			domain.PullRequestStateMerged,
		)
	}

	if pr.HeadBranch != "issue-triage-permalinks" {
		t.Fatalf(
			"HeadBranch = %q, want %q",
			pr.HeadBranch,
			"issue-triage-permalinks",
		)
	}

	if pr.MergedAt == nil {
		t.Fatal("MergedAt is nil, want a timestamp")
	}
}
