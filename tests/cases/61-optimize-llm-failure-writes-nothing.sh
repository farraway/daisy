# Test Plan #61: the two ways the LLM call can fail — an `.error` payload and
# an empty completion — each abort the run, name the workflow, and leave
# every file untouched. Both branches `exit 1` from inside run_cycle and
# neither had ever been executed.
#
# The part that matters is not the exit code but the absence of wreckage: a
# failed rewrite must not leave a half-written prompt or an archived entry
# whose guidance was never incorporated.

mkdir -p "$(fixture_home_dir)/prompts"
target="$(fixture_home_dir)/prompts/widgetflow.md"
cat > "$target" <<'EOF'
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

# --- An API error ---

fixture_stub_llm_raw '{"error": {"type": "invalid_request_error", "message": "credit balance is too low"}}'
fixture_run_with_input "y" optimize

assert_eq "$RUN_EXIT" "1" "an API error aborts the run"
assert_contains "$RUN_STDERR" "widgetflow" "the failure names the workflow it was working on"
assert_contains "$RUN_STDERR" "credit balance is too low" "and passes through the API's own message"
assert_text_eq "$(cat "$target")" "$before" "the target is untouched"
assert_text_eq "$(fixture_read_feedback)" "$feedback_before" "feedback.md is untouched"
assert_text_eq "$(fixture_read_archive)" "$archive_before" "nothing is archived"

# --- An empty completion ---
# A well-formed envelope carrying no rules. Writing this out would blank the
# target's ## Rules section entirely.

fixture_stub_llm_raw '{"content": [{"type": "text", "text": ""}]}'
fixture_run_with_input "y" optimize

assert_eq "$RUN_EXIT" "1" "an empty completion aborts the run"
assert_contains "$RUN_STDERR" "Empty response" "the failure says what was wrong"
assert_contains "$RUN_STDERR" "widgetflow" "and names the workflow"
assert_text_eq "$(cat "$target")" "$before" "the target is still untouched"
assert_text_eq "$(fixture_read_feedback)" "$feedback_before" "feedback.md is still untouched"
assert_text_eq "$(fixture_read_archive)" "$archive_before" "still nothing archived"
