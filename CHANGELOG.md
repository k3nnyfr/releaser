# Changelog

All notable changes to this project will be documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.5.0] - 2026-07-11

### Added

- **Multi-module Maven support** — `maven.pom_paths: [...]` lists multiple `pom.xml` paths (e.g. root + sub-modules); overrides `pom_path`; each path is updated and committed in the same release commit
- **Node.js `package.json` support** — opt-in via `node.package_json` (single path) or `node.package_jsons` (list, overrides the single path); version is bumped in-place alongside `pom.xml` and `CHANGELOG.md`
- **`git.bump_rules` config** — controls which version component each commit type bumps: `breaking`, `feat`, `fix` each accept `"patch"` (default) or `"minor"`; allows e.g. `feat: "minor"` to bump the minor component instead of patch
- **Injectable function vars in `cmd/main.go`** — `absPath`, `gitAllCommits`, `gitCommitsSince`, `gitCommitFiles` are now package-level vars overridable in tests to inject errors, enabling 100% per-package statement coverage across all 12 packages

### Changed

- **`--pom` flag** now clears `maven.pom_paths` before setting `maven.pom_path`, ensuring the CLI flag always wins over a multi-path config file entry
- **`version.Next()` signature** — accepts a `map[commits.Type]semver.BumpLevel` bump-rules map as a sixth parameter; `nil` defaults to all-patch (no change to existing behaviour)
- **Verbose config table** — now includes `git.bump_rules.breaking/feat/fix`, `maven.pom_paths` (effective list), and `node.paths` rows

## [1.4.0] - 2026-07-11

### Added

- **GitHub release support** — new `internal/ghclient` package (minimal HTTP client, no SDK); configured via `github.token` + `github.repo` in `.releaser.yml` or `GITHUB_TOKEN` env var; GitHub takes precedence over GitLab when both are configured
- **SSH agent push** — `gitutil.Push()` now attempts go-git SSH agent auth (`gitssh.NewSSHAgentAuth`) for `git@` / `ssh://` remotes before falling back to the system `git` binary; no extra configuration needed
- **`--release-env-file` flag** — override the dotenv artifact path (relative to repo root; default `release.env`); pass `""` to disable writing the file entirely (e.g. for local runs)
- **`git.releasable_types` config** — opt-in list of commit types that count as releasable (`fix`, `feat`, `breaking`); defaults to all three; useful for maintenance branches where some types should not trigger a release
- **`commits.Group()`, `ExtractSubject()`, `ReleasableSet()`** — exported helpers in `internal/commits`; shared by `notes` and `changelog`, eliminating duplicated grouping and subject-extraction logic
- **CHANGELOG deduplication guard** — `changelog.Update()` is now idempotent; skips the write if a `## [version]` section already exists, preventing duplicate entries on CI reruns
- **Publisher interface** — `releasePublisher` interface + `buildPublisher()` in `cmd/main.go`; GitLab and GitHub are now interchangeable backends; new providers can be added without touching the orchestration logic
- **`artifacts: reports: dotenv: release.env`** in `.releaser.gitlab-ci.yml` — exposes `NEXT_VERSION` to downstream GitLab CI jobs out of the box

### Changed

- **Always load config sources** — `LoadWithSources()` is now called unconditionally instead of only in `--verbose` mode; single code path, no behavioural difference
- **`version.Next()` signature** — accepts a `map[commits.Type]bool` releasable set as a fifth parameter; `nil` defaults to all three types (no change to existing behaviour)
- **Verbose config table** — now includes `git.releasable_types`, `github.token`, and `github.repo` rows

## [1.3.0] - 2026-07-07

### Added

- **`release.env` dotenv artifact** — on every real release (not `--dry-run`) a `release.env` file is written to the repository root containing `NEXT_VERSION=<tag>`; the file is never committed, allowing GitLab CI to expose it as a dotenv artifact and pass the version to downstream jobs (deploy, notify, etc.)

## [1.2.0] - 2026-07-07

### Added

- **`--verbose` flag** — prints a configuration table (each key, its value, and source: `default` / `config file` / `env: VARNAME` / `flag: --name`), lists every commit since the last tag with its parsed type and bump decision, and shows the final version choice; all output goes to stderr
- **Colored, structured CLI output** — progress lines use `·` / `✓` / `!` prefix symbols; `--verbose` mode uses `▸` section headers (configuration / branch / commits / version); commit type column colored by kind (cyan=feat, green=fix, red=breaking); config source tags colored; respects `NO_COLOR` and `TERM=dumb`; auto-disabled when stderr is not a TTY
- **Name and version header** — `releaser  vX.Y.Z` printed to stderr at the start of every invocation

### Changed

- **Default `tag_prefix` is now empty** — tags are bare version numbers (`1.2.3`) by default; add `tag_prefix: "v"` to `.releaser.yml` or pass `--tag-prefix v` to opt in to the `v`-prefixed convention

