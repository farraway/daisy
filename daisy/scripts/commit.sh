#!/usr/bin/env bash
# Invocation: run as `daisy commit` — do not execute this file directly.
# Daisy git commit helper
# --home commits the active home's data in the home's own repository;
# --not-home commits $DAISY_ROOT's repository minus any registered home that
# still lies inside it (none, once the homes have their own repositories).
# Usage: commit.sh --home|--not-home "commit message"

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

usage() {
    echo "Usage: commit.sh --home|--not-home \"commit message\"" >&2
}

MODE="$1"
MESSAGE="$2"

case "$MODE" in
    --home|--not-home) ;;
    *)
        usage
        exit 1
        ;;
esac

if [ -z "$MESSAGE" ]; then
    echo "Error: Commit message required" >&2
    usage
    exit 1
fi

# Validate environment (quick check)
if [ -z "$DAISY_ROOT" ]; then
    echo "Error: DAISY_ROOT not set" >&2
    echo "Add to ~/.zshenv: export DAISY_ROOT=/path/to/daisy" >&2
    exit 1
fi

if [ ! -d "$DAISY_ROOT" ]; then
    echo "Error: DAISY_ROOT directory not found: $DAISY_ROOT" >&2
    exit 1
fi

if [ "$MODE" = "--home" ]; then
    if [ -z "$DAISY_HOME" ] || [ -z "$DAISY_HOME_NAME" ]; then
        echo "Error: DAISY_HOME not resolved" >&2
        exit 1
    fi
    if ! daisy_home_repo_ok "$DAISY_HOME"; then
        echo "Error: Home '$DAISY_HOME_NAME' at $DAISY_HOME is not in its own git repository (or the root's)" >&2
        echo "  Run: git -C $(printf '%q' "$DAISY_HOME") init" >&2
        exit 1
    fi
    REPO="$DAISY_HOME"
    PATHSPEC=(-- .)
    SCOPE_LABEL="home '$DAISY_HOME_NAME'"
    # Narrower than the repository while the home is a subdirectory of the
    # root's (until the split).
    SCOPED=0
    daisy_is_repo_root "$REPO" || SCOPED=1
else
    if ! daisy_is_repo_root "$DAISY_ROOT"; then
        echo "Error: DAISY_ROOT is not a git repository" >&2
        exit 1
    fi
    REPO="$DAISY_ROOT"
    PATHSPEC=(-- .)
    SCOPE_LABEL="root"
    # Narrower than the repository while anything is excluded from it (see
    # daisy_root_exclusions). Exclusions are literal: a path is a path, not
    # pathspec magic or a glob.
    SCOPED=0
    EXCLUDED=""
    while IFS= read -r rel; do
        PATHSPEC+=(":(exclude,literal)$rel")
        EXCLUDED="${EXCLUDED:+$EXCLUDED, }$rel"
    done < <(daisy_root_exclusions)
    if [ -n "$EXCLUDED" ]; then
        SCOPED=1
        SCOPE_LABEL="root (excluding $EXCLUDED)"
    fi
fi

# Navigate to repo and commit. The pathspec is relative to the repo path.
cd "$REPO" || exit 1

# Check if there are changes to commit. `git diff` alone only sees changes
# to already-tracked files — a brand-new untracked file (e.g. a freshly
# picked-up plan) wouldn't show, and this would silently no-op. `git status
# --porcelain` catches untracked files too.
if [ -z "$(git status --porcelain "${PATHSPEC[@]}")" ]; then
    echo "No changes to commit in $SCOPE_LABEL" >&2
    exit 0
fi

# Stage and commit
git add "${PATHSPEC[@]}" || {
    echo "Error: Failed to stage changes" >&2
    exit 1
}

# While the scope is narrower than the repository (a home still under the
# root; the root with homes still inside it), the commit takes the pathspec
# too, or a bare `git commit` would sweep anything already staged elsewhere
# in the index into it. When the scope is the whole repository the commit is
# bare: a partial commit is refused during a merge, and a repository synced
# between machines will see merges.
COMMIT_PATHSPEC=()
if [ "$SCOPED" = "1" ]; then
    COMMIT_PATHSPEC=("${PATHSPEC[@]}")
fi
git commit -m "$MESSAGE" "${COMMIT_PATHSPEC[@]}" || {
    echo "Error: Failed to commit changes" >&2
    exit 1
}

# Success
COMMIT_HASH=$(git rev-parse --short HEAD)
echo "📝 Committed: $MESSAGE ($COMMIT_HASH)"
exit 0
