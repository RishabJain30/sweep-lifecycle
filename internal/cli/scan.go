package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	githubprovider "github.com/RishabJain30/sweep-lifecycle/internal/providers/github"
	neonprovider "github.com/RishabJain30/sweep-lifecycle/internal/providers/neon"
	scanservice "github.com/RishabJain30/sweep-lifecycle/internal/scan"
	"github.com/spf13/cobra"
)

type scanRunner func(
	ctx context.Context,
	repository string,
	projectID string,
) ([]scanservice.Match, error)

func newScanCommand(run scanRunner) *cobra.Command {
	var repository string
	var neonProjectID string

	command := &cobra.Command{
		Use:   "scan",
		Short: "Correlate preview resources with pull requests",
		Args:  cobra.NoArgs,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(repository) == "" {
				return errors.New("--repo is required")
			}

			if strings.TrimSpace(neonProjectID) == "" {
				return errors.New("--neon-project is required")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			matches, err := run(
				cmd.Context(),
				repository,
				neonProjectID,
			)
			if err != nil {
				return err
			}

			if len(matches) == 0 {
				_, err := fmt.Fprintln(
					cmd.OutOrStdout(),
					"No correlated preview resources found.",
				)
				return err
			}

			if _, err := fmt.Fprintln(
				cmd.OutOrStdout(),
				"Correlated preview resources",
			); err != nil {
				return err
			}

			for _, match := range matches {
				_, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"\nNeon branch: %s\n"+
						"Resource ID: %s\n"+
						"Associated PR: #%d\n"+
						"PR state: %s\n"+
						"PR head repository: %s\n"+
						"PR head branch: %s\n"+
						"Source branch exists: %t\n"+
						"Default: %t\n"+
						"Protected: %t\n",
					match.Branch.Name,
					match.Branch.ID,
					match.PullRequest.Number,
					match.PullRequest.State,
					headRepositoryDisplay(match.PullRequest.HeadRepository),
					match.PullRequest.HeadBranch,
					match.SourceBranchExists,
					match.Branch.Default,
					match.Branch.Protected,
				)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	command.Flags().StringVar(
		&repository,
		"repo",
		"",
		"GitHub repository in owner/name format",
	)

	command.Flags().StringVar(
		&neonProjectID,
		"neon-project",
		"",
		"Neon project ID",
	)

	return command
}

// headRepositoryDisplay reports a fork/source repository that GitHub no
// longer exposes, instead of printing an empty value.
func headRepositoryDisplay(headRepository string) string {
	if headRepository == "" {
		return "(unknown - source repository no longer available)"
	}

	return headRepository
}

// runScan creates the real providers and executes the scan service.
func runScan(
	ctx context.Context,
	repository string,
	projectID string,
) ([]scanservice.Match, error) {
	githubClient, err := githubprovider.NewClient(
		os.Getenv("GITHUB_TOKEN"),
	)
	if err != nil {
		return nil, fmt.Errorf("configure GitHub: %w", err)
	}

	neonClient, err := neonprovider.NewClient(
		os.Getenv("NEON_API_KEY"),
	)
	if err != nil {
		return nil, fmt.Errorf("configure Neon: %w", err)
	}

	service := scanservice.NewService(
		neonClient,
		githubClient,
	)

	return service.Scan(ctx, repository, projectID)
}
