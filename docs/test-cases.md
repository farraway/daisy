# Daisy Validation Test Cases

This document contains test cases for validating AI behavior in the daisy system. Use these to verify workflows produce expected outputs.

## Test Case 1: Starting a New Day

### Setup

**tasks/todo.txt:**
```
(A) 2026-01-15 Fix critical bug @jira +PROJ-1234
(B) 2026-01-15 Review design doc @jira +PROJ-1235
(B) 2026-01-15 Sprint planning @meeting
2026-01-15 Triage new tickets +INBOX
2026-01-15 Update documentation @jira +PROJ-1236
(B) 2026-01-15 Review PR#1234 @git +PROJ-1237
(C) 2026-01-15 Refactor utils module @jira
x 2026-01-15 2026-01-10 Old completed task @jira
```

### Command
```
start a new day
```

### Expected Output

**today.md sections should contain:**

```markdown
#### High-Priority Tasks
- [ ] @jira Fix critical bug +PROJ-1234
- [ ] @jira Review design doc +PROJ-1235
- [ ] @meeting Sprint planning

#### Task Inbox
- [ ] Triage new tickets +INBOX
- [ ] @jira Update documentation +PROJ-1236

#### GitHub PRs
- [ ] @git Review PR#1234 +PROJ-1237
```

**Verification:**
- ✅ (A) and (B) tasks in High-Priority (excluding @git)
- ✅ Tasks without priority in Task Inbox (excluding @git)
- ✅ @git/@github tasks in separate section regardless of priority
- ✅ (C) priority task NOT included (not high-priority)
- ✅ Completed tasks (x prefix) NOT included

---

## Test Case 2: Completing a Task

### Setup

**tasks/todo.txt:**
```
(A) 2026-01-16 Fix critical bug @jira +PROJ-1234
(B) 2026-01-16 Review design doc @jira
```

**today.md excerpt:**
```markdown
#### High-Priority Tasks
- [ ] @jira Fix critical bug +PROJ-1234
- [ ] @jira Review design doc
```

### Command
```
done bug fix
```

### Expected Output

**today.md after:**
```markdown
#### High-Priority Tasks
- [x] @jira Fix critical bug +PROJ-1234
- [ ] @jira Review design doc
```

**tasks/todo.txt after:**
```
(B) 2026-01-16 Review design doc @jira
x 2026-01-16 2026-01-16 Fix critical bug @jira +PROJ-1234
```

**Verification:**
- ✅ Checkbox changed `[ ]` → `[x]` in today.md
- ✅ Task moved to end of todo.txt
- ✅ Priority STRIPPED from completed task
- ✅ Completion date added (first date)
- ✅ Creation date preserved (second date)
- ✅ Description unchanged
- ✅ Exactly one commit, containing both file edits
- ✅ Exactly one line of stdout: `✅ Done: {description} (commit {hash})` —
  no `Logged:`/`Committed:`/`No changes to commit` chatter (the Log entry
  and the one commit are made in-process)
- ✅ Matching is case-insensitive literal substring, not regex — `.`, `*`,
  `[`, `/` in the pattern match themselves

**If the description is absent from today.md** (e.g. an Inbox-only item):
todo.txt still completes, a warning is printed on stderr, and the run still
exits 0 and commits.

---

## Test Case 3: Cancelling a Task

### Setup

**tasks/todo.txt:**
```
(B) 2026-01-16 Old task @jira
(C) 2026-01-16 Another task @jira
```

### Command
```
cancel old task
```

### Expected Output

**tasks/todo.txt after:**
```
(C) 2026-01-16 Another task @jira
z 2026-01-16 2026-01-16 Old task @jira
```

**Verification:**
- ✅ Task marked with `z` prefix
- ✅ Priority STRIPPED
- ✅ Moved to end (after completed tasks if any)
- ✅ Format: `z {cancellation_date} {creation_date} {description}`

---

## Test Case 4: New Day with Cancelled Tasks

### Setup

**tasks/todo.txt:**
```
(A) 2026-01-16 Important task @jira
z 2026-01-15 2026-01-14 Old cancelled task 1
z 2026-01-15 2026-01-13 Old cancelled task 2
```

### Command
```
start a new day
```

### Expected Output

**tasks/todo.txt after:**
```
(A) 2026-01-16 Important task @jira
```

**Verification:**
- ✅ All lines starting with `z ` removed
- ✅ Active tasks preserved
- ✅ Report: "🗑️ Deleted 2 cancelled tasks"

---

## Test Case 5: Starting a New Week

### Setup

**tasks/todo.txt:**
```
(A) 2026-01-20 New priority task @jira
(B) 2026-01-20 Another new task @jira
x 2026-01-19 2026-01-15 Completed task 1 @jira
x 2026-01-18 2026-01-14 Completed task 2 @jira
x 2026-01-17 2026-01-13 Completed task 3 @jira
z 2026-01-19 2026-01-16 Cancelled task
```

**tasks/done.txt (before):**
```
x 2026-01-10 2026-01-05 Old completed task @jira
```

