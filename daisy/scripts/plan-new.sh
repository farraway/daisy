#!/usr/bin/env bash
# Create a new Daisy plan file and symlink it as PLAN.md at the workspace root.
# Usage: plan-new.sh <description> [project-tag]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

require_env || exit 1

DESCRIPTION="$1"
PROJECT_TAG="${2-}"

if [ -z "$DESCRIPTION" ]; then
    echo "Error: Description required" >&2
    echo "Usage: plan-new.sh <description> [project-tag]" >&2
    exit 1
fi

if [ -z "$DAISY_WORKSPACE" ]; then
    echo "Error: Could not determine workspace root" >&2
    exit 1
fi

# Error if PLAN.md already exists
if [ -e "$DAISY_WORKSPACE/PLAN.md" ]; then
    echo "Error: PLAN.md already exists in this workspace" >&2
    echo "Archive it first: /daisy archive PLAN.md" >&2
    exit 1
fi

# Strip leading + from project tag if present
PROJECT="${PROJECT_TAG#\+}"

# Generate slug: replace non-alphanumeric characters with hyphens, preserve case
SLUG=$(printf '%s' "$DESCRIPTION" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+-/-/g' | sed 's/^-//;s/-$//')

# Generate timestamp
TIMESTAMP=$(date +%y%m%d%H%M%S)

# Build filename
if [ -n "$PROJECT" ]; then
    FILENAME="${TIMESTAMP}.${PROJECT}.${SLUG}.plan.md"
else
    FILENAME="${TIMESTAMP}.${SLUG}.plan.md"
fi

TEMPLATE="$DAISY_ROOT/daisy/templates/plan.md"
PLAN_FILE="$DAISY_HOME/plans/$FILENAME"
CREATED=$(date '+%Y-%m-%d %H:%M')

if [ ! -f "$TEMPLATE" ]; then
    echo "Error: Plan template not found at $TEMPLATE" >&2
    exit 1
fi

# Populate template via awk
EM_DASH="—"
awk -v desc="$DESCRIPTION" -v proj="$PROJECT" -v created="$CREATED" \
    -v daisy_root="$DAISY_ROOT" -v emdash="$EM_DASH" '
    /\{Description\}\{project_suffix\}/ {
        if (proj != "") {
            print "# " desc " " emdash " +" proj
        } else {
            print "# " desc
        }
        next
    }
    /\*\*Project:\*\* \+\{project\}/ {
        if (proj != "") {
            print "**Project:** +" proj
        }
        # no project: skip line
        next
    }
    /\{YYYY-MM-DD HH:MM\}/ {
        sub(/\{YYYY-MM-DD HH:MM\}/, created)
        print
        next
    }
    /\$DAISY_ROOT/ {
        gsub(/\$DAISY_ROOT/, daisy_root)
        print
        next
    }
    { print }
' "$TEMPLATE" > "$PLAN_FILE"

# Create relative symlink from workspace root
(cd "$DAISY_WORKSPACE" && ln -s ".daisy/plans/$FILENAME" PLAN.md)

# Add PLAN.md to .gitignore if not already present
GITIGNORE="$DAISY_WORKSPACE/.gitignore"
if [ -f "$GITIGNORE" ]; then
    if ! grep -qxF "PLAN.md" "$GITIGNORE"; then
        echo "PLAN.md" >> "$GITIGNORE"
        echo "  ✓ Added PLAN.md to .gitignore"
    fi
else
    echo "PLAN.md" > "$GITIGNORE"
    echo "  ✓ Created .gitignore with PLAN.md"
fi

echo "✅ Plan created: $FILENAME"
echo "   Linked as: $DAISY_WORKSPACE/PLAN.md"

exit 0
