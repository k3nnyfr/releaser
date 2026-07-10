package version

import (
	"testing"

	"git.k3nny.fr/releaser/internal/commits"
)

func TestNext(t *testing.T) {
	cases := []struct {
		desc         string
		major, minor int
		currentPatch int
		types        []commits.Type
		want         string
		wantOk       bool
	}{
		{
			desc:         "first release with feat",
			major:        1, minor: 2, currentPatch: -1,
			types:  []commits.Type{commits.TypeFeat},
			want:   "1.2.0",
			wantOk: true,
		},
		{
			desc:         "bump from patch 3",
			major:        1, minor: 2, currentPatch: 3,
			types:  []commits.Type{commits.TypeFix},
			want:   "1.2.4",
			wantOk: true,
		},
		{
			desc:         "breaking change still bumps patch on release branch",
			major:        2, minor: 0, currentPatch: 1,
			types:  []commits.Type{commits.TypeBreaking},
			want:   "2.0.2",
			wantOk: true,
		},
		{
			desc:         "only chore commits — nothing to release",
			major:        1, minor: 2, currentPatch: 5,
			types:  []commits.Type{commits.TypeNone, commits.TypeNone},
			want:   "",
			wantOk: false,
		},
		{
			desc:         "no commits at all",
			major:        1, minor: 2, currentPatch: 0,
			types:  nil,
			want:   "",
			wantOk: false,
		},
		{
			desc:         "one releasable commit among chores",
			major:        1, minor: 3, currentPatch: 7,
			types:  []commits.Type{commits.TypeNone, commits.TypeFix, commits.TypeNone},
			want:   "1.3.8",
			wantOk: true,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got, ok := Next(c.major, c.minor, c.currentPatch, c.types, nil)
			if ok != c.wantOk {
				t.Errorf("ok=%v, want %v", ok, c.wantOk)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
