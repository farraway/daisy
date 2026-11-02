#!/usr/bin/env bash
# Invocation: run as `daisy home` — do not execute this file directly.
# Manage the homes registry: which home names exist on this machine and
# where each one lives. A home is its own git repository and the root never
# holds one; the registry is $DAISY_CONFIG/homes (default
# ~/.config/daisy/homes), one `name=/absolute/path` per line.
#
# Usage:
#   daisy home list                 List registered homes: name and path
#   daisy home add <name> <path>    Register an existing home directory
#   daisy home remove <name>        Unregister a home; its files are left in place
#   daisy home new <name> [path]    Create a home from the template at path
#                                   (prompted when omitted; default
#                                   $DAISY_LOCAL/homes/<name>), git init it
#                                   with a first commit, and register it

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

TEMPLATE="$DAISY_ROOT/templates/home"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_root || exit 1
    if [ ! -d "$TEMPLATE" ]; then
        echo "Error: home template not found at templates/home/" >&2
        exit 1
    fi
    exit 0
fi

require_root || exit 1

usage() {
    echo "Usage: daisy home list | add <name> <path> | remove <name> | new <name> [path]" >&2
    exit 1
}

# A name is a registry key and a directory name (daisy_name_ok).
require_unregistered_name() {
    local name="$1" existing
    if ! daisy_name_ok "$name"; then
        echo "Error: '$name' is not a valid home name (letters, digits, '.', '_', '-'; starting with a letter or digit)" >&2
        exit 1
    fi
    if existing=$(daisy_home_path "$name"); then
        echo "Error: Home '$name' is already registered at $existing" >&2
        echo "  Run 'daisy home remove $name' first to register it elsewhere" >&2
        exit 1
    fi
}

# A prompt with stdin closed must say so, not exit silently under set -e.
ask() {
    read -rp "$1" "$2" || {
        echo "" >&2
        echo "Error: no answer to the prompt (stdin closed) — pass the value as an argument, or run interactively" >&2
        exit 1
    }
}

home_list() {
    local name path
    if [ -z "$(daisy_homes)" ]; then
        echo "No homes registered in $(daisy_homes_file)" >&2
        echo "  Run 'daisy home new <name>' to create one, or 'daisy home add <name> <path>' to register an existing home" >&2
        return 0
    fi
    while IFS= read -r name; do
        path=$(daisy_home_path "$name") || path=""
        if [ -n "$path" ] && [ -d "$path" ]; then
            printf '%-14s %s\n' "$name" "$path"
        else
            printf '%-14s %s  (missing)\n' "$name" "$path"
        fi
    done < <(daisy_homes)
}

home_add() {
    local name="$1" path="$2" problem
    [ -n "$name" ] && [ -n "$path" ] || usage
    require_unregistered_name "$name"
    if [ ! -d "$path" ]; then
        echo "Error: $path is not a directory" >&2
        echo "  Run 'daisy home new $name $path' to create a home there" >&2
        exit 1
    fi
    path=$(daisy_realdir "$path")
    problem=$(daisy_home_path_problem "$path")
    if [ -n "$problem" ]; then
        echo "Error: $path $problem" >&2
        exit 1
    fi
    if [ ! -f "$path/include.txt" ]; then
        echo "Error: $path is not a Daisy home (no include.txt)" >&2
        echo "  A home is a copy of templates/home/; 'daisy home new' makes one" >&2
        exit 1
    fi
    if ! daisy_home_repo_ok "$path"; then
        echo "Error: $path is not in its own git repository (or the root's)" >&2
        echo "  Every Daisy mutation commits in the home's repository: run 'git -C $(printf '%q' "$path") init' and commit it first" >&2
        exit 1
    fi
    # The same staging call commit.sh makes, as a dry run: it fails when the
    # directory is gitignored in its repository.
    if ! git -C "$path" add --dry-run -- . >/dev/null 2>&1; then
        echo "Error: nothing under $path can be staged in its repository (is it gitignored?)" >&2
        echo "  Un-ignore it, or give the home its own repository" >&2
        exit 1
    fi
    daisy_register_home "$name" "$path"
    echo "✓ Registered home '$name' at $path"
}

