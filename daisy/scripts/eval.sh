#!/usr/bin/env bash
# Display and record Daisy eval cases.
# Usage:
#   daisy eval                              # list available eval cases
#   daisy eval <case>                       # display criteria for a named case
#   daisy eval --record <case> pass|fail    # record result (prompts for notes)
#   daisy eval --record <case> pass|fail "notes"
#
# Called via `daisy eval` (see $DAISY_ROOT/daisy.sh).

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

EVALS_DIR="$DAISY_ROOT/prompts/evals"

if [ ! -d "$EVALS_DIR" ]; then
    echo "Error: No evals directory found at $EVALS_DIR" >&2
    exit 1
fi

# --- Record mode ---

if [ "$1" = "--record" ]; then
    shift
    CASE_NAME="$1"
    RESULT="$2"
    NOTES="${3:-}"
    shift 2 2>/dev/null || true

    if [ -z "$CASE_NAME" ]; then
        echo "Usage: daisy eval --record <case> pass|fail [\"notes\"]" >&2
        exit 1
    fi

    CASE_FILE="$EVALS_DIR/${CASE_NAME}.md"
    if [ ! -f "$CASE_FILE" ]; then
        echo "Error: Eval case not found: $CASE_NAME" >&2
        echo "  Run 'daisy eval' to list available cases." >&2
        exit 1
    fi

    case "$RESULT" in
        pass|fail) ;;
        *)
            echo "Error: result must be 'pass' or 'fail' (got: $RESULT)" >&2
            exit 1
            ;;
    esac

    # Prompt for notes if not provided
    if [ -z "$NOTES" ] && [ -t 0 ]; then
        read -rp "Notes (optional): " NOTES
    fi

    RESULTS_DIR="$DAISY_HOME/feedback/eval"
    RESULTS_FILE="$RESULTS_DIR/results.md"

    if [ ! -d "$RESULTS_DIR" ]; then
        mkdir -p "$RESULTS_DIR"
    fi

    if [ ! -f "$RESULTS_FILE" ]; then
        cat > "$RESULTS_FILE" <<'HEADER'
# Eval Results

Records of manual eval runs. Used to track eval accuracy and improve eval criteria.
Run `daisy eval <case>` to view criteria; `daisy eval --record <case> pass|fail` to add an entry.

---
HEADER
    fi

    TIMESTAMP=$(date '+%Y-%m-%d %H%M')

    {
        echo ""
        echo "## $TIMESTAMP"
        echo "**Case:** $CASE_NAME"
        echo "**Result:** $RESULT"
        if [ -n "$NOTES" ]; then
            echo ""
            echo "$NOTES"
        fi
    } >> "$RESULTS_FILE"

    echo "  ✓ Recorded $RESULT for eval '$CASE_NAME' ($(date '+%Y-%m-%d %H:%M'))"
    echo "  File: $RESULTS_FILE"
    exit 0
fi

# --- Show a named case ---

if [ -n "$1" ]; then
    CASE_NAME="$1"
    CASE_FILE="$EVALS_DIR/${CASE_NAME}.md"

    if [ ! -f "$CASE_FILE" ]; then
        echo "Error: Eval case not found: $CASE_NAME" >&2
        echo "  Run 'daisy eval' to list available cases." >&2
        exit 1
    fi

    cat "$CASE_FILE"
    echo ""
    echo "  Record result: daisy eval --record $CASE_NAME pass|fail \"notes\""
    exit 0
fi

# --- List all cases ---

CASES=()
while IFS= read -r -d '' f; do
    CASES+=("$f")
done < <(find "$EVALS_DIR" -maxdepth 1 -name '*.md' -print0 | sort -z)

if [ ${#CASES[@]} -eq 0 ]; then
    echo "No eval cases found in $EVALS_DIR"
    exit 0
fi

echo "Available eval cases:"
echo ""
for f in "${CASES[@]}"; do
    name=$(basename "$f" .md)
    title=$(grep '^# ' "$f" | head -1 | sed 's/^# //')
    printf "  %-20s  %s\n" "$name" "$title"
done
echo ""
echo "  Usage: daisy eval <case>                           # display criteria"
echo "         daisy eval --record <case> pass|fail        # record a result"
