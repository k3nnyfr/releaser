---
title: Installation
weight: 10
---

## Pre-built binaries

Download the latest release for your platform from the [Releases page](https://git.k3nny.fr/releaser/releases).

```bash
# Linux (amd64)
curl -sSL https://git.k3nny.fr/releaser/releases/download/v1.5.0/releaser-v1.5.0-linux-amd64 \
  -o /usr/local/bin/releaser
chmod +x /usr/local/bin/releaser
```

Available platforms: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, `windows-amd64.exe`.

## Docker

```bash
docker pull git.k3nny.fr/releaser/releaser:latest

# Run in the current repository
docker run --rm \
  -v "$PWD:/repo" \
  -e GITLAB_TOKEN="$GITLAB_TOKEN" \
  git.k3nny.fr/releaser/releaser:latest
```

## Build from source

Requires Go 1.21+.

```bash
git clone https://git.k3nny.fr/releaser/releaser.git
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
