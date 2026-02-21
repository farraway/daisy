#!/usr/bin/env bash
# Daisy CLI - unified command-line interface for the Daisy productivity system.
#
# Usage:
#   daisy <command> [args...]
#
# Install:
#   /path/to/daisy.sh install

set -e

# --- resolve DAISY_ROOT ---
# For `install`, derive from the script's real location (resolving symlinks).
# For everything else, require the env var.

COMMAND="${1:-help}"

if [ "$COMMAND" = "install" ]; then
    # Resolve symlinks to find the real script location
    SOURCE="${BASH_SOURCE[0]}"
    while [ -L "$SOURCE" ]; do
        DIR="$(cd "$(dirname "$SOURCE")" && pwd)"
        SOURCE="$(readlink "$SOURCE")"
        [[ "$SOURCE" != /* ]] && SOURCE="$DIR/$SOURCE"
    done
    DAISY_ROOT="$(cd "$(dirname "$SOURCE")" && pwd)"
elif [ -z "$DAISY_ROOT" ]; then
    echo "Error: DAISY_ROOT not set. Run '$0 install' first." >&2
    exit 1
fi

SCRIPTS="$DAISY_ROOT/daisy/scripts"

# --- workspace root resolution ---

find_workspace_root() {
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        if [ -d "$dir/.daisy" ]; then
            echo "$dir"
            return 0
        fi
        dir=$(dirname "$dir")
    done
    return 1
}

require_workspace() {
    WORKSPACE_ROOT=$(find_workspace_root) || {
        echo "Error: Not in a Daisy workspace (no .daisy/ directory found)" >&2
        echo "  Run 'daisy init <home>' to initialize this workspace." >&2
        exit 1
    }
    pushd "$WORKSPACE_ROOT" > /dev/null
}

# --- built-in: help ---

show_help() {
    cat <<'EOF'
Usage: daisy <command> [args...]

Commands:
  init [--new] <home> [path]   Initialize Daisy in a workspace
  clean [-f]                   Remove Daisy from the current workspace
  status                       Show quick workspace summary
  healthcheck [--force]        Run system health check
  log <message...>             Add a log entry to today.md
  done <pattern>               Mark a task as complete
  new-day                      Start a new day
  new-week                     Start a new week
  build [home]                 Rebuild AGENTS.md for a home
  install                      Set up ~/bin/daisy symlink and shell environment
  help                         Show this help

Environment:
  DAISY_ROOT    Path to the daisy repository (required)
EOF
}

# --- built-in: clean ---

cmd_clean() {
    local force=false
    if [ "$1" = "-f" ] || [ "$1" = "--force" ]; then
        force=true
    fi

    require_workspace

    local home_name="unknown"
    if [ -f ".daisy/home" ]; then
        home_name=$(cat ".daisy/home")
    fi

    if [ "$force" != true ]; then
        echo "This will remove Daisy from: $WORKSPACE_ROOT"
        echo "  Home: $home_name"
        echo ""
        read -rp "Continue? [y/N] " confirm
        case "$confirm" in
            [yY]|[yY][eE][sS]) ;;
            *) echo "Cancelled."; popd > /dev/null; exit 0 ;;
        esac
    fi

    # 1. Remove .daisy/ directory
    if [ -d ".daisy" ]; then
        rm -rf ".daisy"
        echo "  ✓ Removed .daisy/"
    fi

    # 2. Remove daisy symlink
    if [ -L "daisy" ]; then
        rm -f "daisy"
        echo "  ✓ Removed daisy symlink"
    fi

    # 3. Remove Cursor rule symlink
    if [ -L ".cursor/rules/daisy.md" ]; then
        rm -f ".cursor/rules/daisy.md"
        echo "  ✓ Removed Cursor rule"
        # Remove empty .cursor/rules/ if we created it
        rmdir ".cursor/rules" 2>/dev/null || true
        rmdir ".cursor" 2>/dev/null || true
    fi

    # 4. Remove Daisy entries from .gitignore
    if [ -f ".gitignore" ]; then
        local cleaned=false
        for line in "# Daisy workspace config (per-machine)" ".daisy/" "daisy" ".cursor/rules/daisy.md"; do
            if grep -qxF "$line" ".gitignore" 2>/dev/null; then
                grep -vxF "$line" ".gitignore" > ".gitignore.tmp"
                mv ".gitignore.tmp" ".gitignore"
                cleaned=true
            fi
        done
        # Remove trailing blank lines left behind
        sed -e :a -e '/^\n*$/{$d;N;ba' -e '}' ".gitignore" > ".gitignore.tmp"
        mv ".gitignore.tmp" ".gitignore"
        if [ "$cleaned" = true ]; then
            echo "  ✓ Cleaned .gitignore"
        fi
        # Remove .gitignore if now empty
        if [ ! -s ".gitignore" ]; then
            rm -f ".gitignore"
            echo "  ✓ Removed empty .gitignore"
        fi
    fi

    # 5. Remove Daisy entries from .cursorignore
    if [ -f ".cursorignore" ]; then
        local cleaned=false
        for line in "# Allow Cursor to index daisy paths (gitignored but needed for agent context)" "!.daisy/" "!daisy" "!.cursor/rules/daisy.md"; do
            if grep -qxF "$line" ".cursorignore" 2>/dev/null; then
                grep -vxF "$line" ".cursorignore" > ".cursorignore.tmp"
                mv ".cursorignore.tmp" ".cursorignore"
                cleaned=true
            fi
        done
        sed -e :a -e '/^\n*$/{$d;N;ba' -e '}' ".cursorignore" > ".cursorignore.tmp"
        mv ".cursorignore.tmp" ".cursorignore"
        if [ "$cleaned" = true ]; then
            echo "  ✓ Cleaned .cursorignore"
        fi
        if [ ! -s ".cursorignore" ]; then
            rm -f ".cursorignore"
            echo "  ✓ Removed empty .cursorignore"
        fi
    fi

    # 6. Unset local git identity if it matches the home's gitconfig
    local gitconfig="$DAISY_ROOT/home/$home_name/gitconfig"
    if [ -f "$gitconfig" ] && [ -d ".git" ]; then
        local cfg_name cfg_email local_name local_email
        cfg_name=$(grep '^name=' "$gitconfig" | cut -d= -f2-)
        cfg_email=$(grep '^email=' "$gitconfig" | cut -d= -f2-)
        local_name=$(git config --local user.name 2>/dev/null || true)
        local_email=$(git config --local user.email 2>/dev/null || true)

        if [ -n "$cfg_name" ] && [ "$local_name" = "$cfg_name" ]; then
            git config --local --unset user.name 2>/dev/null || true
        fi
        if [ -n "$cfg_email" ] && [ "$local_email" = "$cfg_email" ]; then
            git config --local --unset user.email 2>/dev/null || true
        fi
        echo "  ✓ Cleared local git identity"
    fi

    echo ""
    echo "Done. Daisy has been removed from this workspace."
    popd > /dev/null
}

# --- built-in: status ---

cmd_status() {
    require_workspace
    source "$SCRIPTS/common.sh"
    require_env || { popd > /dev/null; exit 1; }

    echo "Daisy | home: $DAISY_HOME_NAME | workspace: $WORKSPACE_ROOT"
    echo ""

    local today_file="$DAISY_HOME/journal/today.md"
    local todo_file="$DAISY_HOME/tasks/todo.txt"

    if [ ! -f "$today_file" ]; then
        echo "  No today.md — run 'daisy new-day' to start."
        popd > /dev/null
        return
    fi

    # Count tasks in today.md
    local pending completed
    pending=$(grep -c '^\- \[ \]' "$today_file" 2>/dev/null) || true
    completed=$(grep -c '^\- \[x\]' "$today_file" 2>/dev/null) || true
    : "${pending:=0}" "${completed:=0}"

    # Count log entries (lines like "- 1423 ..." under #### Log)
    local log_count
    log_count=$(awk '
        /^#### Log/ { in_log=1; next }
        in_log && /^#### / { exit }
        in_log && /^- [0-9]{4} / { count++ }
        END { print count+0 }
    ' "$today_file")

    echo "Tasks:  $pending pending, $completed completed today"
    echo "Log:    $log_count entries"

    # Show todo.txt summary if it exists
    if [ -f "$todo_file" ]; then
        local total_pending
        total_pending=$(grep -cvE '^(x |z |$)' "$todo_file" 2>/dev/null) || true
        : "${total_pending:=0}"
        echo "Backlog: $total_pending tasks in todo.txt"
    fi

    popd > /dev/null
}

# --- built-in: install ---

detect_shell_rc() {
    case "$(basename "$SHELL")" in
        zsh)  echo "$HOME/.zshenv" ;;
        bash) echo "$HOME/.bashrc" ;;
        fish) echo "$HOME/.config/fish/config.fish" ;;
        *)    echo "$HOME/.profile" ;;
    esac
}

cmd_install() {
    local rc_file
    rc_file=$(detect_shell_rc)
    local is_fish=false
    [ "$(basename "$SHELL")" = "fish" ] && is_fish=true

    echo "Installing Daisy..."
    echo "  DAISY_ROOT: $DAISY_ROOT"
    echo ""

    # 1. Create ~/bin if needed
    if [ ! -d "$HOME/bin" ]; then
        mkdir -p "$HOME/bin"
        echo "  ✓ Created ~/bin"
    fi

    # 2. Symlink ~/bin/daisy
    local target="$DAISY_ROOT/daisy.sh"
    local link="$HOME/bin/daisy"

    if [ -L "$link" ]; then
        local existing
        existing=$(readlink "$link")
        if [ "$existing" = "$target" ]; then
            echo "  ✓ ~/bin/daisy symlink already correct"
        else
            rm -f "$link"
            ln -s "$target" "$link"
            echo "  ✓ Updated ~/bin/daisy symlink (was: $existing)"
        fi
    elif [ -e "$link" ]; then
        echo "  ⚠ ~/bin/daisy exists but is not a symlink. Remove it and re-run." >&2
        exit 1
    else
        ln -s "$target" "$link"
        echo "  ✓ Created ~/bin/daisy → $target"
    fi

    # 3. Select default home
    local homes=()
    for dir in "$DAISY_ROOT"/home/*/; do
        [ -d "$dir" ] && homes+=("$(basename "$dir")")
    done

    local selected_home=""
    if [ ${#homes[@]} -eq 0 ]; then
        echo ""
        echo "No homes found. Creating one now..."
        read -rp "  Home name: " selected_home
        if [ -z "$selected_home" ]; then
            echo "  Skipped home creation." >&2
        else
            "$SCRIPTS/create-home.sh" "$selected_home"
        fi
    elif [ ${#homes[@]} -eq 1 ]; then
        selected_home="${homes[0]}"
        echo "  Using only available home: $selected_home"
    else
        echo ""
        echo "Available homes:"
        local i=1
        for h in "${homes[@]}"; do
            echo "  $i) $h"
            i=$((i + 1))
        done
        echo ""
        read -rp "Select default home [1]: " choice
        choice="${choice:-1}"
        if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -ge 1 ] && [ "$choice" -le ${#homes[@]} ]; then
            selected_home="${homes[$((choice - 1))]}"
        else
            echo "  Invalid selection." >&2
            exit 1
        fi
    fi

    # 4. Write shell environment
    echo ""
    echo "  Shell config: $rc_file"

    local root_line home_line
    if [ "$is_fish" = true ]; then
        root_line="set -gx DAISY_ROOT \"$DAISY_ROOT\""
        home_line="set -gx DAISY_HOME \"\$DAISY_ROOT/home/$selected_home\""
    else
        root_line="export DAISY_ROOT=\"$DAISY_ROOT\""
        home_line="export DAISY_HOME=\"\$DAISY_ROOT/home/$selected_home\""
    fi

    if [ -f "$rc_file" ] && grep -q 'DAISY_ROOT' "$rc_file" 2>/dev/null; then
        # Update existing DAISY_ROOT line
        local current_root
        if [ "$is_fish" = true ]; then
            current_root=$(grep 'DAISY_ROOT' "$rc_file" | grep -oE '"[^"]*"' | head -1 | tr -d '"')
        else
            current_root=$(grep 'DAISY_ROOT=' "$rc_file" | grep -oE '"[^"]*"' | head -1 | tr -d '"')
        fi

        if [ "$current_root" = "$DAISY_ROOT" ]; then
            echo "  ✓ DAISY_ROOT already set correctly"
        else
            if [ "$is_fish" = true ]; then
                sed -i.bak "s|set -gx DAISY_ROOT .*|$root_line|" "$rc_file"
            else
                sed -i.bak "s|export DAISY_ROOT=.*|$root_line|" "$rc_file"
            fi
            rm -f "${rc_file}.bak"
            echo "  ✓ Updated DAISY_ROOT (was: $current_root)"
        fi

        # Update DAISY_HOME line
        if [ -n "$selected_home" ]; then
            if grep -q 'DAISY_HOME' "$rc_file" 2>/dev/null; then
                if [ "$is_fish" = true ]; then
                    sed -i.bak "s|set -gx DAISY_HOME .*|$home_line|" "$rc_file"
                else
                    sed -i.bak "s|export DAISY_HOME=.*|$home_line|" "$rc_file"
                fi
                rm -f "${rc_file}.bak"
                echo "  ✓ Updated DAISY_HOME to: $selected_home"
            else
                echo "$home_line" >> "$rc_file"
                echo "  ✓ Added DAISY_HOME: $selected_home"
            fi
        fi
    else
        # Append fresh block
        {
            echo ""
            echo "# daisy"
            echo "$root_line"
            [ -n "$selected_home" ] && echo "$home_line"
        } >> "$rc_file"
        echo "  ✓ Added DAISY_ROOT and DAISY_HOME to $rc_file"
    fi

    echo ""
    echo "Done. To activate, run:"
    echo "  source $rc_file"
    echo ""
    echo "Then initialize Daisy in a workspace:"
    echo "  cd /path/to/project"
    echo "  daisy init ${selected_home:-<home>}"
}

# --- subcommand dispatch ---

shift 2>/dev/null || true

case "$COMMAND" in
    install)
        cmd_install "$@"
        ;;
    init)
        exec "$SCRIPTS/daisy-init.sh" "$@"
        ;;
    clean)
        cmd_clean "$@"
        ;;
    status)
        cmd_status "$@"
        ;;
    healthcheck)
        require_workspace
        "$SCRIPTS/healthcheck.sh" "$@"
        popd > /dev/null
        ;;
    log)
        require_workspace
        "$SCRIPTS/log.sh" "$*"
        popd > /dev/null
        ;;
    done)
        require_workspace
        "$SCRIPTS/done.sh" "$@"
        popd > /dev/null
        ;;
    new-day)
        require_workspace
        "$SCRIPTS/new-day.sh" "$@"
        popd > /dev/null
        ;;
    new-week)
        require_workspace
        "$SCRIPTS/new-week.sh" "$@"
        popd > /dev/null
        ;;
    build)
        if [ -n "$1" ]; then
            "$SCRIPTS/build-prompt.sh" "$@"
        else
            require_workspace
            "$SCRIPTS/build-prompt.sh" "$@"
            popd > /dev/null
        fi
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Unknown command: $COMMAND" >&2
        echo "" >&2
        show_help >&2
        exit 1
        ;;
esac
