## Trigger

Read the full `$DAISY_ROOT/prompts/github.md` when:
- User asks to check, review, or create a PR on public GitHub
- User mentions @git or @github context tasks
- User asks about PRs, issues, or repositories on github.com
- User says "check PR", "review PR", or "open PR"
- Working with GitHub PRs section in today.md

## Rules

1. **Add a task when a PR is opened.** Create a todo.txt entry with `@git` context: `(C) YYYY-MM-DD @git {title} [{owner}/{repo}/PR#{num}](url) +PROJECT`.
2. **Mark complete and log when a PR is merged.** Mark the `@git` task done in todo.txt and log the merge in today.md.
3. **Never merge without green CI.** Fix all failing checks before requesting review or merging.
4. **Use the `git` skill for conflicts.** When a merge or rebase conflict blocks a PR, resolve it via the `git` skill rather than improvising.

# GitHub PR Workflow

Opening a PR is not just a GitHub operation — it creates a tracked
commitment. The task entry and the merge log are part of what "open a PR"
means here, which is why they live with the operations rather than
somewhere else.

## Operations

Use the GitHub MCP server's tools for public github.com operations (pull
requests, reviews, issues, code, commits); their own descriptions carry the
parameters. Never use the `aicodinggithub` server's tools for github.com —
those connect to an internal GitHub Enterprise instance.

Full workflow integration patterns: [`docs/github.md`](docs/github.md).
