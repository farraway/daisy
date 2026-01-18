# Daisy - System Architecture & Internal Specifications

This document contains the detailed internal specifications for the daisy productivity system.

**For user-focused workflows, see `@daisy/prompts/daisy.md`**

## Purpose

This document is for:
- System designers working on daisy architecture
- Developers implementing new workflows or scripts
- Troubleshooting format or sync issues
- Understanding the detailed parsing and conversion algorithms

## Task Format Specification

### Active Task Format

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

### Completed Task Format

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

### Cancelled Task Format

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

## Task Priority System - Detailed Rules

### Priority Floors (Advanced)

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

### Task Extraction Rules

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

## Task File Organization

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

**Important:** Tasks should be grouped by priority, but exact ordering within a priority group is flexible.

### done.txt Structure

```
Archived completed tasks (chronological):
x 2026-01-08 2026-01-05 Old completed task 1
x 2026-01-09 2026-01-06 Old completed task 2
x 2026-01-10 2026-01-07 Old completed task 3
```

Tasks moved here during weekly archival to keep `todo.txt` focused on active work.

## Task Synchronization Rules

**`tasks/todo.txt` is the canonical source of truth for all tasks.**

### Bidirectional Sync Requirements

**ANY task change must update BOTH `todo.txt` and `today.md`:**

1. **Adding a new task:**
   - Add to `todo.txt` with priority (A/B/C/D), creation date, tags
   - Add to `today.md` if priority is (A) or (B) and working on it today

2. **Completing a task:**
   - Mark [x] in `today.md`
   - Update in `todo.txt`: strip priority, add `x YYYY-MM-DD` prefix, move to end

3. **Changing priority:**
   - Update priority (A)/(B)/(C)/(D) in `todo.txt`
   - Update section in `today.md` (Now, Next, Inbox)

4. **Changing due dates:**
   - Update `due:YYYY-MM-DD` in `todo.txt`
   - Reflect urgency in `today.md` (add **OVERDUE** flag if past due)

5. **Starting a new day:**
   - Pull high-priority (A) and (B) tasks from `todo.txt` → `today.md`
   - Archive previous `today.md` → `journal.md`

### Common Sync Issues

**Priority mismatches:**
- Task is (A) in `today.md` but (B) in `todo.txt`
- **Fix:** Update `todo.txt` to match intended priority
- **Cause:** Priority changed in one file but not the other

**Completion status mismatches:**
- Task marked [x] in `today.md` but still active in `todo.txt`
- **Fix:** Update `todo.txt` with `x YYYY-MM-DD` prefix and move to end
- **Cause:** "done" command only updated one file

**Missing tasks:**
- Task exists in `todo.txt` but not in `today.md`
- **Fix:** Usually intentional (lower priority not pulled into today)
- **Cause:** Only (A) and (B) tasks are extracted during "new day"

### Sync Validation Algorithm

**Command:** "sync tasks" or "check sync"

**Algorithm:**
```
1. Compare tasks in today.md vs todo.txt
2. For each task in today.md:
   a. Find matching task in todo.txt (by description substring)
   b. Check priority matches (section vs prefix)
   c. Check completion status matches (checkbox vs x prefix)
   d. Report discrepancies
3. Offer to synchronize automatically:
   - "Found 3 mismatches. Fix automatically?"
```

**Example Output:**
```
⚠️ Task sync issues found:

1. Priority mismatch:
   today.md: (A) Certificate training
   todo.txt: (B) Certificate training
   → Should be (A) in todo.txt

2. Completion mismatch:
   today.md: [x] PagerDuty migration
   todo.txt: (A) PagerDuty migration (still active)
   → Should be completed in todo.txt

Fix these automatically?
```

## CRITICAL: Format Preservation Rules

When modifying files, preserve EXACT formatting:

### Todo.txt Format Rules

