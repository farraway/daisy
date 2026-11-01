# Test Plan #57: the first case to reach optimize's write path at all. Every
# case before this one stopped at the floor or at ensure_llm_deps, which is
# why all five defects in the rewrite path were found by reading rather than
# by running. DAISY_OPTIMIZE_RESPONSE_FILE supplies a canned response so the
# cycle runs end to end with no API call.
#
# What it pins: --dry-run reaches the diff and writes nothing. Nothing has
# ever verified that.

mkdir -p "$(fixture_home_dir)/prompts"
target="$(fixture_home_dir)/prompts/widgetflow.md"
cat > "$target" <<'EOF'
## Trigger

Read this when relevant.

## Rules

1. Measure before cutting.

# Document Title

## Next Section

Body text.
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

fixture_stub_llm_response "## Rules

1. Measure twice before cutting."

fixture_run optimize --dry-run

assert_eq "$RUN_EXIT" "0" "a stubbed run needs neither curl nor a key"
assert_contains "$RUN_STDOUT" "Target: $target" "the home prompt resolves as the target"
assert_contains "$RUN_STDOUT" "Measure twice before cutting" "the proposal reaches the diff"
assert_contains "$RUN_STDOUT" "dry-run" "the run says it wrote nothing"
assert_text_eq "$(cat "$target")" "$before" "--dry-run leaves the target byte-identical"
assert_text_eq "$(fixture_read_feedback)" "$feedback_before" "--dry-run archives nothing"

# The stub is the only thing that made the above hermetic. Without it the
# same run must still stop at the dependency check rather than quietly
# reaching api.anthropic.com on a developer's exported key.
unset DAISY_OPTIMIZE_RESPONSE_FILE
fixture_run_without_api_key optimize --dry-run

assert_eq "$RUN_EXIT" "1" "an unstubbed run still requires the key"
assert_contains "$RUN_STDERR" "ANTHROPIC_API_KEY" "and says which key is missing"
