package correlation

import (
	"regexp"
	"strconv"
)

var previewPRNamePattern = regexp.MustCompile(
	`^preview-pr-([1-9][0-9]*)$`,
)

// ExtractPullRequestNumber extracts a PR number from a recognized preview name.
func ExtractPullRequestNumber(resourceName string) (int, bool) {
	matches := previewPRNamePattern.FindStringSubmatch(resourceName)
	if len(matches) != 2 {
		return 0, false
	}

	number, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}

	return number, true
}