- Active: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Completed: `x YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- Cancelled: `z YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- NEVER change date format (always `YYYY-MM-DD`)
- NEVER add/remove priority parentheses on active tasks
- NEVER preserve priority on completed/cancelled tasks
- NEVER reorder fields
- Preserve ALL spaces exactly

### Today.md Format Rules

- Preserve markdown heading levels (`####`)
- Preserve checkbox format: `- [ ]`, `- [x]`, `- [z]` (note spaces)
- NEVER add/remove blank lines between sections
- Time format: `HHMM` (24-hour, no colons)
- Log entry format: `- HHMM - message` (note dashes and spaces)

### Journal.md Archive Rules

- Append daily entries (curated/abridged during weekly review only)
- MAY be modified ONLY to:
  - Consolidate quiet days during weekly review
  - Update person references to canonical ~alias format
  - Fix chronological ordering errors
- NEVER delete stakeholder interactions or task progress
- NEVER modify retrospective content (represents historical perspective)

## Template Specifications

### journal-day.md Template

Used by `new-day.sh` to generate `today.md`:

```markdown
### {DATE} {DAY}

#### Agenda
- {TIME} Plan Day
- 1230 ...
- 1530 ...

#### Tasks

**Now:**
{HIGH_PRIORITY_TASKS}

**Next:**
{NEXT_PRIORITY_TASKS}

**Inbox:**
- [ ] Check calendar for upcoming events
- [ ] Check that todo.txt is up-to-date
- [ ] Plan day
- [ ] Retrospective
{INBOX_TASKS}

**GitHub PRs:**
{GITHUB_TASKS}

#### Log

- {TIME} New day started

#### Retrospective

* **Successes:** 
* **Misses:** 
* **What would a Sage do next:** 
```

**Placeholders:**
- `{DATE}` → YYYY-MM-DD
- `{DAY}` → Day of week (Monday, Tuesday, etc.)
- `{TIME}` → HHMM in Pacific Time
- `{HIGH_PRIORITY_TASKS}` → Priority A tasks as markdown checkboxes
- `{NEXT_PRIORITY_TASKS}` → Priority B tasks as markdown checkboxes
- `{INBOX_TASKS}` → No-priority tasks as markdown checkboxes (appended after default checklist)
- `{GITHUB_TASKS}` → @git/@github tasks as markdown checkboxes

**Subsection formatting:**
- Use bold text (`**Section:**`) not headings (simplified to just "Now" and "Next")
- Preserve blank line before each subsection
- Inbox section includes default daily checklist items
- If section is empty (no extracted tasks), the placeholder is replaced with empty string

### journal-week.md Template

Used by `new-week.sh` to generate `today.md` for week start:

```markdown
#### Weekly Retrospective

* **Successes:** 
* **Misses:** 
* **What would a Sage do next:** 

---

### {DATE} {DAY}

#### Resolutions

- Who would you like to be?

#### Agenda
- {TIME} Plan Day
- 1230 ...
- 1530 ...

#### Tasks

**Now:**
{HIGH_PRIORITY_TASKS}

**Next:**
{NEXT_PRIORITY_TASKS}

**Inbox:**
- [ ] Retrospective for previous week
- [ ] Set resolutions for this week.
- [ ] Sync todo.txt with @jira and @github
- [ ] Zero Email Inboxes
- [ ] Zero Chat Notifications
- [ ] Check calendar for upcoming events
- [ ] Check that todo.txt is up-to-date
- [ ] Plan day
- [ ] Retrospective
{INBOX_TASKS}

**GitHub PRs:**
{GITHUB_TASKS}

#### Log

- {TIME} New day started

#### Retrospective

* **Successes:** 
* **Misses:** 
* **What would a Sage do next:** 
```

**Key Differences from journal-day.md:**
- Starts with "Weekly Retrospective" section at top (for previous week)
- Includes "Resolutions" section (identity-based goal setting)
- Extended inbox checklist with weekly startup items (email/chat zero-ing, JIRA/GitHub sync)
- Same task extraction logic and placeholders as daily template

