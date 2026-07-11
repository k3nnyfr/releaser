package pyproject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pyprojectPEP621 = `[project]
name = "my-package"
version = "1.2.3"
description = "A test package"
dependencies = ["requests>=2.0", "click>=8.0"]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"
`

const pyprojectPoetry = `[tool.poetry]
name = "my-package"
version = "1.2.3"
description = "A test package"

[tool.poetry.dependencies]
python = "^3.9"
requests = "^2.0"
`

const pyprojectBoth = `[project]
name = "my-package"
version = "1.2.3"

[tool.poetry]
name = "my-package"
version = "1.2.3"
`

const pyprojectBuildSystemFirst = `[build-system]
requires = ["hatchling"]

[project]
name = "my-package"
version = "1.2.3"
`

func writePyproject(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pyproject.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ── ReadVersion ───────────────────────────────────────────────────────────────

func TestReadVersionPEP621(t *testing.T) {
	got, err := ReadVersion(writePyproject(t, pyprojectPEP621))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestReadVersionPoetry(t *testing.T) {
	got, err := ReadVersion(writePyproject(t, pyprojectPoetry))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestReadVersionPEP621TakesPrecedence(t *testing.T) {
	// When both [project] and [tool.poetry] are present, [project] wins.
	got, err := ReadVersion(writePyproject(t, pyprojectBoth))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestReadVersionBuildSystemIgnored(t *testing.T) {
	// version under [build-system] must not be returned.
	got, err := ReadVersion(writePyproject(t, pyprojectBuildSystemFirst))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestReadVersionMissingFile(t *testing.T) {
	_, err := ReadVersion(filepath.Join(t.TempDir(), "pyproject.toml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadVersionNoVersion(t *testing.T) {
	_, err := ReadVersion(writePyproject(t, `[build-system]
requires = ["hatchling"]
`))
	if err == nil {
		t.Error("expected error when no version is found")
	}
}

// ── WriteVersion ──────────────────────────────────────────────────────────────

func TestWriteVersionPEP621(t *testing.T) {
	path := writePyproject(t, pyprojectPEP621)
	if err := WriteVersion(path, "1.2.3", "1.2.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, `version = "1.2.4"`) {
		t.Errorf("expected updated version; got:\n%s", s)
	}
	// build-system section must not be touched
	if strings.Contains(s, `"hatchling>=1.2.4"`) {
		t.Error("build-system section was incorrectly modified")
	}
}

func TestWriteVersionPoetry(t *testing.T) {
	path := writePyproject(t, pyprojectPoetry)
	if err := WriteVersion(path, "1.2.3", "1.2.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `version = "1.2.4"`) {
		t.Errorf("expected updated version; got:\n%s", data)
	}
}

func TestWriteVersionBothSectionsUpdatesProject(t *testing.T) {
	path := writePyproject(t, pyprojectBoth)
	if err := WriteVersion(path, "1.2.3", "1.2.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	// [project] version updated
	if !strings.Contains(s, `[project]`) || strings.Index(s, `version = "1.2.4"`) > strings.Index(s, `[tool.poetry]`) {
		t.Error("expected [project] version to be updated first")
	}
}

func TestWriteVersionMissingFile(t *testing.T) {
	err := WriteVersion(filepath.Join(t.TempDir(), "pyproject.toml"), "1.0.0", "1.0.1")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestWriteVersionNotFound(t *testing.T) {
	path := writePyproject(t, pyprojectPEP621)
	err := WriteVersion(path, "9.9.9", "9.9.10")
	if err == nil {
		t.Error("expected error when old version not found")
	}
}

func TestWriteVersionReadOnly(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: chmod restrictions do not apply when running as root")
	}
	path := writePyproject(t, pyprojectPEP621)
	os.Chmod(path, 0444)
	defer os.Chmod(path, 0644)
	if err := WriteVersion(path, "1.2.3", "1.2.4"); err == nil {
		t.Error("expected error writing to read-only file")
	}
}

// ── replaceVersion ────────────────────────────────────────────────────────────

func TestReplaceVersionNotFound(t *testing.T) {
	content := `[build-system]
requires = ["hatchling"]
`
	got, ok := replaceVersion(content, "1.0.0", "1.0.1")
	if ok {
		t.Error("expected ok=false when no matching section")
	}
	if got != content {
		t.Error("content should be unchanged when not found")
	}
}

// ── fuzz ─────────────────────────────────────────────────────────────────────

// FuzzReadVersion verifies ReadVersion never panics on arbitrary file content.
func FuzzReadVersion(f *testing.F) {
	f.Add(pyprojectPEP621)
	f.Add(pyprojectPoetry)
	f.Add(pyprojectBoth)
	f.Add(`[project]` + "\n")
	f.Add("")
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, content string) {
		path := filepath.Join(t.TempDir(), "pyproject.toml")
		os.WriteFile(path, []byte(content), 0644) //nolint:errcheck
		ReadVersion(path)                          //nolint:errcheck
	})
}

// FuzzWriteVersion verifies WriteVersion never panics on arbitrary content or version strings.
func FuzzWriteVersion(f *testing.F) {
	f.Add(pyprojectPEP621, "1.2.3", "1.2.4")
	f.Add(pyprojectPoetry, "1.2.3", "1.2.4")
	f.Add(pyprojectBoth, "1.2.3", "1.2.4")
	f.Add("", "1.0.0", "1.0.1")
	f.Add(`[project]`+"\n"+"version = \"1.0.0\"\n", "1.0.0", "1.0.1")

	f.Fuzz(func(t *testing.T, content, oldVersion, newVersion string) {
		path := filepath.Join(t.TempDir(), "pyproject.toml")
		os.WriteFile(path, []byte(content), 0644)  //nolint:errcheck
		WriteVersion(path, oldVersion, newVersion)  //nolint:errcheck
	})
}
