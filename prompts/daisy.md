# Daisy - Personal Productivity System Workflow

You are assisting with a personal productivity system that uses todo.txt format and daily markdown journals.

This document provides the complete, authoritative specification for all daisy workflows. Follow the algorithms documented below exactly.

## File Structure

The system uses symlinks in the repo root for home switching:

- `prompt.md` → symlink to active home's bootstrap prompt
- `tasks/` → symlink to active home's task directory (contains todo.txt, done.txt, alias.txt)
- `journal.md` → symlink to active home's journal archive
- `today.md` → symlink to active home's current day journal

Actual data lives in `home/{home}/` directories (e.g., `home/work/`, `home/personal/`).

**Important - Symlink Resolution:**
- These are symbolic links, not actual files
- When reading/writing, always follow the symlink to the target file
- Example: `today.md` → `home/work/journal/today.md` (actual file location)
- Never modify the symlinks themselves
- All file operations work on the resolved target files in `home/{home}/`

### Task Management Files

**`tasks/todo.txt`** - Active tasks following the todo.txt format:
- Format: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Priorities: (A) = now, (B) = next, (C) = soon, (D) = someday
- Tasks without priority = B+ (inbox, needs triage)
- Completed tasks are marked with `x YYYY-MM-DD` prefix (priority stripped) and moved to end of file
- Cancelled tasks are marked with `z YYYY-MM-DD` prefix and moved to end of file
- Tasks stay in todo.txt until weekly archival or cleanup

**`tasks/done.txt`** - Archived completed tasks:
- Format: `x YYYY-MM-DD YYYY-MM-DD Description +Project @context`
- First date = completion date, second date = creation date
- Tasks are moved here during weekly rollover

**`tasks/alias.txt`** - People/role reference mapping:
- Format: `~alias Full Name <email@domain.com> #tag1 #tag2`
- Example: `~deaclose Dean Close <deaclose@cisco.com> #me #dean`
- Use `~alias` format consistently across all files

### Journal Files

**`journal.md`** - Archive of all past daily entries (append-only, curated weekly)

**`today.md`** - Current day's work log (gets archived at day's end)

**`templates/journal-day.md`** - Template for daily entries

**`templates/journal-week.md`** - Template for weekly entries

## Task Priority System

Tasks in `tasks/todo.txt` use a priority prefix to indicate urgency and importance:

### Priority Levels

#### (A) - Now (Urgent & Important)
- Do today, critical
- Blocks other work
- Immediate action required
- Examples: Production bugs, deadline today, emergency requests

#### (B) - Next (Important, Not Urgent)
- Do this week
- High value work
- Scheduled commitments
- Examples: Sprint commitments, stakeholder requests, important features

#### (C) - Soon (Moderate Priority)
- Do when A/B complete
- Default priority for most tasks
- Standard backlog work
- Examples: Feature work, improvements, technical debt

#### (D) - Someday (Low Priority / Backlog)
- Nice to have
- No deadline
- Long-term aspirations
- Examples: Learning goals, exploratory work, "wouldn't it be cool if..."

### Tasks Without Priority Prefix (B+ / Inbox)

Tasks without a priority prefix (e.g., `2026-01-15 Task description +Project`) represent **B+ priority** - the task inbox:

- **Rank:** Between (B) and (C)
- **Not High-Priority:** Don't appear in High-Priority section
- **Need triage:** Should be reviewed and assigned proper priority
- **Extraction:** Appear in `Task Inbox` section of `today.md`

**Purpose:** Capture tasks quickly without forcing immediate prioritization.

**Example:**
```
(A) 2026-01-15 Critical bug fix                      ← High-Priority
(B) 2026-01-15 Sprint commitment                     ← High-Priority
2026-01-15 Important but needs triage +Project       ← Task Inbox (B+)
(C) 2026-01-15 Standard work                         ← Soon
(D) 2026-01-15 Someday item                          ← Someday
```

### Task Sections in today.md

Daily journal (`today.md`) organizes tasks into three sections:

