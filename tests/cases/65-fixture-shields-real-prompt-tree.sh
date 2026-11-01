# Test Plan #65: an applying run against a *shared* prompt writes the
# fixture's copy and leaves the repo's own file alone.
#
# fixture_setup used to symlink the real prompts/ and skills/ trees, so
# DAISY_ROOT/prompts was the repo's prompts/. Every case avoided writing
# there by targeting $DAISY_HOME/prompts/ — by convention, with nothing
# enforcing it, while the unknown-workflow fallback routes anything
# unrecognised to prompts/daisy.md. This is the same shape as the
# ANTHROPIC_API_KEY hole: a safeguard that depends on being remembered is not
# a safeguard.
#
# `retrospective` is deliberately a shared prompt with a real ## Rules
# section, not a fixture-local one — the point is to drive the case straight
# at the file that used to be at risk.

real_prompt="$REAL_DAISY_ROOT/prompts/retrospective.md"
fixture_prompt="$FIXTURE_ROOT/prompts/retrospective.md"
real_before=$(cat "$real_prompt")

assert_eq "$(readlink "$FIXTURE_ROOT/prompts" || echo not-a-symlink)" "not-a-symlink" \
    "the fixture's prompts tree is a copy, not a link to the repo's"

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** retrospective

Retrospective guidance was wrong once.

## 2026-08-02 1100
**Workflow:** retrospective

Retrospective guidance was wrong again.
EOF

fixture_stub_llm_response "## Rules

1. Report a measurement, not a score."

fixture_run_with_input "y" optimize

assert_eq "$RUN_EXIT" "0" "the run applies"
assert_contains "$RUN_STDOUT" "$fixture_prompt" "the target is the fixture's copy"

assert_contains "$(cat "$fixture_prompt")" "1. Report a measurement, not a score." \
    "the fixture's copy is rewritten"
assert_text_eq "$(cat "$real_prompt")" "$real_before" \
    "the repo's own prompts/retrospective.md is byte-identical"
