#!/usr/bin/env bash
# Mark a task as complete
# Usage: done.sh "task pattern"

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_env || exit 1

    if [ ! -f "$DAISY_HOME/journal/today.md" ]; then
        echo "Error: today.md not found" >&2
        exit 1
    fi

    if [ ! -f "$DAISY_HOME/tasks/todo.txt" ]; then
        echo "Error: todo.txt not found" >&2
        exit 1
    fi

    exit 0
fi

require_env || exit 1

# Run master health check
if ! "$DAISY_ROOT/daisy/scripts/healthcheck.sh" >/dev/null 2>&1; then
    echo "Error: System health check failed" >&2
    echo "Run: $DAISY_ROOT/daisy/scripts/healthcheck.sh" >&2
    exit 1
fi

# Check for pattern argument
if [ -z "$1" ]; then
    echo "Error: Task pattern required" >&2
    echo "Usage: done.sh \"task pattern\"" >&2
    exit 1
fi

PATTERN="$1"
TODAY=$(date +%Y-%m-%d)
TIME=$(date +%H%M)

# Mark complete in today.md
if [ -f "$DAISY_HOME/journal/today.md" ]; then
    # Find first matching incomplete task and mark complete
    if grep -i "- \[ \].*$PATTERN" "$DAISY_HOME/journal/today.md" > /dev/null; then
        # Extract the task description for reporting
        TASK_DESC=$(grep -i -m 1 "- \[ \].*$PATTERN" "$DAISY_HOME/journal/today.md" | sed 's/^- \[ \] //')
        
        # Mark first match as complete
        sed -i.bak "0,/- \[ \].*$PATTERN/s/- \[ \]/- [x]/" "$DAISY_HOME/journal/today.md"
        rm -f "$DAISY_HOME/journal/today.md.bak"
        echo "✅ Marked complete in today.md: $TASK_DESC"
    else
        echo "⚠️  Task not found in today.md (may not be in active tasks)"
    fi
fi

# Mark complete in todo.txt
if [ -f "$DAISY_HOME/tasks/todo.txt" ]; then
    # Find matching active task
    if grep -i -v "^[xz] " "$DAISY_HOME/tasks/todo.txt" | grep -i "$PATTERN" > /dev/null; then
        # Extract original task
        ORIGINAL=$(grep -i -v "^[xz] " "$DAISY_HOME/tasks/todo.txt" | grep -i -m 1 "$PATTERN")
        
        # Extract creation date and description (remove priority if present)
        CREATION_DATE=$(echo "$ORIGINAL" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1)
        DESCRIPTION=$(echo "$ORIGINAL" | sed -E 's/^(\([A-D]\) )?[0-9]{4}-[0-9]{2}-[0-9]{2} //')
        
        # Create completed task format
        COMPLETED="x $TODAY $CREATION_DATE $DESCRIPTION"
        
        # Remove original task and append completed version to end
        grep -i -v "$PATTERN" "$DAISY_HOME/tasks/todo.txt" > "$DAISY_HOME/tasks/todo.txt.tmp" || true
        mv "$DAISY_HOME/tasks/todo.txt.tmp" "$DAISY_HOME/tasks/todo.txt"
        echo "$COMPLETED" >> "$DAISY_HOME/tasks/todo.txt"
        
        echo "✅ Marked complete in todo.txt"
        
        # Extract brief summary for commit (first 50 chars of description)
        SUMMARY=$(echo "$DESCRIPTION" | cut -c1-50)
    else
        echo "⚠️  Task not found in todo.txt"
        SUMMARY="$PATTERN"
    fi
fi

# Add log entry
if [ -f "$DAISY_HOME/journal/today.md" ]; then
    # Find Log section and append
    if grep "^#### Log" "$DAISY_HOME/journal/today.md" > /dev/null; then
        # Insert after Log heading
        sed -i.bak "/^#### Log/a\\
\\
- $TIME Completed: $SUMMARY" "$DAISY_HOME/journal/today.md"
        rm -f "$DAISY_HOME/journal/today.md.bak"
    fi
fi

echo "✅ Logged completion at $TIME"

# Commit changes
"$DAISY_ROOT/daisy/scripts/commit.sh" "Completed: $SUMMARY"

exit 0
