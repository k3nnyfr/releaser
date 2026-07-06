package commits

import (
	"strings"
	"testing"
)

// FuzzParse verifies that the parser never panics and always returns a valid Type.
func FuzzParse(f *testing.F) {
	// Seed corpus: representative conventional commit messages
	seeds := []string{
		"feat: add OAuth2 login",
		"fix: correct null pointer",
		"feat!: remove legacy API",
		"feat(auth): add login\n\nBREAKING CHANGE: old endpoint removed",
		"chore: update deps",
		"Feat:missing space",
		"FIX(scope)!: mixed case breaking",
		"",
		"\x00",
		"BREAKING CHANGE: bare footer",
		"not a conventional commit at all",
		"feat: \n\n body only",
		strings.Repeat("a", 4096),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, msg string) {
		result := Parse(msg)
		if result < TypeNone || result > TypeBreaking {
			t.Errorf("Parse(%q) returned out-of-range type %d", msg, result)
		}
	})
}

func TestTypeString(t *testing.T) {
	cases := []struct {
		t    Type
		want string
	}{
		{TypeNone, "none"},
		{TypeFix, "fix"},
		{TypeFeat, "feat"},
		{TypeBreaking, "breaking"},
		{Type(99), "none"}, // unknown → default
	}
	for _, c := range cases {
		if got := c.t.String(); got != c.want {
			t.Errorf("Type(%d).String() = %q, want %q", c.t, got, c.want)
		}
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		msg  string
		want Type
	}{
		// standard cases
		{"feat: add login page", TypeFeat},
		{"fix: correct null pointer", TypeFix},
		{"chore: update deps", TypeNone},
		{"docs: update README", TypeNone},

		// non-strict: case-insensitive
		{"Feat: add login page", TypeFeat},
		{"FIX: correct null pointer", TypeFix},
		{"FEAT: add login", TypeFeat},

		// non-strict: missing space after colon
		{"feat:add login", TypeFeat},
		{"fix:correct null pointer", TypeFix},

		// with scope
		{"feat(auth): add OAuth2", TypeFeat},
		{"fix(db): prevent deadlock", TypeFix},

		// breaking change via !
		{"feat!: remove legacy API", TypeBreaking},
		{"fix!: change response format", TypeBreaking},
		{"feat(api)!: rename endpoint", TypeBreaking},

		// breaking change via footer
		{"feat: new API\n\nBREAKING CHANGE: old API removed", TypeBreaking},
		{"refactor: cleanup\n\nBREAKING CHANGE: interface changed", TypeBreaking},

		// unparseable — silently ignored
		{"WIP", TypeNone},
		{"Merge branch 'main' into release/1.2", TypeNone},
		{"", TypeNone},
		{"fix a bug without colon", TypeNone},
	}

	for _, c := range cases {
		got := Parse(c.msg)
		if got != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.msg, got, c.want)
		}
	}
}
