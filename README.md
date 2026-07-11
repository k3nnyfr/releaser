# releaser

<img src="docs/static/images/releaser-logo-128.png" alt="releaser logo" width="128">

![release](https://img.shields.io/badge/release-v1.7.0-blue.svg)

A CI-friendly release automation tool for GitFlow workflows using Conventional Commits.

**[Documentation](https://releaser.k3nny.fr)** · **[Repository](https://git.k3nny.fr/k3nny/releaser)**

## Problem

Standard tools like `semantic-release` are designed for trunk-based development. In a GitFlow setup with versioned release branches (`release/1.1`, `release/1.2`), they either fail to respect the branch's version range or require brittle configuration.

`releaser` is built for this exact workflow: it reads the branch name to pin the `major.minor`, parses Conventional Commits to determine the patch increment, and handles everything from `pom.xml` update to GitLab/GitHub tag+release creation.

## How it works

```
release/1.2 branch
  └─ last tag: 1.2.3 (or none → start at 1.2.0)
       └─ commits since tag → Conventional Commits analysis
            └─ next version: 1.2.4
```

1. **Branch parsing** — extracts `major.minor` from branch name (e.g. `release/1.2` → `1.2`)
2. **Tag discovery** — finds the latest tag matching `major.minor.*` on the current branch
3. **Commit analysis** — parses Conventional Commits between last tag and HEAD
4. **Version bump** — increments patch (the minor is owned by the branch)
5. **Release** — updates `pom.xml` / `package.json` / `build.gradle`, commits, tags, creates GitLab or GitHub release

## Version bump rules

By default, all releasable commits bump the **patch** component (minor is pinned to the branch). You can override this per commit type via `git.bump_rules` in `.releaser.yml`:

| Commit type      | Default | Configurable via `bump_rules`                  |
|------------------|---------|------------------------------------------------|
| `fix:`           | patch   | `fix: "minor"` to bump minor instead           |
| `feat:`          | patch   | `feat: "minor"` to bump minor instead          |
| `feat!:` / `BREAKING CHANGE` | patch | `breaking: "minor"` to bump minor  |
| `chore:`, `docs:`, etc. | none   | —                                       |
| unparseable msg  | none    | non-strict mode: silently ignored              |

## Usage

```bash
# Scaffold a default .releaser.yml in the current repository
releaser --init

# Simulate next version (no side effects)
releaser --dry-run

# Full release: update pom.xml + CHANGELOG.md, commit, tag, push, create release
releaser

# Commit and tag locally — skip push and release creation
releaser --no-push

# Push commit and tag but skip creating the release
releaser --no-release

# Update files but stop before committing (review first)
releaser --no-commit
# … then commit manually and re-run:
releaser --tag-only

# Explicitly target a branch (useful in detached HEAD CI)
releaser --branch release/1.2

# Write changelog to a custom file
releaser --changelog-file CHANGES.md

# Show configuration sources, commit list, and version decision
releaser --verbose --dry-run

# Target a specific pom.xml
releaser --pom path/to/pom.xml

# Override tag prefix from CLI (empty = no prefix)
releaser --tag-prefix ""

# Override branch pattern (e.g. also match hotfix/ branches)
releaser --branch-pattern "^(?:.*/)?(?:release|hotfix)/(\d+)\.(\d+)$"

# Write dotenv artifact to a custom path (or "" to disable)
releaser --release-env-file deploy/version.env
```

## Configuration

`releaser` reads `.releaser.yml` from the repository root. All fields are optional — missing values fall back to the defaults shown below.

```yaml
git:
  tag_prefix: ""                                     # default: no prefix; set to "v" for v-prefixed tags
  branch_pattern: "^(?:.*/)?release/(\\d+)\\.(\\d+)$"  # two capture groups: major, minor
  commit_message: "chore(release): {version} [skip ci]"
  author_name: ""                                    # defaults to git config user.name
  author_email: ""                                   # defaults to git config user.email
  releasable_types:                                  # default: all three
    - fix
    - feat
    - breaking
  bump_rules:                                        # which version component each type bumps
    breaking: "patch"                                # "minor" to bump minor on breaking changes
    feat: "patch"
    fix: "patch"

maven:
  pom_path: "pom.xml"                                # single pom.xml, relative to repo root
  # pom_paths:                                       # multi-module: list overrides pom_path
  #   - "pom.xml"
  #   - "module-a/pom.xml"
  #   - "module-b/pom.xml"

node:                                                # opt-in — no default; omit to skip
  # package_json: "package.json"                     # single path
  # package_jsons:                                   # monorepo: list overrides package_json
  #   - "packages/frontend/package.json"
  #   - "packages/backend/package.json"

gradle:                                              # opt-in — no default; omit to skip
  # build_file: "build.gradle"                       # Groovy or Kotlin DSL; single path
  # build_files:                                     # multi-module: list overrides build_file
  #   - "build.gradle"
  #   - "module-a/build.gradle"

python:                                              # opt-in — no default; omit to skip
  # pyproject_toml: "pyproject.toml"                 # PEP 621 [project] or [tool.poetry]
  # pyproject_tomls:                                 # monorepo: list overrides pyproject_toml
  #   - "pyproject.toml"
  #   - "packages/cli/pyproject.toml"

gitlab:
  url: "https://gitlab.example.com"                  # or env CI_SERVER_URL
  token: ""                                          # env GITLAB_TOKEN (never commit this)
  project: ""                                        # env CI_PROJECT_ID or CI_PROJECT_PATH

github:
  token: ""                                          # env GITHUB_TOKEN (never commit this)
  repo: ""                                           # "owner/repo" format
```

### Environment variables

| Variable           | Used for                          |
|--------------------|-----------------------------------|
| `GITLAB_TOKEN`     | GitLab API auth + HTTPS push auth |
| `CI_SERVER_URL`    | GitLab instance URL               |
| `CI_PROJECT_ID`    | GitLab project identifier (numeric) |
| `CI_PROJECT_PATH`  | GitLab project identifier (fallback) |
| `GITHUB_TOKEN`     | GitHub API auth                   |

When both `github.*` and `gitlab.*` are configured, GitHub takes precedence.

## CI integration (GitLab CI example)

```yaml
release:
  stage: release
  image: registry.example.com/releaser:latest
  rules:
    - if: $CI_COMMIT_BRANCH =~ /^release\/.+$/
  variables:
    GITLAB_TOKEN: $RELEASE_TOKEN   # project/group CI variable with api + write_repository scope
  script:
    - releaser
  artifacts:
    reports:
      dotenv: release.env          # exposes NEXT_VERSION to downstream jobs
```
