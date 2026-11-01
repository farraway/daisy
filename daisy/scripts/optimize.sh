#!/usr/bin/env bash
# Invocation: run as `daisy optimize` — do not execute this file directly.
# Run the prompt learning optimization loop.
# Reads collected feedback, generates an improved ## Rules section via LLM,
# shows a diff, and applies the change on approval.
#
# With no --workflow, entries are grouped by their **Workflow:** header and
# each group runs its own cycle against its own prompt file. With --workflow,
# only entries declaring that workflow are in scope.
#
# Usage:
#   daisy optimize [--workflow <name>] [--dry-run] [--force]
#   daisy optimize --list   # print entries added since last optimize run
#
# --dry-run is for non-interactive use (tests, agents that cannot answer the
# approval prompt). It is not a preview of what an applying run will write:
# each invocation makes its own LLM call, so the proposal it shows is one
# sample and the applying run generates another. To review what will actually
# be written, run without --dry-run — the approval prompt shows the same diff,
# defaults to no, and offers e(edit) on the exact text about to be written.
#
# Requires: jq, curl, ANTHROPIC_API_KEY — checked when a cycle first needs
# the LLM, so a run whose groups are all below the floor needs none of them.
#
# Called via `daisy optimize` (see $DAISY_ROOT/daisy.sh).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_root || exit 1
    require_env || exit 1
    if [ ! -d "$DAISY_HOME/feedback" ]; then
        echo "Error: feedback directory not found (needed for feedback.md and archive.md)" >&2
        exit 1
    fi
    exit 0
fi

# --- Parse arguments ---

WORKFLOW=""
DRY_RUN=false
LIST_MODE=false
FORCE=false

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
        --force)
            FORCE=true
            shift
            ;;
        --list)
            LIST_MODE=true
            shift
            ;;
        *)
            echo "Unknown option: $1" >&2
            echo "Usage: daisy optimize [--workflow <name>] [--dry-run] [--force] [--list]" >&2
            exit 1
            ;;
    esac
done

# --- Check dependencies ---

require_root || exit 1
require_env || exit 1

# --- List mode: print feedback entries added since last optimize run ---

if [ "$LIST_MODE" = true ]; then
    FEEDBACK_FILE="$DAISY_HOME/feedback/feedback.md"

    [ -f "$FEEDBACK_FILE" ] || exit 0

    # Everything in feedback.md is unprocessed by construction — a real
    # optimize run archives entries it used (see below), so there's no
    # cutoff left to compare against; just show what's currently pending.
    awk '
        /^## [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{4}/ {
            if (entry != "") { print entry; print "" }
            entry = $0
            next
        }
        entry != "" { entry = entry "\n" $0 }
        END { if (entry != "") print entry }
    ' "$FEEDBACK_FILE"

    exit 0
fi

# jq, curl and the API key are needed only once a group actually reaches the
# LLM. Checking them up front made "every group is below the floor" — a
# verdict costing no API call — indistinguishable from a misconfigured
# machine, and made the no-call paths untestable without a live key.
LLM_DEPS_CHECKED=false
ensure_llm_deps() {
    [ "$LLM_DEPS_CHECKED" = true ] && return 0
    if ! command -v jq >/dev/null 2>&1; then
        echo "Error: jq is required for prompt optimization. Install with: brew install jq" >&2
        exit 1
    fi
    # A stubbed run makes no request, so it needs neither curl nor a key.
    # jq it still needs — the payload is built and the response parsed on
    # both sides of the seam. Demanding all three would leave the write path
    # exactly as untestable as it was.
    if [ -z "${DAISY_OPTIMIZE_RESPONSE_FILE:-}" ]; then
        if ! command -v curl >/dev/null 2>&1; then
            echo "Error: curl is required for prompt optimization" >&2
            exit 1
        fi
        if [ -z "$ANTHROPIC_API_KEY" ]; then
            echo "Error: ANTHROPIC_API_KEY is not set" >&2
            echo "  Add to ~/.zshenv: export ANTHROPIC_API_KEY='your-key-here'" >&2
            exit 1
        fi
    fi
    LLM_DEPS_CHECKED=true
}

