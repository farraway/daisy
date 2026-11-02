# Test Plan #40: --workflow X briefs only the target X names. It used to
# also sweep in entries carrying no **Workflow:** header, which meant a
# scoped run and an unscoped run routed the same entry to different prompts.

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** plan

Plan workflow did the wrong thing.

## 2026-08-02 1100

An entry with no workflow header at all.
FB

fixture_run_cli optimize --workflow plan

assert_eq "$RUN_EXIT" "0" "a scoped briefing succeeds"
assert_contains "$RUN_STDOUT" "[prompts/plan] 1 entry" "only the declared entry is in scope"
assert_not_contains "$RUN_STDOUT" "An entry with no workflow header" "the unheadered entry is out of scope"
assert_not_contains "$RUN_STDOUT" "[prompts/daisy]" "and so is its target"

fixture_run_cli optimize --workflow daisy-write
assert_eq "$RUN_EXIT" "1" "a scope naming no file is refused"
assert_contains "$RUN_STDERR" "names no prompt or skill" "with the reason"