## Home Management - Detailed Algorithms

### Detecting Active Home

```
1. Check $DAISY_HOME environment variable
   - If not set, error: "⚠️ DAISY_HOME not set. Add to ~/.zshenv"
2. Extract home name from $DAISY_HOME path:
   - Example: /path/to/daisy/home/work → Home: "work"
3. Verify home directory exists at $DAISY_HOME
4. Verify include.txt exists at $DAISY_HOME/include.txt
```

### System Health Check Algorithm

**Command:** "check system" or "verify setup" or "check home"

```
1. Detect active home:
   a. Check if $DAISY_HOME is set
   b. If not: Report "⚠️ No active home. Set DAISY_HOME in ~/.zshenv"
   c. Extract home name from $DAISY_HOME path

2. Verify home structure:
   a. Check if $DAISY_HOME directory exists
   b. Check if $DAISY_HOME/include.txt exists
   c. Check if $DAISY_HOME/tasks/ directory exists
   d. Check if $DAISY_HOME/journal/ directory exists

3. Verify Required Files:
   a. Check $DAISY_HOME/tasks/todo.txt exists
   b. Check $DAISY_HOME/tasks/done.txt exists
   c. Check $DAISY_HOME/tasks/alias.txt exists
   d. Check $DAISY_HOME/journal/journal.md exists
   e. Check $DAISY_HOME/journal/today.md exists
   f. Report: ✅ or ⚠️ Missing: {path}

4. Verify Required Symlinks:
   a. Check if $DAISY_ROOT/tasks/ symlink exists and points to $DAISY_HOME/tasks/
   b. Check if $DAISY_ROOT/journal.md symlink exists and points to $DAISY_HOME/journal/journal.md
   c. Check if $DAISY_ROOT/today.md symlink exists and points to $DAISY_HOME/journal/today.md
   d. Report: ✅ {link} → {target} or ⚠️ Issue with {link}

5. Validate todo.txt format (if exists):
   a. Read tasks/todo.txt
   b. For each line:
      - Skip empty lines
      - Verify matches active/completed/cancelled format
      - Check dates are YYYY-MM-DD format
      - Check priority format (A-D) if present
   c. Find format violations
   d. Report: ✅ Format valid or ⚠️ Format issues: {list}

6. Check for orphaned completed tasks:
   a. Count lines starting with "x " not at end of active section
   b. Count lines starting with "z " anywhere (should be 0 after cleanup)
   c. Report: ✅ or ℹ️ {N} completed tasks need archival

7. Verify PROMPT.md is up to date:
   a. Check if $DAISY_ROOT/PROMPT.md exists
   b. If missing: Report "⚠️ PROMPT.md not found. Run build-prompt.sh"
   c. Check modification time of PROMPT.md vs include.txt
   d. If include.txt is newer: Report "ℹ️ PROMPT.md may be stale. Run build-prompt.sh"

8. Final Report:
   ✅ Home: {name}
   ✅ All required files present
   ✅ All symlinks correct
   ✅ Todo.txt format valid
   ✅ No orphaned tasks
   ✅ PROMPT.md up to date
   
   OR
   
   ⚠️ Home: {name} - Issues found:
   - Missing files: {list}
   - Symlink issues: {list}
   - Format issues: {list}
   - Orphaned tasks: {N}
   - PROMPT.md needs rebuild
   
   Suggestions:
   - Create missing files from templates
   - Fix symlinks: See "Home Switching Algorithm"
   - Archive completed tasks: "start a new week"
   - Rebuild prompt: $DAISY_ROOT/scripts/build-prompt.sh
```

### Home Switching Algorithm

**Command:** "switch to [home]"

