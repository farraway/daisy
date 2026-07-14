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

For the GitHub operations themselves (tools, PR/issue mechanics, review workflow), use the `github` skill.
