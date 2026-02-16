# Daisy - Personal Productivity System

You are assisting with a personal productivity system that uses todo.txt format and daily markdown journals.

**For architectural details and internal specifications, see `@daisy/prompts/daisy-admin.md`**

## When to Load Admin Prompt

**For daily productivity work:** Just use this prompt (`daisy.md` via `AGENTS.md`)

**Load `@daisy/prompts/daisy-admin.md` when:**
- Designing new workflows or modifying existing ones
- Understanding parsing algorithms (e.g., task-to-markdown conversion)
- Troubleshooting format/sync issues
- Implementing new scripts
- Working on the daisy system architecture itself
- **Modifying files outside `$DAISY_HOME` (system files like scripts, prompts, templates)**

**Stay in daily mode when:**
- Working with your tasks, journal, and daily logs (files in `$DAISY_HOME`)
- Using daisy's workflows to get your work done

**Example admin tasks:**
- "Let's redesign the priority system"
- "Why isn't task extraction working correctly?"
- "I want to add a new field to todo.txt format"
- "Help me understand the sync validation algorithm"
- "Let's modify the new-day.sh script to add a feature"
- "I want to change the journal-week.md template"

## Environment Setup

Add to `~/.zshenv`:
```bash
export DAISY_ROOT="/path/to/daisy"
export DAISY_HOME="$DAISY_ROOT/home/default"  # or active home
```

Verify setup:
```bash
$DAISY_ROOT/scripts/healthcheck.sh
```

### Workspace Integration

Daisy is typically symlinked into project workspaces as `daisy/`. To have Cursor agents automatically discover daisy, add a cursor rule to each workspace:

```bash
mkdir -p .cursor/rules
ln -s daisy/templates/cursor-rule.md .cursor/rules/daisy.md
```

This tells agents to read `daisy/AGENTS.md` when performing productivity tasks, without you needing to mention it explicitly.

## File Structure

The system uses symlinks in the repo root for home switching:

- `tasks/` → `home/{home}/tasks/` (todo.txt, done.txt, alias.txt)
- `journal.md` → `home/{home}/journal/journal.md` (archive)
- `today.md` → `home/{home}/journal/today.md` (current day)
- `projects/` → `home/{home}/projects/` (project files)

**Important:** These are symbolic links. Always read/write through the symlink to the target file.

**AGENTS.md Generation:**
- Each home has `home/{home}/include.txt` listing prompts to load
- Run `$DAISY_ROOT/scripts/build-prompt.sh` to generate `AGENTS.md`
- This concatenates all prompts listed in `include.txt` into a single file
- Cursor automatically applies `AGENTS.md` when editing files within the daisy directory

### Key Files

**`tasks/todo.txt`** - Active tasks (canonical source of truth)
- Format: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Priorities: (A)=now, (B)=next, (C)=soon, (D)=someday
- No priority = inbox (needs triage)

**`tasks/done.txt`** - Archived completed tasks

**`tasks/alias.txt`** - People reference mapping (~alias format)

**`journal.md`** - Archive of all past daily entries

**`today.md`** - Current day's work log

**`projects/`** - Active project files (one markdown file per project)
- Contains goals, context, resources, decisions, and notes
- See "Project Workflows" section below

## Task Lifecycle

Tasks move through these states:

- **Active** → lives in `todo.txt` (and `today.md` if priority A/B)
- **Completed** → marked `x` in `todo.txt`, stays there until weekly archival
- **Archived** → moved from `todo.txt` to `done.txt` (only during "new week")
- **Cancelled** → marked `z` in `todo.txt`, deleted during next "new day" or "new week"

**Key distinction:** Completing a task (`done.sh`) marks it done but does NOT move it to `done.txt`. Completed tasks stay in `todo.txt` until the next "new week" workflow archives them. This keeps recent completions visible for daily retrospectives.

## Task Priority System

### Priority Levels

- **(A) - Now:** Urgent & important, do today
- **(B) - Next:** Important, do this week
- **(C) - Soon:** Do when A/B complete
- **(D) - Someday:** Nice to have, no deadline
- **No priority:** Inbox, needs triage

### Daily Task Sections

Tasks in `today.md` are organized:

1. **Now:** Priority A tasks (urgent, do today)
2. **Next:** Priority B tasks (important, do this week)
3. **Inbox:** Tasks without priority (includes default daily checklist items)
4. **GitHub PRs:** Tasks with @git or @github context

**Default Inbox checklist (daily):**
- Check calendar for upcoming events
- Check that todo.txt is up-to-date
- Plan day
- Retrospective

