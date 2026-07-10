package commits

import (
	"regexp"
	"strings"
)

// headerSubjectRe captures the subject (description) from a conventional commit header.
var headerSubjectRe = regexp.MustCompile(`(?i)^\w+(?:\([^)]*\))?!?\s*:\s*(.+)`)

// ExtractSubject returns the description part of a conventional commit header.
// Falls back to the trimmed raw header if the pattern does not match.
func ExtractSubject(header string) string {
	m := headerSubjectRe.FindStringSubmatch(header)
	if m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(header)
}

// Group splits messages into breaking changes, features, and fixes.
// Only the first line of each message is considered; the subject is extracted.
// Messages with TypeNone are silently dropped.
func Group(messages []string) (breaking, feats, fixes []string) {
	for _, msg := range messages {
		t := Parse(msg)
		if t == TypeNone {
			continue
		}
		first := strings.SplitN(strings.TrimSpace(msg), "\n", 2)[0]
		subject := ExtractSubject(first)
		switch t {
		case TypeBreaking:
			breaking = append(breaking, subject)
		case TypeFeat:
			feats = append(feats, subject)
		case TypeFix:
			fixes = append(fixes, subject)
		}
	}
	return
}

// ReleasableSet converts a slice of type-name strings to a set for use in version.Next.
// An empty or nil slice defaults to all three releasable types (fix, feat, breaking).
func ReleasableSet(typeNames []string) map[Type]bool {
	if len(typeNames) == 0 {
		return map[Type]bool{TypeFix: true, TypeFeat: true, TypeBreaking: true}
	}
	m := make(map[Type]bool, len(typeNames))
	for _, name := range typeNames {
		switch strings.ToLower(name) {
		case "fix":
			m[TypeFix] = true
		case "feat":
			m[TypeFeat] = true
		case "breaking":
			m[TypeBreaking] = true
		}
	}
	return m
}

// Type represents the semantic weight of a commit for versioning purposes.
type Type int

const (
	TypeNone     Type = iota // no version bump
	TypeFix                  // fix: → patch bump
	TypeFeat                 // feat: → patch bump (minor is pinned to branch)
	TypeBreaking             // feat!: or BREAKING CHANGE → patch bump
)

// headerRe matches conventional commit headers in non-strict mode:
// case-insensitive, optional scope, optional breaking marker, flexible whitespace around colon.
var headerRe = regexp.MustCompile(`(?i)^(\w+)(?:\([^)]*\))?(!)?\s*:\s*\S`)

// Parse extracts the commit type from a commit message.
// Unparseable messages return TypeNone — never an error.
func Parse(message string) Type {
	msg := strings.TrimSpace(message)

	// BREAKING CHANGE anywhere in the body/footer takes priority (spec §10).
	if strings.Contains(msg, "BREAKING CHANGE") {
		return TypeBreaking
	}

	first := strings.SplitN(msg, "\n", 2)[0]
	m := headerRe.FindStringSubmatch(first)
	if m == nil {
		return TypeNone
	}

	if m[2] == "!" {
		return TypeBreaking
	}

	switch strings.ToLower(m[1]) {
	case "feat":
		return TypeFeat
	case "fix":
		return TypeFix
	default:
		return TypeNone
	}
}

func (t Type) String() string {
	switch t {
	case TypeFix:
		return "fix"
	case TypeFeat:
		return "feat"
	case TypeBreaking:
		return "breaking"
	default:
		return "none"
	}
}