1. **High-Priority Tasks** - `{HIGH_PRIORITY_TASKS}`
   - Priority (A) or (B) only
   - Excludes @git/@github tasks (they have their own section)

2. **Task Inbox** - `{TASK_INBOX}`
   - Tasks without priority prefix
   - Excludes @git/@github tasks
   - Review and assign priority during the day

3. **GitHub PRs** - `{GITHUB_TASKS}`
   - All tasks with @git or @github home
   - Any priority level or no priority

### Task Format Specification

#### Active Task Format

```
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

#### Completed Task Format

```
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

#### Cancelled Task Format

```
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

### Task-to-Markdown Conversion

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
| `(B) 2026-01-15 @git @jira Multi-home task` | `- [ ] @git @jira Multi-home task` |

**Edge cases:**
- No contexts in body → Just description (no home prefix)
- Home in middle of description → Move all contexts to front

### Task File Organization

**todo.txt Structure:**

```
Active tasks (top of file):
(A) 2026-01-15 High priority task
(B) 2026-01-15 Next priority task
2026-01-15 Inbox task (needs triage)
(C) 2026-01-15 Soon task
(D) 2026-01-15 Someday task

Completed tasks (end of file, before cancelled):
x 2026-01-15 2026-01-13 Completed task 1
x 2026-01-15 2026-01-14 Completed task 2

Cancelled tasks (end of file, after completed):
z 2026-01-15 2026-01-14 Cancelled task 1
z 2026-01-15 2026-01-13 Cancelled task 2
(deleted during next new day/week)
```

**done.txt Structure:**

```
Archived completed tasks:
x 2026-01-08 2026-01-05 Old completed task 1
x 2026-01-09 2026-01-06 Old completed task 2
x 2026-01-10 2026-01-07 Old completed task 3
```

Tasks moved here during weekly archival to keep `todo.txt` focused on active work.

### Special Cases: @git/@github Home Extraction

Tasks with `@git` or `@github` home are extracted separately regardless of priority:

```
(A) 2026-01-15 @git Critical PR +PROJ-1234
→ Goes to "GitHub PRs" section, NOT "High-Priority"

2026-01-15 @git Review PR (no priority)
→ Goes to "GitHub PRs" section

(B) 2026-01-15 @git @jira Multi-home task
→ Goes to "GitHub PRs" section (has @git)
```

**Rationale:** GitHub tasks need focused attention in their own workflow section.

## CRITICAL: Format Preservation Rules

When modifying files, preserve EXACT formatting:

### Todo.txt Format
- Active: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Completed: `x YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- Cancelled: `z YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- NEVER change date format (always `YYYY-MM-DD`)
- NEVER add/remove priority parentheses on active tasks
- NEVER preserve priority on completed/cancelled tasks
- NEVER reorder fields
- Preserve ALL spaces exactly

### Today.md Format
- Preserve markdown heading levels (`####`)
- Preserve checkbox format: `- [ ]`, `- [x]`, `- [z]` (note spaces)
- NEVER add/remove blank lines between sections
- Time format: `HHMM` (24-hour, no colons)
- Log entry format: `- HHMM - message` (note dashes and spaces)

### Journal.md Archive Rules
- Append daily entries (curated/abridged)
- MAY be modified ONLY to:
  - Consolidate quiet days during weekly review
  - Update person references to canonical ~alias format
  - Fix chronological ordering errors
- NEVER delete stakeholder interactions or task progress
- NEVER modify retrospective content (represents historical perspective)

## Home Management

### Detecting Active Home

The active home is determined by resolving the `prompt.md` symlink in the repo root.

**Algorithm:**
```
1. Check if root/prompt.md symlink exists
   - If missing, error: "⚠️ No active home. Run home setup."
2. Resolve symlink to get target path
   - Example: prompt.md → home/work/prompt.md
3. Extract home name from path:
   - Target: home/work/prompt.md → Home: "work"
4. Read the resolved prompt.md file
5. Parse "Home Declaration" section to get requirements
```

