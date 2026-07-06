package branch

import (
	"strings"
	"testing"
)

// FuzzParse verifies the parser never panics, and that successful parses produce
// non-negative major/minor values.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"release/1.2",
		"hotfix/10.3",
		"origin/release/0.0",
		"main",
		"release/",
		"release/1.2.3",
		"",
		"\x00",
		"release/999999.999999",
		strings.Repeat("release/", 100) + "1.2",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, name string) {
		info, err := Parse(name, DefaultBranchPattern)
		if err != nil {
			return // invalid input is expected — just must not panic
		}
		if info.Major < 0 || info.Minor < 0 {
			t.Errorf("Parse(%q) returned negative major/minor: %+v", name, info)
		}
	})
}

func TestParse(t *testing.T) {
	cases := []struct {
		input   string
		pattern string
		major   int
		minor   int
		err     bool
	}{
		// default pattern
		{"release/1.2", DefaultBranchPattern, 1, 2, false},
		{"release/10.3", DefaultBranchPattern, 10, 3, false},
		{"origin/release/1.2", DefaultBranchPattern, 1, 2, false},
		{"refs/heads/release/1.2", DefaultBranchPattern, 1, 2, false},
		{"main", DefaultBranchPattern, 0, 0, true},
		{"release/1", DefaultBranchPattern, 0, 0, true},
		{"release/1.2.3", DefaultBranchPattern, 0, 0, true},
		{"feature/1.2", DefaultBranchPattern, 0, 0, true},

		// custom pattern — hotfix + release
		{"hotfix/2.3", `^(?:.*/)?(?:release|hotfix)/(\d+)\.(\d+)$`, 2, 3, false},
		{"release/2.3", `^(?:.*/)?(?:release|hotfix)/(\d+)\.(\d+)$`, 2, 3, false},
		{"feature/2.3", `^(?:.*/)?(?:release|hotfix)/(\d+)\.(\d+)$`, 0, 0, true},

		// invalid regex
		{"release/1.2", `(unclosed`, 0, 0, true},

		// valid regex but capture groups match non-digits → Atoi error
		{"release/abc.1", `^release/(\w+)\.(\d+)$`, 0, 0, true}, // major non-numeric
		{"release/1.abc", `^release/(\d+)\.(\w+)$`, 0, 0, true}, // minor non-numeric
	}

	for _, c := range cases {
		got, err := Parse(c.input, c.pattern)
		if c.err {
			if err == nil {
				t.Errorf("Parse(%q, %q): expected error, got none", c.input, c.pattern)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q, %q): unexpected error: %v", c.input, c.pattern, err)
			continue
		}
		if got.Major != c.major || got.Minor != c.minor {
			t.Errorf("Parse(%q) = {%d, %d}, want {%d, %d}", c.input, got.Major, got.Minor, c.major, c.minor)
		}
	}
}

func TestMatchesTag(t *testing.T) {
	cases := []struct {
		prefix string
		tag    string
		patch  int
		ok     bool
	}{
		{"v", "v1.2.0", 0, true},
		{"v", "v1.2.5", 5, true},
		{"v", "v1.2.99", 99, true},
		{"v", "v1.3.0", 0, false},
		{"v", "v2.2.0", 0, false},
		{"v", "1.2.0", 0, false},
		{"v", "v1.2.", 0, false},
		{"v", "v1.2.x", 0, false},
		{"", "1.2.0", 0, true},
		{"", "1.2.7", 7, true},
		{"", "v1.2.0", 0, false},
	}

	for _, c := range cases {
		info := Info{Major: 1, Minor: 2, TagPrefix: c.prefix}
		patch, ok := info.MatchesTag(c.tag)
		if ok != c.ok {
			t.Errorf("[prefix=%q] MatchesTag(%q): ok=%v, want %v", c.prefix, c.tag, ok, c.ok)
			continue
		}
		if ok && patch != c.patch {
			t.Errorf("[prefix=%q] MatchesTag(%q): patch=%d, want %d", c.prefix, c.tag, patch, c.patch)
		}
	}
}

func TestTagName(t *testing.T) {
	cases := []struct {
		prefix  string
		version string
		want    string
	}{
		{"v", "1.2.4", "v1.2.4"},
		{"", "1.2.4", "1.2.4"},
		{"release-", "1.2.4", "release-1.2.4"},
	}
	for _, c := range cases {
		info := Info{Major: 1, Minor: 2, TagPrefix: c.prefix}
		if got := info.TagName(c.version); got != c.want {
			t.Errorf("TagName(%q) = %q, want %q", c.version, got, c.want)
		}
	}
}