```
1. Detect current home:
   a. Get current $DAISY_HOME value
   b. Extract home name from path

2. Verify target home exists:
   a. Check if $DAISY_ROOT/home/{home}/ directory exists
   b. If not found, offer: "Home '{home}' not found. Create from template?"
   c. Verify $DAISY_ROOT/home/{home}/include.txt exists
   d. Verify $DAISY_ROOT/home/{home}/tasks/ exists
   e. Verify $DAISY_ROOT/home/{home}/journal/ exists

3. Update environment variable:
   a. Instruct user: "Add to ~/.zshenv: export DAISY_HOME=\"$DAISY_ROOT/home/{home}\""
   b. Instruct user: "Then run: source ~/.zshenv"

4. Update symlinks:
   a. Remove old symlinks:
      - rm $DAISY_ROOT/tasks
      - rm $DAISY_ROOT/journal.md
      - rm $DAISY_ROOT/today.md
   b. Create new symlinks:
      - ln -sf home/{home}/tasks tasks
      - ln -sf home/{home}/journal/journal.md journal.md
      - ln -sf home/{home}/journal/today.md today.md
   c. Report: "✅ Activated home: {home}"

5. Rebuild PROMPT.md:
   a. Run $DAISY_ROOT/scripts/build-prompt.sh
   b. This reads new home's include.txt and regenerates PROMPT.md
   c. Report: "✅ Built PROMPT.md for home: {home}"

6. Verify setup:
   a. Run healthcheck.sh to verify all files present
   b. If any missing, warn: "⚠️ Missing files: [list]"
   c. Offer to create missing files from templates
```

### Creating New Home Algorithm

**Command:** "create home [name]"

```
1. Check if home/{name}/ exists
   - If exists, error: "Home '{name}' already exists"

2. Copy templates/home/ to home/{name}/

3. Instruct user to customize home/{name}/include.txt:
   - List which prompts to load (one per line)
   - Common prompts: daisy, retrospective, cisco, jira, github, webex
   - Example:
     ```
     daisy
     retrospective
     github
     ```

4. Create required symlinks:
   a. ln -sf home/{name}/tasks tasks
   b. ln -sf home/{name}/journal/journal.md journal.md
   c. ln -sf home/{name}/journal/today.md today.md

5. Set environment variable and build prompt:
   a. Instruct user: "export DAISY_HOME=\"$DAISY_ROOT/home/{name}\""
   b. Run $DAISY_ROOT/scripts/build-prompt.sh
   c. Report: "✅ Created home: {name}"

6. Ask: "Activate this home now?"
   - If yes, follow "Home Switching Algorithm" above
```

## Workflow Implementation Details

### Status Command Algorithm

**Command:** "status" or "daisy status"

```
1. Detect active home (check $DAISY_HOME environment variable)

2. Count tasks in todo.txt:
   a. Read tasks/todo.txt
   b. Count active tasks (not starting with "x " or "z ")
   c. Count by priority:
      - high_priority = lines starting with "(A) " or "(B) "
      - inbox = lines without priority prefix
      - soon = lines starting with "(C) "
      - someday = lines starting with "(D) "
   d. Count completed tasks this week:
      - Lines starting with "x YYYY-MM-DD" where date >= last Monday

3. Analyze today.md:
   a. Check if exists
   b. Count incomplete tasks: lines with "- [ ]"
   c. Count completed tasks: lines with "- [x]"
   d. Extract date from first heading (### YYYY-MM-DD DayName)

4. Check for overdue tasks:
   a. Parse each line in todo.txt for "due:YYYY-MM-DD"
   b. Compare to today's date
   c. If due date < today, add to overdue list

5. Run sync validation (same as sync command)
   a. Compare today.md vs todo.txt
   b. Count mismatches

6. Check journal.md:
   a. Find last date entry (### YYYY-MM-DD)
   b. Compare to today's date

7. Report formatted status:
   📊 Daisy Status ({date} {day})
   
   Home: {name}
   Tasks: {N} active ({high} high-priority, {overdue} overdue)
   Today: {incomplete} incomplete, {completed} completed
   Journal: Last entry {last_date}
   Sync: {status}
   
   [If overdue tasks exist:]
   Overdue tasks:
   - {task 1}
   - {task 2}
   
   [If sync issues exist:]
   ⚠️ Sync issues: {N} discrepancies
   Run "sync tasks" to fix
```

