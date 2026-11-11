#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh and test cases — not executed directly.
# Builds and tears down a hermetic fixture for one test case: its own
# DAISY_ROOT (a temp dir with the real templates, prompts, skills and
# AGENTS.md copied in and tests symlinked, its own git repo), its own home (a
# fresh testhome outside the root, its own git repo), its own homes registry
# (DAISY_CONFIG) and data dir (DAISY_LOCAL), plus its own workspace dir. No
# test may reach a real home — see feedback.md (2026-07-17). Every case
# drives Daisy through `daisy` on PATH (fixture_run_cli), the binary under
# test; nothing under the root but tests/ is bash.

REAL_DAISY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

fixture_home_dir() {
    echo "$FIXTURE_HOME"
}

# Registers a home name in the fixture registry ($DAISY_CONFIG/homes, one
# `name=path` per line). The directory need not exist: the registry is what
# makes a name a home, and the publication hygiene scan derives its terms
# from registered names — which is why this writes the line itself rather
# than going through `daisy home add`, which refuses a path that is not a home.
fixture_register_home() {
    local file="$DAISY_CONFIG/homes"
    mkdir -p "$DAISY_CONFIG"
    if [ -s "$file" ] && [ -n "$(tail -c1 "$file")" ]; then echo >> "$file"; fi
    printf '%s=%s\n' "$1" "${2:-$FIXTURE_LOCAL/homes/$1}" >> "$file"
}

# Drops every registry line of a name, as `daisy home remove` does.
fixture_unregister_home() {
    local file="$DAISY_CONFIG/homes" kept
    kept=$(awk -F= -v name="$1" '{ key = $1; gsub(/^[[:space:]]+|[[:space:]]+$/, "", key) } key != name' "$file")
    printf '%s\n' "$kept" > "$file"
}

# Sets up FIXTURE_ROOT, FIXTURE_HOME, FIXTURE_CONFIG, FIXTURE_LOCAL,
# FIXTURE_WORKSPACE, and exports DAISY_ROOT, DAISY_CONFIG and DAISY_LOCAL so
# that `daisy` on PATH resolves inside the fixture.
fixture_setup() {
    FIXTURE_ROOT=$(mktemp -d)
    FIXTURE_HOME_PARENT=$(mktemp -d)
    FIXTURE_HOME="$FIXTURE_HOME_PARENT/testhome"
    FIXTURE_CONFIG=$(mktemp -d)
    FIXTURE_LOCAL=$(mktemp -d)
    FIXTURE_WORKSPACE=$(mktemp -d)

    # Every variable the binary and git read from the machine is pinned
    # (praxis rule 14): HOME is a fixture directory (a case may set its own
    # override), USER a name in no file, the cache's TTL the default, no
    # GIT_* variable from outside, and git reads no global or system config
    # (a signing or hook setting there would change what a commit does).
    FIXTURE_USER_HOME=$(mktemp -d)
    local v
    for v in $(compgen -e); do
        case "$v" in GIT_*) unset "$v" ;; esac
    done
    export HOME="$FIXTURE_USER_HOME" USER=fixtureuser GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
    unset DAISY_HEALTHCHECK_TTL XDG_CONFIG_HOME

    # `prompts`, `skills`, `templates` and `AGENTS.md` (whose Command
    # Reference the health check compares with the cli table) are *copied*:
    # symlinked, DAISY_ROOT/prompts was literally the repo's, so any applying
    # case whose workflow resolved to a shared prompt rewrote the real file
    # (case 65). `tests` is symlinked — copying it would copy this suite into
    # itself — for the `test` component's runner; its link text is the real
    # root's path, under $HOME, which the hygiene scan reads as published,
    # so the fixture's own .git/info/exclude keeps that plumbing off the
    # surface (not .gitignore, which cases rewrite).
    cp -r "$REAL_DAISY_ROOT/templates" "$FIXTURE_ROOT/templates"
    cp -r "$REAL_DAISY_ROOT/prompts" "$FIXTURE_ROOT/prompts"
    cp -r "$REAL_DAISY_ROOT/skills" "$FIXTURE_ROOT/skills"
    cp "$REAL_DAISY_ROOT/AGENTS.md" "$FIXTURE_ROOT/AGENTS.md"

    # Not "Daisy Test": healthcheck's publication-hygiene scan derives its
    # search terms from the repo's git identity and matches them
    # case-insensitively, so that name collides with the literal command
    # `daisy test` in prompts/plan.md. Invisible while the shared trees were
    # symlinked (the scan skipped them); a real failure once they
    # are copied. The identity must not be Daisy vocabulary.
    git init -q "$FIXTURE_ROOT"
    echo "/tests" >> "$FIXTURE_ROOT/.git/info/exclude"
    ln -s "$REAL_DAISY_ROOT/tests" "$FIXTURE_ROOT/tests"
    git -C "$FIXTURE_ROOT" config user.name "Fixture Author"
    git -C "$FIXTURE_ROOT" config user.email "fixture@example.invalid"
    git -C "$FIXTURE_ROOT" add -A
    git -C "$FIXTURE_ROOT" commit -q -m "fixture: initial root"

    # The home is its own repository outside the root, as a real one is, and
    # has a default identity (the template ships none), as a bound one does.
    cp -r "$REAL_DAISY_ROOT/templates/home" "$FIXTURE_HOME"
    printf 'name=Fixture Author\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"
    git init -q "$FIXTURE_HOME"
    git -C "$FIXTURE_HOME" config user.name "Fixture Author"
    git -C "$FIXTURE_HOME" config user.email "fixture@example.invalid"
    git -C "$FIXTURE_HOME" add -A
    git -C "$FIXTURE_HOME" commit -q -m "fixture: initial home"

    export DAISY_CONFIG="$FIXTURE_CONFIG"
    export DAISY_LOCAL="$FIXTURE_LOCAL"
    fixture_register_home testhome "$FIXTURE_HOME"

    fixture_bind_workspace "$FIXTURE_WORKSPACE"

    export DAISY_ROOT="$FIXTURE_ROOT"
    unset DAISY_HEALTHCHECK_PASSED
    FIXTURE_CWD=""
}

