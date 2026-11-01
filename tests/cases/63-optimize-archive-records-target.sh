# Test Plan #63: an archived entry records which file absorbed it. Entries
# archived by optimize carried no provenance at all — the hand-applied ones
# have an "Addressed by:" line only because it was typed by hand — so the
# archive could say an entry was folded in but never where.
#
# Two groups writing to two different targets, in one run: the record is
# per-entry, and a provenance line that names the wrong file is worse than
# none. The archive pass runs once at the top level and knows only flat
# indices into ALL_ENTRIES, so this is the assertion that the per-cycle fact
# actually survives the trip out of run_cycle.

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

# One `y` per group — two groups clear the floor, so the prompt is reached
# twice and each answer must be its own line of stdin.
fixture_run_with_input "y
y" optimize

assert_eq "$RUN_EXIT" "0" "both groups apply"

# The provenance line follows its entry's body, so read the lines after it
# rather than searching the archive as a whole.
provenance_for() {
    grep -A3 -- "$1" "$(fixture_home_dir)/feedback/archive.md" | grep -- "Addressed by"
}

widget=$(provenance_for "Widget guidance was wrong once.")
assert_contains "$widget" "**Addressed by:** \`daisy optimize\`" "the archived entry carries provenance"
assert_contains "$widget" "— \`prompts/widgetflow.md\` ## Rules" "naming the file that absorbed it, relative to the home"
assert_not_contains "$widget" "gadgetflow" "and not the other group's target"

gadget=$(provenance_for "Gadget guidance was wrong once.")
assert_contains "$gadget" "— \`prompts/gadgetflow.md\` ## Rules" "the other group records its own target"
assert_not_contains "$gadget" "widgetflow" "and not the first group's target"

# Both entries of a group get the line, not just the first.
assert_contains "$(provenance_for 'Widget guidance was wrong again.')" \
    "— \`prompts/widgetflow.md\` ## Rules" "every trained entry is annotated"
