package github

import (
	"fmt"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

type apiPullRequest struct {
	Number   int        `json:"number"`
	State    string     `json:"state"`
	ClosedAt *time.Time `json:"closed_at"`
	MergedAt *time.Time `json:"merged_at"`

	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`

		Repository *struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`

	Base struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"base"`
}

func (pr apiPullRequest) toDomain() (domain.PullRequest, error) {
	state, err := mapPullRequestState(pr.State, pr.MergedAt)
	var headRepository string

	if pr.Head.Repository != nil {
		headRepository = pr.Head.Repository.FullName
	}
	if err != nil {
		return domain.PullRequest{}, err
	}

	return domain.PullRequest{
		Repository:     pr.Base.Repository.FullName,
		Number:         pr.Number,
		HeadRepository: headRepository,
		HeadBranch:     pr.Head.Ref,
		HeadSHA:        pr.Head.SHA,
		State:          state,
		ClosedAt:       pr.ClosedAt,
		MergedAt:       pr.MergedAt,
	}, nil
}

func mapPullRequestState(
	state string,
	mergedAt *time.Time,
) (domain.PullRequestState, error) {
	switch {
	case state == "open":
		return domain.PullRequestStateOpen, nil
	case state == "closed" && mergedAt != nil:
		return domain.PullRequestStateMerged, nil
	case state == "closed":
		return domain.PullRequestStateClosed, nil
	default:
		return "", fmt.Errorf("unsupported GitHub pull request state %q", state)
	}
}
