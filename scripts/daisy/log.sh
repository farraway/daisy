#!/usr/bin/env bash
# Add a log entry to today.md
# Usage: log.sh "log message"

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

    exit 0
fi

require_env || exit 1

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
    # Find Log section and insert at the blank line before next section
    if grep "^#### Log" "$DAISY_HOME/journal/today.md" > /dev/null; then
        # Use awk to insert at the last blank line in Log section (before next section)
        awk -v time="$TIME" -v msg="$MESSAGE" '
        /^#### Log/ { in_log=1; found_content=0; print; next }
        in_log && /^$/ && found_content { 
            # Blank line after we found content - insert here
            print "- " time " " msg
            in_log=0
        }
        in_log && /^[^$]/ { found_content=1 }
        { print }
        END {
            # If still in log section at end of file, append
            if (in_log) print "- " time " " msg
        }
        ' "$DAISY_HOME/journal/today.md" > "$DAISY_HOME/journal/today.md.tmp"
        
        mv "$DAISY_HOME/journal/today.md.tmp" "$DAISY_HOME/journal/today.md"
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
