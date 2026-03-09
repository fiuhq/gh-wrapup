# gh-wrapup

**You cooked. Now wrap it up. 🍽️**

Atomically create a GitHub issue + PR as a single unit of work.

[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![gh extension](https://img.shields.io/badge/gh-extension-8250df)](https://cli.github.com/manual/gh_extension)
[![Release](https://img.shields.io/github/v/release/fiuhq/gh-wrapup)](https://github.com/fiuhq/gh-wrapup/releases)

---

## The Problem

Creating an issue and a linked PR requires two separate commands with manual URL parsing in between. GitHub has no atomic "unit of work" primitive. In agentic development workflows — where automated systems write code and need to formalize that work — this gap means either brittle multi-step scripts or duplicate issues when a run is retried.

## The Solution

One command. Four modes. No duplicates.

```bash
gh wrapup upsert \
  --title "Sidebar nav doesn't collapse on mobile" \
  --labels "bug,frontend" \
  --pr-title "fix(nav): collapse sidebar on mobile viewports" \
  --branch "fix/sidebar-mobile"
```

```
✓ Issue #42 created: https://github.com/org/repo/issues/42
✓ Branch fix/sidebar-mobile created
✓ PR #43 created: https://github.com/org/repo/pull/43
  └─ Closes #42
```

---

## Install

```bash
gh ext install fiuhq/gh-wrapup
```

Requires [gh CLI](https://cli.github.com) 2.0+.

---

## Commands

### `gh wrapup upsert`

Idempotent create-or-update. The single command for all issue + PR workflows.

Operates in four modes depending on which flags are provided:

| Mode | Flags | Behavior |
|------|-------|----------|
| **1** (default) | `--title` | Search-or-create issue, create-or-update branch + PR |
| **2** (existing issue) | `--issue N` | Fetch issue N, create branch + PR with `Closes #N` |
| **3** (existing PR) | `--title` + `--pr N` | Create issue, prepend `Closes #N` to existing PR body |
| **4** (link both) | `--issue N` + `--pr M` | Prepend `Closes #N` to existing PR body. No branch created. |

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--title` | — | Issue title (required unless `--issue` is set) |
| `--body` | `""` | Issue body |
| `--body-file` | — | Read issue body from file (`-` for stdin) |
| `--labels` | — | Comma-separated labels for the issue |
| `--assignee` | — | Issue assignee (GitHub username) |
| `--milestone` | — | Milestone name or number |
| `--issue` | — | Use existing issue number instead of creating one |
| `--pr-title` | issue title | PR title |
| `--pr-body` | `""` | Additional PR body text |
| `--branch` | auto | Branch name (default: `{number}-{slugified-title}`) |
| `--base` | repo default | Base branch for the PR |
| `--draft` | `false` | Create PR as draft |
| `--pr` | — | Use existing PR number instead of creating one |
| `--repo` | current repo | Target repository (`owner/repo`) |
| `--issue-search` | — | Custom search query to find existing issue |
| `--json` | `false` | Output result as JSON |

#### Examples

**Mode 1 — Full atomic (issue + branch + PR):**

```bash
gh wrapup upsert \
  --title "API rate limiter drops valid requests under load" \
  --body-file ./issue-body.md \
  --labels "bug,backend,p1" \
  --pr-title "fix(api): tune rate limiter sliding window" \
  --branch "fix/rate-limiter-load" \
  --base "develop" \
  --draft \
  --repo myorg/myapi
```

**Mode 2 — From existing issue:**

```bash
gh wrapup upsert --issue 42 --pr-title "fix(nav): collapse sidebar on mobile viewports"
```

```
~ Found issue #42: Sidebar nav doesn't collapse on mobile
✓ Branch 42-sidebar-nav-doesnt-collapse created
✓ PR #43 created: https://github.com/org/repo/pull/43
  └─ Closes #42
```

**Mode 3 — Create issue, link to existing PR:**

```bash
gh wrapup upsert --title "Sidebar nav doesn't collapse on mobile" --pr 43
```

```
✓ Issue #42 created: https://github.com/org/repo/issues/42
✓ PR #43 updated: https://github.com/org/repo/pull/43
  └─ Closes #42
```

**Mode 4 — Link both existing:**

```bash
gh wrapup upsert --issue 42 --pr 43
```

```
~ Found issue #42: Sidebar nav doesn't collapse on mobile
~ PR #43 already linked to issue #42
  └─ Closes #42
```

#### JSON output

Pass `--json` to get structured output for scripting or agent workflows:

```bash
gh wrapup upsert --title "Fix login timeout" --pr-title "fix(auth): increase session TTL" --json
```

```json
{
  "issue": {
    "number": 42,
    "url": "https://github.com/org/repo/issues/42"
  },
  "pr": {
    "number": 43,
    "url": "https://github.com/org/repo/pull/43"
  },
  "created": true
}
```

---

## Why `upsert` matters for agentic development

AI coding agents operate in retry loops. A task fails, the agent retries from the beginning. With a bare `create`, every retry produces a new issue and a new PR — polluting the repo with duplicates that require manual cleanup. And in retroactive linking scenarios (agent already pushed code before formalizing the issue), you needed two separate commands.

`upsert` handles all four scenarios from a single call:

```
Agent receives task
  → writes code
  → gh wrapup upsert --title "..." --pr-title "..."
  → Done: PR #43

Agent crashes on the next step and retries
  → writes code again
  → gh wrapup upsert --title "..." --pr-title "..."
  → Done: PR #43  ← same PR, no duplicate

Agent pushed code first, needs to formalize after
  → gh wrapup upsert --title "..." --pr 43
  → Issue #42 created, PR #43 updated with Closes #42

Agent recovers from crash mid-linking
  → gh wrapup upsert --issue 42 --pr 43
  → PR #43 updated with Closes #42
```

The issue title is the natural idempotency key. The branch name is the PR idempotency key. No state file, no external lock, no coordination needed.

---

## Comparison

| Without gh-wrapup | With gh-wrapup |
|---|---|
| `gh issue create` → parse URL → `gh pr create --body "Closes #N"` | `gh wrapup upsert --title "..."` |
| 2–3 commands, manual linking | 1 command, automatic linking |
| Not retry-safe | Fully idempotent in all 4 modes |
| Agents must parse issue URLs | `--json` flag, zero parsing |
| Duplicates on retry | No duplicates, ever |
| Separate commands for each scenario | Single command handles all scenarios |

---

## How it works

1. Resolves or creates a GitHub issue via REST API, capturing the issue number
2. Creates a branch from the base branch HEAD (or reuses an existing branch)
3. Creates a PR with `Closes #N` in the body, linking it to the issue
4. When the PR merges, GitHub automatically closes the issue

No webhooks, no background jobs, no state stored outside GitHub.

---

## Configuration

Zero configuration required. `gh-wrapup` inherits the token from `gh auth`. It works with any repository you have push access to.

- Cross-repo: pass `--repo owner/repo`
- Draft PRs: pass `--draft`
- Pipe body from stdin: `--body-file -`
- CI/automation: set `GH_TOKEN` environment variable as you would for `gh`

---

## Contributing

```bash
git clone https://github.com/fiuhq/gh-wrapup
cd gh-wrapup
go build -o gh-wrapup .
go vet ./...
```

PRs welcome. Open an issue first for significant changes.

---

## License

MIT
