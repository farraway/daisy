# Workflow Implementation Details

Detailed algorithms for agent-driven workflows. For user-facing command summaries, see `prompts/daisy.md`.

## Status Command

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
   Daisy Status ({date} {day})
   
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
   Sync issues: {N} discrepancies
   Run "sync tasks" to fix
```

## Add Task

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
   e. Report: "Added to todo.txt: {formatted_task}"

6. Add to today.md (if high priority):
   a. If priority is (A) or (B):
      - Convert to markdown format: "- [ ] @context {description}"
      - Find appropriate section in today.md
      - Append to section
      - Report: "Added to today.md: {formatted_task}"
   b. If priority is (C), (D), or none:
      - Report: "Low priority - not added to today.md. Will appear on next 'new day'"

7. Commit changes (call commit.sh)
```

## Change Priority

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
      - Report: "No task found matching: {pattern}"
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
   h. Report: "Updated todo.txt: {old_priority} -> {new_priority}"

4. Update today.md (if exists in today.md):
   a. Read today.md
   b. Search for matching task line (case-insensitive)
   c. If found:
      - Determine old section (Now, Next, Inbox, etc.)
      - Remove from old section
      - Determine new section based on new priority:
        * (A) -> Now
        * (B) -> Next
        * None -> Inbox
        * (C) or (D) -> Not in today.md (remove if present)
      - If new priority is (A) or (B), add to appropriate section
      - Write updated today.md
      - Report: "Updated today.md: moved to {new_section}"
   d. If not found and new priority is (A) or (B):
      - Ask: "Task not in today.md. Add it now?"
      - If yes, convert to markdown and add to appropriate section
   e. If not found and priority is (C), (D), or none:
      - Report: "Task will appear in today.md at next 'new day'"

5. Commit changes (call commit.sh)
```

## New Day

**Command:** "Daisy, start a new day" or "new day"

**Pre-workflow:** Check yesterday's retrospective in today.md. If the Retrospective section is incomplete, offer to complete it before proceeding.

```
1. Call new-day.sh
   What it does:
   a. Archives yesterday's today.md content to journal.md
   b. Deletes tasks with "z" prefix from todo.txt (cancelled tasks)
   c. Reads todo.txt and extracts:
      - Priority A tasks → Now section
      - Priority B tasks → Next section
      - No-priority tasks → Inbox section
      - (C) and (D) tasks are NOT carried into today.md
   d. Generates new today.md from journal-day.md template
   e. Auto-commits

   Does NOT archive completed tasks to done.txt.
   Completed tasks (x prefix) stay in todo.txt until new-week.sh.
```

**Post-workflow:** Remind about daily inbox checklist:
- Check calendar for upcoming events
- Workout
- Check that todo.txt is up-to-date
- Plan day
- Retrospective

---

## New Week

**Command:** "Daisy, start a new week" or "new week"

**Pre-workflow:** Same as New Day — check yesterday's retrospective first.

```
1. Call new-week.sh
   What it does (everything new-day does, PLUS):
   a. Archives completed tasks (x prefix) from todo.txt → done.txt
      (This is the ONLY workflow that moves tasks to done.txt)
   b. Uses journal-week.md template, which adds:
      - Weekly retrospective section
      - Resolutions section
      - Extended inbox checklist
```

**Post-workflow:** Remind about weekly inbox checklist:
- Retrospective for previous week
- Set resolutions for this week
- Sync todo.txt with @jira and @github
- Zero Email Inboxes
- Zero Chat Notifications
- Check calendar, workout, check todo.txt, plan day, retrospective

---

## Complete Task

**Command:** "Daisy, done [pattern]" or "done [pattern]"

```
1. Find task by pattern in todo.txt (case-insensitive)
   - If multiple matches: list them and ask which one
   - If no match: report "No task found matching: {pattern}"

2. Mark complete in today.md:
   - Change "- [ ]" to "- [x]" on the matching line

3. Mark complete in todo.txt:
   - Strip priority prefix (e.g., "(A) ")
   - Add "x YYYY-MM-DD " prefix

4. Add log entry with timestamp

5. Commit changes (call done.sh "pattern")

NOTE: The completed task stays in todo.txt (marked with "x" prefix)
until the next new-week.sh archives it to done.txt.
```

---

## Cancel Task

**Command:** "Daisy, cancel [pattern]"

```
1. Find task by pattern (case-insensitive)

2. Mark cancelled in today.md:
   - Change "- [ ]" to "- [z]" on the matching line

3. Mark cancelled in todo.txt:
   - Strip priority prefix
   - Add "z YYYY-MM-DD " prefix

4. Auto-commit

NOTE: Cancelled tasks are soft-deleted. They remain in todo.txt
with the "z" prefix until the next new-day.sh or new-week.sh
deletes them.
```

---

## Sync Validation

**Command:** "Daisy, sync tasks" or "Daisy, check sync"

```
1. Read today.md task lines (- [ ], - [x], - [z])
2. Read todo.txt active tasks
3. Compare:
   a. Priority mismatches: task in Now section but not (A) in todo.txt
   b. Completion mismatches: task marked [x] in today.md but not "x" in todo.txt
   c. Missing tasks: (A) or (B) tasks in todo.txt not in today.md
4. Report discrepancies
5. Offer to fix automatically
```

---

## Home Switching

**Command:** "Daisy, switch to [home]"

```
1. Run: daisy init <home-name>
   - Replaces .daisy/ symlinks in the current workspace to point to the new home
   - Does not affect other workspaces
   - Each workspace independently tracks its own home via .daisy/home

2. Verify switch: read .daisy/home to confirm new home name
```

To create a new home from scratch:
```
daisy create-home <home-name> [--activate]
```
This copies daisy/templates/home/ to home/{name}/ and optionally runs daisy init.
