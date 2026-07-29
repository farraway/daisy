#!/usr/bin/env bash
# Invocation: run as `daisy files` — do not execute this file directly.
# Report the home name, workspace root, and resolved real paths for every
# per-home file/directory — the single source of truth for "where does X
# actually live," independent of whether a given .daisy/ symlink exists.
# Usage: files.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_env || exit 1
    exit 0
fi

require_env || exit 1

echo "Daisy | home: $DAISY_HOME_NAME | workspace: $DAISY_WORKSPACE"
echo ""

echo "Journal:"
printf '  %-12s %s\n' "today.md" "$(readlink -f "$DAISY_HOME/journal/today.md")"
printf '  %-12s %s\n' "journal.md" "$(readlink -f "$DAISY_HOME/journal/journal.md")"
if [ -f "$DAISY_HOME/journal/journal-pre-rotation.md" ]; then
    printf '  %-12s %s\n' "pre-rotation" "$(readlink -f "$DAISY_HOME/journal/journal-pre-rotation.md")"
fi
for f in "$DAISY_HOME"/journal/journal-*-last14.md "$DAISY_HOME"/journal/journal-*-mtd.md "$DAISY_HOME"/journal/journal-*-ytd.md; do
    [ -f "$f" ] || continue
    printf '  %-12s %s\n' "$(basename "$f")" "$(readlink -f "$f")"
done
CLOSED=$(find "$DAISY_HOME/journal" -maxdepth 1 -regextype posix-extended -regex '.*/journal-[0-9]{4}([0-9]{2})?\.md' 2>/dev/null | sort)
if [ -n "$CLOSED" ]; then
    while IFS= read -r f; do
        printf '  %-12s %s\n' "$(basename "$f")" "$(readlink -f "$f")"
    done <<< "$CLOSED"
fi
echo ""

echo "Tasks:"
printf '  %-12s %s\n' "todo.txt" "$(readlink -f "$DAISY_HOME/tasks/todo.txt")"
printf '  %-12s %s\n' "done.txt" "$(readlink -f "$DAISY_HOME/tasks/done.txt")"
printf '  %-12s %s\n' "alias.txt" "$(readlink -f "$DAISY_HOME/tasks/alias.txt")"
echo ""

echo "Projects:"
printf '  %-12s %s\n' "active" "$(readlink -f "$DAISY_HOME/projects")"
printf '  %-12s %s\n' "archived" "$(readlink -f "$DAISY_HOME/projects/_archive")"
echo ""

echo "Feedback:"
printf '  %-12s %s\n' "feedback.md" "$(readlink -f "$DAISY_HOME/feedback/feedback.md")"
if [ -f "$DAISY_HOME/feedback/archive.md" ]; then
    printf '  %-12s %s\n' "archive.md" "$(readlink -f "$DAISY_HOME/feedback/archive.md")"
fi
echo ""

echo "Plans:"
printf '  %-12s %s\n' "plans/" "$(readlink -f "$DAISY_HOME/plans")"
printf '  %-12s %s\n' "archive/" "$(readlink -f "$DAISY_HOME/plans/archive")"
if [ -n "$DAISY_WORKSPACE" ] && [ -L "$DAISY_WORKSPACE/PLAN.md" ]; then
    printf '  %-12s %s\n' "current plan" "$(readlink -f "$DAISY_WORKSPACE/PLAN.md")"
fi
echo ""

echo "Prompts:"
printf '  %-12s %s\n' "home/" "$(readlink -f "$DAISY_HOME/prompts")"
echo ""

printf '%-14s %s\n' "AGENTS.md:" "$(readlink -f "$DAISY_HOME/AGENTS.md")"
