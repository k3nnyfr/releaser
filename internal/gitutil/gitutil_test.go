package gitutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"git.k3nny.fr/releaser/internal/branch"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func newTestRepo(t *testing.T) (*gogit.Repository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	return repo, dir
}

func testSig() *object.Signature {
	return &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()}
}

// addCommit writes content to test.txt, stages it, and creates a commit.
func addCommit(t *testing.T, repo *gogit.Repository, dir, message, content string) plumbing.Hash {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("test.txt"); err != nil {
		t.Fatal(err)
	}
	hash, err := w.Commit(message, &gogit.CommitOptions{Author: testSig()})
	if err != nil {
		t.Fatalf("commit %q: %v", message, err)
	}
	return hash
}

func addTag(t *testing.T, repo *gogit.Repository, name string) {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTag(name, head.Hash(), nil); err != nil {
		t.Fatalf("create lightweight tag %s: %v", name, err)
	}
}

func addAnnotatedTag(t *testing.T, repo *gogit.Repository, name string) {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateTag(name, head.Hash(), &gogit.CreateTagOptions{
		Tagger:  testSig(),
		Message: "release " + name,
	})
	if err != nil {
		t.Fatalf("create annotated tag %s: %v", name, err)
	}
}

// ── IsWorkingTreeClean ────────────────────────────────────────────────────────

func TestIsWorkingTreeClean(t *testing.T) {
	t.Run("clean after commit", func(t *testing.T) {
		repo, dir := newTestRepo(t)
		addCommit(t, repo, dir, "chore: init", "initial")

		clean, err := IsWorkingTreeClean(repo)
		if err != nil {
			t.Fatal(err)
		}
		if !clean {
			t.Error("expected clean working tree after commit")
		}
	})

	t.Run("dirty when tracked file is modified", func(t *testing.T) {
		repo, dir := newTestRepo(t)
		addCommit(t, repo, dir, "chore: init", "initial")

		os.WriteFile(filepath.Join(dir, "test.txt"), []byte("modified"), 0644)

		clean, err := IsWorkingTreeClean(repo)
		if err != nil {
			t.Fatal(err)
		}
		if clean {
			t.Error("expected dirty working tree after modification")
		}
	})

	t.Run("dirty when new file is staged", func(t *testing.T) {
		repo, dir := newTestRepo(t)
		addCommit(t, repo, dir, "chore: init", "initial")

		os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("new"), 0644)
		w, _ := repo.Worktree()
		w.Add("staged.txt")

		clean, err := IsWorkingTreeClean(repo)
		if err != nil {
			t.Fatal(err)
		}
		if clean {
			t.Error("expected dirty working tree with staged file")
		}
	})

	t.Run("untracked file does not dirty the tree", func(t *testing.T) {
		repo, dir := newTestRepo(t)
		addCommit(t, repo, dir, "chore: init", "initial")
		os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new"), 0644)

		clean, err := IsWorkingTreeClean(repo)
		if err != nil {
			t.Fatal(err)
		}
		if !clean {
			t.Error("untracked file should not dirty the working tree")
		}
	})
}

// ── CurrentBranch ─────────────────────────────────────────────────────────────

func TestCurrentBranch(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "initial")

	name, err := CurrentBranch(repo)
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Error("branch name should not be empty")
	}
}

// clearCIBranchEnv unsets the CI branch variables so detached-HEAD tests
// behave the same on a developer machine and inside an actual CI job.
func clearCIBranchEnv(t *testing.T) {
	t.Helper()
	for _, name := range ciBranchEnvVars {
		t.Setenv(name, "")
	}
}

func detachHead(t *testing.T, repo *gogit.Repository) {
	t.Helper()
	head, _ := repo.Head()
	detached := plumbing.NewHashReference(plumbing.HEAD, head.Hash())
	if err := repo.Storer.SetReference(detached); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentBranchDetached(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "initial")
	detachHead(t, repo)
	clearCIBranchEnv(t)

	_, err := CurrentBranch(repo)
	if err == nil {
		t.Error("expected error for detached HEAD")
	}
}

