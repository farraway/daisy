#!/usr/bin/env bash
# Shared functions for daisy scripts
# Source this at the top of each script:
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "$SCRIPT_DIR/common.sh"

# Resolve DAISY_HOME for the current workspace.
# 1. Walk up from $PWD looking for .daisy/home
# 2. Fall back to $DAISY_HOME env var
# 3. Error if neither exists
#
# Sets: DAISY_HOME, DAISY_HOME_NAME
resolve_home() {
    # Walk up directory tree looking for .daisy/home
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
            export DAISY_HOME DAISY_HOME_NAME
            return 0
        fi
        dir=$(dirname "$dir")
    done

    # Fall back to DAISY_HOME env var
    if [ -n "$DAISY_HOME" ]; then
        DAISY_HOME_NAME=$(basename "$DAISY_HOME")
        export DAISY_HOME DAISY_HOME_NAME
        return 0
    fi

    echo "Error: Cannot resolve home. No .daisy/home found and DAISY_HOME not set." >&2
    echo "  Run 'daisy init <home>' in your workspace, or set DAISY_HOME in ~/.zshenv" >&2
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
