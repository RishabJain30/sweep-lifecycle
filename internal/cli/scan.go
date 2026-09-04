package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	githubprovider "github.com/RishabJain30/sweep-lifecycle/internal/providers/github"
	neonprovider "github.com/RishabJain30/sweep-lifecycle/internal/providers/neon"
	vercelprovider "github.com/RishabJain30/sweep-lifecycle/internal/providers/vercel"
	scanservice "github.com/RishabJain30/sweep-lifecycle/internal/scan"
	"github.com/spf13/cobra"
)

const (
	formatText = "text"
	formatJSON = "json"
)

type scanRunner func(
	ctx context.Context,
	cfg scanservice.Config,
) (scanservice.Result, error)

func newScanCommand(run scanRunner) *cobra.Command {
	var cfg scanservice.Config
	var format string

	command := &cobra.Command{
		Use:   "scan",
		Short: "Correlate preview resources with pull requests",
		Args:  cobra.NoArgs,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(cfg.Repository) == "" {
				return errors.New("--repo is required")
			}

			if _, _, err := githubprovider.ParseRepository(cfg.Repository); err != nil {
				return err
			}

			if strings.TrimSpace(cfg.NeonProjectID) == "" {
				return errors.New("--neon-project is required")
			}

			if format != formatText && format != formatJSON {
				return fmt.Errorf(
					"--format must be %q or %q, got %q",
					formatText,
					formatJSON,
					format,
				)
			}

			if os.Getenv("VERCEL_TOKEN") != "" &&
				strings.TrimSpace(cfg.VercelProjectID) == "" {
				return errors.New(
					"--vercel-project is required when VERCEL_TOKEN is set",
				)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := run(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			if format == formatJSON {
				return writeJSONReport(cmd.OutOrStdout(), result)
			}

			return writeTextReport(cmd.OutOrStdout(), result)
		},
	}

	command.Flags().StringVar(
		&cfg.Repository,
		"repo",
		"",
		"GitHub repository in owner/name format",
	)

	command.Flags().StringVar(
		&cfg.NeonProjectID,
		"neon-project",
		"",
		"Neon project ID",
	)

	command.Flags().StringVar(
		&cfg.VercelProjectID,
		"vercel-project",
		"",
		"Vercel project ID (required when VERCEL_TOKEN is set)",
	)

	command.Flags().StringVar(
		&format,
		"format",
		formatText,
		"Output format: text or json",
	)

	return command
}

// writeTextReport renders a practical, human-readable cleanup-candidate
// report: provider discovery status, candidates with their evidence and
// score, protected/skipped resources, warnings, and summary counts.
func writeTextReport(w io.Writer, result scanservice.Result) error {
	fprintf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(w, format, args...)
		return err
	}

	if err := fprintf("Providers:\n"); err != nil {
		return err
	}

	for _, status := range result.ProviderStatuses {
		if err := fprintf(
			"  %s: %s - %s\n",
			status.Provider,
			status.Status,
			status.Detail,
		); err != nil {
			return err
		}
	}

	if err := fprintf(
		"\nCleanup candidates (%d):\n",
		len(result.Candidates),
	); err != nil {
		return err
	}

	if len(result.Candidates) == 0 {
		if err := fprintf("  (none)\n"); err != nil {
			return err
		}
	}

	for _, candidate := range result.Candidates {
		if err := fprintf(
			"\n  %s %s (%s)\n",
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

		if err := writeScore(w, candidate.Score); err != nil {
			return err
		}
	}

	if err := fprintf(
		"\nProtected / skipped resources (%d):\n",
		len(result.Skipped),
	); err != nil {
		return err
	}

	if len(result.Skipped) == 0 {
		if err := fprintf("  (none)\n"); err != nil {
			return err
		}
	}

	for _, skipped := range result.Skipped {
		if err := fprintf(
			"  %s %s (%s): %s\n",
			skipped.Provider,
			skipped.ResourceID,
			skipped.ResourceName,
			skipped.Reason,
		); err != nil {
			return err
		}
	}

	if err := fprintf(
		"\nWarnings (%d):\n",
		len(result.Warnings),
	); err != nil {
		return err
	}

	if len(result.Warnings) == 0 {
		if err := fprintf("  (none)\n"); err != nil {
			return err
		}
	}

	for _, warning := range result.Warnings {
		if err := fprintf(
			"  %s %s: %s\n",
			warning.Provider,
			warning.Resource,
			warning.Message,
		); err != nil {
			return err
		}
	}

	highConfidence := 0
	for _, candidate := range result.Candidates {
		if candidate.Score.Confidence == highConfidenceLevel {
			highConfidence++
		}
	}

	return fprintf(
		"\nSummary: %d candidate(s) (%d high confidence), %d skipped, "+
			"%d warning(s)\n",
		len(result.Candidates),
		highConfidence,
		len(result.Skipped),
		len(result.Warnings),
	)
}

func writeJSONReport(w io.Writer, result scanservice.Result) error {
	body, err := marshalReport(result)
	if err != nil {
		return fmt.Errorf("encode JSON report: %w", err)
	}

	_, err = fmt.Fprintln(w, string(body))

	return err
}

// runScan creates the real providers and executes the scan service.
// Vercel is optional: it is only configured (and only scanned) when
// VERCEL_TOKEN is set.
func runScan(
	ctx context.Context,
	cfg scanservice.Config,
) (scanservice.Result, error) {
	githubClient, err := githubprovider.NewClient(os.Getenv("GITHUB_TOKEN"))
	if err != nil {
		return scanservice.Result{}, fmt.Errorf("configure GitHub: %w", err)
	}

	neonClient, err := neonprovider.NewClient(os.Getenv("NEON_API_KEY"))
	if err != nil {
		return scanservice.Result{}, fmt.Errorf("configure Neon: %w", err)
	}

	vercelClient, err := newOptionalVercelClient()
	if err != nil {
		return scanservice.Result{}, fmt.Errorf("configure Vercel: %w", err)
	}

	service := scanservice.NewService(neonClient, githubClient, vercelClient, nil)

	return service.Scan(ctx, cfg)
}

// newOptionalVercelClient constructs a Vercel client only when VERCEL_TOKEN
// is set. It returns a nil DeploymentLister when Vercel is not configured,
// which scanservice.Service treats as "provider skipped".
func newOptionalVercelClient() (scanservice.DeploymentLister, error) {
	token := os.Getenv("VERCEL_TOKEN")
	if token == "" {
		return nil, nil
	}

	var opts []vercelprovider.Option
	if teamID := os.Getenv("VERCEL_TEAM_ID"); teamID != "" {
		opts = append(opts, vercelprovider.WithTeamID(teamID))
	}

	client, err := vercelprovider.NewClient(token, opts...)
	if err != nil {
		return nil, err
	}

	return client, nil
}