func TestCurrentBranchDetachedCIFallback(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "initial")
	detachHead(t, repo)

	t.Run("CI_COMMIT_BRANCH wins over CI_COMMIT_REF_NAME", func(t *testing.T) {
		clearCIBranchEnv(t)
		t.Setenv("CI_COMMIT_BRANCH", "release/1.2")
		t.Setenv("CI_COMMIT_REF_NAME", "release/9.9")

		name, err := CurrentBranch(repo)
		if err != nil {
			t.Fatal(err)
		}
		if name != "release/1.2" {
			t.Errorf("got %q, want release/1.2", name)
		}
	})

	t.Run("CI_COMMIT_REF_NAME when CI_COMMIT_BRANCH unset", func(t *testing.T) {
		clearCIBranchEnv(t)
		t.Setenv("CI_COMMIT_REF_NAME", "release/3.4")

		name, err := CurrentBranch(repo)
		if err != nil {
			t.Fatal(err)
		}
		if name != "release/3.4" {
			t.Errorf("got %q, want release/3.4", name)
		}
	})

	t.Run("GITHUB_REF_NAME as last resort", func(t *testing.T) {
		clearCIBranchEnv(t)
		t.Setenv("GITHUB_REF_NAME", "release/5.6")

		name, err := CurrentBranch(repo)
		if err != nil {
			t.Fatal(err)
		}
		if name != "release/5.6" {
			t.Errorf("got %q, want release/5.6", name)
		}
	})

	t.Run("env ignored when HEAD is on a branch", func(t *testing.T) {
		attached, attachedDir := newTestRepo(t)
		addCommit(t, attached, attachedDir, "chore: init", "initial")
		clearCIBranchEnv(t)
		t.Setenv("CI_COMMIT_BRANCH", "release/9.9")

		name, err := CurrentBranch(attached)
		if err != nil {
			t.Fatal(err)
		}
		if name == "release/9.9" {
			t.Error("CI env var must not override an attached HEAD")
		}
	})
}

// ── LatestTag ─────────────────────────────────────────────────────────────────

func TestLatestTagNoTags(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: first", "v1")

	info := branch.Info{Major: 1, Minor: 2, TagPrefix: "v"}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "" || patch != -1 {
		t.Errorf("expected no tag: got %q patch=%d", tag, patch)
	}
}

func TestLatestTagSingle(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: first", "v1")
	addTag(t, repo, "v1.2.0")
	addCommit(t, repo, dir, "fix: second", "v2")

	info := branch.Info{Major: 1, Minor: 2, TagPrefix: "v"}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.2.0" || patch != 0 {
		t.Errorf("got tag=%q patch=%d, want v1.2.0 patch=0", tag, patch)
	}
}

func TestLatestTagPicksHighestPatch(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.2.0")
	addCommit(t, repo, dir, "fix: c2", "v2")
	addTag(t, repo, "v1.2.1")
	addCommit(t, repo, dir, "feat: c3", "v3")
	addTag(t, repo, "v1.2.5")
	addCommit(t, repo, dir, "fix: c4", "v4") // HEAD after the last tag

	info := branch.Info{Major: 1, Minor: 2, TagPrefix: "v"}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.2.5" || patch != 5 {
		t.Errorf("got tag=%q patch=%d, want v1.2.5 patch=5", tag, patch)
	}
}

func TestLatestTagCrossVersionIgnored(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.2.0") // matching
	addCommit(t, repo, dir, "fix: c2", "v2")
	addTag(t, repo, "v1.3.0") // different minor — must be ignored
	addCommit(t, repo, dir, "fix: c3", "v3")

	info := branch.Info{Major: 1, Minor: 2, TagPrefix: "v"}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.2.0" || patch != 0 {
		t.Errorf("got tag=%q patch=%d, want v1.2.0 patch=0 (v1.3.0 must be ignored)", tag, patch)
	}
}

func TestLatestTagNoPrefixVariant(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "2.0.0") // no "v" prefix
	addCommit(t, repo, dir, "fix: c2", "v2")

	info := branch.Info{Major: 2, Minor: 0, TagPrefix: ""}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "2.0.0" || patch != 0 {
		t.Errorf("got tag=%q patch=%d, want 2.0.0 patch=0", tag, patch)
	}
}

func TestLatestTagAnnotated(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addAnnotatedTag(t, repo, "v1.2.0")
	addCommit(t, repo, dir, "fix: c2", "v2")

	info := branch.Info{Major: 1, Minor: 2, TagPrefix: "v"}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.2.0" || patch != 0 {
		t.Errorf("annotated tag: got %q patch=%d, want v1.2.0", tag, patch)
	}
}

