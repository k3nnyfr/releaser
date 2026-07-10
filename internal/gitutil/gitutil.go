package gitutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"git.k3nny.fr/releaser/internal/branch"
)

// IsWorkingTreeClean returns true when there are no staged or modified tracked files.
// Untracked files are ignored so that unreleased work-in-progress does not block a release.
func IsWorkingTreeClean(repo *gogit.Repository) (bool, error) {
	w, err := repo.Worktree()
	if err != nil {
		return false, err
	}
	status, err := w.Status()
	if err != nil {
		return false, err
	}
	for _, s := range status {
		if s.Staging != gogit.Unmodified && s.Staging != gogit.Untracked {
			return false, nil
		}
		if s.Worktree != gogit.Unmodified && s.Worktree != gogit.Untracked {
			return false, nil
		}
	}
	return true, nil
}

// CurrentBranch returns the short branch name from HEAD.
// Returns an error when HEAD is detached (common in CI pipelines).
func CurrentBranch(repo *gogit.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("read HEAD: %w", err)
	}
	if !head.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is detached — use --branch to specify the release branch")
	}
	return head.Name().Short(), nil
}

type tagCandidate struct {
	name  string
	patch int
}

// LatestTag returns the highest-patch tag matching the branch's version range that is
// reachable from HEAD. Returns ("", -1, nil) when no matching ancestor tag exists.
func LatestTag(repo *gogit.Repository, info branch.Info) (string, int, error) {
	head, err := repo.Head()
	if err != nil {
		return "", -1, err
	}
	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", -1, err
	}

	refs, err := repo.Tags()
	if err != nil {
		return "", -1, err
	}

	var candidates []tagCandidate
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		patch, ok := info.MatchesTag(name)
		if !ok {
			return nil
		}

		commitHash, err := resolveTagToCommit(repo, ref)
		if err != nil {
			return nil // silently skip malformed tags
		}

		tagCommit, err := repo.CommitObject(commitHash)
		if err != nil {
			return nil
		}

		if tagCommit.Hash == headCommit.Hash {
			candidates = append(candidates, tagCandidate{name, patch})
			return nil
		}

		anc, err := tagCommit.IsAncestor(headCommit)
		if err != nil || !anc {
			return nil
		}

		candidates = append(candidates, tagCandidate{name, patch})
		return nil
	})
	if err != nil {
		return "", -1, err
	}

	if len(candidates) == 0 {
		return "", -1, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].patch > candidates[j].patch
	})

	best := candidates[0]
	return best.name, best.patch, nil
}

// CommitsSince returns commit messages from HEAD down to (but not including) the tagged commit.
func CommitsSince(repo *gogit.Repository, tagName string) ([]string, error) {
	ref, err := repo.Tag(tagName)
	if err != nil {
		return nil, fmt.Errorf("resolve tag %q: %w", tagName, err)
	}

	tagHash, err := resolveTagToCommit(repo, ref)
	if err != nil {
		return nil, fmt.Errorf("resolve tag commit: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, err
	}

	iter, err := repo.Log(&gogit.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, err
	}

	var messages []string
	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == tagHash {
			return storer.ErrStop
		}
		messages = append(messages, c.Message)
		return nil
	})
	return messages, err
}

// AllCommits returns all commit messages reachable from HEAD.
func AllCommits(repo *gogit.Repository) ([]string, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}

	iter, err := repo.Log(&gogit.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, err
	}

	var messages []string
	err = iter.ForEach(func(c *object.Commit) error {
		messages = append(messages, c.Message)
		return nil
	})
	return messages, err
}

// AuthorFromConfig reads user.name and user.email from the repository's git config.
// Local config overrides global; returns empty strings if neither is set.
func AuthorFromConfig(repo *gogit.Repository) (name, email string) {
	cfg, err := repo.ConfigScoped(gitconfig.GlobalScope)
	if err == nil {
		name = cfg.User.Name
		email = cfg.User.Email
	}
	// Local config overrides global
	local, err := repo.Config()
	if err == nil {
		if local.User.Name != "" {
			name = local.User.Name
		}
		if local.User.Email != "" {
			email = local.User.Email
		}
	}
	return
}

