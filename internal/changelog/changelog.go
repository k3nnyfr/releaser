package changelog

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"git.k3nny.fr/releaser/internal/commits"
)

var headerSubjectRe = regexp.MustCompile(`(?i)^\w+(?:\([^)]*\))?!?\s*:\s*(.+)`)

// Update inserts a new release section into the CHANGELOG file at path.
// If the file does not exist it is created with a standard header.
// Only commits with a releasable type (fix, feat, breaking) produce bullets;
// if none are found the file is left untouched.
func Update(path, tag, version string, messages []string) error {
	section := buildSection(version, messages)
	if section == "" {
		return nil
	}

	existing := ""
	data, err := os.ReadFile(path)
	if err == nil {
		existing = string(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var out string
	if existing == "" {
		out = "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n" +
			section + "\n"
	} else {
		// Insert above the first ## [ heading so newest release is always at top.
		if idx := strings.Index(existing, "\n## ["); idx >= 0 {
			out = existing[:idx+1] + section + "\n\n" + existing[idx+1:]
		} else {
			out = strings.TrimRight(existing, "\n") + "\n\n" + section + "\n"
		}
	}

	return os.WriteFile(path, []byte(out), 0644)
}

func buildSection(version string, messages []string) string {
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

	if len(breaking)+len(feats)+len(fixes) == 0 {
		return ""
	}

	date := time.Now().Format("2006-01-02")
	var sb strings.Builder
	fmt.Fprintf(&sb, "## [%s] - %s\n", version, date)
	writeSection(&sb, "Breaking Changes", breaking)
	writeSection(&sb, "Added", feats)
	writeSection(&sb, "Fixed", fixes)

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

func extractSubject(header string) string {
	m := headerSubjectRe.FindStringSubmatch(header)
	if m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(header)
}
