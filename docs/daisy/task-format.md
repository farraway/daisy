# Task Format Specification

## Active Task Format

```regex
^(\([A-D]\) )?({date_created} )({description}.*)$

Where:
  Group 1: Priority (optional): (A), (B), (C), or (D)
  Group 2: Creation date (required): YYYY-MM-DD + space
  Group 3: Description (required): Text with @context, +PROJECT, ~ALIAS labels
```

**Examples:**
```
(A) 2026-01-15 Fix critical bug @jira +PROJ-1234
(B) 2026-01-15 Review PR @git +PROJ-1236
2026-01-15 Triage this task +INBOX
(C) 2026-01-15 Standard work @context
```

## Completed Task Format

```regex
^x ({date_completed} )({date_created} )({description}.*)$

Where:
  x: Completion marker (lowercase x + space)
  Group 1: Completion date (required): YYYY-MM-DD + space
  Group 2: Creation date (required): YYYY-MM-DD + space
  Group 3: Description (required): Original description WITHOUT priority
```

**Example transformation:**
```
Before: (B) 2026-01-13 Fix bug @jira +PROJ-1234
After:  x 2026-01-15 2026-01-13 Fix bug @jira +PROJ-1234
```

**Note:** Priority is STRIPPED when marking complete. It's no longer relevant once done.

## Cancelled Task Format

```regex
^z ({date_cancelled} )({date_created} )({description}.*)$

Where:
  z: Cancellation marker (lowercase z + space)
  Group 1: Cancellation date (required): YYYY-MM-DD + space
  Group 2: Creation date (required): YYYY-MM-DD + space
  Group 3: Description (required): Original description WITHOUT priority
```

**Example transformation:**
```
Before: (C) 2026-01-14 Old task @context
After:  z 2026-01-15 2026-01-14 Old task @context
```

**Cleanup:** Cancelled tasks are automatically deleted during "new day" or "new week" workflows.

## Priority Floors (Advanced)

Tags can set minimum priority levels that cannot be demoted:

```
Priority floors (cannot demote):
- +URGENT: minimum priority A
- +OVERDUE: minimum priority B
- +TODAY: minimum priority B

Final priority = max(task priority, tag minimum priority)
```

**Examples:**
```
(C) 2026-01-15 Task +URGENT       → Treated as (A)
(D) 2026-01-15 Task +OVERDUE      → Treated as (B)
(A) 2026-01-15 Task +TODAY        → Treated as (A) (no demotion)
2026-01-15 Task +TODAY            → Treated as (B)
```

**Note:** Not yet implemented in scripts. Current scripts use explicit priority only.

## Task Extraction Rules

When extracting tasks from `todo.txt` to `today.md`:

1. **Now section:** Priority (A) tasks, excluding @git/@github
2. **Next section:** Priority (B) tasks, excluding @git/@github
3. **Inbox section:** Tasks without priority prefix, excluding @git/@github (includes default checklist items)
4. **GitHub PRs section:** ALL tasks with @git or @github context, any priority

**Special case:** Tasks with @git/@github always go to GitHub section regardless of priority.

**Task preservation:** Any incomplete task in yesterday's today.md should be preserved unless explicitly completed or cancelled. (Not yet implemented in scripts)

## Task-to-Markdown Conversion

When extracting tasks from `todo.txt` to create markdown checklist in `today.md`:

**Source format (todo.txt):**
```regex
^([xz] )?(\([A-D]\) )?({date_completed} )?({date_created} )?(.*)$

Where:
  Group 1: Completion status (x or z, optional)
  Group 2: Priority (optional)
  Group 3: Completion/cancellation date (optional)
  Group 4: Creation date (required for active tasks)
  Group 5: Body (description with labels)
```

**Target format (today.md):**
```markdown
- [ ] {contexts} {description}
- [x] {contexts} {description}  (if completed)
- [z] {contexts} {description}  (if cancelled)
```

**Conversion Steps:**
1. Parse source line using regex groups
2. Determine checkbox state:
   - Group 1 = "x " → use `- [x]`
   - Group 1 = "z " → use `- [z]`
   - Group 1 empty → use `- [ ]`
3. Drop priority (group 2)
4. Drop dates (groups 3, 4)
5. Extract body (group 5)
6. Parse body for @context labels (tokens starting with `@`)
7. Reformat: checkbox + contexts (space-separated) + remaining description

**Conversion Examples:**

| Input (todo.txt) | Output (today.md) |
|------------------|-------------------|
| `(A) 2026-01-15 Fix bug @jira +PROJ-1234` | `- [ ] @jira Fix bug +PROJ-1234` |
| `x 2026-01-16 2026-01-15 Fix bug @jira +PROJ-1234` | `- [x] @jira Fix bug +PROJ-1234` |
| `2026-01-15 Review PR @git +PROJ-1236` | `- [ ] @git Review PR +PROJ-1236` |
| `z 2026-01-16 2026-01-15 Old task @context` | `- [z] @context Old task` |
| `(B) 2026-01-15 @git @jira Multi-context task` | `- [ ] @git @jira Multi-context task` |

**Edge cases:**
- No contexts in body → Just description (no context prefix)
- Context in middle of description → Move all contexts to front

## File Organization

### todo.txt Structure

```
Active tasks (top of file, grouped by priority):
(A) 2026-01-15 High priority task 1
(A) 2026-01-15 High priority task 2
(B) 2026-01-15 Next priority task 1
(B) 2026-01-15 Next priority task 2
2026-01-15 Inbox task 1 (needs triage)
2026-01-15 Inbox task 2
(C) 2026-01-15 Soon task 1
(C) 2026-01-15 Soon task 2
(D) 2026-01-15 Someday task 1

Completed tasks (end of file, before cancelled):
x 2026-01-15 2026-01-13 Completed task 1
x 2026-01-15 2026-01-14 Completed task 2

Cancelled tasks (end of file, after completed):
z 2026-01-15 2026-01-14 Cancelled task 1
z 2026-01-15 2026-01-13 Cancelled task 2
(deleted during next new day/week)
```

Tasks should be grouped by priority, but exact ordering within a priority group is flexible.

### done.txt Structure

```
Archived completed tasks (chronological):
x 2026-01-08 2026-01-05 Old completed task 1
x 2026-01-09 2026-01-06 Old completed task 2
x 2026-01-10 2026-01-07 Old completed task 3
```

Tasks moved here during weekly archival to keep `todo.txt` focused on active work.
