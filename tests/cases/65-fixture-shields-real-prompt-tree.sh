# Test Plan #65: an applying run against a *shared* prompt writes the
# fixture's copy and leaves the repo's own file alone.
#
# fixture_setup used to symlink the real prompts/ and skills/ trees, so
# DAISY_ROOT/prompts was the repo's prompts/. Every case avoided writing
# there by targeting $DAISY_HOME/prompts/ — by convention, with nothing
# enforcing it. A safeguard that depends on being remembered is not a
# safeguard.
#
# `retrospective` is deliberately a shared prompt with a real ## Rules
# section, not a fixture-local one — the point is to drive the case straight
# at the file that used to be at risk.

real_prompt="$REAL_DAISY_ROOT/prompts/retrospective.md"
fixture_prompt="$FIXTURE_ROOT/prompts/retrospective.md"
real_before=$(cat "$real_prompt")

assert_eq "$(readlink "$FIXTURE_ROOT/prompts" || echo not-a-symlink)" "not-a-symlink" \
    "the fixture's prompts tree is a copy, not a link to the repo's"

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** retrospective

Retrospective guidance was wrong once.
FB

proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Report a measurement, not a score.
RU
)
fixture_run_cli optimize apply --workflow retrospective --rules "$proposal" --yes

assert_eq "$RUN_EXIT" "0" "the run applies"
assert_contains "$RUN_STDOUT" "$fixture_prompt" "the target is the fixture's copy"
assert_contains "$(cat "$fixture_prompt")" "1. Report a measurement, not a score." \
    "the fixture's copy is rewritten"
assert_text_eq "$(cat "$real_prompt")" "$real_before" \
    "the repo's own prompts/retrospective.md is byte-identical"
