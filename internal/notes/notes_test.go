package notes

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	messages := []string{
		"feat: add OAuth2 login",
		"fix: correct null pointer in user service",
		"feat(search): improve performance",
		"chore: update dependencies",         // ignored
		"Merge branch 'feature/x' into main", // ignored
		"feat!: remove legacy API",
		"fix: handle empty response",
		"WIP something",                       // ignored
	}

	got := Generate("v1.2.4", messages)

	must := []string{
		"## v1.2.4",
		"### Breaking Changes",
		"- remove legacy API",
		"### Features",
		"- add OAuth2 login",
		"- improve performance",
		"### Bug Fixes",
		"- correct null pointer in user service",
		"- handle empty response",
	}
	for _, want := range must {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}

	absent := []string{"update dependencies", "Merge branch", "WIP"}
	for _, s := range absent {
		if strings.Contains(got, s) {
			t.Errorf("unexpected %q in output:\n%s", s, got)
		}
	}
}

func TestGenerateOnlyFixes(t *testing.T) {
	messages := []string{
		"fix: patch SQL injection",
		"fix: escape HTML output",
	}
	got := Generate("v1.2.1", messages)

	if strings.Contains(got, "### Features") {
		t.Error("Features section should be absent when there are no feat commits")
	}
	if strings.Contains(got, "### Breaking Changes") {
		t.Error("Breaking Changes section should be absent")
	}
	if !strings.Contains(got, "### Bug Fixes") {
		t.Error("Bug Fixes section should be present")
	}
}

func TestGenerateEmpty(t *testing.T) {
	got := Generate("v1.2.0", []string{"chore: bump deps", "docs: update readme"})
	if strings.Contains(got, "###") {
		t.Errorf("no sections expected when no releasable commits, got:\n%s", got)
	}
}

func TestExtractSubject(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"feat: add login", "add login"},
		{"feat(auth): add OAuth2", "add OAuth2"},
		{"feat!: remove API", "remove API"},
		{"FIX:typo", "typo"},
		{"plain message", "plain message"},
	}
	for _, c := range cases {
		got := extractSubject(c.header)
		if got != c.want {
			t.Errorf("extractSubject(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

// FuzzGenerate verifies that Generate never panics on arbitrary inputs and
// always includes the tag name in the output.
func FuzzGenerate(f *testing.F) {
	f.Add("v1.2.4", "feat: add thing")
	f.Add("v1.0.0", "fix: patch")
	f.Add("1.2.0", "feat!: breaking change")
	f.Add("v0.0.1", "")
	f.Add("", "feat: msg")
	f.Add("v1.0.0", "BREAKING CHANGE: dropped")
	f.Add("tag\nwith\nnewline", "feat: something")
	f.Add("v1.0.0", strings.Repeat("feat: x\n", 1000))

	f.Fuzz(func(t *testing.T, tag, msg string) {
		result := Generate(tag, []string{msg})
		if !strings.Contains(result, tag) {
			t.Errorf("output does not contain tag %q\noutput: %s", tag, result)
		}
	})
}
