#!/usr/bin/env bash
# Invocation: run as `daisy done` — do not execute this file directly.
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
TODO_FILE="$DAISY_HOME/tasks/todo.txt"
TODAY_FILE="$DAISY_HOME/journal/today.md"

# --- Resolve exactly one active task from todo.txt ---
# Literal (-F), case-insensitive (-i) substring; -e guards patterns that
# start with "-". Only active lines are candidates — historical x/z records
# are excluded before matching, not filtered out of the results afterward.
mapfile -t MATCHES < <(grep -v "^[xz] " "$TODO_FILE" | grep -F -i -e "$PATTERN")

if [ "${#MATCHES[@]}" -eq 0 ]; then
    echo "No task found matching: $PATTERN" >&2
    exit 1
fi

if [ "${#MATCHES[@]}" -gt 1 ]; then
    echo "Multiple tasks match \"$PATTERN\":" >&2
    for i in "${!MATCHES[@]}"; do
        printf '  %d) %s\n' "$((i + 1))" "${MATCHES[$i]}" >&2
    done
    exit 1
fi

ORIGINAL="${MATCHES[0]}"
CREATION_DATE=$(echo "$ORIGINAL" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1)
DESCRIPTION=$(echo "$ORIGINAL" | sed -E 's/^(\([A-D]\) )?[0-9]{4}-[0-9]{2}-[0-9]{2} //')

# --- Complete in todo.txt by whole-line identity, never by pattern ---
# Only the one resolved line is a candidate for removal — a second task or a
# historical record that happens to also match $PATTERN is untouched.
COMPLETED="x $TODAY $CREATION_DATE $DESCRIPTION"
awk -v line="$ORIGINAL" '$0 != line { print }' "$TODO_FILE" > "$TODO_FILE.tmp"
echo "$COMPLETED" >> "$TODO_FILE.tmp"
mv "$TODO_FILE.tmp" "$TODO_FILE"

# --- Complete in today.md by exact whole-line equality ---
# Flips only the first line that equals "- [ ] $DESCRIPTION" exactly; every
# other line — including ones elsewhere in the file that merely contain the
# pattern — passes through byte-for-byte. Absent from today.md is a warning,
# not a failure: Inbox-only items (checked off manually) are expected here.
TODAY_UPDATED=1
if awk -v target="$DESCRIPTION" '
    BEGIN { hit = 0 }
    !hit && $0 == "- [ ] " target { print "- [x] " target; hit = 1; next }
    { print }
    END { exit (hit ? 0 : 1) }
' "$TODAY_FILE" > "$TODAY_FILE.tmp"; then
    mv "$TODAY_FILE.tmp" "$TODAY_FILE"
else
    rm -f "$TODAY_FILE.tmp"
    TODAY_UPDATED=0
fi

# --- Journal, then commit ---
# `daisy log` performs its own commit, so by the time the trailing `daisy
# commit --home` below runs, the edits above are usually already committed
# and it no-ops (commit.sh already guards on `git status --porcelain`). If
# journaling fails, nothing has been committed yet, so this call is what
# actually commits the todo.txt/today.md edits — caught, not left to set -e,
# so a journal failure can't abort before the edits are saved.
set +e
LOG_OUTPUT=$(daisy log "Done: $DESCRIPTION" 2>&1)
LOG_STATUS=$?
set -e

COMMIT_OUTPUT=$(daisy commit --home "Completed: $DESCRIPTION" 2>&1) || true
COMMIT_HASH=$(git -C "$DAISY_ROOT" rev-parse --short HEAD)

# --- Output contract: exit code is authoritative, one summary line on success ---
if [ "$TODAY_UPDATED" -eq 0 ]; then
    echo "⚠️  Task not found in today.md (may not be in active tasks)" >&2
fi

if [ "$LOG_STATUS" -ne 0 ]; then
    echo "Error: journal entry failed — todo.txt and today.md are committed ($COMMIT_HASH), but nothing was recorded in today.md's #### Log section" >&2
    echo "$LOG_OUTPUT" >&2
    echo "$COMMIT_OUTPUT" >&2
    exit 1
fi

echo "✅ Done: $DESCRIPTION (commit $COMMIT_HASH)"
exit 0
