---
title: Installation
weight: 10
---

## Pre-built binaries

Download the latest release for your platform from either forge — both publish identical binaries for every tag:

- [git.k3nny.fr releases](https://git.k3nny.fr/k3nny/releaser/releases) (primary)
- [GitHub releases](https://github.com/k3nnyfr/releaser/releases) (mirror)

```bash
# Linux (amd64)
curl -sSL https://git.k3nny.fr/k3nny/releaser/releases/download/v1.10.0/releaser-v1.10.0-linux-amd64 \
  -o /usr/local/bin/releaser
chmod +x /usr/local/bin/releaser

# Same binary from the GitHub mirror
curl -sSL https://github.com/k3nnyfr/releaser/releases/download/v1.10.0/releaser-v1.10.0-linux-amd64 \
  -o /usr/local/bin/releaser
```

Available platforms: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, `windows-amd64.exe`. Asset names embed the tag (`releaser-<tag>-<os>-<arch>`), so download URLs must name an explicit version.

## Docker

```bash
docker pull git.k3nny.fr/k3nny/releaser/releaser:latest

# Run in the current repository
docker run --rm \
  -v "$PWD:/repo" \
  -e GITLAB_TOKEN="$GITLAB_TOKEN" \
  git.k3nny.fr/k3nny/releaser/releaser:latest
```

## Build from source

Requires the Go version declared in `go.mod` (1.26+).

```bash
git clone https://git.k3nny.fr/k3nny/releaser.git   # or https://github.com/k3nnyfr/releaser.git
cd releaser
go build -o /usr/local/bin/releaser ./cmd
```

## Verify

```bash
releaser --version
```

## First run

Scaffold a default `.releaser.yml` in your repository root:

```bash
releaser --init
```

Then do a dry run to check the version that would be produced:

```bash
releaser --dry-run
```