### Add Task Algorithm

**Command:** "add task [description]"

```
1. Parse user input:
   a. Extract description after "add task" or "new task"
   b. Identify @context labels in description
   c. Identify +PROJECT tags in description
   d. Check for due:YYYY-MM-DD
   e. Check for priority hint: "(A)", "(B)", "(C)", "(D)" in description

2. Determine priority:
   a. If explicit priority in description, use it
   b. If no priority, ask: "Priority? (A=urgent, B=this week, C=soon, D=someday, or Enter for inbox)"
   c. Wait for user response
   d. Default to no priority (inbox) if user just presses Enter

3. Get current date as YYYY-MM-DD

4. Format for todo.txt:
   a. If priority provided: "({priority}) {date} {description}"
   b. If no priority: "{date} {description}"
   c. Preserve all @context and +PROJECT tags
   d. Preserve due:YYYY-MM-DD if present

5. Add to todo.txt:
   a. Read tasks/todo.txt
   b. Find insertion point:
      - If has priority: after last task with same priority, before next priority
      - If no priority: after all (B) tasks, before (C) tasks
   c. Insert new task at correct position
   d. Write updated todo.txt
   e. Report: "✅ Added to todo.txt: {formatted_task}"

6. Add to today.md (if high priority):
   a. If priority is (A) or (B):
      - Convert to markdown format: "- [ ] @context {description}"
      - Find appropriate section in today.md
      - Append to section
      - Report: "✅ Added to today.md: {formatted_task}"
   b. If priority is (C), (D), or none:
      - Report: "ℹ️ Low priority - not added to today.md. Will appear on next 'new day'"

7. Commit changes (call commit.sh)
```

### Change Priority Algorithm

**Command:** "priority [pattern] to [A|B|C|D]"

```
1. Parse command:
   a. Extract task pattern (substring to search)
   b. Extract target priority (A, B, C, D, or none/inbox)
   c. If target priority not specified, ask: "Change to which priority? (A/B/C/D/none)"

2. Find matching task in todo.txt:
   a. Read tasks/todo.txt
   b. Search for active tasks (not starting with "x " or "z ") matching pattern
   c. If multiple matches:
      - List all with numbers
      - Ask: "Which one? (1-N, or provide more specific pattern)"
      - Wait for user response
   d. If no match:
      - Report: "⚠️ No task found matching: {pattern}"
      - Return

3. Update todo.txt:
   a. Parse matched task: ^(\([A-D]\) )?({date} )(.*)$
   b. Extract date (group 2) and description (group 3)
   c. Remove from current position
   d. Reformat with new priority:
      - If target is A/B/C/D: "({priority}) {date} {description}"
      - If target is "none" or "inbox": "{date} {description}"
   e. Find new insertion point (among tasks with same priority)
   f. Insert task at new position
   g. Write updated todo.txt
   h. Report: "✅ Updated todo.txt: {old_priority} → {new_priority}"

4. Update today.md (if exists in today.md):
   a. Read today.md
   b. Search for matching task line (case-insensitive)
   c. If found:
      - Determine old section (Now, Next, Inbox, etc.)
      - Remove from old section
      - Determine new section based on new priority:
        * (A) → Now
        * (B) → Next
        * None → Inbox
        * (C) or (D) → Not in today.md (remove if present)
      - If new priority is (A) or (B), add to appropriate section
      - Write updated today.md
      - Report: "✅ Updated today.md: moved to {new_section}"
   d. If not found and new priority is (A) or (B):
      - Ask: "Task not in today.md. Add it now?"
      - If yes, convert to markdown and add to appropriate section
   e. If not found and priority is (C), (D), or none:
      - Report: "ℹ️ Task will appear in today.md at next 'new day'"

5. Commit changes (call commit.sh)
```