## [1.1.0] - 2026-07-07

### Added

- **CHANGELOG.md auto-update** — every release now writes a new dated section to `CHANGELOG.md` (grouped by Breaking Changes / Added / Fixed), committed alongside `pom.xml` in the release commit; file is created if it does not exist
- **`--changelog-file` flag** — override the default `CHANGELOG.md` path (e.g. `--changelog-file CHANGES.md`)
- **`--init` flag** — scaffolds a fully-commented default `.releaser.yml` in the repository root; errors if the file already exists
- **`CommitFiles`** in `gitutil` — internal helper that stages multiple files before a single commit, used to bundle `pom.xml` + `CHANGELOG.md` in one release commit

### Fixed

- **Push without token** — go-git's HTTPS transport does not use the system credential store; when `GITLAB_TOKEN` is unset the push now delegates to the `git` CLI so credential helpers, SSH agents, and `netrc` all work as expected

## [0.4.2] - 2026-07-07

### Fixed

- **CI build step** — `go build ./cmd/...` failed with "output already exists and is a directory" because Go tried to write a binary named `cmd`, conflicting with the source directory; fixed by passing `-o /dev/null`
- **Optional pom.xml** — releaser no longer fails when `pom.xml` (or the configured `maven.pom_path`) does not exist; it logs a notice and proceeds directly to tag and push, making the tool usable in non-Maven projects

## [0.4.0] - 2026-07-07

### Added

- **Branch parser** — extracts `major.minor` from `release/X.Y` branch names using a configurable regex pattern
- **Git tag discovery** — finds the latest `vX.Y.*` tag that is a reachable ancestor of HEAD (ignores tags on other version lines or unrelated branches)
- **Conventional Commits parser** — non-strict mode: case-insensitive type, optional scope, flexible whitespace; unrecognised messages are silently skipped
- **Commit range walker** — scans from last matching tag to HEAD; falls back to full history on first release in a branch
- **Version calculator** — increments patch; returns nothing when there are no releasable commits (exit code 2 so CI can distinguish "nothing to do" from errors)
- **`--dry-run` flag** — prints next version without making any changes
- **`--branch` flag** — overrides automatic branch detection for detached-HEAD CI environments
- **`--no-push` flag** — commits and tags locally without pushing or creating a GitLab release
- **`--no-commit` flag** — updates `pom.xml` and stops; re-run with `--tag-only` after manual review
- **`--tag-only` flag** — tags HEAD without touching `pom.xml` (pair with `--no-commit` flow)
- **Dirty working-tree check** — aborts before any changes if tracked files are modified or staged
- **`.releaser.yml` config** — optional YAML loaded from repo root; missing file is not an error; all fields have sensible defaults
- **Configurable tag prefix** — `git.tag_prefix` in config or `--tag-prefix` flag (default: `v`)
- **Configurable branch pattern** — `git.branch_pattern` in config or `--branch-pattern` flag (default: `^(?:.*/)?release/(\d+)\.(\d+)$`)
- **Configurable commit message** — `git.commit_message` with `{version}` placeholder (default: `chore(release): {version} [skip ci]`)
- **Configurable commit author** — `git.author_name` / `git.author_email`; falls back to repo git config
- **Configurable `pom_path`** — `maven.pom_path` in config or `--pom` flag (default: `pom.xml`)
- **`pom.xml` reader / writer** — in-place version update; correctly ignores `<parent>` and dependency version elements
- **GitLab release creation** — minimal HTTP client (no SDK); auto-generates release notes grouped by Breaking Changes / Features / Bug Fixes
- **HTTPS push with token auth** — `oauth2` + `GITLAB_TOKEN`
- **`GITLAB_TOKEN`, `CI_SERVER_URL`, `CI_PROJECT_ID`, `CI_PROJECT_PATH`** env var support for GitLab CI environments
- **`Dockerfile`** — multi-stage build, Alpine final image with `ca-certificates`
- **`.releaser.gitlab-ci.yml`** — reusable GitLab CI job template (`extends: .releaser:release`) with `GIT_DEPTH: 0` and HTTPS push pre-wired
- **`.releaser.yml` template** — annotated example of every config option with its default value
- **Gitea CI workflow** — runs `vet`, `staticcheck`, `test`, `build` on every push
- **Gitea release workflow** — cross-compiles five platform binaries (linux/amd64+arm64, darwin/amd64+arm64, windows/amd64) and publishes them as release assets on tag push
- **Taskfile** — `build`, `run`, `test`, `cov`, `cov:text`, `lint`, `fmt`, `tidy`, `fuzz`, `fuzz:all`, `ci`, `clean`, `docker:build`, `docker:run`
- **96% test coverage** — integration tests use real in-memory git repositories; fuzz tests cover all parser entry-points
