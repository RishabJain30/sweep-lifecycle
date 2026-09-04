package domain

import "testing"

func TestVercelDeploymentIsProtected(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		protected bool
	}{
		{
			name:      "production deployment is protected",
			target:    "production",
			protected: true,
		},
		{
			name:      "staging deployment is not protected",
			target:    "staging",
			protected: false,
		},
		{
			name:      "ordinary preview deployment is not protected",
			target:    "",
			protected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deployment := VercelDeployment{Target: test.target}

			if got := deployment.IsProtected(); got != test.protected {
				t.Fatalf(
					"IsProtected() = %v, want %v",
					got,
					test.protected,
				)
			}
		})
	}
}