### Logging Work - Abridged Archival

**IMPORTANT:** Abridging only happens during **weekly review**, never during daily archival.

When starting a new week (step 1 of "Starting a New Week" workflow), optionally curate quiet days in journal.md:

```
Goal: Create useful historical record without verbose minutiae (ONLY during weekly review)

Preserve (never lose):
- Stakeholder interactions: "Met with ~person", "~person decided"
- Task progress: "Completed", "Blocked by", "Started"
- Discoveries: "Found", "Discovered", "Traced to"
- Decisions: "Decided to", "Chose", "Approved"
- Milestones: "Opened PR", "Merged", "Released"

Condense:
- Multiple "working on" entries → Time range + outcome
  Example: "0930, 1015, 1045 working on X" → "0930-1200 - Investigated X, found Y"
- Routine status updates → Omit if outcome is logged

Format:
- Time ranges for extended work: "0930-1200 - {activity and outcome}"
- Explicit times for events: "1130 - Met with ~person about X"
- Keep stakeholder aliases: ~person format preserved
```

**Archival Example:**

**Original today.md log:**
```
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

**Abridged for journal.md:**
```
- 0930-1445 - Investigated PROJ-1234, found race condition in adapter init
- 1215 - Met with ~jdoe about approach, decided instance-based pattern
- 1530-1600 - PR#1545 opened, approved by ~jdoe, merged
```

## Reference - Validation After Modifications

After modifying any file, verify:

### Todo.txt Validation

- All active tasks: `(P) YYYY-MM-DD Description` or `YYYY-MM-DD Description`
- All completed tasks: `x YYYY-MM-DD YYYY-MM-DD Description` (no priority)
- All cancelled tasks: `z YYYY-MM-DD YYYY-MM-DD Description` (no priority)
- Completed tasks before cancelled at end of file
- No duplicate lines
- All dates are YYYY-MM-DD format

### Today.md Validation

- All markdown headings preserved (####)
- Checkbox format correct: `- [ ]`, `- [x]`, `- [z]`
- Log entries format: `- HHMM - message`
- No extra blank lines added
- All sections present

### Journal.md Validation

- Entries in chronological order (oldest first)
- Stakeholder interactions preserved
- Task progress documented
- Proper blank line separation between entries

## Reference - Cross-File Consistency

When completing a task, ensure:
1. today.md checkbox changes `[ ]` → `[x]`
2. tasks/todo.txt gets `x YYYY-MM-DD` prefix
3. Priority STRIPPED from completed task
4. Completed task moves to END of todo.txt
5. Log entry added documenting completion
6. No other changes to either file

When referencing people:
1. Always use `~alias` from tasks/alias.txt
2. NEVER use bare names or emails
3. Check alias.txt if unsure

## Reference - Timestamps

- Use 24-hour format: `1430` not `2:30 PM`
- No colons in times: `1430` not `14:30`
- Default to Pacific Time unless specified otherwise
- When converting: explicitly note "converted to Pacific Time"

## Script Implementation Notes

Current scripts (`new-day.sh`, `new-week.sh`, `done.sh`, `log.sh`) implement basic workflows but do NOT yet implement:

- Priority floor rules (+URGENT, +OVERDUE, +TODAY)
- Task preservation from yesterday's today.md
- Quiet day consolidation
- Advanced sync validation

These features are documented here for future implementation.

## See Also

- **`@daisy/prompts/daisy.md`** - User-focused workflows
- **`@daisy/docs/todotxt.md`** - Full todo.txt format specification
- **`@daisy/docs/examples/daisy.md`** - Complete interaction examples
