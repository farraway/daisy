# Test Plan #59: anything but y at the approval prompt cancels, and
# cancelling leaves every file exactly as it found it and commits nothing.
# A cancelled run that archived its entries anyway would silently discard
# guidance that was never incorporated.

target="$(fixture_home_dir)/prompts/widgetflow.md"
fixture_write_home_prompt widgetflow <<'PR'
## Trigger

Read this when relevant.

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
archive_before=$(fixture_read_archive)
commits_before=$(fixture_commit_count)

proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Measure twice before cutting.
RU
)
for answer in n "" yes-please; do
    fixture_run_cli_with_input "$answer" optimize apply --workflow prompts/widgetflow --rules "$proposal"
    assert_eq "$RUN_EXIT" "0" "declining ('$answer') is not an error"
    assert_contains "$RUN_STDOUT" "Cancelled" "the run says it changed nothing ('$answer')"
done
assert_text_eq "$(cat "$target")" "$before" "the target is byte-identical"
assert_text_eq "$(fixture_read_feedback)" "$feedback_before" "feedback.md is byte-identical"
assert_text_eq "$(fixture_read_archive)" "$archive_before" "nothing is archived"
assert_eq "$(fixture_commit_count)" "$commits_before" "nothing is committed"
assert_eq "$([ -e "$(fixture_home_dir)/feedback/.last-optimized" ] && echo stamped || echo none)" "none" \
    "no optimize run is recorded"
