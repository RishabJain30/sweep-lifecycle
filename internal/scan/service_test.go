package scan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
	"github.com/RishabJain30/sweep-lifecycle/internal/scoring"
)

var fixedNow = time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return fixedNow }

// mature is well past evidence.RecentResourceThreshold before fixedNow, so
// tests are not sensitive to the exact recency window.
var mature = fixedNow.Add(-30 * 24 * time.Hour)

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

type stubDeploymentLister struct {
	deployments []domain.VercelDeployment
	err         error
}

func (stub stubDeploymentLister) ListDeployments(
	_ context.Context,
	_ string,
) ([]domain.VercelDeployment, error) {
	return stub.deployments, stub.err
}

type branchLookup struct {
	repository string
	branch     string
}

type stubSourceControl struct {
	pullRequests            map[int]domain.PullRequest
	pullRequestErr          error
	pullRequestCalls        []int
	pullRequestRepositories []string

	branchExists bool
	branchErr    error
	branchCalls  []branchLookup
}

func (stub *stubSourceControl) GetPullRequest(
	_ context.Context,
	repository string,
	number int,
) (domain.PullRequest, error) {
	stub.pullRequestCalls = append(stub.pullRequestCalls, number)
	stub.pullRequestRepositories = append(
		stub.pullRequestRepositories,
		repository,
	)

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

func findCandidate(candidates []Candidate, resourceID string) (Candidate, bool) {
	for _, candidate := range candidates {
		if candidate.ResourceID == resourceID {
			return candidate, true
		}
	}

	return Candidate{}, false
}

func findSkipped(skipped []Skipped, resourceID string) (Skipped, bool) {
	for _, entry := range skipped {
		if entry.ResourceID == resourceID {
			return entry, true
		}
	}

	return Skipped{}, false
}

func TestServiceScanCorrelatesPreviewBranch(t *testing.T) {
	branches := stubBranchLister{
		branches: []domain.DatabaseBranch{
			{Name: "production", Default: true},
			{
				ID:        "br-preview",
				Name:      "preview-pr-1",
				CreatedAt: mature,
				UpdatedAt: mature,
			},
			{ID: "br-manual", Name: "manual-test"},
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
		branchExists: false,
	}

	service := NewService(branches, sourceControl, nil, fixedClock)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf(
			"candidate count = %d, want %d: %+v",
			len(result.Candidates),
			1,
			result.Candidates,
		)
	}

	candidate := result.Candidates[0]

	if candidate.ResourceName != "preview-pr-1" {
		t.Fatalf(
			"resource name = %q, want %q",
			candidate.ResourceName,
			"preview-pr-1",
		)
	}

	if candidate.PullRequest.Number != 1 {
		t.Fatalf(
			"PR number = %d, want %d",
			candidate.PullRequest.Number,
			1,
		)
	}

	if candidate.SourceBranchExists {
		t.Fatal("SourceBranchExists = true, want false")
	}

	if candidate.Score.Confidence != scoring.ConfidenceHigh {
		t.Fatalf(
			"Confidence = %s, want %s",
			candidate.Score.Confidence,
			scoring.ConfidenceHigh,
		)
	}

	// production is protected and manual-test does not match the naming
	// convention: both must be skipped, not candidates or warnings.
	if len(result.Skipped) != 2 {
		t.Fatalf(
			"skipped count = %d, want %d: %+v",
			len(result.Skipped),
			2,
			result.Skipped,
		)
	}

	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %+v, want none", result.Warnings)
	}

	if len(sourceControl.pullRequestCalls) != 1 ||
		sourceControl.pullRequestCalls[0] != 1 {
		t.Fatalf(
			"GitHub PR calls = %v, want only PR 1",
			sourceControl.pullRequestCalls,
		)
	}

	branchCall := sourceControl.branchCalls[0]
	if branchCall.repository != "RishabJain30/sweep-lifecycle" ||
		branchCall.branch != "feat/example" {
		t.Fatalf("unexpected branch call: %+v", branchCall)
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
				{
					Name:      "preview-pr-1",
					CreatedAt: mature,
					UpdatedAt: mature,
				},
			},
		},
		sourceControl,
		nil,
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(result.Candidates))
	}

	if result.Candidates[0].SourceBranchExists {
		t.Fatal("SourceBranchExists = true, want false")
	}

	if result.Candidates[0].SourceBranchChecked {
		t.Fatal(
			"SourceBranchChecked = true, want false when the head " +
				"repository is missing",
		)
	}

	if len(sourceControl.branchCalls) != 0 {
		t.Fatalf(
			"GitHub branch calls = %v, want none",
			sourceControl.branchCalls,
		)
	}
}

