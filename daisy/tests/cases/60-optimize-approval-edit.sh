# Test Plan #60: `e` at the approval prompt opens the proposal in $EDITOR and
# writes back whatever the editor left. This branch is the reason --dry-run
# is not the review point (2026-08-18): it is the only place the exact text
# about to be written can be corrected. It had never been executed.
#
# The editor is a non-interactive `sed -i`, so the case asserts on the
# resulting file rather than on editor behaviour.

mkdir -p "$(fixture_home_dir)/prompts"
target="$(fixture_home_dir)/prompts/widgetflow.md"
cat > "$target" <<'EOF'
## Rules

1. Measure before cutting.
EOF

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

fixture_stub_llm_response "## Rules

1. Measure PROPOSED before cutting."

# Exported, not prefixed: a `VAR=x somefunc` assignment is visible to the
# function but not to the processes it spawns, and optimize runs in one.
export EDITOR='sed -i s/PROPOSED/CORRECTED/'
fixture_run_with_input "e" optimize
unset EDITOR

assert_eq "$RUN_EXIT" "0" "the edit branch completes"

written=$(cat "$target")
assert_contains "$written" "1. Measure CORRECTED before cutting." "what the editor left is what gets written"
assert_not_contains "$written" "PROPOSED" "the unedited proposal does not reach the file"
