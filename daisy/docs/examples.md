# Daisy Workflow Examples

Concrete examples of how to interact with the daisy productivity system. With `daisy init` and the Cursor rule installed, just address Daisy by name.

## Table of Contents

1. [Session Startup](#session-startup)
2. [Starting a New Day](#starting-a-new-day)
3. [Logging Work](#logging-work)
4. [Completing Tasks](#completing-tasks)
5. [Starting a New Week](#starting-a-new-week)
6. [End of Day Retrospective](#end-of-day-retrospective)
7. [Multi-File Updates](#multi-file-updates)
8. [Home Switching](#home-switching)

---

## Session Startup

### Example 1: Start Day (Cursor Rule Auto-Loads)

With the Cursor rule installed via `daisy init`, the agent reads `.daisy/AGENTS.md` automatically when you address Daisy.

**User:**
```
Daisy, start a new day
```

**AI Response:**
```
📦 Archived yesterday's work to journal.md
✅ New day started: 2026-01-15 Thursday

📋 High Priority Tasks (8):
- Implement PagerDuty Adapter using ServiceMetadata +PROJ-1234 +FY26Q2
- @jira Modify promote/demote commands to accept incident type arguments +PROJ-1101 +FY26Q2
- Ask ~jdoe about coverage during leave
- Provide ETA to ~mgarcia for CC work due:2026-01-15 +FY26Q2
- Complete Certificate Training before leave
- Complete handoff documentation starts:2026-01-21
- Get tax documents next week
- On leave from Jan 21st starts:2026-01-21

🔧 GitHub PRs (3):
- @git Template pagerduty utility [PR#1545]
- @git Implement GET /incidents/{id}/roomMeetingInfo [PR#154]
- @git Refactor incident creation workflows [PR#1541]

Ready for commands.
```

### Example 2: Check Status Without Starting Day

**User:**
```
Daisy, what are my tasks?
```

**AI Response:**
```
✅ Daisy loaded (work context). Current high-priority tasks (from today.md):
- [ ] Implement PagerDuty Adapter using ServiceMetadata +PROJ-1234
- [ ] Provide ETA to ~mgarcia for CC work due:2026-01-15

MCP integrations available: JIRA, GitHub, Webex
```

---

## Starting a New Day

### Template Variable Substitution

When creating `today.md` from `daisy/templates/journal-day.md`, the system substitutes:

**Template:**
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

**Becomes (with substitutions):**
```markdown
### 2026-01-15 Thursday

#### Agenda
- 0930 Plan Day
- 1230 ...
- 1530 ...

#### Tasks

**Now:**
- [ ] Implement PagerDuty Adapter using ServiceMetadata +PROJ-1234 +FY26Q2
- [ ] Ask ~jdoe about coverage during leave
- [ ] Provide ETA to ~mgarcia for CC work due:2026-01-15 +FY26Q2

**Next:**
- [ ] @jira Modify promote/demote commands +PROJ-1101 +FY26Q2

**Inbox:**
- [ ] Check calendar for upcoming events
- [ ] Check that todo.txt is up-to-date
- [ ] Plan day
- [ ] Retrospective

**GitHub PRs:**
- [ ] @git Template pagerduty utility [PR#1545]
- [ ] @git Implement GET /incidents/{id}/roomMeetingInfo [PR#154]

#### Log

- 0930 New day started

#### Retrospective

* **Successes:** 
* **Misses:** 
* **What would a Sage do next:** 
```

### Full Workflow Steps

**User:** "Daisy, start a new day"

**AI executes:**

1. **Check for yesterday's work:**
   - Read `.daisy/today.md`
   - If content exists, append to `.daisy/journal.md` with blank line separator

2. **Extract tasks from `.daisy/tasks/todo.txt`:**
   - Priority A and B tasks (excluding @git/@github)
   - All @git/@github tasks (any priority)
   - Strip priority and date prefixes

3. **Apply template substitutions**

4. **Write new `.daisy/today.md`**

5. **Confirm to user**

---

## Logging Work

### Example 1: Simple Log Entry

**User:**
```
Daisy, log completed PROJ-1234 implementation
```

**AI Response:**
```
✅ Logged: completed PROJ-1234 implementation
```

**Result in `.daisy/today.md`:**
```markdown
#### Log

- 0930 New day started
- 1523 - completed PROJ-1234 implementation
```

### Example 2: Log with Context

**User:**
```
Daisy, log met with ~bsmith about PROJ-1100 frontend - he's taking ownership
```

**AI Response:**
```
✅ Logged: met with ~bsmith about PROJ-1100 frontend - he's taking ownership
```

**Result in `.daisy/today.md`:**
```markdown
#### Log

- 0930 New day started
- 1523 - completed PROJ-1234 implementation
- 1545 - met with ~bsmith about PROJ-1100 frontend - he's taking ownership
```

### Example 3: Log a Decision

**User:**
```
Daisy, log decided to use instance-based adapter pattern for PagerDuty to avoid race conditions
```

**AI Response:**
```
✅ Logged: decided to use instance-based adapter pattern for PagerDuty to avoid race conditions
```

---

## Completing Tasks

### Example 1: Complete by Pattern Match

**User:**
```
Daisy, done PROJ-1234
```

**AI executes:**

1. **Update `.daisy/today.md`:**
   ```diff
   - - [ ] Implement PagerDuty Adapter using ServiceMetadata +PROJ-1234 +FY26Q2
   + - [x] Implement PagerDuty Adapter using ServiceMetadata +PROJ-1234 +FY26Q2
   ```

2. **Update `.daisy/tasks/todo.txt`:**
   ```diff
   # Move from active section:
   - (A) 2026-01-13 Implement PagerDuty Adapter using ServiceMetadata +PROJ-1234 +FY26Q2
   
   # To completed section at end:
   + x 2026-01-15 2026-01-13 Implement PagerDuty Adapter using ServiceMetadata +PROJ-1234 +FY26Q2
   ```

**AI Response:**
```
✅ Marked complete in today.md
✅ Marked complete in todo.txt

Would you like me to update JIRA PROJ-1234 status to Resolved?
```

### Example 2: Complete with JIRA Update

**User:**
```
Daisy, done PROJ-1234, yes update JIRA
```

**AI executes:**

1. Mark complete in today.md and todo.txt (as above)
2. Query JIRA for available transitions
3. Transition ticket to "Resolved"
4. Add comment with completion details

**AI Response:**
```
✅ Marked complete in today.md
✅ Marked complete in todo.txt
✅ Updated JIRA PROJ-1234:
   - Status: In Progress → Resolved
   - Added comment: "Implemented PagerDuty Adapter using instance-based pattern to prevent race conditions."
```

### Example 3: Complete GitHub PR

**User:**
```
Daisy, done PR 1545
```

**AI Response:**
```
✅ Marked complete in today.md: @git Template pagerduty utility [PR#1545]
✅ Marked complete in todo.txt

PR#1545 status: Merged
Associated JIRA: PROJ-1234
Would you like me to update JIRA PROJ-1234?
```

---

## Starting a New Week

### Full Workflow

**User:** "Daisy, start a new week"

**AI executes:**

1. **Archive completed tasks from todo.txt to done.txt:**
   
   **Before (todo.txt):**
   ```
   (A) 2026-01-13 Active task +PROJ-1234
   (B) 2026-01-09 Another active task +PROJ-1101
   x 2026-01-15 2026-01-13 Completed task 1 +PROJ-1238
   x 2026-01-15 2026-01-14 Completed task 2 +PROJ-1238
   x 2026-01-15 2025-12-19 Completed task 3 +PROJ-1100
   ```
   
   **After (todo.txt):**
   ```
   (A) 2026-01-13 Active task +PROJ-1234
   (B) 2026-01-09 Another active task +PROJ-1101
   ```
   
   **Appended to done.txt:**
   ```
   x 2026-01-15 2026-01-13 Completed task 1 +PROJ-1238
   x 2026-01-15 2026-01-14 Completed task 2 +PROJ-1238
   x 2026-01-15 2025-12-19 Completed task 3 +PROJ-1100
   ```

2. **Start new day** (see "Starting a New Day" section)

**AI Response:**
```
📦 Archived 3 completed tasks to done.txt
📦 Archived yesterday's work to journal.md
✅ New day started: 2026-01-20 Monday

📋 High Priority Tasks (2):
- Active task +PROJ-1234
- Another active task +PROJ-1101

Ready for the week!
```

---

## End of Day Retrospective

### Example 1: Synthesizing the Day

**User:**
```
Daisy, help me with my retrospective
```

**AI Response:**
```
Looking at your day:

**Completed:**
- Implemented PagerDuty Adapter (PROJ-1234)
- Coordinated with ~bsmith on PROJ-1100 handoff
- Merged PR#1545

**Logged activities:**
- 1523 - completed PROJ-1234 implementation
- 1545 - met with ~bsmith about PROJ-1100 frontend
- 1612 - opened PR#1545 for review
- 1645 - PR#1545 approved and merged

**Suggested retrospective:**

* **Successes:** 
  - Completed PROJ-1234 with clean adapter pattern preventing race conditions
  - Effective handoff to ~bsmith - clear communication saved future issues
  - Quick PR turnaround (30 minutes from open to merge)

* **Misses:** 
  - Didn't provide ETA to ~mgarcia (was due today) - need to follow up tomorrow
  - Certificate training still pending - blocking for next week

* **What would a Sage do next:** 
  - Block 30 minutes tomorrow morning to send ~mgarcia detailed ETA with dependencies
  - Complete certificate training before EOD tomorrow to unblock leave planning
  - Document adapter pattern decision in team wiki for future reference
```

### Example 2: Reflecting on Challenges

**User:**
```
Daisy, today was tough - spent 4 hours debugging a race condition. Help me reframe this professionally.
```

**AI Response:**
```
Reframing for professional retrospective:

* **Successes:** 
  - Deep investigation into race condition led to discovery of fundamental architectural issue
  - Systematic debugging approach isolated the problem to adapter instantiation timing
  - Solution (instance-based pattern) is more robust and will prevent similar issues

* **Misses:** 
  - Underestimated complexity of concurrent PagerDuty calls - original estimate was 2 hours
  - Should have consulted team patterns earlier - similar solution exists in ServiceNow adapter

* **What would a Sage do next:** 
  - Document this race condition pattern and solution for team knowledge base
  - Schedule pairing session with backend team to review concurrency patterns
  - Add integration test to catch similar timing issues in CI
```

---

## Multi-File Updates

### Example: Completing Task with Full Workflow

**User:**
```
Daisy, done PROJ-1100, reassigned to ~bsmith
```

**AI executes:**

1. **Update `.daisy/today.md`:**
   ```diff
   - - [ ] Coordinate with ~bsmith on dashboard frontend for the new reporting flow +PROJ-1100
   + - [x] Coordinate with ~bsmith on dashboard frontend for the new reporting flow +PROJ-1100
   ```

2. **Update `.daisy/tasks/todo.txt`:**
   ```diff
   - (C) 2025-12-19 Coordinate with ~bsmith on dashboard frontend +PROJ-1100 +FY26Q2 @jira
   + x 2026-01-15 2025-12-19 Coordinate with ~bsmith on dashboard frontend +PROJ-1100 +FY26Q2 @jira
   ```

3. **Update JIRA PROJ-1100:**
   - Reassign to bsmith
   - Add comment about handoff
   - Keep status as "In Progress"

4. **Log the handoff:**
   ```markdown
   - 1545 - Handed off PROJ-1100 to ~bsmith with frontend context
   ```

**AI Response:**
```
✅ Marked complete in today.md
✅ Marked complete in todo.txt
✅ Updated JIRA PROJ-1100:
   - Reassigned: jsmith → bsmith
   - Added comment: "Handing off to @bsmith for frontend implementation.
     Backend endpoint is ready at /incidents/{id}/roomMeetingInfo. See PR#154 for details."
✅ Logged handoff in today.md

Create a follow-up task to notify ~tlee?
```

---

## Home Switching

### Example: Switch from Work to Personal

Each workspace independently tracks its own home. To use a different home, re-run `daisy init`.

**User:**
```
Daisy, switch to personal
```

**AI executes:**
```bash
daisy init personal
```

**AI Response:**
```
✅ Switched to personal home

.daisy/ symlinks updated:
- tasks/   → daisy/home/personal/tasks/
- today.md → daisy/home/personal/journal/today.md
- journal.md → daisy/home/personal/journal/journal.md
- projects/ → daisy/home/personal/projects/
- AGENTS.md → daisy/home/personal/AGENTS.md

Local git identity set: Jane Doe <jane@example.net>

Other workspaces are unaffected.
```

**User:**
```
Daisy, start a new day
```

**AI Response:**
```
📦 Archived yesterday's work to journal.md
✅ New day started: 2026-01-15 Thursday

📋 High Priority Tasks (3):
- Finish chapter 3 of book +writing
- Review investment portfolio +finance
- Plan weekend hiking trip +health

No GitHub PRs.

Ready for commands.
```

---

## Key Patterns

### Cross-File Consistency

When completing tasks, the AI ensures consistency across:
1. `.daisy/today.md` - Mark task with `[x]`
2. `.daisy/tasks/todo.txt` - Add `x` prefix, strip priority, move to end
3. External systems (JIRA/GitHub) - Update status if requested
4. `.daisy/today.md` log section - Record the completion

### Professional Tone (Work Context)

When logging work in a professional/work context, the AI automatically filters/reframes:
- Bad: "Wasted time on X's terrible code"
- Good: "Investigated legacy code, identified refactoring opportunities"

In personal context, more casual language is acceptable.
