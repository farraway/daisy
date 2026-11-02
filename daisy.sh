#!/usr/bin/env bash
# Daisy CLI - unified command-line interface for the Daisy productivity system.
#
# Usage:
#   daisy <command> [args...]
#
# The binary (bin/daisy) runs this for the commands it has not taken over.

set -e

COMMAND="${1:-help}"

if [ -z "$DAISY_ROOT" ]; then
    echo "Error: DAISY_ROOT not set. Run '<root>/bin/daisy install' first." >&2
    exit 1
fi

SCRIPTS="$DAISY_ROOT/daisy/scripts"
source "$SCRIPTS/common.sh"

# --- workspace root resolution ---

# Echoes the workspace root. Returns 1 when there is none, 2 when a level
# is a workspace that cannot be read (daisy_workspace_problem, which has
# already said why).
find_workspace_root() {
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        daisy_workspace_problem "$dir" && return 2
        if daisy_is_workspace_dir "$dir"; then
            echo "$dir"
            return 0
        fi
        dir=$(dirname "$dir")
    done
    return 1
}

require_workspace() {
    WORKSPACE_ROOT=$(find_workspace_root) || {
        if [ $? -eq 1 ]; then
            echo "Error: Not in a Daisy workspace (no .daisy/ directory found)" >&2
            echo "  Run 'daisy init <identity>' to initialize this workspace." >&2
        fi
        exit 1
    }
    pushd "$WORKSPACE_ROOT" > /dev/null
}

# --- built-in: help ---

show_help() {
    cat <<'EOF'
Usage: daisy <command> [args...]

Commands not yet in the binary (bin/daisy help lists every command):
  check-secrets                Check which secrets/tokens are configured
  commit --home|--not-home <message>
                                Commit changes, scoped to home data or everything else
  healthcheck [--force]        Run system health check
  test [name-substring]        Run the hermetic acceptance suite

  Every command also accepts --help/-h (as the sole argument) to print its usage.

Environment:
  DAISY_ROOT    Path to the daisy repository (required)
EOF
}

# --- built-in: per-command help ---
#
# `daisy <command> --help`/`-h` (as the sole argument, checked in the
# dispatch section below) for a command the binary has not taken over: the
# script's own header comment, else its show_help() line. `daisy help` and
# every Go command's help are the binary's.

show_command_help() {
    local cmd="$1"
    local script="${cmd}.sh"

    if [ -f "$SCRIPTS/$script" ]; then
        awk '
            NR==1 { next }
            /^#/ { sub(/^# ?/, ""); print; next }
            { exit }
        ' "$SCRIPTS/$script"
        return
    fi

    show_help | grep -E "^  ${cmd}[[:space:]]" || show_help
}

# --- built-in: status ---

cmd_status() {
    require_workspace
    require_env || { popd > /dev/null; exit 1; }

    echo "Daisy | home: $DAISY_HOME_NAME | identity: $DAISY_IDENTITY_NAME | workspace: $WORKSPACE_ROOT"
    daisy_report_drift
    resolve_projects
    echo "Projects: $(daisy_projects_summary)"
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
    status)
        cmd_status "$@"
        ;;
    healthcheck)
        require_workspace
        "$SCRIPTS/healthcheck.sh" "$@"
        popd > /dev/null
        ;;
    files)
        require_workspace
        "$SCRIPTS/files.sh" "$@"
        popd > /dev/null
        ;;
    check-secrets)
        require_workspace
        "$SCRIPTS/check-secrets.sh" "$@"
        popd > /dev/null
        ;;
    commit)
        require_workspace
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
