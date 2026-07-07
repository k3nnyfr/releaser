package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	"git.k3nny.fr/releaser/internal/branch"
	"git.k3nny.fr/releaser/internal/changelog"
	"git.k3nny.fr/releaser/internal/commits"
	"git.k3nny.fr/releaser/internal/config"
	"git.k3nny.fr/releaser/internal/glclient"
	"git.k3nny.fr/releaser/internal/gitutil"
	"git.k3nny.fr/releaser/internal/maven"
	"git.k3nny.fr/releaser/internal/notes"
	semver "git.k3nny.fr/releaser/internal/version"
)

const defaultConfigTemplate = `# .releaser.yml — configuration for git.k3nny.fr/releaser
# All fields are optional. Uncomment and adjust what you need.
# CLI flags always take precedence over values set here.

git:
  # Prefix prepended to every version tag.
  # tag_prefix: "v"

  # Regex that identifies release branches. Must contain exactly two capture
  # groups: group 1 = major version, group 2 = minor version.
  # branch_pattern: "^(?:.*/)?release/(\\d+)\\.(\\d+)$"

  # Template for the version-bump commit message.
  # {version} is replaced with the full tag name (e.g. "v1.2.3").
  # commit_message: "chore(release): {version} [skip ci]"

  # Override the git commit author. When omitted, releaser reads user.name
  # and user.email from the repository's git config.
  # author_name: ""
  # author_email: ""

maven:
  # Path to pom.xml, relative to the repository root.
  # pom_path: "pom.xml"

gitlab:
  # GitLab instance URL. Falls back to the CI_SERVER_URL environment variable.
  # url: "https://gitlab.example.com"

  # Personal or CI access token with api scope.
  # Falls back to the GITLAB_TOKEN environment variable.
  # Tip: never commit a real token here — use the environment variable instead.
  # token: ""

  # Numeric project ID or "namespace/project" path.
  # Falls back to CI_PROJECT_ID, then CI_PROJECT_PATH environment variables.
  # project: ""
`

var (
	version             = "dev" // overridden at build time via -ldflags "-X main.version=..."
	errNothingToRelease = errors.New("nothing to release")
)

// exitFn is a variable so tests can intercept os.Exit calls.
var exitFn = os.Exit

func newRootCmd() *cobra.Command {
	var (
		init_          bool
		verbose        bool
		dryRun         bool
		noPush         bool
		noRelease      bool
		noCommit       bool
		tagOnly        bool
		branchOverride string
		repoPath       string
		pomOverride    string
		changelogFile  string
		tagPrefixFlag  string
		tagPrefixSet   bool
		patternFlag    string
		patternSet     bool
	)

	root := &cobra.Command{
		Use:          "releaser",
		Short:        "GitFlow release automation for Conventional Commits",
		Version:      version,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tagPrefixSet = cmd.Flags().Changed("tag-prefix")
			patternSet = cmd.Flags().Changed("branch-pattern")
			return run(options{
				init:           init_,
				verbose:        verbose,
				repoPath:       repoPath,
				branchOverride: branchOverride,
				pomOverride:    pomOverride,
				changelogFile:  changelogFile,
				tagPrefixFlag:  tagPrefixFlag,
				tagPrefixSet:   tagPrefixSet,
				patternFlag:    patternFlag,
				patternSet:     patternSet,
				dryRun:         dryRun,
				noPush:         noPush,
				noRelease:      noRelease,
				noCommit:       noCommit,
				tagOnly:        tagOnly,
			})
		},
	}

	root.Flags().BoolVar(&init_, "init", false, "create a default .releaser.yml in the repository and exit")
	root.Flags().BoolVar(&verbose, "verbose", false, "print configuration sources, commit list, and version decision")
	root.Flags().BoolVar(&dryRun, "dry-run", false, "print next version without making changes")
	root.Flags().BoolVar(&noPush, "no-push", false, "create commit and tag locally without pushing or creating a GitLab release")
	root.Flags().BoolVar(&noRelease, "no-release", false, "push commit and tag but skip creating the GitLab release")
	root.Flags().BoolVar(&noCommit, "no-commit", false, "update files but do not commit, tag, or push")
	root.Flags().BoolVar(&tagOnly, "tag-only", false, "tag HEAD without updating files (assumes version was already committed)")
	root.Flags().StringVar(&branchOverride, "branch", "", "override branch name detection (required in detached HEAD)")
	root.Flags().StringVar(&repoPath, "repo", ".", "path to git repository")
	root.Flags().StringVar(&pomOverride, "pom", "", "override maven.pom_path from config")
	root.Flags().StringVar(&changelogFile, "changelog-file", "CHANGELOG.md", "path to changelog file relative to repo root")
	root.Flags().StringVar(&tagPrefixFlag, "tag-prefix", "", "override git.tag_prefix from config")
	root.Flags().StringVar(&patternFlag, "branch-pattern", "", "override git.branch_pattern from config")

	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if errors.Is(err, errNothingToRelease) {
			exitFn(2)
			return
		}
		exitFn(1)
	}
}

