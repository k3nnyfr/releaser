---
title: Configuration
weight: 30
---

`releaser` reads `.releaser.yml` from the repository root. All fields are optional — missing values fall back to the defaults shown below. Run `releaser --init` to scaffold the file with annotations.

## Full reference

```yaml
git:
  tag_prefix: ""                                     # default: no prefix; "v" for v1.2.3 style
  branch_pattern: "^(?:.*/)?release/(\\d+)\\.(\\d+)$"  # two capture groups: major, minor
  commit_message: "chore(release): {version} [skip ci]"
  author_name: ""                                    # defaults to git config user.name
  author_email: ""                                   # defaults to git config user.email

  # Limit which commit types trigger a release (default: all three).
  releasable_types:
    - fix
    - feat
    - breaking

  # Control which version component each commit type bumps.
  # Valid values: "patch" (default) or "minor".
  bump_rules:
    breaking: "patch"
    feat: "patch"
    fix: "patch"

maven:
  pom_path: "pom.xml"                                # single pom.xml, relative to repo root

  # Multi-module: list overrides pom_path.
  # pom_paths:
  #   - "pom.xml"
  #   - "module-a/pom.xml"
  #   - "module-b/pom.xml"

node:                                                # opt-in — omit section to skip
  # package_json: "package.json"                     # single path

  # Monorepo: list overrides package_json.
  # package_jsons:
  #   - "packages/frontend/package.json"
  #   - "packages/backend/package.json"

gradle:                                              # opt-in — omit section to skip
  # build_file: "build.gradle"                       # Groovy or Kotlin DSL; single path

  # Multi-module: list overrides build_file.
  # build_files:
  #   - "build.gradle"
  #   - "module-a/build.gradle"

python:                                              # opt-in — omit section to skip
  # pyproject_toml: "pyproject.toml"                 # PEP 621 [project].version or [tool.poetry].version

  # Monorepo: list overrides pyproject_toml.
  # pyproject_tomls:
  #   - "pyproject.toml"
  #   - "packages/cli/pyproject.toml"

gitlab:
  url: "https://gitlab.example.com"                  # or env CI_SERVER_URL
  token: ""                                          # prefer env GITLAB_TOKEN
  project: ""                                        # prefer env CI_PROJECT_ID or CI_PROJECT_PATH

github:
  token: ""                                          # prefer env GITHUB_TOKEN
  repo: ""                                           # "owner/repo" format
```

{{< hint info >}}
When both `github.*` and `gitlab.*` are configured, GitHub takes precedence.
{{< /hint >}}

## Environment variables

| Variable | Used for |
|----------|----------|
| `GITLAB_TOKEN` | GitLab API auth + HTTPS push auth |
| `CI_SERVER_URL` | GitLab instance URL |
| `CI_PROJECT_ID` | GitLab project identifier (numeric) |
| `CI_PROJECT_PATH` | GitLab project identifier (fallback) |
| `GITHUB_TOKEN` | GitHub API auth |

## Config sources

Run `releaser --verbose --dry-run` to see every config key, its resolved value, and where it came from (`default` / `config file` / `env: VARNAME` / `flag: --name`).

## `git.releasable_types`

By default `fix`, `feat`, and `breaking` commits all trigger a release. Use `releasable_types` to restrict this — for example, on a maintenance branch where you want only bug fixes to release:

```yaml
git:
  releasable_types:
    - fix
```

## `git.bump_rules`

By default every releasable commit bumps the **patch** component. The `bump_rules` map lets you promote specific types to bump **minor** instead. This is useful on a branch that manages its own minor versioning:

```yaml
git:
  bump_rules:
    feat: "minor"      # feat: commits bump minor, not patch
    breaking: "minor"  # breaking changes bump minor too
    fix: "patch"       # fix: stays patch (this is the default)
```

## Multi-module Maven

`pom_paths` accepts a list and overrides `pom_path`. All listed files are updated and committed in the same release commit:

```yaml
maven:
  pom_paths:
    - "pom.xml"
    - "module-a/pom.xml"
    - "module-b/pom.xml"
```

The `--pom` CLI flag sets a single path and clears `pom_paths`.

## Node.js support

The `node` section is opt-in — if omitted, no `package.json` is touched. Use `package_jsons` for monorepos:

```yaml
node:
  package_jsons:
    - "packages/frontend/package.json"
    - "packages/backend/package.json"
```

## Python support

The `python` section is opt-in — if omitted, no `pyproject.toml` is touched. `releaser` reads `[project].version` (PEP 621) first; if not found it falls back to `[tool.poetry].version`. The original file formatting is preserved on write.

```yaml
python:
  pyproject_toml: "pyproject.toml"
```

Use `pyproject_tomls` for monorepos:

```yaml
python:
  pyproject_tomls:
    - "pyproject.toml"
    - "packages/cli/pyproject.toml"
    - "packages/lib/pyproject.toml"
```

The `--pyproject <path>` CLI flag sets a single path and clears `pyproject_tomls`.

## Gradle support

The `gradle` section is opt-in — if omitted, no build file is touched. Both Groovy DSL (`version = '1.2.3'`) and Kotlin DSL (`version = "1.2.3"`) are supported; the original quote style is preserved on write.

```yaml
gradle:
  build_file: "build.gradle"
```

Use `build_files` for multi-module projects:

```yaml
gradle:
  build_files:
    - "build.gradle"
    - "module-a/build.gradle"
    - "module-b/build.gradle"
```

The `--gradle <path>` CLI flag sets a single build file path and clears `build_files`.
