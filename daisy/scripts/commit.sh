#!/usr/bin/env bash
# Invocation: run as `daisy commit` — do not execute this file directly.
# Daisy git commit helper
# Commits changes scoped to either the active home's data or everything else
# Usage: commit.sh --home|--not-home "commit message"

set -e

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

if [ ! -d "$DAISY_ROOT/.git" ]; then
    echo "Error: DAISY_ROOT is not a git repository" >&2
    exit 1
fi

if [ "$MODE" = "--home" ]; then
    if [ -z "$DAISY_HOME_NAME" ]; then
        echo "Error: DAISY_HOME_NAME not set" >&2
        exit 1
    fi
    PATHSPEC=(-- "home/$DAISY_HOME_NAME")
    SCOPE_LABEL="home/$DAISY_HOME_NAME"
else
    PATHSPEC=(-- . ":!home")
    SCOPE_LABEL="everything except home/"
fi

# Navigate to repo and commit
cd "$DAISY_ROOT" || exit 1

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

git commit -m "$MESSAGE" || {
    echo "Error: Failed to commit changes" >&2
    exit 1
}

# Success
COMMIT_HASH=$(git rev-parse --short HEAD)
echo "📝 Committed: $MESSAGE ($COMMIT_HASH)"
exit 0
