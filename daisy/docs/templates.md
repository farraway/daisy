# Template Specifications

The template files live at `daisy/templates/journal-day.md` and `daisy/templates/journal-week.md`. This document describes the placeholder substitution system and formatting rules.

## journal-day.md Template

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

### Placeholders

- `{DATE}` → YYYY-MM-DD
- `{DAY}` → Day of week (Monday, Tuesday, etc.)
- `{TIME}` → HHMM in Pacific Time
- `{HIGH_PRIORITY_TASKS}` → Priority A tasks as markdown checkboxes
- `{NEXT_PRIORITY_TASKS}` → Priority B tasks as markdown checkboxes
- `{INBOX_TASKS}` → No-priority tasks as markdown checkboxes (appended after default checklist)
- `{GITHUB_TASKS}` → @git/@github tasks as markdown checkboxes

### Formatting Rules

- Use bold text (`**Section:**`) not headings (simplified to just "Now" and "Next")
- Preserve blank line before each subsection
- Inbox section includes default daily checklist items
- If section is empty (no extracted tasks), the placeholder is replaced with empty string

## journal-week.md Template

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

### Key Differences from journal-day.md

- Starts with "Weekly Retrospective" section at top (for previous week)
- Includes "Resolutions" section (identity-based goal setting)
- Extended inbox checklist with weekly startup items (email/chat zero-ing, JIRA/GitHub sync)
- Same task extraction logic and placeholders as daily template