### Command
```
start a new week
```

### Expected Output

**tasks/todo.txt after:**
```
(A) 2026-01-20 New priority task @jira
(B) 2026-01-20 Another new task @jira
```

**tasks/done.txt after:**
```
x 2026-01-10 2026-01-05 Old completed task @jira
x 2026-01-19 2026-01-15 Completed task 1 @jira
x 2026-01-18 2026-01-14 Completed task 2 @jira
x 2026-01-17 2026-01-13 Completed task 3 @jira
```

**Verification:**
- ✅ Cancelled tasks deleted
- ✅ Completed tasks moved to done.txt
- ✅ Active tasks remain in todo.txt
- ✅ done.txt preserves existing entries
- ✅ Report: "🗑️ Deleted 1 cancelled task"
- ✅ Report: "📦 Archived 3 completed tasks"

---

## Test Case 6: Task-to-Markdown Conversion

### Input (todo.txt lines)

| Input | Expected Output (today.md) |
|-------|---------------------------|
| `(A) 2026-01-15 Fix bug @jira +PROJ-1234` | `- [ ] @jira Fix bug +PROJ-1234` |
| `x 2026-01-16 2026-01-15 Fix bug @jira +PROJ-1234` | `- [x] @jira Fix bug +PROJ-1234` |
| `2026-01-15 Review PR @git +PROJ-1236` | `- [ ] @git Review PR +PROJ-1236` |
| `z 2026-01-16 2026-01-15 Old task @context` | `- [z] @context Old task` |
| `(B) 2026-01-15 @git @jira Multi-context task` | `- [ ] @git @jira Multi-context task` |
| `(C) 2026-01-15 No context task +PROJECT` | `- [ ] No context task +PROJECT` |

**Verification Rules:**
- ✅ Priority stripped
- ✅ Dates stripped
- ✅ @context labels moved to front
- ✅ Checkbox state matches completion status (x/z/none)
- ✅ +PROJECT labels preserved in position
- ✅ Description preserved exactly

---

## Test Case 7: Abridged Logging

### Input (today.md log section)

```markdown
#### Log
- 0930 - Started investigation of PROJ-1234
- 1015 - Still working on PROJ-1234
- 1045 - Making progress
- 1130 - Found race condition in adapter init
- 1215 - Met with ~jdoe about approach
- 1245 - Decided to use instance-based pattern
- 1445 - Implemented fix
- 1530 - PR#1545 opened
- 1545 - PR approved by ~jdoe
- 1600 - PR#1545 merged
```

### Expected Output (journal.md)

```markdown
#### Log
- 0930-1445 - Investigated PROJ-1234, found race condition in adapter init
- 1215 - Met with ~jdoe about approach, decided instance-based pattern
- 1530-1600 - PR#1545 opened, approved by ~jdoe, merged
```

**Verification:**
- ✅ Multiple "working on" entries condensed to time range
- ✅ Stakeholder interactions preserved (~jdoe)
- ✅ Decisions preserved ("decided to use...")
- ✅ Milestones preserved (PR opened, merged)
- ✅ Discoveries preserved ("found race condition")
- ✅ Time ranges show extended work

---

## Test Case 8: System Health Check

### Setup

A workspace bound to the home `work` (`daisy init work`), the home created
by `daisy home new work` from `templates/home/`, and a root whose
publishable files carry no identifying term.

### Command
```
check system
```

### Expected Process

The agent runs `daisy healthcheck` (`--force` to bypass the cached pass)
and shows its output.

### Expected Output

```
✓ DAISY_ROOT: <root>
✓ DAISY_HOME: <home> (home: work)
✓ Git repository: <short hash>
✓ today.md format valid
✓ Component: list
  … one Component line per command with a health hook …
✓ Component: test
✓ Publication hygiene: no identifying terms outside the homes
✓ Feedback: 0 entries (never optimized)

✓ System healthy
```

**Verification:**
- ✅ Exit code 0
- ✅ The pass is cached in `<home>/.healthcheck.ttl` for 300 seconds

---

## Test Case 9: System Health Check (Issues)

### Setup

As Test Case 8, after a `daisy new-day` (a new home's today.md is empty
and passes); then today.md loses its `#### Log` heading, and the workspace
link `.daisy/journal.md` is deleted.

### Command
```
check system
```

### Expected Process

1. The agent runs `daisy healthcheck --force`, shows the full output and
   walks the user through each issue.
2. The health check does not inspect the workspace's links; `daisy status`
   or `daisy files` reports them.

### Expected Output

```
✗ today.md missing required heading: #### Log
…
✗ 1 issue(s) found
```

and from `daisy status`, on stderr:

```
⚠️  the binding link .daisy/journal.md is missing or does not point into the home; re-run 'daisy init work'
```

**Verification:**
- ✅ `daisy healthcheck` exits 1; a failing check clears the cached pass
- ✅ The fixes offered are the ones printed: restore the heading, re-run
  `daisy init work`

---

## Test Case 10: Home Switching

### Setup