// CommitFiles stages all filePaths (relative to worktree root) and creates a commit.
func CommitFiles(repo *gogit.Repository, filePaths []string, message, authorName, authorEmail string) (plumbing.Hash, error) {
	w, err := repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	for _, p := range filePaths {
		if _, err := w.Add(p); err != nil {
			return plumbing.ZeroHash, fmt.Errorf("git add %s: %w", p, err)
		}
	}

	hash, err := w.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("git commit: %w", err)
	}
	return hash, nil
}

// CommitFile stages a single file and creates a commit.
func CommitFile(repo *gogit.Repository, filePath, message, authorName, authorEmail string) (plumbing.Hash, error) {
	return CommitFiles(repo, []string{filePath}, message, authorName, authorEmail)
}

// CreateTag creates a lightweight tag on HEAD.
func CreateTag(repo *gogit.Repository, tagName string) error {
	head, err := repo.Head()
	if err != nil {
		return err
	}
	if _, err := repo.CreateTag(tagName, head.Hash(), nil); err != nil {
		return fmt.Errorf("create tag %s: %w", tagName, err)
	}
	return nil
}

// Push pushes the given branch and tag to the "origin" remote.
// When token is non-empty, go-git is used with HTTPS basic auth (oauth2/token) — suitable for CI.
// When token is empty and the remote URL is SSH, go-git SSH agent auth is attempted first.
// Falls back to the system git binary so that credential helpers, netrc, and SSH keys work normally.
func Push(repo *gogit.Repository, branchName, tagName, token string) error {
	if token != "" {
		return pushWithGoGit(repo, branchName, tagName, token)
	}
	// Try SSH agent auth when the remote URL uses SSH transport.
	if remote, err := repo.Remote("origin"); err == nil {
		urls := remote.Config().URLs
		if len(urls) > 0 && isSSHURL(urls[0]) {
			if err := pushWithSSHAgent(repo, branchName, tagName); err == nil {
				return nil
			}
		}
	}
	return pushWithCLI(repo, branchName, tagName)
}

func isSSHURL(u string) bool {
	return strings.HasPrefix(u, "git@") || strings.HasPrefix(u, "ssh://")
}

func pushWithSSHAgent(repo *gogit.Repository, branchName, tagName string) error {
	auth, err := gitssh.NewSSHAgentAuth("git")
	if err != nil {
		return err
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("remote origin not found: %w", err)
	}

	opts := &gogit.PushOptions{
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branchName, branchName)),
			gitconfig.RefSpec(fmt.Sprintf("refs/tags/%s:refs/tags/%s", tagName, tagName)),
		},
		Auth: auth,
	}

	if err := remote.Push(opts); err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git push via SSH agent: %w", err)
	}
	return nil
}

func pushWithGoGit(repo *gogit.Repository, branchName, tagName, token string) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("remote origin not found: %w", err)
	}

	opts := &gogit.PushOptions{
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branchName, branchName)),
			gitconfig.RefSpec(fmt.Sprintf("refs/tags/%s:refs/tags/%s", tagName, tagName)),
		},
		Auth: &githttp.BasicAuth{
			Username: "oauth2",
			Password: token,
		},
	}

	if err := remote.Push(opts); err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func pushWithCLI(repo *gogit.Repository, branchName, tagName string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	cmd := exec.Command("git", "-C", wt.Filesystem.Root(), "push", "origin",
		fmt.Sprintf("HEAD:refs/heads/%s", branchName),
		fmt.Sprintf("refs/tags/%s:refs/tags/%s", tagName, tagName),
	)
	cmd.Stdout = os.Stderr // git push status goes to stderr conventionally
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

// resolveTagToCommit follows tag objects until it reaches a commit.
// Handles both lightweight tags (ref → commit) and annotated tags (ref → tag object → … → commit).
func resolveTagToCommit(repo *gogit.Repository, ref *plumbing.Reference) (plumbing.Hash, error) {
	hash := ref.Hash()
	for {
		obj, err := repo.Object(plumbing.AnyObject, hash)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		switch o := obj.(type) {
		case *object.Commit:
			return o.Hash, nil
		case *object.Tag:
			hash = o.Target
		default:
			return plumbing.ZeroHash, fmt.Errorf("unexpected object type %s at %s", obj.Type(), hash)
		}
	}
}
