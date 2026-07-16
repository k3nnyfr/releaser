package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/spf13/cobra"

	"git.k3nny.fr/releaser/internal/branch"
	"git.k3nny.fr/releaser/internal/changelog"
	"git.k3nny.fr/releaser/internal/commits"
	"git.k3nny.fr/releaser/internal/config"
	"git.k3nny.fr/releaser/internal/ghclient"
	"git.k3nny.fr/releaser/internal/gitutil"
	"git.k3nny.fr/releaser/internal/glclient"
	"git.k3nny.fr/releaser/internal/gradle"
	"git.k3nny.fr/releaser/internal/maven"
	"git.k3nny.fr/releaser/internal/pyproject"
	"git.k3nny.fr/releaser/internal/node"
	"git.k3nny.fr/releaser/internal/notes"
	"git.k3nny.fr/releaser/internal/notify"
	semver "git.k3nny.fr/releaser/internal/version"
)

const defaultConfigTemplate = `# .releaser.yml — configuration for git.k3nny.fr/releaser
# All fields are optional. Uncomment and adjust what you need.
# CLI flags always take precedence over values set here.

git:
  # Prefix prepended to every version tag (default: no prefix).
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

  # Limit which commit types trigger a release (default: fix, feat, breaking).
  # releasable_types:
  #   - fix
  #   - feat
  #   - breaking

  # Configure which version component each commit type bumps.
  # Valid values: "patch" (default) or "minor".
  # bump_rules:
  #   breaking: "minor"   # bump minor version instead of patch on breaking changes
  #   feat: "patch"
  #   fix: "patch"

maven:
  # Single pom.xml path, relative to the repository root.
  # pom_path: "pom.xml"

  # Multiple pom.xml paths for multi-module projects (overrides pom_path).
  # pom_paths:
  #   - "pom.xml"
  #   - "module-a/pom.xml"
  #   - "module-b/pom.xml"

node:
  # Single package.json path (node processing is opt-in — no default).
  # package_json: "package.json"

  # Multiple package.json paths for monorepos (overrides package_json).
  # package_jsons:
  #   - "package.json"
  #   - "packages/frontend/package.json"
  #   - "packages/backend/package.json"

gradle:
  # Single build.gradle or build.gradle.kts path (opt-in — no default).
  # Both Groovy DSL (single-quoted) and Kotlin DSL (double-quoted) are supported.
  # build_file: "build.gradle"

  # Multiple build files for multi-module projects (overrides build_file).
  # build_files:
  #   - "build.gradle"
  #   - "module-a/build.gradle"
  #   - "module-b/build.gradle"

python:
  # Single pyproject.toml path (opt-in — no default).
  # Reads [project].version (PEP 621) first, then [tool.poetry].version.
  # pyproject_toml: "pyproject.toml"

  # Multiple pyproject.toml paths for monorepos (overrides pyproject_toml).
  # pyproject_tomls:
  #   - "pyproject.toml"
  #   - "packages/cli/pyproject.toml"
  #   - "packages/lib/pyproject.toml"

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

github:
  # GitHub personal access token with repo scope.
  # Falls back to the GITHUB_TOKEN environment variable.
  # token: ""

  # Repository in "owner/repo" format.
  # repo: ""

notify:
  # Every field below is opt-in. A target is only used when its required
  # fields are set (via this file, or the matching environment variable).
  # Notification failures never fail the release — they're logged as warnings.

  # Slack incoming webhook URL. Falls back to SLACK_WEBHOOK_URL.
  # slack_webhook_url: ""

  # Microsoft Teams incoming webhook URL. Falls back to TEAMS_WEBHOOK_URL.
  # teams_webhook_url: ""

  # Google Chat incoming webhook URL. Falls back to GOOGLE_CHAT_WEBHOOK_URL.
  # google_chat_webhook_url: ""

  # Telegram bot token and chat ID — both required. Fall back to
  # TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID.
  # telegram_bot_token: ""
  # telegram_chat_id: ""

  # Generic webhook URL — POSTed a {"version": ..., "notes": ...} JSON body.
  # Falls back to RELEASER_WEBHOOK_URL.
  # webhook_url: ""
`

var (
	version             = "dev" // overridden at build time via -ldflags "-X main.version=..."
	errNothingToRelease = errors.New("nothing to release")
)