**Current state:** the workspace is bound to `work`; a second home,
`personal`, is registered (`daisy home list` shows both).

### Command
```
switch to personal
```

### Expected Process

1. The agent runs `daisy init personal` (an identity: a registered home's
   name, or an `identities/<id>` file in exactly one registered home).
2. `init` builds the new binding in a stage beside `.daisy/`, unsets the
   previous identity from the workspace repository's git config, applies
   the new one, and swaps the stage in.
3. Verify with `daisy status`, whose header names the home and identity.

### Expected Output

```
Initializing Daisy in: <workspace>
  Home: personal
  Identity: personal
  ↻ Switching home: work → personal
  …
  ✓ Bound .daisy/ to home: personal

Done. Daisy is ready in this workspace.
…
```

**Verification:**
- ✅ `.daisy/home` reads `personal`; the six links point into the
  `personal` home
- ✅ No home file is deleted or changed (only the binding)
- ✅ Other workspaces bound to `work` are unaffected

---

## Test Case 11: Pattern Matching (Disambiguation)

### Setup

**tasks/todo.txt:**
```
(A) 2026-01-16 Fix bug in adapter module @jira
(A) 2026-01-16 Fix bug in utils module @jira
(B) 2026-01-16 Review design doc @jira
```

### Command
```
done bug
```

### Expected Behavior

`daisy done` refuses outright — it does not guess or wait interactively, since
resolution and disambiguation both happen inside the command, not the agent:

```
Multiple tasks match "bug":
  1) (A) 2026-01-16 Fix bug in adapter module @jira
  2) (A) 2026-01-16 Fix bug in utils module @jira
```

- Exit code: non-zero
- **todo.txt and today.md are untouched** — no partial completion, no commit
- The agent relays the numbered candidates to the user, then re-runs `done`
  with a more specific pattern (e.g. `done adapter`), which resolves to
  exactly one match and completes normally

**Verification:**
- ✅ Both matching tasks listed, numbered
- ✅ Non-zero exit — an agent trusting the exit code will not report success
- ✅ No file touched on refusal
- ✅ A more specific pattern on retry resolves and completes

---

## Test Case 12: Priority Extraction Edge Cases

### Setup

**tasks/todo.txt:**
```
(A) 2026-01-16 High priority normal task @jira
(B) 2026-01-16 High priority git task @git
2026-01-16 Inbox normal task @jira
2026-01-16 Inbox git task @github
(C) 2026-01-16 Soon normal task @jira
(C) 2026-01-16 Soon git task @git
x 2026-01-16 2026-01-15 Completed task @jira
z 2026-01-16 2026-01-15 Cancelled task @jira
```

### Command
```
start a new day
```

### Expected today.md Sections

**High-Priority Tasks:**
```
- [ ] @jira High priority normal task
```

**Task Inbox:**
```
- [ ] @jira Inbox normal task
```

**GitHub PRs:**
```
- [ ] @git High priority git task
- [ ] @github Inbox git task
- [ ] @git Soon git task
```

**Verification:**
- ✅ (A) non-git → High-Priority
- ✅ (B) with @git → GitHub PRs (NOT High-Priority)
- ✅ No priority, no git → Task Inbox
- ✅ No priority, with @github → GitHub PRs
- ✅ (C) with @git → GitHub PRs
- ✅ x and z tasks EXCLUDED from all sections

---

## Test Case 13: Aggregating a Project's Tasks

### Setup

**tasks/todo.txt:**
```
(B) 2026-01-16 Design the schema +widgets
x 2026-01-15 2026-01-10 Draft the proposal +widgets
(C) 2026-01-16 Unrelated task +other
```

**tasks/done.txt:**
```
x 2026-01-05 2026-01-01 Kickoff meeting +widgets
```

### Command
```
daisy tasks --all +widgets
```

### Expected Output

```
(B) 2026-01-16 Design the schema +widgets
x 2026-01-15 2026-01-10 Draft the proposal +widgets
x 2026-01-05 2026-01-01 Kickoff meeting +widgets
```

**Verification:**
- ✅ Bare lines only — no header, no summary count, no decoration
- ✅ `+other` excluded — literal substring match on `+widgets`, not a
  looser tag match
- ✅ Both todo.txt (active + not-yet-rotated completed) and done.txt lines
  present, since no `--todo`/`--done` filter is given; `--all` is what puts
  `+widgets` in scope from a workspace not bound to it
- ✅ Read-only: todo.txt and done.txt are byte-identical after the command,
  and no commit is produced
- ✅ No pattern given → every line of the selected file(s)
- ✅ No matches → nothing on stdout, non-zero exit (grep convention)

---

## Running These Tests

To validate AI behavior:

1. Set up the test case exactly as specified
2. Run the command
3. Compare output to expected output
4. Verify all checkmarks (✅) are satisfied
5. If any verification fails, check AI prompt interpretation

## Adding New Tests

When adding test cases:
- Provide complete setup (exact file contents)
- Specify exact command
- Define expected output precisely
- Include verification checklist
- Cover edge cases and error conditions
