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

// SourceControl provides the GitHub lifecycle information needed by a scan.
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

type Match struct {
	Branch             domain.DatabaseBranch
	PullRequest        domain.PullRequest
	SourceBranchExists bool
}

type Service struct {
	branches      BranchLister
	sourceControl SourceControl
}

// NewService creates a scan service using the supplied provider clients.
func NewService(
	branches BranchLister,
	sourceControl SourceControl,
) *Service {
	return &Service{
		branches:      branches,
		sourceControl: sourceControl,
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

		pr, err := service.sourceControl.GetPullRequest(
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

		sourceBranchExists := false

		// An empty head repository means GitHub no longer exposes the
		// repository that originally contained the source branch.
		if pr.HeadRepository != "" {
			sourceBranchExists, err = service.sourceControl.BranchExists(
				ctx,
				pr.HeadRepository,
				pr.HeadBranch,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"check GitHub branch %s:%s for PR #%d: %w",
					pr.HeadRepository,
					pr.HeadBranch,
					pr.Number,
					err,
				)
			}
		}

		matches = append(matches, Match{
			Branch:             branch,
			PullRequest:        pr,
			SourceBranchExists: sourceBranchExists,
		})
	}

	return matches, nil
}
