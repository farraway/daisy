#!/usr/bin/env bash
# Invocation: sourced by daisy/tests/run.sh and test cases — not executed directly.
# Builds and tears down a hermetic fixture for one test case: its own
# DAISY_ROOT (a temp dir with the real daisy/prompts/skills trees symlinked
# in, its own git repo, and a fresh testhome) plus its own workspace dir. No
# test may reach a real home under the real DAISY_ROOT — see feedback.md
# (2026-07-17).

REAL_DAISY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

fixture_home_dir() {
    echo "$FIXTURE_ROOT/home/testhome"
}

# Sets up FIXTURE_ROOT, FIXTURE_WORKSPACE, and exports DAISY_ROOT so that
# `daisy` on PATH (and every daisy/scripts/*.sh) resolves inside the fixture.
fixture_setup() {
    FIXTURE_ROOT=$(mktemp -d)
    FIXTURE_WORKSPACE=$(mktemp -d)

    ln -s "$REAL_DAISY_ROOT/daisy" "$FIXTURE_ROOT/daisy"
    ln -s "$REAL_DAISY_ROOT/prompts" "$FIXTURE_ROOT/prompts"
    ln -s "$REAL_DAISY_ROOT/skills" "$FIXTURE_ROOT/skills"

    mkdir -p "$FIXTURE_ROOT/home"
    cp -r "$REAL_DAISY_ROOT/daisy/templates/home" "$FIXTURE_ROOT/home/testhome"

    git init -q "$FIXTURE_ROOT"
    git -C "$FIXTURE_ROOT" config user.name "Daisy Test"
    git -C "$FIXTURE_ROOT" config user.email "test@daisy.invalid"
    git -C "$FIXTURE_ROOT" add -A
    git -C "$FIXTURE_ROOT" commit -q -m "fixture: initial home"

    mkdir -p "$FIXTURE_WORKSPACE/.daisy"
    echo "testhome" > "$FIXTURE_WORKSPACE/.daisy/home"

    export DAISY_ROOT="$FIXTURE_ROOT"
    unset DAISY_HEALTHCHECK_PASSED
}

fixture_teardown() {
    rm -rf "$FIXTURE_ROOT" "$FIXTURE_WORKSPACE"
    fixture_teardown_home_override
}

# Runs a daisy/scripts/<name>.sh inside the fixture workspace, capturing
# stdout/stderr/exit code into RUN_STDOUT/RUN_STDERR/RUN_EXIT.
fixture_run() {
    local script="$1"; shift
    local err_file
    err_file=$(mktemp)
    RUN_STDOUT=$(cd "$FIXTURE_WORKSPACE" && "$DAISY_ROOT/daisy/scripts/$script.sh" "$@" 2>"$err_file")
    RUN_EXIT=$?
    RUN_STDERR=$(cat "$err_file")
    rm -f "$err_file"
}

# Runs `daisy <command> ...` (the installed CLI) inside the fixture
# workspace — for cases that must prove the dispatch layer, not just the
# script directly.
fixture_run_cli() {
    local err_file
    err_file=$(mktemp)
    RUN_STDOUT=$(cd "$FIXTURE_WORKSPACE" && HOME="${FIXTURE_HOME_OVERRIDE:-$HOME}" daisy "$@" 2>"$err_file")
    RUN_EXIT=$?
    RUN_STDERR=$(cat "$err_file")
    rm -f "$err_file"
}

# Sets FIXTURE_HOME_OVERRIDE to a fresh temp dir and exports it as HOME for
# subsequent fixture_run_cli calls — for cases (install, healthcheck, init)
# that read/write $HOME/.claude/** and must never touch the real one.
fixture_setup_home_override() {
    FIXTURE_HOME_OVERRIDE=$(mktemp -d)
}

fixture_teardown_home_override() {
    [ -n "${FIXTURE_HOME_OVERRIDE:-}" ] && rm -rf "$FIXTURE_HOME_OVERRIDE"
    unset FIXTURE_HOME_OVERRIDE
}

fixture_write_todo() {
    cat > "$(fixture_home_dir)/tasks/todo.txt"
}

fixture_write_done() {
    cat > "$(fixture_home_dir)/tasks/done.txt"
}

fixture_write_today() {
    cat > "$(fixture_home_dir)/journal/today.md"
}

fixture_read_todo() {
    cat "$(fixture_home_dir)/tasks/todo.txt"
}

fixture_read_done() {
    cat "$(fixture_home_dir)/tasks/done.txt"
}

fixture_read_today() {
    cat "$(fixture_home_dir)/journal/today.md"
}

# Text up to (excluding) "#### Log" — for asserting the Tasks section is
# untouched aside from the intended checkbox, independent of the new log
# entry `daisy log` appends on every completion.
strip_log_section() {
    awk '/^#### Log/{exit} {print}'
}

fixture_commit_count() {
    git -C "$FIXTURE_ROOT" log --oneline | wc -l | tr -d ' '
}

# Standard today.md matching the reproduction in PLAN.md's Research section:
# Now: Alpha/Beta, Next: Gamma/"Delta target task"/Epsilon. The target
# pattern "Delta target" sits mid-Next with tasks above it in both Now and
# Next — exactly the range the old sed 0,/pattern/ bug swept.
seed_today_basic() {
    fixture_write_today <<'EOF'
### 2026-07-28 Tuesday

#### Agenda
- 0900 Plan Day

#### Tasks

**Now:**
- [ ] Alpha task +daisy
- [ ] Beta task +work

**Next:**
- [ ] Gamma task +daisy
- [ ] Delta target task +daisy
- [ ] Epsilon task


**Inbox:**
- [ ] Review weekly resolutions
- [ ] Check calendar for upcoming events
- [ ] Check that todo.txt is up-to-date
- [ ] Plan day
- [ ] Workout
- [ ] Retrospective


**GitHub PRs:**


#### Log

- 0900 New day started

#### Retrospective

* **Successes:**
* **Misses:**
* **What would a Sage do next:**
EOF
}

# Matching todo.txt for seed_today_basic — same descriptions verbatim so a
# done.sh resolution against todo.txt can key the today.md edit by exact
# whole-line match.
seed_todo_basic() {
    fixture_write_todo <<'EOF'
(B) 2026-07-27 Alpha task +daisy
(B) 2026-07-27 Beta task +work
(C) 2026-07-24 Gamma task +daisy
(B) 2026-07-27 Delta target task +daisy
(D) 2026-07-20 Epsilon task
EOF
}

seed_basic() {
    seed_today_basic
    seed_todo_basic
}

# A small done.txt for `daisy tasks` cases — historical completions with
# +tags intact, as done.sh (post-fix) produces them.
seed_done_basic() {
    fixture_write_done <<'EOF'
x 2026-07-01 2026-06-01 Ship the old widget +daisy
x 2026-07-02 2026-06-05 Unrelated finished chore +work
EOF
}
