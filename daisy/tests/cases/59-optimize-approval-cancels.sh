# Test Plan #59: anything but y or e at the approval prompt cancels, and
# cancelling must leave every file exactly as it found it. A cancelled run
# that archived its entries anyway would silently discard guidance that was
# never incorporated — the entries would be gone from feedback.md with no
# rewrite to show for them.

mkdir -p "$(fixture_home_dir)/prompts"
target="$(fixture_home_dir)/prompts/widgetflow.md"
cat > "$target" <<'EOF'
## Trigger

Read this when relevant.

## Rules

1. Measure before cutting.
EOF
before=$(cat "$target")

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** widgetflow

Widget guidance was wrong once.

## 2026-08-02 1100
**Workflow:** widgetflow

Widget guidance was wrong again.
EOF
feedback_before=$(fixture_read_feedback)
archive_before=$(fixture_read_archive)

fixture_stub_llm_response "## Rules

1. Measure twice before cutting."

fixture_run_with_input "n" optimize

assert_eq "$RUN_EXIT" "0" "declining is not an error"
assert_contains "$RUN_STDOUT" "Cancelled" "the run says it changed nothing"
assert_text_eq "$(cat "$target")" "$before" "the target is byte-identical"
assert_text_eq "$(fixture_read_feedback)" "$feedback_before" "feedback.md is byte-identical"
assert_text_eq "$(fixture_read_archive)" "$archive_before" "nothing is archived"
