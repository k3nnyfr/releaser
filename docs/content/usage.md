---
title: CLI Reference
weight: 20
---

## Common workflows

```bash
# Scaffold a default .releaser.yml
releaser --init

# Preview next version (no side effects)
releaser --dry-run

# Full release: bump versions, commit, tag, push, create release
releaser

# Commit and tag locally — skip push and release creation
releaser --no-push

# Push commit and tag but skip creating the release
releaser --no-release

# Update files but stop before committing
releaser --no-commit
# ... review changes, then commit manually and re-run:
releaser --tag-only

# Verbose mode: show config sources, commit analysis, version decision
releaser --verbose --dry-run
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Print next version and exit without making any changes |
| `--branch <name>` | auto-detected | Override branch name (useful in detached HEAD / CI) |
| `--branch-pattern <regex>` | `^(?:.*/)?release/(\d+)\.(\d+)$` | Override branch pattern (two capture groups: major, minor) |
| `--tag-prefix <prefix>` | `""` | Prefix for version tags (e.g. `v` → `v1.2.3`) |
| `--pom <path>` | `pom.xml` | Path to pom.xml relative to repo root |
| `--gradle <path>` | — | Override `gradle.build_file` from config |
| `--pyproject <path>` | — | Override `python.pyproject_toml` from config |
| `--changelog-file <path>` | `CHANGELOG.md` | Path to changelog file |
| `--release-env-file <path>` | `release.env` | Path for dotenv artifact; pass `""` to disable |
| `--no-commit` | false | Update version files but stop before committing |
| `--no-push` | false | Commit and tag locally, skip push and release |
| `--no-release` | false | Push branch and tag but skip release creation |
| `--tag-only` | false | Skip version file updates — tag HEAD and push |
| `--init` | false | Scaffold a default `.releaser.yml` and exit |
| `--verbose` | false | Print config table, commit analysis, and version decision |

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (config, git, API, etc.) |
| `2` | No releasable commits found — nothing to do |

## Version bump rules

By default all releasable commits bump the **patch** component (minor is pinned to the branch). Override per commit type via `git.bump_rules` in `.releaser.yml`:

| Commit type | Default | Configurable via `bump_rules` |
|-------------|---------|-------------------------------|
| `fix:` | patch | `fix: "minor"` to bump minor |
| `feat:` | patch | `feat: "minor"` to bump minor |
| `feat!:` / `BREAKING CHANGE` | patch | `breaking: "minor"` to bump minor |
| `chore:`, `docs:`, etc. | none | — |
| unparseable message | none | non-strict: silently ignored |
