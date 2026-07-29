#!/usr/bin/env bash
# Invocation: run as `daisy list` — do not execute this file directly.
# List active prompts (from include.txt, with a one-line trigger summary) and
# installed skills (name + description) for the current home and workspace —
# the on-demand discovery surface for what's available, without paying the
# context cost of loading everything up front.
# Usage: list.sh [--full]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

if [ "$1" = "--healthcheck" ]; then
    require_env || exit 1
    exit 0
fi

FULL=false
[ "$1" = "--full" ] && FULL=true

require_env || exit 1

echo "Prompts ($DAISY_HOME_NAME):"
INCLUDE_FILE="$DAISY_HOME/include.txt"
if [ -f "$INCLUDE_FILE" ]; then
    while IFS= read -r line || [ -n "$line" ]; do
        [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
        line=$(echo "$line" | xargs)
        [ -z "$line" ] && continue

        LAZY=false
        name="$line"
        if [[ "$line" =~ ^~ ]]; then
            LAZY=true
            name="${line#\~}"
        fi

        path="$DAISY_HOME/prompts/${name}.md"
        [ -f "$path" ] || path="$DAISY_ROOT/prompts/${name}.md"
        if [ ! -f "$path" ]; then
            printf '  %-20s (missing: %s)\n' "$name" "$path"
            continue
        fi

        if [ "$LAZY" = true ]; then
            trigger=$(awk '/^## Trigger/{f=1;next} f && /^#/{exit} f' "$path" \
                | grep -v '^Read the full' \
                | sed -e 's/^- //' -e '/^$/d' \
                | head -1)
            if [ "${#trigger}" -gt 100 ]; then
                trigger="${trigger:0:97}..."
            fi
            printf '  %-20s %s\n' "$name" "${trigger:-(lazy, no trigger summary)}"
        else
            printf '  %-20s (always loaded)\n' "$name"
        fi
    done < "$INCLUDE_FILE"
else
    echo "  (no include.txt found)"
fi
echo ""

echo "Skills:"
# Consolidated: user-level (root skills, from `daisy install`) then
# workspace-level (home skills, from `daisy init`) — workspace overrides
# user by name, same precedence root/home skills already have at their
# respective install steps.
declare -A SKILL_SRC
USER_SKILLS_DIR="$HOME/.claude/skills"
WORKSPACE_SKILLS_DIR="$DAISY_WORKSPACE/.claude/skills"

if [ -d "$USER_SKILLS_DIR" ]; then
    for dir in "$USER_SKILLS_DIR"/*/; do
        [ -f "${dir}SKILL.md" ] || continue
        SKILL_SRC["$(basename "$dir")"]="${dir}SKILL.md"
    done
fi
if [ -d "$WORKSPACE_SKILLS_DIR" ]; then
    for dir in "$WORKSPACE_SKILLS_DIR"/*/; do
        [ -f "${dir}SKILL.md" ] || continue
        SKILL_SRC["$(basename "$dir")"]="${dir}SKILL.md"
    done
fi

if [ "${#SKILL_SRC[@]}" -eq 0 ]; then
    echo "  (none installed — run: daisy install, then daisy init $DAISY_HOME_NAME)"
else
    for name in $(printf '%s\n' "${!SKILL_SRC[@]}" | sort); do
        skill_md="${SKILL_SRC[$name]}"
        if [ "$FULL" = true ]; then
            text=$(awk -F': ' '/^description:/{ $1=""; sub(/^ /, ""); print; exit }' "$skill_md")
        else
            text=$(awk -F': ' '/^short_description:/{ $1=""; sub(/^ /, ""); print; exit }' "$skill_md")
            [ -z "$text" ] && text=$(awk -F': ' '/^description:/{ $1=""; sub(/^ /, ""); print; exit }' "$skill_md")
        fi
        text="${text%\"}"; text="${text#\"}"
        printf '  %-20s %s\n' "$name" "$text"
    done
fi
