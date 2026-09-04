package neon

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientListBranchesFollowsPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}

			if r.URL.Path != "/projects/test-project/branches" {
				t.Errorf(
					"path = %q, want %q",
					r.URL.Path,
					"/projects/test-project/branches",
				)
			}

			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf(
					"Authorization = %q, want %q",
					got,
					"Bearer test-token",
				)
			}

			if got := r.URL.Query().Get("limit"); got != "100" {
				t.Errorf("limit = %q, want %q", got, "100")
			}

			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Query().Get("cursor") {
			case "":
				fmt.Fprint(w, `{
					"branches": [{
						"id": "br-production",
						"project_id": "test-project",
						"parent_id": null,
						"name": "production",
						"current_state": "ready",
						"default": true,
						"protected": false,
						"created_at": "2026-09-01T10:00:00Z",
						"updated_at": "2026-09-01T10:00:00Z",
						"expires_at": null
					}],
					"pagination": {
						"next": "cursor-2"
					}
				}`)

			case "cursor-2":
				fmt.Fprint(w, `{
					"branches": [{
						"id": "br-preview",
						"project_id": "test-project",
						"parent_id": "br-production",
						"name": "preview-pr-1",
						"current_state": "ready",
						"default": false,
						"protected": false,
						"created_at": "2026-09-02T10:00:00Z",
						"updated_at": "2026-09-02T10:00:00Z",
						"expires_at": null
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

	branches, err := client.ListBranches(
		context.Background(),
		"test-project",
	)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}

	if len(branches) != 2 {
		t.Fatalf("branch count = %d, want %d", len(branches), 2)
	}

	if branches[0].Name != "production" {
		t.Fatalf(
			"first branch name = %q, want %q",
			branches[0].Name,
			"production",
		)
	}

	if branches[1].Name != "preview-pr-1" {
		t.Fatalf(
			"second branch name = %q, want %q",
			branches[1].Name,
			"preview-pr-1",
		)
	}

	if branches[1].ParentID == nil {
		t.Fatal("preview branch ParentID = nil, want a parent")
	}

	if got := *branches[1].ParentID; got != "br-production" {
		t.Fatalf(
			"preview branch ParentID = %q, want %q",
			got,
			"br-production",
		)
	}
}

func TestClientListBranchesReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	_, err := client.ListBranches(
		context.Background(),
		"test-project",
	)
	if err == nil {
		t.Fatal("ListBranches() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want it to contain 401", err)
	}
}

func TestClientListBranchesRejectsRepeatedCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"branches": [],
				"pagination": {
					"next": "repeated-cursor"
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

	_, err := client.ListBranches(
		context.Background(),
		"test-project",
	)
	if err == nil {
		t.Fatal("ListBranches() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "repeated pagination cursor") {
		t.Fatalf(
			"error = %q, want repeated pagination cursor error",
			err,
		)
	}
}

func TestNewClientRequiresToken(t *testing.T) {
	_, err := NewClient(" ")

	if err == nil {
		t.Fatal("NewClient() error = nil, want an error")
	}
}

func TestClientGetBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}

			wantPath := "/projects/test-project/branches/br-preview"
			if r.URL.Path != wantPath {
				t.Errorf(
					"path = %q, want %q",
					r.URL.Path,
					wantPath,
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
				"branch": {
					"id": "br-preview",
					"project_id": "test-project",
					"parent_id": "br-production",
					"name": "preview-pr-1",
					"current_state": "ready",
					"default": false,
					"protected": false,
					"created_at": "2026-09-02T10:00:00Z",
					"updated_at": "2026-09-02T10:00:00Z",
					"expires_at": null
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

	branch, err := client.GetBranch(
		context.Background(),
		"test-project",
		"br-preview",
	)
	if err != nil {
		t.Fatalf("GetBranch() error = %v", err)
	}

	if branch.Name != "preview-pr-1" {
		t.Fatalf("Name = %q, want %q", branch.Name, "preview-pr-1")
	}

	if branch.ID != "br-preview" {
		t.Fatalf("ID = %q, want %q", branch.ID, "br-preview")
	}
}

func TestClientGetBranchReturnsAPIError(t *testing.T) {
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

	_, err := client.GetBranch(
		context.Background(),
		"test-project",
		"br-missing",
	)
	if err == nil {
		t.Fatal("GetBranch() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want it to contain 404", err)
	}
}

func TestClientGetBranchRequiresProjectAndBranchID(t *testing.T) {
	client := &Client{
		httpClient: http.DefaultClient,
		baseURL:    "https://example.invalid",
		token:      "test-token",
	}

	if _, err := client.GetBranch(context.Background(), "", "br-preview"); err == nil {
		t.Fatal("GetBranch() error = nil, want an error for missing project ID")
	}

	if _, err := client.GetBranch(context.Background(), "test-project", ""); err == nil {
		t.Fatal("GetBranch() error = nil, want an error for missing branch ID")
	}
}
