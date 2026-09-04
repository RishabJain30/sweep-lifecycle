package domain

import "testing"

func TestPullRequestIsFinished(t *testing.T) {
	tests := []struct {
		name  string
		state PullRequestState
		want  bool
	}{
		{
			name:  "open pull request is not finished",
			state: PullRequestStateOpen,
			want:  false,
		},
		{
			name:  "closed pull request is finished",
			state: PullRequestStateClosed,
			want:  true,
		},
		{
			name:  "merged pull request is finished",
			state: PullRequestStateMerged,
			want:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pr := PullRequest{State: test.state}

			if got := pr.IsFinished(); got != test.want {
				t.Fatalf("IsFinished() = %v, want %v", got, test.want)
			}
		})
	}
}
