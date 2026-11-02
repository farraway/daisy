# Test Plan #67: a workflow tag names the file its guidance belongs in, and
# `daisy feedback` refuses one that names nothing.
#
# The tag was unvalidated free text: feedback.sh wrote whatever it was given
# and the resolver guessed afterwards, from a string nothing had constrained.
# That is how `daisy-write`, `daisy-scripts` and the literal
# `new-day, new-week` reached the archive, each training daisy.md by accident
# through an "Unknown workflow" fallback.
#
# It is also how `git` and `github` were conflated: the alias table was
# consulted *before* the name lookups, so a `github|git)` arm swallowed `git`
# and skills/git was unreachable. The briefing's Target line is where a tag's
# resolution shows.

target_of() {
    fixture_write_feedback <<FB
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** $1

An entry.
FB
    fixture_run_cli optimize --workflow "$1"
    if [ "$RUN_EXIT" != 0 ]; then echo "exit $RUN_EXIT"; return; fi
    printf '%s\n' "$RUN_STDOUT" | sed -n 's/^Target: //p' | head -1
}

# --- Qualified tags name their layer ---

assert_eq "$(target_of prompts/github)" "$DAISY_ROOT/prompts/github.md" \
    "prompts/<name> resolves in the prompts layer"
assert_eq "$(target_of skills/git)" "$DAISY_ROOT/skills/git/SKILL.md" \
    "skills/<name> resolves in the skills layer"

# --- The two names have come apart ---

assert_eq "$(target_of github)" "$DAISY_ROOT/prompts/github.md" \
    "a bare github still reaches the PR prompt"
assert_eq "$(target_of git)" "$DAISY_ROOT/skills/git/SKILL.md" \
    "a bare git now reaches the skill, not the PR prompt"

# --- Bare legacy names keep working ---

assert_eq "$(target_of daisy)" "$DAISY_ROOT/prompts/daisy.md" "a bare prompt name resolves"
assert_eq "$(target_of retrospective)" "$DAISY_ROOT/prompts/retrospective.md" "as do the others"
assert_eq "$(target_of new-day)" "$DAISY_ROOT/prompts/daisy.md" \
    "an alias with no file of its own still maps to daisy.md"

# --- A home prompt shadows the root's ---

mkdir -p "$(fixture_home_dir)/prompts"
printf '## Rules\n\n1. Home rule.\n' > "$(fixture_home_dir)/prompts/github.md"
assert_eq "$(target_of prompts/github)" "$(fixture_home_dir)/prompts/github.md" \
    "the home's prompt is looked up first"
rm "$(fixture_home_dir)/prompts/github.md"

# --- Tags naming nothing resolve to nothing ---

assert_eq "$(target_of daisy-write)" "exit 1" "an undefined tag names no file"
assert_eq "$(target_of prompts/nope)" "exit 1" "nor does a qualified tag for a missing prompt"
assert_eq "$(target_of skills/nope)" "exit 1" "nor one for a missing skill"

# --- An entry already holding one is briefed, not guessed at ---

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** daisy-write

Hand-written, naming no file.
FB
fixture_run_cli optimize
assert_eq "$RUN_EXIT" "0" "an unresolvable entry does not fail the briefing"
assert_contains "$RUN_STDOUT" "[daisy-write] 1 entry" "it is listed under its own tag"
assert_contains "$RUN_STDOUT" "names no prompt or skill" "with the reason"
assert_not_contains "$RUN_STDOUT" "[prompts/daisy] 1 entry" "and is not defaulted to daisy.md"

# --- daisy feedback refuses to write one ---

before=$(fixture_read_feedback)
fixture_run_cli feedback --workflow daisy-write "Should never be recorded."

assert_eq "$RUN_EXIT" "1" "an unresolvable workflow is refused"
assert_contains "$RUN_STDERR" "names no prompt or skill" "the refusal says why"
assert_contains "$RUN_STDERR" "prompts/daisy" "and lists what is available"
assert_contains "$RUN_STDERR" "skills/git" "including the skills the old list omitted"
assert_text_eq "$(fixture_read_feedback)" "$before" "and nothing is appended"

# --- A qualified tag is accepted and written verbatim ---

fixture_run_cli feedback --workflow skills/git "Conflict guidance was wrong."

assert_eq "$RUN_EXIT" "0" "a qualified workflow is accepted"
assert_contains "$(fixture_read_feedback)" "**Workflow:** skills/git" \
    "and recorded as given, so optimize needs no guess"
