# Home Management - Architecture

## Per-Workspace Home Resolution

Homes are resolved per-workspace, not globally. Each workspace has a `.daisy/` directory containing:
- `home` - plain text file with the home name (e.g., "work")
- Symlinks to the active home's data (tasks/, today.md, journal.md, projects/, AGENTS.md)

**Resolution order** (used by all scripts via `daisy/scripts/common.sh`):
1. Walk up from `$PWD` looking for `.daisy/home`
2. Fall back to `$DAISY_HOME` env var
3. Error if neither exists

This allows different workspaces on the same machine to use different homes concurrently.

## Detecting Active Home

```
1. Walk up from $PWD looking for .daisy/home file
   - If found, read home name from file
   - Resolve: DAISY_HOME = $DAISY_ROOT/home/{name}
2. Fall back to $DAISY_HOME environment variable
3. If neither: error "Cannot resolve home. Run daisy-init <home>"
4. Verify home directory exists at $DAISY_HOME
5. Verify include.txt exists at $DAISY_HOME/include.txt
```

## System Health Check Algorithm

**Command:** "check system" or "verify setup" or "check home"

```
1. Resolve active home (via .daisy/home or $DAISY_HOME)
   - If neither found: Report error with instructions

2. Verify home structure:
   a. Check if $DAISY_HOME directory exists
   b. Check if $DAISY_HOME/include.txt exists
   c. Check if $DAISY_HOME/tasks/ directory exists
   d. Check if $DAISY_HOME/journal/ directory exists
   e. Check if $DAISY_HOME/projects/ directory exists

3. Verify Required Files:
   a-e. Check todo.txt, done.txt, alias.txt, journal.md, today.md exist

4. Validate todo.txt format (if exists)

5. Check for orphaned completed tasks

6. Verify AGENTS.md is up to date:
   a. Check if $DAISY_HOME/AGENTS.md exists
   b. If missing: Report "Run: build-prompt.sh {home-name}"
   c. If include.txt is newer than AGENTS.md: Report stale

7. Final Report with suggestions for any issues found
```

## Home Switching (Per-Workspace)

**Command:** "switch to [home]" or re-run `daisy-init <home>`

```
1. Run: daisy-init <home-name>
   - This replaces .daisy/ symlinks in the current workspace
   - Does NOT affect other workspaces

2. No global env var change needed
   - Each workspace resolves its own home from .daisy/home
```

## Creating New Home

**Command:** "create home [name]"

```
1. Check if home/{name}/ exists
   - If exists, error: "Home '{name}' already exists"

2. Copy daisy/templates/home/ to home/{name}/

3. Instruct user to customize home/{name}/include.txt

4. Build AGENTS.md:
   a. Run $DAISY_ROOT/daisy/scripts/build-prompt.sh {name}
   b. Output: home/{name}/AGENTS.md

5. Ask: "Activate this home in the current workspace?"
   - If yes, run: daisy-init {name}
```

## Related Scripts

| Script | Purpose |
|--------|---------|
| `daisy-init.sh` | Initialize/switch home in a workspace |
| `create-home.sh` | Create new home from template |
| `common.sh` | Shared home resolution functions |
| `healthcheck.sh` | System validation |
| `switch-home.sh` | DEPRECATED - use daisy-init instead |