# Echoes the raw API response for a rules-rewrite request. This is the seam
# the test suite writes through: DAISY_OPTIMIZE_RESPONSE_FILE supplies a
# canned response instead of calling the API, which is the only way a case
# can reach the write path below — the path every defect in this loop has
# come from. No Daisy code path ever sets it.
#
# It returns the *raw* response, not the extracted text, so the two failure
# branches on it (an `.error` payload, an empty completion) stay on the
# testable side of the seam.
request_rules_rewrite() {
    if [ -n "${DAISY_OPTIMIZE_RESPONSE_FILE:-}" ]; then
        if [ ! -f "$DAISY_OPTIMIZE_RESPONSE_FILE" ]; then
            echo "Error: DAISY_OPTIMIZE_RESPONSE_FILE is set but names no file: $DAISY_OPTIMIZE_RESPONSE_FILE" >&2
            exit 1
        fi
        cat "$DAISY_OPTIMIZE_RESPONSE_FILE"
        return
    fi
    curl -s \
        "https://api.anthropic.com/v1/messages" \
        -H "x-api-key: $ANTHROPIC_API_KEY" \
        -H "anthropic-version: 2023-06-01" \
        -H "content-type: application/json" \
        -d "$1"
}

MODEL="${DAISY_OPTIMIZE_MODEL:-claude-sonnet-4-6}"
FEEDBACK_FILE="$DAISY_HOME/feedback/feedback.md"
# A group below this floor is skipped rather than run: a rules section should
# not be rewritten from a single data point. This is the only rationing left
# in the loop — every entry at or above the floor is used and archived.
# It is a default, not a rule: --force says "I have considered it, run
# anyway", which is what three single entries hand-applied on 2026-08-17
# needed and had no way to express.
MIN_ENTRIES=2

if [ ! -f "$FEEDBACK_FILE" ]; then
    echo "Error: No feedback file found at $FEEDBACK_FILE" >&2
    echo "  Run: daisy feedback --workflow <name> \"<description>\"" >&2
    exit 1
fi

# --- Parse feedback entries from feedback.md ---
# Entries start with "## YYYY-MM-DD HHMM". Records are NUL-delimited, not
# newline-delimited: `mapfile` (no -d) splits on every newline byte in its
# input, including the ones *inside* a multi-line entry — a plain
# `print entry` would silently fragment each entry into one array element
# per line, not one element per entry. NUL never appears in the source text,
# so it's a safe record separator here.
mapfile -d '' -t ALL_ENTRIES < <(awk '
    /^## [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{4}/ {
        if (entry != "") printf "%s%c", entry, 0
        entry = $0
        next
    }
    entry != "" { entry = entry "\n" $0 }
    END { if (entry != "") printf "%s%c", entry, 0 }
' "$FEEDBACK_FILE")

