#!/usr/bin/env bash
# Daisy git commit helper
# Commits changes to the active home directory
# Usage: commit.sh "commit message"

set -e

# Check for commit message
if [ -z "$1" ]; then
    echo "Error: Commit message required" >&2
    echo "Usage: commit.sh \"commit message\"" >&2
    exit 1
fi

MESSAGE="$1"

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

# Navigate to repo and commit
cd "$DAISY_ROOT" || exit 1

# Check if there are changes to commit
if git diff --quiet HEAD home 2>/dev/null && git diff --cached --quiet home 2>/dev/null; then
    echo "No changes to commit in home/" >&2
    exit 0
fi

# Stage and commit
git add home || {
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
