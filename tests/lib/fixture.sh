#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh and test cases — not executed directly.
# Builds and tears down a hermetic fixture for one test case: its own
# DAISY_ROOT (a temp dir with the real scripts, templates and tests symlinked
# in and the prompts/skills trees copied, its own git repo), its own home (a fresh
# testhome outside the root, its own git repo), its own homes registry
# (DAISY_CONFIG) and data dir (DAISY_LOCAL), plus its own workspace dir. No
# test may reach a real home — see feedback.md (2026-07-17). Every case
# drives Daisy through `daisy` on PATH (fixture_run, fixture_run_cli), never
# a script.

REAL_DAISY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# For the registry writers: the fixture registers homes the way production
# does, so the two cannot drift apart. They read DAISY_CONFIG, which
# fixture_setup exports before the first call.
source "$REAL_DAISY_ROOT/daisy/scripts/common.sh"

fixture_home_dir() {
    echo "$FIXTURE_HOME"
}

# Registers a home name in the fixture registry. The directory need not
# exist: the registry is what makes a name a home, and the publication
# hygiene scan derives its terms from registered names.
fixture_register_home() {
    daisy_register_home "$1" "${2:-$FIXTURE_LOCAL/homes/$1}"
}

fixture_unregister_home() {
    daisy_unregister_home "$1"
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

    # `daisy.sh`, `daisy` (the scripts), `templates` and `tests` are
    # symlinked: they are the code under test, which nothing writes, and
    # copying `tests` would copy this suite into itself. The binary on PATH
    # execs $DAISY_ROOT/daisy.sh for every command it has not taken over, so
    # the fixture root must carry the entry point. `prompts` and `skills` are *copied*:
    # symlinked, DAISY_ROOT/prompts was literally the repo's, so any applying
    # case whose workflow resolved to a shared prompt rewrote the real file.
    # Cases avoided that by convention alone, and the unknown-workflow
    # fallback routes anything unrecognised to prompts/daisy.md. 148K and
    # ~4ms per fixture, against a git init already paid for here.
    ln -s "$REAL_DAISY_ROOT/daisy.sh" "$FIXTURE_ROOT/daisy.sh"
    ln -s "$REAL_DAISY_ROOT/daisy" "$FIXTURE_ROOT/daisy"
    ln -s "$REAL_DAISY_ROOT/templates" "$FIXTURE_ROOT/templates"
    ln -s "$REAL_DAISY_ROOT/tests" "$FIXTURE_ROOT/tests"
    cp -r "$REAL_DAISY_ROOT/prompts" "$FIXTURE_ROOT/prompts"
    cp -r "$REAL_DAISY_ROOT/skills" "$FIXTURE_ROOT/skills"

    # Not "Daisy Test": healthcheck's publication-hygiene scan derives its
    # search terms from the repo's git identity and matches them
    # case-insensitively, so that name collides with the literal command
    # `daisy test` in prompts/plan.md. Invisible while the shared trees were
    # symlinked (publishable_files skipped them); a real failure once they
    # are copied. The identity must not be Daisy vocabulary.
    git init -q "$FIXTURE_ROOT"
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

    # No case may reach the real API, and the suite must not depend on the
    # developer's environment to enforce that. A case that meant to stub the
    # LLM and did not (a mistyped helper name is silent under run.sh's
    # `set -u`) once ran against api.anthropic.com on an exported key and
    # still passed. Blank by default; fixture_stub_llm_response is the only
    # supported way past ensure_llm_deps.
    FIXTURE_SAVED_API_KEY="${ANTHROPIC_API_KEY:-}"
    export ANTHROPIC_API_KEY=""
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
# .daisy/home and the six links — so the binding check in status/files has
# nothing to report, as in a real workspace.
fixture_bind_workspace() {
    local dir="$1" link target
    mkdir -p "$dir/.daisy"
    echo "testhome" > "$dir/.daisy/home"
    while IFS=$'\t' read -r link target; do
        ln -sfn "$FIXTURE_HOME/$target" "$dir/.daisy/$link"
    done < <(daisy_workspace_links)
}

fixture_write_project_binding() {
    cat > "${FIXTURE_CWD:-$FIXTURE_WORKSPACE}/.daisy/project"
}

fixture_add_project() {
    printf '# %s\n' "$1" > "$(fixture_home_dir)/projects/$1.md"
}

fixture_teardown() {
    rm -rf "$FIXTURE_ROOT" "$FIXTURE_HOME_PARENT" "$FIXTURE_CONFIG" "$FIXTURE_LOCAL" "$FIXTURE_WORKSPACE"
    unset DAISY_CONFIG DAISY_LOCAL
    fixture_teardown_home_override
    fixture_teardown_llm_stub
}

# Points optimize's LLM seam at a canned response carrying the given rules
# text, so a case can drive the write path with no API call. The stub file
# lives in FIXTURE_ROOT and dies with it.
fixture_stub_llm_response() {
    fixture_stub_llm_raw "$(jq -n --arg text "$1" '{content: [{type: "text", text: $text}]}')"
}

# Stubs the seam with a verbatim response body — for the branches that read
# the envelope rather than the completion: an `.error` payload, or a
# `.content[0].text` that is empty or absent.
fixture_stub_llm_raw() {
    printf '%s\n' "$1" > "$FIXTURE_ROOT/llm-response.json"
    export DAISY_OPTIMIZE_RESPONSE_FILE="$FIXTURE_ROOT/llm-response.json"
}

# run.sh sources every case into one shell, so an exported stub would leak
# into the next case; the key is restored to whatever the developer had.
fixture_teardown_llm_stub() {
    unset DAISY_OPTIMIZE_RESPONSE_FILE
    export ANTHROPIC_API_KEY="${FIXTURE_SAVED_API_KEY:-}"
    unset FIXTURE_SAVED_API_KEY
}

# The `daisy` command words that stand for each script the cases were
# written against — the one place the suite knows a script's name. A rename
# (the Renames table in docs/cli.md) changes the entry here, not the cases;
# the words are split at the call site so an entry may be several.
fixture_command() {
    case "$1" in
        daisy-init)   echo "init" ;;
        build-prompt) echo "build" ;;
        check-secrets|commit|done|eval|feedback|files|healthcheck|home|list|log|\
        new-day|new-week|optimize|plan-archive|plan-new|plan-pickup|projects|\
        rotate|tasks|test)
            echo "$1" ;;
        *)  echo "fixture_command: no daisy command stands for script '$1'" >&2
            return 1 ;;
    esac
}

