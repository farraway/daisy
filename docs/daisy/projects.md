# Project Management - Architecture

## Design Rationale

**Problem:** Projects have context beyond their tasks -- goals, resources, open questions, decisions, stakeholders. The `+PROJECT` tag in todo.txt captures *what to do* but not *why* or *how*. Users end up putting this context in JIRA because JIRA has structure for it, which makes JIRA the management tool rather than the communication tool.

**Solution:** Each active project gets a markdown file in `projects/`. This is where the user thinks and manages their work. JIRA becomes an export target -- a place to push curated status updates outward to the company.

**Key principle:** The project file is the source of truth for the user's understanding of a project. JIRA is the source of truth for the company's understanding.

## Directory Structure

```
home/{home}/projects/
├── {project-name}.md          # Active projects
├── {project-name}.md
└── _archive/                  # Closed projects
    └── {project-name}.md
```

Accessed via workspace symlink:
```
.daisy/projects/ → daisy/home/{home}/projects/
```

## Project File Specification

See `templates/project.md` for the canonical template. Key sections:

| Section | Purpose | When Updated |
|---------|---------|-------------|
| Header (tag, JIRA, status, goal) | Quick reference metadata | On creation, on close |
| Context | Why the project exists, what problem it solves | On creation, rarely changed |
| Outcomes | Measurable deliverables (checkboxes) | On creation, marked complete as achieved |
| Resources | Links to docs, code, people | Throughout project lifecycle |
| Decisions | Timestamped record of choices made and why | When decisions happen |
| Open Questions | Unresolved uncertainties | Added/removed throughout |
| Notes | Freeform thinking, research, observations | Throughout project lifecycle |

## Project-Task Linking

Projects and tasks are linked bidirectionally through the `+PROJECT` tag:

- **Tasks -> Project:** Tasks in todo.txt carry `+project-name` tags
- **Project -> Tasks:** The project file's `Tag` field identifies which todo.txt tasks belong to it
- **Aggregation:** The "project status" command pulls all tasks with the matching tag

## Start Project Algorithm

```
1. Parse project name from user input
2. Check if projects/{name}.md already exists
   - If exists: "Project '{name}' already exists. Open it?"
3. Copy templates/project.md to projects/{name}.md
4. Fill in known details:
   a. Tag: +{name} (kebab-case)
   b. JIRA: ticket key if mentioned
   c. Status: active
   d. Started: today's date
   e. Goal: from conversation context
5. Ask: "Would you like to create initial tasks for this project?"
   - If yes, walk through adding tasks with +{name} tag
6. Commit: "New project: {name}"
```

## Project Status Algorithm

```
1. Read projects/{name}.md
2. Search todo.txt for all lines containing +{name}:
   a. Count active tasks (no x/z prefix)
   b. Count completed tasks (x prefix)
   c. Count cancelled tasks (z prefix)
3. Search today.md log for entries mentioning +{name} or project name
4. Extract open questions from project file
5. Report:
   - Project metadata (status, goal)
   - Task summary (active/completed/blocked)
   - Recent activity from logs
   - Open questions
   - Unresolved decisions
```

## Close Project Algorithm

```
1. Read projects/{name}.md
2. Review outcomes:
   a. For each outcome checkbox, confirm complete or incomplete
   b. Add completion notes
3. Add closing section:
   - Closed: YYYY-MM-DD
   - Final status: completed | abandoned | superseded
   - Summary: one-paragraph retrospective
4. Create projects/_archive/ if it doesn't exist
5. Move file: projects/{name}.md → projects/_archive/{name}.md
6. Handle remaining tasks:
   a. Find active tasks with +{name} in todo.txt
   b. For each: ask "Cancel, reassign to another project, or keep?"
7. Commit: "Closed project: {name}"
```

## JIRA Sync Algorithm

```
1. Read projects/{name}.md
2. Extract JIRA ticket key from header
3. Draft JIRA-appropriate summary:
   a. Current status (one line)
   b. Recent progress (bullet points, professional tone)
   c. Blockers or risks (if any)
   d. Next steps
4. Present draft to user for approval
5. Post as JIRA comment via MCP tools or API
6. Optionally update JIRA ticket status field
7. Log: "Synced project {name} status to JIRA {ticket}"
```
