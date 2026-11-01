# Test Plan #64: running out of stdin at the approval prompt cancels that
# group. It must not abort the run.
#
# `read` returns non-zero at EOF and the script runs under `set -e`, so the
# script exited from inside run_cycle — after that cycle had written its
# prompt file, and before the top-level archive pass. The state left behind
# was the damage: the target rewritten, the entries that trained it still in
# feedback.md, and AGENTS.md never rebuilt, so the next run would train again
# on entries already folded in.
#
# Two groups, one answer.

mkdir -p "$(fixture_home_dir)/prompts"
for wf in widgetflow gadgetflow; do
    cat > "$(fixture_home_dir)/prompts/$wf.md" <<'EOF'
## Rules

1. Measure before cutting.
EOF
done

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** widgetflow

Widget guidance was wrong once.

## 2026-08-02 1100
**Workflow:** widgetflow

Widget guidance was wrong again.

## 2026-08-03 1200
**Workflow:** gadgetflow

Gadget guidance was wrong once.

## 2026-08-04 1300
**Workflow:** gadgetflow

Gadget guidance was wrong again.
EOF

fixture_stub_llm_response "## Rules

1. Measure twice before cutting."

fixture_run_with_input "y" optimize

assert_eq "$RUN_EXIT" "0" "running out of input is not a failure"

# The answered group completes in full — including the archive pass, which
# ran at the top level and so was the part the abort skipped.
assert_contains "$(cat "$(fixture_home_dir)/prompts/widgetflow.md")" \
    "Measure twice before cutting." "the answered group is written"
archived=$(fixture_read_archive)
assert_contains "$archived" "Widget guidance was wrong once" "and its entries are archived"
assert_not_contains "$(fixture_read_feedback)" "Widget guidance was wrong once" \
    "and removed from feedback.md — the pair that must not come apart"

# The unanswered group is cancelled, and says why in terms distinguishable
# from a typed "n".
assert_contains "$RUN_STDOUT" "EOF" "the cancel names the reason"
assert_text_eq "$(cat "$(fixture_home_dir)/prompts/gadgetflow.md")" \
    "## Rules

1. Measure before cutting." "the unanswered group's target is untouched"
assert_contains "$(fixture_read_feedback)" "Gadget guidance was wrong once" \
    "and its entries stay in feedback.md"
