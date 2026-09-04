package vercel

import (
	"strconv"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

type apiListDeploymentsResponse struct {
	Deployments []apiDeployment  `json:"deployments"`
	Pagination  apiPaginationRef `json:"pagination"`
}

type apiPaginationRef struct {
	Next *int64 `json:"next"`
}

// apiDeployment maps the fields Sweep needs from Vercel's deployment
// representation. meta is an arbitrary string map; for deployments
// connected to a GitHub repository, Vercel populates the git-related keys
// read in toDomain (githubCommitRef, githubCommitSha, githubPrId,
// githubOrg, githubRepo). Those keys are not enumerated in Vercel's REST
// API schema (meta is documented as a free-form map), but they are the
// same values Vercel exposes at build time as VERCEL_GIT_COMMIT_REF,
// VERCEL_GIT_COMMIT_SHA, VERCEL_GIT_PULL_REQUEST_ID, VERCEL_GIT_REPO_OWNER,
// and VERCEL_GIT_REPO_SLUG. Their absence is handled gracefully: Sweep
// simply cannot correlate the deployment with a pull request.
type apiDeployment struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	ProjectID string            `json:"projectId"`
	URL       string            `json:"url"`
	Created   int64             `json:"created"`
	Target    *string           `json:"target"`
	Meta      map[string]string `json:"meta"`
}

// toDomain converts Vercel's representation into Sweep's representation.
func (d apiDeployment) toDomain() domain.VercelDeployment {
	var target string
	if d.Target != nil {
		target = *d.Target
	}

	deployment := domain.VercelDeployment{
		ID:           d.UID,
		ProjectID:    d.ProjectID,
		Name:         d.Name,
		URL:          d.URL,
		Target:       target,
		GitBranch:    d.Meta["githubCommitRef"],
		GitCommitSHA: d.Meta["githubCommitSha"],
		CreatedAt:    time.UnixMilli(d.Created).UTC(),
	}

	if number, err := strconv.Atoi(d.Meta["githubPrId"]); err == nil {
		deployment.PullRequestNumber = &number
	}

	if org, repo := d.Meta["githubOrg"], d.Meta["githubRepo"]; org != "" &&
		repo != "" {
		deployment.SourceRepository = org + "/" + repo
	}

	return deployment
}
