package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"git.k3nny.fr/releaser/internal/config"
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
	// --pom points to a non-existent file: pom update is skipped, tag is still created.
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--no-push", "--branch", "release/1.2", "--repo", dir, "--pom", "nonexistent.xml")
	if err != nil {
		t.Fatalf("missing pom should be skipped, got error: %v", err)
	}

	// Tag must still have been created.
	repo2, _ := gogit.PlainOpen(dir)
	_, err = repo2.Tag("1.2.0")
	if err != nil {
		t.Error("expected tag 1.2.0 to be created")
	}
}

func TestRunNoPomAtDefaultPath(t *testing.T) {
	// Repo with no pom.xml at the default path: runs without error, creates tag.
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	addFile(t, dir, "main.go", "package main")
	commitAll(t, repo, dir, "fix: initial")

	err = execCmd(t, "--no-push", "--branch", "release/2.0", "--repo", dir)
	if err != nil {
		t.Fatalf("no pom.xml should not be an error: %v", err)
	}

	repo2, _ := gogit.PlainOpen(dir)
	_, err = repo2.Tag("2.0.0")
	if err != nil {
		t.Error("expected tag 2.0.0 to be created")
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

	// Pre-create a 1.2.0 ref pointing to a garbage hash.
	// LatestTag skips it (resolveTagToCommit fails for garbage hash),
	// so run() calculates "1.2.0" as the first-ever version — then
	// CreateTag("1.2.0") fails because the ref already exists.
	fakeRef := plumbing.NewHashReference(
		plumbing.NewTagReferenceName("1.2.0"),
		plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	)
	if err := repo.Storer.SetReference(fakeRef); err != nil {
		t.Fatal(err)
	}

	err := execCmd(t, "--tag-only", "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error: 1.2.0 ref already exists")
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

	// Tag the initial commit as 1.2.0 (simulates a prior release)
	initialHead, _ := repo.Head()
	repo.CreateTag("1.2.0", initialHead.Hash(), nil)

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

func TestRunNoRelease(t *testing.T) {
	_, dir := setupRepoWithRemote(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	// --no-release skips GitLab release even when credentials are configured
	t.Setenv("CI_SERVER_URL", "https://gitlab.example.com")
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "test-token")

	err := execCmd(t, "--no-release", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("--no-release: unexpected error: %v", err)
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

func TestRunInit(t *testing.T) {
	dir := t.TempDir()
	err := execCmd(t, "--init", "--repo", dir)
	if err != nil {
		t.Fatalf("--init: unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".releaser.yml"))
	if err != nil {
		t.Fatal("expected .releaser.yml to be created")
	}
	if len(data) == 0 {
		t.Error("expected non-empty .releaser.yml")
	}
}

func TestRunInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".releaser.yml"), []byte("existing"), 0644)
	err := execCmd(t, "--init", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when .releaser.yml already exists")
	}
}

func TestRunChangelogCreated(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// feat")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("feat: add shiny feature", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatal("expected CHANGELOG.md to be created")
	}
	s := string(data)
	if !strings.Contains(s, "## [1.2.0]") {
		t.Error("expected version header in CHANGELOG")
	}
	if !strings.Contains(s, "add shiny feature") {
		t.Error("expected feat subject in CHANGELOG")
	}
}

func TestRunChangelogFile(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--no-push", "--branch", "release/1.2", "--repo", dir, "--changelog-file", "CHANGES.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "CHANGES.md")); err != nil {
		t.Error("expected CHANGES.md to be created")
	}
}

func TestRunReleaseEnv(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "release.env"))
	if err != nil {
		t.Fatal("expected release.env to be created")
	}
	content := strings.TrimSpace(string(data))
	if content != "NEXT_VERSION=1.2.0" {
		t.Errorf("release.env content = %q, want %q", content, "NEXT_VERSION=1.2.0")
	}
}

func TestRunReleaseEnvDryRun(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "release.env")); err == nil {
		t.Error("release.env must not be created in --dry-run mode")
	}
}

