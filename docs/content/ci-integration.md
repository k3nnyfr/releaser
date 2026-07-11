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

## Detached HEAD

In CI environments where `git checkout` leaves the repository in detached HEAD state, pass the branch name explicitly:

```yaml
script:
  - releaser --branch "$CI_COMMIT_BRANCH"
```

## SSH push

When pushing over SSH (`git@host:...` or `ssh://...` remotes), `releaser` attempts go-git SSH agent auth automatically — no extra configuration needed as long as the CI runner has an SSH agent socket available.

For HTTPS remotes without a token, `releaser` delegates to the system `git` binary so credential helpers and `netrc` work as expected.
