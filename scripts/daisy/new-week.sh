#!/usr/bin/env bash
# Start a new week - archives completed tasks and starts new day with weekly template
# Usage: new-week.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_env || exit 1

    if [ ! -f "$DAISY_ROOT/templates/journal-week.md" ]; then
        echo "Error: Template missing: templates/journal-week.md" >&2
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

require_env || exit 1

# Run master health check
if ! "$DAISY_ROOT/scripts/healthcheck.sh" >/dev/null 2>&1; then
    echo "Error: System health check failed" >&2
    echo "Run: $DAISY_ROOT/scripts/healthcheck.sh" >&2
    exit 1
fi

# Archive yesterday's work (lossless) with weekly retrospective
if [ -f "$DAISY_HOME/journal/today.md" ]; then
    # Check if journal.md has content, add separator if so
    if [ -s "$DAISY_HOME/journal/journal.md" ]; then
        echo -e "\n---\n" >> "$DAISY_HOME/journal/journal.md"
    fi
    cat "$DAISY_HOME/journal/today.md" >> "$DAISY_HOME/journal/journal.md"
    echo "📦 Archived yesterday to journal.md"
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

# Extract tasks from todo.txt
high_priority_tasks=()
next_priority_tasks=()
inbox_tasks=()
github_tasks=()

while IFS= read -r line; do
    # Skip empty lines, completed (x), and cancelled (z)
    [[ -z "$line" ]] && continue
    [[ "$line" =~ ^x\  ]] && continue
    [[ "$line" =~ ^z\  ]] && continue
    
    # Check for @git or @github
    if [[ "$line" =~ @git ]] || [[ "$line" =~ @github ]]; then
        # Extract description (everything after date)
        desc=$(echo "$line" | sed -E 's/^(\([A-D]\) )?[0-9]{4}-[0-9]{2}-[0-9]{2} //')
        github_tasks+=("- [ ] $desc")
        continue
    fi
    
    # Priority A tasks
    if [[ "$line" =~ ^\(A\)\  ]]; then
        desc=$(echo "$line" | sed -E 's/^\(A\) [0-9]{4}-[0-9]{2}-[0-9]{2} //')
        high_priority_tasks+=("- [ ] $desc")
    # Priority B tasks
    elif [[ "$line" =~ ^\(B\)\  ]]; then
        desc=$(echo "$line" | sed -E 's/^\(B\) [0-9]{4}-[0-9]{2}-[0-9]{2} //')
        next_priority_tasks+=("- [ ] $desc")
    # Inbox tasks (no priority)
    elif [[ "$line" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}\  ]]; then
        desc=$(echo "$line" | sed -E 's/^[0-9]{4}-[0-9]{2}-[0-9]{2} //')
        inbox_tasks+=("- [ ] $desc")
    fi
done < "$DAISY_HOME/tasks/todo.txt"

# Get current date/time
DATE=$(date +%Y-%m-%d)
DAY=$(date +%A)
TIME=$(date +%H%M)

# Generate new today.md from WEEKLY template
TEMPLATE=$(cat "$DAISY_ROOT/templates/journal-week.md")

# Replace placeholders
TEMPLATE="${TEMPLATE//\{DATE\}/$DATE}"
TEMPLATE="${TEMPLATE//\{DAY\}/$DAY}"
TEMPLATE="${TEMPLATE//\{TIME\}/$TIME}"

# Build task sections
HIGH_PRIORITY_SECTION=""
if [ ${#high_priority_tasks[@]} -gt 0 ]; then
    for task in "${high_priority_tasks[@]}"; do
        HIGH_PRIORITY_SECTION+="$task"$'\n'
    done
fi

NEXT_PRIORITY_SECTION=""
if [ ${#next_priority_tasks[@]} -gt 0 ]; then
    for task in "${next_priority_tasks[@]}"; do
        NEXT_PRIORITY_SECTION+="$task"$'\n'
    done
fi

INBOX_SECTION=""
if [ ${#inbox_tasks[@]} -gt 0 ]; then
    for task in "${inbox_tasks[@]}"; do
        INBOX_SECTION+="$task"$'\n'
    done
fi

GITHUB_SECTION=""
if [ ${#github_tasks[@]} -gt 0 ]; then
    for task in "${github_tasks[@]}"; do
        GITHUB_SECTION+="$task"$'\n'
    done
fi

# Replace task placeholders
TEMPLATE="${TEMPLATE//\{HIGH_PRIORITY_TASKS\}/$HIGH_PRIORITY_SECTION}"
TEMPLATE="${TEMPLATE//\{NEXT_PRIORITY_TASKS\}/$NEXT_PRIORITY_SECTION}"
TEMPLATE="${TEMPLATE//\{INBOX_TASKS\}/$INBOX_SECTION}"
TEMPLATE="${TEMPLATE//\{GITHUB_TASKS\}/$GITHUB_SECTION}"

# Write new today.md
echo "$TEMPLATE" > "$DAISY_HOME/journal/today.md"

# Report
echo "✅ New week started: $DATE $DAY"
echo "   High priority tasks: ${#high_priority_tasks[@]}"
echo "   Next priority tasks: ${#next_priority_tasks[@]}"
echo "   Inbox tasks: ${#inbox_tasks[@]}"
echo "   GitHub tasks: ${#github_tasks[@]}"

# Commit changes
"$DAISY_ROOT/scripts/commit.sh" "New week: $DATE $DAY"

exit 0
