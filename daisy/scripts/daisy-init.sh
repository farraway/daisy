#!/usr/bin/env bash
# Invocation: run as `daisy init` — do not execute this file directly.
# Initialize Daisy in a workspace with a specific home.
# Creates .daisy/ directory with symlinks to the home's data.
#
# Usage:
#   daisy-init <home>              # init in current directory
#   daisy-init <home> /path/to/ws  # init in specified directory
#   daisy-init --new <home>        # create a new home, then init
#
# Re-running with a different home switches the workspace to that home.
#
# Called via `daisy init` (see $DAISY_ROOT/daisy.sh). Can also be invoked directly.

set -e

if [ -z "$DAISY_ROOT" ]; then
    echo "Error: DAISY_ROOT not set" >&2
    echo "Add to ~/.zshenv:  export DAISY_ROOT=\"/path/to/daisy\"" >&2
    exit 1
fi

source "$DAISY_ROOT/daisy/scripts/common.sh"

# Parse arguments
CREATE_NEW=false
if [ "$1" = "--new" ]; then
    CREATE_NEW=true
    shift
fi

HOME_NAME="$1"
TARGET="${2:-.}"

if [ -z "$HOME_NAME" ]; then
    echo "Usage: daisy-init [--new] <home> [workspace-path]" >&2
    echo "" >&2
    echo "Options:" >&2
    echo "  --new    Create a new home before initializing" >&2
    echo "" >&2
    echo "Available homes:" >&2
    for dir in "$DAISY_ROOT"/home/*/; do
        [ -d "$dir" ] && echo "  $(basename "$dir")" >&2
    done
    exit 1
fi

# Create new home if requested
HOME_DIR="$DAISY_ROOT/home/$HOME_NAME"
if [ "$CREATE_NEW" = true ]; then
    if [ -d "$HOME_DIR" ]; then
        echo "Error: Home '$HOME_NAME' already exists at $HOME_DIR" >&2
        exit 1
    fi
    "$DAISY_ROOT/daisy/scripts/create-home.sh" "$HOME_NAME"
fi

# Validate home exists
if [ ! -d "$HOME_DIR" ]; then
    echo "Error: Home '$HOME_NAME' not found at $HOME_DIR" >&2
    echo "" >&2
    echo "Available homes:" >&2
    for dir in "$DAISY_ROOT"/home/*/; do
        [ -d "$dir" ] && echo "  $(basename "$dir")" >&2
    done
    echo "" >&2
    echo "To create a new home: daisy-init --new $HOME_NAME" >&2
    exit 1
fi

TARGET="$(cd "$TARGET" && pwd)"

echo "Initializing Daisy in: $TARGET"
echo "  Home: $HOME_NAME"

# --- git init ---

if [ ! -d "$TARGET/.git" ]; then
    git init "$TARGET" >/dev/null 2>&1
    echo "  ✓ Initialized git repository"
fi

# --- .daisy/ directory with home config and symlinks ---

DAISY_DIR="$TARGET/.daisy"

# If .daisy/ already exists, check if switching homes
if [ -d "$DAISY_DIR" ] && [ -f "$DAISY_DIR/home" ]; then
    OLD_HOME=$(cat "$DAISY_DIR/home")
    if [ "$OLD_HOME" = "$HOME_NAME" ]; then
        echo "  ✓ Already initialized with home: $HOME_NAME"
    else
        echo "  ↻ Switching home: $OLD_HOME → $HOME_NAME"
        # Remove old symlinks
        rm -f "$DAISY_DIR/AGENTS.md" "$DAISY_DIR/tasks" "$DAISY_DIR/today.md" "$DAISY_DIR/journal.md" "$DAISY_DIR/projects" "$DAISY_DIR/plans" "$DAISY_DIR/feedback.md"
    fi
fi

mkdir -p "$DAISY_DIR"

# Write home config
echo "$HOME_NAME" > "$DAISY_DIR/home"

# Create absolute symlinks pointing directly to $DAISY_ROOT
cd "$DAISY_DIR"
for link in tasks today.md journal.md projects plans feedback.md; do
    rm -f "$link"
done
rm -f AGENTS.md  # may be symlink (old) or file (new) — always regenerate below

ln -s "$DAISY_ROOT/home/$HOME_NAME/tasks" tasks
ln -s "$DAISY_ROOT/home/$HOME_NAME/journal/today.md" today.md
ln -s "$DAISY_ROOT/home/$HOME_NAME/journal/journal.md" journal.md
ln -s "$DAISY_ROOT/home/$HOME_NAME/projects" projects
ln -s "$DAISY_ROOT/home/$HOME_NAME/plans" plans
ln -s "$DAISY_ROOT/home/$HOME_NAME/feedback/feedback.md" feedback.md
cd "$TARGET"

echo "  ✓ Created .daisy/ symlinks for home: $HOME_NAME"

# Generate AGENTS.md into the workspace
DAISY_HOME="$HOME_DIR" "$DAISY_ROOT/daisy/scripts/build-prompt.sh" \
    --output "$DAISY_DIR/AGENTS.md" "$HOME_NAME"
echo "  ✓ Generated .daisy/AGENTS.md"

