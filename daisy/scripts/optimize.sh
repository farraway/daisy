#!/usr/bin/env bash
# Run the prompt learning optimization loop.
# Reads collected feedback, generates an improved ## Rules section via LLM,
# shows a diff, and applies the change on approval.
#
# Usage:
#   daisy optimize [--workflow <name>] [--dry-run]
#
# Requires: ANTHROPIC_API_KEY, jq, curl
#
# Called via `daisy optimize` (see $DAISY_ROOT/daisy.sh).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_root || exit 1
    require_env || exit 1
    exit 0
fi

# --- Parse arguments ---

WORKFLOW=""
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --workflow|-w)
            WORKFLOW="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            echo "Unknown option: $1" >&2
            echo "Usage: daisy optimize [--workflow <name>] [--dry-run]" >&2
            exit 1
            ;;
    esac
done

# --- Check dependencies ---

require_root || exit 1
require_env || exit 1

if ! command -v jq >/dev/null 2>&1; then
    echo "Error: jq is required for prompt optimization. Install with: brew install jq" >&2
    exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
    echo "Error: curl is required for prompt optimization" >&2
    exit 1
fi
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "Error: ANTHROPIC_API_KEY is not set" >&2
    echo "  Add to ~/.zshenv: export ANTHROPIC_API_KEY='your-key-here'" >&2
    exit 1
fi

MODEL="${DAISY_OPTIMIZE_MODEL:-claude-sonnet-4-6}"
FEEDBACK_FILE="$DAISY_HOME/feedback/feedback.md"

if [ ! -f "$FEEDBACK_FILE" ]; then
    echo "Error: No feedback file found at $FEEDBACK_FILE" >&2
    echo "  Run: daisy feedback --workflow <name> \"<description>\"" >&2
    exit 1
fi

# --- Parse feedback entries from feedback.md ---
# Entries start with "## YYYY-MM-DD HHMM"

mapfile -t ALL_ENTRIES < <(awk '
    /^## [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{4}/ {
        if (entry != "") print entry
        entry = $0
        next
    }
    entry != "" { entry = entry "\n" $0 }
    END { if (entry != "") print entry }
' "$FEEDBACK_FILE")

if [ ${#ALL_ENTRIES[@]} -eq 0 ]; then
    echo "No feedback entries found in $FEEDBACK_FILE"
    echo "  Add entries with: daisy feedback [--workflow <name>] \"<description>\""
    exit 0
fi

# Filter by workflow if specified
if [ -n "$WORKFLOW" ]; then
    FILTERED_ENTRIES=()
    for entry in "${ALL_ENTRIES[@]}"; do
        if echo "$entry" | grep -q "^\*\*Workflow:\*\* $WORKFLOW" || ! echo "$entry" | grep -q "^\*\*Workflow:\*\*"; then
            FILTERED_ENTRIES+=("$entry")
        fi
    done
    ENTRIES=("${FILTERED_ENTRIES[@]}")
    echo "Found ${#ENTRIES[@]} feedback entries (workflow filter: $WORKFLOW)"
else
    ENTRIES=("${ALL_ENTRIES[@]}")
    echo "Found ${#ENTRIES[@]} total feedback entries"
fi

TOTAL=${#ENTRIES[@]}

if [ "$TOTAL" -lt 2 ]; then
    echo "Need at least 2 feedback entries to optimize (have $TOTAL)."
    echo "  Add more with: daisy feedback [--workflow <name>] \"<description>\""
    exit 0
fi

# --- 80/20 train/test split ---

TRAIN_COUNT=$(( TOTAL * 80 / 100 ))
[ "$TRAIN_COUNT" -lt 1 ] && TRAIN_COUNT=1
TEST_COUNT=$(( TOTAL - TRAIN_COUNT ))

TRAIN_ENTRIES=("${ENTRIES[@]:0:$TRAIN_COUNT}")
TEST_ENTRIES=("${ENTRIES[@]:$TRAIN_COUNT}")

echo "  Train: $TRAIN_COUNT entries, Test: $TEST_COUNT entries"
echo ""

# --- Map workflow to prompt file ---

resolve_prompt_file() {
    local wf="$1"
    # Check home-specific prompt first, then shared
    local home_prompt="$DAISY_HOME/prompts/${wf}.md"
    [ -f "$home_prompt" ] && echo "$home_prompt" && return

    case "$wf" in
        daisy|new-day|new-week|done|log|add|priority|cancel|status|sync|project|home|"")
            echo "$DAISY_ROOT/prompts/daisy.md"
            ;;
        retrospective)
            echo "$DAISY_ROOT/prompts/retrospective.md"
            ;;
        plan)
            echo "$DAISY_ROOT/prompts/plan.md"
            ;;
        github)
            echo "$DAISY_ROOT/prompts/github.md"
            ;;
        *)
            # Try shared prompts directory
            local shared="$DAISY_ROOT/prompts/${wf}.md"
            if [ -f "$shared" ]; then
                echo "$shared"
            else
                echo "$DAISY_ROOT/prompts/daisy.md"
                echo "  ⚠ Unknown workflow '$wf', defaulting to daisy.md" >&2
            fi
            ;;
    esac
}

PROMPT_FILE=$(resolve_prompt_file "$WORKFLOW")

if [ ! -f "$PROMPT_FILE" ]; then
    echo "Error: Prompt file not found: $PROMPT_FILE" >&2
    exit 1
fi

echo "Optimizing: $PROMPT_FILE"
echo ""

# --- Extract current ## Rules section ---

