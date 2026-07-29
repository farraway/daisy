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
  build [home]                 Rebuild AGENTS.md for a home
  check-secrets                Check which secrets/tokens are configured
  clean [-f]                   Remove Daisy from the current workspace
  commit --home|--not-home <message>
                                Commit changes, scoped to home data or everything else
  done <pattern>               Mark a task as complete
  eval [<case>] [--record]     List, display, or record an eval case result
  feedback [--workflow <n>]    Record a prompt failure for optimization
  files                        Show resolved real paths for every home file
  healthcheck [--force]        Run system health check
  help                         Show this help
  init [--new] <home> [path]   Initialize Daisy in a workspace
  install [--update]           Set up daisy + root skills; --update re-runs
                                only symlink/skills/permissions, no prompts
  list                         List active prompts and installed skills
  log <message...>             Add a log entry to today.md
  new-day                      Start a new day
  new-week                     Start a new week
  optimize [--workflow <n>]    Run prompt learning loop on collected feedback
  plan-archive                 Archive the active Daisy plan
  plan-new [--spec] <desc>     Create a new Daisy plan (or unregistered spec draft)
  plan-pickup <path>           Promote a local spec draft into a tracked plan
  projects [--archived]        List active or archived projects with paths
  rotate                       Rotate journal.md into archive window files
  status                       Show quick workspace summary
  tasks [--all|--done|--todo] [pattern]
                                Search todo.txt/done.txt (read-only)
  test [name-substring]        Run the hermetic daisy/scripts test suite

  Every command also accepts --help/-h (as the sole argument) to print its usage.

Environment:
  DAISY_ROOT    Path to the daisy repository (required)
EOF
}

# --- built-in: per-command help ---
#
# Every `daisy <command>` supports `--help`/`-h` when passed as the sole
# argument (checked in the dispatch section below, before any command runs).
# Help text is read from each script's own header comment rather than
# authored separately here.

show_command_help() {
    local cmd="$1"
    local script=""

    case "$cmd" in
        build) script="build-prompt.sh" ;;
        init) script="daisy-init.sh" ;;
        status|clean|install|help) script="" ;;
        *) script="${cmd}.sh" ;;
    esac

    if [ -n "$script" ] && [ -f "$SCRIPTS/$script" ]; then
        awk '
            NR==1 { next }
            /^#/ { sub(/^# ?/, ""); print; next }
            { exit }
        ' "$SCRIPTS/$script"
        return
    fi

    # Built-ins with no standalone script: fall back to their show_help() line.
    show_help | grep -E "^  ${cmd}[[:space:]]" || show_help
}

# --- built-in: clean ---