### System Health Check

**User says:** "check system" or "verify setup" or "check home"

**Algorithm:**
```
1. **Detect active home:**
   a. Check if root/prompt.md exists
   b. If not: Report "⚠️ No active home detected. prompt.md symlink missing."
   c. Resolve prompt.md symlink to target path
   d. Extract home name from path (e.g., home/work/prompt.md → "work")

2. **Read Home Declaration:**
   a. Read the resolved prompt.md file
   b. Parse "Home Declaration" section
   c. Extract "Required Files" list
   d. Extract "Required Symlinks" list

3. **Verify Required Files:**
   a. For each file in Required Files list:
      - Check if file exists at declared path
      - Report: ✅ or ⚠️ Missing: {path}
   b. Collect missing files into list

4. **Verify Required Symlinks:**
   a. For each symlink in Required Symlinks list:
      - Check if symlink exists in repo root
      - Verify symlink points to correct target
      - Check if target file exists
      - Report: ✅ {link} → {target} or ⚠️ Issue with {link}
   b. Collect issues into list

5. **Validate todo.txt format (if exists):**
   a. Read tasks/todo.txt
   b. For each line:
      - Skip empty lines
      - Verify matches active/completed/cancelled format
      - Check dates are YYYY-MM-DD format
      - Check priority format (A-D) if present
   c. Find format violations
   d. Report: ✅ Format valid or ⚠️ Format issues: {list}

6. **Check for orphaned completed tasks:**
   a. Count lines starting with "x " not at end of active section
   b. Count lines starting with "z " anywhere (should be 0 after cleanup)
   c. Report: ✅ or ℹ️ {N} completed tasks need archival

7. **Final Report:**
   ✅ Home: {name}
   ✅ All required files present
   ✅ All symlinks correct
   ✅ Todo.txt format valid
   ✅ No orphaned tasks
   
   OR
   
   ⚠️ Home: {name} - Issues found:
   - Missing files: {list}
   - Symlink issues: {list}
   - Format issues: {list}
   - Orphaned tasks: {N}
   
   Suggestions:
   - Create missing files from templates
   - Re-run home setup: "switch to {name}"
   - Archive completed tasks: "start a new week"
```

**Example Interaction:**

```
User: check system

AI: 
✅ System Health Check

Home: work
Required Files: ✅ All 6 files present
Symlinks:
  ✅ prompt.md → home/work/prompt.md
  ✅ tasks → home/work/tasks
  ✅ journal.md → home/work/journal/journal.md
  ✅ today.md → home/work/journal/today.md
Format: ✅ todo.txt format valid (45 tasks)
Status: ✅ System healthy
```

### Switching Homes

**User says:** "Switch to [home]" or "Use [home] home"

**Algorithm:**
```
1. **Detect current home:**
   a. Resolve root/prompt.md symlink
   b. Read current home's prompt.md
   c. Parse "Required Symlinks" declaration

2. **Teardown current home:**
   a. For each symlink declared in current prompt.md:
      - Remove the symlink from repo root
   b. Report: "Deactivated home: {current}"

3. **Verify target home exists:**
   a. Check if home/{home}/prompt.md exists
   b. If not found, offer: "Home '{home}' not found. Create from template?"

4. **Setup new home:**
   a. Read home/{home}/prompt.md
   b. Parse "Required Symlinks" declaration
   c. For each declared symlink:
      - Create symlink from root to target in home/{home}/
   d. Report: "✅ Activated home: {home}"

5. **Verify setup:**
   a. Parse "Required Files" declaration from new home's prompt.md
   b. Check if each file exists at declared path
   c. If any missing, warn: "⚠️ Missing files: [list]"
   d. Offer to create missing files from templates
```

**Example Interaction:**

```
User: switch to personal

AI:
Deactivated home: work
✅ Activated home: personal

Verified files:
  ✅ tasks/todo.txt
  ✅ tasks/done.txt
  ✅ journal/journal.md
  ⚠️ Missing: journal/today.md

Would you like me to create today.md? (say "start a new day")
```

