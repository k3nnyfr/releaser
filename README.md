# releaser

![release](https://img.shields.io/badge/release-v1.1.0-blue.svg)

A CI-friendly release automation tool for GitFlow workflows using Conventional Commits.

## Problem

Standard tools like `semantic-release` are designed for trunk-based development. In a GitFlow setup with versioned release branches (`release/1.1`, `release/1.2`), they either fail to respect the branch's version range or require brittle configuration.

`releaser` is built for this exact workflow: it reads the branch name to pin the `major.minor`, parses Conventional Commits to determine the patch increment, and handles everything from `pom.xml` update to GitLab tag+release creation.

## How it works

```
release/1.2 branch
  └─ last tag: v1.2.3 (or none → start at v1.2.0)
       └─ commits since tag → Conventional Commits analysis
            └─ next version: v1.2.4
```

1. **Branch parsing** — extracts `major.minor` from branch name (e.g. `release/1.2` → `1.2`)
2. **Tag discovery** — finds the latest tag matching `major.minor.*` on the current branch
3. **Commit analysis** — parses Conventional Commits between last tag and HEAD
4. **Version bump** — increments patch (the minor is owned by the branch)
5. **Release** — updates `pom.xml`, commits, tags, creates GitLab release

## Version bump rules

| Commit type      | Bump    | Notes                                      |
|------------------|---------|--------------------------------------------|
| `fix:`           | patch   |                                            |
| `feat:`          | patch   | minor is pinned to branch                  |
| `feat!:` / `BREAKING CHANGE` | patch | same — branch defines the minor boundary |
| `chore:`, `docs:`, etc. | none |                                        |
| unparseable msg  | none    | non-strict mode: silently ignored          |

## Usage

```bash
# Scaffold a default .releaser.yml in the current repository
releaser --init

# Simulate next version (no side effects)
releaser --dry-run

# Full release: update pom.xml + CHANGELOG.md, commit, tag, push, GitLab release
releaser

# Commit and tag locally — skip push and GitLab release
releaser --no-push

# Push commit and tag but skip creating the GitLab release
releaser --no-release

# Update files but stop before committing (review first)
releaser --no-commit
# … then commit manually and re-run:
releaser --tag-only

# Explicitly target a branch (useful in detached HEAD CI)
releaser --branch release/1.2

# Write changelog to a custom file
releaser --changelog-file CHANGES.md

# Target a specific pom.xml
releaser --pom path/to/pom.xml

# Override tag prefix from CLI (empty = no prefix)
releaser --tag-prefix ""

# Override branch pattern (e.g. also match hotfix/ branches)
releaser --branch-pattern "^(?:.*/)?(?:release|hotfix)/(\d+)\.(\d+)$"
```

## Configuration

`releaser` reads `.releaser.yml` from the repository root. All fields are optional — missing values fall back to the defaults shown below.

```yaml
git:
  tag_prefix: "v"                                    # set to "" for tags without prefix
  branch_pattern: "^(?:.*/)?release/(\\d+)\\.(\\d+)$"  # two capture groups: major, minor
  commit_message: "chore(release): {version} [skip ci]"
  author_name: ""                                    # defaults to git config user.name
  author_email: ""                                   # defaults to git config user.email

maven:
  pom_path: "pom.xml"                                # relative to repo root

gitlab:
  url: "https://gitlab.example.com"                  # or env CI_SERVER_URL
  token: ""                                          # env GITLAB_TOKEN (never commit this)
  project: ""                                        # env CI_PROJECT_ID or CI_PROJECT_PATH
```

### Environment variables

GitLab-related fields are automatically read from the CI environment if not set in the config file:

| Variable          | Used for                          |
|-------------------|-----------------------------------|
| `GITLAB_TOKEN`    | API auth + HTTPS push auth        |
| `CI_SERVER_URL`   | GitLab instance URL               |
| `CI_PROJECT_ID`   | Project identifier (numeric)      |
| `CI_PROJECT_PATH` | Project identifier (fallback)     |

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
```
