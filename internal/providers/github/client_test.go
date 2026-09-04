package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

func TestClientGetPullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}

			if r.URL.Path != "/repos/cli/cli/pulls/14343" {
				t.Errorf(
					"path = %q, want %q",
					r.URL.Path,
					"/repos/cli/cli/pulls/14343",
				)
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
				"number": 14343,
				"state": "closed",
				"closed_at": "2026-09-04T09:02:01Z",
				"merged_at": "2026-09-04T09:02:01Z",
				"head": {
					"ref": "issue-triage-permalinks",
					"sha": "92029a9e735475ab83b9daffeff500c92a9fe2ad"
				},
				"base": {
					"repo": {
						"full_name": "cli/cli"
					}
				}
			}`)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	pr, err := client.GetPullRequest(
		context.Background(),
		"cli/cli",
		14343,
	)
	if err != nil {
		t.Fatalf("GetPullRequest() error = %v", err)
	}

	if pr.Number != 14343 {
		t.Fatalf("Number = %d, want %d", pr.Number, 14343)
	}

	if pr.State != domain.PullRequestStateMerged {
		t.Fatalf(
			"State = %q, want %q",
			pr.State,
			domain.PullRequestStateMerged,
		)
	}
}

func TestClientGetPullRequestReturnsAPIError(t *testing.T) {
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

	_, err := client.GetPullRequest(
		context.Background(),
		"cli/cli",
		999999,
	)
	if err == nil {
		t.Fatal("GetPullRequest() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want it to contain 404", err)
	}
}

func TestNewClientRequiresToken(t *testing.T) {
	_, err := NewClient("")

	if err == nil {
		t.Fatal("NewClient() error = nil, want an error")
	}
}
