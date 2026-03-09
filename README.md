# gh-wrapup

**You cooked. Now wrap it up. 🍽️**

Atomically create a GitHub issue + PR as a single unit of work.

[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![gh extension](https://img.shields.io/badge/gh-extension-8250df)](https://cli.github.com/manual/gh_extension)
[![Release](https://img.shields.io/github/v/release/pdinh/gh-wrapup)](https://github.com/pdinh/gh-wrapup/releases)

---

## The Problem

Creating an issue and a linked PR requires two separate commands with manual URL parsing in between. GitHub has no atomic "unit of work" primitive. In agentic development workflows — where automated systems write code and need to formalize that work — this gap means either brittle multi-step scripts or duplicate issues when a run is retried.

## The Solution

```bash
gh wrapup create \
  --title "Sidebar nav doesn't collapse on mobile" \
  --labels "bug,frontend" \
  --pr-title "fix(nav): collapse sidebar on mobile viewports" \
  --branch "fix/sidebar-mobile"
```

```
✓ Issue #42 created: https://github.com/org/repo/issues/42
✓ Branch 42-fix-sidebar-nav created
✓ PR #43 created: https://github.com/org/repo/pull/43
  └─ Closes #42
```

---

## Install

```bash
gh ext install pdinh/gh-wrapup
```

Requires [gh CLI](https://cli.github.com) 2.0+.

---

## Commands

### `gh wrapup create`

Create a GitHub issue and a linked PR in a single command.

| Flag | Default | Description |
|------|---------|-------------|
| `--title` | _(required)_ | Issue title |
| `--body` | `""` | Issue body |
| `--body-file` | — | Read issue body from file (`-` for stdin) |
| `--labels` | — | Comma-separated labels to apply to the issue |
| `--pr-title` | _(required)_ | PR title |
| `--pr-body` | `""` | PR body (auto-prepended with `Closes #N`) |
| `--branch` | auto | Branch name (default: `{number}-{slugified-title}`) |
| `--base` | `main` | Base branch for the PR |
| `--draft` | `false` | Open PR as draft |
| `--repo` | current repo | Target repo (`owner/repo`) |

**Minimal:**

```bash
gh wrapup create --title "Fix login timeout" --pr-title "fix(auth): increase session TTL"
```

**Full:**

```bash
gh wrapup create \
  --title "API rate limiter drops valid requests under load" \
  --body-file ./issue-body.md \
  --labels "bug,backend,p1" \
  --pr-title "fix(api): tune rate limiter sliding window" \
  --branch "fix/rate-limiter-load" \
  --base "develop" \
  --draft \
  --repo myorg/myapi
```

---

### `gh wrapup upsert`

Idempotent create-or-update. The killer feature for automated workflows.

- Searches for an existing open issue with the same title
- If found: updates it, reuses the issue number
- If a branch with the given name already exists: reuses it, creates or updates the PR
- Safe to call repeatedly — never creates duplicates

```bash
gh wrapup upsert \
  --title "Sidebar nav doesn't collapse on mobile" \
  --pr-title "fix(nav): collapse sidebar on mobile viewports" \
  --branch "fix/sidebar-mobile"
```

Same command, called again after a crash:

```
~ Issue #42 already exists, skipping creation
~ Branch fix/sidebar-mobile already exists, skipping creation
~ PR #43 already exists, updating description
✓ Done: https://github.com/org/repo/pull/43
```

Accepts all the same flags as `create`.

---

### `gh wrapup from-issue`

Create a PR from an issue that already exists.

```bash
gh wrapup from-issue \
  --issue 42 \
  --pr-title "fix(nav): collapse sidebar on mobile viewports" \
  --branch "fix/sidebar-mobile"
```

| Flag | Default | Description |
|------|---------|-------------|
| `--issue` | _(required)_ | Existing issue number |
| `--pr-title` | _(required)_ | PR title |
| `--branch` | auto | Branch name |
| `--base` | `main` | Base branch |
| `--draft` | `false` | Open PR as draft |
| `--repo` | current repo | Target repo (`owner/repo`) |

---

## Why `upsert` matters for agentic development

AI coding agents (Claude Code, Codex, Devin, SWE-agent) operate in retry loops. A task fails, the agent retries from the beginning. With `create`, every retry produces a new issue and a new PR — polluting the repo with duplicates that require manual cleanup.

`upsert` makes the entire workflow idempotent:

```
Agent receives task
  → writes code
  → gh wrapup upsert --title "..." --pr-title "..."
  → Done: PR #43

Agent crashes on the next step and retries
  → writes code again
  → gh wrapup upsert --title "..." --pr-title "..."
  → Done: PR #43  ← same PR, no duplicate
```

The issue title is the natural idempotency key. The branch name is the PR idempotency key. No state file, no external lock, no coordination needed.

---

## Comparison

| Without gh-wrapup | With gh-wrapup |
|---|---|
| `gh issue create` → parse URL → extract number → `gh pr create --body "Closes #N"` | `gh wrapup create --title "..."` |
| 3 commands, manual linking | 1 command, automatic linking |
| Not retry-safe | `upsert` is fully idempotent |
| Agents must parse issue URLs | Structured output, zero parsing |
| Duplicates on retry | No duplicates, ever |

---

## How it works

1. Creates the GitHub issue via REST API, captures the issue number
2. Creates a branch from the base branch HEAD (or reuses an existing branch)
3. Creates a PR with `Closes #N` in the body, linking it to the issue
4. When the PR merges, GitHub automatically closes the issue

No webhooks, no background jobs, no state stored outside GitHub.

---

## Configuration

Zero configuration required. `gh-wrapup` inherits the token from `gh auth`. It works with any repository you have push access to.

- Cross-repo: pass `--repo owner/repo` to any command
- Draft PRs: pass `--draft`
- Pipe body from stdin: `--body-file -`
- CI/automation: set `GH_TOKEN` environment variable as you would for `gh`

---

## Contributing

```bash
git clone https://github.com/pdinh/gh-wrapup
cd gh-wrapup
go build -o gh-wrapup .
go vet ./...
```

PRs welcome. Open an issue first for significant changes.

---

## License

MIT
