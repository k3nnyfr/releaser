package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	if err := Update(path, "v1.2.0", "1.2.0", []string{
		"feat: add widget",
		"fix: off-by-one in parser",
	}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "## [1.2.0]") {
		t.Error("expected version header")
	}
	if !strings.Contains(s, "### Added") {
		t.Error("expected Added section")
	}
	if !strings.Contains(s, "### Fixed") {
		t.Error("expected Fixed section")
	}
	if !strings.Contains(s, "add widget") {
		t.Error("expected feat subject")
	}
	if !strings.Contains(s, "off-by-one in parser") {
		t.Error("expected fix subject")
	}
}

func TestUpdateExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	// Seed with an older release.
	os.WriteFile(path, []byte("# Changelog\n\n## [1.1.0] - 2026-01-01\n\n### Added\n- old stuff\n"), 0644)

	if err := Update(path, "v1.2.0", "1.2.0", []string{"feat: new thing"}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	s := string(data)

	newIdx := strings.Index(s, "## [1.2.0]")
	oldIdx := strings.Index(s, "## [1.1.0]")
	if newIdx < 0 || oldIdx < 0 {
		t.Fatal("both versions should appear in CHANGELOG")
	}
	if newIdx > oldIdx {
		t.Error("new version should appear before old version")
	}
}

func TestUpdateNoReleasableCommits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	if err := Update(path, "v1.0.1", "1.0.1", []string{
		"chore: update deps",
		"docs: fix typo",
	}); err != nil {
		t.Fatal(err)
	}

	// File should NOT have been created.
	if _, err := os.Stat(path); err == nil {
		t.Error("file should not be created when there are no releasable commits")
	}
}

func TestUpdateBreakingSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	if err := Update(path, "v2.0.0", "2.0.0", []string{
		"feat!: redesign API",
		"fix(core): nil panic",
	}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "### Breaking Changes") {
		t.Error("expected Breaking Changes section")
	}
	if !strings.Contains(s, "redesign API") {
		t.Error("expected breaking subject")
	}
}

func TestUpdateReadError(t *testing.T) {
	dir := t.TempDir()
	// Create a directory where the file should be — ReadFile will error.
	os.Mkdir(filepath.Join(dir, "CHANGELOG.md"), 0755)

	err := Update(filepath.Join(dir, "CHANGELOG.md"), "v1.0.0", "1.0.0", []string{"fix: something"})
	if err == nil {
		t.Error("expected error when path is a directory")
	}
}
