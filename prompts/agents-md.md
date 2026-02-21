## Trigger

Read the full `daisy/prompts/agents-md.md` when:
- User asks to create an AGENTS.md file for a project
- User asks to improve, review, or update an existing AGENTS.md
- User asks "what should go in AGENTS.md?" or similar
- Setting up a new repository and the user wants agent guidance configured

# Writing Effective AGENTS.md Files

AGENTS.md is a **README for coding agents** -- a dedicated, predictable place to provide the context and instructions AI coding agents need to work on a project. It is an open standard supported by Cursor, Codex, Gemini CLI, Aider, and many others.

See [agents.md](https://agents.md/) for the full specification and examples.

## Purpose

README.md is for humans. AGENTS.md complements it with the precise, sometimes detailed context agents need: build steps, test commands, code style conventions, and project-specific constraints that would clutter a README or aren't relevant to human contributors.

## Recommended Sections

Not all sections are required. Include what helps an agent work effectively.

### Setup Commands

How to install dependencies, start dev servers, run builds. Be explicit about package managers and tool versions.

```markdown
## Setup commands
- Install deps: `pnpm install`
- Start dev server: `pnpm dev`
- Run tests: `pnpm test`
- Build: `pnpm build`
```

### Code Style

Conventions the agent should follow. Be specific -- agents follow explicit rules well.

```markdown
## Code style
- TypeScript strict mode
- Single quotes, no semicolons
- Use functional patterns where possible
- Prefer named exports over default exports
- Error handling: use Result types, not try/catch
```

### Testing Instructions

How to run tests, what framework is used, expectations for new code.

```markdown
## Testing instructions
- Run all tests: `pnpm test`
- Run single test: `pnpm vitest run -t "test name"`
- Add or update tests for any code you change
- All tests must pass before committing
```

### PR / Commit Instructions

Format expectations for commits and pull requests.

```markdown
## PR instructions
- Title format: [package-name] Short description
- Always run `pnpm lint && pnpm test` before committing
- Squash commits before merging
```

### Security Considerations

Constraints the agent must respect.

```markdown
## Security
- Never hardcode credentials or API keys
- Use environment variables for all secrets
- Sanitize user input before database queries
```

### Architecture Notes

High-level context that helps the agent understand the codebase structure, data flow, or design decisions. Keep this concise -- link to deeper docs if needed.

## Nested AGENTS.md for Monorepos

Place an AGENTS.md in each package or subdirectory that needs its own instructions. The nearest AGENTS.md to the file being edited takes precedence. This lets each subproject ship tailored instructions while the root AGENTS.md provides project-wide defaults.

```
repo/
├── AGENTS.md              # Project-wide defaults
├── packages/
│   ├── frontend/
│   │   └── AGENTS.md      # Frontend-specific (React, CSS conventions)
│   └── backend/
│       └── AGENTS.md      # Backend-specific (API conventions, DB patterns)
```

## Writing Guidelines

**Be imperative and specific.** Agents follow explicit instructions well. Vague guidance produces vague results.

- Good: "Use `pnpm test --filter api` to run API tests"
- Bad: "Make sure to test your changes"

**Keep it current.** Treat AGENTS.md as living documentation. Update it when build commands, conventions, or architecture change.

**Don't duplicate the README.** AGENTS.md is for agent-specific operational context, not project descriptions or contribution guidelines aimed at humans.

**Don't include secrets.** No API keys, tokens, or credentials. Reference environment variables or secret management tools instead.

**Don't over-constrain.** Give agents enough guidance to be effective without prescribing every decision. Focus on conventions that matter for consistency.

**Include what you'd tell a new teammate on day one.** Build commands, test commands, deployment steps, naming conventions, security gotchas, how the codebase is organized.