if [ ${#ALL_ENTRIES[@]} -eq 0 ]; then
    echo "No feedback entries found in $FEEDBACK_FILE"
    echo "  Add entries with: daisy feedback [--workflow <name>] \"<description>\""
    exit 0
fi

# --- Group entries by declared workflow ---
# An entry with no **Workflow:** header belongs to `daisy`, the prompt that
# owns the unlabelled workflows. This is the only default: an *unspecified*
# --workflow means "every group", never "the daisy group".

entry_workflow() {
    local wf
    wf=$(printf '%s\n' "$1" | awk '
        /^\*\*Workflow:\*\*/ {
            sub(/^\*\*Workflow:\*\*[[:space:]]*/, "")
            gsub(/[[:space:]]+$/, "")
            print
            exit
        }')
    echo "${wf:-daisy}"
}

entry_count_phrase() {
    if [ "$1" -eq 1 ]; then echo "1 entry"; else echo "$1 entries"; fi
}

declare -A GROUP_IDX=()
GROUP_ORDER=()

for i in "${!ALL_ENTRIES[@]}"; do
    wf=$(entry_workflow "${ALL_ENTRIES[$i]}")
    # --workflow scopes to entries *declaring* that workflow. It used to also
    # sweep in unheadered entries, which routed the same entry differently
    # depending on whether the run was scoped.
    if [ -n "$WORKFLOW" ] && [ "$wf" != "$WORKFLOW" ]; then
        continue
    fi
    if [ -z "${GROUP_IDX[$wf]+set}" ]; then
        GROUP_ORDER+=("$wf")
        GROUP_IDX[$wf]="$i"
    else
        GROUP_IDX[$wf]+=" $i"
    fi
done

if [ ${#GROUP_ORDER[@]} -eq 0 ]; then
    echo "No feedback entries for workflow '$WORKFLOW' in $FEEDBACK_FILE"
    echo "  Add one with: daisy feedback --workflow $WORKFLOW \"<description>\""
    exit 0
fi

IN_SCOPE=0
for wf in "${GROUP_ORDER[@]}"; do
    # shellcheck disable=SC2086
    set -- ${GROUP_IDX[$wf]}
    IN_SCOPE=$(( IN_SCOPE + $# ))
done

if [ -n "$WORKFLOW" ]; then
    echo "Found $(entry_count_phrase $IN_SCOPE) for workflow '$WORKFLOW'"
else
    echo "Found $(entry_count_phrase $IN_SCOPE) across ${#GROUP_ORDER[@]} workflow(s)"
fi

# --- Map workflow to prompt file ---
# The mapping itself is resolve_workflow_target in common.sh, shared with
# `daisy feedback`. This wrapper adds only optimize's fallback: an
# unresolvable tag still gets a cycle against daisy.md rather than aborting
# the run, because the entry exists and its guidance should not be dropped.

resolve_prompt_file() {
    local target
    if target=$(resolve_workflow_target "$1"); then
        echo "$target"
        return
    fi
    # Resolution lives in common.sh so `daisy feedback` can refuse a tag
    # naming no file *before* it is written. Reaching here means an entry
    # predating that check, or one hand-written into feedback.md.
    echo "$DAISY_ROOT/prompts/daisy.md"
    echo "  ⚠ Unknown workflow '$1', defaulting to daisy.md" >&2
    echo "     Qualify it as prompts/<name> or skills/<name>; see: daisy feedback" >&2
}

# --- One optimization cycle for one workflow group ---
# Usage: run_cycle <workflow> <entry-index>...
# Appends the indices it trained on to TRAINED_IDX, and records the file each
# one fed in TRAINED_TARGET; sets APPLIED_ANY when it writes a prompt file.
#
# TRAINED_TARGET exists because the archive pass runs once, at the top level,
# after every cycle — by then "which target did this entry contribute to" is
# a per-cycle fact that only run_cycle ever knew.

TRAINED_IDX=()
declare -A TRAINED_TARGET=()
PROMPT_APPLIED=false
SKILL_APPLIED=false

run_cycle() {
    local wf="$1"; shift
    local idx=("$@")
    local total=${#idx[@]}

    echo ""
    echo "[$wf] $(entry_count_phrase $total)"

    # Resolve before the floor check: knowing where a group *would* write is
    # exactly what makes a skipped group's report useful, and it is how a
    # vendored target gets refused rather than silently queued.
    local prompt_file
    prompt_file=$(resolve_prompt_file "$wf")
    if [ ! -f "$prompt_file" ]; then
        echo "Error: Prompt file not found: $prompt_file" >&2
        exit 1
    fi
    echo "  Target: $prompt_file"

    if [[ "$prompt_file" == */SKILL.md ]] && skill_is_vendored "$(dirname "$prompt_file")"; then
        echo "  Skipped — '$wf' is a vendored skill (carries LICENSE/NOTICE.md)."
        echo "  Rewriting Daisy's copy from local feedback would fork it from upstream."
        return 0
    fi

    if [ "$total" -lt "$MIN_ENTRIES" ]; then
        if [ "$FORCE" = false ]; then
            echo "  Skipped — below the ${MIN_ENTRIES}-entry floor."
            echo "  Add another with: daisy feedback --workflow $wf \"<description>\""
            echo "  Run anyway with:  daisy optimize --workflow $wf --force"
            return 0
        fi
        echo "  Below the ${MIN_ENTRIES}-entry floor — running anyway (--force)."
    fi

    # Every in-scope entry trains. An 80/20 split once withheld 20% of a
    # group for a holdout nothing scored against — test_count was computed,
    # printed and never read, and there was no test_idx. Because only
    # TRAINED_IDX is archived, the withheld entries stayed in feedback.md;
    # at N=2 that put the group below MIN_ENTRIES with nothing in prospect,
    # stranding them rather than deferring them.
    echo "  Entries: $total"

    local train_idx=("${idx[@]}")

    # --- Extract current ## Rules section ---
    local current_rules
    current_rules=$(extract_rules_section "$prompt_file")

    local has_anchor=true
    if [ -z "$current_rules" ]; then
        has_anchor=false
        echo "  No ## Rules section — the optimizer will create one."
        current_rules="## Rules

(none yet)"
    fi

    local before_lines
    before_lines=$(echo "$current_rules" | wc -l | tr -d ' ')

    # --- Build train set text ---
    local train_text="" n=0
    local i
    for i in "${train_idx[@]}"; do
        n=$((n + 1))
        train_text+="### Failure $n"$'\n'
        train_text+="${ALL_ENTRIES[$i]}"$'\n\n'
    done

    # --- Call LLM to generate improved Rules section ---
    ensure_llm_deps
    echo "  Calling $MODEL..."

    local system_prompt="You are a prompt engineer specializing in improving AI agent system prompts. You will be given a current prompt's ## Rules section and a list of agent failures. Your job is to produce an improved ## Rules section that prevents these failures while keeping rules general, concise, and non-redundant. Return ONLY the improved ## Rules section — nothing else. Start with '## Rules' on its own line, followed by the numbered rules."

    local user_content="Here is the current ## Rules section from the Daisy productivity agent prompt:

$current_rules

Here are cases where the agent failed or behaved incorrectly (training set):

$train_text

Please return an improved ## Rules section that:
1. Prevents the failures described above
2. Keeps existing rules that are still valid
3. Removes rules that are redundant with each other
4. Expresses rules concisely in imperative form (numbered list)
5. Generalizes from specific failures to broadly applicable rules

Return ONLY the ## Rules section (starting with '## Rules'). Do not include any explanation or other text."

    local payload
    payload=$(jq -n \
        --arg model "$MODEL" \
        --arg system "$system_prompt" \
        --arg user_content "$user_content" \
        '{
            model: $model,
            max_tokens: 2048,
            system: $system,
            messages: [{ role: "user", content: $user_content }]
        }')

    local response
    response=$(request_rules_rewrite "$payload")

    if echo "$response" | jq -e '.error' >/dev/null 2>&1; then
        echo "Error: API call failed for workflow '$wf'" >&2
        echo "$response" | jq -r '.error.message' >&2
        exit 1
    fi

    local new_rules
    new_rules=$(echo "$response" | jq -r '.content[0].text')

    if [ -z "$new_rules" ] || [ "$new_rules" = "null" ]; then
        echo "Error: Empty response from LLM for workflow '$wf'" >&2
        echo "$response" >&2
        exit 1
    fi

    local after_lines
    after_lines=$(echo "$new_rules" | wc -l | tr -d ' ')

    # --- Show results ---
    echo ""
    echo "  === Proposed changes for $wf ==="
    echo "  Lines: $before_lines → $after_lines"
    echo ""
    echo "--- diff ---"
    diff <(echo "$current_rules") <(echo "$new_rules") || true
    echo "--- end diff ---"
    echo ""

    if [ "$DRY_RUN" = true ]; then
        echo "  (dry-run — nothing written. This is one sample; an applying"
        echo "   run makes its own LLM call and will propose different text."
        echo "   To review what will actually be written, run without"
        echo "   --dry-run and use the approval prompt's e(edit).)"
        return 0
    fi

    # --- Approval prompt ---
    # `read` returns non-zero at EOF, and this script runs under `set -e` —
    # so running out of input used to exit from *inside* this function, after
    # a prompt file had been written and before the top-level archive pass.
    # That left the target rewritten with its entries still in feedback.md,
    # which is the one pair that must never come apart: the next run would
    # train again on guidance already folded in.
    local choice
    if ! read -rp "Apply these changes to $(basename "$prompt_file")? [y/N/e(edit)] " choice; then
        echo ""
        echo "  Cancelled — no answer on stdin (EOF); $wf left unchanged."
        return 0
    fi
    case "$choice" in
        [yY])
            : # proceed
            ;;
        [eE])
            local tmpfile
            # ${TMPDIR:-/tmp}, not a hardcoded /tmp: a template containing a
            # slash makes mktemp ignore TMPDIR, so the edit branch failed
            # outright anywhere /tmp is not writable.
            tmpfile=$(mktemp "${TMPDIR:-/tmp}/daisy-rules-XXXXXX.md")
            echo "$new_rules" > "$tmpfile"
            # EDITOR is a command line, not an executable path — "code --wait"
            # and "emacsclient -nw" are ordinary values. Quoting it made the
            # whole string one filename, so every multi-word editor failed
            # here. Unquoted so it splits, as every other consumer of EDITOR
            # does.
            # shellcheck disable=SC2086
            ${EDITOR:-vi} "$tmpfile"
            new_rules=$(cat "$tmpfile")
            rm -f "$tmpfile"
            ;;
        *)
            echo "  Cancelled — $wf left unchanged."
            return 0
            ;;
    esac

    # --- Replace (or insert) the ## Rules section ---
    if [ "$has_anchor" = true ]; then
        replace_rules_section "$prompt_file" "$new_rules" > "${prompt_file}.tmp"
    else
        # No anchor to replace. Insert after the leading frontmatter/heading
        # block — before the first "## " section, or at the end if there is
        # none. Without this the generated rules were silently discarded:
        # replace_rules_section only ever fires on an existing "## Rules"
        # line, so with no anchor there was nothing to write into.
        awk -v new_rules="$new_rules" '
            !inserted && /^## / { print new_rules; print ""; inserted = 1 }
            { print }
            END { if (!inserted) { print ""; print new_rules } }
        ' "$prompt_file" > "${prompt_file}.tmp"
    fi
    mv "${prompt_file}.tmp" "$prompt_file"

    echo "  ✓ Updated $prompt_file"
    if [[ "$prompt_file" == */SKILL.md ]]; then
        SKILL_APPLIED=true
    else
        PROMPT_APPLIED=true
    fi
    TRAINED_IDX+=("${train_idx[@]}")
    for i in "${train_idx[@]}"; do
        TRAINED_TARGET[$i]="$prompt_file"
    done
}

for wf in "${GROUP_ORDER[@]}"; do
    # shellcheck disable=SC2086
    run_cycle "$wf" ${GROUP_IDX[$wf]}
done

echo ""

# --- Rebuild AGENTS.md (once, after every group) ---

if [ "$PROMPT_APPLIED" = true ]; then
    echo "Rebuilding AGENTS.md..."
    "$DAISY_ROOT/daisy/scripts/build-prompt.sh" "$DAISY_HOME_NAME" 2>/dev/null || \
        "$DAISY_ROOT/daisy/scripts/build-prompt.sh" --output "$DAISY_HOME/AGENTS.md" "$DAISY_HOME_NAME"
    echo "  ✓ AGENTS.md rebuilt"
fi

# Skills are distributed by copy, so a rewritten source is not yet in effect
# anywhere. Say so rather than running the installers: they also touch
# symlinks and permission grants, which is more than optimize was asked to do.
if [ "$SKILL_APPLIED" = true ]; then
    echo "A skill source changed. The installed copies are still the old version — propagate with:"
    echo "  daisy install --update      # user-level skills (~/.claude/skills)"
    echo "  daisy init $DAISY_HOME_NAME  # this workspace's skills"
fi

# --- Archive processed feedback entries ---
# Entries actually used to train a rewrite move out of feedback.md into
# feedback/archive.md — membership in TRAINED_IDX, not just "in scope for
# this run". Since the 80/20 split was removed every entry in a group that
# runs is trained on, so TRAINED_IDX is now the whole group; the distinction
# still matters because groups that were skipped, cancelled, or dry-run
# contribute nothing here, and archiving an entry never fed to the LLM would
# silently lose guidance that was never incorporated.

if [ ${#TRAINED_IDX[@]} -gt 0 ]; then
    ARCHIVE_FILE="$DAISY_HOME/feedback/archive.md"
    if [ ! -f "$ARCHIVE_FILE" ]; then
        TEMPLATE_ARCHIVE="$DAISY_ROOT/daisy/templates/home/feedback/archive.md"
        if [ -f "$TEMPLATE_ARCHIVE" ]; then
            cp "$TEMPLATE_ARCHIVE" "$ARCHIVE_FILE"
        else
            printf '# Feedback Archive\n\nEntries already folded into a prompt'"'"'s ## Rules section by `daisy optimize`.\n\n---\n' > "$ARCHIVE_FILE"
        fi
    fi

    declare -A IS_TRAINED=()
    for i in "${TRAINED_IDX[@]}"; do
        IS_TRAINED[$i]=1
    done

    # Each entry is followed by the same **Addressed by:** line the
    # hand-applied entries carry, so the archive answers "where did this
    # go?" the same way however the entry was folded in. A home's own file
    # is named relative to the home (the archive's own repository); a shared
    # file keeps the `$DAISY_ROOT/` prefix the prompts themselves use. Both
    # forms survive the home or the root moving.
    provenance_path() {
        case "$1" in
            "$DAISY_HOME"/*) echo "${1#"$DAISY_HOME"/}" ;;
            "$DAISY_ROOT"/*) echo "\$DAISY_ROOT/${1#"$DAISY_ROOT"/}" ;;
            *) echo "$1" ;;
        esac
    }
    ARCHIVED_ON=$(date '+%Y-%m-%d')
    {
        for i in "${!ALL_ENTRIES[@]}"; do
            [ -n "${IS_TRAINED[$i]+set}" ] || continue
            # Every entry but the last carries the blank line that preceded
            # the next heading; command substitution strips it, so the
            # provenance line sits exactly one blank below its entry
            # whichever entry it is.
            entry=$(printf '%s' "${ALL_ENTRIES[$i]}")
            echo ""
            echo "$entry"
            echo ""
            echo "**Addressed by:** \`daisy optimize\` $ARCHIVED_ON — \`$(provenance_path "${TRAINED_TARGET[$i]}")\` ## Rules"
        done
    } >> "$ARCHIVE_FILE"

    # Rewrite feedback.md: keep its header (everything before the first
    # entry) plus only the entries not archived.
    # Both the header and each entry are emitted through command
    # substitution, which strips their trailing blank lines: the header ends
    # with the blank that preceded the first entry, and every entry but the
    # last carries the blank that preceded the next heading. Without that,
    # each `echo ""` below added a second one and every boundary in the file
    # gained a line.
    {
        printf '%s\n' "$(awk '/^## [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{4}/ { exit } { print }' "$FEEDBACK_FILE")"
        for i in "${!ALL_ENTRIES[@]}"; do
            [ -n "${IS_TRAINED[$i]+set}" ] && continue
            echo ""
            printf '%s\n' "$(printf '%s' "${ALL_ENTRIES[$i]}")"
        done
    } > "${FEEDBACK_FILE}.tmp"
    mv "${FEEDBACK_FILE}.tmp" "$FEEDBACK_FILE"

    COUNT=${#TRAINED_IDX[@]}
    NOUN="entries"; [ "$COUNT" -eq 1 ] && NOUN="entry"
    echo "  ✓ Archived $COUNT $NOUN to $ARCHIVE_FILE"

    # --- Record optimization event ---
    TIMESTAMP=$(date '+%Y-%m-%d %H%M')
    echo "$TIMESTAMP ${WORKFLOW:-all}" > "$DAISY_HOME/feedback/.last-optimized"
fi

echo ""
echo "Done. Run 'daisy feedback' to continue collecting failures for the next cycle."
