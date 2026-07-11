package notes

import (
	"fmt"
	"strings"

	"git.k3nny.fr/releaser/internal/commits"
)

// Generate produces grouped markdown release notes from a list of commit messages.
// Commits are grouped into Breaking Changes, Features, and Bug Fixes.
// Commits with no releasable type are omitted.
func Generate(tagName string, messages []string) string {
	breaking, feats, fixes := commits.Group(messages)

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
