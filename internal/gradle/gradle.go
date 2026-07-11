package gradle

import (
	"fmt"
	"os"
	"regexp"
)

// versionRe matches a Gradle/Kotlin DSL version assignment on its own line.
// Group 1 captures the version string (without quotes).
// Handles both double-quoted (Kotlin/Groovy) and single-quoted (Groovy) forms,
// with or without spaces around =.
var versionRe = regexp.MustCompile(`(?m)^[ \t]*version\s*=\s*["']([^"']+)["']`)

// ReadVersion returns the version value from a build.gradle or build.gradle.kts file.
func ReadVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	m := versionRe.FindStringSubmatch(string(data))
	if m == nil {
		return "", fmt.Errorf("no version assignment found in %s", path)
	}
	return m[1], nil
}

// WriteVersion replaces the version assignment in a build.gradle or build.gradle.kts
// file in-place. oldVersion must match what ReadVersion returned. Quote style is preserved.
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

// replaceVersion finds and replaces the first version assignment line in a Gradle build file.
// Quote style (single or double) of the original line is preserved.
// Returns the updated content and true if a replacement was made.
func replaceVersion(content, oldVersion, newVersion string) (string, bool) {
	re := regexp.MustCompile(`(?m)^([ \t]*version\s*=\s*)(["'])` + regexp.QuoteMeta(oldVersion) + `["']`)
	m := re.FindStringSubmatchIndex(content)
	if m == nil {
		return content, false
	}
	prefix := content[m[2]:m[3]] // "version = " etc., preserving whitespace
	quote := content[m[4]:m[5]]  // " or '
	return content[:m[0]] + prefix + quote + newVersion + quote + content[m[1]:], true
}
