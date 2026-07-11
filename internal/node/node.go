package node

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReadVersion returns the version field from a package.json file.
func ReadVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if pkg.Version == "" {
		return "", fmt.Errorf("no version field in %s", path)
	}
	return pkg.Version, nil
}

// WriteVersion replaces the version field in a package.json file in-place.
// oldVersion must match what ReadVersion returned. Formatting is preserved.
func WriteVersion(path, oldVersion, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	old := `"version": "` + oldVersion + `"`
	repl := `"version": "` + newVersion + `"`
	if !strings.Contains(string(data), old) {
		return fmt.Errorf("version %q not found in %s", oldVersion, path)
	}
	updated := strings.Replace(string(data), old, repl, 1)
	return os.WriteFile(path, []byte(updated), 0644)
}