type options struct {
	init           bool
	verbose        bool
	repoPath       string
	branchOverride string
	pomOverride    string
	changelogFile  string
	tagPrefixFlag  string
	tagPrefixSet   bool
	patternFlag    string
	patternSet     bool
	dryRun         bool
	noPush         bool
	noRelease      bool
	noCommit       bool
	tagOnly        bool
}

// vlog prints a verbose line to stderr, prefixed with "[verbose] ", when verbose is true.
func vlog(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] "+format+"\n", args...)
	}
}

func printVerboseConfig(cfg config.Config, src config.Sources) {
	fmt.Fprintln(os.Stderr, "[verbose] configuration:")
	rows := []struct{ key, val string }{
		{"git.tag_prefix", cfg.Git.TagPrefix},
		{"git.branch_pattern", cfg.Git.BranchPattern},
		{"git.commit_message", cfg.Git.CommitMessage},
		{"git.author_name", cfg.Git.AuthorName},
		{"git.author_email", cfg.Git.AuthorEmail},
		{"maven.pom_path", cfg.Maven.PomPath},
		{"gitlab.url", cfg.GitLab.URL},
		{"gitlab.token", func() string {
			if cfg.GitLab.Token != "" {
				return "(set)"
			}
			return "(not set)"
		}()},
		{"gitlab.project", cfg.GitLab.Project},
	}
	for _, r := range rows {
		source := src[r.key]
		if source == "" {
			source = "default"
		}
		fmt.Fprintf(os.Stderr, "  %-25s = %-40s [%s]\n", r.key, r.val, source)
	}
}

func initConfig(absRepo string) error {
	path := filepath.Join(absRepo, ".releaser.yml")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf(".releaser.yml already exists in %s — delete it first if you want to reset", absRepo)
	}
	if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0644); err != nil {
		return fmt.Errorf("write .releaser.yml: %w", err)
	}
	fmt.Printf("created %s\n", path)
	return nil
}