### Creating New Home

**User says:** "Create home [name]" or "New home [name]"

**Algorithm:**
```
1. Check if home/{name}/ exists
   - If exists, error: "Home '{name}' already exists"

2. Copy templates/home/ to home/{name}/

3. Instruct user to customize home/{name}/prompt.md:
   - Replace all `[home]` with {name}
   - Replace all `[Home Name]` with descriptive name
   - Choose which prompts to load (adjust Home-Specific Prompts section)

4. Ask: "Activate this home now?"
   - If yes, follow "Switching Homes" algorithm above
```

**Example Interaction:**

```
User: create home personal

AI:
✅ Created new home directory: home/personal/
✅ Copied template files

Next steps:
1. Customize home/personal/prompt.md (replace [home] with "personal")
2. Choose which prompts to load (edit Home-Specific Prompts section)
3. Activate: "switch to personal"

Would you like me to activate this home now?
```

## Workflows

### Starting a New Week

**User says:** "start a new week" or "new week"

**Algorithm:**

1. **Review past week for quiet days:**
   ```
   a. Scan journal.md entries since last week
   b. Identify "quiet days":
      - Log has ≤ 2 entries (e.g., just "New day started")
      - No tasks marked complete (no `[x]` checkboxes)
      - Retrospective is empty OR only generic content
      - No stakeholder interactions (~person mentions)
   c. If 2+ quiet days found, offer consolidation:
      "Found 3 quiet days (Mon, Wed, Fri). Consolidate into single entry?"
   d. If user agrees:
      - Replace quiet days with:
        "### 2026-01-13, 2026-01-15, 2026-01-17 (Light Activity Days)
         
         Minimal logged activity. Focus on other priorities."
   ```

2. **Delete cancelled tasks:**
   ```
   a. Read tasks/todo.txt
   b. Remove all lines starting with "z "
   c. Rewrite tasks/todo.txt without cancelled tasks
   d. Report: "🗑️ Deleted N cancelled tasks" (if N > 0)
   ```

3. **Archive completed tasks from todo.txt to done.txt:**
   ```
   a. Read tasks/todo.txt
   b. Separate into two lists:
      - active: lines NOT starting with "x "
      - completed: lines starting with "x "
   c. If completed list not empty:
      - Append each completed task to tasks/done.txt
      - Rewrite tasks/todo.txt with ONLY active tasks
      - Report: "📦 Archived N completed tasks"
   d. If completed list empty:
      - Report: "ℹ️ No completed tasks to archive"
   ```

4. **Follow daily startup process** (see below)

**Example Interaction:**

```
User: start a new week

AI:
Found 2 quiet days (Mon, Wed). Consolidate into single entry?

User: yes

AI:
✅ Consolidated 2 quiet days in journal.md
🗑️ Deleted 3 cancelled tasks
📦 Archived 12 completed tasks to done.txt
✅ New day started: 2026-01-20 Monday
   High priority tasks: 5
   Inbox tasks: 8
   GitHub tasks: 2
```

### Starting a New Day

**User says:** "start a new day" or "new day"

**Algorithm:**

1. **Archive yesterday's work:**
   ```
   a. Check if today.md exists
   b. If exists, read entire contents
   c. Create abridged/curated version:
      - Keep date heading verbatim
      - Summarize completed tasks (just titles)
      - Abridge log section (see Logging section below)
      - Keep retrospective verbatim
   d. Append abridged version to journal.md:
      - Open journal.md in append mode
      - If journal.md has content, write "\n\n" (two newlines)
      - Write abridged today.md contents
      - Write "\n" (one final newline)
      - Report: "📦 Archived yesterday's work to journal.md"
   ```

2. **Delete cancelled tasks:**
   ```
   a. Read tasks/todo.txt
   b. Remove all lines starting with "z "
   c. Rewrite tasks/todo.txt
   d. Report: "🗑️ Deleted N cancelled tasks" (if N > 0)
   ```

