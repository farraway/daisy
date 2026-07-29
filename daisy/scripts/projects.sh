#!/usr/bin/env bash
# Invocation: run as `daisy projects` — do not execute this file directly.
# List active or archived projects, each with its resolved real path.
# Usage:
#   projects.sh              # list active projects (excludes _archive/)
#   projects.sh --archived   # list archived projects

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_env || exit 1
    if [ ! -d "$DAISY_HOME/projects" ]; then
        echo "Error: projects directory not found" >&2
        exit 1
    fi
    exit 0
fi

require_env || exit 1

if [ "$1" = "--archived" ]; then
    DIR="$DAISY_HOME/projects/_archive"
    LABEL="Archived projects"
else
    DIR="$DAISY_HOME/projects"
    LABEL="Active projects"
fi

shopt -s nullglob
files=("$DIR"/*.md)
shopt -u nullglob

if [ ${#files[@]} -eq 0 ]; then
    echo "$LABEL: none"
    exit 0
fi

echo "$LABEL:"
for f in "${files[@]}"; do
    name=$(basename "$f" .md)
    printf '  %-30s %s\n' "$name" "$(readlink -f "$f")"
done
