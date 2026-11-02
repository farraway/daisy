# Test Plan #56: the briefing counts each target's rules, and flags every
# active prompt or skill whose ## Rules holds more than 10 for a whole-file
# rewrite, pending entries or not (D12: past 10 rules a prompt has accreted
# rules its body should absorb). Ten is not flagged; eleven is.

rules() {
    local n=$1 i
    printf '## Trigger\n\nRead this when relevant.\n\n## Rules\n\n'
    for i in $(seq 1 "$n"); do printf '%d. Rule %d.\n' "$i" "$i"; done
}
rules 10 | fixture_write_home_prompt tenflow
rules 11 | fixture_write_home_prompt elevenflow
printf '~tenflow\n~elevenflow\n' >> "$(fixture_home_dir)/include.txt"

fixture_write_feedback <<'FB'
# Feedback Log

---
FB

fixture_run_cli optimize

assert_eq "$RUN_EXIT" "0" "nothing pending is not an error"
assert_contains "$RUN_STDOUT" "Nothing pending" "and says so"
assert_contains "$RUN_STDOUT" "[prompts/elevenflow] rewrite flagged: 11 rules" "eleven rules are flagged"
assert_not_contains "$RUN_STDOUT" "[prompts/tenflow]" "ten are not"
assert_contains "$RUN_STDOUT" "--rewrite" "the flag says how to apply a rewrite"

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/tenflow

Ten-flow guidance was wrong.

## 2026-08-02 1100
**Workflow:** prompts/elevenflow

Eleven-flow guidance was wrong.
FB

fixture_run_cli optimize

assert_contains "$RUN_STDOUT" "[prompts/tenflow] 1 entry" "a pending target is briefed"
assert_contains "$RUN_STDOUT" "Rules: 10
" "with its count"
assert_contains "$RUN_STDOUT" "Rules: 11 — more than 10: rewrite flagged" "a pending target over 10 carries the flag"
assert_not_contains "$RUN_STDOUT" "Nothing pending" "entries are pending"
