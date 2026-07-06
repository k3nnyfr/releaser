package commits

import (
	"regexp"
	"strings"
)

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
