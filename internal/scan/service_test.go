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

type branchLookup struct {
	repository string
	branch     string
}

type stubSourceControl struct {
	pullRequests     map[int]domain.PullRequest
	pullRequestErr   error
	pullRequestCalls []int

	branchExists bool
	branchErr    error
	branchCalls  []branchLookup
}

func (stub *stubSourceControl) GetPullRequest(
	_ context.Context,
	_ string,
	number int,
) (domain.PullRequest, error) {
	stub.pullRequestCalls = append(stub.pullRequestCalls, number)

	if stub.pullRequestErr != nil {
		return domain.PullRequest{}, stub.pullRequestErr
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

// BranchExists simulates GitHub branch lookup and records its input.
func (stub *stubSourceControl) BranchExists(
	_ context.Context,
	repository string,
	branch string,
) (bool, error) {
	stub.branchCalls = append(stub.branchCalls, branchLookup{
		repository: repository,
		branch:     branch,
	})

	return stub.branchExists, stub.branchErr
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

	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			1: {
				Repository:     "RishabJain30/sweep-lifecycle",
				Number:         1,
				HeadRepository: "RishabJain30/sweep-lifecycle",
				HeadBranch:     "feat/example",
				State:          domain.PullRequestStateMerged,
			},
		},
		branchExists: true,
	}

	service := NewService(branches, sourceControl)

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

	if !matches[0].SourceBranchExists {
		t.Fatal("SourceBranchExists = false, want true")
	}

	if len(sourceControl.pullRequestCalls) != 1 ||
		sourceControl.pullRequestCalls[0] != 1 {
		t.Fatalf(
			"GitHub PR calls = %v, want only PR 1",
			sourceControl.pullRequestCalls,
		)
	}

	if len(sourceControl.branchCalls) != 1 {
		t.Fatalf(
			"GitHub branch call count = %d, want 1",
			len(sourceControl.branchCalls),
		)
	}

	branchCall := sourceControl.branchCalls[0]

	if branchCall.repository != "RishabJain30/sweep-lifecycle" {
		t.Fatalf(
			"branch repository = %q, want %q",
			branchCall.repository,
			"RishabJain30/sweep-lifecycle",
		)
	}

	if branchCall.branch != "feat/example" {
		t.Fatalf(
			"branch = %q, want %q",
			branchCall.branch,
			"feat/example",
		)
	}
}

func TestServiceScanTreatsMissingHeadRepositoryAsMissingBranch(t *testing.T) {
	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			1: {
				Number:     1,
				HeadBranch: "feat/deleted-fork",
				State:      domain.PullRequestStateMerged,
			},
		},
	}

	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{Name: "preview-pr-1"},
			},
		},
		sourceControl,
	)

	matches, err := service.Scan(
		context.Background(),
		"RishabJain30/sweep-lifecycle",
		"test-project",
	)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("match count = %d, want 1", len(matches))
	}

	if matches[0].SourceBranchExists {
		t.Fatal("SourceBranchExists = true, want false")
	}

	if len(sourceControl.branchCalls) != 0 {
		t.Fatalf(
			"GitHub branch calls = %v, want none",
			sourceControl.branchCalls,
		)
	}
}

func TestServiceScanReturnsNeonError(t *testing.T) {
	service := NewService(
		stubBranchLister{
			err: errors.New("connection failed"),
		},
		&stubSourceControl{},
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

func TestServiceScanReturnsPullRequestError(t *testing.T) {
	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{Name: "preview-pr-1"},
			},
		},
		&stubSourceControl{
			pullRequestErr: errors.New("connection failed"),
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
			"error = %q, want GitHub PR context",
			err,
		)
	}
}

func TestServiceScanReturnsBranchLookupError(t *testing.T) {
	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			1: {
				Number:         1,
				HeadRepository: "RishabJain30/sweep-lifecycle",
				HeadBranch:     "feat/example",
			},
		},
		branchErr: errors.New("connection failed"),
	}

	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{Name: "preview-pr-1"},
			},
		},
		sourceControl,
	)

	_, err := service.Scan(
		context.Background(),
		"RishabJain30/sweep-lifecycle",
		"test-project",
	)
	if err == nil {
		t.Fatal("Scan() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "check GitHub branch") {
		t.Fatalf(
			"error = %q, want GitHub branch context",
			err,
		)
	}
}