func TestRunVerbose(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// feat")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("feat: add new thing", &gogit.CommitOptions{Author: testSig()})

	// Capture stderr output by redirecting it temporarily.
	old := os.Stderr
	r, wPipe, _ := os.Pipe()
	os.Stderr = wPipe

	err := execCmd(t, "--dry-run", "--verbose", "--branch", "release/1.2", "--repo", dir)

	wPipe.Close()
	os.Stderr = old

	rawBytes, _ := io.ReadAll(r)
	output := string(rawBytes)

	if err != nil {
		t.Fatalf("--verbose: unexpected error: %v", err)
	}

	checks := []string{
		"▸ configuration",
		"git.tag_prefix",
		"[default]",
		"▸ branch",
		"release/1.2",
		"major=1, minor=2",
		"▸ commits",
		"feat: add new thing",
		"patch bump",
		"▸ version",
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Errorf("--verbose output missing %q\nfull output:\n%s", want, output)
		}
	}
}

// ── ui.go coverage ────────────────────────────────────────────────────────────

func TestPaintColor(t *testing.T) {
	old := useColor
	useColor = true
	defer func() { useColor = old }()

	got := paint(ansiGreen, "hello")
	if !strings.Contains(got, "hello") || !strings.Contains(got, ansiReset) || !strings.Contains(got, ansiGreen) {
		t.Errorf("paint with color = %q", got)
	}
}

func TestFmtSourceEnv(t *testing.T) {
	old := useColor
	useColor = false
	defer func() { useColor = old }()

	got := fmtSource("env: GITLAB_TOKEN")
	if got != "[env: GITLAB_TOKEN]" {
		t.Errorf("fmtSource env = %q", got)
	}
}

func TestFmtSourceFlag(t *testing.T) {
	old := useColor
	useColor = false
	defer func() { useColor = old }()

	got := fmtSource("flag: --tag-prefix")
	if got != "[flag: --tag-prefix]" {
		t.Errorf("fmtSource flag = %q", got)
	}
}

func TestFmtSourceConfigFile(t *testing.T) {
	old := useColor
	useColor = false
	defer func() { useColor = old }()

	got := fmtSource("config file")
	if got != "[config file]" {
		t.Errorf("fmtSource config file = %q", got)
	}
}

// ── buildPublisher coverage ───────────────────────────────────────────────────

func TestBuildPublisherGitHub(t *testing.T) {
	cfg := config.Config{
		GitHub: config.GitHubConfig{Token: "ghtoken", Repo: "owner/repo"},
	}
	pub, err := buildPublisher(cfg)
	if err != nil {
		t.Fatalf("buildPublisher GitHub: %v", err)
	}
	if pub == nil {
		t.Fatal("expected non-nil publisher for GitHub config")
	}
}

func TestBuildPublisherGitLabNoToken(t *testing.T) {
	cfg := config.Config{
		GitLab: config.GitLabConfig{URL: "https://gitlab.example.com", Project: "42"},
	}
	_, err := buildPublisher(cfg)
	if err == nil {
		t.Fatal("expected error when GitLab URL+Project set but token is empty")
	}
}

// ── printVerboseConfig coverage ───────────────────────────────────────────────

func TestPrintVerboseConfigDirect(t *testing.T) {
	// Use a sparse Sources map (missing keys → source == "" → hits "default" branch).
	// Also set non-empty ReleasableTypes and both tokens to cover those branches.
	cfg := config.Config{
		Git: config.GitConfig{
			ReleasableTypes: []string{"fix", "feat"},
		},
		GitLab: config.GitLabConfig{Token: "secret"},
		GitHub: config.GitHubConfig{Token: "ghsecret"},
	}
	src := config.Sources{} // empty → all lookups return ""

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printVerboseConfig(cfg, src)

	w.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)

	if !strings.Contains(string(out), "fix, feat") {
		t.Error("expected releasable types joined in output")
	}
	if !strings.Contains(string(out), "(set)") {
		t.Error("expected '(set)' for configured tokens")
	}
}

// ── initConfig coverage ───────────────────────────────────────────────────────

func TestInitConfigWriteFails(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755)

	err := initConfig(dir)
	if err == nil {
		t.Fatal("expected error writing .releaser.yml to read-only directory")
	}
}

// ── run() injectable error paths ─────────────────────────────────────────────

