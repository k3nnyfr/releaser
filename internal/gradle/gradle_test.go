package gradle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gradleGroovy = `plugins {
    id 'java'
}

group = 'com.example'
version = '1.2.3'
description = 'My project'
`

const gradleKotlin = `plugins {
    kotlin("jvm") version "1.9.0"
}

group = "com.example"
version = "1.2.3"
description = "My project"
`

func writeGradle(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "build.gradle")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ── ReadVersion ──────────────────────────────────────────────────────────────

func TestReadVersionGroovy(t *testing.T) {
	got, err := ReadVersion(writeGradle(t, gradleGroovy))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestReadVersionKotlin(t *testing.T) {
	got, err := ReadVersion(writeGradle(t, gradleKotlin))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestReadVersionNoSpaces(t *testing.T) {
	got, err := ReadVersion(writeGradle(t, `version="1.0.0"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.0.0" {
		t.Errorf("got %q, want 1.0.0", got)
	}
}

func TestReadVersionMissingFile(t *testing.T) {
	_, err := ReadVersion(filepath.Join(t.TempDir(), "build.gradle"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadVersionNoVersionField(t *testing.T) {
	_, err := ReadVersion(writeGradle(t, `group = "com.example"`))
	if err == nil {
		t.Error("expected error when no version assignment is present")
	}
}

// ── WriteVersion ─────────────────────────────────────────────────────────────

func TestWriteVersionKotlin(t *testing.T) {
	path := writeGradle(t, gradleKotlin)
	if err := WriteVersion(path, "1.2.3", "1.2.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `version = "1.2.4"`) {
		t.Errorf("expected updated version; got:\n%s", data)
	}
	// plugin version declaration must not be touched
	if !strings.Contains(string(data), `kotlin("jvm") version "1.9.0"`) {
		t.Error("plugin version was incorrectly modified")
	}
}

func TestWriteVersionGroovy(t *testing.T) {
	path := writeGradle(t, gradleGroovy)
	if err := WriteVersion(path, "1.2.3", "1.2.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `version = '1.2.4'`) {
		t.Errorf("expected updated version with single quotes; got:\n%s", data)
	}
}

func TestWriteVersionMissingFile(t *testing.T) {
	err := WriteVersion(filepath.Join(t.TempDir(), "build.gradle"), "1.0.0", "1.0.1")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestWriteVersionNotFound(t *testing.T) {
	path := writeGradle(t, gradleKotlin)
	err := WriteVersion(path, "9.9.9", "9.9.10")
	if err == nil {
		t.Error("expected error when old version not found in file")
	}
}

func TestWriteVersionReadOnly(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: chmod restrictions do not apply when running as root")
	}
	path := writeGradle(t, gradleKotlin)
	os.Chmod(path, 0444)
	defer os.Chmod(path, 0644)
	err := WriteVersion(path, "1.2.3", "1.2.4")
	if err == nil {
		t.Error("expected error writing to read-only file")
	}
}

// ── replaceVersion ────────────────────────────────────────────────────────────

func TestReplaceVersionNotFound(t *testing.T) {
	content := `group = "com.example"`
	got, ok := replaceVersion(content, "1.0.0", "1.0.1")
	if ok {
		t.Error("expected ok=false when version not present")
	}
	if got != content {
		t.Error("content should be unchanged when not found")
	}
}

// ── fuzz ──────────────────────────────────────────────────────────────────────

// FuzzReadVersion verifies ReadVersion never panics on arbitrary file content.
func FuzzReadVersion(f *testing.F) {
	f.Add(gradleGroovy)
	f.Add(gradleKotlin)
	f.Add(`version="1.0.0"`)
	f.Add(`group = "com.example"`)
	f.Add("")
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, content string) {
		path := filepath.Join(t.TempDir(), "build.gradle")
		os.WriteFile(path, []byte(content), 0644) //nolint:errcheck
		ReadVersion(path)                          //nolint:errcheck
	})
}

// FuzzWriteVersion verifies WriteVersion never panics on arbitrary content or version strings.
func FuzzWriteVersion(f *testing.F) {
	f.Add(gradleGroovy, "1.2.3", "1.2.4")
	f.Add(gradleKotlin, "1.2.3", "1.2.4")
	f.Add(`version="1.0.0"`, "1.0.0", "1.0.1")
	f.Add("", "1.0.0", "1.0.1")
	f.Add(`version = "1.0.0"`, "", "1.0.1")

	f.Fuzz(func(t *testing.T, content, oldVersion, newVersion string) {
		path := filepath.Join(t.TempDir(), "build.gradle")
		os.WriteFile(path, []byte(content), 0644)  //nolint:errcheck
		WriteVersion(path, oldVersion, newVersion)  //nolint:errcheck
	})
}