cmd_clean() {
    local force=false
    if [ "$1" = "-f" ] || [ "$1" = "--force" ]; then
        force=true
    fi

    require_workspace
    source "$SCRIPTS/common.sh"

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

    # Discover this workspace's installed skill names the same way
    # daisy-init.sh does, so removal can't drift from installation.
    local skill_names=()
    while IFS= read -r -d '' skill_md; do
        skill_names+=("$(basename "$(dirname "$skill_md")")")
    done < <(find "$DAISY_ROOT/home/$home_name/skills" -name SKILL.md -print0 2>/dev/null)

    # 1. Remove .daisy/ directory
    if [ -d ".daisy" ]; then
        rm -rf ".daisy"
        echo "  ✓ Removed .daisy/"
    fi

    # 2. Remove Cursor rule files
    local cursor_rules_removed=false
    for rule_name in "${DAISY_CURSOR_RULE_FILES[@]}"; do
        if [ -f ".cursor/rules/$rule_name" ]; then
            rm -f ".cursor/rules/$rule_name"
            cursor_rules_removed=true
        fi
    done
    if [ "$cursor_rules_removed" = true ]; then
        echo "  ✓ Removed Cursor rules"
        rmdir ".cursor/rules" 2>/dev/null || true
        rmdir ".cursor" 2>/dev/null || true
    fi

    # 3. Remove Claude command
    if [ -f "$DAISY_CLAUDE_COMMAND_FILE" ]; then
        rm -f "$DAISY_CLAUDE_COMMAND_FILE"
        echo "  ✓ Removed Claude command"
        rmdir ".claude/commands" 2>/dev/null || true
    fi

    # 4. Remove installed skills
    if [ ${#skill_names[@]} -gt 0 ]; then
        for name in "${skill_names[@]}"; do
            rm -rf ".claude/skills/$name"
            rm -f ".cursor/rules/$name.mdc"
        done
        echo "  ✓ Removed ${#skill_names[@]} skill(s) from .claude/skills/ and .cursor/rules/"
        rmdir ".cursor/rules" 2>/dev/null || true
        rmdir ".cursor" 2>/dev/null || true
    fi

    # 5. Remove Daisy entries from .gitignore
    if [ -f ".gitignore" ]; then
        local gitignore_entries=("${DAISY_GITIGNORE_BASE_ENTRIES[@]}")
        for name in "${skill_names[@]}"; do
            gitignore_entries+=(".claude/skills/$name/" ".cursor/rules/$name.mdc")
        done
        local cleaned=false
        for line in "# Daisy workspace config (per-machine)" "${gitignore_entries[@]}"; do
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

    # 6. Remove Daisy permissions from .claude/settings.local.json
    if [ -f ".claude/settings.local.json" ] && command -v jq >/dev/null 2>&1; then
        # Current local rules (aligned with DAISY_ALLOW_BASE) + legacy rules for old workspaces
        daisy_rules_json=$(printf '%s\n' \
            "${DAISY_ALLOW_BASE[@]}" \
            "Write(.daisy/**)" \
            "Bash($DAISY_ROOT/daisy/scripts/*)" \
            "Read($DAISY_ROOT/**)" "Edit($DAISY_ROOT/**)" "Write($DAISY_ROOT/**)" \
            | jq -R . | jq -s .)
        updated=$(jq --argjson rules "$daisy_rules_json" '
            if .permissions.allow then
                .permissions.allow |= map(select(. as $r | $rules | index($r) | not))
            else . end
        ' ".claude/settings.local.json")
        remaining=$(echo "$updated" | jq '(.permissions.allow // []) | length')
        top_keys=$(echo "$updated" | jq 'keys | length')
        if [ "$remaining" -eq 0 ] && [ "$top_keys" -le 1 ]; then
            rm -f ".claude/settings.local.json"
            echo "  ✓ Removed .claude/settings.local.json"
        else
            echo "$updated" | jq '.' > ".claude/settings.local.json"
            echo "  ✓ Removed Daisy permissions from .claude/settings.local.json"
        fi
    fi

    # 7. Remove Daisy entries from .cursorignore
    if [ -f ".cursorignore" ]; then
        local cleaned=false
        for line in "# Allow Cursor to index daisy paths (gitignored but needed for agent context)" "${DAISY_CURSORIGNORE_BASE_ENTRIES[@]}"; do
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

    # 8. Unset local git identity if it matches the home's gitconfig
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

    # Show the active plan if PLAN.md is a symlink to a tracked Daisy plan
    if [ -L "PLAN.md" ]; then
        local plan_path plan_title
        plan_path=$(readlink -f "PLAN.md")
        plan_title=$(head -1 "$plan_path" | sed 's/^#\+ *//')
        echo ""
        echo "Plan:   $plan_title"
        echo "        $plan_path"
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
    # --update re-runs only the non-interactive steps (symlink, skills,
    # permissions) — for repeat runs after a root skill changes. Plain
    # `install` is the one-time interactive bootstrap (also selects a
    # default home and writes shell rc) and assumes a real terminal.
    local update=false
    [ "$1" = "--update" ] && update=true

    local rc_file
    rc_file=$(detect_shell_rc)
    local is_fish=false
    [ "$(basename "$SHELL")" = "fish" ] && is_fish=true

    if [ "$update" = false ] && [ -L "$HOME/bin/daisy" ]; then
        local existing_target
        existing_target=$(readlink "$HOME/bin/daisy")
        if [ "$existing_target" = "$DAISY_ROOT/daisy.sh" ]; then
            echo "Error: Daisy is already installed." >&2
            echo "Run 'daisy install --update' to re-apply symlink/skills/permissions." >&2
            exit 1
        fi
    fi

    if [ "$update" = true ]; then
        echo "Updating Daisy..."
    else
        echo "Installing Daisy..."
    fi
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

    # 3. Install root skills
    local skills_claude_dir="$HOME/.claude/skills"
    mkdir -p "$skills_claude_dir"
    local skill_count=0
    local source_names=()
    while IFS= read -r -d '' skill_md; do
        local skill_dir skill_name
        skill_dir="$(dirname "$skill_md")"
        skill_name="$(basename "$skill_dir")"
        source_names+=("$skill_name")
        rm -rf "${skills_claude_dir:?}/$skill_name"
        cp -r "$skill_dir" "$skills_claude_dir/$skill_name"
        skill_count=$((skill_count + 1))
    done < <(find "$DAISY_ROOT/skills" -name SKILL.md -print0 2>/dev/null)
    echo "  ✓ Installed $skill_count skill(s) into $skills_claude_dir"

    # Prune skills previously installed here that no longer exist in
    # $DAISY_ROOT/skills — otherwise a deleted source skill's copy lingers
    # in $skills_claude_dir forever.
    local pruned_count=0
    for installed_dir in "$skills_claude_dir"/*/; do
        [ -d "$installed_dir" ] || continue
        local installed_name found
        installed_name="$(basename "$installed_dir")"
        found=false
        for name in "${source_names[@]}"; do
            if [ "$name" = "$installed_name" ]; then
                found=true
                break
            fi
        done
        if [ "$found" = false ]; then
            rm -rf "${skills_claude_dir:?}/$installed_name"
            pruned_count=$((pruned_count + 1))
        fi
    done
    if [ "$pruned_count" -gt 0 ]; then
        echo "  ✓ Pruned $pruned_count stale skill(s) from $skills_claude_dir"
    fi

    local selected_home=""
    if [ "$update" = false ]; then
        # 4. Select default home
        local homes=()
        for dir in "$DAISY_ROOT"/home/*/; do
            [ -d "$dir" ] && homes+=("$(basename "$dir")")
        done

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

        # 5. Write shell environment
        echo ""
        echo "  Shell config: $rc_file"

        local root_line home_line
        if [ "$is_fish" = true ]; then
            root_line="set -gx DAISY_ROOT \"$DAISY_ROOT\""
            home_line="set -gx DAISY_DEFAULT_HOME \"$selected_home\""
        else
            root_line="export DAISY_ROOT=\"$DAISY_ROOT\""
            home_line="export DAISY_DEFAULT_HOME=\"$selected_home\""
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

            # Update DAISY_DEFAULT_HOME line
            if [ -n "$selected_home" ]; then
                if grep -q 'DAISY_DEFAULT_HOME' "$rc_file" 2>/dev/null; then
                    if [ "$is_fish" = true ]; then
                        sed -i.bak "s|set -gx DAISY_DEFAULT_HOME .*|$home_line|" "$rc_file"
                    else
                        sed -i.bak "s|export DAISY_DEFAULT_HOME=.*|$home_line|" "$rc_file"
                    fi
                    rm -f "${rc_file}.bak"
                    echo "  ✓ Updated DAISY_DEFAULT_HOME to: $selected_home"
                else
                    echo "$home_line" >> "$rc_file"
                    echo "  ✓ Added DAISY_DEFAULT_HOME: $selected_home"
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
            echo "  ✓ Added DAISY_ROOT and DAISY_DEFAULT_HOME to $rc_file"
        fi
    fi

    # 6. Claude Code global permissions
    local global_claude_settings="$HOME/.claude/settings.json"
    local daisy_global_allow=(
        "Read(/$DAISY_ROOT/**)"
        "Edit(/$DAISY_ROOT/**)"
        "Bash(daisy:*)"
    )

    if command -v jq >/dev/null 2>&1; then
        mkdir -p "$(dirname "$global_claude_settings")"
        local existing='{}'
        [ -f "$global_claude_settings" ] && existing=$(cat "$global_claude_settings")
        local updated="$existing"
        # Heal existing installs: remove the legacy raw-scripts grant superseded by Bash(daisy:*),
        # and the single-slash Read/Edit rules superseded by the //-absolute forms above. A
        # single leading slash anchors at the settings file's own directory (~/.claude/ for user
        # settings), not the filesystem root, so those old rules silently matched nothing.
        for legacy in \
            "Bash($DAISY_ROOT/daisy/scripts/*)" \
            "Read($DAISY_ROOT/**)" \
            "Edit($DAISY_ROOT/**)" \
            "Read(//$DAISY_ROOT/**)" \
            "Edit(//$DAISY_ROOT/**)"
        do
            updated=$(echo "$updated" | jq --arg r "$legacy" '
                if .permissions.allow then
                    .permissions.allow |= map(select(. != $r))
                else . end
            ')
        done
        for rule in "${daisy_global_allow[@]}"; do
            updated=$(echo "$updated" | jq \
                --arg r "$rule" \
                '.permissions.allow //= [] | if (.permissions.allow | index($r)) == null then .permissions.allow += [$r] else . end')
        done
        echo "$updated" | jq '.' > "$global_claude_settings"
        echo "  ✓ Configured ~/.claude/settings.json with Daisy permissions"
    else
        echo "  ⚠ jq not available — skipping Claude Code permissions (add manually)"
    fi

    echo ""
    if [ "$update" = true ]; then
        echo "Done."
    else
        echo "Done. To activate, run:"
        echo "  source $rc_file"
        echo ""
        echo "Then initialize Daisy in a workspace:"
        echo "  cd /path/to/project"
        echo "  daisy init ${selected_home:-<home>}"
    fi
}

# --- subcommand dispatch ---

shift 2>/dev/null || true

# --help/-h as the sole remaining argument works uniformly for every command,
# without ever invoking it for real. Deliberately not a scan across all
# arguments — that would misfire on legitimate free-text content (e.g. a log
# message that happens to mention "--help").
if [ "$#" -eq 1 ] && { [ "$1" = "--help" ] || [ "$1" = "-h" ]; }; then
    show_command_help "$COMMAND"
    exit 0
fi

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
            "$SCRIPTS/build-prompt.sh" --output "$WORKSPACE_ROOT/.daisy/AGENTS.md"
            popd > /dev/null
        fi
        ;;
    feedback)
        require_workspace
        "$SCRIPTS/feedback.sh" "$@"
        popd > /dev/null
        ;;
    optimize)
        require_workspace
        "$SCRIPTS/optimize.sh" "$@"
        popd > /dev/null
        ;;
    eval)
        require_workspace
        "$SCRIPTS/eval.sh" "$@"
        popd > /dev/null
        ;;
    files)
        require_workspace
        "$SCRIPTS/files.sh" "$@"
        popd > /dev/null
        ;;
    list)
        require_workspace
        "$SCRIPTS/list.sh" "$@"
        popd > /dev/null
        ;;
    projects)
        require_workspace
        "$SCRIPTS/projects.sh" "$@"
        popd > /dev/null
        ;;
    tasks)
        require_workspace
        "$SCRIPTS/tasks.sh" "$@"
        popd > /dev/null
        ;;
    rotate)
        require_workspace
        "$SCRIPTS/rotate.sh" "$@"
        popd > /dev/null
        ;;
    check-secrets)
        require_workspace
        "$SCRIPTS/check-secrets.sh" "$@"
        popd > /dev/null
        ;;
    plan-new)
        require_workspace
        "$SCRIPTS/plan-new.sh" "$@"
        popd > /dev/null
        ;;
    plan-pickup)
        require_workspace
        "$SCRIPTS/plan-pickup.sh" "$@"
        popd > /dev/null
        ;;
    plan-archive)
        require_workspace
        "$SCRIPTS/plan-archive.sh" "$@"
        popd > /dev/null
        ;;
    commit)
        require_workspace
        source "$SCRIPTS/common.sh"
        require_env || { popd > /dev/null; exit 1; }
        "$SCRIPTS/commit.sh" "$@"
        popd > /dev/null
        ;;
    test)
        require_workspace
        "$SCRIPTS/test.sh" "$@"
        popd > /dev/null
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
