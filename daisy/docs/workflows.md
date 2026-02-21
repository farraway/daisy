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