# --- Cursor rules ---

RULES_DIR="$TARGET/.cursor/rules"
mkdir -p "$RULES_DIR"

for RULE_NAME in "${DAISY_CURSOR_RULE_FILES[@]}"; do
    TEMPLATE="cursor-rule${RULE_NAME#daisy}"
    RULE_FILE="$RULES_DIR/$RULE_NAME"
    rm -f "$RULE_FILE"
    cp "$DAISY_ROOT/daisy/templates/$TEMPLATE" "$RULE_FILE"
    echo "  ✓ Installed Cursor rule: $RULE_NAME"
done

# Sweep obsolete rule names left behind by a prior .md → .mdc rename or a
# name later dropped from DAISY_CURSOR_RULE_FILES — daisy-init.sh only ever
# writes the current names, so nothing else removes these.
OBSOLETE_SWEPT=0
for OBSOLETE_NAME in "${DAISY_CURSOR_RULE_OBSOLETE[@]}"; do
    OBSOLETE_FILE="$RULES_DIR/$OBSOLETE_NAME"
    if [ -f "$OBSOLETE_FILE" ]; then
        rm -f "$OBSOLETE_FILE"
        OBSOLETE_SWEPT=$((OBSOLETE_SWEPT + 1))
        echo "  ✓ Removed obsolete Cursor rule: $OBSOLETE_NAME"
    fi
done

# --- Claude command ---

CLAUDE_COMMAND_PATH="$TARGET/$DAISY_CLAUDE_COMMAND_FILE"
mkdir -p "$(dirname "$CLAUDE_COMMAND_PATH")"
cp "$DAISY_ROOT/daisy/templates/claude-command.md" "$CLAUDE_COMMAND_PATH"
echo "  ✓ Installed Claude command: daisy"

# --- Skills ---
# Home skills only — root skills install separately via `daisy install`
# (user-level, ~/.claude/skills/), not per-workspace. Copied (not symlinked)
# so a workspace's skill set is a point-in-time snapshot, refreshed only by
# re-running `daisy init`. Only the specific skill-named subdirectories/files
# below are touched — anything else already in .claude/skills/ or
# .cursor/rules/ (unrelated, user-managed) is left alone.

SKILLS_CLAUDE_DIR="$TARGET/.claude/skills"
SKILLS_CURSOR_DIR="$TARGET/.cursor/rules"
mkdir -p "$SKILLS_CLAUDE_DIR" "$SKILLS_CURSOR_DIR"

declare -A SKILL_SRC
# find, not a one-level glob — a home's skills could nest, matching root's
# discovery convention. The skill's name is always the immediate parent
# directory of its SKILL.md, regardless of how deep that directory sits.
while IFS= read -r -d '' skill_md; do
    dir="$(dirname "$skill_md")"
    SKILL_SRC["$(basename "$dir")"]="$dir/"
done < <(find "$HOME_DIR/skills" -name SKILL.md -print0 2>/dev/null)

SKILL_COUNT=0
for name in "${!SKILL_SRC[@]}"; do
    src="${SKILL_SRC[$name]}"

    # Claude Code: copy the full skill directory (SKILL.md plus any sibling
    # reference files) so nothing a skill discloses to gets left behind.
    rm -rf "${SKILLS_CLAUDE_DIR:?}/$name"
    cp -r "$src" "$SKILLS_CLAUDE_DIR/$name"

    # Cursor has no workspace-scoped skills directory, so deliver the same
    # skill as a pointer rule — same "Read X when Y" pattern cursor-rule.mdc
    # already uses to deliver .daisy/AGENTS.md.
    desc=$(awk -F': ' '/^description:/{ $1=""; sub(/^ /, ""); print; exit }' "$src/SKILL.md")
    desc="${desc%\"}"; desc="${desc#\"}"
    cat > "$SKILLS_CURSOR_DIR/$name.mdc" <<EOF
---
description: $desc
alwaysApply: false
---

# $name