func TestRunVerboseInit(t *testing.T) {
	dir := t.TempDir()
	err := execCmd(t, "--init", "--verbose", "--repo", dir)
	if err != nil {
		t.Fatalf("--init --verbose: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".releaser.yml")); err != nil {
		t.Error("expected .releaser.yml to be created")
	}
}

func TestRunAbsPathError(t *testing.T) {
	old := absPath
	absPath = func(string) (string, error) { return "", fmt.Errorf("injected abs error") }
	defer func() { absPath = old }()

	err := execCmd(t, "--repo", ".")
	if err == nil {
		t.Fatal("expected error when filepath.Abs fails")
	}
}

func TestRunWorkingTreeCheckFails(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	// Corrupt the git index so IsWorkingTreeClean fails.
	os.WriteFile(filepath.Join(dir, ".git", "index"), []byte("garbage"), 0644)

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error for corrupt git index")
	}
}

func TestRunLatestTagFails(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	tagsDir := filepath.Join(dir, ".git", "refs", "tags")
	os.Chmod(tagsDir, 0000)
	defer os.Chmod(tagsDir, 0755)

	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error for unreadable tags directory")
	}
}

func TestRunAllCommitsError(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	old := gitAllCommits
	gitAllCommits = func(_ *gogit.Repository) ([]string, error) {
		return nil, fmt.Errorf("injected AllCommits error")
	}
	defer func() { gitAllCommits = old }()

	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error from AllCommits")
	}
}

func TestRunCommitsSinceError(t *testing.T) {
	repo, dir := setupRepo(t)
	head, _ := repo.Head()
	repo.CreateTag("1.2.0", head.Hash(), nil)

	addFile(t, dir, "x.go", "// fix")
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	old := gitCommitsSince
	gitCommitsSince = func(_ *gogit.Repository, _ string) ([]string, error) {
		return nil, fmt.Errorf("injected CommitsSince error")
	}
	defer func() { gitCommitsSince = old }()

	err := execCmd(t, "--dry-run", "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error from CommitsSince")
	}
}

// ── verbose commit section coverage ──────────────────────────────────────────

func TestRunVerboseBreakingAndFix(t *testing.T) {
	// Covers: verbose "since: lastTag", message truncation (>70 chars),
	// TypeBreaking color (ansiRed+ansiBold), TypeFix color (ansiGreen).
	repo, dir := setupRepo(t)
	head, _ := repo.Head()
	repo.CreateTag("1.2.0", head.Hash(), nil)

	w, _ := repo.Worktree()

	addFile(t, dir, "a.go", "a")
	w.Add("a.go")
	w.Commit("feat!: redesign the entire public API surface which is a very long commit message header", &gogit.CommitOptions{Author: testSig()})

	addFile(t, dir, "b.go", "b")
	w.Add("b.go")
	w.Commit("fix: correct null pointer in edge case handler", &gogit.CommitOptions{Author: testSig()})

	old := os.Stderr
	r, wp, _ := os.Pipe()
	os.Stderr = wp

	err := execCmd(t, "--dry-run", "--verbose", "--branch", "release/1.2", "--repo", dir)

	wp.Close()
	os.Stderr = old
	io.ReadAll(r)

	if err != nil {
		t.Fatalf("verbose breaking+fix: unexpected error: %v", err)
	}
}

// ── release.env write error ───────────────────────────────────────────────────

func TestRunReleaseEnvWriteFails(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	// Empty directory is invisible to git — dirty check passes.
	os.Mkdir(filepath.Join(dir, "release.env"), 0755)

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when release.env is a directory")
	}
}

// ── pom stat error ────────────────────────────────────────────────────────────

func TestRunPomStatError(t *testing.T) {
	// A null byte in the path makes os.Stat return EINVAL (not ErrNotExist),
	// so hasPom=true and the stat error is propagated.
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir, "--pom", "pom\x00.xml")
	if err == nil {
		t.Fatal("expected error for pom path with null byte (EINVAL)")
	}
}

// ── changelog update error ────────────────────────────────────────────────────

func TestRunChangelogUpdateFails(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	// Empty directory is invisible to git — dirty check passes.
	// changelog.Update will fail trying to ReadFile on a directory.
	os.Mkdir(filepath.Join(dir, "CHANGELOG.md"), 0755)

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when CHANGELOG.md is a directory")
	}
}

