package domain

type PullRequestState string

const (
	PullRequestStateOpen   PullRequestState = "open"
	PullRequestStateClosed PullRequestState = "closed"
	PullRequestStateMerged PullRequestState = "merged"
)

type PullRequest struct {
	Repository string
	Number     int
	HeadBranch string
	HeadSHA    string
	State      PullRequestState
}

func (pr PullRequest) IsFinished() bool {
	return pr.State == PullRequestStateClosed ||
		pr.State == PullRequestStateMerged
}
