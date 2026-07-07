# Changelog

All notable changes to this project will be documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
