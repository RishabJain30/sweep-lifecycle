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
	owner, name, err := parseRepository(repository)
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

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf(
			"create GitHub request: %w",
			err,
		)
	}

	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "sweep")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf(
			"send GitHub request: %w",
			err,
		)
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

// parseRepository separates an owner/name repository identifier.
func parseRepository(repository string) (string, string, error) {
	parts := strings.Split(repository, "/")

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf(
			"repository must use the owner/name format, got %q",
			repository,
		)
	}

	return parts[0], parts[1], nil
}
