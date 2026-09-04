package scan

import (
	"context"
	"fmt"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/correlation"
	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

// BranchLister discovers Neon database branches.
type BranchLister interface {
	ListBranches(
		ctx context.Context,
		projectID string,
	) ([]domain.DatabaseBranch, error)
}

// DeploymentLister discovers Vercel preview deployments.
type DeploymentLister interface {
	ListDeployments(
		ctx context.Context,
		projectID string,
	) ([]domain.VercelDeployment, error)
}

// Clock returns the current time. Scans inject it so evidence age and
// scoring stay deterministic in tests.
type Clock func() time.Time

// Skipped is a resource Sweep evaluated but excluded from cleanup
// candidates, along with the deterministic reason it was excluded.
type Skipped struct {
	Provider     string
	ResourceID   string
	ResourceName string
	Reason       string
}

// Warning reports a partial, per-resource provider failure. Other
// successful results are unaffected.
type Warning struct {
	Provider string
	Resource string
	Message  string
}

// ProviderStatus reports whether a provider was queried, skipped because
// it is not configured, and how many resources it returned.
type ProviderStatus struct {
	Provider string
	Status   string // "ok" or "skipped"
	Detail   string
}

// Result is the complete, honest outcome of one scan.
type Result struct {
	ProviderStatuses []ProviderStatus
	Candidates       []Candidate
	Skipped          []Skipped
	Warnings         []Warning
}

// Config configures one scan.
type Config struct {
	Repository      string
	NeonProjectID   string
	VercelProjectID string
}

// Service correlates preview infrastructure across providers with GitHub
// pull requests. Vercel is optional: a Service constructed with a nil
// DeploymentLister only scans Neon.
type Service struct {
	neon   BranchLister
	github SourceControl
	vercel DeploymentLister
	now    Clock
}

// NewService creates a scan service using the supplied provider clients.
// vercel may be nil when the Vercel provider is not configured.
func NewService(
	neon BranchLister,
	github SourceControl,
	vercel DeploymentLister,
	now Clock,
) *Service {
	if now == nil {
		now = time.Now
	}

	return &Service{
		neon:   neon,
		github: github,
		vercel: vercel,
		now:    now,
	}
}

// Scan discovers preview resources from every configured provider,
// correlates each with a GitHub pull request, and evaluates deterministic
// evidence and score for every result. A discovery failure for a
// configured provider fails the whole scan; a failure evaluating one
// resource is recorded as a Warning and every other resource is still
// reported.
func (service *Service) Scan(
	ctx context.Context,
	cfg Config,
) (Result, error) {
	now := service.now()

	var result Result

	branches, err := service.neon.ListBranches(ctx, cfg.NeonProjectID)
	if err != nil {
		return Result{}, fmt.Errorf("list Neon branches: %w", err)
	}

	result.ProviderStatuses = append(result.ProviderStatuses, ProviderStatus{
		Provider: "neon",
		Status:   "ok",
		Detail:   fmt.Sprintf("discovered %d branch(es)", len(branches)),
	})

	for _, branch := range branches {
		service.evaluateNeonBranch(ctx, cfg.Repository, branch, now, &result)
	}

	if service.vercel == nil {
		result.ProviderStatuses = append(
			result.ProviderStatuses,
			ProviderStatus{
				Provider: "vercel",
				Status:   "skipped",
				Detail:   "Vercel is not configured",
			},
		)

		return result, nil
	}

	deployments, err := service.vercel.ListDeployments(
		ctx,
		cfg.VercelProjectID,
	)
	if err != nil {
		return Result{}, fmt.Errorf("list Vercel deployments: %w", err)
	}

	result.ProviderStatuses = append(result.ProviderStatuses, ProviderStatus{
		Provider: "vercel",
		Status:   "ok",
		Detail:   fmt.Sprintf("discovered %d deployment(s)", len(deployments)),
	})

	for _, deployment := range deployments {
		service.evaluateVercelDeployment(
			ctx,
			cfg.Repository,
			deployment,
			now,
			&result,
		)
	}

	return result, nil
}

func (service *Service) evaluateNeonBranch(
	ctx context.Context,
	repository string,
	branch domain.DatabaseBranch,
	now time.Time,
	result *Result,
) {
	prNumber, matched := correlation.ExtractPullRequestNumber(branch.Name)

	candidate, warning := EvaluateResource(
		ctx,
		service.github,
		repository,
		ResourceInput{
			Provider:              "neon",
			ResourceID:            branch.ID,
			ResourceName:          branch.Name,
			ResourceProtected:     branch.IsProtected(),
			ResourceCreatedAt:     branch.CreatedAt,
			ResourceUpdatedAt:     branch.UpdatedAt,
			NameMatchesConvention: matched,
			PullRequestNumber:     prNumber,
		},
		now,
	)

	record(result, candidate, warning)
}

func (service *Service) evaluateVercelDeployment(
	ctx context.Context,
	repository string,
	deployment domain.VercelDeployment,
	now time.Time,
	result *Result,
) {
	var prNumber int
	if deployment.PullRequestNumber != nil {
		prNumber = *deployment.PullRequestNumber
	}

	candidate, warning := EvaluateResource(
		ctx,
		service.github,
		DeploymentRepository(deployment, repository),
		ResourceInput{
			Provider:          "vercel",
			ResourceID:        deployment.ID,
			ResourceName:      deployment.Name,
			ResourceProtected: deployment.IsProtected(),
			ResourceCreatedAt: deployment.CreatedAt,
			ResourceUpdatedAt: deployment.CreatedAt,
			PullRequestNumber: prNumber,
		},
		now,
	)

	record(result, candidate, warning)
}

// DeploymentRepository reports which GitHub repository to correlate a
// Vercel deployment's pull request against. A deployment's own Git
// metadata is authoritative when Vercel reports it: a Vercel project can
// be connected to a different repository than --repo names (a fork, a
// rename, or a multi-repo setup), and querying the wrong repository could
// correlate a deployment with an unrelated pull request. fallbackRepository
// is only used for deployments with no Git metadata at all. scan.Service
// and the explain command both call this so a deployment is never
// correlated against the wrong repository in one path but not the other.
func DeploymentRepository(
	deployment domain.VercelDeployment,
	fallbackRepository string,
) string {
	if deployment.SourceRepository != "" {
		return deployment.SourceRepository
	}

	return fallbackRepository
}

func record(result *Result, candidate Candidate, warning string) {
	if warning != "" {
		result.Warnings = append(result.Warnings, Warning{
			Provider: candidate.Provider,
			Resource: candidate.ResourceID,
			Message:  warning,
		})
	}

	if candidate.Score.Excluded {
		result.Skipped = append(result.Skipped, Skipped{
			Provider:     candidate.Provider,
			ResourceID:   candidate.ResourceID,
			ResourceName: candidate.ResourceName,
			Reason:       candidate.Score.ExclusionReason,
		})

		return
	}

	result.Candidates = append(result.Candidates, candidate)
}
