# Test Plan #66: the feedback.md an optimize run leaves behind has one blank
# line before each surviving entry, matching the file it was parsed from.
#
# Entries are parsed carrying the blank line that preceded the next heading,
# and the rewrite loop prefixes each with its own `echo ""` — so every
# boundary gained a line, including the one after the header. Stable rather
# than accreting (entries are re-parsed each run), so this is cosmetic; it is
# pinned because nothing pinned this file's shape at all, which is why the
# drift was invisible.

mkdir -p "$(fixture_home_dir)/prompts"
cat > "$(fixture_home_dir)/prompts/widgetflow.md" <<'EOF'
## Rules

1. Measure before cutting.
EOF

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** widgetflow

Trained one.

## 2026-08-02 1100
**Workflow:** widgetflow

Trained two.

## 2026-08-03 1200
**Workflow:** loneflow

Survivor, below the floor.

## 2026-08-04 1300
**Workflow:** otherflow

Second survivor.
EOF

fixture_stub_llm_response "## Rules

1. Measure twice before cutting."

fixture_run_with_input "y" optimize

assert_eq "$RUN_EXIT" "0" "the run applies"

# Byte-for-byte: the header, then each untrained entry separated by exactly
# one blank line — the same shape the entries were read from.
assert_text_eq "$(fixture_read_feedback)" "# Feedback Log

---

## 2026-08-03 1200
**Workflow:** loneflow

Survivor, below the floor.

## 2026-08-04 1300
**Workflow:** otherflow

Second survivor." "the rewritten feedback.md keeps its original spacing"
