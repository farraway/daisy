# Test Plan #39: there is no entry floor. One entry is enough for the
# briefing to present its target, with the entry and the target's current
# ## Rules section, so the agent can propose; whether one entry is enough
# to change the rules is the agent's and the user's judgement, not a constant
# (D12). The old loop skipped a group below two entries and needed
# `--force` to say otherwise.

fixture_write_home_prompt widgetflow <<'PR'
## Trigger

Read this when relevant.

## Rules

1. Measure before cutting.

## Next Section

Body text.
PR

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

The only entry there is.
FB

fixture_run_cli optimize

assert_eq "$RUN_EXIT" "0" "a single entry is briefed"
assert_contains "$RUN_STDOUT" "[prompts/widgetflow] 1 entry" "the target is listed"
assert_contains "$RUN_STDOUT" "The only entry there is." "with its entry"
assert_contains "$RUN_STDOUT" "1. Measure before cutting." "and its current rules"
assert_contains "$RUN_STDOUT" "Rules: 1" "and the rule count"
assert_not_contains "$RUN_STDOUT" "Body text." "the rest of the prompt is not the rules"
assert_not_contains "$RUN_STDOUT" "Skipped" "nothing is skipped"
assert_not_contains "$RUN_STDOUT" "floor" "there is no floor to name"
assert_contains "$RUN_STDOUT" "daisy optimize apply --workflow prompts/widgetflow --rules" \
    "the briefing says how to apply a proposal"
