// Package util provides shared utilities for gh-wrapup CLI commands.
package util

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const maxSlugLen = 50

// nonAlphanumRe matches any character that is not a letter, digit, or hyphen.
var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9-]+`)

// multiHyphenRe matches runs of more than one consecutive hyphen.
var multiHyphenRe = regexp.MustCompile(`-{2,}`)

// Slugify converts a human-readable title into a git-safe branch name and
// prefixes it with the issue number.
//
// Examples:
//
//	Slugify("Fix sidebar navigation", 42) → "42-fix-sidebar-navigation"
//	Slugify("Add OAuth2 / SSO support!", 7) → "7-add-oauth2-sso-support"
func Slugify(title string, issueNumber int) string {
	// Lowercase.
	s := strings.ToLower(title)

	// Replace non-alphanumeric characters (including spaces, slashes, etc.) with hyphens.
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, s)

	// Collapse to lowercase ASCII only (drop accents etc. that survived Map).
	s = nonAlphanumRe.ReplaceAllString(s, "-")

	// Collapse runs of hyphens.
	s = multiHyphenRe.ReplaceAllString(s, "-")

	// Trim leading/trailing hyphens.
	s = strings.Trim(s, "-")

	// Truncate the title portion so the full branch stays ≤ maxSlugLen chars.
	prefix := fmt.Sprintf("%d-", issueNumber)
	maxTitle := maxSlugLen - len(prefix)
	if maxTitle < 1 {
		maxTitle = 1
	}
	if len(s) > maxTitle {
		s = s[:maxTitle]
		// Don't end on a hyphen.
		s = strings.TrimRight(s, "-")
	}

	return prefix + s
}
