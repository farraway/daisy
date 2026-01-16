#!/usr/bin/env bash
# Start a new week - archives completed tasks and starts new day
# Usage: new-week.sh

set -e

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    if [ -z "$DAISY_HOME" ]; then
        echo "Error: DAISY_HOME not set" >&2
        exit 1
    fi
    
    if [ ! -f "$DAISY_HOME/tasks/todo.txt" ]; then
        echo "Error: todo.txt not found" >&2
        exit 1
    fi
    
    if [ ! -f "$DAISY_HOME/tasks/done.txt" ]; then
        echo "Error: done.txt not found" >&2
        exit 1
    fi
    
    exit 0
fi

# Run master health check
if ! "$DAISY_ROOT/scripts/healthcheck.sh" >/dev/null 2>&1; then
    echo "Error: System health check failed" >&2
    echo "Run: $DAISY_ROOT/scripts/healthcheck.sh" >&2
    exit 1
fi

# Delete cancelled tasks
cancelled_count=0
if [ -f "$DAISY_HOME/tasks/todo.txt" ]; then
    original_count=$(wc -l < "$DAISY_HOME/tasks/todo.txt")
    grep -v "^z " "$DAISY_HOME/tasks/todo.txt" > "$DAISY_HOME/tasks/todo.txt.tmp" || true
    new_count=$(wc -l < "$DAISY_HOME/tasks/todo.txt.tmp")
    cancelled_count=$((original_count - new_count))
    mv "$DAISY_HOME/tasks/todo.txt.tmp" "$DAISY_HOME/tasks/todo.txt"
    
    if [ $cancelled_count -gt 0 ]; then
        echo "🗑️  Deleted $cancelled_count cancelled task(s)"
    fi
fi

# Archive completed tasks to done.txt
completed_count=0
if [ -f "$DAISY_HOME/tasks/todo.txt" ]; then
    # Extract completed tasks
    grep "^x " "$DAISY_HOME/tasks/todo.txt" >> "$DAISY_HOME/tasks/done.txt" || true
    
    # Count completed tasks
    completed_count=$(grep -c "^x " "$DAISY_HOME/tasks/todo.txt" || echo "0")
    
    # Keep only active tasks
    grep -v "^x " "$DAISY_HOME/tasks/todo.txt" > "$DAISY_HOME/tasks/todo.txt.tmp" || true
    mv "$DAISY_HOME/tasks/todo.txt.tmp" "$DAISY_HOME/tasks/todo.txt"
    
    if [ $completed_count -gt 0 ]; then
        echo "📦 Archived $completed_count completed task(s) to done.txt"
    fi
fi

# Get current date for commit message
DATE=$(date +%Y-%m-%d)

# Commit weekly archival
"$DAISY_ROOT/scripts/commit.sh" "New week: $DATE"

# Now start a new day
"$DAISY_ROOT/scripts/daisy/new-day.sh"

exit 0
