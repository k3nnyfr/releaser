package version

import (
	"fmt"

	"git.k3nny.fr/releaser/internal/commits"
)

// Next computes the next version string (without tag prefix, e.g. "1.2.4").
// currentPatch is -1 when no tag exists yet (first release will be X.Y.0).
// Returns ("", false) when there are no releasable commits.
func Next(major, minor, currentPatch int, types []commits.Type) (string, bool) {
	for _, t := range types {
		if t != commits.TypeNone {
			return fmt.Sprintf("%d.%d.%d", major, minor, currentPatch+1), true
		}
	}
	return "", false
}
