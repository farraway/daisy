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

**Symlinks (correct):**
```
prompt.md → home/work/prompt.md
tasks → home/work/tasks
journal.md → home/work/journal/journal.md
today.md → home/work/journal/today.md
```

**Files (all exist):**
```
home/work/prompt.md
home/work/tasks/todo.txt
home/work/tasks/done.txt
home/work/tasks/alias.txt
home/work/journal/journal.md
home/work/journal/today.md
```

**tasks/todo.txt (valid format):**
```
(A) 2026-01-16 Valid task @jira
2026-01-16 Valid inbox task
x 2026-01-16 2026-01-15 Valid completed task
```

### Command
```
check system
```

### Expected Output

```
✅ System Health Check

Home: work
Required Files: ✅ All 6 files present
Symlinks:
  ✅ prompt.md → home/work/prompt.md
  ✅ tasks → home/work/tasks
  ✅ journal.md → home/work/journal/journal.md
  ✅ today.md → home/work/journal/today.md
Format: ✅ todo.txt format valid (3 tasks)
Status: ✅ System healthy
```

---

## Test Case 9: System Health Check (Issues)

### Setup

**Symlinks (broken):**
```
prompt.md → home/work/prompt.md (exists)
tasks → home/work/tasks (exists)
journal.md → MISSING
today.md → home/work/journal/today.md (target missing)
```

**Missing files:**
```
home/work/journal/journal.md (missing)
home/work/journal/today.md (missing)
```

**tasks/todo.txt (invalid format):**
```
(A) 2026-01-16 Valid task @jira
INVALID LINE NO DATE
x 2026-13-99 2026-01-15 Invalid date format
```

### Command
```
check system
```

### Expected Output

```
⚠️ System Health Check

Home: work
Required Files: ⚠️ 2 of 6 files missing
  ⚠️ Missing: home/work/journal/journal.md
  ⚠️ Missing: home/work/journal/today.md

Symlinks:
  ✅ prompt.md → home/work/prompt.md
  ✅ tasks → home/work/tasks
  ⚠️ journal.md → symlink missing
  ⚠️ today.md → target file missing

Format: ⚠️ 2 format issues in todo.txt
  Line 2: Missing date
  Line 3: Invalid date format

Status: ⚠️ System needs attention

Suggestions:
- Create missing files from templates
- Re-run home setup: "switch to work"
- Fix format issues in todo.txt
```

---

## Test Case 10: Home Switching

### Setup

**Current state:**
```
Active home: work
Symlinks point to home/work/*
```

**Target:**
```
home/personal/prompt.md exists
home/personal/tasks/ exists
home/personal/journal/ exists
```

### Command
```
switch to personal
```

### Expected Process

1. **Detect current:** Parse prompt.md → "work"
2. **Teardown:** Remove 4 symlinks (prompt.md, tasks, journal.md, today.md)
3. **Verify target:** Check home/personal/prompt.md exists
4. **Setup:** Create 4 new symlinks to home/personal/*
5. **Verify:** Check all required files exist

### Expected Output

```
Deactivated home: work
✅ Activated home: personal

Verified files:
  ✅ tasks/todo.txt
  ✅ tasks/done.txt
  ✅ tasks/alias.txt
  ✅ journal/journal.md
  ✅ journal/today.md
  ✅ prompt.md

All systems ready!
```

**Verification:**
- ✅ Old symlinks removed
- ✅ New symlinks created
- ✅ Symlinks point to home/personal/*
- ✅ All files verified
- ✅ No files deleted (only symlinks changed)

---

## Test Case 11: Pattern Matching (Disambiguation)

### Setup

**today.md:**
```markdown
#### High-Priority Tasks
- [ ] @jira Fix bug in adapter module
- [ ] @jira Fix bug in utils module
- [ ] @jira Review design doc
```

### Command
```
done bug
```

### Expected Behavior

```
Multiple tasks match "bug":
1. Fix bug in adapter module
2. Fix bug in utils module

Which one? (or provide more specific pattern)
```

**User responds:** "adapter"

**AI completes:** Task 1 only

**Verification:**
- ✅ AI detects multiple matches
- ✅ AI lists options
- ✅ AI waits for clarification
- ✅ AI completes correct task after clarification

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
