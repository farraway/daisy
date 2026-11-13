# Test Plan #56: the briefing counts each target's rules, and flags every
# active prompt or skill whose ## Rules holds more than 3 for a whole-file
# rewrite, pending entries or not (D12: past a few rules a prompt has
# accreted rules its body should absorb; 3, the maintainer's choice of
# 2026-11-12). Three is not flagged; four is.

rules() {
    local n=$1 i
    printf '## Trigger\n\nRead this when relevant.\n\n## Rules\n\n'
    for i in $(seq 1 "$n"); do printf '%d. Rule %d.\n' "$i" "$i"; done
}
rules 3 | fixture_write_home_prompt threeflow
rules 4 | fixture_write_home_prompt fourflow
printf '~threeflow\n~fourflow\n' >> "$(fixture_home_dir)/include.txt"

fixture_write_feedback <<'FB'
# Feedback Log

---
FB

fixture_run_cli optimize

assert_eq "$RUN_EXIT" "0" "nothing pending is not an error"
assert_contains "$RUN_STDOUT" "Nothing pending" "and says so"
assert_contains "$RUN_STDOUT" "[prompts/fourflow] rewrite flagged: 4 rules" "four rules are flagged"
assert_not_contains "$RUN_STDOUT" "[prompts/threeflow]" "three are not"
assert_contains "$RUN_STDOUT" "--rewrite" "the flag says how to apply a rewrite"

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/threeflow

Three-flow guidance was wrong.

## 2026-08-02 1100
**Workflow:** prompts/fourflow

Four-flow guidance was wrong.
FB

fixture_run_cli optimize

assert_contains "$RUN_STDOUT" "[prompts/threeflow] 1 entry" "a pending target is briefed"
assert_contains "$RUN_STDOUT" "Rules: 3
" "with its count"
assert_contains "$RUN_STDOUT" "Rules: 4 — more than 3: rewrite flagged" "a pending target over 3 carries the flag"
assert_not_contains "$RUN_STDOUT" "Nothing pending" "entries are pending"
