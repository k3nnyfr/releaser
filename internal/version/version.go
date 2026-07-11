package version

import (
	"fmt"

	"git.k3nny.fr/releaser/internal/commits"
)

// BumpLevel controls which version component is incremented.
type BumpLevel int

const (
	BumpPatch BumpLevel = iota
	BumpMinor
)

// Next computes the next version string (without tag prefix, e.g. "1.2.4").
// currentPatch is -1 when no tag exists yet (first release will be X.Y.0).
// releasable is the set of commit types that trigger a bump; nil defaults to all three.
// bumpRules maps each type to its bump level; nil defaults to BumpPatch for all.
// Returns ("", false) when there are no releasable commits.
func Next(major, minor, currentPatch int, types []commits.Type, releasable map[commits.Type]bool, bumpRules map[commits.Type]BumpLevel) (string, bool) {
	if releasable == nil {
		releasable = commits.ReleasableSet(nil)
	}
	found := false
	useMinor := false
	for _, t := range types {
		if releasable[t] {
			found = true
			if bumpRules[t] == BumpMinor {
				useMinor = true
			}
		}
	}
	if !found {
		return "", false
	}
	if useMinor {
		return fmt.Sprintf("%d.%d.0", major, minor+1), true
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, currentPatch+1), true
}
