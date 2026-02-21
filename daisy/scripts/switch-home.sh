#!/usr/bin/env bash
# Switch active daisy home (DEPRECATED - use daisy-init <home> instead)
# Kept for backward compatibility. Rebuilds AGENTS.md for a home.
# Usage: switch-home.sh <home-name>

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_root || exit 1
    if [ ! -d "$DAISY_ROOT/home" ]; then
        echo "Error: home directory not found" >&2
        exit 1
    fi
    exit 0
fi

require_root || exit 1

echo "⚠️  switch-home.sh is deprecated. Use 'daisy-init <home>' in your workspace instead." >&2
echo "" >&2

# Check for target home argument
if [ -z "$1" ]; then
    echo "Usage: switch-home.sh <home-name>" >&2
    echo "" >&2
    echo "Available homes:" >&2
    for dir in "$DAISY_ROOT"/home/*/; do
        name=$(basename "$dir")
        if [ "$DAISY_HOME" = "$DAISY_ROOT/home/$name" ]; then
            echo "  $name (active)" >&2
        else
            echo "  $name" >&2
        fi
    done
    exit 1
fi

TARGET="$1"
TARGET_DIR="$DAISY_ROOT/home/$TARGET"

# Verify target home exists
if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: Home '$TARGET' not found at $TARGET_DIR" >&2
    echo "" >&2
    echo "Available homes:" >&2
    for dir in "$DAISY_ROOT"/home/*/; do
        echo "  $(basename "$dir")" >&2
    done
    echo "" >&2
    echo "To create a new home: $DAISY_ROOT/daisy/scripts/create-home.sh $TARGET" >&2
    exit 1
fi

# Verify target home has required structure
for required in "include.txt" "tasks" "journal" "projects"; do
    if [ "$required" = "include.txt" ]; then
        if [ ! -f "$TARGET_DIR/$required" ]; then
            echo "Error: $TARGET_DIR/$required not found" >&2
            exit 1
        fi
    else
        if [ ! -d "$TARGET_DIR/$required" ]; then
            echo "Error: $TARGET_DIR/$required/ not found" >&2
            exit 1
        fi
    fi
done

# Detect current home
CURRENT_NAME=""
if [ -n "$DAISY_HOME" ]; then
    CURRENT_NAME=$(basename "$DAISY_HOME")
fi

# Check if already on target
if [ "$CURRENT_NAME" = "$TARGET" ]; then
    echo "Already on home: $TARGET" >&2
    exit 0
fi

cd "$DAISY_ROOT"

# Remove old symlinks (if they exist)
for link in tasks journal.md today.md projects; do
    rm -f "$link"
done

# Create new symlinks
ln -sf "home/$TARGET/tasks" tasks
ln -sf "home/$TARGET/journal/journal.md" journal.md
ln -sf "home/$TARGET/journal/today.md" today.md
ln -sf "home/$TARGET/projects" projects

# Rebuild AGENTS.md
DAISY_HOME="$TARGET_DIR" "$DAISY_ROOT/daisy/scripts/build-prompt.sh"

# Report
if [ -n "$CURRENT_NAME" ]; then
    echo "Deactivated home: $CURRENT_NAME"
fi
echo "✅ Activated home: $TARGET"

# Check if DAISY_HOME env var needs updating
if [ "$DAISY_HOME" != "$TARGET_DIR" ]; then
    echo ""
    echo "⚠️  Update your environment to persist this change:"
    echo "   Edit ~/.zshenv and set:"
    echo "     export DAISY_HOME=\"\$DAISY_ROOT/home/$TARGET\""
    echo "   Then run: source ~/.zshenv"
fi

exit 0
