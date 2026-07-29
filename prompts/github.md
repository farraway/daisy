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

Use the **github** MCP server (`user-github-*` tools) for all public GitHub
operations. Never use `mcp_aicodinggithub_*` tools for github.com — those
connect to an internal GitHub Enterprise instance.

### Key Tools

- `user-github-list_pull_requests` / `user-github-search_pull_requests` — find PRs
- `user-github-pull_request_read` — details, diff, status, reviews, comments
- `user-github-create_pull_request` / `user-github-update_pull_request` — manage PRs
- `user-github-merge_pull_request` — merge PRs
- `user-github-pull_request_review_write` — create/submit reviews
- `user-github-list_issues` / `user-github-search_issues` — find issues
- `user-github-issue_write` / `user-github-issue_read` — manage issues
- `user-github-get_file_contents` / `user-github-search_code` — browse code
- `user-github-list_commits` / `user-github-get_commit` — browse commits

### Common Operations

**List My Open PRs**

```
user-github-search_pull_requests
  query: "author:@me is:open"
  owner: {owner}
  repo: {repo}
```

**Check PR Status**

```
user-github-pull_request_read
  method: "get"        # details
  method: "get_status" # build/check status
  method: "get_diff"   # diff
  method: "get_files"  # changed files list
```

**Create a PR**

```
user-github-create_pull_request
  owner, repo, title, head, base, body
```

**Review a PR**

For complex reviews with line-level comments:
1. `user-github-pull_request_review_write` method: "create"
2. `user-github-add_comment_to_pending_review`
3. `user-github-pull_request_review_write` method: "submit_pending"

For simple reviews:
- `user-github-pull_request_review_write` with event: "APPROVE" / "REQUEST_CHANGES" / "COMMENT"

Full workflow integration patterns: [`daisy/docs/github.md`](daisy/docs/github.md).
