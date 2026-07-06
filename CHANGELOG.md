# Changelog

All notable changes to this project will be documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
