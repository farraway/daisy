---
name: agents-md
description: AGENTS.md authoring — the README for coding agents. Use when the user asks to create, review, improve, or update an AGENTS.md file, asks what belongs in one, or is setting up agent guidance for a repository.
short_description: Write and review AGENTS.md files
---

AGENTS.md is a **README for coding agents** — a dedicated, predictable place
for the context and instructions agents need to work on a project. It is an
open standard supported by Cursor, Codex, Gemini CLI, Aider, and others; see
[agents.md](https://agents.md/) for the full specification.

README.md is for humans. AGENTS.md complements it with the precise,
sometimes tedious detail agents need — build steps, test commands, style
conventions, project-specific constraints — that would clutter a README or
doesn't concern human contributors.

## Recommended sections

Include what helps an agent work; none of these are mandatory.

**Setup commands** — installing dependencies, dev servers, builds. Be
explicit about package managers and tool versions.

```markdown
## Setup commands
- Install deps: `pnpm install`
- Start dev server: `pnpm dev`
- Run tests: `pnpm test`
- Build: `pnpm build`
```

**Code style** — conventions to follow. Agents apply explicit rules well.

```markdown
## Code style
- TypeScript strict mode
- Single quotes, no semicolons
- Use functional patterns where possible
- Prefer named exports over default exports
- Error handling: use Result types, not try/catch
```

**Testing instructions** — how to run tests, the framework, expectations for
new code.

```markdown
## Testing instructions
- Run all tests: `pnpm test`
- Run single test: `pnpm vitest run -t "test name"`
- Add or update tests for any code you change
- All tests must pass before committing
```

**PR / commit instructions** — format expectations.

```markdown
## PR instructions
- Title format: [package-name] Short description
- Always run `pnpm lint && pnpm test` before committing
- Squash commits before merging
```

**Security considerations** — constraints the agent must respect.

```markdown
## Security
- Never hardcode credentials or API keys
- Use environment variables for all secrets
- Sanitize user input before database queries
```

**Architecture notes** — high-level structure, data flow, or design
decisions. Keep it concise; link to deeper docs.

## Nested AGENTS.md for monorepos

Place an AGENTS.md in each package that needs its own instructions. The
nearest one to the file being edited wins, so each subproject ships tailored
guidance while the root provides project-wide defaults.

```
repo/
├── AGENTS.md              # Project-wide defaults
├── packages/
│   ├── frontend/
│   │   └── AGENTS.md      # Frontend-specific (React, CSS conventions)
│   └── backend/
│       └── AGENTS.md      # Backend-specific (API conventions, DB patterns)
```

## Writing guidelines

**Be imperative and specific.** Vague guidance produces vague results.

- Good: "Use `pnpm test --filter api` to run API tests"
- Bad: "Make sure to test your changes"

**Include what you'd tell a new teammate on day one** — build and test
commands, deployment steps, naming conventions, security gotchas, how the
codebase is organized.

**Keep it current.** Treat it as living documentation; update it when
commands, conventions, or architecture change.

**Keep it distinct from the README.** AGENTS.md carries agent-specific
operational context, not project descriptions or human contribution
guidelines.

**Reference secrets, never embed them.** Point at environment variables or a
secret manager.

**Leave room for judgment.** Give enough guidance to be effective without
prescribing every decision; focus on the conventions that matter for
consistency.