func run(o options) error {
	// --- Config ---
	absRepo, err := filepath.Abs(o.repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	if o.init {
		vlog(o.verbose, "creating .releaser.yml in %s", absRepo)
		return initConfig(absRepo)
	}

	var (
		cfg config.Config
		src config.Sources
	)
	if o.verbose {
		cfg, src, err = config.LoadWithSources(absRepo)
	} else {
		cfg, err = config.Load(absRepo)
	}
	if err != nil {
		return err
	}
	cfg.ApplyEnvWithSources(src)

	// CLI flags take precedence over config file and env vars
	if o.tagPrefixSet {
		cfg.Git.TagPrefix = o.tagPrefixFlag
		if src != nil {
			src["git.tag_prefix"] = "flag: --tag-prefix"
		}
	}
	if o.pomOverride != "" {
		cfg.Maven.PomPath = o.pomOverride
		if src != nil {
			src["maven.pom_path"] = "flag: --pom"
		}
	}
	if o.patternSet {
		cfg.Git.BranchPattern = o.patternFlag
		if src != nil {
			src["git.branch_pattern"] = "flag: --branch-pattern"
		}
	}

	if o.verbose {
		printVerboseConfig(cfg, src)
	}

	// --- Git ---
	repo, err := gogit.PlainOpenWithOptions(absRepo, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}

	branchName := o.branchOverride
	if branchName == "" {
		branchName, err = gitutil.CurrentBranch(repo)
		if err != nil {
			return err
		}
	}

	info, err := branch.Parse(branchName, cfg.Git.BranchPattern)
	if err != nil {
		return err
	}
	info.TagPrefix = cfg.Git.TagPrefix

	vlog(o.verbose, "branch: %s → major=%d, minor=%d (pinned by branch)", branchName, info.Major, info.Minor)

	// --- Dirty check (before any changes) ---
	// Skipped in --no-commit mode: the user intentionally has changes in flight.
	if !o.dryRun && !o.noCommit {
		clean, err := gitutil.IsWorkingTreeClean(repo)
		if err != nil {
			return fmt.Errorf("check working tree: %w", err)
		}
		if !clean {
			return fmt.Errorf("working tree has uncommitted changes — commit or stash them before releasing")
		}
	}

	// --- Tag discovery ---
	lastTag, currentPatch, err := gitutil.LatestTag(repo, info)
	if err != nil {
		return fmt.Errorf("find latest tag: %w", err)
	}

	if o.verbose {
		if lastTag == "" {
			vlog(true, "last tag: none — scanning all commits")
		} else {
			vlog(true, "last tag: %s → current patch=%d", lastTag, currentPatch)
		}
	}

	// --- Commit range ---
	var messages []string
	if lastTag == "" {
		fmt.Fprintln(os.Stderr, "info: no previous tag found — scanning all commits")
		messages, err = gitutil.AllCommits(repo)
	} else {
		fmt.Fprintf(os.Stderr, "info: last tag: %s\n", lastTag)
		messages, err = gitutil.CommitsSince(repo, lastTag)
	}
	if err != nil {
		return fmt.Errorf("read commits: %w", err)
	}

	fmt.Fprintf(os.Stderr, "info: %d commit(s) to analyze\n", len(messages))

	// --- Version calculation ---
	types := make([]commits.Type, len(messages))
	for i, msg := range messages {
		types[i] = commits.Parse(msg)
	}

	if o.verbose {
		vlog(true, "commits analyzed (%d):", len(messages))
		for i, msg := range messages {
			first := strings.SplitN(strings.TrimSpace(msg), "\n", 2)[0]
			t := types[i]
			var decision string
			switch t {
			case commits.TypeBreaking:
				decision = "breaking → patch bump"
			case commits.TypeFeat:
				decision = "feat → patch bump"
			case commits.TypeFix:
				decision = "fix → patch bump"
			default:
				decision = "ignored"
			}
			fmt.Fprintf(os.Stderr, "  %-60s  %s\n", first, decision)
		}
	}

	nextVersion, ok := semver.Next(info.Major, info.Minor, currentPatch, types)
	if !ok {
		fmt.Fprintln(os.Stderr, "info: no releasable commits found")
		return errNothingToRelease
	}

	nextTag := info.TagName(nextVersion)

	if o.verbose {
		highestType := commits.TypeNone
		for _, t := range types {
			if t > highestType {
				highestType = t
			}
		}
		vlog(true, "version decision: highest type=%s → next=%s (tag: %s)", highestType, nextVersion, nextTag)
	}

	fmt.Printf("next version: %s (tag: %s)\n", nextVersion, nextTag)

	if o.dryRun {
		fmt.Fprintln(os.Stderr, "dry-run: no changes made")
		return nil
	}

	// --- pom.xml + CHANGELOG.md (skipped with --tag-only) ---
	if !o.tagOnly {
		var filesToCommit []string

		// pom.xml
		pomPath := filepath.Join(absRepo, cfg.Maven.PomPath)
		_, statErr := os.Stat(pomPath)
		hasPom := !errors.Is(statErr, os.ErrNotExist)
		if statErr != nil && hasPom {
			return fmt.Errorf("check pom path: %w", statErr)
		}
		if hasPom {
			currentPomVersion, err := maven.ReadVersion(pomPath)
			if err != nil {
				return fmt.Errorf("read pom version: %w", err)
			}
			fmt.Fprintf(os.Stderr, "info: pom.xml: %s → %s\n", currentPomVersion, nextVersion)
			if err := maven.WriteVersion(pomPath, currentPomVersion, nextVersion); err != nil {
				return fmt.Errorf("update pom version: %w", err)
			}
			filesToCommit = append(filesToCommit, cfg.Maven.PomPath)
		} else {
			fmt.Fprintln(os.Stderr, "info: no pom.xml found — skipping version bump")
		}

		// CHANGELOG.md
		changelogAbsPath := filepath.Join(absRepo, o.changelogFile)
		if err := changelog.Update(changelogAbsPath, nextTag, nextVersion, messages); err != nil {
			return fmt.Errorf("update changelog: %w", err)
		}
		fmt.Fprintf(os.Stderr, "info: %s updated\n", o.changelogFile)
		filesToCommit = append(filesToCommit, o.changelogFile)

		if o.noCommit {
			fmt.Printf("files updated to %s — commit manually then re-run with --tag-only\n", nextVersion)
			return nil
		}

		// --- Git commit ---
		authorName, authorEmail := gitutil.AuthorFromConfig(repo)
		if cfg.Git.AuthorName != "" {
			authorName = cfg.Git.AuthorName
		}
		if cfg.Git.AuthorEmail != "" {
			authorEmail = cfg.Git.AuthorEmail
		}
		commitMsg := strings.ReplaceAll(cfg.Git.CommitMessage, "{version}", nextTag)
		if _, err := gitutil.CommitFiles(repo, filesToCommit, commitMsg, authorName, authorEmail); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		fmt.Fprintf(os.Stderr, "info: committed: %s\n", commitMsg)
	}

	// --- Git tag ---
	if err := gitutil.CreateTag(repo, nextTag); err != nil {
		return fmt.Errorf("create tag: %w", err)
	}
	fmt.Fprintf(os.Stderr, "info: tag created: %s\n", nextTag)

	if o.noPush {
		fmt.Printf("released %s locally — push manually with: git push && git push --tags\n", nextTag)
		return nil
	}

	// --- Push ---
	fmt.Fprintln(os.Stderr, "info: pushing commit and tag...")
	if err := gitutil.Push(repo, branchName, nextTag, cfg.GitLab.Token); err != nil {
		return fmt.Errorf("push: %w", err)
	}
	fmt.Fprintln(os.Stderr, "info: pushed")

	// --- GitLab release ---
	if o.noRelease {
		fmt.Printf("released %s\n", nextTag)
		return nil
	}
	if cfg.GitLab.URL == "" || cfg.GitLab.Project == "" {
		fmt.Fprintln(os.Stderr, "warning: GitLab URL or project not configured — skipping release creation")
		fmt.Printf("released %s\n", nextTag)
		return nil
	}
	if cfg.GitLab.Token == "" {
		return fmt.Errorf("GITLAB_TOKEN not set — required for release creation")
	}

	releaseNotes := notes.Generate(nextTag, messages)
	gl := glclient.New(cfg.GitLab.URL, cfg.GitLab.Token, cfg.GitLab.Project)

	if err := gl.CreateRelease(context.Background(), nextTag, releaseNotes); err != nil {
		return fmt.Errorf("create GitLab release: %w", err)
	}

	fmt.Fprintf(os.Stderr, "info: GitLab release created: %s\n", nextTag)
	fmt.Printf("released %s\n", nextTag)
	return nil
}