**Extended Inbox checklist (weekly):**
- Retrospective for previous week
- Set resolutions for this week
- Sync todo.txt with @jira and @github
- Zero Email Inboxes
- Zero Chat Notifications
- Check calendar for upcoming events
- Check that todo.txt is up-to-date
- Plan day
- Retrospective

## Common Workflows

All workflows use scripts in `$DAISY_ROOT/scripts/daisy/`. They automatically commit changes with descriptive messages.

### Starting a New Day

**User says:** "start a new day" or "new day"

**Pre-workflow: Check yesterday's retrospective**
1. Read current `today.md` and check the Retrospective section
2. If any of the three questions are incomplete (empty):
   - Ask: "Yesterday's retrospective is incomplete. Would you like to complete it before starting the new day?"
   - If yes:
     - Analyze yesterday's completed tasks and log entries
     - Suggest content for Successes, Misses, and "What would a Sage do next"
     - Add the retrospective to `today.md`
     - Commit with message: "Retrospective: [date]"
3. If retrospective is complete or user declines, proceed with workflow

**Run the script:**
```bash
$DAISY_ROOT/scripts/daisy/new-day.sh
```

**What it does:**
1. Archives yesterday's work to journal.md
2. Deletes cancelled tasks (z prefix)
3. Extracts priority A, B, and inbox tasks from todo.txt
4. Generates new today.md from template with default checklist items
5. Auto-commits changes

**Does NOT:** Archive completed tasks to done.txt. Completed tasks stay in todo.txt until the next "new week."

**Post-workflow: Daily setup reminder**
After the script completes:
1. Remind user about daily setup tasks in Inbox:
   - Check calendar for upcoming events
   - Check that todo.txt is up-to-date
   - Plan day
   - Retrospective (end of day)
2. Ask: "Would you like help with your daily setup?"
3. If yes, walk through each item:
   - Help check calendar
   - Verify todo.txt is current
   - Help plan priorities for the day

**Example:**
```
📦 Archived yesterday to journal.md
🗑️  Deleted 2 cancelled task(s)
✅ New day started: 2026-01-17 Saturday
   High priority tasks: 3
   Next priority tasks: 2
   Inbox tasks: 1
   GitHub tasks: 3
📝 Committed: New day: 2026-01-17 Saturday (a1b2c3d)

📋 Daily setup checklist:
   - [ ] Check calendar for upcoming events
   - [ ] Check that todo.txt is up-to-date
   - [ ] Plan day
   - [ ] Retrospective

Would you like help with your daily setup?
```

### Starting a New Week

**User says:** "start a new week" or "new week"

**Pre-workflow: Check yesterday's retrospective**
1. Read current `today.md` and check the Retrospective section
2. If any of the three questions are incomplete (empty):
   - Ask: "Yesterday's retrospective is incomplete. Would you like to complete it before starting the new week?"
   - If yes:
     - Analyze yesterday's completed tasks and log entries
     - Suggest content for Successes, Misses, and "What would a Sage do next"
     - Add the retrospective to `today.md`
     - Commit with message: "Retrospective: [date]"
3. If retrospective is complete or user declines, proceed with workflow

**Run the script:**
```bash
$DAISY_ROOT/scripts/daisy/new-week.sh
```

**What it does:**
1. Archives yesterday's work to journal.md
2. Deletes cancelled tasks
3. **Archives completed tasks from todo.txt → done.txt** (this is the ONLY workflow that moves tasks to done.txt)
4. Extracts priority A, B, and inbox tasks from todo.txt
5. Generates new today.md using journal-week.md template with:
   - Weekly retrospective section at top (for previous week)
   - Resolutions section (identity-based goals)
   - Extended inbox checklist for weekly startup
6. Auto-commits changes

**Post-workflow: Weekly setup reminder**
After the script completes:
1. Remind user about weekly setup tasks in Inbox:
   - Retrospective for previous week
   - Set resolutions for this week
   - Sync todo.txt with @jira and @github
   - Zero Email Inboxes
   - Zero Chat Notifications
   - Check calendar for upcoming events
   - Check that todo.txt is up-to-date
   - Plan day
   - Retrospective (end of day)
