package notes

import (
	"fmt"
	"regexp"
	"strings"

	"git.k3nny.fr/releaser/internal/commits"
)

// headerSubjectRe captures the subject (description) part of a conventional commit header.
var headerSubjectRe = regexp.MustCompile(`(?i)^\w+(?:\([^)]*\))?!?\s*:\s*(.+)`)

// Generate produces grouped markdown release notes from a list of commit messages.
// Commits are grouped into Breaking Changes, Features, and Bug Fixes.
// Commits with no releasable type are omitted.
func Generate(tagName string, messages []string) string {
	var breaking, feats, fixes []string

	for _, msg := range messages {
		t := commits.Parse(msg)
		if t == commits.TypeNone {
			continue
		}
		first := strings.SplitN(strings.TrimSpace(msg), "\n", 2)[0]
		subject := extractSubject(first)

		switch t {
		case commits.TypeBreaking:
			breaking = append(breaking, subject)
		case commits.TypeFeat:
			feats = append(feats, subject)
		case commits.TypeFix:
			fixes = append(fixes, subject)
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s\n", tagName)
	writeSection(&sb, "Breaking Changes", breaking)
	writeSection(&sb, "Features", feats)
	writeSection(&sb, "Bug Fixes", fixes)

	return strings.TrimRight(sb.String(), "\n")
}

func writeSection(sb *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n### %s\n", title)
	for _, item := range items {
		fmt.Fprintf(sb, "- %s\n", item)
	}
}

// extractSubject returns the description part of a conventional commit header.
// Falls back to the raw header if the pattern does not match.
func extractSubject(header string) string {
	m := headerSubjectRe.FindStringSubmatch(header)
	if m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(header)
}
