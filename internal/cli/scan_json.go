package cli

import (
	"encoding/json"

	scanservice "github.com/RishabJain30/sweep-lifecycle/internal/scan"
	"github.com/RishabJain30/sweep-lifecycle/internal/scoring"
)

const highConfidenceLevel = scoring.ConfidenceHigh

// The types below define Sweep's stable JSON scan report schema
// (`sweep scan --format json`). Fields are deliberately explicit and
// snake_case rather than a direct encoding of internal Go types, so
// internal refactors do not silently change the schema. Nothing here ever
// carries a credential: only resource identifiers, pull request metadata,
// and evidence/score text.

type jsonReport struct {
	Providers  []jsonProviderStatus `json:"providers"`
	Candidates []jsonCandidate      `json:"candidates"`
	Skipped    []jsonSkipped        `json:"skipped"`
	Warnings   []jsonWarning        `json:"warnings"`
	Summary    jsonSummary          `json:"summary"`
}

type jsonProviderStatus struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
}

type jsonCandidate struct {
	Provider     string           `json:"provider"`
	ResourceID   string           `json:"resource_id"`
	ResourceName string           `json:"resource_name"`
	PullRequest  *jsonPullRequest `json:"pull_request"`
	SourceBranch jsonSourceBranch `json:"source_branch"`
	Score        jsonScore        `json:"score"`
}

type jsonPullRequest struct {
	Number         int    `json:"number"`
	State          string `json:"state"`
	HeadRepository string `json:"head_repository"`
	HeadBranch     string `json:"head_branch"`
}

type jsonSourceBranch struct {
	Checked bool `json:"checked"`
	Exists  bool `json:"exists"`
}

type jsonScore struct {
	PolicyVersion  string         `json:"policy_version"`
	Value          int            `json:"value"`
	Confidence     string         `json:"confidence"`
	Recommendation string         `json:"recommendation"`
	Evidence       []jsonEvidence `json:"evidence"`
}

type jsonEvidence struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Points      int    `json:"points"`
}

type jsonSkipped struct {
	Provider     string `json:"provider"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Reason       string `json:"reason"`
}

type jsonWarning struct {
	Provider string `json:"provider"`
	Resource string `json:"resource"`
	Message  string `json:"message"`
}

type jsonSummary struct {
	CandidateCount      int `json:"candidate_count"`
	HighConfidenceCount int `json:"high_confidence_count"`
	SkippedCount        int `json:"skipped_count"`
	WarningCount        int `json:"warning_count"`
}

// toJSONReport translates a scan Result into Sweep's stable JSON schema.
func toJSONReport(result scanservice.Result) jsonReport {
	report := jsonReport{
		Providers:  make([]jsonProviderStatus, 0, len(result.ProviderStatuses)),
		Candidates: make([]jsonCandidate, 0, len(result.Candidates)),
		Skipped:    make([]jsonSkipped, 0, len(result.Skipped)),
		Warnings:   make([]jsonWarning, 0, len(result.Warnings)),
	}

	for _, status := range result.ProviderStatuses {
		report.Providers = append(report.Providers, jsonProviderStatus{
			Provider: status.Provider,
			Status:   status.Status,
			Detail:   status.Detail,
		})
	}

	highConfidence := 0

	for _, candidate := range result.Candidates {
		if candidate.Score.Confidence == highConfidenceLevel {
			highConfidence++
		}

		report.Candidates = append(report.Candidates, toJSONCandidate(candidate))
	}

	for _, skipped := range result.Skipped {
		report.Skipped = append(report.Skipped, jsonSkipped{
			Provider:     skipped.Provider,
			ResourceID:   skipped.ResourceID,
			ResourceName: skipped.ResourceName,
			Reason:       skipped.Reason,
		})
	}

	for _, warning := range result.Warnings {
		report.Warnings = append(report.Warnings, jsonWarning{
			Provider: warning.Provider,
			Resource: warning.Resource,
			Message:  warning.Message,
		})
	}

	report.Summary = jsonSummary{
		CandidateCount:      len(report.Candidates),
		HighConfidenceCount: highConfidence,
		SkippedCount:        len(report.Skipped),
		WarningCount:        len(report.Warnings),
	}

	return report
}

func toJSONCandidate(candidate scanservice.Candidate) jsonCandidate {
	jc := jsonCandidate{
		Provider:     candidate.Provider,
		ResourceID:   candidate.ResourceID,
		ResourceName: candidate.ResourceName,
		SourceBranch: jsonSourceBranch{
			Checked: candidate.SourceBranchChecked,
			Exists:  candidate.SourceBranchExists,
		},
		Score: toJSONScore(candidate),
	}

	if candidate.PullRequestFound {
		jc.PullRequest = &jsonPullRequest{
			Number:         candidate.PullRequest.Number,
			State:          string(candidate.PullRequest.State),
			HeadRepository: candidate.PullRequest.HeadRepository,
			HeadBranch:     candidate.PullRequest.HeadBranch,
		}
	}

	return jc
}

func toJSONScore(candidate scanservice.Candidate) jsonScore {
	score := jsonScore{
		PolicyVersion:  candidate.Score.PolicyVersion,
		Value:          candidate.Score.Score,
		Confidence:     string(candidate.Score.Confidence),
		Recommendation: candidate.Score.Recommendation,
		Evidence:       make([]jsonEvidence, 0, len(candidate.Score.Contributions)),
	}

	for _, contribution := range candidate.Score.Contributions {
		score.Evidence = append(score.Evidence, jsonEvidence{
			Kind:        string(contribution.Kind),
			Description: contribution.Description,
			Points:      contribution.Points,
		})
	}

	return score
}

func marshalReport(result scanservice.Result) ([]byte, error) {
	return json.MarshalIndent(toJSONReport(result), "", "  ")
}
