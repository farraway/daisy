# Test Plan #64: running out of stdin at the approval prompt cancels, says
# so in terms distinguishable from a typed "n", writes nothing and is not a
# failure. Under the bash loop's `set -e` an EOF exited from inside the
# cycle, after the prompt was written and before its entries were archived
# — the one pair that must never come apart.

target="$(fixture_home_dir)/prompts/widgetflow.md"
fixture_write_home_prompt widgetflow <<'PR'
## Rules

1. Measure before cutting.
PR
before=$(cat "$target")

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Widget guidance was wrong once.
FB
feedback_before=$(fixture_read_feedback)
commits_before=$(fixture_commit_count)

proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Measure twice before cutting.
RU
)
fixture_run_cli optimize apply --workflow prompts/widgetflow --rules "$proposal" </dev/null

assert_eq "$RUN_EXIT" "0" "running out of input is not a failure"
assert_contains "$RUN_STDOUT" "EOF" "the cancel names the reason"
assert_text_eq "$(cat "$target")" "$before" "the target is untouched"
assert_text_eq "$(fixture_read_feedback)" "$feedback_before" "its entries stay in feedback.md"
assert_eq "$(fixture_commit_count)" "$commits_before" "nothing is committed"
