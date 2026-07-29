#!/usr/bin/env bash
# Invocation: run as `daisy plan-pickup` — do not execute this file directly.
# Promote a local, unregistered spec (`{slug}_PLAN.md`, from `plan-new.sh
# --spec`) into a real Daisy-tracked plan: copy it into $DAISY_HOME/plans/
# using the standard timestamp+slug naming, symlink it as PLAN.md. Purely
# mechanical — never touches the original file. Whether to delete the
# now-duplicated original is an agent/user decision, made in conversation
# after a successful pickup (see prompts/plan.md), not this script's job:
# every other Daisy script is non-interactive, and a `read` prompt here would
# hang or silently no-op when run non-interactively by an agent.
# Usage: plan-pickup.sh <path-to-spec-file>

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

if [ "$1" = "--healthcheck" ]; then
    require_env || exit 1
    exit 0
fi

require_env || exit 1

SPEC_PATH="$1"

if [ -z "$SPEC_PATH" ]; then
    echo "Error: Path to a spec file required" >&2
    echo "Usage: plan-pickup.sh <path-to-spec-file>" >&2
    exit 1
fi

if [ ! -f "$SPEC_PATH" ]; then
    echo "Error: File not found: $SPEC_PATH" >&2
    exit 1
fi

if [ -z "$DAISY_WORKSPACE" ]; then
    echo "Error: Could not determine workspace root" >&2
    exit 1
fi

if [ -e "$DAISY_WORKSPACE/PLAN.md" ]; then
    echo "Error: PLAN.md already exists in this workspace" >&2
    echo "Archive it first: /daisy archive PLAN.md" >&2
    exit 1
fi

# Description comes from the file's own H1 header, written by plan-new.sh's
# template population as either "# {Description}" or "# {Description} — +{project}"
HEADER=$(grep -m1 '^# ' "$SPEC_PATH" | sed 's/^# //')
if [ -z "$HEADER" ]; then
    echo "Error: Could not find a '# {Description}' header in $SPEC_PATH" >&2
    exit 1
fi

PROJECT=""
DESCRIPTION="$HEADER"
if [[ "$HEADER" == *" — +"* ]]; then
    DESCRIPTION="${HEADER% — +*}"
    PROJECT="${HEADER##*— +}"
fi

SLUG=$(printf '%s' "$DESCRIPTION" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+-/-/g' | sed 's/^-//;s/-$//')
TIMESTAMP=$(date +%y%m%d%H%M%S)

if [ -n "$PROJECT" ]; then
    FILENAME="${TIMESTAMP}.${PROJECT}.${SLUG}.plan.md"
else
    FILENAME="${TIMESTAMP}.${SLUG}.plan.md"
fi

DEST="$DAISY_HOME/plans/$FILENAME"
cp "$SPEC_PATH" "$DEST"

(cd "$DAISY_WORKSPACE" && ln -s ".daisy/plans/$FILENAME" PLAN.md)

echo "✅ Picked up: $(basename "$SPEC_PATH") → $FILENAME"
echo "   Linked as: $DAISY_WORKSPACE/PLAN.md"
echo "   Original left in place at: $SPEC_PATH"

daisy commit --home "Pick up plan: $FILENAME"

exit 0
