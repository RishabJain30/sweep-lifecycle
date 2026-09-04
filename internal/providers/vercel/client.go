package vercel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

const (
	defaultBaseURL = "https://api.vercel.com"
	defaultTimeout = 15 * time.Second
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	teamID     string
}

// Option configures optional Client behavior.
type Option func(*Client)

// WithTeamID scopes every request to a Vercel team, as required by tokens
// that belong to a team rather than a personal account.
func WithTeamID(teamID string) Option {
	return func(c *Client) {
		c.teamID = teamID
	}
}

// NewClient creates an authenticated Vercel API client.
func NewClient(token string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("Vercel token is required")
	}

	client := &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL: defaultBaseURL,
		token:   token,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// ListDeployments retrieves every deployment for a Vercel project.
func (c *Client) ListDeployments(
	ctx context.Context,
	projectID string,
) ([]domain.VercelDeployment, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("Vercel project ID is required")
	}

	var deployments []domain.VercelDeployment

	var cursor int64
	seenCursors := make(map[int64]struct{})

	for {
		page, nextCursor, err := c.listDeploymentsPage(
			ctx,
			projectID,
			cursor,
		)
		if err != nil {
			return nil, err
		}

		deployments = append(deployments, page...)

		if nextCursor == 0 {
			break
		}

		if _, seen := seenCursors[nextCursor]; seen {
			return nil, fmt.Errorf(
				"Vercel API returned repeated pagination cursor %d",
				nextCursor,
			)
		}

		seenCursors[nextCursor] = struct{}{}
		cursor = nextCursor
	}

	return deployments, nil
}

// listDeploymentsPage retrieves and converts one page of Vercel
// deployments. A zero until means the first page.
func (c *Client) listDeploymentsPage(
	ctx context.Context,
	projectID string,
	until int64,
) ([]domain.VercelDeployment, int64, error) {
	endpoint, err := url.Parse(
		strings.TrimRight(c.baseURL, "/") + "/v7/deployments",
	)
	if err != nil {
		return nil, 0, fmt.Errorf("create Vercel URL: %w", err)
	}

	query := endpoint.Query()
	query.Set("projectId", projectID)
	query.Set("limit", "100")

	if until != 0 {
		query.Set("until", strconv.FormatInt(until, 10))
	}

	c.setTeamQuery(query)
	endpoint.RawQuery = query.Encode()

	response, err := c.get(ctx, endpoint.String())
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	var apiResponse apiListDeploymentsResponse
	if err := json.NewDecoder(response.Body).Decode(&apiResponse); err != nil {
		return nil, 0, fmt.Errorf("decode Vercel response: %w", err)
	}

	deployments := make(
		[]domain.VercelDeployment,
		0,
		len(apiResponse.Deployments),
	)

	for _, apiDeployment := range apiResponse.Deployments {
		deployments = append(deployments, apiDeployment.toDomain())
	}

	var nextCursor int64
	if apiResponse.Pagination.Next != nil {
		nextCursor = *apiResponse.Pagination.Next
	}

	return deployments, nextCursor, nil
}

// GetDeployment retrieves a single deployment by ID.
func (c *Client) GetDeployment(
	ctx context.Context,
	deploymentID string,
) (domain.VercelDeployment, error) {
	if strings.TrimSpace(deploymentID) == "" {
		return domain.VercelDeployment{}, errors.New(
			"Vercel deployment ID is required",
		)
	}

	endpoint, err := url.Parse(
		strings.TrimRight(c.baseURL, "/") +
			"/v13/deployments/" +
			url.PathEscape(deploymentID),
	)
	if err != nil {
		return domain.VercelDeployment{}, fmt.Errorf(
			"create Vercel URL: %w",
			err,
		)
	}

	query := endpoint.Query()
	c.setTeamQuery(query)
	endpoint.RawQuery = query.Encode()

	response, err := c.get(ctx, endpoint.String())
	if err != nil {
		return domain.VercelDeployment{}, err
	}
	defer response.Body.Close()

	var apiResponse apiDeployment
	if err := json.NewDecoder(response.Body).Decode(&apiResponse); err != nil {
		return domain.VercelDeployment{}, fmt.Errorf(
			"decode Vercel response: %w",
			err,
		)
	}

	return apiResponse.toDomain(), nil
}

func (c *Client) setTeamQuery(query url.Values) {
	if c.teamID != "" {
		query.Set("teamId", c.teamID)
	}
}

// get sends an authenticated GET request and returns the response for the
// caller to decode. Callers must close the response body. A non-200 status
// is translated into a descriptive error.
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
		return nil, fmt.Errorf("create Vercel request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sweep")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send Vercel request: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()

		return nil, fmt.Errorf("Vercel API returned %s", response.Status)
	}

	return response, nil
}
