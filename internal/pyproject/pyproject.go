package pyproject

import (
	"fmt"
	"os"
	"regexp"
)

// sectionRe matches a TOML section from its header to the next header (or end of file).
// TOML section headers always appear at the start of a line, so \n[ is a reliable boundary.
var (
	projectSectionRe = regexp.MustCompile(`(?s)\[project\].*?(?:\n\[|\z)`)
	poetrySectionRe  = regexp.MustCompile(`(?s)\[tool\.poetry\].*?(?:\n\[|\z)`)
)

// versionLineRe matches version = "x.y.z" on its own line inside a section.
var versionLineRe = regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)

// ReadVersion returns the project version from a pyproject.toml file.
// It checks [project] (PEP 621) first, then [tool.poetry] (Poetry).
func ReadVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	content := string(data)
	for _, sectionRe := range []*regexp.Regexp{projectSectionRe, poetrySectionRe} {
		section := sectionRe.FindString(content)
		if section == "" {
			continue
		}
		m := versionLineRe.FindStringSubmatch(section)
		if m != nil {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("no version found in %s", path)
}

// WriteVersion replaces the project version in a pyproject.toml file in-place.
// oldVersion must match what ReadVersion returned.
func WriteVersion(path, oldVersion, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	updated, ok := replaceVersion(string(data), oldVersion, newVersion)
	if !ok {
		return fmt.Errorf("version %q not found in %s", oldVersion, path)
	}
	return os.WriteFile(path, []byte(updated), 0644)
}

// replaceVersion finds and replaces the version in the first [project] or [tool.poetry]
// section that contains it. Returns the updated content and true on success.
// regexp.QuoteMeta is used so version strings with dots or specials are safe.
func replaceVersion(content, oldVersion, newVersion string) (string, bool) {
	re := regexp.MustCompile(`(?m)^(version\s*=\s*)"` + regexp.QuoteMeta(oldVersion) + `"`)
	for _, sectionRe := range []*regexp.Regexp{projectSectionRe, poetrySectionRe} {
		loc := sectionRe.FindStringIndex(content)
		if loc == nil {
			continue
		}
		section := content[loc[0]:loc[1]]
		m := re.FindStringSubmatchIndex(section)
		if m == nil {
			continue
		}
		// m[0]:m[1] = full match; m[2]:m[3] = group 1 (prefix "version = ")
		newSection := section[:m[0]] + section[m[2]:m[3]] + `"` + newVersion + `"` + section[m[1]:]
		return content[:loc[0]] + newSection + content[loc[1]:], true
	}
	return content, false
}
