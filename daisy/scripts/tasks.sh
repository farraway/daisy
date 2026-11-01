#!/usr/bin/env bash
# Invocation: run as `daisy tasks` — do not execute this file directly.
# Search todo.txt/done.txt. Read-only: never writes or commits.
# Usage: tasks.sh [--all] [--todo|--done] [pattern]
#
# Two orthogonal axes:
#   scope — this workspace's projects by default; --all widens to every project
#   files — both by default; --todo or --done narrows to one
#
# They used to be conflated on --all ("search both files"), which meant an
# agent in a single-project workspace had no way to ask for its own tasks.
#
# --todo names the *file*, not "incomplete tasks": todo.txt holds active tasks
# and any completed `x` lines `new-week` has not yet rotated out. --done
# searches done.txt.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_env || exit 1
    if [ ! -d "$DAISY_HOME/tasks" ]; then
        echo "Error: tasks directory not found" >&2
        exit 1
    fi
    exit 0
fi

require_env || exit 1

SCOPE_ALL=false
FILE_FILTER=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --all)
            SCOPE_ALL=true
            shift
            ;;
        --todo|--done)
            FILE_FILTER="$1"
            shift
            ;;
        *)
            break
            ;;
    esac
done

PATTERN="$1"

FILES=()
case "$FILE_FILTER" in
    --todo) FILES=("$DAISY_HOME/tasks/todo.txt") ;;
    --done) FILES=("$DAISY_HOME/tasks/done.txt") ;;
    *)      FILES=("$DAISY_HOME/tasks/todo.txt" "$DAISY_HOME/tasks/done.txt") ;;
esac

EXISTING_FILES=()
for f in "${FILES[@]}"; do
    [ -f "$f" ] && EXISTING_FILES+=("$f")
done

if [ ${#EXISTING_FILES[@]} -eq 0 ]; then
    exit 1
fi

# --- Project scope ---
# Matches the +tag at a word boundary the tag syntax actually has: a trailing
# space or end of line. A plain \b would let +daisy match inside +daisy-dev.

SCOPE_RE=""
if [ "$SCOPE_ALL" != true ]; then
    resolve_projects
    if [ ${#DAISY_PROJECTS[@]} -eq 0 ]; then
        # Returning nothing would be a silent lie — the tasks exist, this
        # workspace just does not say which are its own.
        echo "⚠️  This workspace resolves to no project; listing every project's tasks." >&2
        echo "  Bind it by creating .daisy/project, or pass --all to ask for this deliberately." >&2
    else
        for p in "${DAISY_PROJECTS[@]}"; do
            SCOPE_RE+="${SCOPE_RE:+|}\\+${p}([[:space:]]|\$)"
        done
    fi
fi

filter_scope() {
    if [ -n "$SCOPE_RE" ]; then
        grep -E -i -- "$SCOPE_RE"
    else
        cat
    fi
}

filter_pattern() {
    if [ -n "$PATTERN" ]; then
        grep -F -i -e "$PATTERN"
    else
        cat
    fi
}

cat -- "${EXISTING_FILES[@]}" | filter_scope | filter_pattern