// ── CommitsSince ──────────────────────────────────────────────────────────────

func TestCommitsSince(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: baseline", "v0")
	addTag(t, repo, "v1.2.0")
	addCommit(t, repo, dir, "fix: bug fix", "v1")
	addCommit(t, repo, dir, "feat: new feature", "v2")

	msgs, err := CommitsSince(repo, "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 commits since v1.2.0, got %d: %v", len(msgs), msgs)
	}
	// Log is reverse-chronological: newest first
	if !strings.Contains(msgs[0], "feat: new feature") {
		t.Errorf("msgs[0] = %q, want feat: new feature", msgs[0])
	}
	if !strings.Contains(msgs[1], "fix: bug fix") {
		t.Errorf("msgs[1] = %q, want fix: bug fix", msgs[1])
	}
}

func TestCommitsSinceTagOnHead(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: something", "v1")
	addTag(t, repo, "v1.2.0")

	// HEAD IS the tag — no commits since
	msgs, err := CommitsSince(repo, "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 commits since tag on HEAD, got %d", len(msgs))
	}
}

// ── AllCommits ────────────────────────────────────────────────────────────────

func TestAllCommits(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addCommit(t, repo, dir, "fix: c2", "v2")
	addCommit(t, repo, dir, "fix: c3", "v3")

	msgs, err := AllCommits(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 commits, got %d", len(msgs))
	}
}

// ── CommitFile ────────────────────────────────────────────────────────────────

func TestCommitFile(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: initial", "init")

	pomPath := filepath.Join(dir, "pom.xml")
	if err := os.WriteFile(pomPath, []byte("<project><version>1.2.4</version></project>"), 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := CommitFile(repo, "pom.xml", "chore(release): v1.2.4 [skip ci]", "Releaser", "ci@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if hash.IsZero() {
		t.Error("expected non-zero commit hash")
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		t.Fatal(err)
	}
	if commit.Message != "chore(release): v1.2.4 [skip ci]" {
		t.Errorf("commit message = %q", commit.Message)
	}
	if commit.Author.Name != "Releaser" {
		t.Errorf("author name = %q, want Releaser", commit.Author.Name)
	}
	if commit.Author.Email != "ci@example.com" {
		t.Errorf("author email = %q", commit.Author.Email)
	}
}

// ── CreateTag ─────────────────────────────────────────────────────────────────

func TestCreateTag(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: something", "v1")

	if err := CreateTag(repo, "v1.2.0"); err != nil {
		t.Fatal(err)
	}

	ref, err := repo.Tag("v1.2.0")
	if err != nil {
		t.Fatalf("tag v1.2.0 not found after creation: %v", err)
	}
	head, _ := repo.Head()
	if ref.Hash() != head.Hash() {
		t.Error("lightweight tag does not point to HEAD")
	}
}

func TestCreateTagDuplicate(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: something", "v1")
	addTag(t, repo, "v1.2.0")

	if err := CreateTag(repo, "v1.2.0"); err == nil {
		t.Error("expected error when creating duplicate tag")
	}
}

// ── Push (error path only — no real remote needed) ───────────────────────────

func TestPushRemoteFails(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: something", "v1")
	addTag(t, repo, "v1.2.0")

	// Remote config exists but points nowhere → Push will error on remote.Push
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"/nonexistent/bare/repo"},
	}); err != nil {
		t.Fatal(err)
	}

	err := Push(repo, "master", "v1.2.0", "")
	if err == nil {
		t.Error("expected push error for invalid remote path")
	}
}

func TestPushNoRemote(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: something", "v1")

	err := Push(repo, "master", "v1.0.0", "")
	if err == nil {
		t.Error("expected error when no remote is configured")
	}
}

// ── bare-repo error paths ─────────────────────────────────────────────────────

func TestIsWorkingTreeCleanBareRepo(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, true) // bare = true
	if err != nil {
		t.Fatal(err)
	}
	_, err = IsWorkingTreeClean(repo)
	if err == nil {
		t.Error("expected error for bare repo (no worktree)")
	}
}

func TestCurrentBranchEmptyRepo(t *testing.T) {
	// Fresh repo with no commits — HEAD symbolic ref points to refs/heads/master
	// but that ref doesn't exist yet, so Head() returns plumbing.ErrReferenceNotFound.
	repo, _ := newTestRepo(t)
	_, err := CurrentBranch(repo)
	if err == nil {
		t.Error("expected error for empty repo (no HEAD commit)")
	}
}

