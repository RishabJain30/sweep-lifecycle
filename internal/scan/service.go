package scan

import (
	"context"
	"fmt"

	"github.com/RishabJain30/sweep-lifecycle/internal/correlation"
	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

type BranchLister interface {
	ListBranches(
		ctx context.Context,
		projectID string,
	) ([]domain.DatabaseBranch, error)
}

type PullRequestGetter interface {
	GetPullRequest(
		ctx context.Context,
		repository string,
		number int,
	) (domain.PullRequest, error)
}

type Match struct {
	Branch      domain.DatabaseBranch
	PullRequest domain.PullRequest
}

type Service struct {
	branches     BranchLister
	pullRequests PullRequestGetter
}

// NewService creates a scan service using the supplied provider clients.
func NewService(
	branches BranchLister,
	pullRequests PullRequestGetter,
) *Service {
	return &Service{
		branches:     branches,
		pullRequests: pullRequests,
	}
}

// Scan correlates unprotected Neon branches with GitHub pull requests.
func (service *Service) Scan(
	ctx context.Context,
	repository string,
	projectID string,
) ([]Match, error) {
	branches, err := service.branches.ListBranches(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list Neon branches: %w", err)
	}

	var matches []Match

	for _, branch := range branches {
		if branch.IsProtected() {
			continue
		}

		prNumber, matched := correlation.ExtractPullRequestNumber(branch.Name)
		if !matched {
			continue
		}

		pr, err := service.pullRequests.GetPullRequest(
			ctx,
			repository,
			prNumber,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"get GitHub PR %s#%d for branch %q: %w",
				repository,
				prNumber,
				branch.Name,
				err,
			)
		}

		matches = append(matches, Match{
			Branch:      branch,
			PullRequest: pr,
		})
	}

	return matches, nil
}
