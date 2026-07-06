package gitutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

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

func TestCurrentBranchDetached(t *testing.T) {
	repo, dir := newTestRepo(t)
	addCommit(t, repo, dir, "chore: init", "initial")

	// Detach HEAD by replacing the symbolic ref with a hash ref
	head, _ := repo.Head()
	detached := plumbing.NewHashReference(plumbing.HEAD, head.Hash())
	if err := repo.Storer.SetReference(detached); err != nil {
		t.Fatal(err)
	}

	_, err := CurrentBranch(repo)
	if err == nil {
		t.Error("expected error for detached HEAD")
	}
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
