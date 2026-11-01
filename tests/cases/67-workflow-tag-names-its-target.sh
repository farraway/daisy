# Test Plan #67: a workflow tag names the file its guidance belongs in, and
# `daisy feedback` refuses one that names nothing.
#
# The tag was unvalidated free text: feedback.sh wrote whatever it was given
# and the resolver guessed afterwards, from a string nothing had constrained.
# That is how `daisy-write`, `daisy-scripts` and the literal
# `new-day, new-week` reached the archive, each training daisy.md by accident
# through the "Unknown workflow" fallback.
#
# It is also how `git` and `github` were conflated: the alias table was
# consulted *before* the name lookups, so a `github|git)` arm swallowed `git`
# and skills/git was unreachable.

resolve() { bash -c "source '$DAISY_ROOT/daisy/scripts/common.sh'; DAISY_HOME='$(fixture_home_dir)' resolve_workflow_target '$1'"; }

# --- Qualified tags name their layer ---

assert_eq "$(resolve prompts/github)" "$DAISY_ROOT/prompts/github.md" \
    "prompts/<name> resolves in the prompts layer"
assert_eq "$(resolve skills/git)" "$DAISY_ROOT/skills/git/SKILL.md" \
    "skills/<name> resolves in the skills layer"

# --- The two names have come apart ---

assert_eq "$(resolve github)" "$DAISY_ROOT/prompts/github.md" \
    "a bare github still reaches the PR prompt"
assert_eq "$(resolve git)" "$DAISY_ROOT/skills/git/SKILL.md" \
    "a bare git now reaches the skill, not the PR prompt"

# --- Bare legacy names keep working ---

assert_eq "$(resolve daisy)" "$DAISY_ROOT/prompts/daisy.md" "a bare prompt name resolves"
assert_eq "$(resolve retrospective)" "$DAISY_ROOT/prompts/retrospective.md" "as do the others"
assert_eq "$(resolve new-day)" "$DAISY_ROOT/prompts/daisy.md" \
    "an alias with no file of its own still maps to daisy.md"

# --- Tags naming nothing resolve to nothing ---

assert_eq "$(resolve daisy-write)" "" "an undefined tag names no file"
assert_eq "$(resolve prompts/nope)" "" "nor does a qualified tag for a missing prompt"
assert_eq "$(resolve skills/nope)" "" "nor one for a missing skill"

# --- daisy feedback refuses to write one ---

before=$(fixture_read_feedback)
fixture_run feedback --workflow daisy-write "Should never be recorded."

assert_eq "$RUN_EXIT" "1" "an unresolvable workflow is refused"
assert_contains "$RUN_STDERR" "names no prompt or skill" "the refusal says why"
assert_contains "$RUN_STDERR" "prompts/daisy" "and lists what is available"
assert_contains "$RUN_STDERR" "skills/git" "including the skills the old list omitted"
assert_text_eq "$(fixture_read_feedback)" "$before" "and nothing is appended"

# --- A qualified tag is accepted and written verbatim ---

fixture_run feedback --workflow skills/git "Conflict guidance was wrong."

assert_eq "$RUN_EXIT" "0" "a qualified workflow is accepted"
assert_contains "$(fixture_read_feedback)" "**Workflow:** skills/git" \
    "and recorded as given, so optimize needs no guess"
