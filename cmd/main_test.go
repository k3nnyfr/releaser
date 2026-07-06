package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func testSig() *object.Signature {
	return &object.Signature{Name: "CI", Email: "ci@example.com", When: time.Now()}
}

// setupRepo creates a temporary git repo with pom.xml committed.
func setupRepo(t *testing.T) (repo *gogit.Repository, dir string) {
	t.Helper()
	dir = t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	writePom(t, dir, "1.2.3")
	commitAll(t, repo, dir, "chore: init")
	return repo, dir
}

// setupRepoWithRemote adds a bare remote so Push can succeed.
func setupRepoWithRemote(t *testing.T) (repo *gogit.Repository, dir string) {
	t.Helper()
	repo, dir = setupRepo(t)

	remoteDir := t.TempDir()
	if _, err := gogit.PlainInit(remoteDir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	}); err != nil {
		t.Fatal(err)
	}
	return repo, dir
}

func writePom(t *testing.T, dir, version string) {
	t.Helper()
	pom := `<?xml version="1.0"?>
<project>
  <version>` + version + `</version>
</project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, repo *gogit.Repository, dir, msg string) plumbing.Hash {
	t.Helper()
	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	// stage everything
	if _, err := w.Add("."); err != nil {
		t.Fatal(err)
	}
	h, err := w.Commit(msg, &gogit.CommitOptions{Author: testSig()})
	if err != nil {
		t.Fatalf("commit %q: %v", msg, err)
	}
	return h
}

func addFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// ── newRootCmd integration tests ──────────────────────────────────────────────

func execCmd(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunDryRun(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "src.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("src.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("dry-run: unexpected error: %v", err)
	}
}

func TestRunNothingToRelease(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "src.go", "chore")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("src.go")
	w.Commit("chore: update deps", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir)
	if !errors.Is(err, errNothingToRelease) {
		t.Fatalf("expected errNothingToRelease, got %v", err)
	}
}

func TestRunNotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
}

func TestRunBadBranchName(t *testing.T) {
	_, dir := setupRepo(t)
	err := execCmd(t, "--dry-run", "--branch", "main", "--repo", dir)
	if err == nil {
		t.Fatal("expected error: 'main' doesn't match release pattern")
	}
}

func TestRunDetachedHead(t *testing.T) {
	repo, dir := setupRepo(t)
	// Detach HEAD
	head, _ := repo.Head()
	repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, head.Hash()))

	err := execCmd(t, "--dry-run", "--repo", dir) // no --branch
	if err == nil {
		t.Fatal("expected error for detached HEAD without --branch")
	}
}

func TestRunBadConfig(t *testing.T) {
	_, dir := setupRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".releaser.yml"), []byte("{[invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error for invalid .releaser.yml")
	}
}

func TestRunDirtyTree(t *testing.T) {
	_, dir := setupRepo(t)
	// Modify a tracked file without staging
	os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project><version>dirty</version></project>"), 0644)

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error for dirty working tree")
	}
}

func TestRunFlagOverrides(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "src.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("src.go")
	w.Commit("fix: something", &gogit.CommitOptions{Author: testSig()})

	// --tag-prefix "" --branch-pattern with defaults via flags
	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir,
		"--tag-prefix", "rel-", "--branch-pattern", `^(?:.*/)?release/(\d+)\.(\d+)$`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPomOverride(t *testing.T) {
	_, dir := setupRepo(t)
	// Write a second pom at a custom path
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	writePom(t, filepath.Join(dir, "sub"), "1.2.3")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("sub/pom.xml")
	w.Commit("chore: add sub pom", &gogit.CommitOptions{Author: testSig()})

	addFile(t, dir, "x.go", "// fix")
	w.Add("x.go")
	w.Commit("fix: patch", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir, "--pom", "sub/pom.xml")
	if err != nil {
		t.Fatalf("--pom override: unexpected error: %v", err)
	}
}

func TestRunMissingPom(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch", &gogit.CommitOptions{Author: testSig()})

	// Use --pom to point to a non-existent file; keeps the working tree clean.
	err := execCmd(t, "--branch", "release/1.2", "--repo", dir, "--pom", "nonexistent.xml")
	if err == nil {
		t.Fatal("expected error for missing pom.xml")
	}
}

func TestRunPomNoVersion(t *testing.T) {
	_, dir := setupRepo(t)
	// Write a pom with no <version> tag as an untracked file (tree stays clean)
	os.WriteFile(filepath.Join(dir, "no-version.xml"), []byte("<project></project>"), 0644)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir, "--pom", "no-version.xml")
	if err == nil {
		t.Fatal("expected error when pom has no <version>")
	}
}

func TestRunPomReadOnly(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	// Make pom.xml read-only: ReadVersion succeeds (readable), WriteVersion fails (not writable)
	os.Chmod(filepath.Join(dir, "pom.xml"), 0444)
	defer os.Chmod(filepath.Join(dir, "pom.xml"), 0644)

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when pom.xml is read-only")
	}
}

func TestRunAuthorFromConfig(t *testing.T) {
	_, dir := setupRepo(t)
	// Write .releaser.yml as untracked so the working tree stays clean
	cfg := "git:\n  author_name: ReleaserBot\n  author_email: bot@example.com\n"
	os.WriteFile(filepath.Join(dir, ".releaser.yml"), []byte(cfg), 0644)

	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("run with author_name/author_email config: %v", err)
	}
}

func TestRunNoPush(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("--no-push: unexpected error: %v", err)
	}

	// Verify pom.xml was updated
	data, _ := os.ReadFile(filepath.Join(dir, "pom.xml"))
	if string(data) == "" {
		t.Error("pom.xml should have been updated")
	}
}

func TestRunNoCommit(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--no-commit", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("--no-commit: unexpected error: %v", err)
	}
}

func TestRunTagOnly(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--tag-only", "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("--tag-only --no-push: unexpected error: %v", err)
	}
}

func TestRunDuplicateTag(t *testing.T) {
	_, dir := setupRepo(t)
	repo, _ := gogit.PlainOpen(dir)

	// Add a fix commit so there are releasable changes
	addFile(t, dir, "x.go", "// fix")
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	// Pre-create a v1.2.0 ref pointing to a garbage hash.
	// LatestTag skips it (resolveTagToCommit fails for garbage hash),
	// so run() calculates "v1.2.0" as the first-ever version — then
	// CreateTag("v1.2.0") fails because the ref already exists.
	fakeRef := plumbing.NewHashReference(
		plumbing.NewTagReferenceName("v1.2.0"),
		plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	)
	if err := repo.Storer.SetReference(fakeRef); err != nil {
		t.Fatal(err)
	}

	err := execCmd(t, "--tag-only", "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error: v1.2.0 ref already exists")
	}
}

func TestRunPushFails(t *testing.T) {
	_, dir := setupRepo(t)
	repo, _ := gogit.PlainOpen(dir)
	// Remote exists but points to a non-existent directory → push fails
	repo.CreateRemote(&gitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{"/nonexistent/path/to/bare/repo"},
	})

	addFile(t, dir, "x.go", "// fix")
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected push error for invalid remote URL")
	}
}

func TestRunGitLabError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Tag already has a release"}`))
	}))
	defer srv.Close()

	_, dir := setupRepoWithRemote(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w2, _ := repo.Worktree()
	w2.Add("x.go")
	w2.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	t.Setenv("CI_SERVER_URL", srv.URL)
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "test-token")

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when GitLab API returns 422")
	}
}