// exitFn is a variable so tests can intercept os.Exit calls.
var exitFn = os.Exit

// injectable function variables for testing error paths.
var (
	absPath        = filepath.Abs
	gitAllCommits  = gitutil.AllCommits
	gitCommitsSince = gitutil.CommitsSince
	gitCommitFiles = gitutil.CommitFiles
)

// releasePublisher is implemented by both glclient and ghclient.
type releasePublisher interface {
	CreateRelease(ctx context.Context, tagName, body string) error
}

// buildPublisher selects and returns the active release publisher based on config.
// GitHub takes precedence over GitLab when both are configured.
// Returns (nil, nil) when no provider is configured — caller should skip release creation.
func buildPublisher(cfg config.Config) (releasePublisher, error) {
	if cfg.GitHub.Token != "" && cfg.GitHub.Repo != "" {
		return ghclient.New(cfg.GitHub.Token, cfg.GitHub.Repo), nil
	}
	if cfg.GitLab.URL != "" && cfg.GitLab.Project != "" {
		if cfg.GitLab.Token == "" {
			return nil, fmt.Errorf("GITLAB_TOKEN not set — required for release creation")
		}
		return glclient.New(cfg.GitLab.URL, cfg.GitLab.Token, cfg.GitLab.Project), nil
	}
	return nil, nil
}

func newRootCmd() *cobra.Command {
	var (
		init_           bool
		verbose         bool
		dryRun          bool
		noPush          bool
		noRelease       bool
		noCommit        bool
		tagOnly         bool
		allowShallow    bool
		check           bool
		branchOverride  string
		repoPath        string
		pomOverride     string
		gradleOverride     string
		pyprojectOverride  string
		changelogFile      string
		tagPrefixFlag   string
		tagPrefixSet    bool
		patternFlag     string
		patternSet      bool
		releaseEnvFile  string
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
				gradleOverride:    gradleOverride,
				pyprojectOverride: pyprojectOverride,
				changelogFile:     changelogFile,
				tagPrefixFlag:  tagPrefixFlag,
				tagPrefixSet:   tagPrefixSet,
				patternFlag:    patternFlag,
				patternSet:     patternSet,
				dryRun:         dryRun,
				noPush:         noPush,
				noRelease:      noRelease,
				noCommit:       noCommit,
				tagOnly:        tagOnly,
				allowShallow:   allowShallow,
				check:          check,
				releaseEnvFile: releaseEnvFile,
			})
		},
	}

	root.Flags().BoolVar(&init_, "init", false, "create a default .releaser.yml in the repository and exit")
	root.Flags().BoolVar(&verbose, "verbose", false, "print configuration sources, commit list, and version decision")
	root.Flags().BoolVar(&dryRun, "dry-run", false, "print next version without making changes")
	root.Flags().BoolVar(&noPush, "no-push", false, "create commit and tag locally without pushing or creating a release")
	root.Flags().BoolVar(&noRelease, "no-release", false, "push commit and tag but skip creating the release")
	root.Flags().BoolVar(&noCommit, "no-commit", false, "update files but do not commit, tag, or push")
	root.Flags().BoolVar(&tagOnly, "tag-only", false, "tag HEAD without updating files (assumes version was already committed)")
	root.Flags().BoolVar(&allowShallow, "allow-shallow", false, "proceed in a shallow clone even when no previous release tag is found")
	root.Flags().BoolVar(&check, "check", false, "preflight: validate branch, working tree, remote, auth, version files, and release target without releasing")
	root.Flags().StringVar(&branchOverride, "branch", "", "override branch name detection (detached HEAD falls back to CI_COMMIT_BRANCH, CI_COMMIT_REF_NAME, GITHUB_REF_NAME)")
	root.Flags().StringVar(&repoPath, "repo", ".", "path to git repository")
	root.Flags().StringVar(&pomOverride, "pom", "", "override maven.pom_path from config")
	root.Flags().StringVar(&gradleOverride, "gradle", "", "override gradle.build_file from config")
	root.Flags().StringVar(&pyprojectOverride, "pyproject", "", "override python.pyproject_toml from config")
	root.Flags().StringVar(&changelogFile, "changelog-file", "CHANGELOG.md", "path to changelog file relative to repo root")
	root.Flags().StringVar(&tagPrefixFlag, "tag-prefix", "", "override git.tag_prefix from config")
	root.Flags().StringVar(&patternFlag, "branch-pattern", "", "override git.branch_pattern from config")
	root.Flags().StringVar(&releaseEnvFile, "release-env-file", "release.env", "write NEXT_VERSION dotenv to this path (relative to repo root; empty to disable)")

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
	init            bool
	verbose         bool
	repoPath        string
	branchOverride  string
	pomOverride     string
	gradleOverride    string
	pyprojectOverride string
	changelogFile     string
	tagPrefixFlag     string
	tagPrefixSet    bool
	patternFlag     string
	patternSet      bool
	dryRun          bool
	noPush          bool
	noRelease       bool
	noCommit        bool
	tagOnly         bool
	allowShallow    bool
	check           bool
	releaseEnvFile  string
}

