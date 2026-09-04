package neon

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAPIBranchToDomain(t *testing.T) {
	const responseJSON = `{
		"branches": [
			{
				"id": "br-withered-mud-ay863ii1",
				"project_id": "ancient-mode-02595345",
				"parent_id": null,
				"name": "production",
				"current_state": "ready",
				"default": true,
				"protected": false,
				"created_at": "2026-09-04T12:51:26Z",
				"updated_at": "2026-09-04T13:05:27Z",
				"expires_at": null
			}
		],
		"pagination": {
			"sort_by": "updated_at",
			"sort_order": "DESC"
		}
	}`

	var response apiListBranchesResponse
	if err := json.Unmarshal([]byte(responseJSON), &response); err != nil {
		t.Fatalf("decode API response: %v", err)
	}

	if len(response.Branches) != 1 {
		t.Fatalf(
			"branch count = %d, want %d",
			len(response.Branches),
			1,
		)
	}

	branch := response.Branches[0].toDomain()

	if branch.ID != "br-withered-mud-ay863ii1" {
		t.Fatalf(
			"ID = %q, want %q",
			branch.ID,
			"br-withered-mud-ay863ii1",
		)
	}

	if branch.Provider != "neon" {
		t.Fatalf("Provider = %q, want %q", branch.Provider, "neon")
	}

	if branch.Name != "production" {
		t.Fatalf("Name = %q, want %q", branch.Name, "production")
	}

	if branch.ParentID != nil {
		t.Fatalf("ParentID = %q, want nil", *branch.ParentID)
	}

	if !branch.IsProtected() {
		t.Fatal("IsProtected() = false, want true")
	}

	if got := branch.CreatedAt.Format(time.RFC3339); got != "2026-09-04T12:51:26Z" {
		t.Fatalf(
			"CreatedAt = %q, want %q",
			got,
			"2026-09-04T12:51:26Z",
		)
	}
}