func TestServiceScanReturnsNeonDiscoveryError(t *testing.T) {
	service := NewService(
		stubBranchLister{err: errors.New("connection failed")},
		&stubSourceControl{},
		nil,
		fixedClock,
	)

	_, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err == nil {
		t.Fatal("Scan() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "list Neon branches") {
		t.Fatalf("error = %q, want Neon context", err)
	}
}

func TestServiceScanPullRequestFailureBecomesWarningNotHardFailure(t *testing.T) {
	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{
					Name:      "preview-pr-1",
					CreatedAt: mature,
					UpdatedAt: mature,
				},
			},
		},
		&stubSourceControl{
			pullRequestErr: errors.New("connection failed"),
		},
		nil,
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf(
			"Scan() error = %v, want no error (partial failures are "+
				"warnings)",
			err,
		)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("warnings = %+v, want exactly one", result.Warnings)
	}

	if !strings.Contains(result.Warnings[0].Message, "pull request #1") {
		t.Fatalf(
			"warning message = %q, want it to mention pull request #1",
			result.Warnings[0].Message,
		)
	}

	if len(result.Candidates) != 0 {
		t.Fatalf(
			"candidates = %+v, want none: incomplete evidence must "+
				"never be reported as a cleanup candidate",
			result.Candidates,
		)
	}

	candidate, found := findCandidate(result.Uncertain, "")
	if !found {
		t.Fatalf(
			"uncertain = %+v, want the unresolved PR to still be "+
				"reported",
			result.Uncertain,
		)
	}

	if candidate.Score.Confidence != scoring.ConfidenceLow {
		t.Fatalf(
			"Confidence = %s, want %s when the pull request lookup failed",
			candidate.Score.Confidence,
			scoring.ConfidenceLow,
		)
	}
}

func TestServiceScanBranchLookupFailureBecomesWarningNotHardFailure(t *testing.T) {
	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			1: {
				Number:         1,
				HeadRepository: "RishabJain30/sweep-lifecycle",
				HeadBranch:     "feat/example",
				State:          domain.PullRequestStateMerged,
			},
		},
		branchErr: errors.New("connection failed"),
	}

	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{
					ID:        "br-preview",
					Name:      "preview-pr-1",
					CreatedAt: mature,
					UpdatedAt: mature,
				},
			},
		},
		sourceControl,
		nil,
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v, want no error", err)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("warnings = %+v, want exactly one", result.Warnings)
	}

	if !strings.Contains(result.Warnings[0].Message, "feat/example") {
		t.Fatalf(
			"warning message = %q, want it to mention the source branch",
			result.Warnings[0].Message,
		)
	}

	if len(result.Candidates) != 0 {
		t.Fatalf(
			"candidates = %+v, want none: incomplete evidence must "+
				"never be reported as a cleanup candidate",
			result.Candidates,
		)
	}

	candidate, found := findCandidate(result.Uncertain, "br-preview")
	if !found {
		t.Fatalf("uncertain = %+v, want br-preview", result.Uncertain)
	}

	if candidate.Score.Confidence != scoring.ConfidenceLow {
		t.Fatalf(
			"Confidence = %s, want %s when the branch check failed",
			candidate.Score.Confidence,
			scoring.ConfidenceLow,
		)
	}
}

// TestServiceScanMergedPRWithExistingBranchIsUncertainNotCandidate covers
// a merged PR whose branch was never deleted: score clamps to 0 (the
// existing branch is a strong negative signal), so it must be reported as
// Uncertain, not presented as a cleanup candidate.
func TestServiceScanMergedPRWithExistingBranchIsUncertainNotCandidate(t *testing.T) {
	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			1: {
				Number:         1,
				State:          domain.PullRequestStateMerged,
				HeadRepository: "RishabJain30/sweep-lifecycle",
				HeadBranch:     "feat/example",
			},
		},
		branchExists: true,
	}

	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{
					ID:        "br-preview",
					Name:      "preview-pr-1",
					CreatedAt: mature,
					UpdatedAt: mature,
				},
			},
		},
		sourceControl,
		nil,
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.Candidates) != 0 {
		t.Fatalf(
			"candidates = %+v, want none: a merged PR whose branch "+
				"still exists must never be a cleanup candidate",
			result.Candidates,
		)
	}

	if _, found := findCandidate(result.Uncertain, "br-preview"); !found {
		t.Fatalf(
			"uncertain = %+v, want br-preview reported as uncertain",
			result.Uncertain,
		)
	}
}

