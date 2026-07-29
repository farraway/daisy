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

# Get message from args or stdin
if [ $# -gt 0 ]; then
    MESSAGE="$*"
elif [ ! -t 0 ]; then
    MESSAGE=$(cat)
else
    echo "Usage: daisy feedback [--workflow <name>] \"<description>\"" >&2
    echo "  or:  echo \"<description>\" | daisy feedback [--workflow <name>]" >&2
    echo "" >&2
    echo "Workflows: daisy, new-day, new-week, done, log, plan, retrospective, project, github, jira" >&2
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