3. **Extract high priority tasks from todo.txt:**
   ```
   a. Check if tasks/todo.txt exists
   b. If not exists or empty, set high_priority_tasks = []
   c. For each line in todo.txt:
      - Skip if empty or starts with "x " or "z " (completed/cancelled)
      - Check if starts with "(A) " or "(B) "
      - Check if contains "@git" or "@github" (case-insensitive)
      - If priority A/B AND NOT git/github:
         * Parse using regex: ^(\([A-D]\) )?({date} )(.*)$
         * Extract description (group 3)
         * Parse for @context labels, move to front
         * Format as: "- [ ] @context Description"
         * Add to high_priority_tasks list
   d. If list empty, set to: ["- [ ] No high priority tasks"]
   ```

4. **Extract inbox tasks from todo.txt:**
   ```
   a. For each line in todo.txt:
      - Skip if starts with "x ", "z ", or has priority "(A-D)"
      - Skip if contains "@git" or "@github"
      - If matches (no priority, no git):
         * Parse using regex: ^({date} )(.*)$
         * Extract description (group 2)
         * Parse for @context labels, move to front
         * Format as: "- [ ] @context Description"
         * Add to inbox_tasks list
   b. If list empty, set to: ["- [ ] No inbox tasks"]
   ```

5. **Extract GitHub tasks from todo.txt:**
   ```
   a. For each line in todo.txt:
      - Skip if starts with "x " or "z "
      - Check if contains "@git" or "@github" (case-insensitive)
      - If found:
         * Parse using regex: ^([xz] )?(\([A-D]\) )?({date} )?({date} )?(.*)$
         * Extract description (group 5)
         * Parse for @context labels, move to front
         * Format as: "- [ ] @git Description" (or @github)
         * Add to github_tasks list
   b. If list empty, set to: ["- [ ] No GitHub tasks"]
   ```

6. **Create new today.md from template:**
   ```
   a. Read templates/journal-day.md
   b. Get current date/time in Pacific Time
   c. Perform string replacements:
      - {DATE} → YYYY-MM-DD (e.g., "2026-01-15")
      - {DAY} → Day name (e.g., "Thursday")
      - {TIME} → HHMM (e.g., "1404")
      - {HIGH_PRIORITY_TASKS} → joined high_priority_tasks (one per line)
      - {TASK_INBOX} → joined inbox_tasks (one per line)
      - {GITHUB_TASKS} → joined github_tasks (one per line)
   d. Write result to today.md
   ```

7. **Confirm to user:**
   ```
   Report:
   "✅ New day started: YYYY-MM-DD DayName"
   "   High priority tasks: N"
   "   Inbox tasks: M"
   "   GitHub tasks: K"
   ```

**Example Interaction:**

```
User: start a new day

AI:
📦 Archived yesterday's work to journal.md
🗑️ Deleted 1 cancelled task
✅ New day started: 2026-01-16 Thursday
   High priority tasks: 3
   Inbox tasks: 5
   GitHub tasks: 2

Today's focus:
- [ ] @jira Complete PROJ-1234 implementation
- [ ] @jira Review design doc for PROJ-1235
- [ ] @meeting Sprint planning at 1400
```

**Given this todo.txt:**
```
(A) 2026-01-15 Complete PROJ-1234 implementation @jira +PROJ-1234
(B) 2026-01-15 Review design doc @jira +PROJ-1235
(B) 2026-01-15 Sprint planning at 1400 @meeting
2026-01-15 Triage new tickets +INBOX
2026-01-15 Update documentation @jira +PROJ-1236
(B) 2026-01-15 Review PR#1234 @git +PROJ-1237
(C) 2026-01-15 Refactor utils module @jira
x 2026-01-15 2026-01-10 Old completed task
```

**Creates today.md with:**
```markdown
#### High-Priority Tasks
- [ ] @jira Complete PROJ-1234 implementation +PROJ-1234
- [ ] @jira Review design doc +PROJ-1235
- [ ] @meeting Sprint planning at 1400

#### Task Inbox
- [ ] Triage new tickets +INBOX
- [ ] @jira Update documentation +PROJ-1236

#### GitHub PRs
- [ ] @git Review PR#1234 +PROJ-1237
```