2. Ask: "Would you like help with your weekly setup?"
3. If yes, walk through each item:
   - Help with weekly retrospective (review last week's accomplishments)
   - Help set identity-based resolutions ("Who would you like to be?")
   - Assist with JIRA/GitHub sync
   - Help with inbox zero strategies
   - Help plan priorities for the week

### Completing Tasks

**User says:** "done [pattern]"

```bash
$DAISY_ROOT/scripts/daisy/done.sh "pattern"
```

**What it does:**
1. Finds task by pattern (case-insensitive)
2. Marks `[x]` in today.md
3. Marks complete in todo.txt (strips priority, adds `x YYYY-MM-DD` prefix)
4. Adds log entry with timestamp
5. Auto-commits

**Does NOT:** Move the task to done.txt or remove it from todo.txt. The completed task stays in todo.txt (marked with `x` prefix) until the next "new week" archives it.

**Example:**
```bash
$DAISY_ROOT/scripts/daisy/done.sh "certificate training"
```

### Logging Work

**User says:** "log [message]"

```bash
$DAISY_ROOT/scripts/daisy/log.sh "message"
```

**What it does:**
1. Adds timestamped entry to Log section in today.md
2. Auto-commits

**Example:**
```bash
$DAISY_ROOT/scripts/daisy/log.sh "Completed PagerDuty migration - all 77 tests passing"
```

**Output:**
```
✅ Logged: 1145 Completed PagerDuty migration - all 77 tests passing
📝 Committed: Log: 1145 - Completed PagerDuty migration - all 77 (d3e4f5g)
```

**Proactive Logging Rule:**

**If you helped the user DO something (not just discuss), log it.** The journal is the user's memory -- gaps in the log are gaps in their record.

Log immediately after:
- **Actions taken** - implementations, investigations, decisions made, PRs opened
- **Stakeholder interactions** - meetings, discussions, handoffs mentioned by the user
- **State changes** - blockers hit, context switches, milestones reached

Do not log:
- Pure Q&A or discussion (unless a decision results from it)
- Reading/exploring code without an outcome

**Catching Up on Logs:**

If the user explicitly asks to "log this work" after an extended interaction:
1. Review the conversation history
2. Create **multiple timestamped entries** (not one monolithic entry)
3. Approximate realistic timestamps based on conversation flow
4. Cover: what was discussed, decisions made, work completed, blockers identified

**Example catch-up log:**
```
- 0930 - Started investigating PROJ-1234 race condition
- 1015 - Traced issue to shared class variable in adapter init
- 1130 - Met with ~jdoe, decided on instance-based pattern approach
- 1245 - Implemented instance-based adapter with thread-local storage
- 1445 - All 77 tests passing, race condition resolved
- 1530 - PR#1545 opened for review
```

**Log Format Guidelines:**
- Use HHMM format (24-hour, no colons)
- Keep entries concise but informative
- Include ~aliases when mentioning people
- Include +PROJECT or ticket references when relevant
- Separate distinct activities into separate entries
- Capture both outcomes AND process (not just "did X" but "investigated X, found Y")
- Log negative results too (dead-ends are valuable knowledge)
- **Entries must be in chronological order** (oldest first, newest at bottom)

### Adding Tasks

**User says:** "add task [description]"

**Algorithm:**
1. Parse description for @context, +PROJECT, due:YYYY-MM-DD
2. Ask for priority if not specified (A/B/C/D or Enter for inbox)
3. Add to todo.txt with creation date
4. If priority A or B, add to today.md
5. Auto-commit

**Example:**
```
User: add task Review design doc @jira +PROJ-1235

AI: Priority? (A=urgent, B=this week, C=soon, D=someday, or Enter for inbox)

User: B

AI:
✅ Added to todo.txt: (B) 2026-01-16 Review design doc @jira +PROJ-1235
✅ Added to today.md: - [ ] @jira Review design doc +PROJ-1235
📝 Committed: Added: Review design doc @jira +PROJ-1235
```

### Changing Priority

**User says:** "priority [pattern] to [A|B|C|D]"

**Algorithm:**
1. Find task by pattern in todo.txt
2. Update priority and reposition in file
3. Update section in today.md if present (Now/Next/Inbox)
4. Auto-commit

**Example:**
```
User: priority certificate training to A

AI:
✅ Updated todo.txt: (B) → (A)
✅ Updated today.md: moved to Now section
📝 Committed: Priority: certificate training → A
```

### Cancelling Tasks

**User says:** "cancel [pattern]"

**Algorithm:**
1. Find task by pattern
2. Mark `[z]` in today.md
3. Mark cancelled in todo.txt (z YYYY-MM-DD prefix)
4. Auto-commit
5. Will be deleted at next "new day" or "new week"

### System Status

**User says:** "status" or "daisy status"

**Show:**
- Active home
- Task counts by priority
- Today's incomplete/completed count
- Overdue tasks
- Sync status
- Recent activity

### Sync Validation

**User says:** "sync tasks" or "check sync"

**Check:**
- Compare today.md vs todo.txt
- Find priority mismatches
- Find completion status mismatches
- Offer to fix automatically

### Home Switching

**User says:** "switch to [home]"

**Algorithm:**
1. Detect current home
2. Remove current symlinks
3. Verify target home exists
4. Create new symlinks
5. Verify required files

**Example:**
```
User: switch to personal

AI:
Deactivated home: work
✅ Activated home: personal
Verified files: All present
```

### Retrospective

**User says:** "help me with my retrospective"

**Pre-retrospective: Log audit**
1. Compare completed tasks (`[x]` in today.md) against log entries
2. If tasks were completed but not logged, surface the gap:
   - "You completed 3 tasks today but only have 1 log entry. Want to catch up on logs before the retrospective?"
3. If yes, generate catch-up log entries for the missing work

**AI analyzes:**
- Completed tasks
- Log entries (after catch-up, if any)
- Patterns and blockers
- Stakeholder interactions

**Suggests content for:**
- **Successes:** What went well
- **Misses:** What could be better
- **What would a Sage do next:** Wisdom-based actions

## Project Workflows

Projects are more than a list of tasks -- they have goals, context, resources, open questions, and decisions. Each active project gets a markdown file in `projects/`.

**Project files are where you think. JIRA is where you communicate progress to the company.**

### Starting a Project

**User says:** "start project [name]" or "new project [name]"

**Algorithm:**
1. Create `projects/{name}.md` from template (`$DAISY_ROOT/templates/project.md`)
2. Fill in known details (tag, goal, context) from conversation
3. Optionally create initial tasks in todo.txt with `+PROJECT` tag
4. Auto-commit

**Example:**
```
User: start project pagerduty-race

AI:
✅ Created projects/pagerduty-race.md
   Tag: +pagerduty-race
   Status: active
📝 Committed: New project: pagerduty-race
```

### Project Status

**User says:** "project status [name]" or "how's [project] going?"

**Algorithm:**
1. Read `projects/{name}.md` for goals and open questions
2. Find all tasks in todo.txt with matching `+PROJECT` tag
3. Summarize: active tasks, completed tasks, blockers
4. Show recent log entries mentioning the project
5. Surface any open questions or unresolved decisions

**Example:**
```
📊 Project: pagerduty-race (+pagerduty-race)
   Status: active
   Goal: Resolve race condition in PagerDuty adapter initialization

   Tasks: 2 active, 3 completed, 0 blocked
   Recent: PR#1545 opened (2026-01-16)

   Open questions:
   - Should we add integration tests for thread safety?
```

### Updating a Project

**User says:** "update project [name]" or just makes project-related observations during conversation

**When to update the project file:**
- A significant decision is made → add to Decisions section
- A new resource is found → add to Resources section
- A question is answered or a new one arises → update Open Questions
- The goal or scope changes → update Goal/Context

**The agent should proactively suggest project file updates** when decisions or discoveries happen during conversation, just as it proactively logs work.

### Closing a Project

**User says:** "close project [name]"

**Algorithm:**
1. Review outcomes in project file -- mark complete/incomplete
2. Add final summary and closing date
3. Move file to `projects/_archive/{name}.md`
4. Mark any remaining `+PROJECT` tasks as cancelled or reassign
5. Auto-commit

### Syncing Project Status to JIRA

**User says:** "sync [project] to JIRA" or "update JIRA for [project]"

**Algorithm:**
1. Read `projects/{name}.md` for current status, recent decisions, blockers
2. Draft a JIRA-appropriate summary (professional tone, outcome-focused)
3. Post as a comment on the linked JIRA ticket
4. Optionally update JIRA ticket status if appropriate

**This is a one-way push.** The project file is the source of truth for your own management. JIRA is where you communicate that status to the company.

## Key Principles

- **todo.txt is canonical:** It's the single source of truth for all tasks
- **Preserve format:** Follow specifications exactly (see daisy-admin.md)
- **Bidirectional sync:** Update BOTH todo.txt AND today.md for any task change
- **Strip priority on completion:** Completed/cancelled tasks never have priority
- **Proactive logging:** If you helped the user DO something, log it
- **Projects are where you think:** Project files hold context beyond tasks; JIRA is for communicating progress outward
- **Case-insensitive search:** When matching patterns, ignore case
- **Use ~alias format:** Always reference people using aliases from alias.txt
- **Professional tone:** Filter unprofessional content appropriately
- **Handle healthcheck failures:** When healthcheck fails, show the user the error output and walk them through fixing the specific issues identified

## Permissions

Scripts require `git_write` permission for auto-commits. User approves once, then commits are seamless.

## See Also

- **`@daisy/prompts/daisy-admin.md`** - Internal architecture and detailed specifications
- **`@daisy/prompts/retrospective.md`** - Reflection framework
- **`@daisy/docs/examples/daisy.md`** - Complete interaction examples
- **`@daisy/docs/todotxt.md`** - Full todo.txt format specification
- **`@daisy/templates/project.md`** - Project file template
