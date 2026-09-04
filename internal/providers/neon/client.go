package neon

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
	defaultBaseURL = "https://console.neon.tech/api/v2"
	defaultTimeout = 15 * time.Second
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient creates an authenticated Neon API client.
func NewClient(token string) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("Neon API key is required")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL: defaultBaseURL,
		token:   token,
	}, nil
}

// ListBranches retrieves every branch in a Neon project.
func (c *Client) ListBranches(
	ctx context.Context,
	projectID string,
) ([]domain.DatabaseBranch, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("Neon project ID is required")
	}

	var branches []domain.DatabaseBranch
	cursor := ""
	seenCursors := make(map[string]struct{})

	for {
		page, nextCursor, err := c.listBranchesPage(
			ctx,
			projectID,
			cursor,
		)
		if err != nil {
			return nil, err
		}

		branches = append(branches, page...)

		if nextCursor == "" {
			break
		}

		if _, seen := seenCursors[nextCursor]; seen {
			return nil, fmt.Errorf(
				"Neon API returned repeated pagination cursor %q",
				nextCursor,
			)
		}

		seenCursors[nextCursor] = struct{}{}
		cursor = nextCursor
	}

	return branches, nil
}

// listBranchesPage retrieves and converts one page of Neon branches.
func (c *Client) listBranchesPage(
	ctx context.Context,
	projectID string,
	cursor string,
) ([]domain.DatabaseBranch, string, error) {
	endpoint, err := url.Parse(
		strings.TrimRight(c.baseURL, "/") +
			"/projects/" +
			url.PathEscape(projectID) +
			"/branches",
	)
	if err != nil {
		return nil, "", fmt.Errorf("create Neon URL: %w", err)
	}

	query := endpoint.Query()
	query.Set("limit", "100")

	if cursor != "" {
		query.Set("cursor", cursor)
	}

	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint.String(),
		nil,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create Neon request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sweep")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("send Neon request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf(
			"Neon API returned %s",
			response.Status,
		)
	}

	var apiResponse apiListBranchesResponse
	if err := json.NewDecoder(response.Body).Decode(&apiResponse); err != nil {
		return nil, "", fmt.Errorf(
			"decode Neon response: %w",
			err,
		)
	}

	branches := make([]domain.DatabaseBranch, 0, len(apiResponse.Branches))

	for _, apiBranch := range apiResponse.Branches {
		branches = append(branches, apiBranch.toDomain())
	}

	nextCursor := ""
	if apiResponse.Pagination.Next != nil {
		nextCursor = *apiResponse.Pagination.Next
	}

	return branches, nextCursor, nil
}
