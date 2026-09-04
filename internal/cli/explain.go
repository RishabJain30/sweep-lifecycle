package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/correlation"
	githubprovider "github.com/RishabJain30/sweep-lifecycle/internal/providers/github"
	neonprovider "github.com/RishabJain30/sweep-lifecycle/internal/providers/neon"
	vercelprovider "github.com/RishabJain30/sweep-lifecycle/internal/providers/vercel"
	scanservice "github.com/RishabJain30/sweep-lifecycle/internal/scan"
	"github.com/spf13/cobra"
)

// explainTarget identifies the single resource an explain invocation
// evaluates, plus the context needed to correlate it with GitHub.
type explainTarget struct {
	Provider      string
	ResourceID    string
	Repository    string
	NeonProjectID string
}

type explainRunner func(
	ctx context.Context,
	target explainTarget,
) (scanservice.Candidate, error)

func newExplainCommand(run explainRunner) *cobra.Command {
	var target explainTarget

	command := &cobra.Command{
		Use:   "explain <provider:resource-id>",
		Short: "Explain why a resource received its scan result",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			provider, resourceID, err := parseExplainTarget(args[0])
			if err != nil {
				return err
			}

			target.Provider = provider
			target.ResourceID = resourceID

			if strings.TrimSpace(target.Repository) == "" {
				return errors.New("--repo is required")
			}

			if target.Provider == "neon" &&
				strings.TrimSpace(target.NeonProjectID) == "" {
				return errors.New(
					"--neon-project is required to explain a neon " +
						"resource",
				)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			candidate, err := run(cmd.Context(), target)
			if err != nil {
				return err
			}

			return writeExplainReport(cmd.OutOrStdout(), candidate)
		},
	}

	command.Flags().StringVar(
		&target.Repository,
		"repo",
		"",
		"GitHub repository in owner/name format",
	)

	command.Flags().StringVar(
		&target.NeonProjectID,
		"neon-project",
		"",
		"Neon project ID (required to explain a neon:<id> resource)",
	)

	return command
}

// parseExplainTarget splits "provider:resource-id" into its parts.
func parseExplainTarget(raw string) (provider string, resourceID string, err error) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf(
			`resource must be in "provider:resource-id" format, got %q`,
			raw,
		)
	}

	provider, resourceID = parts[0], parts[1]

	if provider != "neon" && provider != "vercel" {
		return "", "", fmt.Errorf(
			`unsupported provider %q, want "neon" or "vercel"`,
			provider,
		)
	}

	return provider, resourceID, nil
}

// writeExplainReport renders a resource's identity followed by the exact
// same evidence and score rendering scan uses, so explain never drifts
// from scan's reasoning.
func writeExplainReport(w io.Writer, candidate scanservice.Candidate) error {
	if _, err := fmt.Fprintf(
		w,
		"Resource: %s %s (%s)\n",
		candidate.Provider,
		candidate.ResourceID,
		candidate.ResourceName,
	); err != nil {
		return err
	}

	if err := writePullRequestDetails(
		w,
		candidate.PullRequest,
		candidate.PullRequestFound,
		candidate.SourceBranchChecked,
		candidate.SourceBranchExists,
	); err != nil {
		return err
	}

	if candidate.Score.Excluded {
		if _, err := fmt.Fprintf(
			w,
			"  Excluded: %s\n",
			candidate.Score.ExclusionReason,
		); err != nil {
			return err
		}
	}

	return writeScore(w, candidate.Score)
}

// runExplain creates the real providers needed for one resource and
// evaluates it through the exact same scanservice.EvaluateResource scan
// uses.
func runExplain(
	ctx context.Context,
	target explainTarget,
) (scanservice.Candidate, error) {
	githubClient, err := githubprovider.NewClient(os.Getenv("GITHUB_TOKEN"))
	if err != nil {
		return scanservice.Candidate{}, fmt.Errorf(
			"configure GitHub: %w",
			err,
		)
	}

	switch target.Provider {
	case "neon":
		return explainNeonBranch(ctx, githubClient, target)
	case "vercel":
		return explainVercelDeployment(ctx, githubClient, target)
	default:
		return scanservice.Candidate{}, fmt.Errorf(
			"unsupported provider %q",
			target.Provider,
		)
	}
}

func explainNeonBranch(
	ctx context.Context,
	github scanservice.SourceControl,
	target explainTarget,
) (scanservice.Candidate, error) {
	neonClient, err := neonprovider.NewClient(os.Getenv("NEON_API_KEY"))
	if err != nil {
		return scanservice.Candidate{}, fmt.Errorf(
			"configure Neon: %w",
			err,
		)
	}

	branch, err := neonClient.GetBranch(
		ctx,
		target.NeonProjectID,
		target.ResourceID,
	)
	if err != nil {
		return scanservice.Candidate{}, fmt.Errorf(
			"get Neon branch: %w",
			err,
		)
	}

	prNumber, matched := correlation.ExtractPullRequestNumber(branch.Name)

	candidate, _ := scanservice.EvaluateResource(
		ctx,
		github,
		target.Repository,
		scanservice.ResourceInput{
			Provider:              "neon",
			ResourceID:            branch.ID,
			ResourceName:          branch.Name,
			ResourceProtected:     branch.IsProtected(),
			ResourceCreatedAt:     branch.CreatedAt,
			ResourceUpdatedAt:     branch.UpdatedAt,
			NameMatchesConvention: matched,
			PullRequestNumber:     prNumber,
		},
		time.Now(),
	)

	return candidate, nil
}

func explainVercelDeployment(
	ctx context.Context,
	github scanservice.SourceControl,
	target explainTarget,
) (scanservice.Candidate, error) {
	token := os.Getenv("VERCEL_TOKEN")
	if token == "" {
		return scanservice.Candidate{}, errors.New(
			"VERCEL_TOKEN is required to explain a vercel resource",
		)
	}

	var opts []vercelprovider.Option
	if teamID := os.Getenv("VERCEL_TEAM_ID"); teamID != "" {
		opts = append(opts, vercelprovider.WithTeamID(teamID))
	}

	vercelClient, err := vercelprovider.NewClient(token, opts...)
	if err != nil {
		return scanservice.Candidate{}, fmt.Errorf(
			"configure Vercel: %w",
			err,
		)
	}

	deployment, err := vercelClient.GetDeployment(ctx, target.ResourceID)
	if err != nil {
		return scanservice.Candidate{}, fmt.Errorf(
			"get Vercel deployment: %w",
			err,
		)
	}

	var prNumber int
	if deployment.PullRequestNumber != nil {
		prNumber = *deployment.PullRequestNumber
	}

	candidate, _ := scanservice.EvaluateResource(
		ctx,
		github,
		target.Repository,
		scanservice.ResourceInput{
			Provider:          "vercel",
			ResourceID:        deployment.ID,
			ResourceName:      deployment.Name,
			ResourceProtected: deployment.IsProtected(),
			ResourceCreatedAt: deployment.CreatedAt,
			ResourceUpdatedAt: deployment.CreatedAt,
			PullRequestNumber: prNumber,
		},
		time.Now(),
	)

	return candidate, nil
}
