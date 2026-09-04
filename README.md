# Sweep

Garbage collection for your engineering stack.

Sweep is a read-only, open-source Go CLI that finds temporary engineering
resources that outlived the pull request or feature that created them.

## The problem

A pull request opens, a preview database branch and a preview deployment
get created for it, and everyone moves on. The PR merges or closes weeks
later. Nobody deletes the preview resources, because nobody is watching for
that moment. Multiply that by every PR a team ships, and previews quietly
outlive their purpose across every provider that creates them.

Sweep treats GitHub pull requests as the source of truth for a preview
resource's lifecycle, and reports which resources in Neon and Vercel no
longer have a reason to exist.

## v0.1 scope

The v0.1 wedge is narrow and deliberate:

> Find preview infrastructure associated with closed or merged GitHub pull
> requests.

- **GitHub** is the lifecycle source: pull request state, and whether a
  pull request's source branch still exists.
- **Neon** and **Vercel** are the only external resource providers.
  Vercel is optional; Neon-only setups work fine.
- Sweep produces a report. **It never deletes anything.**

Explicitly out of scope for v0.1: a hosted backend, a database, user
accounts, feature flags, a scheduler, and any other provider beyond Neon
and Vercel.

## Safety philosophy

- **Sweep never deletes resources.** There is no delete command, no
  `--force`, no destructive code path anywhere in this codebase.
- **Scoring is fully deterministic.** No AI or probabilistic model decides
  whether a resource is safe to remove. The same evidence always produces
  the same score, confidence, and recommendation — see
  [internal/scoring](internal/scoring/scoring.go).
- **Protected resources are always excluded.** Default branches,
  explicitly protected branches, and Vercel production deployments are
  never candidates, regardless of how strong any other signal looks.
- **Partial failures are reported, not hidden.** If Sweep can't check one
  resource, it says so as a warning and still reports everything else it
  successfully evaluated. An API failure is never silently treated as "the
  resource is gone."

## Architecture

```
cmd/sweep              CLI entry point
internal/cli           Cobra commands: scan, explain
internal/domain        Provider-independent domain models (PullRequest,
                        DatabaseBranch, VercelDeployment)
internal/providers/    One package per external provider (github, neon,
                        vercel), each with its own HTTP client, API-shape
                        mapping, and fake-server tests
internal/correlation   Extracts a PR number from a preview resource name
internal/evidence      Builds provider-independent lifecycle evidence
internal/scoring       Deterministic, versioned scoring policy
internal/scan          Orchestrates discovery + correlation + evidence +
                        scoring into a scan Result
```

`scan` and `explain` both call the exact same
[`scan.EvaluateResource`](internal/scan/candidate.go) function to turn one
resource into evidence and a score. Neither command duplicates the other's
reasoning.

## Installation and local build

Requires Go 1.26+ (see `go.mod` for the exact version).

```bash
git clone https://github.com/RishabJain30/sweep-lifecycle.git
cd sweep-lifecycle
go build -o bin/sweep ./cmd/sweep
```

Or run directly without building a binary:

```bash
go run ./cmd/sweep scan --repo owner/name --neon-project <project-id>
```

## Provider setup

### GitHub (required)

Set `GITHUB_TOKEN` to a personal access token (classic or fine-grained)
with read access to the repository's pull requests and branches. The
`repo` scope (classic tokens) or fine-grained "Pull requests: Read" and
"Contents: Read" permissions are sufficient — Sweep never writes to
GitHub.

### Neon (required)

Set `NEON_API_KEY` to a Neon API key with read access to the project you
want to scan. Sweep only calls Neon's read-only branch-listing and
branch-detail endpoints.

### Vercel (optional)

Set `VERCEL_TOKEN` to a Vercel access token to enable Vercel discovery. If
`VERCEL_TOKEN` is unset, Sweep skips Vercel entirely and scans Neon only —
`scan`'s provider status section reports Vercel as skipped, not failed.

- `VERCEL_TEAM_ID` (optional): required if the token belongs to a team
  rather than a personal account.
- `--vercel-project` (required only when `VERCEL_TOKEN` is set): the
  Vercel project ID to scan.

A token with read-only access to Deployments is sufficient. Sweep never
calls any Vercel write or delete endpoint.

## Environment variables and flags

| Name | Kind | Required | Description |
|---|---|---|---|
| `GITHUB_TOKEN` | env | yes | GitHub API token |
| `NEON_API_KEY` | env | yes | Neon API key |
| `VERCEL_TOKEN` | env | no | Enables Vercel discovery |
| `VERCEL_TEAM_ID` | env | no | Scopes Vercel requests to a team |
| `--repo` | flag | yes | GitHub repository, `owner/name` |
| `--neon-project` | flag | yes | Neon project ID |
| `--vercel-project` | flag | only if `VERCEL_TOKEN` is set | Vercel project ID |
| `--format` | flag | no (`scan` only) | `text` (default) or `json` |