func TestAllCommitsEmptyRepo(t *testing.T) {
	repo, _ := newTestRepo(t)
	_, err := AllCommits(repo)
	if err == nil {
		t.Error("expected error for empty repo")
	}
}

func TestCreateTagEmptyRepo(t *testing.T) {
	repo, _ := newTestRepo(t)
	err := CreateTag(repo, "v1.0.0")
	if err == nil {
		t.Error("expected error when creating tag on empty repo")
	}
}

func TestLatestTagEmptyRepo(t *testing.T) {
	repo, _ := newTestRepo(t)
	_, _, err := LatestTag(repo, branch.Info{Major: 1, Minor: 2, TagPrefix: "v"})
	if err == nil {
		t.Error("expected error for empty repo")
	}
}

func TestCommitFileBareRepo(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = CommitFile(repo, "pom.xml", "chore: test", "Test", "t@t.com")
	if err == nil {
		t.Error("expected error for bare repo (no worktree)")
	}
}

func TestCommitFileNonexistentFile(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "init")

	// Try to stage a file that doesn't exist in the worktree
	_, err := CommitFile(repo, "nonexistent.xml", "chore: bad", "Test", "t@t.com")
	if err == nil {
		t.Error("expected error when staging nonexistent file")
	}
}

// ── LatestTag edge cases ──────────────────────────────────────────────────────

func TestLatestTagOnHead(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.2.0") // tag is ON HEAD, not behind HEAD

	info := branch.Info{Major: 1, Minor: 2, TagPrefix: "v"}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatal(err)
	}
	// Tag at HEAD should still be found (it is the current release base)
	if tag != "v1.2.0" || patch != 0 {
		t.Errorf("got %q patch=%d, want v1.2.0 patch=0 when tag is on HEAD", tag, patch)
	}
}

func TestLatestTagNonAncestorIgnored(t *testing.T) {
	repo, dir := newTestRepo(t)
	baseHash := addCommit(t, repo, dir, "chore: base", "base")

	// Create sibling branch from base commit and put a v1.2.0 tag on it
	w, _ := repo.Worktree()
	if err := w.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("sibling"),
		Hash:   baseHash,
		Create: true,
	}); err != nil {
		t.Fatal(err)
	}
	addCommit(t, repo, dir, "feat: sibling work", "sibling")
	addTag(t, repo, "v1.2.0")

	// Switch back to master and add a commit (diverges from sibling)
	if err := w.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	}); err != nil {
		t.Fatal(err)
	}
	addCommit(t, repo, dir, "fix: mainline only", "mainline")

	info := branch.Info{Major: 1, Minor: 2, TagPrefix: "v"}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatal(err)
	}
	// v1.2.0 is on the sibling branch — not an ancestor of current HEAD
	if tag != "" || patch != -1 {
		t.Errorf("non-ancestor tag should be ignored: got %q patch=%d", tag, patch)
	}
}

