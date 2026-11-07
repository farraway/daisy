# GitHub (Public) — Reference

Reference for github.com operations via the GitHub MCP server. For the AI agent's operational rules, see `prompts/github.md`. For resolving merge and rebase conflicts, see the `git` skill.

---

## MCP Server

Use the GitHub MCP server's tools for public github.com operations (pull
requests, reviews, issues, code, commits); their own descriptions carry the
parameters, so this reference lists no tool names.

**Never use the `aicodinggithub` server's tools for github.com.** Those
connect to an internal GitHub Enterprise instance.

---

## Workflow Integration

### Opening a PR — Add Task

When a PR is opened, add to todo.txt:
```
(C) YYYY-MM-DD @git {PR title} [{owner}/{repo}/PR#{num}](url) +PROJECT
```

### Checking PR Status

When user says "check PR 123":
1. Read the PR's details and its check status through the server's PR tools
2. Report: state, checks, reviews

### When PR is Merged

1. Mark complete in todo.txt: `x YYYY-MM-DD ...`
2. Log merge in today.md

---

## Best Practices

- **Draft PRs** — use draft status for work-in-progress
- **Small PRs** — easier to review, faster to merge
- **Descriptive commits** — include what and why
- **Clean history** — rebase/squash before merge if needed
- **CI/CD green** — fix all failing checks before requesting review or merging
