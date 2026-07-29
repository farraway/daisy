#!/usr/bin/env bash
# Invocation: run as `daisy tasks` — do not execute this file directly.
# Search todo.txt/done.txt. Read-only: never writes or commits.
# Usage: tasks.sh [--all|--done|--todo] [pattern]
#
# --todo (default) searches todo.txt, which holds active tasks *and* any
# completed `x` lines `new-week` hasn't yet rotated out — the flag names the
# file, not "incomplete tasks". --done searches done.txt. --all searches
# both. With no pattern, every line of the selected file(s) is listed.

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

SCOPE="--todo"
case "$1" in
    --all|--done|--todo)
        SCOPE="$1"
        shift
        ;;
esac

PATTERN="$1"

FILES=()
case "$SCOPE" in
    --todo) FILES=("$DAISY_HOME/tasks/todo.txt") ;;
    --done) FILES=("$DAISY_HOME/tasks/done.txt") ;;
    --all)  FILES=("$DAISY_HOME/tasks/todo.txt" "$DAISY_HOME/tasks/done.txt") ;;
esac

EXISTING_FILES=()
for f in "${FILES[@]}"; do
    [ -f "$f" ] && EXISTING_FILES+=("$f")
done

if [ ${#EXISTING_FILES[@]} -eq 0 ]; then
    exit 1
fi

if [ -n "$PATTERN" ]; then
    grep -h -F -i -e "$PATTERN" -- "${EXISTING_FILES[@]}"
else
    cat -- "${EXISTING_FILES[@]}"
fi
