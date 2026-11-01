#!/usr/bin/env bash
# Invocation: run as `daisy feedback` — do not execute this file directly.
# Record a Daisy failure for prompt optimization.
# Usage:
#   daisy feedback [--workflow <name>] "<description>"
#   echo "description" | daisy feedback [--workflow <name>]
#
# Called via `daisy feedback` (see $DAISY_ROOT/daisy.sh).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_root || exit 1
    require_env || exit 1
    exit 0
fi

require_root || exit 1
require_env || exit 1

# Parse --workflow flag
WORKFLOW=""
while [ "$1" = "--workflow" ] || [ "$1" = "-w" ]; do
    shift
    WORKFLOW="$1"
    shift
done

# A tag naming no file is refused here rather than guessed at later. Nothing
# used to check it, so `daisy-write`, `daisy-scripts` and the literal
# `new-day, new-week` all reached the archive and trained daisy.md by
# accident, through optimize's "Unknown workflow" fallback.
if [ -n "$WORKFLOW" ] && ! resolve_workflow_target "$WORKFLOW" >/dev/null; then
    echo "Error: workflow '$WORKFLOW' names no prompt or skill" >&2
    echo "  Qualify the target, e.g. --workflow prompts/daisy or --workflow skills/git." >&2
    echo "" >&2
    echo "  Available:" >&2
    list_workflow_targets | sed 's/^/    /' >&2
    exit 1
fi

# Get message from args or stdin
if [ $# -gt 0 ]; then
    MESSAGE="$*"
elif [ ! -t 0 ]; then
    MESSAGE=$(cat)
else
    echo "Usage: daisy feedback [--workflow <name>] \"<description>\"" >&2
    echo "  or:  echo \"<description>\" | daisy feedback [--workflow <name>]" >&2
    echo "" >&2
    # Generated, not hardcoded: the list this replaced offered `jira` (no
    # prompt, no skill, no arm) and omitted every skill.
    echo "Workflows:" >&2
    list_workflow_targets | sed 's/^/  /' >&2
    exit 1
fi

if [ -z "$MESSAGE" ]; then
    echo "Error: feedback message cannot be empty" >&2
    exit 1
fi

FEEDBACK_FILE="$DAISY_HOME/feedback/feedback.md"

if [ ! -f "$FEEDBACK_FILE" ]; then
    echo "Error: feedback file not found at $FEEDBACK_FILE" >&2
    echo "  Run: mkdir -p \"$DAISY_HOME/feedback\" && touch \"$FEEDBACK_FILE\"" >&2
    exit 1
fi

TIMESTAMP=$(date '+%Y-%m-%d %H%M')

{
    echo ""
    echo "## $TIMESTAMP"
    if [ -n "$WORKFLOW" ]; then
        echo "**Workflow:** $WORKFLOW"
    fi
    echo ""
    echo "$MESSAGE"
} >> "$FEEDBACK_FILE"

echo "  ✓ Feedback recorded ($([ -n "$WORKFLOW" ] && echo "workflow: $WORKFLOW, " || echo "")$(date '+%Y-%m-%d %H:%M'))"
echo "  File: $FEEDBACK_FILE"
