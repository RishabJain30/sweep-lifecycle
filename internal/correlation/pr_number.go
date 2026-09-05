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
// itself contain a "/" separator or uppercase letters, so it cannot be
// mistaken for, or embed, an unrelated resource name. The suffix's
// segments alone, however, are not enough to rule out spelling out a
// second complete "preview-pr-<number>" shape (for example
// "preview-pr-42-preview-pr-9"), since "preview" and "pr" are themselves
// valid lowercase-alphanumeric segments - embeddedPreviewPRPattern below
// catches that case.
var previewPRNamePattern = regexp.MustCompile(
	`^preview[-/]pr-([1-9][0-9]*)(?:-[a-z0-9]+)*$`,
)

// embeddedPreviewPRPattern matches the "preview-pr-<number>" /
// "preview/pr-<number>" shape anywhere in a string, unanchored. It backs
// the ambiguity check in ExtractPullRequestNumber: a suffix that spells out
// a second complete preview-PR name leaves it unclear which PR number is
// authoritative, so any suffix containing this shape is rejected rather
// than silently correlated to the first number found.
var embeddedPreviewPRPattern = regexp.MustCompile(`preview[-/]pr-[0-9]+`)

// ExtractPullRequestNumber extracts a PR number from a recognized preview
// name. It returns false for names that don't match the convention at all,
// for a PR number of zero, for any other malformed variant (a missing
// number, a negative number, or an unrecognized separator/suffix), and for
// a name whose suffix embeds a second complete preview-PR shape (ambiguous
// as to which PR number applies).
func ExtractPullRequestNumber(resourceName string) (int, bool) {
	loc := previewPRNamePattern.FindStringSubmatchIndex(resourceName)
	if loc == nil {
		return 0, false
	}

	number, err := strconv.Atoi(resourceName[loc[2]:loc[3]])
	if err != nil {
		return 0, false
	}

	suffix := resourceName[loc[3]:]
	if embeddedPreviewPRPattern.MatchString(suffix) {
		return 0, false
	}

	return number, true
}