// maskedIfSet reports whether a secret-like config value is set, without
// printing the value itself.
func maskedIfSet(v string) string {
	if v != "" {
		return "(set)"
	}
	return "(not set)"
}

func printVerboseConfig(cfg config.Config, src config.Sources) {
	logSection("configuration")
	rows := []struct{ key, val string }{
		{"git.tag_prefix", cfg.Git.TagPrefix},
		{"git.branch_pattern", cfg.Git.BranchPattern},
		{"git.commit_message", cfg.Git.CommitMessage},
		{"git.author_name", cfg.Git.AuthorName},
		{"git.author_email", cfg.Git.AuthorEmail},
		{"git.releasable_types", func() string {
			if len(cfg.Git.ReleasableTypes) == 0 {
				return "(all)"
			}
			return strings.Join(cfg.Git.ReleasableTypes, ", ")
		}()},
		{"git.bump_rules.breaking", func() string {
			if cfg.Git.BumpRules.Breaking == "" {
				return "patch"
			}
			return cfg.Git.BumpRules.Breaking
		}()},
		{"git.bump_rules.feat", func() string {
			if cfg.Git.BumpRules.Feat == "" {
				return "patch"
			}
			return cfg.Git.BumpRules.Feat
		}()},
		{"git.bump_rules.fix", func() string {
			if cfg.Git.BumpRules.Fix == "" {
				return "patch"
			}
			return cfg.Git.BumpRules.Fix
		}()},
		{"maven.pom_paths", strings.Join(cfg.Maven.EffectivePomPaths(), ", ")},
		{"node.paths", func() string {
			paths := cfg.Node.EffectivePaths()
			if len(paths) == 0 {
				return "(not configured)"
			}
			return strings.Join(paths, ", ")
		}()},
		{"gradle.paths", func() string {
			paths := cfg.Gradle.EffectiveBuildFiles()
			if len(paths) == 0 {
				return "(not configured)"
			}
			return strings.Join(paths, ", ")
		}()},
		{"python.paths", func() string {
			paths := cfg.Python.EffectivePaths()
			if len(paths) == 0 {
				return "(not configured)"
			}
			return strings.Join(paths, ", ")
		}()},
		{"gitlab.url", cfg.GitLab.URL},
		{"gitlab.token", maskedIfSet(cfg.GitLab.Token)},
		{"gitlab.project", cfg.GitLab.Project},
		{"github.token", maskedIfSet(cfg.GitHub.Token)},
		{"github.repo", cfg.GitHub.Repo},
		{"notify.slack_webhook_url", maskedIfSet(cfg.Notify.SlackWebhookURL)},
		{"notify.teams_webhook_url", maskedIfSet(cfg.Notify.TeamsWebhookURL)},
		{"notify.google_chat_webhook_url", maskedIfSet(cfg.Notify.GoogleChatWebhookURL)},
		{"notify.telegram_bot_token", maskedIfSet(cfg.Notify.TelegramBotToken)},
		{"notify.telegram_chat_id", maskedIfSet(cfg.Notify.TelegramChatID)},
		{"notify.webhook_url", maskedIfSet(cfg.Notify.WebhookURL)},
	}
	for _, r := range rows {
		source := src[r.key]
		if source == "" {
			source = "default"
		}
		val := r.val
		if val == "" {
			val = paint(ansiDim, "(empty)")
		}
		fmt.Fprintf(os.Stderr, "  %-25s = %-45s %s\n", r.key, val, fmtSource(source))
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

func parseBumpRules(rules config.BumpRulesConfig) map[commits.Type]semver.BumpLevel {
	m := map[commits.Type]semver.BumpLevel{}
	if rules.Breaking == "minor" {
		m[commits.TypeBreaking] = semver.BumpMinor
	}
	if rules.Feat == "minor" {
		m[commits.TypeFeat] = semver.BumpMinor
	}
	if rules.Fix == "minor" {
		m[commits.TypeFix] = semver.BumpMinor
	}
	return m
}

func run(o options) error {
	logHeader(version)

	// --- Config ---
	absRepo, err := absPath(o.repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	if o.init {
		if o.verbose {
			logStep("creating .releaser.yml in %s", absRepo)
		}
		return initConfig(absRepo)
	}

	cfg, src, err := config.LoadWithSources(absRepo)
	if err != nil {
		return err
	}
	cfg.ApplyEnvWithSources(src)

	// CLI flags take precedence over config file and env vars
	if o.tagPrefixSet {
		cfg.Git.TagPrefix = o.tagPrefixFlag
		src["git.tag_prefix"] = "flag: --tag-prefix"
	}
	if o.pomOverride != "" {
		cfg.Maven.PomPath = o.pomOverride
		cfg.Maven.PomPaths = nil
		src["maven.pom_paths"] = "flag: --pom"
	}
	if o.gradleOverride != "" {
		cfg.Gradle.BuildFile = o.gradleOverride
		cfg.Gradle.BuildFiles = nil
		src["gradle.build_files"] = "flag: --gradle"
	}
	if o.pyprojectOverride != "" {
		cfg.Python.PyprojectTOML = o.pyprojectOverride
		cfg.Python.PyprojectTOMLs = nil
		src["python.pyproject_tomls"] = "flag: --pyproject"
	}
	if o.patternSet {
		cfg.Git.BranchPattern = o.patternFlag
		src["git.branch_pattern"] = "flag: --branch-pattern"
	}

	if o.verbose {
		printVerboseConfig(cfg, src)
	}

	if o.check {
		return runCheck(o, cfg, absRepo)
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

	if o.verbose {
		logSection("branch")
		fmt.Fprintf(os.Stderr, "  %s → major=%d, minor=%d %s\n",
			paint(ansiBold, branchName), info.Major, info.Minor,
			paint(ansiDim, "(pinned by branch)"))
	}

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

	// A shallow clone with no tag found is ambiguous: either a genuine first
	// release, or the previous tags sit beyond the fetch depth — in which case
	// releasing would silently restart at X.Y.0. When a tag was found the
	// version math is correct regardless of shallowness.
	if lastTag == "" && !o.allowShallow {
		shallow, err := gitutil.IsShallow(repo)
		if err != nil {
			return fmt.Errorf("check shallow clone: %w", err)
		}
		if shallow {
			return fmt.Errorf("no release tag found in a shallow clone — previous tags may be beyond the fetch depth; fetch full history (GIT_DEPTH: 0 in GitLab CI, fetch-depth: 0 in GitHub Actions, or git fetch --unshallow), or pass --allow-shallow if this is truly the first release")
		}
	}

	// --- Commit range ---
	var messages []string
	if lastTag == "" {
		messages, err = gitAllCommits(repo)
	} else {
		messages, err = gitCommitsSince(repo, lastTag)
	}
	if err != nil {
		return fmt.Errorf("read commits: %w", err)
	}

	if !o.verbose {
		if lastTag == "" {
			logStep("no previous tag — scanning all %d commit(s)", len(messages))
		} else {
			logStep("last tag: %s  (%d commit(s) to analyze)", lastTag, len(messages))
		}
	}

	// --- Version calculation ---
	types := make([]commits.Type, len(messages))
	for i, msg := range messages {
		types[i] = commits.Parse(msg)
	}

	if o.verbose {
		logSection(fmt.Sprintf("commits (%d)", len(messages)))
		if lastTag != "" {
			fmt.Fprintf(os.Stderr, "  since: %s  (patch=%d)\n", paint(ansiCyan, lastTag), currentPatch)
		}
		for i, msg := range messages {
			first := strings.SplitN(strings.TrimSpace(msg), "\n", 2)[0]
			if len(first) > 70 {
				first = first[:67] + "..."
			}
			t := types[i]
			typeLabel := fmt.Sprintf("%-9s", t.String())
			if t == commits.TypeNone {
				fmt.Fprintf(os.Stderr, "  %s\n", paint(ansiDim, typeLabel+first))
			} else {
				var col string
				switch t {
				case commits.TypeBreaking:
					col = ansiRed + ansiBold
				case commits.TypeFeat:
					col = ansiCyan
				default: // fix
					col = ansiGreen
				}
				fmt.Fprintf(os.Stderr, "  %s %s  %s\n",
					paint(col, typeLabel), first, paint(ansiDim, "→ patch bump"))
			}
		}
	}

	releasable := commits.ReleasableSet(cfg.Git.ReleasableTypes)
	nextVersion, ok := semver.Next(info.Major, info.Minor, currentPatch, types, releasable, parseBumpRules(cfg.Git.BumpRules))
	if !ok {
		logWarn("no releasable commits found")
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
		logSection("version")
		fmt.Fprintf(os.Stderr, "  highest type: %s  →  next: %s  (tag: %s)\n",
			paint(ansiCyan, highestType.String()),
			paint(ansiBold, nextVersion),
			paint(ansiBold+ansiCyan, nextTag))
	}

	fmt.Printf("next version: %s (tag: %s)\n", nextVersion, nextTag)

	if o.dryRun {
		logStep("dry-run: no changes made")
		return nil
	}

	// --- release.env (GitLab CI dotenv artifact) ---
	if o.releaseEnvFile != "" {
		releaseEnvPath := filepath.Join(absRepo, o.releaseEnvFile)
		if err := os.WriteFile(releaseEnvPath, []byte("NEXT_VERSION="+nextTag+"\n"), 0644); err != nil {
			return fmt.Errorf("write %s: %w", o.releaseEnvFile, err)
		}
		logDone("%s: NEXT_VERSION=%s", o.releaseEnvFile, nextTag)
	}

	// --- pom.xml + CHANGELOG.md (skipped with --tag-only) ---
	if !o.tagOnly {
		var filesToCommit []string

		// pom.xml (supports multi-module via pom_paths)
		anyPom := false
		for _, relPomPath := range cfg.Maven.EffectivePomPaths() {
			pomPath := filepath.Join(absRepo, relPomPath)
			_, statErr := os.Stat(pomPath)
			hasPom := !errors.Is(statErr, os.ErrNotExist)
			if statErr != nil && hasPom {
				return fmt.Errorf("check pom path: %w", statErr)
			}
			if hasPom {
				anyPom = true
				currentPomVersion, err := maven.ReadVersion(pomPath)
				if err != nil {
					return fmt.Errorf("read pom version: %w", err)
				}
				if err := maven.WriteVersion(pomPath, currentPomVersion, nextVersion); err != nil {
					return fmt.Errorf("update pom version: %w", err)
				}
				logDone("%s: %s → %s", relPomPath, currentPomVersion, nextVersion)
				filesToCommit = append(filesToCommit, relPomPath)
			}
		}
		if !anyPom {
			logWarn("no pom.xml — skipping version bump")
		}

		// package.json (opt-in via node.package_json / node.package_jsons)
		for _, relPkgPath := range cfg.Node.EffectivePaths() {
			pkgPath := filepath.Join(absRepo, relPkgPath)
			currentNodeVersion, err := node.ReadVersion(pkgPath)
			if err != nil {
				return fmt.Errorf("read package.json version: %w", err)
			}
			if err := node.WriteVersion(pkgPath, currentNodeVersion, nextVersion); err != nil {
				return fmt.Errorf("update package.json version: %w", err)
			}
			logDone("%s: %s → %s", relPkgPath, currentNodeVersion, nextVersion)
			filesToCommit = append(filesToCommit, relPkgPath)
		}

		// build.gradle / build.gradle.kts (opt-in via gradle.build_file / gradle.build_files)
		for _, relGradlePath := range cfg.Gradle.EffectiveBuildFiles() {
			gradlePath := filepath.Join(absRepo, relGradlePath)
			currentGradleVersion, err := gradle.ReadVersion(gradlePath)
			if err != nil {
				return fmt.Errorf("read gradle version: %w", err)
			}
			if err := gradle.WriteVersion(gradlePath, currentGradleVersion, nextVersion); err != nil {
				return fmt.Errorf("update gradle version: %w", err)
			}
			logDone("%s: %s → %s", relGradlePath, currentGradleVersion, nextVersion)
			filesToCommit = append(filesToCommit, relGradlePath)
		}

		// pyproject.toml (opt-in via python.pyproject_toml / python.pyproject_tomls)
		for _, relPyprojectPath := range cfg.Python.EffectivePaths() {
			pyprojectPath := filepath.Join(absRepo, relPyprojectPath)
			currentPyVersion, err := pyproject.ReadVersion(pyprojectPath)
			if err != nil {
				return fmt.Errorf("read pyproject version: %w", err)
			}
			if err := pyproject.WriteVersion(pyprojectPath, currentPyVersion, nextVersion); err != nil {
				return fmt.Errorf("update pyproject version: %w", err)
			}
			logDone("%s: %s → %s", relPyprojectPath, currentPyVersion, nextVersion)
			filesToCommit = append(filesToCommit, relPyprojectPath)
		}

		// CHANGELOG.md
		changelogAbsPath := filepath.Join(absRepo, o.changelogFile)
		if err := changelog.Update(changelogAbsPath, nextTag, nextVersion, messages); err != nil {
			return fmt.Errorf("update changelog: %w", err)
		}
		logDone("%s updated", o.changelogFile)
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
		if _, err := gitCommitFiles(repo, filesToCommit, commitMsg, authorName, authorEmail); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		logDone("committed: %s", commitMsg)
	}

	// --- Git tag ---
	if err := gitutil.CreateTag(repo, nextTag); err != nil {
		return fmt.Errorf("create tag: %w", err)
	}
	logDone("tag: %s", nextTag)

	if o.noPush {
		fmt.Printf("released %s locally — push manually with: git push && git push --tags\n", nextTag)
		return nil
	}

	// --- Push ---
	logStep("pushing commit and tag...")
	if err := gitutil.Push(repo, branchName, nextTag, cfg.GitLab.Token); err != nil {
		return fmt.Errorf("push: %w", err)
	}
	logDone("pushed")

	releaseNotes := notes.Generate(nextTag, messages)

	if o.noRelease {
		notifyRelease(cfg, nextTag, releaseNotes)
		fmt.Printf("released %s\n", nextTag)
		return nil
	}

	// --- Release creation ---
	publisher, err := buildPublisher(cfg)
	if err != nil {
		return err
	}
	if publisher == nil {
		logWarn("no release provider configured — skipping release creation")
	} else {
		if err := publisher.CreateRelease(context.Background(), nextTag, releaseNotes); err != nil {
			return fmt.Errorf("create release: %w", err)
		}
		logDone("release created: %s", nextTag)
	}

	notifyRelease(cfg, nextTag, releaseNotes)
	fmt.Printf("released %s\n", nextTag)
	return nil
}

// runCheck is the --check preflight: it validates everything a release needs
// without writing or pushing anything, and reports every problem at once
// instead of stopping at the first. Purely informational lines use logStep;
// validated checks use logDone; failures use logWarn and fail the run.
func runCheck(o options, cfg config.Config, absRepo string) error {
	logSection("check")

	failed := 0
	fail := func(format string, args ...any) {
		logWarn(format, args...)
		failed++
	}

	repo, err := gogit.PlainOpenWithOptions(absRepo, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}

	// --- branch resolution + pattern match ---
	var info branch.Info
	branchOK := false
	branchName := o.branchOverride
	var branchErr error
	if branchName == "" {
		branchName, branchErr = gitutil.CurrentBranch(repo)
	}
	if branchErr == nil {
		info, branchErr = branch.Parse(branchName, cfg.Git.BranchPattern)
	}
	if branchErr != nil {
		fail("branch: %v", branchErr)
	} else {
		info.TagPrefix = cfg.Git.TagPrefix
		branchOK = true
		logDone("branch: %s → major=%d, minor=%d", branchName, info.Major, info.Minor)
	}

	// --- working tree ---
	if clean, err := gitutil.IsWorkingTreeClean(repo); err != nil {
		fail("working tree: %v", err)
	} else if !clean {
		fail("working tree has uncommitted changes")
	} else {
		logDone("working tree clean")
	}

	// --- tag discovery + shallow clone ---
	if !branchOK {
		logStep("tag discovery skipped (branch unresolved)")
	} else if lastTag, _, err := gitutil.LatestTag(repo, info); err != nil {
		fail("tag discovery: %v", err)
	} else if lastTag != "" {
		logDone("latest tag: %s", lastTag)
	} else if shallow, err := gitutil.IsShallow(repo); err != nil {
		fail("shallow check: %v", err)
	} else if shallow && !o.allowShallow {
		fail("no release tag found in a shallow clone — previous tags may be beyond the fetch depth; fetch full history or pass --allow-shallow")
	} else if shallow {
		logStep("no previous tag in a shallow clone (--allow-shallow)")
	} else {
		logDone("no previous tag (first release)")
	}

	// --- remote + push auth ---
	if remote, err := repo.Remote("origin"); err != nil {
		fail("remote origin: %v", err)
	} else {
		url := remote.Config().URLs[0]
		// The same parse the push transport performs — catches malformed
		// remote URLs (e.g. shell quoting accidents in CI set-url lines).
		if _, err := transport.NewEndpoint(url); err != nil {
			fail("remote origin URL: %v", err)
		} else {
			logDone("remote origin: %s", url)
		}
		switch {
		case cfg.GitLab.Token != "":
			logStep("push auth: HTTPS token (oauth2)")
		case strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://"):
			logStep("push auth: SSH agent, git CLI fallback")
		default:
			logStep("push auth: git CLI (credential helpers, netrc)")
		}
	}

	// --- version files ---
	anyPom := false
	for _, relPomPath := range cfg.Maven.EffectivePomPaths() {
		pomPath := filepath.Join(absRepo, relPomPath)
		_, statErr := os.Stat(pomPath)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			fail("%s: %v", relPomPath, statErr)
			continue
		}
		anyPom = true
		if v, err := maven.ReadVersion(pomPath); err != nil {
			fail("%s: %v", relPomPath, err)
		} else {
			logDone("%s: version %s", relPomPath, v)
		}
	}
	if !anyPom {
		logStep("no pom.xml — Maven version bump will be skipped")
	}
	for _, relPkgPath := range cfg.Node.EffectivePaths() {
		if v, err := node.ReadVersion(filepath.Join(absRepo, relPkgPath)); err != nil {
			fail("%s: %v", relPkgPath, err)
		} else {
			logDone("%s: version %s", relPkgPath, v)
		}
	}
	for _, relGradlePath := range cfg.Gradle.EffectiveBuildFiles() {
		if v, err := gradle.ReadVersion(filepath.Join(absRepo, relGradlePath)); err != nil {
			fail("%s: %v", relGradlePath, err)
		} else {
			logDone("%s: version %s", relGradlePath, v)
		}
	}
	for _, relPyprojectPath := range cfg.Python.EffectivePaths() {
		if v, err := pyproject.ReadVersion(filepath.Join(absRepo, relPyprojectPath)); err != nil {
			fail("%s: %v", relPyprojectPath, err)
		} else {
			logDone("%s: version %s", relPyprojectPath, v)
		}
	}

	// --- release target (mirrors buildPublisher precedence) ---
	switch {
	case cfg.GitHub.Token != "" && cfg.GitHub.Repo != "":
		logDone("release target: GitHub (%s)", cfg.GitHub.Repo)
	case cfg.GitLab.URL != "" && cfg.GitLab.Project != "":
		if cfg.GitLab.Token == "" {
			fail("release target: GitLab configured but GITLAB_TOKEN not set")
		} else {
			logDone("release target: GitLab (%s, project %s)", cfg.GitLab.URL, cfg.GitLab.Project)
		}
	default:
		logStep("no release provider configured — release creation will be skipped")
	}

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	fmt.Println("all checks passed")
	return nil
}

// notifyRelease sends best-effort release notifications to every configured
// target. Failures are logged as warnings, not errors — the release itself
// already succeeded by the time this runs.
func notifyRelease(cfg config.Config, tagName, releaseNotes string) {
	notifyCfg := notify.Config{
		SlackWebhookURL:      cfg.Notify.SlackWebhookURL,
		TeamsWebhookURL:      cfg.Notify.TeamsWebhookURL,
		GoogleChatWebhookURL: cfg.Notify.GoogleChatWebhookURL,
		TelegramBotToken:     cfg.Notify.TelegramBotToken,
		TelegramChatID:       cfg.Notify.TelegramChatID,
		WebhookURL:           cfg.Notify.WebhookURL,
	}
	for _, err := range notify.SendAll(context.Background(), notifyCfg, notify.Message{Version: tagName, Notes: releaseNotes}) {
		logWarn("notification failed: %v", err)
	}
}
