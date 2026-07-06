package maven

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const simplePom = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>myapp</artifactId>
    <version>1.2.3</version>
    <name>My App</name>
</project>`

const pomWithParent = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.2.0</version>
    </parent>
    <groupId>com.example</groupId>
    <artifactId>myapp</artifactId>
    <version>1.2.3</version>
</project>`

const pomWithSnapshot = `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <artifactId>myapp</artifactId>
    <version>1.2.4-SNAPSHOT</version>
</project>`

func writePom(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pom.xml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadVersion(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"simple pom", simplePom, "1.2.3"},
		{"pom with parent", pomWithParent, "1.2.3"},
		{"snapshot version", pomWithSnapshot, "1.2.4-SNAPSHOT"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ReadVersion(writePom(t, c.content))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestReadVersionParentNotMatched(t *testing.T) {
	// ReadVersion must return the project version, NOT the parent version (3.2.0).
	got, err := ReadVersion(writePom(t, pomWithParent))
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3 (parent version must be ignored)", got)
	}
}

func TestWriteVersion(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		oldVersion string
		newVersion string
		wantInFile string
	}{
		{"simple replace", simplePom, "1.2.3", "1.2.4", "<version>1.2.4</version>"},
		{"with parent", pomWithParent, "1.2.3", "1.2.4", "<version>1.2.4</version>"},
		{"snapshot to release", pomWithSnapshot, "1.2.4-SNAPSHOT", "1.2.4", "<version>1.2.4</version>"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writePom(t, c.content)
			if err := WriteVersion(path, c.oldVersion, c.newVersion); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			data, _ := os.ReadFile(path)
			content := string(data)

			if !contains(content, c.wantInFile) {
				t.Errorf("expected %q in updated pom, got:\n%s", c.wantInFile, content)
			}

			// Parent version must be unchanged in the parent block case.
			if c.name == "with parent" && !contains(content, "<version>3.2.0</version>") {
				t.Error("parent version was modified")
			}
		})
	}
}

func TestWriteVersionParentPreserved(t *testing.T) {
	path := writePom(t, pomWithParent)
	if err := WriteVersion(path, "1.2.3", "1.2.4"); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadVersion(path)
	if got != "1.2.4" {
		t.Errorf("project version = %q, want 1.2.4", got)
	}

	data, _ := os.ReadFile(path)
	if !contains(string(data), "<version>3.2.0</version>") {
		t.Error("parent version was modified")
	}
}

func TestReadVersionMissingFile(t *testing.T) {
	_, err := ReadVersion(filepath.Join(t.TempDir(), "nonexistent.xml"))
	if err == nil {
		t.Error("expected error for missing pom file")
	}
}

func TestReadVersionNoVersionTag(t *testing.T) {
	_, err := ReadVersion(writePom(t, "<project><artifactId>app</artifactId></project>"))
	if err == nil {
		t.Error("expected error when no <version> tag present")
	}
}

func TestWriteVersionMissingFile(t *testing.T) {
	err := WriteVersion(filepath.Join(t.TempDir(), "nosuchfile.xml"), "1.0", "1.1")
	if err == nil {
		t.Error("expected error for missing pom file")
	}
}

func TestWriteVersionNotFound(t *testing.T) {
	err := WriteVersion(writePom(t, simplePom), "9.9.9", "9.9.10")
	if err == nil {
		t.Error("expected error when old version not found in pom")
	}
}

func TestReplaceVersionParentNoMatch(t *testing.T) {
	// Has </parent> but the target version only appears inside the parent block,
	// not after it. replaceProjectVersion should return content unchanged.
	content := `<project><parent><version>3.0.0</version></parent></project>`
	result := replaceProjectVersion(content, "3.0.0", "4.0.0")
	if result != content {
		t.Errorf("expected content unchanged; got:\n%s", result)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

// FuzzReadVersion verifies that ReadVersion never panics on arbitrary file content.
func FuzzReadVersion(f *testing.F) {
	f.Add(simplePom)
	f.Add(pomWithParent)
	f.Add(pomWithSnapshot)
	f.Add("")
	f.Add("<project></project>")
	f.Add("<version>1.0</version>")
	f.Add("\x00\x01\xff")

	f.Fuzz(func(t *testing.T, content string) {
		path := filepath.Join(t.TempDir(), "pom.xml")
		os.WriteFile(path, []byte(content), 0644) //nolint:errcheck
		// Must not panic regardless of content
		ReadVersion(path) //nolint:errcheck
	})
}

// FuzzReplaceProjectVersion verifies the internal replacer is robust against arbitrary inputs.
func FuzzReplaceProjectVersion(f *testing.F) {
	f.Add(simplePom, "1.2.3", "1.2.4")
	f.Add(pomWithParent, "1.2.3", "2.0.0")
	f.Add("", "1.0", "2.0")
	f.Add("<version>1.0</version>", "1.0", "1.1")
	f.Add("\x00", "", "1.0")

	f.Fuzz(func(t *testing.T, content, oldVersion, newVersion string) {
		result := replaceProjectVersion(content, oldVersion, newVersion)
		// Result must always be a string (no panic)
		// If a replacement happened, oldVersion must not appear where newVersion was inserted
		if oldVersion == newVersion {
			return
		}
		if result != content {
			// Replacement occurred — newVersion must appear in result
			if !strings.Contains(result, "<version>"+newVersion+"</version>") {
				t.Errorf("replacement happened but newVersion not found in result")
			}
		}
	})
}