Credentials are only ever read from environment variables. They are never
written to logs, error messages, JSON output, or committed anywhere in
this repository.

## `scan` examples

```bash
# Neon only
GITHUB_TOKEN=*** NEON_API_KEY=*** \
  sweep scan --repo RishabJain30/sweep-lifecycle --neon-project my-project

# Neon + Vercel
GITHUB_TOKEN=*** NEON_API_KEY=*** VERCEL_TOKEN=*** \
  sweep scan --repo RishabJain30/sweep-lifecycle \
    --neon-project my-project --vercel-project prj_abc123

# Machine-readable output
sweep scan --repo owner/name --neon-project my-project --format json
```

Text output is organized into five sections, in order: provider discovery
status, cleanup candidates (each with its correlated PR, source-branch
status, deterministic evidence, score, and confidence), protected/skipped
resources with their exclusion reason, warnings from any partial failures,
and summary counts.

## `explain` examples

```bash
sweep explain neon:br-abc123 --repo owner/name --neon-project my-project
sweep explain vercel:dpl_abc123 --repo owner/name
```

`explain` fetches one resource directly, evaluates it through the same
evidence-and-scoring pipeline `scan` uses, and prints its full reasoning:
identity, correlated pull request, source-branch status, every evidence
item with its point contribution, final score, confidence, and (if
applicable) why it was excluded.

## JSON output

`sweep scan --format json` prints a single JSON object:

```json
{
  "providers": [{"provider": "neon", "status": "ok", "detail": "..."}],
  "candidates": [{
    "provider": "neon",
    "resource_id": "br-abc123",
    "resource_name": "preview-pr-42",
    "pull_request": {"number": 42, "state": "merged", "head_repository": "...", "head_branch": "..."},
    "source_branch": {"checked": true, "exists": false},
    "score": {
      "policy_version": "v1",
      "value": 78,
      "confidence": "HIGH",
      "recommendation": "...",
      "evidence": [{"kind": "pull_request_merged", "description": "...", "points": 30}]
    }
  }],
  "skipped": [{"provider": "neon", "resource_id": "...", "resource_name": "...", "reason": "..."}],
  "warnings": [{"provider": "neon", "resource": "...", "message": "..."}],
  "summary": {"candidate_count": 1, "high_confidence_count": 1, "skipped_count": 0, "warning_count": 0}
}
```

This schema is covered by tests in
[`internal/cli/scan_test.go`](internal/cli/scan_test.go) and is considered
stable: new fields may be added, but existing fields keep their meaning.
`pull_request` is `null` when Sweep could not retrieve the correlated pull
request.

## Confidence interpretation

Confidence expresses how much to trust a score, independent of how high
it is:

- **HIGH** — the pull request is finished (merged or closed) **and** its
  source branch is confirmed missing **and** the score clears the
  high-confidence threshold. This is the only path to HIGH.
- **MEDIUM** — either a finished pull request or a missing source branch,
  but not both, or the score didn't clear the high threshold.
- **LOW** — evidence is recent (the resource was created or updated
  inside Sweep's recency window), incomplete (a lookup failed), or simply
  insufficient. A resource whose name merely matches the preview naming
  convention, with nothing else, stays LOW.

Weights and thresholds are named constants in
[`internal/scoring/scoring.go`](internal/scoring/scoring.go), documented
there with the reasoning behind each one.

## Known limitations

- Vercel correlation depends on Vercel populating Git metadata
  (`meta.githubPrId`, `githubCommitRef`, etc.) on the deployment, which
  only happens for deployments connected to a GitHub repository. Vercel
  deployments without that metadata cannot be correlated and are reported
  as skipped.
- A systemic GitHub failure (for example, an invalid token) surfaces as one
  warning per resource rather than a single aggregate error, since each
  resource's GitHub lookup is evaluated independently.
- Sweep does not paginate around GitHub or Neon or Vercel rate limits; a
  very large project may need multiple runs if a provider throttles it.
- There is no persistent state. Every `scan` re-discovers and
  re-correlates from scratch; nothing is cached between runs.

## Development and test commands

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
git diff --check
```

Real-provider integration tests are opt-in and skip automatically without
credentials:

```bash
go test -tags integration ./... \
  # GITHUB_TOKEN / NEON_API_KEY+NEON_PROJECT_ID / VERCEL_TOKEN+VERCEL_PROJECT_ID
```

Fake local HTTP servers (`httptest`) back every provider's unit tests, so
the default `go test ./...` run requires no network access and no
credentials.

## v0.1 never deletes resources

There is no delete command, no destructive provider call, and no code
path in this repository that removes a Neon branch, a Vercel deployment,
or anything on GitHub. Sweep only ever reads.
