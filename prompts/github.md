## Trigger

Read the full `$DAISY_ROOT/prompts/github.md` when:
- User asks to check, review, or create a PR on public GitHub
- User mentions @git or @github context tasks
- User asks about PRs, issues, or repositories on github.com
- User says "check PR", "review PR", or "open PR"
- Working with GitHub PRs section in today.md

## Rules

1. **Use `user-github-*` tools for public GitHub.** Never use `mcp_aicodinggithub_*` tools for github.com — those connect to an internal GitHub Enterprise instance.
2. **Add a task when a PR is opened.** Create a todo.txt entry with `@git` context: `(C) YYYY-MM-DD @git {title} [{owner}/{repo}/PR#{num}](url) +PROJECT`.
3. **Mark complete and log when a PR is merged.** Mark the `@git` task done in todo.txt and log the merge in today.md.
4. **Never merge without green CI.** Fix all failing checks before requesting review or merging.

# GitHub (Public) Quick Reference

Quick reference for **github.com** operations via the **github** MCP server.

## MCP Server

Use the **github** MCP server tools for all public GitHub operations. These tools use a `user-github-*` naming convention and handle authentication automatically.

Key tools:
- `user-github-list_pull_requests` / `user-github-search_pull_requests` - Find PRs
- `user-github-pull_request_read` - Get PR details, diff, status, reviews, comments
- `user-github-create_pull_request` / `user-github-update_pull_request` - Manage PRs
- `user-github-merge_pull_request` - Merge PRs
- `user-github-pull_request_review_write` - Create/submit reviews
- `user-github-list_issues` / `user-github-search_issues` - Find issues
- `user-github-issue_write` / `user-github-issue_read` - Manage issues
- `user-github-get_file_contents` / `user-github-search_code` - Browse code
- `user-github-list_commits` / `user-github-get_commit` - Browse commits

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

## Workflow Integration

### Creating PR Task

When user opens a PR, add to todo.txt:
```
(C) YYYY-MM-DD @git {PR title} [{owner}/{repo}/PR#{num}](url) +PROJECT
```

### Checking PR Status

When user says "check PR 123":
1. Use `pull_request_read` with method "get" and "get_status"
2. Report: state, checks, reviews

### When PR is Merged

1. Mark complete in todo.txt: `x YYYY-MM-DD ...`
2. Log in today.md

## Best Practices

- **Draft PRs** - Use draft status for work-in-progress
- **Small PRs** - Easier to review, faster to merge
- **Descriptive commits** - Include what and why
- **Clean history** - Rebase/squash before merge if needed
- **CI/CD green** - Fix all tests before requesting review
