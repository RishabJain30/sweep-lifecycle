package correlation

import "testing"

func TestExtractPullRequestNumber(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		wantNumber   int
		wantMatch    bool
	}{
		{
			name:         "preview resource with PR number",
			resourceName: "preview-pr-1",
			wantNumber:   1,
			wantMatch:    true,
		},
		{
			name:         "preview resource with larger PR number",
			resourceName: "preview-pr-482",
			wantNumber:   482,
			wantMatch:    true,
		},
		{
			name:         "production branch",
			resourceName: "production",
			wantNumber:   0,
			wantMatch:    false,
		},
		{
			name:         "zero is not a valid PR number",
			resourceName: "preview-pr-0",
			wantNumber:   0,
			wantMatch:    false,
		},
		{
			name:         "additional suffix is rejected",
			resourceName: "preview-pr-1-copy",
			wantNumber:   0,
			wantMatch:    false,
		},
		{
			name:         "additional prefix is rejected",
			resourceName: "old-preview-pr-1",
			wantNumber:   0,
			wantMatch:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			number, matched := ExtractPullRequestNumber(test.resourceName)

			if number != test.wantNumber {
				t.Fatalf(
					"number = %d, want %d",
					number,
					test.wantNumber,
				)
			}

			if matched != test.wantMatch {
				t.Fatalf(
					"matched = %v, want %v",
					matched,
					test.wantMatch,
				)
			}
		})
	}
}