func TestServiceScanProtectedBranchIsSkippedNotCandidate(t *testing.T) {
	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{ID: "br-prod", Name: "production", Default: true},
			},
		},
		&stubSourceControl{},
		nil,
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.Candidates) != 0 {
		t.Fatalf(
			"candidates = %+v, want none for a protected branch",
			result.Candidates,
		)
	}

	skipped, found := findSkipped(result.Skipped, "br-prod")
	if !found {
		t.Fatalf("skipped = %+v, want br-prod", result.Skipped)
	}

	if !strings.Contains(skipped.Reason, "protected") {
		t.Fatalf("reason = %q, want it to mention protected", skipped.Reason)
	}
}

func TestServiceScanOpenPullRequestIsSkippedNotCandidate(t *testing.T) {
	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{ID: "br-preview", Name: "preview-pr-1"},
			},
		},
		&stubSourceControl{
			pullRequests: map[int]domain.PullRequest{
				1: {Number: 1, State: domain.PullRequestStateOpen},
			},
		},
		nil,
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	skipped, found := findSkipped(result.Skipped, "br-preview")
	if !found {
		t.Fatalf(
			"skipped = %+v, want the open pull request to be skipped",
			result.Skipped,
		)
	}

	if !strings.Contains(skipped.Reason, "open") {
		t.Fatalf("reason = %q, want it to mention open", skipped.Reason)
	}
}

func TestServiceScanUncorrelatedBranchIsSkipped(t *testing.T) {
	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{ID: "br-manual", Name: "manual-test"},
			},
		},
		&stubSourceControl{},
		nil,
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	skipped, found := findSkipped(result.Skipped, "br-manual")
	if !found {
		t.Fatalf(
			"skipped = %+v, want the uncorrelated branch to be skipped",
			result.Skipped,
		)
	}

	if !strings.Contains(skipped.Reason, "no pull request") {
		t.Fatalf(
			"reason = %q, want it to mention no pull request",
			skipped.Reason,
		)
	}
}

