package domain

import "time"

type DatabaseBranch struct {
	ID        string
	Provider  string
	ProjectID string
	ParentID  *string
	Name      string
	State     string
	Default   bool
	Protected bool
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt *time.Time
}

// IsProtected reports whether Sweep must exclude the branch from cleanup.
func (branch DatabaseBranch) IsProtected() bool {
	return branch.Default || branch.Protected
}