# Creates a second workspace under FIXTURE_WORKSPACE with a chosen directory
# name, bound to the same fixture home, and points subsequent fixture_run
# calls at it. For cases where the *name* is the input — project resolution
# derives its default from the basename before the first dot, which a
# mktemp -d workspace cannot exercise.
fixture_workspace_named() {
    local name="$1"
    local dir="$FIXTURE_WORKSPACE/$name"
    fixture_bind_workspace "$dir"
    FIXTURE_CWD="$dir"
}

# Binds a directory to the fixture home the way `daisy init` leaves it:
# .daisy/home and the six links (env.Links in the binary) — so the binding
# check in status/files has nothing to report, as in a real workspace.
fixture_bind_workspace() {
    local dir="$1" pair
    mkdir -p "$dir/.daisy"
    echo "testhome" > "$dir/.daisy/home"
    for pair in tasks:tasks today.md:journal/today.md journal.md:journal/journal.md \
        projects:projects plans:plans feedback.md:feedback/feedback.md; do
        ln -sfn "$FIXTURE_HOME/${pair#*:}" "$dir/.daisy/${pair%%:*}"
    done
}

fixture_write_project_binding() {
    cat > "${FIXTURE_CWD:-$FIXTURE_WORKSPACE}/.daisy/project"
}

fixture_add_project() {
    printf '# %s\n' "$1" > "$(fixture_home_dir)/projects/$1.md"
}

fixture_teardown() {
    rm -rf "$FIXTURE_ROOT" "$FIXTURE_HOME_PARENT" "$FIXTURE_CONFIG" "$FIXTURE_LOCAL" "$FIXTURE_WORKSPACE" "$FIXTURE_USER_HOME"
    unset DAISY_CONFIG DAISY_LOCAL
    fixture_teardown_home_override
}

# Runs `daisy "$@"` (the installed entry point) inside the fixture
# workspace, capturing stdout/stderr/exit code into RUN_STDOUT/RUN_STDERR/
# RUN_EXIT. HOME is the override when a case set one (install, healthcheck,
# init read and write $HOME/.claude/**), else the fixture's own. stdin is inherited, so a caller may
# feed one with a here-string.
fixture_run_cli() {
    local err_file
    err_file=$(mktemp)
    RUN_STDOUT=$(cd "${FIXTURE_CWD:-$FIXTURE_WORKSPACE}" && HOME="${FIXTURE_HOME_OVERRIDE:-$FIXTURE_USER_HOME}" daisy "$@" 2>"$err_file")
    RUN_EXIT=$?
    RUN_STDERR=$(cat "$err_file")
    rm -f "$err_file"
}

# Like fixture_run_cli, but supplies stdin — for a command that asks
# (`optimize apply`'s approval, `home new`'s questions).
fixture_run_cli_with_input() {
    local input="$1"; shift
    fixture_run_cli "$@" <<<"$input"
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

fixture_write_feedback() {
    cat > "$(fixture_home_dir)/feedback/feedback.md"
}

fixture_read_feedback() {
    cat "$(fixture_home_dir)/feedback/feedback.md"
}

fixture_read_archive() {
    cat "$(fixture_home_dir)/feedback/archive.md"
}

# Writes stdin as the home's own prompt <name>.md, the target an optimize
# case drives: a home file, so applying it commits the home.
fixture_write_home_prompt() {
    mkdir -p "$(fixture_home_dir)/prompts"
    cat > "$(fixture_home_dir)/prompts/$1.md"
}

# Writes stdin as a proposal file in the workspace, the text the agent hands
# `optimize apply`; echoes its path.
fixture_write_proposal() {
    cat > "$FIXTURE_WORKSPACE/$1"
    echo "$FIXTURE_WORKSPACE/$1"
}

# Commits in the fixture root's repository: an applying run must leave it as
# it was (a root target is left for `daisy commit`, never committed).
fixture_root_commit_count() {
    git -C "$FIXTURE_ROOT" log --oneline | wc -l | tr -d ' '
}

fixture_write_done() {
    cat > "$(fixture_home_dir)/tasks/done.txt"
}

fixture_write_today() {
    cat > "$(fixture_home_dir)/journal/today.md"
}

# Dates today.md's `### YYYY-MM-DD Day` heading today, so a `daisy log`
# on the real clock finds the day started and does not quick-start one.
fixture_date_today() {
    local f
    f="$(fixture_home_dir)/journal/today.md"
    awk -v h="### $(date +%F) $(date +%A)" '!done && /^### [0-9]{4}-[0-9]{2}-[0-9]{2} /{$0 = h; done = 1} 1' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
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

# Commits in the home's repository — where every Daisy data mutation lands.
fixture_commit_count() {
    git -C "$FIXTURE_HOME" log --oneline | wc -l | tr -d ' '
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
# a `daisy done` resolution against todo.txt can key the today.md edit by exact
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
# +tags intact, as `daisy done` produces them.
seed_done_basic() {
    fixture_write_done <<'EOF'
x 2026-07-01 2026-06-01 Ship the old widget +daisy
x 2026-07-02 2026-06-05 Unrelated finished chore +work
EOF
}
