# Test Plan #63: an archived entry records which file absorbed it and how:
# `**Addressed by:** `daisy optimize` <date> — `<target>` (rules|rewrite)`.
# Two targets, applied one after the other: the record is per entry, and a
# provenance line that names the wrong file is worse than none.

for wf in widgetflow gadgetflow; do
    fixture_write_home_prompt "$wf" <<'PR'
## Rules

1. Measure before cutting.
PR
done

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Widget guidance was wrong once.

## 2026-08-02 1100
**Workflow:** prompts/gadgetflow

Gadget guidance was wrong once.

## 2026-08-03 1200
**Workflow:** prompts/widgetflow

Widget guidance was wrong again.
FB

proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Measure twice before cutting.
RU
)
fixture_run_cli optimize apply --workflow prompts/widgetflow --rules "$proposal" --yes
assert_eq "$RUN_EXIT" "0" "the first target applies"
fixture_run_cli optimize apply --workflow prompts/gadgetflow --rewrite "$proposal" --yes
assert_eq "$RUN_EXIT" "0" "the second target applies"

today=$(date +%Y-%m-%d)
provenance_for() {
    grep -A3 -- "$1" "$(fixture_home_dir)/feedback/archive.md" | grep -- "Addressed by"
}
assert_eq "$(provenance_for 'Widget guidance was wrong once.')" \
    "**Addressed by:** \`daisy optimize\` $today — \`prompts/widgetflow.md\` (rules)" \
    "the archived entry names the file that absorbed it, relative to the home"
assert_eq "$(provenance_for 'Widget guidance was wrong again.')" \
    "**Addressed by:** \`daisy optimize\` $today — \`prompts/widgetflow.md\` (rules)" \
    "every entry of the target is annotated"
assert_eq "$(provenance_for 'Gadget guidance was wrong once.')" \
    "**Addressed by:** \`daisy optimize\` $today — \`prompts/gadgetflow.md\` (rewrite)" \
    "the other target records its own file and mode"
assert_contains "$(cat "$(fixture_home_dir)/feedback/.last-optimized")" "prompts/gadgetflow" \
    "the last apply is recorded for healthcheck"
