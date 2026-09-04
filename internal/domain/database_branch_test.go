package domain

import "testing"

func TestDatabaseBranchIsProtected(t *testing.T) {
	tests := []struct {
		name      string
		branch    DatabaseBranch
		protected bool
	}{
		{
			name: "default branch is protected",
			branch: DatabaseBranch{
				Default: true,
			},
			protected: true,
		},
		{
			name: "explicitly protected branch is protected",
			branch: DatabaseBranch{
				Protected: true,
			},
			protected: true,
		},
		{
			name: "ordinary preview branch is not protected",
			branch: DatabaseBranch{
				Name: "preview-pr-1",
			},
			protected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.branch.IsProtected(); got != test.protected {
				t.Fatalf(
					"IsProtected() = %v, want %v",
					got,
					test.protected,
				)
			}
		})
	}
}