func TestLatestTagSkipsMalformedRef(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")

	// Create a v1.2.0 tag reference pointing to a hash that doesn't exist.
	// LatestTag must silently skip it instead of returning an error.
	fakeHash := plumbing.NewHash("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	fakeRef := plumbing.NewHashReference(plumbing.NewTagReferenceName("v1.2.0"), fakeHash)
	if err := repo.Storer.SetReference(fakeRef); err != nil {
		t.Fatal(err)
	}

	info := branch.Info{Major: 1, Minor: 2, TagPrefix: "v"}
	tag, patch, err := LatestTag(repo, info)
	if err != nil {
		t.Fatalf("LatestTag should not error on malformed tag ref: %v", err)
	}
	if tag != "" || patch != -1 {
		t.Errorf("malformed tag should be skipped: got %q patch=%d", tag, patch)
	}
}

// ── resolveTagToCommit default case ──────────────────────────────────────────

func TestResolveTagToCommitBlobRef(t *testing.T) {
	repo, dir := newTestRepo(t)
	h := addCommit(t, repo, dir, "fix: c1", "content")

	// Obtain a blob hash from the commit tree to use as an adversarial ref target.
	commit, err := repo.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := commit.Tree()
	if err != nil || len(tree.Entries) == 0 {
		t.Fatal("need at least one tree entry")
	}
	blobHash := tree.Entries[0].Hash

	blobRef := plumbing.NewHashReference(plumbing.NewTagReferenceName("blob-tag"), blobHash)
	_, err = resolveTagToCommit(repo, blobRef)
	if err == nil {
		t.Error("expected error when tag points to a blob (not a commit or tag object)")
	}
}

// ── CommitsSince error paths ──────────────────────────────────────────────────

func TestCommitsSinceBadTag(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: something", "v1")

	_, err := CommitsSince(repo, "v-does-not-exist")
	if err == nil {
		t.Error("expected error for nonexistent tag")
	}
}

func TestCommitsSinceMalformedTagRef(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")

	// Create a tag that references a nonexistent hash so resolveTagToCommit fails.
	fakeHash := plumbing.NewHash("cafebabecafebabecafebabecafebabecafebabe")
	fakeRef := plumbing.NewHashReference(plumbing.NewTagReferenceName("v1.2.0"), fakeHash)
	if err := repo.Storer.SetReference(fakeRef); err != nil {
		t.Fatal(err)
	}

	_, err := CommitsSince(repo, "v1.2.0")
	if err == nil {
		t.Error("expected error when resolveTagToCommit fails on a malformed tag ref")
	}
}

// ── AuthorFromConfig ──────────────────────────────────────────────────────────

func TestAuthorFromConfigDoesNotPanic(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "init")

	// May return empty strings in CI where ~/.gitconfig has no user section — must not panic
	name, email := AuthorFromConfig(repo)
	_ = name
	_ = email
}

func TestAuthorFromConfigLocalOverride(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "init")

	// Write a local git config with user.name / user.email
	localCfg := `[user]
	name = LocalUser
	email = local@example.com
`
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(localCfg), 0644); err != nil {
		t.Fatal(err)
	}

	// Reload repo so it picks up the config file we just wrote
	repo2, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	name, email := AuthorFromConfig(repo2)
	if name != "LocalUser" {
		t.Errorf("name = %q, want LocalUser", name)
	}
	if email != "local@example.com" {
		t.Errorf("email = %q, want local@example.com", email)
	}
}

// ── Push with a real bare remote ──────────────────────────────────────────────

func TestPushWithBareRemote(t *testing.T) {
	// Create a bare repo to act as the remote
	remoteDir := t.TempDir()
	if _, err := gogit.PlainInit(remoteDir, true); err != nil {
		t.Fatal(err)
	}

	// Create the working repo
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: something", "v1")
	addTag(t, repo, "v1.2.0")

	// Wire the bare repo as origin
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	}); err != nil {
		t.Fatal(err)
	}

	// Push with a token (exercises the auth branch even though local transport ignores it)
	err := Push(repo, "master", "v1.2.0", "dummy-token")
	if err != nil {
		t.Fatalf("Push to bare remote failed: %v", err)
	}

	// Push again — should hit the NoErrAlreadyUpToDate branch and return nil
	err = Push(repo, "master", "v1.2.0", "dummy-token")
	if err != nil {
		t.Fatalf("second Push returned unexpected error: %v", err)
	}
}

func TestPushDetachedHeadUpdatesRemoteBranch(t *testing.T) {
	// Regression: in a CI checkout HEAD is detached and refs/heads/<branch>
	// does not exist locally — go-git silently skipped the branch refspec and
	// pushed only the tag. The push must carry HEAD's commit to the branch.
	remoteDir := t.TempDir()
	if _, err := gogit.PlainInit(remoteDir, true); err != nil {
		t.Fatal(err)
	}

	repo, dir := newTestRepo(t)
	hash := addCommit(t, repo, dir, "fix: something", "v1")
	addTag(t, repo, "v1.2.1")
	detachHead(t, repo)
	if err := repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("master")); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	}); err != nil {
		t.Fatal(err)
	}

	if err := Push(repo, "release/1.2", "v1.2.1", "dummy-token"); err != nil {
		t.Fatalf("Push from detached HEAD failed: %v", err)
	}

	bare, err := gogit.PlainOpen(remoteDir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := bare.Reference(plumbing.NewBranchReferenceName("release/1.2"), false)
	if err != nil {
		t.Fatalf("remote branch ref missing after push: %v", err)
	}
	if ref.Hash() != hash {
		t.Errorf("remote branch at %s, want HEAD commit %s", ref.Hash(), hash)
	}
	if _, err := bare.Reference(plumbing.NewTagReferenceName("v1.2.1"), false); err != nil {
		t.Errorf("remote tag ref missing after push: %v", err)
	}
}