# Runs `daisy "$@"` (the installed entry point) inside the fixture
# workspace, capturing stdout/stderr/exit code into RUN_STDOUT/RUN_STDERR/
# RUN_EXIT. HOME is the override when a case set one (install, healthcheck,
# init read and write $HOME/.claude/**). stdin is inherited, so a caller may
# feed one with a here-string.
fixture_run_cli() {
    local err_file
    err_file=$(mktemp)
    RUN_STDOUT=$(cd "${FIXTURE_CWD:-$FIXTURE_WORKSPACE}" && HOME="${FIXTURE_HOME_OVERRIDE:-$HOME}" daisy "$@" 2>"$err_file")
    RUN_EXIT=$?
    RUN_STDERR=$(cat "$err_file")
    rm -f "$err_file"
}

# Runs the daisy command that stands for a script, by name: `fixture_run done
# <pattern>` is `daisy done <pattern>`. An unknown name is a fault in the
# case, not a failing assertion, so it stops the run (after the teardown
# run.sh would otherwise have done).
fixture_run() {
    local script="$1"; shift
    local words
    words=$(fixture_command "$script") || { fixture_teardown; exit 1; }
    # shellcheck disable=SC2086  # the words are meant to split
    fixture_run_cli $words "$@"
}

# Like fixture_run, but supplies stdin — for optimize's approval prompt, a
# bare `read -rp` that no case could answer, which is why none of its three
# branches had ever been executed.
fixture_run_with_input() {
    local input="$1"; shift
    fixture_run "$@" <<<"$input"
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

# Runs a fixture command with no ANTHROPIC_API_KEY, restoring it afterwards.
# For optimize cases proving the paths that reach no LLM (partitioning,
# below-floor groups): the suite must stay hermetic, and a case that would
# call the API if the developer happens to have a key exported is not.
fixture_run_without_api_key() {
    local saved="${ANTHROPIC_API_KEY:-}"
    export ANTHROPIC_API_KEY=""
    fixture_run "$@"
    export ANTHROPIC_API_KEY="$saved"
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
