package correlation

import (
	"regexp"
	"strconv"
)

// previewPRNamePattern recognizes Sweep's preview-resource naming
// convention:
//
//	preview-pr-<number>
//	preview-pr-<number>-<suffix>
//	preview/pr-<number>
//	preview/pr-<number>-<suffix>
//
// The literal "preview" and "pr-" segments, and the "-"/"/" separator
// between them, are matched case-sensitively and anchored at both ends, so
// a name must consist entirely of this shape - nothing extra before "preview"
// (rejects "old-preview-pr-1") or after the optional suffix. The number
// must not have a leading zero, so "0" and "01" are both rejected; a
// missing or non-numeric number ("preview-pr-", "preview-pr-abc") fails to
// match the same way. The optional suffix is one or more "-" separated
// lowercase alphanumeric segments (for example "-checkout" or
// "-checkout-e2e"), which is deliberately conservative: it can never
// itself contain the "preview"/"pr-" prefix shape, a "/" separator, or
// uppercase letters, so it cannot be mistaken for, or embed, an unrelated
// resource name.
var previewPRNamePattern = regexp.MustCompile(
	`^preview[-/]pr-([1-9][0-9]*)(?:-[a-z0-9]+)*$`,
)

// ExtractPullRequestNumber extracts a PR number from a recognized preview
// name. It returns false for names that don't match the convention at all,
// for a PR number of zero, and for any other malformed variant (a missing
// number, a negative number, or an unrecognized separator/suffix).
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
