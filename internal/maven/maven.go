package maven

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var parentBlockRe = regexp.MustCompile(`(?s)<parent>.*?</parent>`)
var versionTagRe = regexp.MustCompile(`<version>([^<]+)</version>`)

// ReadVersion returns the project version from a pom.xml file.
// It ignores the <parent> block version and dependency versions.
func ReadVersion(pomPath string) (string, error) {
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", pomPath, err)
	}

	// Strip <parent>…</parent> before searching so we don't pick up the parent version.
	stripped := parentBlockRe.ReplaceAllString(string(data), "")

	m := versionTagRe.FindStringSubmatch(stripped)
	if m == nil {
		return "", fmt.Errorf("no <version> element found in %s", pomPath)
	}
	return strings.TrimSpace(m[1]), nil
}

// WriteVersion replaces the project version in a pom.xml file in-place.
// oldVersion must match what ReadVersion returned. The <parent> version is never touched.
func WriteVersion(pomPath, oldVersion, newVersion string) error {
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", pomPath, err)
	}

	updated := replaceProjectVersion(string(data), oldVersion, newVersion)
	if updated == string(data) {
		return fmt.Errorf("version %q not found in %s", oldVersion, pomPath)
	}

	return os.WriteFile(pomPath, []byte(updated), 0644)
}

// replaceProjectVersion replaces the first <version>oldVersion</version> that is not
// inside a <parent> block. This preserves the parent version and dependency versions.
func replaceProjectVersion(content, oldVersion, newVersion string) string {
	target := "<version>" + oldVersion + "</version>"
	replacement := "<version>" + newVersion + "</version>"

	// If there is a <parent> block, start searching after it.
	if idx := strings.Index(content, "</parent>"); idx >= 0 {
		after := content[idx:]
		if pos := strings.Index(after, target); pos >= 0 {
			abs := idx + pos
			return content[:abs] + replacement + content[abs+len(target):]
		}
		return content
	}

	return strings.Replace(content, target, replacement, 1)
}
