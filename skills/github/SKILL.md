---
name: github
description: Public GitHub PR and issue operations via the github MCP server. Use when checking, reviewing, or creating a PR on public GitHub, asked about PRs/issues/repositories on github.com, or working with GitHub PRs.
short_description: Public GitHub PR and issue operations
---

# GitHub (Public) Quick Reference

Use the **github** MCP server (`user-github-*` tools) for all public GitHub operations. Never use `mcp_aicodinggithub_*` tools for github.com — those connect to an internal GitHub Enterprise instance.

Never merge without green CI. Fix all failing checks before requesting review or merging.

## Key Tools

- `user-github-list_pull_requests` / `user-github-search_pull_requests` — find PRs
- `user-github-pull_request_read` — details, diff, status, reviews, comments
- `user-github-create_pull_request` / `user-github-update_pull_request` — manage PRs
- `user-github-merge_pull_request` — merge PRs
- `user-github-pull_request_review_write` — create/submit reviews
- `user-github-list_issues` / `user-github-search_issues` — find issues
- `user-github-issue_write` / `user-github-issue_read` — manage issues
- `user-github-get_file_contents` / `user-github-search_code` — browse code
- `user-github-list_commits` / `user-github-get_commit` — browse commits

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
1. `user-github-pull_request_review_write` method: "create"
2. `user-github-add_comment_to_pending_review`
3. `user-github-pull_request_review_write` method: "submit_pending"

For simple reviews:
- `user-github-pull_request_review_write` with event: "APPROVE" / "REQUEST_CHANGES" / "COMMENT"

## Full Reference

For workflow integration patterns and best practices, see [`daisy/docs/github.md`](../../daisy/docs/github.md).
