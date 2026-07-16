---
title: CI Integration
weight: 40
---

## GitLab CI

The simplest setup uses the reusable job template shipped alongside `releaser`:

```yaml
# .gitlab-ci.yml
include:
  - project: releaser/releaser
    file: .releaser.gitlab-ci.yml

release:
  extends: .releaser
  variables:
    GITLAB_TOKEN: $RELEASE_TOKEN   # project/group variable with api + write_repository scope
```

Or write it inline:

```yaml
release:
  stage: release
  image: registry.example.com/releaser:latest
  rules:
    - if: $CI_COMMIT_BRANCH =~ /^release\/.+$/
  variables:
    GITLAB_TOKEN: $RELEASE_TOKEN
    GIT_DEPTH: 0                   # full history — shallow clones hide previous release tags
  script:
    - releaser
  artifacts:
    reports:
      dotenv: release.env          # exposes NEXT_VERSION to downstream jobs
```

### Consuming `NEXT_VERSION` downstream

The `release.env` dotenv artifact exports `NEXT_VERSION=<tag>` automatically. Downstream jobs can use it:

```yaml
deploy:
  stage: deploy
  needs:
    - job: release
      artifacts: true
  script:
    - echo "Deploying version $NEXT_VERSION"
```

Disable the dotenv artifact (e.g. for local runs):

```bash
releaser --release-env-file ""
```

Write it to a custom path:

```bash
releaser --release-env-file deploy/version.env
```

## GitHub Actions / Gitea Actions

```yaml
name: release
on:
  push:
    branches:
      - 'release/**'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0          # full history needed for tag discovery

      - name: Run releaser
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          curl -sSL https://git.k3nny.fr/k3nny/releaser/releases/latest/download/releaser-linux-amd64 \
            -o /usr/local/bin/releaser
          chmod +x /usr/local/bin/releaser
          releaser
```

{{< hint warning >}}
`fetch-depth: 0` is required. A shallow clone (`--depth 1`) hides the previous tag, causing `releaser` to treat every commit as the first release.
{{< /hint >}}

## Preflight checks

`releaser --check` validates the release environment without releasing anything: branch resolution and pattern match, working tree state, shallow clone, remote URL parseability (the same parse the push performs — it catches shell-quoting accidents in `set-url` lines), which push auth would be used, configured version files, and the release target. All problems are reported at once, and the exit code is non-zero if any check fails.

Note the split with `--dry-run`: dry-run answers *"what version would be released?"* (it analyzes commits and computes the bump); `--check` answers *"will the release plumbing work?"* (everything dry-run never touches). Run `--check` in merge-request pipelines to catch broken CI configuration before it blocks a real release:

```yaml
release:check:
  stage: test
  image: registry.example.com/releaser:latest
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  variables:
    GIT_DEPTH: 0
  script:
    - releaser --check --branch "release/0.0"   # any pattern-matching name works for validation
```

## Shallow clones

GitLab CI checks out a shallow clone by default (`GIT_DEPTH: 20`), and shallow clones hide any release tag beyond the fetch depth — tag discovery would silently restart versioning at `X.Y.0`. `releaser` detects this: when the clone is shallow **and** no previous release tag is found, it refuses to release and asks for full history.

Fix it by fetching full history (`GIT_DEPTH: 0` in GitLab CI, `fetch-depth: 0` in GitHub Actions, or `git fetch --unshallow`). If the project genuinely has no release tag yet, pass `--allow-shallow` to release anyway.

A shallow clone whose history does include the latest release tag is fine — the version calculation is unaffected, and `releaser` proceeds normally.

## Detached HEAD

CI runners check out a commit SHA, leaving the repository in detached HEAD state. `releaser` detects this and falls back to the branch name from the CI environment, in order:

1. `CI_COMMIT_BRANCH` (GitLab CI, branch pipelines)
2. `CI_COMMIT_REF_NAME` (GitLab CI)
3. `GITHUB_REF_NAME` (GitHub Actions)

So on branch pipelines no extra configuration is needed. To override the detected name (or on runners that set none of these variables), pass it explicitly:

```yaml
script:
  - releaser --branch "$CI_COMMIT_BRANCH"
```

## SSH push

When pushing over SSH (`git@host:...` or `ssh://...` remotes), `releaser` attempts go-git SSH agent auth automatically — no extra configuration needed as long as the CI runner has an SSH agent socket available.

For HTTPS remotes without a token, `releaser` delegates to the system `git` binary so credential helpers and `netrc` work as expected.
