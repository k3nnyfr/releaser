package branch

import (
	"fmt"
	"regexp"
	"strconv"
)

// DefaultBranchPattern matches "release/X.Y" or "origin/release/X.Y".
// Group 1 = major, group 2 = minor.
const DefaultBranchPattern = `^(?:.*/)?release/(\d+)\.(\d+)$`

// Info holds the major.minor version pinned by the branch name, plus the tag prefix from config.
type Info struct {
	Major     int
	Minor     int
	TagPrefix string // set from config after Parse (default "v")
}

// Parse extracts major and minor from a branch name using the provided regex pattern.
// The pattern must contain exactly two capture groups: group 1 = major, group 2 = minor.
// TagPrefix is left at its zero value — the caller sets it from config.
func Parse(name, pattern string) (Info, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Info{}, fmt.Errorf("invalid branch pattern %q: %w", pattern, err)
	}

	m := re.FindStringSubmatch(name)
	if len(m) < 3 {
		return Info{}, fmt.Errorf("branch %q does not match pattern %q", name, pattern)
	}

	major, err := strconv.Atoi(m[1])
	if err != nil {
		return Info{}, fmt.Errorf("invalid major version in branch %q: %w", name, err)
	}
	minor, err := strconv.Atoi(m[2])
	if err != nil {
		return Info{}, fmt.Errorf("invalid minor version in branch %q: %w", name, err)
	}

	return Info{Major: major, Minor: minor}, nil
}

// MatchesTag checks whether a tag (e.g. "v1.2.3" or "1.2.3") belongs to this branch's version range.
// Uses TagPrefix to match the expected format. Returns the patch number and true on match.
func (i Info) MatchesTag(tag string) (patch int, ok bool) {
	prefix := fmt.Sprintf("%s%d.%d.", i.TagPrefix, i.Major, i.Minor)
	if len(tag) <= len(prefix) || tag[:len(prefix)] != prefix {
		return 0, false
	}
	p, err := strconv.Atoi(tag[len(prefix):])
	if err != nil {
		return 0, false
	}
	return p, true
}

// TagName formats a version string (e.g. "1.2.4") into a full tag name (e.g. "v1.2.4" or "1.2.4").
func (i Info) TagName(version string) string {
	return i.TagPrefix + version
}
