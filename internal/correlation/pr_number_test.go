package correlation

import "testing"

func TestExtractPullRequestNumberValid(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		wantNumber   int
	}{
		{
			name:         "dash-separated preview resource with PR number",
			resourceName: "preview-pr-1",
			wantNumber:   1,
		},
		{
			name:         "dash-separated with a larger PR number",
			resourceName: "preview-pr-482",
			wantNumber:   482,
		},
		{
			name:         "dash-separated with a descriptive suffix",
			resourceName: "preview-pr-42-checkout",
			wantNumber:   42,
		},
		{
			name:         "dash-separated with a multi-segment suffix",
			resourceName: "preview-pr-42-checkout-e2e",
			wantNumber:   42,
		},
		{
			name:         "slash-separated preview resource with PR number",
			resourceName: "preview/pr-42",
			wantNumber:   42,
		},
		{
			name:         "slash-separated with a descriptive suffix",
			resourceName: "preview/pr-42-checkout",
			wantNumber:   42,
		},
		{
			name:         "slash-separated with a multi-segment suffix",
			resourceName: "preview/pr-7-hotfix-retry",
			wantNumber:   7,
		},
		{
			name:         "a short single-word suffix is allowed",
			resourceName: "preview-pr-1-copy",
			wantNumber:   1,
		},
		{
			name:         "a numeric-looking suffix segment is just a suffix",
			resourceName: "preview-pr-42-2",
			wantNumber:   42,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			number, matched := ExtractPullRequestNumber(test.resourceName)

			if !matched {
				t.Fatalf("matched = false, want true")
			}

			if number != test.wantNumber {
				t.Fatalf("number = %d, want %d", number, test.wantNumber)
			}
		})
	}
}

func TestExtractPullRequestNumberInvalid(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
	}{
		{
			name:         "empty name",
			resourceName: "",
		},
		{
			name:         "production branch",
			resourceName: "production",
		},
		{
			name:         "staging branch",
			resourceName: "staging",
		},
		{
			name:         "production-like name containing preview",
			resourceName: "preview-production",
		},
		{
			name:         "zero is not a valid PR number",
			resourceName: "preview-pr-0",
		},
		{
			name:         "leading zero is rejected",
			resourceName: "preview-pr-01",
		},
		{
			name:         "negative number is rejected",
			resourceName: "preview-pr--1",
		},
		{
			name:         "missing number",
			resourceName: "preview-pr-",
		},
		{
			name:         "missing number and separator",
			resourceName: "preview-pr",
		},
		{
			name:         "non-numeric value in the number position",
			resourceName: "preview-pr-abc",
		},
		{
			name:         "missing the pr- segment entirely",
			resourceName: "preview",
		},
		{
			name:         "missing the pr- segment with a number",
			resourceName: "preview-42",
		},
		{
			name:         "additional prefix is rejected",
			resourceName: "old-preview-pr-1",
		},
		{
			name:         "additional suffix without a separator is rejected",
			resourceName: "preview-pr-1copy",
		},
		{
			name:         "number glued to trailing digits is rejected",
			resourceName: "preview-pr-142copy",
		},
		{
			name:         "uppercase suffix is rejected",
			resourceName: "preview-pr-1-COPY",
		},
		{
			name:         "uppercase pr keyword is rejected",
			resourceName: "preview-PR-1",
		},
		{
			name:         "uppercase preview keyword is rejected",
			resourceName: "Preview-pr-1",
		},
		{
			name:         "slash-separated suffix is not a recognized form",
			resourceName: "preview/pr-42/checkout",
		},
		{
			name:         "trailing separator with no suffix content",
			resourceName: "preview-pr-1-",
		},
		{
			name:         "unrelated name with an embedded number",
			resourceName: "my-database-42",
		},
		{
			name:         "whitespace around an otherwise valid name",
			resourceName: " preview-pr-1",
		},
		{
			name:         "PR number overflowing int is rejected safely",
			resourceName: "preview-pr-99999999999999999999",
		},
		{
			name:         "suffix embeds a second complete preview-pr pattern (dash)",
			resourceName: "preview-pr-42-preview-pr-9",
		},
		{
			name:         "suffix embeds a second complete preview-pr pattern (slash)",
			resourceName: "preview/pr-42-preview-pr-9",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			number, matched := ExtractPullRequestNumber(test.resourceName)

			if matched {
				t.Fatalf("matched = true, want false")
			}

			if number != 0 {
				t.Fatalf("number = %d, want 0", number)
			}
		})
	}
}
