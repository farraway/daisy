#!/usr/bin/env bash
# Add a log entry to today.md
# Usage: log.sh "log message"

set -e

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    if [ -z "$DAISY_HOME" ]; then
        echo "Error: DAISY_HOME not set" >&2
        exit 1
    fi
    
    if [ ! -f "$DAISY_HOME/journal/today.md" ]; then
        echo "Error: today.md not found" >&2
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

# Check for message argument
if [ -z "$1" ]; then
    echo "Error: Log message required" >&2
    echo "Usage: log.sh \"log message\"" >&2
    exit 1
fi

MESSAGE="$1"
TIME=$(date +%H%M)

# Add log entry to today.md
if [ -f "$DAISY_HOME/journal/today.md" ]; then
    # Find Log section and append entry
    if grep "^#### Log" "$DAISY_HOME/journal/today.md" > /dev/null; then
        # Append after Log section
        sed -i.bak "/^#### Log/a\\
\\
- $TIME $MESSAGE" "$DAISY_HOME/journal/today.md"
        rm -f "$DAISY_HOME/journal/today.md.bak"
        echo "✅ Logged: $TIME $MESSAGE"
    else
        echo "Error: Log section not found in today.md" >&2
        exit 1
    fi
else
    echo "Error: today.md not found" >&2
    exit 1
fi

# Extract summary for commit (first 60 chars)
SUMMARY=$(echo "$MESSAGE" | cut -c1-60)

# Commit changes
"$DAISY_ROOT/scripts/commit.sh" "Log: $TIME - $SUMMARY"

exit 0
