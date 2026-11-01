# GitHub (Public) — Reference

Reference for github.com operations via the GitHub MCP server. For the AI agent's operational rules, see `prompts/github.md`. For resolving merge and rebase conflicts, see the `git` skill.

---

## MCP Server

Use the **github** MCP server (`user-github-*` tools) for all public GitHub operations.

**Do NOT use `mcp_aicodinggithub_*` tools for public GitHub.** Those connect to an internal GitHub Enterprise instance.

### Key Tools

| Tool | Purpose |
|------|---------|
| `user-github-list_pull_requests` / `user-github-search_pull_requests` | Find PRs |
| `user-github-pull_request_read` | Get PR details, diff, status, reviews, comments |
| `user-github-create_pull_request` / `user-github-update_pull_request` | Manage PRs |
| `user-github-merge_pull_request` | Merge PRs |
| `user-github-pull_request_review_write` | Create/submit reviews |
| `user-github-list_issues` / `user-github-search_issues` | Find issues |
| `user-github-issue_write` / `user-github-issue_read` | Manage issues |
| `user-github-get_file_contents` / `user-github-search_code` | Browse code |
| `user-github-list_commits` / `user-github-get_commit` | Browse commits |

---

## Common Operations

### List My Open PRs

```
user-github-search_pull_requests
  query: "author:@me is:open"
  owner: {owner}
  repo: {repo}
```

### Check PR Status

```
user-github-pull_request_read
  method: "get"        # details
  method: "get_status" # build/check status
  method: "get_diff"   # diff
  method: "get_files"  # changed files list
```

### Create a PR

```
user-github-create_pull_request
  owner, repo, title, head, base, body
```

### Review a PR

For complex reviews with line-level comments:
1. `user-github-pull_request_review_write` method: "create" (creates pending review)
2. `user-github-add_comment_to_pending_review` (add line comments)
3. `user-github-pull_request_review_write` method: "submit_pending" (submit)

For simple reviews:
- `user-github-pull_request_review_write` with event: "APPROVE" / "REQUEST_CHANGES" / "COMMENT"

---

## Workflow Integration

### Opening a PR — Add Task

When a PR is opened, add to todo.txt:
```
(C) YYYY-MM-DD @git {PR title} [{owner}/{repo}/PR#{num}](url) +PROJECT
```

### Checking PR Status

When user says "check PR 123":
1. Use `pull_request_read` with method "get" and "get_status"
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
