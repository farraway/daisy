# Test Plan #58: answering `y` at the approval prompt writes the proposed
# rules into the target and archives the entries that trained them. This is
# the loop's whole purpose and no case had ever run it — the prompt is a bare
# `read -rp`, so until fixture_run_with_input there was no way to answer it.

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

1. Measure twice before cutting.
2. Cut once."

fixture_run_with_input "y" optimize

assert_eq "$RUN_EXIT" "0" "an applying run succeeds"
assert_contains "$RUN_STDOUT" "Updated $target" "the run reports the file it wrote"

written=$(cat "$target")
assert_contains "$written" "1. Measure twice before cutting." "the proposed rules are written"
assert_contains "$written" "2. Cut once." "all of them, not just the first"
assert_not_contains "$written" "Measure before cutting." "the old rules are replaced"

# The section boundary case (#53) proves the replacer in isolation; this
# proves the cycle that calls it does not lose the surrounding document.
assert_contains "$written" "## Trigger" "the section above the rules survives"
assert_contains "$written" "# Document Title" "the H1 between the sections survives"
assert_contains "$written" "Body text." "the section below the rules survives"

# Entries that trained a rewrite move out of feedback.md into the archive.
remaining=$(fixture_read_feedback)
assert_not_contains "$remaining" "Widget guidance was wrong once" "the trained entry leaves feedback.md"
assert_not_contains "$remaining" "Widget guidance was wrong again" "both of them do"
assert_contains "$remaining" "# Feedback Log" "the file keeps its header"

archived=$(fixture_read_archive)
assert_contains "$archived" "Widget guidance was wrong once" "the trained entry lands in the archive"
assert_contains "$archived" "Widget guidance was wrong again" "both of them do"
