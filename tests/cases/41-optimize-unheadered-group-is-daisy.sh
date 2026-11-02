# Test Plan #41: an entry with no **Workflow:** header belongs to
# prompts/daisy — the prompt that owns the unlabelled workflows — on an
# unscoped briefing, preserving the target such entries have always had;
# the conflation removed in #38 was "no filter", not the daisy workflow.

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000

An entry with no workflow header at all.

## 2026-08-02 1100
**Workflow:** plan

Plan workflow did the wrong thing.
FB

fixture_run_cli optimize

assert_eq "$RUN_EXIT" "0" "the briefing succeeds"
assert_contains "$RUN_STDOUT" "[prompts/daisy] 1 entry" "the unheadered entry is daisy's"
assert_contains "$RUN_STDOUT" "Target: $FIXTURE_ROOT/prompts/daisy.md" "resolved to daisy.md"
assert_contains "$RUN_STDOUT" "[prompts/plan] 1 entry" "the headered entry keeps its own target"
