#!/usr/bin/env bash
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

# --- daisy/ symlink to repo ---

if [ -e "$TARGET/daisy" ]; then
    if [ -L "$TARGET/daisy" ]; then
        EXISTING="$(readlink "$TARGET/daisy")"
        if [ "$EXISTING" = "$DAISY_ROOT" ]; then
            echo "  ✓ daisy/ symlink already exists"
        else
            echo "  ⚠ daisy/ symlink exists but points to: $EXISTING" >&2
            echo "    Expected: $DAISY_ROOT" >&2
            echo "    Remove it and re-run if you want to update." >&2
            exit 1
        fi
    else
        echo "Error: $TARGET/daisy exists and is not a symlink" >&2
        exit 1
    fi
else
    ln -s "$DAISY_ROOT" "$TARGET/daisy"
    echo "  ✓ Created daisy/ → $DAISY_ROOT"
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
        rm -f "$DAISY_DIR/AGENTS.md" "$DAISY_DIR/tasks" "$DAISY_DIR/today.md" "$DAISY_DIR/journal.md" "$DAISY_DIR/projects"
    fi
fi

mkdir -p "$DAISY_DIR"

# Write home config
echo "$HOME_NAME" > "$DAISY_DIR/home"

# Create symlinks (relative, so they work if workspace moves)
cd "$DAISY_DIR"
for link in AGENTS.md tasks today.md journal.md projects; do
    rm -f "$link"
done

ln -s "../daisy/home/$HOME_NAME/AGENTS.md" AGENTS.md
ln -s "../daisy/home/$HOME_NAME/tasks" tasks
ln -s "../daisy/home/$HOME_NAME/journal/today.md" today.md
ln -s "../daisy/home/$HOME_NAME/journal/journal.md" journal.md
ln -s "../daisy/home/$HOME_NAME/projects" projects
cd "$TARGET"

echo "  ✓ Created .daisy/ with symlinks for home: $HOME_NAME"

# --- Cursor rules ---

RULES_DIR="$TARGET/.cursor/rules"
mkdir -p "$RULES_DIR"

for RULE_PAIR in "daisy.md:cursor-rule.md" "daisy-logging.md:cursor-rule-logging.md"; do
    RULE_NAME="${RULE_PAIR%%:*}"
    TEMPLATE="${RULE_PAIR##*:}"
    RULE_FILE="$RULES_DIR/$RULE_NAME"
    if [ -L "$RULE_FILE" ] || [ -e "$RULE_FILE" ]; then
        if [ -L "$RULE_FILE" ]; then
            echo "  ✓ Cursor rule $RULE_NAME already installed"
        else
            echo "  ⚠ .cursor/rules/$RULE_NAME exists but is not a symlink"
            echo "    Remove it and re-run if you want to update."
        fi
    else
        ln -s "$DAISY_ROOT/daisy/templates/$TEMPLATE" "$RULE_FILE"
        echo "  ✓ Installed Cursor rule: $RULE_NAME"
    fi
done

# --- .gitignore ---

GITIGNORE="$TARGET/.gitignore"
DAISY_IGNORE_ENTRIES=(".daisy/" "daisy" ".cursor/rules/daisy.md" ".cursor/rules/daisy-logging.md")
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
CURSOR_NEGATE_ENTRIES=("!.daisy/" "!daisy" "!.cursor/rules/daisy.md" "!.cursor/rules/daisy-logging.md")
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

# --- Build AGENTS.md if missing ---

if [ ! -f "$HOME_DIR/AGENTS.md" ]; then
    echo "  Building AGENTS.md..."
    DAISY_HOME="$HOME_DIR" "$DAISY_ROOT/daisy/scripts/build-prompt.sh" "$HOME_NAME"
fi

echo ""
echo "Done. Daisy is ready in this workspace."
echo "Start a Cursor session and say: \"Daisy, start a new day\""
