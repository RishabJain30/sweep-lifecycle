package scan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

type stubBranchLister struct {
	branches []domain.DatabaseBranch
	err      error
}

func (stub stubBranchLister) ListBranches(
	_ context.Context,
	_ string,
) ([]domain.DatabaseBranch, error) {
	return stub.branches, stub.err
}

type stubPullRequestGetter struct {
	pullRequests map[int]domain.PullRequest
	err          error
	calls        []int
}

func (stub *stubPullRequestGetter) GetPullRequest(
	_ context.Context,
	_ string,
	number int,
) (domain.PullRequest, error) {
	stub.calls = append(stub.calls, number)

	if stub.err != nil {
		return domain.PullRequest{}, stub.err
	}

	pr, found := stub.pullRequests[number]
	if !found {
		return domain.PullRequest{}, fmt.Errorf(
			"pull request %d not found",
			number,
		)
	}

	return pr, nil
}

func TestServiceScanCorrelatesPreviewBranch(t *testing.T) {
	branches := stubBranchLister{
		branches: []domain.DatabaseBranch{
			{
				Name:    "production",
				Default: true,
			},
			{
				ID:   "br-preview",
				Name: "preview-pr-1",
			},
			{
				ID:   "br-manual",
				Name: "manual-test",
			},
		},
	}

	pullRequests := &stubPullRequestGetter{
		pullRequests: map[int]domain.PullRequest{
			1: {
				Repository: "RishabJain30/sweep-lifecycle",
				Number:     1,
				State:      domain.PullRequestStateMerged,
			},
		},
	}

	service := NewService(branches, pullRequests)

	matches, err := service.Scan(
		context.Background(),
		"RishabJain30/sweep-lifecycle",
		"test-project",
	)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("match count = %d, want %d", len(matches), 1)
	}

	if matches[0].Branch.Name != "preview-pr-1" {
		t.Fatalf(
			"branch name = %q, want %q",
			matches[0].Branch.Name,
			"preview-pr-1",
		)
	}

	if matches[0].PullRequest.Number != 1 {
		t.Fatalf(
			"PR number = %d, want %d",
			matches[0].PullRequest.Number,
			1,
		)
	}

	if len(pullRequests.calls) != 1 || pullRequests.calls[0] != 1 {
		t.Fatalf(
			"GitHub calls = %v, want only PR 1",
			pullRequests.calls,
		)
	}
}

func TestServiceScanReturnsNeonError(t *testing.T) {
	service := NewService(
		stubBranchLister{
			err: errors.New("connection failed"),
		},
		&stubPullRequestGetter{},
	)

	_, err := service.Scan(
		context.Background(),
		"RishabJain30/sweep-lifecycle",
		"test-project",
	)
	if err == nil {
		t.Fatal("Scan() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "list Neon branches") {
		t.Fatalf(
			"error = %q, want Neon context",
			err,
		)
	}
}

func TestServiceScanReturnsGitHubError(t *testing.T) {
	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{Name: "preview-pr-1"},
			},
		},
		&stubPullRequestGetter{
			err: errors.New("connection failed"),
		},
	)

	_, err := service.Scan(
		context.Background(),
		"RishabJain30/sweep-lifecycle",
		"test-project",
	)
	if err == nil {
		t.Fatal("Scan() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "get GitHub PR") {
		t.Fatalf(
			"error = %q, want GitHub context",
			err,
		)
	}
}
