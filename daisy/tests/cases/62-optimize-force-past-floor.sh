# Test Plan #62: --force runs a group that is below the entry floor, and says
# that it did. The floor and its default are unchanged — a rules section
# should not be rewritten from one data point by accident — but three single
# entries were hand-applied on 2026-08-17 because there was no way to say "I
# have considered it, run anyway".
#
# A flag rather than a second confirmation prompt: optimize already has one
# `read -rp` that makes it undriveable from a tool call.

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

The only entry there is.
EOF

# --- Default: the floor holds ---
# No stub and no key: reaching this verdict must still cost no API call.

fixture_run optimize --dry-run

assert_eq "$RUN_EXIT" "0" "a below-floor group is not an error"
assert_contains "$RUN_STDOUT" "Skipped" "the group is skipped by default"
assert_contains "$RUN_STDOUT" "2-entry floor" "the reason names the floor"
assert_contains "$RUN_STDOUT" "--force" "and points at the way past it"
assert_text_eq "$(cat "$target")" "$before" "nothing is written"

# --- With --force: the same group runs ---

fixture_stub_llm_response "## Rules

1. Measure twice before cutting."

fixture_run optimize --dry-run --force

assert_eq "$RUN_EXIT" "0" "the forced run succeeds"
assert_not_contains "$RUN_STDOUT" "Skipped" "the group is no longer skipped"
assert_contains "$RUN_STDOUT" "running anyway" "the run says it overrode the floor"
assert_contains "$RUN_STDOUT" "2-entry floor" "and still names the floor it overrode"
assert_contains "$RUN_STDOUT" "Measure twice before cutting" "the group reached the LLM and produced a proposal"