func TestRunWithPreviousTag(t *testing.T) {
	repo, dir := setupRepo(t)

	// Tag the initial commit as v1.2.0 (simulates a prior release)
	initialHead, _ := repo.Head()
	repo.CreateTag("v1.2.0", initialHead.Hash(), nil)

	// Fix commit after the tag — run() will use CommitsSince, not AllCommits
	addFile(t, dir, "x.go", "// fix")
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("run with previous tag: %v", err)
	}
}

func TestRunSkipGitLab(t *testing.T) {
	_, dir := setupRepoWithRemote(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	// No GITLAB_TOKEN / URL set → GitLab release is skipped
	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("expected skip-gitlab success, got: %v", err)
	}
}

func TestRunMissingToken(t *testing.T) {
	_, dir := setupRepoWithRemote(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	t.Setenv("CI_SERVER_URL", "https://gitlab.example.com")
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "")

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when GITLAB_TOKEN is not set but GitLab URL is configured")
	}
}

func TestRunWithGitLab(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, dir := setupRepoWithRemote(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w2, _ := repo.Worktree()
	w2.Add("x.go")
	w2.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	t.Setenv("CI_SERVER_URL", srv.URL)
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "test-token")

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("full release with mock GitLab: unexpected error: %v", err)
	}
}

// ── main() tests via exitFn ───────────────────────────────────────────────────

func TestMainSuccess(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: something", &gogit.CommitOptions{Author: testSig()})

	old := os.Args
	os.Args = []string{"releaser", "--dry-run", "--branch", "release/1.2", "--repo", dir}
	defer func() { os.Args = old }()

	called := false
	oldFn := exitFn
	exitFn = func(code int) { called = true }
	defer func() { exitFn = oldFn }()

	main()

	if called {
		t.Error("exitFn should not have been called on success")
	}
}

func TestMainNothingToRelease(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "chore")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("chore: bump deps", &gogit.CommitOptions{Author: testSig()})

	old := os.Args
	os.Args = []string{"releaser", "--branch", "release/1.2", "--repo", dir}
	defer func() { os.Args = old }()

	var gotCode int
	oldFn := exitFn
	exitFn = func(code int) { gotCode = code }
	defer func() { exitFn = oldFn }()

	main()

	if gotCode != 2 {
		t.Errorf("expected exit code 2 for nothing-to-release, got %d", gotCode)
	}
}

func TestMainError(t *testing.T) {
	old := os.Args
	os.Args = []string{"releaser", "--branch", "release/1.2", "--repo", t.TempDir()} // not a git repo
	defer func() { os.Args = old }()

	var gotCode int
	oldFn := exitFn
	exitFn = func(code int) { gotCode = code }
	defer func() { exitFn = oldFn }()

	main()

	if gotCode != 1 {
		t.Errorf("expected exit code 1 for general error, got %d", gotCode)
	}
}