func TestPushWithGoGitUnbornHead(t *testing.T) {
	// Repo with a remote but no commits: pushRefSpecs fails reading HEAD.
	repo, _ := newTestRepo(t)
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{t.TempDir()},
	}); err != nil {
		t.Fatal(err)
	}
	if err := pushWithGoGit(repo, "master", "v1.0.0", "some-token"); err == nil {
		t.Error("expected error for unborn HEAD")
	}
}

// ── IsWorkingTreeClean: w.Status() error path ────────────────────────────────

func TestIsWorkingTreeCleanCorruptIndex(t *testing.T) {
	// Use a filesystem repo so we can corrupt the on-disk index.
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "initial")

	// Overwrite .git/index with garbage so go-git fails to parse it.
	indexPath := filepath.Join(dir, ".git", "index")
	if err := os.WriteFile(indexPath, []byte("not a valid git index"), 0644); err != nil {
		t.Fatal(err)
	}

	// Reopen — fresh repository object with no cached index.
	repo2, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = IsWorkingTreeClean(repo2)
	if err == nil {
		t.Error("expected error when git index is corrupt")
	}
}

// ── LatestTag: head commit object missing ────────────────────────────────────

func TestLatestTagHeadCommitMissing(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.2.0")

	// Detach HEAD to a fake hash that has no backing commit object.
	fakeHash := plumbing.NewHash("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, fakeHash)); err != nil {
		t.Fatal(err)
	}

	_, _, err := LatestTag(repo, branch.Info{Major: 1, Minor: 2, TagPrefix: "v"})
	if err == nil {
		t.Error("expected error when HEAD commit object is missing")
	}
}

// ── LatestTag: Tags() iterator fails ────────────────────────────────────────

func TestLatestTagTagsIterFails(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")

	if os.Getuid() == 0 {
		t.Skip("skipping: chmod restrictions do not apply when running as root")
	}

	// Make .git/refs/tags/ unreadable so that go-git's walkReferencesTree
	// returns EPERM when it tries to list the directory, triggering the
	// Tags() error path.
	tagsDir := filepath.Join(dir, ".git", "refs", "tags")
	os.Chmod(tagsDir, 0000)
	t.Cleanup(func() { os.Chmod(tagsDir, 0755) })

	// Reopen so the filesystem storer holds no cached state.
	repo2, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}

	_, _, err = LatestTag(repo2, branch.Info{Major: 1, Minor: 2, TagPrefix: "v"})
	if err == nil {
		t.Error("expected error when refs/tags is unreadable")
	}
}

// ── LatestTag: IsAncestor fails → ForEach propagates error ───────────────────

func TestLatestTagIsAncestorFails(t *testing.T) {
	// Topology: c0 (base) → c1 (sibling branch, tagged v1.2.0)
	//                      → c2 (master HEAD — diverged from sibling)
	// The tag is NOT an ancestor of HEAD. IsAncestor must walk master's history
	// all the way back to c0; corrupting c0 makes that walk fail.
	repo, dir := newTestRepo(t)
	c0 := addCommit(t, repo, dir, "chore: base", "base")

	w, _ := repo.Worktree()
	if err := w.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("sibling"),
		Hash:   c0,
		Create: true,
	}); err != nil {
		t.Fatal(err)
	}
	addCommit(t, repo, dir, "feat: sibling work", "sibling")
	addTag(t, repo, "v1.2.0") // tag on the sibling commit (not an ancestor of master)

	if err := w.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	}); err != nil {
		t.Fatal(err)
	}
	addCommit(t, repo, dir, "fix: mainline", "mainline") // HEAD on master

	// Corrupt c0 (the common base) so that IsAncestor's commit-graph walk
	// fails when it tries to read c0 as a parent of the master HEAD commit.
	hashStr := c0.String()
	objPath := filepath.Join(dir, ".git", "objects", hashStr[:2], hashStr[2:])
	if err := os.Chmod(objPath, 0644); err != nil {
		t.Fatalf("chmod object: %v", err)
	}
	if err := os.WriteFile(objPath, []byte("corrupt"), 0444); err != nil {
		t.Fatal(err)
	}

	repo2, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	// The ForEach callback propagates the IsAncestor error, so LatestTag
	// must return a non-nil error (covers the refs.ForEach error path).
	_, _, err = LatestTag(repo2, branch.Info{Major: 1, Minor: 2, TagPrefix: "v"})
	if err == nil {
		t.Error("expected error when commit graph is corrupt during IsAncestor")
	}
}