### Logging Work

**AI logs proactively as work progresses:**
- After completing significant work: "1045 - Implemented fix for PROJ-1234"
- After discoveries: "1130 - Found race condition in adapter initialization"
- After decisions: "1215 - Decided to use instance-based pattern for thread safety"
- After stakeholder interactions: "1345 - Met with ~person about handoff"
- After milestones: "1530 - PR#1545 merged"

**User can request conversational logging:**

**User says:** "log current work" or "log [message]"

**Algorithm:**
```
1. Get current time in Pacific Time as HHMM

2. If user said "log current work":
   - Review recent conversation (last 5-10 minutes)
   - Synthesize into concise log entry capturing key points
   - Use format: "HHMM - {synthesized summary}"

3. If user provided explicit message:
   - Use message as-is
   - Format: "HHMM - {message}"

4. Append to Log section in today.md:
   - Find "#### Log" heading
   - Append new entry to end of log section
   - Format: "- HHMM - {message}"
   - No chronology checks, just append

5. Confirm: "✅ Logged: {message}"
```

**Example Interaction:**

```
[After 10 minutes of debugging discussion]

User: log current work

AI:
✅ Logged: 1445 - Traced PROJ-1234 to race condition in adapter init, decided to use instance-based singleton pattern
```

**Abridged Archival:**

When archiving yesterday's today.md to journal.md, curate the log section:

```
**Goal:** Create useful historical record without verbose minutiae

**Preserve (never lose):**
- Stakeholder interactions: "Met with ~person", "~person decided"
- Task progress: "Completed", "Blocked by", "Started"
- Discoveries: "Found", "Discovered", "Traced to"
- Decisions: "Decided to", "Chose", "Approved"
- Milestones: "Opened PR", "Merged", "Released"

**Condense:**
- Multiple "working on" entries → Time range + outcome
  Example: "0930, 1015, 1045 working on X" → "0930-1200 - Investigated X, found Y"
- Routine status updates → Omit if outcome is logged

**Format:**
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

### Completing Tasks

**User says:** "done [pattern]"

Pattern matching is case-insensitive substring search. If multiple matches, ask for clarification. Trust conversation to disambiguate.

**Algorithm:**

1. **Mark complete in today.md:**
   ```
   a. Read today.md
   b. Search for lines containing "- [ ]" and pattern (case-insensitive)
   c. If found:
      - Replace "- [ ]" with "- [x]" (exactly 3 characters change)
      - Report: "✅ Marked complete in today.md: {task}"
   d. If multiple matches:
      - List all matches with numbers
      - Ask: "Which one? (or provide more specific pattern)"
      - Wait for user response
   e. If no match:
      - Report: "ℹ️ Task not found in today.md"
   ```

2. **Mark complete in tasks/todo.txt:**
   ```
   a. Get today's date as YYYY-MM-DD
   b. Read tasks/todo.txt
   c. Search for line matching pattern (case-insensitive)
   d. If found:
      - Parse using regex: ^(\([A-D]\) )?({date} )(.*)$
      - Extract date (group 2) and description (group 3)
      - Strip priority if present
      - Create: "x {today} {original_date} {description}"
      - Move to end of file (after active, before cancelled)
      - Report: "✅ Marked complete in todo.txt: {task}"
   e. If multiple matches, use same disambiguation as step 1
   f. If no match:
      - Report: "⚠️ No matching task in todo.txt for: {pattern}"
   ```

3. **Log completion proactively:**
   ```
   - Append to Log section: "- {TIME} - Completed {task description}"
   ```

**Example Interaction:**

```
User: done PROJ-1234

AI:
✅ Marked complete in today.md: @jira Complete PROJ-1234 implementation
✅ Marked complete in todo.txt: Complete PROJ-1234 implementation @jira +PROJ-1234
✅ Logged: 1545 - Completed PROJ-1234 implementation

