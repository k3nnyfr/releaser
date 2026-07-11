package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSON(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const simplePackage = `{
  "name": "my-app",
  "version": "1.2.3",
  "description": "test"
}`

func TestReadVersion(t *testing.T) {
	got, err := ReadVersion(writeJSON(t, simplePackage))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestReadVersionMissingFile(t *testing.T) {
	_, err := ReadVersion(filepath.Join(t.TempDir(), "package.json"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadVersionInvalidJSON(t *testing.T) {
	_, err := ReadVersion(writeJSON(t, `{not valid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadVersionNoVersionField(t *testing.T) {
	_, err := ReadVersion(writeJSON(t, `{"name":"app"}`))
	if err == nil {
		t.Error("expected error when version field is absent")
	}
}

func TestWriteVersion(t *testing.T) {
	path := writeJSON(t, simplePackage)
	if err := WriteVersion(path, "1.2.3", "1.2.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"version": "1.2.4"`) {
		t.Errorf("expected updated version in file; got:\n%s", data)
	}
	// name and description must be preserved
	if !strings.Contains(string(data), `"name": "my-app"`) {
		t.Error("name field was lost")
	}
}

func TestWriteVersionMissingFile(t *testing.T) {
	err := WriteVersion(filepath.Join(t.TempDir(), "package.json"), "1.0.0", "1.0.1")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestWriteVersionNotFound(t *testing.T) {
	err := WriteVersion(writeJSON(t, simplePackage), "9.9.9", "9.9.10")
	if err == nil {
		t.Error("expected error when old version not found")
	}
}

// FuzzReadVersion verifies ReadVersion never panics on arbitrary content.
func FuzzReadVersion(f *testing.F) {
	f.Add(simplePackage)
	f.Add(`{}`)
	f.Add(`{"version":"1.0.0"}`)
	f.Add(`not json at all`)
	f.Add(``)
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, content string) {
		path := filepath.Join(t.TempDir(), "package.json")
		os.WriteFile(path, []byte(content), 0644) //nolint:errcheck
		ReadVersion(path)                          //nolint:errcheck
	})
}