func TestServiceScanVercelIsSkippedWhenNotConfigured(t *testing.T) {
	service := NewService(
		stubBranchLister{},
		&stubSourceControl{},
		nil,
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:    "RishabJain30/sweep-lifecycle",
		NeonProjectID: "test-project",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	var vercelStatus ProviderStatus
	found := false

	for _, status := range result.ProviderStatuses {
		if status.Provider == "vercel" {
			vercelStatus = status
			found = true
		}
	}

	if !found {
		t.Fatalf(
			"provider statuses = %+v, want a vercel entry",
			result.ProviderStatuses,
		)
	}

	if vercelStatus.Status != "skipped" {
		t.Fatalf(
			"vercel status = %q, want %q",
			vercelStatus.Status,
			"skipped",
		)
	}
}

func TestServiceScanDiscoversVercelDeployments(t *testing.T) {
	prNumber := 9

	service := NewService(
		stubBranchLister{},
		&stubSourceControl{
			pullRequests: map[int]domain.PullRequest{
				9: {
					Number:         9,
					HeadRepository: "RishabJain30/sweep-lifecycle",
					HeadBranch:     "feat/vercel-example",
					State:          domain.PullRequestStateMerged,
				},
			},
		},
		stubDeploymentLister{
			deployments: []domain.VercelDeployment{
				{
					ID:                "dpl_1",
					Name:              "app",
					PullRequestNumber: &prNumber,
					CreatedAt:         mature,
				},
			},
		},
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:      "RishabJain30/sweep-lifecycle",
		NeonProjectID:   "test-project",
		VercelProjectID: "prj_1",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	candidate, found := findCandidate(result.Candidates, "dpl_1")
	if !found {
		t.Fatalf(
			"candidates = %+v, want the Vercel deployment to be evaluated",
			result.Candidates,
		)
	}

	if candidate.Provider != "vercel" {
		t.Fatalf("Provider = %q, want %q", candidate.Provider, "vercel")
	}

	if candidate.PullRequest.Number != 9 {
		t.Fatalf(
			"PR number = %d, want %d",
			candidate.PullRequest.Number,
			9,
		)
	}
}

func TestServiceScanExcludesProductionVercelDeployment(t *testing.T) {
	prNumber := 9

	service := NewService(
		stubBranchLister{},
		&stubSourceControl{},
		stubDeploymentLister{
			deployments: []domain.VercelDeployment{
				{
					ID:                "dpl_prod",
					Name:              "app",
					Target:            "production",
					PullRequestNumber: &prNumber,
				},
			},
		},
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:      "RishabJain30/sweep-lifecycle",
		VercelProjectID: "prj_1",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if _, found := findCandidate(result.Candidates, "dpl_prod"); found {
		t.Fatal("production deployment must never be a candidate")
	}

	skipped, found := findSkipped(result.Skipped, "dpl_prod")
	if !found {
		t.Fatalf("skipped = %+v, want dpl_prod", result.Skipped)
	}

	if !strings.Contains(skipped.Reason, "production") {
		t.Fatalf(
			"reason = %q, want it to mention production",
			skipped.Reason,
		)
	}
}

func TestServiceScanCorrelatesVercelDeploymentAgainstItsOwnRepository(t *testing.T) {
	prNumber := 9

	sourceControl := &stubSourceControl{
		pullRequests: map[int]domain.PullRequest{
			9: {Number: 9, State: domain.PullRequestStateMerged},
		},
	}

	service := NewService(
		stubBranchLister{},
		sourceControl,
		stubDeploymentLister{
			deployments: []domain.VercelDeployment{
				{
					ID:                "dpl_1",
					Name:              "app",
					PullRequestNumber: &prNumber,
					SourceRepository:  "RishabJain30/other-repo",
					CreatedAt:         mature,
				},
			},
		},
		fixedClock,
	)

	_, err := service.Scan(context.Background(), Config{
		// --repo names a different repository than the deployment's own
		// Git metadata: the deployment's metadata must win.
		Repository:      "RishabJain30/sweep-lifecycle",
		VercelProjectID: "prj_1",
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(sourceControl.pullRequestRepositories) != 1 {
		t.Fatalf(
			"PR lookups = %v, want exactly one",
			sourceControl.pullRequestRepositories,
		)
	}

	if got := sourceControl.pullRequestRepositories[0]; got != "RishabJain30/other-repo" {
		t.Fatalf(
			"repository queried = %q, want the deployment's own "+
				"SourceRepository %q, not --repo",
			got,
			"RishabJain30/other-repo",
		)
	}
}

func TestDeploymentRepositoryPrefersSourceRepositoryOverFallback(t *testing.T) {
	tests := []struct {
		name             string
		sourceRepository string
		fallback         string
		want             string
	}{
		{
			name:             "uses source repository when present",
			sourceRepository: "RishabJain30/web",
			fallback:         "RishabJain30/other",
			want:             "RishabJain30/web",
		},
		{
			name:             "falls back when source repository is empty",
			sourceRepository: "",
			fallback:         "RishabJain30/other",
			want:             "RishabJain30/other",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := DeploymentRepository(
				domain.VercelDeployment{SourceRepository: test.sourceRepository},
				test.fallback,
			)

			if got != test.want {
				t.Fatalf("DeploymentRepository() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestServiceScanVercelDiscoveryFailurePreservesNeonResults ensures a
// failure discovering the optional Vercel provider never discards Neon
// results already gathered: Vercel is reported as a failed ProviderStatus,
// but the scan as a whole still succeeds with everything Neon found.
func TestServiceScanVercelDiscoveryFailurePreservesNeonResults(t *testing.T) {
	service := NewService(
		stubBranchLister{
			branches: []domain.DatabaseBranch{
				{
					ID:        "br-preview",
					Name:      "preview-pr-1",
					CreatedAt: mature,
					UpdatedAt: mature,
				},
			},
		},
		&stubSourceControl{
			pullRequests: map[int]domain.PullRequest{
				1: {
					Number:         1,
					State:          domain.PullRequestStateMerged,
					HeadRepository: "RishabJain30/sweep-lifecycle",
					HeadBranch:     "feat/example",
				},
			},
		},
		stubDeploymentLister{err: errors.New("connection failed")},
		fixedClock,
	)

	result, err := service.Scan(context.Background(), Config{
		Repository:      "RishabJain30/sweep-lifecycle",
		NeonProjectID:   "test-project",
		VercelProjectID: "prj_1",
	})
	if err != nil {
		t.Fatalf(
			"Scan() error = %v, want no error: an optional provider's "+
				"discovery failure must not fail the whole scan",
			err,
		)
	}

	if _, found := findCandidate(result.Candidates, "br-preview"); !found {
		t.Fatalf(
			"candidates = %+v, want the Neon result preserved despite "+
				"the Vercel failure",
			result.Candidates,
		)
	}

	var vercelStatus ProviderStatus
	found := false

	for _, status := range result.ProviderStatuses {
		if status.Provider == "vercel" {
			vercelStatus = status
			found = true
		}
	}

	if !found {
		t.Fatalf(
			"provider statuses = %+v, want a vercel entry",
			result.ProviderStatuses,
		)
	}

	if vercelStatus.Status != "error" {
		t.Fatalf(
			"vercel status = %q, want %q",
			vercelStatus.Status,
			"error",
		)
	}

	if !strings.Contains(vercelStatus.Detail, "connection failed") {
		t.Fatalf(
			"vercel detail = %q, want it to explain the failure",
			vercelStatus.Detail,
		)
	}
}