Task state:
today.md:   - [x] @jira Complete PROJ-1234 implementation +PROJ-1234
todo.txt:   x 2026-01-16 2026-01-15 Complete PROJ-1234 implementation @jira +PROJ-1234
```

**Before (todo.txt):**
```
(A) 2026-01-15 Complete PROJ-1234 implementation @jira +PROJ-1234
(B) 2026-01-15 Review design doc @jira +PROJ-1235
```

**After (todo.txt):**
```
(B) 2026-01-15 Review design doc @jira +PROJ-1235
x 2026-01-16 2026-01-15 Complete PROJ-1234 implementation @jira +PROJ-1234
```

### Cancelling Tasks

**User says:** "cancel [pattern]" or "cancelled [pattern]"

**Algorithm:** Same as completing, but:
- Use "- [z]" checkbox in today.md
- Use "z {today} {original_date} {description}" format in todo.txt
- Move to end after completed tasks
- Will be deleted during next new day/week

**Example Interaction:**

```
User: cancel old task

AI:
✅ Marked cancelled in today.md: Old task description
✅ Marked cancelled in todo.txt: Old task description @context
ℹ️ Will be deleted at next "new day" or "new week"
```

### Updating Retrospective

**User can update at any time**

The three retrospective prompts in today.md:
- **Successes:** What went well today
- **Misses:** What could have been better
- **What would a Sage do next:** Wisdom-based next actions

**User says:** "help me with my retrospective" or updates specific bullet

**Algorithm:**
```
1. If user requests help:
   - Review completed tasks (scan for [x])
   - Review log entries
   - Identify patterns: completions, discoveries, blockers, interactions
   - Suggest content for each bullet
   - Reframe any unprofessional content professionally

2. If user provides content:
   - Locate "#### Retrospective" section
   - Find specific bullet
   - Append/update content after the colon
   - Preserve all other content
```

**Example Interaction:**

```
User: help me with my retrospective

AI:
Based on today's work, here's what I observed:

**Successes:**
- Completed PROJ-1234 after thorough investigation
- Identified root cause (race condition) efficiently
- Collaborated with ~jdoe on approach
- PR merged same day

**Misses:**
- Took 5+ hours to find the race condition
- Could have asked for help earlier

**What would a Sage do next:**
- Document the race condition pattern for team
- Add tests to prevent regression
- Review other adapters for similar issues

Would you like me to add these to your retrospective?
```

## Reference - Validation After Modifications

After modifying any file, verify:

1. **Todo.txt validation:**
   - All active tasks: `(P) YYYY-MM-DD Description` or `YYYY-MM-DD Description`
   - All completed tasks: `x YYYY-MM-DD YYYY-MM-DD Description` (no priority)
   - All cancelled tasks: `z YYYY-MM-DD YYYY-MM-DD Description` (no priority)
   - Completed tasks before cancelled at end of file
   - No duplicate lines
   - All dates are YYYY-MM-DD format

2. **Today.md validation:**
   - All markdown headings preserved (####)
   - Checkbox format correct: `- [ ]`, `- [x]`, `- [z]`
   - Log entries format: `- HHMM - message`
   - No extra blank lines added
   - All sections present

3. **Journal.md validation:**
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

## Key Principles

- **Preserve format:** Never deviate from specifications above
- **Strip priority on completion:** Completed/cancelled tasks never have priority
- **Proactive logging:** Log significant events as work progresses
- **Curate for utility:** Abridge archives to preserve important information
- **Case-insensitive search:** When matching patterns, ignore case
- **Conversational disambiguation:** Trust conversation to clarify ambiguous patterns
- **Home-aware:** Remember active home and its conventions
- **Exact preservation:** When not explicitly changing content, preserve character-for-character

## See Also

- **Detailed examples:** `docs/examples/daisy.md` - Complete interaction examples
- **Todo.txt spec:** `docs/todotxt.md` - Full format specification
