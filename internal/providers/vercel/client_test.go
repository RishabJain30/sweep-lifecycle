package vercel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientRequiresToken(t *testing.T) {
	_, err := NewClient(" ")

	if err == nil {
		t.Fatal("NewClient() error = nil, want an error")
	}
}

func TestClientListDeploymentsFollowsPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}

			if r.URL.Path != "/v7/deployments" {
				t.Errorf(
					"path = %q, want %q",
					r.URL.Path,
					"/v7/deployments",
				)
			}

			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf(
					"Authorization = %q, want %q",
					got,
					"Bearer test-token",
				)
			}

			if got := r.URL.Query().Get("projectId"); got != "prj_1" {
				t.Errorf("projectId = %q, want %q", got, "prj_1")
			}

			switch r.URL.Query().Get("until") {
			case "":
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{
					"deployments": [{
						"uid": "dpl_1",
						"name": "app",
						"projectId": "prj_1",
						"url": "app-1.vercel.app",
						"created": 1735689600000,
						"target": "production"
					}],
					"pagination": {"next": 1735689500000}
				}`)

			case "1735689500000":
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{
					"deployments": [{
						"uid": "dpl_2",
						"name": "app",
						"projectId": "prj_1",
						"url": "app-2.vercel.app",
						"created": 1735689400000,
						"target": null,
						"meta": {"githubPrId": "7"}
					}],
					"pagination": {}
				}`)

			default:
				http.Error(w, "unexpected cursor", http.StatusBadRequest)
			}
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	deployments, err := client.ListDeployments(
		context.Background(),
		"prj_1",
	)
	if err != nil {
		t.Fatalf("ListDeployments() error = %v", err)
	}

	if len(deployments) != 2 {
		t.Fatalf("deployment count = %d, want %d", len(deployments), 2)
	}

	if deployments[0].ID != "dpl_1" {
		t.Fatalf("first ID = %q, want %q", deployments[0].ID, "dpl_1")
	}

	if deployments[1].ID != "dpl_2" {
		t.Fatalf("second ID = %q, want %q", deployments[1].ID, "dpl_2")
	}

	if deployments[1].PullRequestNumber == nil ||
		*deployments[1].PullRequestNumber != 7 {
		t.Fatalf(
			"second PullRequestNumber = %v, want 7",
			deployments[1].PullRequestNumber,
		)
	}
}

func TestClientListDeploymentsSendsTeamID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("teamId"); got != "team_1" {
				t.Errorf("teamId = %q, want %q", got, "team_1")
			}

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"deployments": [], "pagination": {}}`)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
		teamID:     "team_1",
	}

	if _, err := client.ListDeployments(context.Background(), "prj_1"); err != nil {
		t.Fatalf("ListDeployments() error = %v", err)
	}
}

func TestClientListDeploymentsRequiresProjectID(t *testing.T) {
	client := &Client{
		httpClient: http.DefaultClient,
		baseURL:    "https://example.invalid",
		token:      "test-token",
	}

	if _, err := client.ListDeployments(context.Background(), ""); err == nil {
		t.Fatal("ListDeployments() error = nil, want an error")
	}
}

func TestClientListDeploymentsReturnsAuthenticationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "bad-token",
	}

	_, err := client.ListDeployments(context.Background(), "prj_1")
	if err == nil {
		t.Fatal("ListDeployments() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want it to contain 401", err)
	}
}

func TestClientListDeploymentsReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "server error", http.StatusInternalServerError)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	_, err := client.ListDeployments(context.Background(), "prj_1")
	if err == nil {
		t.Fatal("ListDeployments() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %q, want it to contain 500", err)
	}
}

func TestClientListDeploymentsReturnsMalformedResponseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"deployments": [ this is not valid json`)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	_, err := client.ListDeployments(context.Background(), "prj_1")
	if err == nil {
		t.Fatal("ListDeployments() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "decode Vercel response") {
		t.Fatalf("error = %q, want a decode error", err)
	}
}

func TestClientListDeploymentsRejectsRepeatedCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"deployments": [],
				"pagination": {"next": 111}
			}`)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	_, err := client.ListDeployments(context.Background(), "prj_1")
	if err == nil {
		t.Fatal("ListDeployments() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "repeated pagination cursor") {
		t.Fatalf(
			"error = %q, want repeated pagination cursor error",
			err,
		)
	}
}

func TestClientGetDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			wantPath := "/v13/deployments/dpl_123"
			if r.URL.Path != wantPath {
				t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
			}

			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf(
					"Authorization = %q, want %q",
					got,
					"Bearer test-token",
				)
			}

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"uid": "dpl_123",
				"name": "app",
				"projectId": "prj_1",
				"url": "app.vercel.app",
				"created": 1735689600000,
				"target": null,
				"meta": {"githubPrId": "9"}
			}`)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	deployment, err := client.GetDeployment(
		context.Background(),
		"dpl_123",
	)
	if err != nil {
		t.Fatalf("GetDeployment() error = %v", err)
	}

	if deployment.ID != "dpl_123" {
		t.Fatalf("ID = %q, want %q", deployment.ID, "dpl_123")
	}

	if deployment.PullRequestNumber == nil ||
		*deployment.PullRequestNumber != 9 {
		t.Fatalf(
			"PullRequestNumber = %v, want 9",
			deployment.PullRequestNumber,
		)
	}
}

func TestClientGetDeploymentRequiresDeploymentID(t *testing.T) {
	client := &Client{
		httpClient: http.DefaultClient,
		baseURL:    "https://example.invalid",
		token:      "test-token",
	}

	if _, err := client.GetDeployment(context.Background(), ""); err == nil {
		t.Fatal("GetDeployment() error = nil, want an error")
	}
}

func TestClientGetDeploymentReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	_, err := client.GetDeployment(context.Background(), "dpl_missing")
	if err == nil {
		t.Fatal("GetDeployment() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want it to contain 404", err)
	}
}
