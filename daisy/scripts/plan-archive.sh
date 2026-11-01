#!/usr/bin/env bash
# Invocation: run as `daisy plan-archive` — do not execute this file directly.
# Archive the active Daisy plan: move plan file to archive/, remove PLAN.md symlink.
# Usage: plan-archive.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

require_env || exit 1

if [ -z "$DAISY_WORKSPACE" ]; then
    echo "Error: Could not determine workspace root" >&2
    exit 1
fi

PLAN_LINK="$DAISY_WORKSPACE/PLAN.md"

if [ ! -e "$PLAN_LINK" ] && [ ! -L "$PLAN_LINK" ]; then
    echo "Error: PLAN.md does not exist in this workspace" >&2
    exit 1
fi

if [ ! -L "$PLAN_LINK" ]; then
    echo "Error: PLAN.md exists but is not a symlink (not a Daisy plan)" >&2
    exit 1
fi

# Get the filename from the symlink target
FILENAME=$(basename "$(readlink "$PLAN_LINK")")

SRC="$DAISY_HOME/plans/$FILENAME"
DEST="$DAISY_HOME/plans/archive/$FILENAME"

if [ ! -f "$SRC" ]; then
    echo "Error: Plan file not found at $SRC" >&2
    echo "The symlink may be broken. Remove PLAN.md manually to proceed." >&2
    exit 1
fi

# Move to archive and remove symlink
mv "$SRC" "$DEST"
rm "$PLAN_LINK"

echo "✅ Archived: $FILENAME"

# Project files link to plans by relative path, so the move invalidates every
# link naming this one. This is the only point that knows both the old and the
# new path, so it is the only point that can repair them without a scan.
# FILENAME is full of dots — escape it before it reaches sed as a pattern.
ESCAPED=$(printf '%s' "$FILENAME" | sed 's/[.[\*^$/]/\\&/g')
UPDATED=0
for PROJECT_FILE in "$DAISY_HOME"/projects/*.md "$DAISY_HOME"/projects/_archive/*.md; do
    [ -f "$PROJECT_FILE" ] || continue
    grep -q "\.\./plans/$ESCAPED" "$PROJECT_FILE" || continue
    sed -i "s|\.\./plans/$ESCAPED|../plans/archive/$FILENAME|g" "$PROJECT_FILE"
    UPDATED=$((UPDATED + 1))
done
if [ "$UPDATED" -gt 0 ]; then
    echo "  ✓ Repointed links in $UPDATED project file(s)"
fi

# Commit
daisy commit --home "Archive plan: $FILENAME"

exit 0
