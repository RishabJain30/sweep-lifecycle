package neon

import (
	"time"

	"github.com/RishabJain30/sweep-lifecycle/internal/domain"
)

type apiListBranchesResponse struct {
	Branches   []apiBranch   `json:"branches"`
	Pagination apiPagination `json:"pagination"`
}

type apiGetBranchResponse struct {
	Branch apiBranch `json:"branch"`
}

type apiPagination struct {
	Next *string `json:"next"`
}

type apiBranch struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"project_id"`
	ParentID     *string    `json:"parent_id"`
	Name         string     `json:"name"`
	CurrentState string     `json:"current_state"`
	Default      bool       `json:"default"`
	Protected    bool       `json:"protected"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

// toDomain converts Neon's representation into Sweep's representation.
func (branch apiBranch) toDomain() domain.DatabaseBranch {
	return domain.DatabaseBranch{
		ID:        branch.ID,
		Provider:  "neon",
		ProjectID: branch.ProjectID,
		ParentID:  branch.ParentID,
		Name:      branch.Name,
		State:     branch.CurrentState,
		Default:   branch.Default,
		Protected: branch.Protected,
		CreatedAt: branch.CreatedAt,
		UpdatedAt: branch.UpdatedAt,
		ExpiresAt: branch.ExpiresAt,
	}
}
