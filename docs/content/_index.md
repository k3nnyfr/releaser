---
title: releaser
---

**CI-friendly release automation for GitFlow workflows using Conventional Commits.**

Standard tools like `semantic-release` are designed for trunk-based development. In a GitFlow setup with versioned release branches (`release/1.1`, `release/1.2`), they either fail to respect the branch's version range or require brittle configuration.

`releaser` is built for this exact workflow: it reads the branch name to pin the `major.minor`, parses Conventional Commits to determine the patch increment, and handles everything from `pom.xml` / `package.json` update to GitLab/GitHub tag and release creation.

## How it works

```
release/1.2 branch
  └─ last tag: 1.2.3 (or none → start at 1.2.0)
       └─ commits since tag → Conventional Commits analysis
            └─ next version: 1.2.4
```

1. **Branch parsing** — extracts `major.minor` from branch name (`release/1.2` → `1.2`)
2. **Tag discovery** — finds the latest tag matching `major.minor.*` on the current branch
3. **Commit analysis** — parses Conventional Commits between last tag and HEAD
4. **Version bump** — increments patch (or minor, if configured via `bump_rules`)
5. **Release** — updates `pom.xml` / `package.json`, commits, tags, creates GitLab or GitHub release
