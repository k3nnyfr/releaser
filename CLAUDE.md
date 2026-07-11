# CLAUDE.md — project guidelines for releaser

## Overview

`releaser` is a single-binary Go tool for GitFlow-based release automation. It targets Conventional Commits, versioned release branches (`release/X.Y`), and GitLab / GitHub release creation.

## Architecture

```
cmd/main.go          — CLI entrypoint (cobra), run() pipeline, verbose output
internal/branch/     — branch name parser → major/minor
internal/changelog/  — CHANGELOG.md writer
internal/commits/    — Conventional Commits parser (non-strict)
internal/config/     — .releaser.yml loader + env var overlay + source tracking
internal/ghclient/   — minimal GitHub Releases API client
internal/gitutil/    — go-git helpers: tag discovery, commit walker, push
internal/glclient/   — minimal GitLab Releases API client
internal/maven/      — pom.xml version reader/writer
internal/node/       — package.json version reader/writer
internal/notes/      — release notes body generator
internal/version/    — semver next-version calculator
```

## Code conventions

- **No third-party test frameworks** — stdlib `testing` only.
- **No interfaces for mocking** — inject function variables (`var absPath = filepath.Abs`) to test error paths.
- **No comments explaining what** — only comments explaining *why* (hidden constraints, invariants, non-obvious workarounds).
- **No error handling for impossible paths** — trust internal invariants; only validate at system boundaries.
- **No abstractions ahead of need** — three similar lines beats a premature helper.

## Test coverage

**100% per-package statement coverage is required** across all packages. Run:

```bash
go test ./... -cover
```

Every package must show `coverage: 100.0% of statements`.

Strategies used in this project:
- **Error path injection**: override `var absPath`, `var gitAllCommits`, etc. to return injected errors.
- **Filesystem tricks**: `os.Mkdir` where a file is expected (invisible to go-git dirty check; fails os.WriteFile/os.ReadFile); `os.Chmod(..., 0444)` to make files read-only.
- **Null byte paths**: `"path\x00name"` causes `os.Stat` to return `EINVAL` (not `ErrNotExist`), useful for testing stat-error paths that differ from file-not-found.
- **In-memory git repos**: use go-git `PlainInit` + local bare remote for push tests.
- **Direct function calls**: call unexported helpers (e.g. `printVerboseConfig`) directly with crafted inputs to cover branches that are dead via normal CLI flow.

## Fuzzing

**Every package that parses free-form text or reads/writes arbitrary file content must have at least one fuzz test.** Run the full seed corpus with:

```bash
go test -run='^Fuzz' ./...
```

All seed cases must pass. The table below is authoritative — keep it in sync when adding packages or parsers:

| Package | Fuzz target(s) | Why |
|---------|---------------|-----|
| `internal/branch` | `FuzzParse` | parses branch name strings |
| `internal/changelog` | `FuzzUpdate` | rewrites arbitrary existing file content |
| `internal/commits` | `FuzzParse` | parses arbitrary commit message strings |
| `internal/glclient` | `FuzzEncodeProjectPath` | encodes arbitrary project path strings |
| `internal/maven` | `FuzzReadVersion`, `FuzzReplaceProjectVersion` | reads/rewrites arbitrary XML file content |
| `internal/node` | `FuzzReadVersion`, `FuzzWriteVersion` | reads/rewrites arbitrary JSON file content |
| `internal/notes` | `FuzzGenerate` | generates notes from arbitrary commit messages |

Packages **not** requiring fuzz tests (no free-form text parsing): `internal/config` (yaml.v3 handles parsing), `internal/ghclient` (HTTP client, no text parsing), `internal/gitutil` (git operations), `internal/version` (typed inputs only), `cmd` (CLI orchestration).

Fuzz seed corpus guidelines:
- Include a realistic happy-path input as the first seed.
- Include empty string, binary/non-UTF-8 bytes (`"\x00\xff"`), and inputs that stress known branches (e.g. existing `## [version]` heading for changelog).
- The fuzz body must never assert on return values — only verify no panic.

## Dependency rules

- **No new external dependencies** unless absolutely necessary. The project deliberately avoids pulling in large ecosystems.
- go-git (`github.com/go-git/go-git/v5`) for all git operations.
- cobra for CLI parsing.
- gopkg.in/yaml.v3 for config.

## Config design

- New config fields go in the appropriate `*Config` struct in `internal/config/config.go`.
- `defaultSources()` must be updated to include every new key.
- `LoadWithSources` overlay detection must cover every new field.
- `printVerboseConfig` in `cmd/main.go` must show every new config value.

## Multi-value config pattern

When a config supports both a single value and multiple values (like `pom_path` / `pom_paths`):
- Single field: `PomPath string`
- Multi field: `PomPaths []string`
- `EffectivePomPaths()` method: `PomPaths` wins if non-empty, else `PomPath` if set, else default.
- `--pom` CLI flag clears `PomPaths` and sets `PomPath` only.
