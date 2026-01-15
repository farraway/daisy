# Daisy Quickstart Guide

Get started with daisy in 5 minutes. This guide covers the 80% use case - daily task management and journaling.

## What is Daisy?

Daisy is a personal productivity system that helps you:
- Track tasks in plain text (todo.txt format)
- Keep a daily work journal in markdown
- Review your week and improve continuously

**Key Philosophy:** Plain text, AI-assisted, flexible and conversational.

## Quick Setup (First Time)

### 1. Choose Your Home

A "home" is a collection of your tasks and journals. Most users need just one:

```bash
# If using cisco home (work):
cd daisy
ln -sf home/work/prompt.md prompt.md
ln -sf home/work/tasks tasks
ln -sf home/work/journal/journal.md journal.md
ln -sf home/work/journal/today.md today.md
```

**Or create a personal home:**

```bash
# Create from template
cp -r templates/home home/personal

# Edit home/personal/prompt.md and replace:
# - [home] → personal
# - [Home Name] → Personal Home

# Activate it
ln -sf home/personal/prompt.md prompt.md
ln -sf home/personal/tasks tasks
ln -sf home/personal/journal/journal.md journal.md
ln -sf home/personal/journal/today.md today.md
```

### 2. Start Your Day

In your AI assistant:

```
Load daisy/prompt.md and start a new day
```

That's it! You'll get a fresh `today.md` with your tasks organized by priority.

## Daily Workflow (The Essentials)

### Morning: Start Your Day

**You say:** "start a new day"

**AI does:**
- Archives yesterday's work to journal.md
- Creates fresh today.md with your tasks organized:
  - **High-Priority Tasks** - (A) and (B) priority items
  - **Task Inbox** - Tasks you haven't prioritized yet
  - **GitHub PRs** - Pull requests needing review

**You get:** Clean slate, focused priorities

---

### During the Day: Log Work

**AI logs automatically** as you work together:
- Completed tasks
- Discoveries
- Decisions
- Meetings

**You can also say:** "log current work" or "log [message]"

**Example:**
```
You: log met with sarah about the api design

AI: ✅ Logged: 1445 - Met with ~sarah about the API design
```

---

### Complete Tasks

**You say:** "done [task description]"

**AI does:**
- Marks task complete in today.md: `- [x]`
- Updates todo.txt: moves task to end with completion date
- Logs the completion

**Example:**
```
You: done bug fix

AI: 
✅ Marked complete in today.md: Fix critical bug
✅ Marked complete in todo.txt
✅ Logged: 1530 - Completed fix critical bug
```

---

### End of Day: Quick Retrospective

Review your three prompts in today.md:
- **Successes:** What went well?
- **Misses:** What could have been better?
- **What would a Sage do next:** Wisdom-based next steps

**You can say:** "help me with my retrospective"

**AI reviews** your day and suggests content based on what you accomplished.

---

### Weekly: Clean Up

**You say:** "start a new week"

**AI does:**
- Archives completed tasks to done.txt
- Consolidates quiet days in journal.md (optional)
- Starts fresh week
- Creates new day

---

## Task Priorities (Simple Version)

When adding tasks to `tasks/todo.txt`:

```
(A) 2026-01-16 Critical bug fix @jira           ← Do today
(B) 2026-01-16 Review design doc @jira          ← Do this week
2026-01-16 Triage new tickets +INBOX            ← Needs priority (Inbox)
(C) 2026-01-16 Refactor utils @jira             ← Do when A/B done
(D) 2026-01-16 Learn new framework              ← Someday
```

**Most tasks are (C)** - that's your standard backlog.

**No priority = Inbox** - decide later.

---

## Common Commands

| You Say | AI Does |
|---------|---------|
| `start a new day` | Archive yesterday, create today.md with prioritized tasks |
| `start a new week` | Archive completed tasks, consolidate journal, start new day |
| `done [task]` | Mark task complete in today.md and todo.txt |
| `log [message]` | Add timestamped entry to today's log |
| `log current work` | AI summarizes recent conversation into log entry |
| `help me with retrospective` | AI suggests retrospective content based on your day |
| `check system` | Verify files, symlinks, and format integrity |
| `switch to [home]` | Change to different work/life context |

---

## File Structure (Don't Worry About This Yet)

```
daisy/
├── prompt.md           # Symlink → loads everything
├── today.md            # Today's journal (symlink)
├── journal.md          # Archive of all days (symlink)
├── tasks/              # Your todo.txt (symlink)
│   ├── todo.txt
│   ├── done.txt
│   └── alias.txt
└── home/
    └── cisco/          # Your actual files live here
        ├── prompt.md
        ├── tasks/
        └── journal/
```

The symlinks let you switch between work/personal contexts easily.

---

## Next Session: Just One Command

```
Load daisy/prompt.md and start a new day
```

That's all you need to remember. The AI handles the rest.

---

## Going Deeper (When Ready)

### Intermediate Topics
- **Understanding priorities:** See `prompts/daisy.md` → "Task Priority System"
- **Using @context labels:** Organize tasks by type (@jira, @git, @meeting)
- **Weekly reviews:** Reflect on patterns and improve continuously
- **Person references:** Use `~alias` format from tasks/alias.txt

### Advanced Topics
- **Creating new homes:** Separate work/personal/side-project contexts
- **Custom prompt chains:** Add specialized prompts (JIRA, GitHub, Webex)
- **Task archival:** Understanding todo.txt → done.txt flow
- **Journal curation:** Abridged archival for useful history

### Full Documentation
- **Complete workflows:** `prompts/daisy.md` - All algorithms and specifications
- **Examples:** `docs/examples/daisy.md` - Detailed interaction examples
- **Todo.txt format:** `docs/todotxt.md` - Format specification

---

## Tips for Success

1. **Start simple:** Just use "start a new day" and "done [task]" for the first week
2. **Let AI log:** Don't stress about logging everything - AI does it as you work
3. **Trust the system:** The format preserves everything, you can always review journal.md
4. **Weekly rhythm:** "start a new week" on Mondays keeps everything clean
5. **Conversational:** Just talk naturally - "done bug fix" works, no need for exact text

---

## Troubleshooting

**"No active home" error:**
- You need to create symlinks (see Quick Setup above)
- Or: "check system" to diagnose issues

**Tasks not appearing in today.md:**
- Check priority: (A) or (B) appear in High-Priority
- No priority? Goes to Task Inbox
- Has @git/@github? Goes to GitHub PRs section

**Want to see how it all works:**
- Read `docs/examples/daisy.md` for detailed walkthroughs
- Read `prompts/daisy.md` for complete specifications

---

## Philosophy

**Plain text** - Future-proof, searchable, greppable, git-friendly

**AI-assisted** - Let AI handle formatting, organization, and memory

**Conversational** - No rigid commands, just talk naturally

**Reflective** - Daily retrospectives help you improve continuously

**Flexible** - Adapt the system to your workflow, not vice versa

---

Ready to start? Just say:

```
Load daisy/prompt.md and start a new day
```