// ── CommitsSince: repo.Head() fails after tag resolve ────────────────────────

func TestCommitsSinceHeadRemoved(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.2.0")

	// Remove HEAD so repo.Head() returns ErrReferenceNotFound.
	if err := repo.Storer.RemoveReference(plumbing.HEAD); err != nil {
		t.Fatal(err)
	}

	_, err := CommitsSince(repo, "v1.2.0")
	if err == nil {
		t.Error("expected error when HEAD reference is missing")
	}
}

// ── CommitsSince: repo.Log() fails ───────────────────────────────────────────

func TestCommitsSinceFakeHead(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.2.0")
	addCommit(t, repo, dir, "fix: c2", "v2")

	// Point HEAD directly to a non-existent commit hash.
	// repo.Head() succeeds (returns the hash) but repo.Log() fails eagerly.
	fakeHash := plumbing.NewHash("cafebabecafebabecafebabecafebabecafebabe")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, fakeHash)); err != nil {
		t.Fatal(err)
	}

	_, err := CommitsSince(repo, "v1.2.0")
	if err == nil {
		t.Error("expected error when HEAD commit object is missing")
	}
}

// ── AllCommits: repo.Log() fails ─────────────────────────────────────────────

func TestAllCommitsFakeHead(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")

	// Point HEAD to a non-existent commit hash so repo.Log() fails eagerly.
	fakeHash := plumbing.NewHash("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, fakeHash)); err != nil {
		t.Fatal(err)
	}

	_, err := AllCommits(repo)
	if err == nil {
		t.Error("expected error when HEAD commit object is missing")
	}
}

// ── CommitFiles: w.Commit() fails ────────────────────────────────────────────

func TestCommitFilesUnchanged(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "initial")

	// test.txt already committed and unchanged — w.Add succeeds, w.Commit fails
	// (go-git rejects empty commits when AllowEmptyCommits is false).
	_, err := CommitFiles(repo, []string{"test.txt"}, "chore: empty", "Test", "t@t.com")
	if err == nil {
		t.Error("expected error when committing unchanged file (empty commit)")
	}
}

// ── Push: SSH agent success / failure paths ──────────────────────────────────

func TestPushSSHAgentSucceeds(t *testing.T) {
	// Mock sshPush so it succeeds without a real SSH agent.
	orig := sshPush
	sshPush = func(_ *gogit.Repository, _, _ string) error { return nil }
	defer func() { sshPush = orig }()

	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"git@example.com:owner/repo.git"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := Push(repo, "master", "v1.0.0", ""); err != nil {
		t.Fatalf("Push with mocked SSH agent should succeed: %v", err)
	}
}

func TestPushSSHAgentFailsFallsBackToCLI(t *testing.T) {
	// SSH URL remote + sshPush fails → falls through to pushWithCLI.
	orig := sshPush
	sshPush = func(_ *gogit.Repository, _, _ string) error { return fmt.Errorf("no agent") }
	defer func() { sshPush = orig }()

	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"git@example.com:owner/repo.git"},
	}); err != nil {
		t.Fatal(err)
	}

	// CLI push will fail (no real remote) — we just verify it ran at all.
	err := Push(repo, "master", "v1.0.0", "")
	if err == nil {
		t.Error("expected error after SSH fallback to CLI with unreachable remote")
	}
}

// ── pushWithSSHAgent internals ────────────────────────────────────────────────

func TestPushWithSSHAgentAuthFails(t *testing.T) {
	orig := newSSHAgentAuth
	newSSHAgentAuth = func(_ string) (*gitssh.PublicKeysCallback, error) {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}
	defer func() { newSSHAgentAuth = orig }()

	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")

	err := pushWithSSHAgent(repo, "master", "v1.0.0")
	if err == nil {
		t.Error("expected error when SSH agent auth fails")
	}
}

