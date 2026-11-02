# Test Plan #38: the briefing groups pending entries by the target their
# **Workflow:** tag names and lists each target once, with its entries. "No
# filter" once meant "the daisy workflow", so a mixed inbox was folded
# wholesale into prompts/daisy.md's ## Rules; a group is a target, never a
# default.

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** plan

Plan workflow did the wrong thing.

## 2026-08-02 1100
**Workflow:** prompts/retrospective

Retrospective workflow did the wrong thing.

## 2026-08-03 1200
**Workflow:** prompts/plan

Plan workflow did the wrong thing again.
FB

fixture_run_cli optimize

assert_eq "$RUN_EXIT" "0" "the briefing succeeds"
assert_contains "$RUN_STDOUT" "Pending feedback: 3 entries for 2 targets" "the summary counts entries and targets"
# `plan` and `prompts/plan` name one file, so they are one target.
assert_contains "$RUN_STDOUT" "[prompts/plan] 2 entries" "both tags for plan.md are one target"
assert_contains "$RUN_STDOUT" "[prompts/retrospective] 1 entry" "retrospective is its own target"
assert_contains "$RUN_STDOUT" "Target: $FIXTURE_ROOT/prompts/plan.md" "the target path is printed"
assert_contains "$RUN_STDOUT" "Plan workflow did the wrong thing again." "the entries are printed"
assert_eq "$(printf '%s\n' "$RUN_STDOUT" | grep -c '^\[prompts/daisy\] [0-9]* entr')" "0" \
    "no entry is defaulted to daisy.md"