home_remove() {
    local name="$1" path
    [ -n "$name" ] || usage
    path=$(daisy_home_path "$name") || {
        echo "Error: Home '$name' is not registered" >&2
        echo "  Run 'daisy home list' to see registered homes" >&2
        exit 1
    }
    daisy_unregister_home "$name"
    echo "✓ Unregistered home '$name' (its files at $path are untouched)"
}

home_new() {
    local name="$1" path="$2" default git_name git_email
    [ -n "$name" ] || usage
    require_unregistered_name "$name"

    if [ ! -d "$TEMPLATE" ]; then
        echo "Error: Home template not found at $TEMPLATE" >&2
        exit 1
    fi

    if [ -z "$path" ]; then
        default="$(daisy_local_dir)/homes/$name"
        ask "  Location [$default]: " path
        path="${path:-$default}"
        path="${path/#\~/$HOME}"
    fi
    if [ -e "$path" ]; then
        echo "Error: $path already exists" >&2
        echo "  Run 'daisy home add $name $path' if it is already a home, or choose another location" >&2
        exit 1
    fi

    # Every answer is collected before anything is written, so an abandoned
    # prompt leaves no half-made home behind.
    echo ""
    echo "Git identity for home '$name':"
    ask "  Name: " git_name
    ask "  Email: " git_email

    # From the first write to registration, any failure removes the copy: a
    # half-made directory would block a retry and could be mis-registered.
    # The first commit is the identity check; git's own message says what
    # is missing.
    mkdir -p "$(dirname "$path")"
    NEW_HOME_PATH="$path"  # global: the EXIT trap runs outside this function's scope
    trap 'if [ ! -e "$NEW_HOME_PATH" ]; then echo "Error: creating the home failed; nothing was written or registered" >&2; elif rm -rf "$NEW_HOME_PATH"; then echo "Error: creating the home failed; $NEW_HOME_PATH was removed and nothing is registered" >&2; else echo "Error: creating the home failed and $NEW_HOME_PATH could not be removed; remove it by hand, nothing is registered" >&2; fi' EXIT
    cp -r "$TEMPLATE" "$path"
    path=$(daisy_realdir "$path")

    if [ -n "$git_name" ] && [ -n "$git_email" ]; then
        printf 'name=%s\nemail=%s\n' "$git_name" "$git_email" > "$path/gitconfig"
        echo "  ✓ Saved git identity to $path/gitconfig"
    else
        echo "  ⚠ Skipped (both name and email required). Edit $path/gitconfig later."
    fi

    git init -q "$path"
    if [ -n "$git_name" ] && [ -n "$git_email" ]; then
        git -C "$path" config user.name "$git_name"
        git -C "$path" config user.email "$git_email"
    fi
    git -C "$path" add -A
    git -C "$path" commit -q -m "New home: $name"
    echo "  ✓ Initialized git repository with first commit"

    daisy_register_home "$name" "$path"
    trap - EXIT

    "$(daisy_bin)" prompt build "$name" ||
        echo "  ⚠ AGENTS.md not built; run 'daisy prompt build $name'" >&2

    echo ""
    echo "✅ Created home: $name"
    echo "   Location: $path"
    echo ""
    echo "📋 Next steps:"
    echo "   1. Edit $path/include.txt to choose which prompts to load"
    echo "      Common prompts: daisy, retrospective, github"
    echo "      Work-specific: add your own under $path/prompts/"
    echo "   2. To use in a workspace: daisy init $name"
}

case "${1:-list}" in
    list)   home_list ;;
    add)    shift; home_add "$@" ;;
    remove) shift; home_remove "$@" ;;
    new)    shift; home_new "$@" ;;
    *)      usage ;;
esac
