package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

const (
	defaultBaseURL = "https://api.github.com"
	defaultTimeout = 15 * time.Second
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient creates a GitHub client with authentication and a request timeout.
func NewClient(token string) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("GitHub token is required")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL: defaultBaseURL,
		token:   token,
	}, nil
}

// GetPullRequest retrieves one pull request and converts it into Sweep's model.
func (c *Client) GetPullRequest(
	ctx context.Context,
	repository string,
	number int,
) (domain.PullRequest, error) {
	owner, name, err := ParseRepository(repository)
	if err != nil {
		return domain.PullRequest{}, err
	}

	if number <= 0 {
		return domain.PullRequest{}, errors.New(
			"pull request number must be greater than zero",
		)
	}

	endpoint := fmt.Sprintf(
		"%s/repos/%s/%s/pulls/%d",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(owner),
		url.PathEscape(name),
		number,
	)

	response, err := c.get(ctx, endpoint)
	if err != nil {
		return domain.PullRequest{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return domain.PullRequest{}, fmt.Errorf(
			"GitHub API returned %s",
			response.Status,
		)
	}

	var apiPR apiPullRequest
	if err := json.NewDecoder(response.Body).Decode(&apiPR); err != nil {
		return domain.PullRequest{}, fmt.Errorf(
			"decode GitHub response: %w",
			err,
		)
	}

	pr, err := apiPR.toDomain()
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf(
			"convert GitHub pull request: %w",
			err,
		)
	}

	return pr, nil
}

// BranchExists reports whether a branch currently exists in a GitHub
// repository. GitHub's branch endpoint returns 404 both when the branch is
// missing and when the repository itself is nonexistent or inaccessible to
// the token (GitHub deliberately does not distinguish the two, to avoid
// revealing that a private repository exists). Reporting the resource as
// "missing" from an access problem would manufacture false cleanup
// evidence, so a branch 404 is only trusted once the repository is
// separately confirmed accessible; otherwise BranchExists returns an error
// so the caller records incomplete evidence instead of a false positive.
func (c *Client) BranchExists(
	ctx context.Context,
	repository string,
	branch string,
) (bool, error) {
	owner, name, err := ParseRepository(repository)
	if err != nil {
		return false, err
	}

	if strings.TrimSpace(branch) == "" {
		return false, errors.New("branch name is required")
	}

	endpoint := fmt.Sprintf(
		"%s/repos/%s/%s/branches/%s",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(owner),
		url.PathEscape(name),
		url.PathEscape(branch),
	)

	response, err := c.get(ctx, endpoint)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return c.branchMissingOrRepositoryInaccessible(ctx, owner, name, branch)
	default:
		return false, fmt.Errorf(
			"GitHub API returned %s while checking branch",
			response.Status,
		)
	}
}

// branchMissingOrRepositoryInaccessible disambiguates a branch-endpoint
// 404 by checking the repository directly: only once the repository
// itself is confirmed accessible does a branch 404 mean the branch is
// actually missing.
func (c *Client) branchMissingOrRepositoryInaccessible(
	ctx context.Context,
	owner string,
	name string,
	branch string,
) (bool, error) {
	accessible, err := c.repositoryAccessible(ctx, owner, name)
	if err != nil {
		return false, fmt.Errorf(
			"verify repository %s/%s is accessible: %w",
			owner,
			name,
			err,
		)
	}

	if !accessible {
		return false, fmt.Errorf(
			"cannot verify branch %q: repository %s/%s is not accessible "+
				"with the current token",
			branch,
			owner,
			name,
		)
	}

	return false, nil
}

// repositoryAccessible reports whether the token can see the repository at
// all, independent of any specific branch.
func (c *Client) repositoryAccessible(
	ctx context.Context,
	owner string,
	name string,
) (bool, error) {
	endpoint := fmt.Sprintf(
		"%s/repos/%s/%s",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(owner),
		url.PathEscape(name),
	)

	response, err := c.get(ctx, endpoint)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf(
			"GitHub API returned %s while checking repository access",
			response.Status,
		)
	}
}

// ParseRepository separates an owner/name repository identifier. It is
// exported so callers (such as CLI flag validation) can reject a malformed
// --repo once, up front, instead of failing separately for every
// correlated resource.
func ParseRepository(repository string) (string, string, error) {
	parts := strings.Split(repository, "/")

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf(
			"repository must use the owner/name format, got %q",
			repository,
		)
	}

	return parts[0], parts[1], nil
}

// get sends an authenticated GET request and returns the raw response for
// the caller to interpret. Callers must always close the response body.
// Unlike Sweep's other provider clients, GitHub callers each interpret
// status codes differently (a 404 means something different to
// BranchExists than to repositoryAccessible), so this helper only
// deduplicates request construction, not status handling.
func (c *Client) get(
	ctx context.Context,
	endpoint string,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create GitHub request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "sweep")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send GitHub request: %w", err)
	}

	return response, nil
}
