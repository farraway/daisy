# Daily Journal Workflow Prompt

You are assisting with a personal productivity system that uses todo.txt format and daily markdown journals.

## File Structure

### Task Management

**`@daisy/todo.txt`** - Active tasks following the todo.txt format (see `@daisy/docs/todotxt.md` for specification):
- Format: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Priorities: (A) = highest, (B) = high, (C) = normal, (D) = low
- Common contexts: `@jira` (Jira tickets), `@github` (Pull requests), `@review` (needs attention)
- Common projects: `+WXSA-XXXXX` (Jira keys), `+FYXXQX` (fiscal quarters)

**`@daisy/done.txt`** - Completed tasks:
- Format: `x YYYY-MM-DD YYYY-MM-DD Description +Project @context`
- First date = completion date, second date = creation date
- Special context: `@cancelled` for tasks closed without implementation

### Journal System

**`@daisy/journal.md`** - Archive of all past daily entries (append-only)

**`@daisy/today.md`** - Current day's work log (gets archived at day's end)

**`@daisy/templates/journal-day.md`** - Template for new daily entries

## Daily Workflow

### Starting a New Day

When the user says "start a new day" or "new day":

1. **Archive yesterday's work:**
   - If `today.md` exists and has content, append it to `journal.md` with a blank line separator
   
2. **Create new `today.md`** using the template with these substitutions:
   - `{DATE}` → Current date as `YYYY-MM-DD`
   - `{DAY}` → Day of week (Monday, Tuesday, etc.)
   - `{TIME}` → Current time as `HHMM` (24-hour format)
   - `{HIGH_PRIORITY_TASKS}` → Extract from `todo.txt`:
     - Lines starting with `(A)` or `(B)`
     - Exclude lines containing `@github`
     - Format as: `- [ ] {task description}` (strip priority and date)
   - `{GITHUB_TASKS}` → Extract from `todo.txt`:
     - All lines containing `@github` (any priority)
     - Format as: `- [ ] {task description}` (strip priority and date)

### During the Day

**Logging work:**
- When user says "log [message]", append to the `#### Log` section:
  - Format: `* HHMM - {message}`

**Completing tasks:**
- When user says "done [task pattern]", mark the matching task in `today.md`:
  - Change `- [ ]` to `- [x]`
  - Also move the task from `todo.txt` to `done.txt` with today's date

**Updating retrospective:**
- User can update the three retrospective bullets at any time during or at end of day

## Example Interaction

**User:** "Start a new day"

**You:** 
1. Check if `today.md` has content → append to `journal.md`
2. Read `todo.txt` to extract priority A/B tasks and @github tasks
3. Create new `today.md` with populated template
4. Confirm: "✅ New day started: 2026-01-06 Monday"

**User:** "Log closed WXSA-15770 as will not implement"

**You:** Add to Log section: `* 1345 - Closed WXSA-15770 as will not implement`

**User:** "Done WXSA-18369"

**You:** 
1. Mark task in `today.md`: `- [x] Avengers pages must provide valid meeting links +WXSA-18369 +FY26Q2 @jira`
2. Move from `todo.txt` to `done.txt`: `x 2026-01-06 2025-12-23 Avengers pages must provide valid meeting links +WXSA-18369 +FY26Q2 @jira`

## Key Principles

- **Preserve format:** Always maintain todo.txt specification exactly
- **Date accuracy:** Use ISO format `YYYY-MM-DD` consistently
- **Task tracking:** Keep `todo.txt`, `done.txt`, and `today.md` synchronized
- **Archive integrity:** Never modify `journal.md` except to append new days
- **Context awareness:** Pay attention to @jira vs @github to categorize work properly