Read \`.claude/skills/$name/SKILL.md\` when: $desc
EOF

    SKILL_COUNT=$((SKILL_COUNT + 1))
done
echo "  ✓ Installed $SKILL_COUNT skill(s) into .claude/skills/ and .cursor/rules/"

# Sweep skill .mdc stubs (and .claude/skills/ copies) whose home skill no
# longer exists in $HOME_DIR/skills — otherwise a deleted skill's pointer
# rule and copy linger in the workspace forever after the next `daisy init`.
SKILL_STUB_SWEPT=0
for MDC_FILE in "$SKILLS_CURSOR_DIR"/*.mdc; do
    [ -f "$MDC_FILE" ] || continue
    STUB_NAME="$(basename "$MDC_FILE" .mdc)"
    case " ${DAISY_CURSOR_RULE_FILES[*]} " in
        *" $STUB_NAME.mdc "*) continue ;;  # core rule, not a skill stub
    esac
    if [ -z "${SKILL_SRC[$STUB_NAME]+x}" ]; then
        rm -f "$MDC_FILE"
        rm -rf "${SKILLS_CLAUDE_DIR:?}/$STUB_NAME"
        SKILL_STUB_SWEPT=$((SKILL_STUB_SWEPT + 1))
    fi
done
if [ "$SKILL_STUB_SWEPT" -gt 0 ]; then
    echo "  ✓ Swept $SKILL_STUB_SWEPT stale skill stub(s) from .claude/skills/ and .cursor/rules/"
fi

# --- Claude Code permissions ---

CLAUDE_SETTINGS="$TARGET/.claude/settings.local.json"
DAISY_ALLOW=("${DAISY_ALLOW_BASE[@]}")

if command -v jq >/dev/null 2>&1; then
    mkdir -p "$(dirname "$CLAUDE_SETTINGS")"
    existing='{}'
    [ -f "$CLAUDE_SETTINGS" ] && existing=$(cat "$CLAUDE_SETTINGS")
    updated="$existing"
    # Heal existing workspaces: remove the legacy raw-scripts grant superseded by Bash(daisy:*)
    updated=$(echo "$updated" | jq --arg r "Bash($DAISY_ROOT/daisy/scripts/*)" '
        if .permissions.allow then
            .permissions.allow |= map(select(. != $r))
        else . end
    ')
    for rule in "${DAISY_ALLOW[@]}"; do
        updated=$(echo "$updated" | jq \
            --arg r "$rule" \
            '.permissions.allow //= [] | if (.permissions.allow | index($r)) == null then .permissions.allow += [$r] else . end')
    done
    echo "$updated" | jq '.' > "$CLAUDE_SETTINGS"
    echo "  ✓ Configured .claude/settings.local.json with Daisy permissions"
else
    echo "  ⚠ jq not available — skipping Claude Code permissions (add manually)"
fi

# --- .gitignore ---

GITIGNORE="$TARGET/.gitignore"
DAISY_IGNORE_ENTRIES=("${DAISY_GITIGNORE_BASE_ENTRIES[@]}")
for name in "${!SKILL_SRC[@]}"; do
    DAISY_IGNORE_ENTRIES+=(".claude/skills/$name/" ".cursor/rules/$name.mdc")
done
GITIGNORE_CHANGED=false

if [ ! -f "$GITIGNORE" ]; then
    touch "$GITIGNORE"
    echo "  ✓ Created .gitignore"
fi

NEED_HEADER=true
for entry in "${DAISY_IGNORE_ENTRIES[@]}"; do
    if ! grep -qxF "$entry" "$GITIGNORE" 2>/dev/null; then
        if [ "$NEED_HEADER" = true ]; then
            echo "" >> "$GITIGNORE"
            echo "# Daisy workspace config (per-machine)" >> "$GITIGNORE"
            NEED_HEADER=false
        fi
        echo "$entry" >> "$GITIGNORE"
        GITIGNORE_CHANGED=true
    fi
done

if [ "$GITIGNORE_CHANGED" = true ]; then
    echo "  ✓ Added daisy paths to .gitignore"
fi

# --- .cursorignore ---

CURSORIGNORE="$TARGET/.cursorignore"
CURSOR_NEGATE_ENTRIES=("${DAISY_CURSORIGNORE_BASE_ENTRIES[@]}")
CURSORIGNORE_CHANGED=false

if [ ! -f "$CURSORIGNORE" ]; then
    touch "$CURSORIGNORE"
fi

NEED_HEADER=true
for entry in "${CURSOR_NEGATE_ENTRIES[@]}"; do
    if ! grep -qxF "$entry" "$CURSORIGNORE" 2>/dev/null; then
        if [ "$NEED_HEADER" = true ]; then
            echo "" >> "$CURSORIGNORE"
            echo "# Allow Cursor to index daisy paths (gitignored but needed for agent context)" >> "$CURSORIGNORE"
            NEED_HEADER=false
        fi
        echo "$entry" >> "$CURSORIGNORE"
        CURSORIGNORE_CHANGED=true
    fi
done

if [ "$CURSORIGNORE_CHANGED" = true ]; then
    echo "  ✓ Configured .cursorignore to index daisy paths"
fi

# --- Git identity ---

GITCONFIG_FILE="$HOME_DIR/gitconfig"
if [ -f "$GITCONFIG_FILE" ]; then
    GIT_NAME=$(grep '^name=' "$GITCONFIG_FILE" | cut -d= -f2-)
    GIT_EMAIL=$(grep '^email=' "$GITCONFIG_FILE" | cut -d= -f2-)

    if [ -n "$GIT_NAME" ] && [ -n "$GIT_EMAIL" ]; then
        git -C "$TARGET" config --local user.name "$GIT_NAME"
        git -C "$TARGET" config --local user.email "$GIT_EMAIL"
        echo "  ✓ Set local git identity: $GIT_NAME <$GIT_EMAIL>"
    else
        echo "  ⚠ home/$HOME_NAME/gitconfig is incomplete (need both name and email)"
    fi
else
    echo "  ⚠ No gitconfig found for home '$HOME_NAME' (git identity not configured)"
fi

echo ""
echo "Done. Daisy is ready in this workspace."
echo "Start a Cursor session and say: \"Daisy, start a new day\""
echo "Or use /daisy in Claude Code: \"/daisy start a new day\""
