#!/usr/bin/env bash
# Invocation: sourced by other daisy scripts — not executed directly, no `daisy` command.
# Shared functions for daisy scripts
# Source this at the top of each script:
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "$SCRIPT_DIR/common.sh"

# Bash >= 5.2 defaults 'patsub_replacement' to on, which makes '&' in the
# replacement side of ${var//pattern/replacement} mean "the matched text"
# (sed-style). Every script that builds today.md by substituting task text
# into a template via this construct silently corrupts any '&' in a task
# description unless this is off. No-op (with the harmless error suppressed)
# on bash < 5.2, which doesn't have the option.
shopt -u patsub_replacement 2>/dev/null || true

# Resolve DAISY_HOME for the current workspace.
# 1. Walk up from $PWD looking for .daisy/home (authoritative)
# 2. Fall back to $DAISY_HOME env var (convenience default, warns)
# 3. Error if neither exists
#
# Sets: DAISY_HOME, DAISY_HOME_NAME
resolve_home() {
    # Walk up directory tree looking for .daisy/home — this is the authoritative source
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        if [ -f "$dir/.daisy/home" ]; then
            DAISY_HOME_NAME=$(cat "$dir/.daisy/home")
            if [ -z "$DAISY_ROOT" ]; then
                echo "Error: DAISY_ROOT not set" >&2
                return 1
            fi
            DAISY_HOME="$DAISY_ROOT/home/$DAISY_HOME_NAME"
            if [ ! -d "$DAISY_HOME" ]; then
                echo "Error: Home '$DAISY_HOME_NAME' not found at $DAISY_HOME" >&2
                return 1
            fi
            DAISY_WORKSPACE="$dir"
            export DAISY_HOME DAISY_HOME_NAME DAISY_WORKSPACE
            return 0
        fi
        dir=$(dirname "$dir")
    done

    # Fall back to DAISY_DEFAULT_HOME env var (default home name) — warn so silent wrong-home bugs are visible
    if [ -n "$DAISY_DEFAULT_HOME" ]; then
        DAISY_HOME_NAME="$DAISY_DEFAULT_HOME"
        DAISY_HOME="$DAISY_ROOT/home/$DAISY_HOME_NAME"
        if [ ! -d "$DAISY_HOME" ]; then
            echo "Error: Home '$DAISY_HOME_NAME' not found at $DAISY_HOME" >&2
            return 1
        fi
        echo "⚠️  No .daisy/home found; falling back to \$DAISY_DEFAULT_HOME ($DAISY_HOME_NAME)" >&2
        export DAISY_HOME DAISY_HOME_NAME
        return 0
    fi

    echo "Error: Cannot resolve home. No .daisy/home found and DAISY_DEFAULT_HOME not set." >&2
    echo "  Run 'daisy init <home>' in your workspace, or set DAISY_DEFAULT_HOME=<name> in ~/.zshenv" >&2
    return 1
}

# Require DAISY_ROOT to be set. Call early in every script.
require_root() {
    if [ -z "$DAISY_ROOT" ]; then
        echo "Error: DAISY_ROOT not set" >&2
        echo "  Add to ~/.zshenv: export DAISY_ROOT=\"/path/to/daisy\"" >&2
        return 1
    fi
    if [ ! -d "$DAISY_ROOT" ]; then
        echo "Error: DAISY_ROOT directory does not exist: $DAISY_ROOT" >&2
        return 1
    fi
}

# Require both DAISY_ROOT and DAISY_HOME. Resolves home via .daisy/home or env var.
require_env() {
    require_root || return 1
    resolve_home || return 1
}

# --- Shared workspace-install artifact lists ---
# Single source of truth for what `daisy-init.sh` installs into a workspace,
# so `cmd_clean` (daisy.sh) can't silently drift out of sync with it again.

DAISY_CURSOR_RULE_FILES=(
    "daisy.mdc"
    "daisy-logging.mdc"
    "daisy-plan.mdc"
)

# Names that used to be current (pre-.mdc-rename, or names later dropped from
# DAISY_CURSOR_RULE_FILES) and must be swept from .cursor/rules/ on init —
# daisy-init.sh only ever writes the names above, so a name that left this
# list stays behind forever otherwise.
DAISY_CURSOR_RULE_OBSOLETE=(
    "daisy.md"
    "daisy-logging.md"
)

DAISY_CLAUDE_COMMAND_FILE=".claude/commands/daisy.md"

DAISY_ALLOW_BASE=(
    "Read(.daisy/**)"
    "Edit(.daisy/**)"
    "Bash(daisy:*)"
)

DAISY_GITIGNORE_BASE_ENTRIES=(
    ".daisy/"
    ".cursor/rules/daisy.mdc"
    ".cursor/rules/daisy-logging.mdc"
    ".cursor/rules/daisy-plan.mdc"
    ".claude/commands/daisy.md"
    ".claude/settings.local.json"
    "PLAN.md"
)

DAISY_CURSORIGNORE_BASE_ENTRIES=(
    "!.daisy/"
    "!.cursor/rules/daisy.mdc"
    "!.cursor/rules/daisy-logging.mdc"
    "!PLAN.md"
)
