package vercel

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAPIDeploymentToDomain(t *testing.T) {
	const response = `{
		"uid": "dpl_123",
		"name": "my-app",
		"projectId": "prj_456",
		"url": "my-app-git-feat-example.vercel.app",
		"created": 1735689600000,
		"target": null,
		"meta": {
			"githubCommitRef": "feat/example",
			"githubCommitSha": "abc123",
			"githubPrId": "42",
			"githubOrg": "RishabJain30",
			"githubRepo": "sweep-lifecycle"
		}
	}`

	var apiDep apiDeployment
	if err := json.Unmarshal([]byte(response), &apiDep); err != nil {
		t.Fatalf("decode API response: %v", err)
	}

	deployment := apiDep.toDomain()

	if deployment.ID != "dpl_123" {
		t.Fatalf("ID = %q, want %q", deployment.ID, "dpl_123")
	}

	if deployment.Target != "" {
		t.Fatalf("Target = %q, want empty (preview)", deployment.Target)
	}

	if deployment.IsProtected() {
		t.Fatal("IsProtected() = true, want false for a preview deployment")
	}

	if deployment.GitBranch != "feat/example" {
		t.Fatalf(
			"GitBranch = %q, want %q",
			deployment.GitBranch,
			"feat/example",
		)
	}

	if deployment.GitCommitSHA != "abc123" {
		t.Fatalf(
			"GitCommitSHA = %q, want %q",
			deployment.GitCommitSHA,
			"abc123",
		)
	}

	if deployment.SourceRepository != "RishabJain30/sweep-lifecycle" {
		t.Fatalf(
			"SourceRepository = %q, want %q",
			deployment.SourceRepository,
			"RishabJain30/sweep-lifecycle",
		)
	}

	if deployment.PullRequestNumber == nil {
		t.Fatal("PullRequestNumber is nil, want 42")
	}

	if *deployment.PullRequestNumber != 42 {
		t.Fatalf(
			"PullRequestNumber = %d, want %d",
			*deployment.PullRequestNumber,
			42,
		)
	}

	wantCreated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !deployment.CreatedAt.Equal(wantCreated) {
		t.Fatalf(
			"CreatedAt = %s, want %s",
			deployment.CreatedAt,
			wantCreated,
		)
	}
}

func TestAPIDeploymentToDomainProductionTarget(t *testing.T) {
	const response = `{
		"uid": "dpl_789",
		"name": "my-app",
		"projectId": "prj_456",
		"url": "my-app.vercel.app",
		"created": 1735689600000,
		"target": "production"
	}`

	var apiDep apiDeployment
	if err := json.Unmarshal([]byte(response), &apiDep); err != nil {
		t.Fatalf("decode API response: %v", err)
	}

	deployment := apiDep.toDomain()

	if !deployment.IsProtected() {
		t.Fatal("IsProtected() = false, want true for a production deployment")
	}
}

func TestAPIDeploymentToDomainWithoutGitMetadata(t *testing.T) {
	const response = `{
		"uid": "dpl_999",
		"name": "my-app",
		"projectId": "prj_456",
		"url": "my-app.vercel.app",
		"created": 1735689600000,
		"target": null
	}`

	var apiDep apiDeployment
	if err := json.Unmarshal([]byte(response), &apiDep); err != nil {
		t.Fatalf("decode API response: %v", err)
	}

	deployment := apiDep.toDomain()

	if deployment.PullRequestNumber != nil {
		t.Fatalf(
			"PullRequestNumber = %v, want nil without Git metadata",
			*deployment.PullRequestNumber,
		)
	}

	if deployment.SourceRepository != "" {
		t.Fatalf(
			"SourceRepository = %q, want empty without Git metadata",
			deployment.SourceRepository,
		)
	}
}
