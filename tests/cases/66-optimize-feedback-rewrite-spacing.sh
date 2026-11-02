# Test Plan #66: the feedback.md an apply leaves behind has one blank line
# before each surviving entry, matching the file it was parsed from, and
# the archive gains each entry followed by its provenance, one blank line
# apart — byte for byte.

fixture_write_home_prompt widgetflow <<'PR'
## Rules

1. Measure before cutting.
PR

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Trained one.

## 2026-08-02 1100
**Workflow:** loneflow

Survivor, naming no file.

## 2026-08-03 1200
**Workflow:** prompts/widgetflow

Trained two.

## 2026-08-04 1300
**Workflow:** plan

Second survivor.
FB
archive_before=$(cat "$(fixture_home_dir)/feedback/archive.md"; echo .)

proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Measure twice before cutting.
RU
)
fixture_run_cli optimize apply --workflow prompts/widgetflow --rules "$proposal" --yes

assert_eq "$RUN_EXIT" "0" "the run applies"
printf '%s' "# Feedback Log

---

## 2026-08-02 1100
**Workflow:** loneflow

Survivor, naming no file.

## 2026-08-04 1300
**Workflow:** plan

Second survivor.
" > "$FIXTURE_WORKSPACE/expected-feedback.md"
cmp -s "$(fixture_home_dir)/feedback/feedback.md" "$FIXTURE_WORKSPACE/expected-feedback.md"
assert_eq "$?" "0" "the rewritten feedback.md keeps its original spacing, byte for byte"

today=$(date +%Y-%m-%d)
printf '%s' "${archive_before%.}
## 2026-08-01 1000
**Workflow:** prompts/widgetflow

Trained one.

**Addressed by:** \`daisy optimize\` $today — \`prompts/widgetflow.md\` (rules)

## 2026-08-03 1200
**Workflow:** prompts/widgetflow

Trained two.

**Addressed by:** \`daisy optimize\` $today — \`prompts/widgetflow.md\` (rules)
" > "$FIXTURE_WORKSPACE/expected-archive.md"
cmp -s "$(fixture_home_dir)/feedback/archive.md" "$FIXTURE_WORKSPACE/expected-archive.md"
assert_eq "$?" "0" "the archive gains each entry and its provenance, byte for byte"
