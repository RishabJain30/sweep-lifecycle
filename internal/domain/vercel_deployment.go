package domain

import "time"

// VercelDeployment is Sweep's internal representation of a Vercel
// deployment.
type VercelDeployment struct {
	ID        string
	ProjectID string
	Name      string
	URL       string

	// Target is "production", "staging", or "" for an ordinary preview
	// deployment.
	Target string

	// PullRequestNumber is set when Vercel's Git metadata identifies the
	// pull request that triggered this deployment.
	PullRequestNumber *int
	GitBranch         string
	GitCommitSHA      string
	// SourceRepository is "owner/repo" when Vercel's Git metadata includes
	// it, and empty otherwise.
	SourceRepository string

	CreatedAt time.Time
}

// IsProtected reports whether Sweep must exclude the deployment from
// cleanup. Production deployments serve live traffic.
func (deployment VercelDeployment) IsProtected() bool {
	return deployment.Target == "production"
}
