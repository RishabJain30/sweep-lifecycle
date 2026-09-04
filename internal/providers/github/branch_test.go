package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientBranchExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}

			wantPath := "/repos/cli/cli/branches/feature%2Fnew-ui"
			if r.URL.EscapedPath() != wantPath {
				t.Errorf(
					"path = %q, want %q",
					r.URL.EscapedPath(),
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

			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		token:      "test-token",
	}

	exists, err := client.BranchExists(
		context.Background(),
		"cli/cli",
		"feature/new-ui",
	)
	if err != nil {
		t.Fatalf("BranchExists() error = %v", err)
	}

	if !exists {
		t.Fatal("BranchExists() = false, want true")
	}
}

func TestClientBranchExistsReturnsFalseForMissingBranch(t *testing.T) {
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

	exists, err := client.BranchExists(
		context.Background(),
		"cli/cli",
		"deleted-branch",
	)
	if err != nil {
		t.Fatalf("BranchExists() error = %v", err)
	}

	if exists {
		t.Fatal("BranchExists() = true, want false")
	}
}

func TestClientBranchExistsReturnsAPIError(t *testing.T) {
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

	_, err := client.BranchExists(
		context.Background(),
		"cli/cli",
		"feature/new-ui",
	)
	if err == nil {
		t.Fatal("BranchExists() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %q, want it to contain 500", err)
	}
}
