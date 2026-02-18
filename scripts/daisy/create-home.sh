#!/usr/bin/env bash
# Create a new daisy home from template
# Usage: create-home.sh <home-name> [--activate]

set -e

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    if [ -z "$DAISY_ROOT" ]; then
        echo "Error: DAISY_ROOT not set" >&2
        exit 1
    fi
    if [ ! -d "$DAISY_ROOT/templates/home" ]; then
        echo "Error: home template not found at templates/home/" >&2
        exit 1
    fi
    exit 0
fi

# Validate environment
if [ -z "$DAISY_ROOT" ]; then
    echo "Error: DAISY_ROOT not set" >&2
    echo "Add to ~/.zshenv: export DAISY_ROOT=/path/to/daisy" >&2
    exit 1
fi

# Check for home name argument
if [ -z "$1" ] || [ "$1" = "--activate" ]; then
    echo "Usage: create-home.sh <home-name> [--activate]" >&2
    echo "" >&2
    echo "Options:" >&2
    echo "  --activate    Switch to the new home after creation" >&2
    echo "" >&2
    echo "Existing homes:" >&2
    for dir in "$DAISY_ROOT"/home/*/; do
        [ -d "$dir" ] && echo "  $(basename "$dir")" >&2
    done
    exit 1
fi

NAME="$1"
ACTIVATE=false
if [ "$2" = "--activate" ]; then
    ACTIVATE=true
fi

TARGET_DIR="$DAISY_ROOT/home/$NAME"

# Check if home already exists
if [ -d "$TARGET_DIR" ]; then
    echo "Error: Home '$NAME' already exists at $TARGET_DIR" >&2
    exit 1
fi

# Validate template exists
if [ ! -d "$DAISY_ROOT/templates/home" ]; then
    echo "Error: Home template not found at $DAISY_ROOT/templates/home/" >&2
    exit 1
fi

# Copy template
cp -r "$DAISY_ROOT/templates/home" "$TARGET_DIR"

# Create projects directory with archive
mkdir -p "$TARGET_DIR/projects/_archive"

echo "✅ Created home: $NAME"
echo "   Location: $TARGET_DIR"
echo ""
echo "📋 Next steps:"
echo "   1. Edit $TARGET_DIR/include.txt to choose which prompts to load"
echo "      Common prompts: daisy, retrospective, github"
echo "      Work-specific: see prompt.md for examples"
echo ""

# Show current include.txt contents
echo "   Current include.txt:"
while IFS= read -r line; do
    echo "      $line"
done < "$TARGET_DIR/include.txt"
echo ""

# Activate if requested
if [ "$ACTIVATE" = true ]; then
    echo "   2. Activating home..."
    "$DAISY_ROOT/scripts/daisy/switch-home.sh" "$NAME"
else
    echo "   2. To activate: $DAISY_ROOT/scripts/daisy/switch-home.sh $NAME"
fi

exit 0
