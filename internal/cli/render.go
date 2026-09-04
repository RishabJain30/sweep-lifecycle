package cli

import (
	"fmt"
	"io"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
	"github.com/RishabJain30/sweep-lifecycle/internal/scoring"
)

// headRepositoryDisplay reports a fork/source repository that GitHub no
// longer exposes, instead of printing an empty value.
func headRepositoryDisplay(headRepository string) string {
	if headRepository == "" {
		return "(unknown - source repository no longer available)"
	}

	return headRepository
}

// writePullRequestDetails renders the correlated pull request and
// source-branch status shared by scan and explain text output.
func writePullRequestDetails(
	w io.Writer,
	pr domain.PullRequest,
	pullRequestFound bool,
	sourceBranchChecked bool,
	sourceBranchExists bool,
) error {
	if !pullRequestFound {
		_, err := fmt.Fprintln(
			w,
			"  Pull request: (could not be retrieved from GitHub)",
		)

		return err
	}

	if _, err := fmt.Fprintf(
		w,
		"  Pull request: #%d (%s)\n"+
			"  PR head repository: %s\n"+
			"  PR head branch: %s\n",
		pr.Number,
		pr.State,
		headRepositoryDisplay(pr.HeadRepository),
		pr.HeadBranch,
	); err != nil {
		return err
	}

	switch {
	case !sourceBranchChecked:
		_, err := fmt.Fprintln(w, "  Source branch: (not checked)")

		return err
	case sourceBranchExists:
		_, err := fmt.Fprintln(w, "  Source branch exists: true")

		return err
	default:
		_, err := fmt.Fprintln(w, "  Source branch exists: false")

		return err
	}
}

// writeScore renders a scoring.Result, including every evidence
// contribution, so scan and explain always show identical reasoning for
// identical evidence.
func writeScore(w io.Writer, score scoring.Result) error {
	if _, err := fmt.Fprintf(
		w,
		"  Score: %d/100 (policy %s)\n"+
			"  Confidence: %s\n"+
			"  Recommendation: %s\n"+
			"  Evidence:\n",
		score.Score,
		score.PolicyVersion,
		score.Confidence,
		score.Recommendation,
	); err != nil {
		return err
	}

	for _, contribution := range score.Contributions {
		if _, err := fmt.Fprintf(
			w,
			"    [%+d] %s\n",
			contribution.Points,
			contribution.Description,
		); err != nil {
			return err
		}
	}

	return nil
}
