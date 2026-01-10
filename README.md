# Daisy - Personal Productivity System

A lightweight productivity system combining todo.txt task management with daily markdown journaling.

## Quick Start

### For AI Assistants

When helping with this workflow, read and follow: **`@daisy/docs/ai-workflow-prompt.md`**

### For Humans

1. Add tasks to `todo.txt` using [todo.txt format](docs/todotxt.md)
2. Ask your AI assistant to "start a new day"
3. Work through tasks in `today.md`
4. Log progress as you go
5. Fill in retrospective at end of day

## File Structure

```
daisy/
├── todo.txt              # Active tasks (todo.txt format)
├── done.txt              # Completed tasks archive
├── today.md              # Current day's journal
├── journal.md            # Archive of all past days
├── alias.txt             # People/role aliases (~person format)
├── perf/                 # Performance reflections and career documentation
├── docs/
│   ├── todotxt.md        # Todo.txt format specification
│   └── ai-workflow-prompt.md  # Instructions for AI assistants
└── templates/
    ├── journal-day.md    # Template for daily entries
    └── journal-week.md   # Template for weekly entries
```

## Todo.txt Format Quick Reference

```
(A) 2026-01-06 High priority task +Project @context
(B) 2026-01-06 Medium priority task +PROJ-5678 @jira
(C) 2026-01-06 Normal task @github
```

**Priority levels:**
- `(A)` - Now (do today)
- `(B)` - Next (this week)
- `(C)` - Soon (default for most tasks)
- `(D)` - Someday (backlog)

**Common contexts:**
- `@jira` - Jira tickets
- `@github` - Pull requests
- `@review` - Needs attention
- `@cancelled` - Closed without implementation (done.txt only)

**Common projects:**
- `+WXSA-XXXXX` - Jira ticket keys
- `+FY26Q2` - Fiscal quarter tags
- `+repo-name` - Repository names

## People References

Use `~alias` to reference people consistently across all files:
- `~jdoe` - References Smitha Gubbi (manager)
- `~deaclose` - References yourself

The `alias.txt` file maintains the mapping:
```
~jdoe Jane Doe <jdoe@example.com> #jane #manager
~deaclose Dean Close <deaclose@cisco.com> #me #dean
```

This enables consistent cross-referencing and avoids ambiguity (first names, nicknames, etc.)

## Daily Workflow

### Morning
```
You: "Start a new day"
AI: [Archives yesterday's work, creates new today.md with your A/B priority tasks]
```

### During the Day
```
You: "Log fixed WXSA-15770 bug"
AI: [Adds timestamped entry to Log section]

You: "Done WXSA-18369"
AI: [Marks task complete in today.md and moves to done.txt]
```

### Evening
```
You: Update Retrospective section with successes, misses, and next steps
AI: [Helps synthesize the day's work]
```

## Example Daily Entry

```markdown
### 2026-01-06 Monday

#### Agenda
- 0930 Stand-up meeting
- 1230 Lunch with team
- 1530 Code review session

#### Tasks

- [x] Avengers pages must provide valid meeting links +WXSA-18369 +FY26Q2 @jira
- [ ] Refactor incident creation workflows +WXSA-18322 +FY26Q2 @jira

#### Pull Requests

- [x] Update to SOE template (PR#1538 webex-teams-bot) +WXSA-15770 +FY25Q4 @github

#### Log

* 0930 - New day started
* 1045 - Investigated WXSA-15770 - discovered bot template never called
* 1345 - Closed WXSA-15770 as will not implement due to architectural constraints
* 1530 - Closed PR#1538 and updated Jira

#### Retrospective

* **Successes:** Deep-dive investigation saved from implementing dead code
* **Misses:** Should have verified architecture before creating PR
* **What would a Sage do next:** Document architectural findings for team
```

## Historical Data Integration

The AI assistant can help populate historical entries from:
- GitHub commit history
- Jira completion records
- Previous task management exports (Wrike, etc.)
- Calendar/agenda archives

**Key principle:** When importing historical data, maintain professional tone by filtering or reframing informal/unprofessional content.

## Integration with AI

This system is designed to work seamlessly with AI assistants (like me!). Simply:

1. Attach the relevant files (`todo.txt`, `today.md`, etc.)
2. Ask for help starting your day, logging work, or completing tasks
3. The AI follows the workflow defined in `docs/ai-workflow-prompt.md`

## Philosophy

- **Simple text files** - Everything in git-friendly markdown and todo.txt
- **AI-native** - Designed for AI assistance, not automation scripts
- **Flexible** - Adapt the system to your workflow, not the other way around
- **Transparent** - All your data is human-readable plain text

## See Also

- [Todo.txt specification](docs/todotxt.md)
- [AI workflow guide](docs/ai-workflow-prompt.md)
