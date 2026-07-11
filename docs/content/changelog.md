---
title: Changelog
weight: 50
---

## v1.5.0 — 2026-07-11

### Added

- **Multi-module Maven support** — `maven.pom_paths: [...]` lists multiple `pom.xml` paths; overrides `pom_path`; each path is updated and committed in the same release commit
- **Node.js `package.json` support** — opt-in via `node.package_json` (single path) or `node.package_jsons` (list); version bumped in-place alongside `pom.xml` and `CHANGELOG.md`
- **`git.bump_rules` config** — controls which version component each commit type bumps: `breaking`, `feat`, `fix` each accept `"patch"` (default) or `"minor"`
- **100% per-package statement coverage** across all 12 packages via injectable function vars

### Changed

- **`--pom` flag** now clears `maven.pom_paths` before setting `maven.pom_path`
- **`version.Next()` signature** — accepts a bump-rules map as a sixth parameter; `nil` defaults to all-patch
- **Verbose config table** — now includes `git.bump_rules.*`, `maven.pom_paths`, and `node.paths` rows

## v1.4.0 — 2026-07-11

### Added

- **GitHub release support** — `internal/ghclient` package; configured via `github.token` + `github.repo`; GitHub takes precedence over GitLab when both are configured
- **SSH agent push** — go-git `gitssh.NewSSHAgentAuth` for `git@` / `ssh://` remotes
- **`--release-env-file` flag** — override dotenv artifact path; pass `""` to disable
- **`git.releasable_types` config** — opt-in list of commit types that count as releasable
- **CHANGELOG deduplication guard** — `changelog.Update()` is idempotent; skips write if section already exists

## v1.3.0 — 2026-07-07

### Added

- **`release.env` dotenv artifact** — written on every real release containing `NEXT_VERSION=<tag>`; never committed; exposes the version to downstream GitLab CI jobs

## v1.2.0 — 2026-07-07

### Added

- **`--verbose` flag** — prints config table, commit list with parsed types, and version decision
- **Colored, structured CLI output** — `·` / `✓` / `!` prefix symbols; TTY-aware ANSI colors; respects `NO_COLOR` and `TERM=dumb`
- **Name and version header** on every invocation

### Changed

- **Default `tag_prefix` is now empty** — bare version numbers (`1.2.3`) by default; add `tag_prefix: "v"` to opt in

## v1.1.0 — 2026-07-07

### Added

- **CHANGELOG.md auto-update** — new dated section written on every release, grouped by commit type
- **`--changelog-file` flag** — override changelog path
- **`--init` flag** — scaffolds a fully-commented `.releaser.yml`

## v1.0 and earlier

See the [full CHANGELOG](https://git.k3nny.fr/releaser/src/branch/main/CHANGELOG.md) in the repository.