CURRENT_RULES=$(awk '
    /^## Rules/ { found=1; print; next }
    found && /^## / { exit }
    found { print }
' "$PROMPT_FILE")

if [ -z "$CURRENT_RULES" ]; then
    echo "Warning: No ## Rules section found in $PROMPT_FILE"
    echo "  The optimizer will create one from the feedback."
    CURRENT_RULES="## Rules

(none yet)"
fi

BEFORE_LINES=$(echo "$CURRENT_RULES" | wc -l | tr -d ' ')
echo "Current ## Rules section: $BEFORE_LINES lines"

# --- Build train set text ---

TRAIN_TEXT=""
for i in "${!TRAIN_ENTRIES[@]}"; do
    TRAIN_TEXT+="### Failure $((i+1))"$'\n'
    TRAIN_TEXT+="${TRAIN_ENTRIES[$i]}"$'\n\n'
done

# --- Call LLM to generate improved Rules section ---

echo "Calling $MODEL to generate improved rules..."
echo ""

SYSTEM_PROMPT="You are a prompt engineer specializing in improving AI agent system prompts. You will be given a current prompt's ## Rules section and a list of agent failures. Your job is to produce an improved ## Rules section that prevents these failures while keeping rules general, concise, and non-redundant. Return ONLY the improved ## Rules section — nothing else. Start with '## Rules' on its own line, followed by the numbered rules."

USER_CONTENT="Here is the current ## Rules section from the Daisy productivity agent prompt:

$CURRENT_RULES

Here are cases where the agent failed or behaved incorrectly (training set):

$TRAIN_TEXT

Please return an improved ## Rules section that:
1. Prevents the failures described above
2. Keeps existing rules that are still valid
3. Removes rules that are redundant with each other
4. Expresses rules concisely in imperative form (numbered list)
5. Generalizes from specific failures to broadly applicable rules

Return ONLY the ## Rules section (starting with '## Rules'). Do not include any explanation or other text."

# Build JSON payload using jq for safe escaping
PAYLOAD=$(jq -n \
    --arg model "$MODEL" \
    --arg system "$SYSTEM_PROMPT" \
    --arg user_content "$USER_CONTENT" \
    '{
        model: $model,
        max_tokens: 2048,
        system: $system,
        messages: [{ role: "user", content: $user_content }]
    }')

RESPONSE=$(curl -s \
    "https://api.anthropic.com/v1/messages" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "content-type: application/json" \
    -d "$PAYLOAD")

# Check for API errors
if echo "$RESPONSE" | jq -e '.error' >/dev/null 2>&1; then
    echo "Error: API call failed" >&2
    echo "$RESPONSE" | jq -r '.error.message' >&2
    exit 1
fi

NEW_RULES=$(echo "$RESPONSE" | jq -r '.content[0].text')

if [ -z "$NEW_RULES" ] || [ "$NEW_RULES" = "null" ]; then
    echo "Error: Empty response from LLM" >&2
    echo "$RESPONSE" >&2
    exit 1
fi

AFTER_LINES=$(echo "$NEW_RULES" | wc -l | tr -d ' ')

# --- Show results ---

echo "=== Proposed Changes ==="
echo ""
echo "Lines: $BEFORE_LINES → $AFTER_LINES"
echo ""
echo "--- diff ---"
diff <(echo "$CURRENT_RULES") <(echo "$NEW_RULES") || true
echo "--- end diff ---"
echo ""

# --- Dry run: exit here ---

if [ "$DRY_RUN" = true ]; then
    echo "(dry-run mode — no changes written)"
    exit 0
fi

# --- Approval prompt ---

read -rp "Apply these changes? [y/N/e(edit)] " CHOICE
case "$CHOICE" in
    [yY])
        : # proceed
        ;;
    [eE])
        TMPFILE=$(mktemp /tmp/daisy-rules-XXXXXX.md)
        echo "$NEW_RULES" > "$TMPFILE"
        "${EDITOR:-vi}" "$TMPFILE"
        NEW_RULES=$(cat "$TMPFILE")
        rm -f "$TMPFILE"
        ;;
    *)
        echo "Cancelled — no changes made."
        exit 0
        ;;
esac

# --- Replace ## Rules section in prompt file ---

# Write updated file: everything before ## Rules, then new rules, then everything after
awk -v new_rules="$NEW_RULES" '
    /^## Rules/ {
        print new_rules
        skip = 1
        next
    }
    skip && /^## / { skip = 0 }
    !skip { print }
' "$PROMPT_FILE" > "${PROMPT_FILE}.tmp"

mv "${PROMPT_FILE}.tmp" "$PROMPT_FILE"

echo "  ✓ Updated $PROMPT_FILE"

# --- Rebuild AGENTS.md ---

echo "  Rebuilding AGENTS.md..."
"$DAISY_ROOT/daisy/scripts/build-prompt.sh" "$DAISY_HOME_NAME" 2>/dev/null || \
    "$DAISY_ROOT/daisy/scripts/build-prompt.sh" --output "$DAISY_HOME/AGENTS.md" "$DAISY_HOME_NAME"
echo "  ✓ AGENTS.md rebuilt"

# --- Record optimization event ---

TIMESTAMP=$(date '+%Y-%m-%d %H%M')
OPT_LOG="$DAISY_HOME/feedback/.last-optimized"
echo "$TIMESTAMP ${WORKFLOW:-all}" > "$OPT_LOG"

echo ""
echo "Done. Run 'daisy feedback' to continue collecting failures for the next cycle."
