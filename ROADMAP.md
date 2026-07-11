# Roadmap

## v0.1 — Core logic (MVP) ✅

- [x] Branch name parser (`release/X.Y` → major, minor)
- [x] Git tag discovery (latest `vX.Y.*` tag reachable from HEAD — ancestor check, not global latest)
- [x] Conventional Commits parser (non-strict mode: case-insensitive, optional scope, flexible whitespace)
- [x] Commit range walker (last tag → HEAD, or full history on first release)
- [x] Version bump calculator — returns plain `X.Y.Z` (prefix kept separate)
- [x] `--dry-run` flag: print next version and exit
- [x] `--branch` flag: override branch detection (detached HEAD in CI)
- [x] Exit code 2 when no releasable commits

## v0.2 — Config + Maven + local git ops ✅

- [x] `.releaser.yml` config file (loaded from repo root, merged over defaults, missing file is not an error)
- [x] Configurable tag prefix (`tag_prefix: "v"` or `tag_prefix: ""` for no prefix)
- [x] Configurable commit message template (`{version}` placeholder)
- [x] Configurable `pom_path` (default `pom.xml`)
- [x] `--pom` CLI flag: override pom path
- [x] `--tag-prefix` CLI flag: override tag prefix
- [x] `pom.xml` version reader (ignores `<parent>` and dependency versions)
- [x] `pom.xml` version writer (in-place, handles `-SNAPSHOT` → release)
- [x] Git commit of `pom.xml` changes (author resolved from git config, overridable in `.releaser.yml`)
- [x] Git tag creation (lightweight, on the release commit)

## v0.3 — GitLab integration ✅

- [x] GitLab release creation via API (minimal HTTP client, no external dependency)
- [x] Release notes body auto-generated from commit log (grouped by Breaking Changes / Features / Bug Fixes)
- [x] `GITLAB_TOKEN` env var support
- [x] `CI_SERVER_URL` / `CI_PROJECT_ID` / `CI_PROJECT_PATH` env var support (GitLab CI native)
- [x] `--no-push` flag: commit+tag locally without pushing or creating GitLab release
- [x] Git push via HTTPS token auth (`oauth2` + `GITLAB_TOKEN`)
- [x] GitLab section in `.releaser.yml`:
  ```yaml
  gitlab:
    url: "https://gitlab.example.com"
    token: ""      # prefer env GITLAB_TOKEN
    project: ""    # prefer env CI_PROJECT_ID
  ```

## v0.4 — Hardening & CI template ✅

- [x] Configurable branch pattern (`git.branch_pattern` in `.releaser.yml`, default `release/X.Y`, supports regex e.g. `(?:release|hotfix)/X.Y`)
- [x] `--branch-pattern` CLI flag to override
- [x] Dirty working tree check — aborts if tracked files are modified or staged (untracked files ignored)
- [x] `--no-commit` flag: update pom.xml then stop — commit manually and re-run with `--tag-only`
- [x] `--tag-only` flag: skip pom.xml update, tag HEAD and push
- [x] `Dockerfile` — multi-stage build, alpine final image with `ca-certificates`
- [x] `.releaser.gitlab-ci.yml` — reusable CI job template with `GIT_DEPTH: 0` and HTTPS push setup
- [x] `.releaser.yml` template — annotated example of every config option with defaults
- [x] Taskfile — `build`, `run`, `test`, `cov`, `cov:text`, `lint`, `fmt`, `tidy`, `fuzz`, `fuzz:all`, `ci`, `clean`, `docker:build`, `docker:run`
- [x] Gitea CI workflow (vet, staticcheck, test, build)
- [x] Gitea release workflow (5-platform cross-compilation, release asset upload)
- [x] 96% test coverage with real in-memory git repos and fuzz tests for all parsers

## v0.5 — Changelog ✅

- [x] `CHANGELOG.md` generation / append (grouped by commit type: Breaking Changes / Added / Fixed)
- [x] `--changelog-file` flag to use a custom filename
- [x] `--init` flag to scaffold a default `.releaser.yml`
- [x] Push falls back to system `git` CLI when no token is set (uses credential helpers, SSH, netrc)

## v1.0 — Production ready

- [x] ~~Integration tests against a real Git repo (with fixture commits and tags)~~ — ✓ shipped v0.4.0 (96% coverage, real in-memory repos)
- [x] ~~Cross-compilation in CI (linux/amd64, linux/arm64, darwin/amd64)~~ — ✓ shipped v0.4.0 (Gitea release workflow, + darwin/arm64 + windows/amd64)
- [x] ~~`--verbose` flag~~ — ✓ shipped v1.2.0 (shows config sources, commit analysis, version decision)
- [x] ~~Colored, structured CLI output~~ — ✓ shipped v1.2.0 (`·` / `✓` / `!` symbols, `▸` section headers in verbose, TTY-aware ANSI colors)
- [x] ~~Name and version header on every run~~ — ✓ shipped v1.2.0
- [x] ~~Default tag prefix changed to empty~~ — ✓ shipped v1.2.0 (bare `1.2.3` tags by default; opt in to `v` prefix via config)
- [x] ~~`release.env` dotenv artifact~~ — ✓ shipped v1.3.0 (`NEXT_VERSION=<tag>` written on every release for GitLab CI downstream jobs)
- [x] ~~GitHub release support~~ — ✓ shipped v1.4.0 (`internal/ghclient`, `GITHUB_TOKEN` env, `github.token`/`github.repo` config; GitHub takes precedence over GitLab)
- [x] ~~SSH agent push~~ — ✓ shipped v1.4.0 (go-git `gitssh.NewSSHAgentAuth` for `git@`/`ssh://` remotes)
- [x] ~~Configurable bump rules~~ — ✓ shipped v1.4.0 (`git.releasable_types` config; filter which commit types trigger a release)
- [x] ~~Documentation site~~ — ✓ shipped v1.5.1 (Hugo + Geekdoc; installation, CLI reference, configuration, CI integration pages; deployed via Gitea CI to `gh-pages`)

## v1.5 — Multi-module, Node.js, configurable bump rules ✅

- [x] Multi-module Maven support (`maven.pom_paths: [...]` updates multiple `pom.xml` files in one release)
- [x] `package.json` version bump for Node.js projects (`node.package_json` / `node.package_jsons`)
- [x] Configurable bump rules per commit type (`git.bump_rules.breaking/feat/fix: "minor" | "patch"`)
- [x] 100% per-package statement coverage across all 12 packages

## Future / backlog

- ~~Gradle support (`build.gradle` / `build.gradle.kts`)~~ — ✓ shipped v1.6.0 (`internal/gradle`; Groovy + Kotlin DSL; multi-module via `gradle.build_files`; `--gradle` flag)
- Slack / Teams notification on release
