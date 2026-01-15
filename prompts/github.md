# GitHub Utilities Quick Reference

Quick reference for common GitHub operations via MCP tools.

## Available Tools

- `mcp_aicodinggithub_call_github_graphql_for_query` - Query data (read-only)
- `mcp_aicodinggithub_call_github_graphql_for_mutation` - Modify data (write)
- `mcp_aicodinggithub_get_pull_request_diff` - Get PR diff
- `mcp_aicodinggithub_call_github_restapi_for_search` - Search code/users

## Common Operations

### 1. List My Open PRs

```graphql
query {
  search(
    query: "author:@me is:pr is:open repo:Platform-Common/webex-teams-bot"
    type: ISSUE
    first: 10
  ) {
    issueCount
    edges {
      node {
        ... on PullRequest {
          number
          title
          url
          createdAt
          isDraft
          reviewDecision
        }
      }
    }
  }
}
```

### 2. List PRs Awaiting My Review

```graphql
query {
  search(
    query: "review-requested:@me is:pr is:open repo:Platform-Common/webex-teams-bot"
    type: ISSUE
    first: 10
  ) {
    issueCount
    edges {
      node {
        ... on PullRequest {
          number
          title
          url
          author { login }
        }
      }
    }
  }
}
```

### 3. Get PR Status

```graphql
query {
  repository(owner: "Platform-Common", name: "webex-teams-bot") {
    pullRequest(number: 1545) {
      title
      state
      isDraft
      mergeable
      reviewDecision
      statusCheckRollup {
        state
      }
      commits(last: 1) {
        nodes {
          commit {
            statusCheckRollup {
              contexts(first: 10) {
                nodes {
                  ... on CheckRun {
                    name
                    conclusion
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}
```

### 4. Add PR Comment

```graphql
mutation {
  addComment(input: {
    subjectId: "PR_NODE_ID"
    body: "Your comment here"
  }) {
    commentEdge {
      node {
        id
        url
      }
    }
  }
}
```

**To get PR node ID:**
```graphql
query {
  repository(owner: "Platform-Common", name: "webex-teams-bot") {
    pullRequest(number: 1545) {
      id
    }
  }
}
```

### 5. Request Review

```graphql
mutation {
  requestReviews(input: {
    pullRequestId: "PR_NODE_ID"
    userIds: ["USER_NODE_ID"]
  }) {
    pullRequest {
      number
    }
  }
}
```

### 6. Get PR Diff

Use the REST API tool:
```
owner: Platform-Common
repo: webex-teams-bot
pull_number: 1545
```

Returns unified diff format.

### 7. Search Code

```
resource: code
parameters: {
  "q": "pagerduty adapter language:python repo:Platform-Common/webex-teams-bot",
  "per_page": 20
}
```

## Workflow Integration

### Creating PR Task

When user opens a PR, add to todo.txt:
```
(C) YYYY-MM-DD @git {PR title} [org/repo/PR#{num}](url) +JIRA-KEY +FY26Q2
```

### Checking PR Status

When user says "check PR 1545":
1. Query PR status (state, checks, reviews)
2. Report: "PR#1545: {title} - {state}, checks: {pass/fail}, reviews: {approved/changes requested}"

### Reviewing PRs

When user says "review PR 1545":
1. Get PR diff
2. Present key changes
3. Help draft review comments if needed

### Merging PR

When PR is merged:
1. Mark complete in todo.txt: `x YYYY-MM-DD ...`
2. Update JIRA with PR link
3. Log in today.md

## Git Commands Quick Reference

### Common Workflows

**Start new feature:**
```bash
git checkout -b fix-wxsa-18425-pagerduty-race
```

**Commit with JIRA reference:**
```bash
git commit -m "PROJ-1234: Fix PagerDuty race condition

- Implement instance-based adapter pattern
- Add thread safety with locks
- Update tests"
```

**Push branch:**
```bash
git push -u origin fix-wxsa-18425-pagerduty-race
```

**Update branch with main:**
```bash
git fetch origin
git rebase origin/main
```

**Interactive rebase (clean up commits):**
```bash
git rebase -i HEAD~3
```

### Branch Naming Conventions

**Work repos pattern:**
- `fix-{jira-key}-{short-description}`
- `feature-{jira-key}-{short-description}`
- `docs-{jira-key}-{short-description}`

**Examples:**
- `fix-wxsa-18425-pagerduty-race`
- `feature-wxsa-18364-cc-escalation`
- `docs-wxsa-18413-catalog-readme`

## PR Description Template

```markdown
## Summary
Brief description of changes

## JIRA
Closes PROJ-5678

## Changes
- Change 1
- Change 2
- Change 3

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Screenshots (if applicable)
...

## Notes for Reviewers
Any specific areas needing attention
```

## Best Practices

- **Link JIRA in PR** - Use "Closes WXSA-XXXXX" or "Relates to WXSA-XXXXX"
- **Draft PRs** - Use draft status for work-in-progress
- **Small PRs** - Easier to review, faster to merge
- **Descriptive commits** - Include what and why
- **Clean history** - Rebase/squash before merge if needed
- **Response to reviews** - Address all comments, respond explicitly
- **CI/CD green** - Fix all tests before requesting review