// ── CommitFiles error ─────────────────────────────────────────────────────────

func TestRunCommitFilesError(t *testing.T) {
	_, dir := setupRepo(t)
	addFile(t, dir, "x.go", "// fix")
	repo, _ := gogit.PlainOpen(dir)
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: patch something", &gogit.CommitOptions{Author: testSig()})

	old := gitCommitFiles
	gitCommitFiles = func(_ *gogit.Repository, _ []string, _, _, _ string) (plumbing.Hash, error) {
		return plumbing.ZeroHash, fmt.Errorf("injected commit error")
	}
	defer func() { gitCommitFiles = old }()

	err := execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error from CommitFiles")
	}
}

// ── parseBumpRules coverage ───────────────────────────────────────────────────

func TestParseBumpRules(t *testing.T) {
	rules := config.BumpRulesConfig{Breaking: "minor", Feat: "minor", Fix: "minor"}
	m := parseBumpRules(rules)
	if len(m) != 3 {
		t.Errorf("expected 3 entries in bump rules map, got %d", len(m))
	}
}

// ── printVerboseConfig — bump_rules and node rows ─────────────────────────────

func TestPrintVerboseConfigBumpRulesAndNode(t *testing.T) {
	cfg := config.Config{
		Git: config.GitConfig{
			BumpRules: config.BumpRulesConfig{Breaking: "minor", Feat: "minor", Fix: "minor"},
		},
		Node: config.NodeConfig{PackageJSON: "package.json"},
	}
	src := config.Sources{}

	old := os.Stderr
	r, wp, _ := os.Pipe()
	os.Stderr = wp
	printVerboseConfig(cfg, src)
	wp.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "minor") {
		t.Error("expected 'minor' in output for bump_rules")
	}
	if !strings.Contains(output, "package.json") {
		t.Error("expected 'package.json' in output for node.paths")
	}
}

// ── node package.json handling ────────────────────────────────────────────────

func writePackageJSON(t *testing.T, dir, ver string) {
	t.Helper()
	content := fmt.Sprintf(`{"name": "my-app", "version": "%s"}`, ver)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRunNodeVersionBump(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, dir, "0.0.0")
	commitAll(t, repo, dir, "chore: init")

	// .releaser.yml is untracked — go-git IsClean ignores untracked files
	os.WriteFile(filepath.Join(dir, ".releaser.yml"), []byte("node:\n  package_json: \"package.json\"\n"), 0644)

	addFile(t, dir, "x.go", "// fix")
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: something", &gogit.CommitOptions{Author: testSig()})

	err = execCmd(t, "--no-push", "--branch", "release/1.2", "--repo", dir)
	if err != nil {
		t.Fatalf("node version bump: unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	if !strings.Contains(string(data), `"version": "1.2.0"`) {
		t.Errorf("expected version 1.2.0 in package.json, got: %s", data)
	}
}

func TestRunNodeReadVersionFails(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	// invalid JSON — ReadVersion will fail
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{not json`), 0644)
	commitAll(t, repo, dir, "chore: init")

	os.WriteFile(filepath.Join(dir, ".releaser.yml"), []byte("node:\n  package_json: \"package.json\"\n"), 0644)

	addFile(t, dir, "x.go", "// fix")
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: something", &gogit.CommitOptions{Author: testSig()})

	err = execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when package.json has invalid JSON")
	}
}

func TestRunNodeWriteVersionFails(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, dir, "0.0.0")
	commitAll(t, repo, dir, "chore: init")

	os.WriteFile(filepath.Join(dir, ".releaser.yml"), []byte("node:\n  package_json: \"package.json\"\n"), 0644)

	addFile(t, dir, "x.go", "// fix")
	w, _ := repo.Worktree()
	w.Add("x.go")
	w.Commit("fix: something", &gogit.CommitOptions{Author: testSig()})

	// Make package.json read-only so WriteVersion fails
	os.Chmod(filepath.Join(dir, "package.json"), 0444)
	defer os.Chmod(filepath.Join(dir, "package.json"), 0644)

	err = execCmd(t, "--branch", "release/1.2", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when package.json is read-only")
	}
}
