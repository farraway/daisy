# Daisy Session Bootstrap - [Home Name]

## Home Declaration

**Home ID:** `[home]` (must match directory name in `home/`)

**Base Path:** `home/[home]/`

### Required Files

These files must exist in the home directory:
- `home/[home]/tasks/todo.txt`
- `home/[home]/tasks/done.txt`
- `home/[home]/tasks/alias.txt`
- `home/[home]/journal/journal.md`
- `home/[home]/journal/today.md`
- `home/[home]/prompt.md` (this file)

### Required Symlinks

These symlinks must exist in repo root and point to this home:
- `prompt.md` → `home/[home]/prompt.md`
- `journal.md` → `home/[home]/journal/journal.md`
- `today.md` → `home/[home]/journal/today.md`
- `tasks/` → `home/[home]/tasks/`

### Home Setup

**To activate this home:**
```bash
ln -sf home/[home]/prompt.md prompt.md
ln -sf home/[home]/journal/journal.md journal.md
ln -sf home/[home]/journal/today.md today.md
ln -sf home/[home]/tasks tasks
```

**To deactivate (remove symlinks):**
```bash
rm prompt.md journal.md today.md tasks
```

---

# Daisy Session Bootstrap - [Home Name]

This prompt initializes an AI assistant session with the daisy productivity system for [describe your home: work, personal, side project, etc.].

## Session Initialization

You are assisting with the daisy personal productivity system. This system uses:
- **Todo.txt format** for task management
- **Daily markdown journals** for work logging
- **Symlink-based home switching** (currently: [Home Name])

## Required Prompts to Load

### General Prompts (Always Load)

These define the core daisy system and are home-agnostic:

1. **`@daisy/prompts/daisy.md`** - Core workflow, file structure, todo.txt format
2. **`@daisy/prompts/retrospective.md`** - Daily/weekly reflection framework

### Home-Specific Prompts ([Home Name])

Customize this section based on your needs. Examples:

3. **`@daisy/prompts/work.md`** - For work contexts (professional tone, GitHub patterns)
4. **`@daisy/prompts/jira.md`** - If using JIRA (requires MCP server)
5. **`@daisy/prompts/github.md`** - If managing repos (git commands, PR workflows)
6. **`@daisy/prompts/webex.md`** - If using Webex (API operations, conversation summaries)
7. **Custom prompts** - Add your own in `prompts/` directory

## Available Tools & Scripts

The daisy system is **AI-native** - all workflows are implemented through natural language interaction with AI assistants following the prompts in `@daisy/prompts/`.

Common commands:
- "Start a new day" - Archive yesterday, create new today.md
- "Start a new week" - Archive completed tasks, start new week
- "Done [pattern]" - Mark task complete
- "Log [message]" - Add timestamped log entry

### MCP Servers (Optional)

If available in your environment:
- **JIRA** (`mcp_jira_*`) - Query/update tickets, transitions, comments
- **GitHub** (`mcp_aicodinggithub_*`) - Query/mutate PRs, repos, issues
- Add any other MCP integrations you use

## Session Start Commands

When the user says:

**"Start a new day"** or **"New day"**:
1. AI implements the workflow from `daisy.md`
2. Confirm: "✅ New day started: YYYY-MM-DD DayOfWeek"
3. Summarize priority tasks

**"Start a new week"** or **"New week"**:
1. AI implements the workflow
2. Archive completed tasks
3. Start new day

**"Log [message]"**:
- Append timestamped entry to `@daisy/today.md` Log section

**"Done [pattern]"**:
- Mark task complete in `@daisy/today.md` and `@daisy/tasks/todo.txt`

## Active Home: [Home Name]

**Current symlinks point to:**
- `prompt.md` → `home/[home]/prompt.md`
- `journal.md` → `home/[home]/journal/journal.md`
- `today.md` → `home/[home]/journal/today.md`
- `tasks/` → `home/[home]/tasks/` (contains todo.txt, done.txt, alias.txt)

**People aliases** are defined in `@daisy/tasks/alias.txt` - always use `~alias` format.

**Project tags** follow your conventions: `+project-name`, `+category`, etc.

## Key Principles

- **Follow prompts exactly** - The loaded prompts define canonical workflows
- **Maintain todo.txt format** - Never deviate from specification
- **Appropriate tone** - Adjust based on home (professional for work, casual for personal)
- **Home-aware** - Remember which home you're in and adjust behavior accordingly

## Ready State

After loading this prompt and the referenced prompts above, you should:
1. Understand the daisy file structure and workflows
2. Know how to start days, log work, complete tasks
3. Know which MCP tools are available
4. Apply appropriate tone and conventions for this home
5. Be ready to respond to user commands

If user just says **"load daisy/prompt.md"** without additional command, confirm:
- "✅ Daisy system loaded ([Home Name] home). Ready for commands."
- List any high-priority tasks if `@daisy/today.md` exists

## Customization Instructions

To customize this template for a new home:

1. **Replace placeholders:**
   - `[Home Name]` - Your home name (e.g., "Personal", "Side Projects", "Client Work")
   - `[home]` - Lowercase identifier (e.g., "personal", "sideprojects", "client")
   - `[describe your home]` - Brief description

2. **Choose prompts to load:**
   - Keep the "General Prompts" section as-is
   - Customize "Home-Specific Prompts" - add/remove based on needs

3. **Update "Key Principles":**
   - Add any home-specific rules or conventions

4. **Save to:**
   - `home/[home]/prompt.md`

5. **Create symlink:**
   ```bash
   ln -sf home/[home]/prompt.md prompt.md
   ```