func TestPushWithSSHAgentNoRemote(t *testing.T) {
	orig := newSSHAgentAuth
	newSSHAgentAuth = func(user string) (*gitssh.PublicKeysCallback, error) {
		return &gitssh.PublicKeysCallback{User: user}, nil
	}
	defer func() { newSSHAgentAuth = orig }()

	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	// No remote configured → repo.Remote("origin") fails.

	err := pushWithSSHAgent(repo, "master", "v1.0.0")
	if err == nil {
		t.Error("expected error when no origin remote is configured")
	}
}

func TestPushWithSSHAgentPushFails(t *testing.T) {
	orig := newSSHAgentAuth
	newSSHAgentAuth = func(user string) (*gitssh.PublicKeysCallback, error) {
		return &gitssh.PublicKeysCallback{User: user}, nil
	}
	defer func() { newSSHAgentAuth = orig }()

	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.0.0")
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"/nonexistent/bare/repo"},
	}); err != nil {
		t.Fatal(err)
	}

	err := pushWithSSHAgent(repo, "master", "v1.0.0")
	if err == nil {
		t.Error("expected error when remote push fails")
	}
}

func TestPushWithSSHAgentSuccess(t *testing.T) {
	remoteDir := t.TempDir()
	if _, err := gogit.PlainInit(remoteDir, true); err != nil {
		t.Fatal(err)
	}

	orig := newSSHAgentAuth
	newSSHAgentAuth = func(user string) (*gitssh.PublicKeysCallback, error) {
		return &gitssh.PublicKeysCallback{User: user}, nil
	}
	defer func() { newSSHAgentAuth = orig }()

	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.0.0")
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	}); err != nil {
		t.Fatal(err)
	}

	// Local transport ignores auth — push succeeds regardless of the mock callback.
	if err := pushWithSSHAgent(repo, "master", "v1.0.0"); err != nil {
		t.Fatalf("expected success pushing to local bare remote: %v", err)
	}
}

// ── pushWithGoGit error paths ─────────────────────────────────────────────────

func TestPushWithGoGitNoRemote(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	// No remote → repo.Remote("origin") fails inside pushWithGoGit.
	err := Push(repo, "master", "v1.0.0", "some-token")
	if err == nil {
		t.Error("expected error when no origin remote is configured")
	}
}

func TestPushWithSSHAgentUnbornHead(t *testing.T) {
	orig := newSSHAgentAuth
	newSSHAgentAuth = func(user string) (*gitssh.PublicKeysCallback, error) {
		return &gitssh.PublicKeysCallback{User: user}, nil
	}
	defer func() { newSSHAgentAuth = orig }()

	// Repo with a remote but no commits: pushRefSpecs fails reading HEAD.
	repo, _ := newTestRepo(t)
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{t.TempDir()},
	}); err != nil {
		t.Fatal(err)
	}
	if err := pushWithSSHAgent(repo, "master", "v1.0.0"); err == nil {
		t.Error("expected error for unborn HEAD")
	}
}

func TestPushWithGoGitPushFails(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "fix: c1", "v1")
	addTag(t, repo, "v1.0.0")
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"/nonexistent/bare/repo"},
	}); err != nil {
		t.Fatal(err)
	}
	err := Push(repo, "master", "v1.0.0", "some-token")
	if err == nil {
		t.Error("expected error when remote push fails")
	}
}

// ── pushWithCLI: bare repo → no worktree ────────────────────────────────────

func TestPushWithCLIBareRepo(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	// No token, no SSH URL → goes to pushWithCLI → Worktree() fails for bare repo.
	err = Push(repo, "master", "v1.0.0", "")
	if err == nil {
		t.Error("expected error for bare repo (no worktree)")
	}
}

func TestPushWithCLISuccess(t *testing.T) {
	// Non-bare repo + local bare remote + no token + no SSH URL → pushWithCLI → success.
	repo, dir := newTestRepo(t)
	sig := testSig()

	wt, _ := repo.Worktree()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	wt.Add("f.txt")
	hash, err := wt.Commit("init", &gogit.CommitOptions{Author: sig})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTag("v1.0.0", hash, nil); err != nil {
		t.Fatal(err)
	}

	remoteDir := t.TempDir()
	if _, err := gogit.PlainInit(remoteDir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	}); err != nil {
		t.Fatal(err)
	}

	// Detect default branch name (go-git uses "master" but git config may differ).
	head, _ := repo.Head()
	branchName := head.Name().Short()

	if err := Push(repo, branchName, "v1.0.0", ""); err != nil {
		t.Fatalf("pushWithCLI success: %v", err)
	}
}
