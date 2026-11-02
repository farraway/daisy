# Test Plan #49: a workflow naming a Daisy-authored skill resolves to that
# skill's *source* SKILL.md. Placement follows runtime coupling, so the
# learning loop has to follow content into the craft layer — but it must
# rewrite the source Daisy owns, never an installed copy that `daisy install`
# would overwrite on its next run.

skill_dir="$(fixture_home_dir)/skills/widgetcraft"
mkdir -p "$skill_dir"
cat > "$skill_dir/SKILL.md" <<'SK'
---
name: widgetcraft
description: Making widgets well.
---

## Rules

1. Measure before cutting.
SK

fixture_write_feedback <<'FB'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** widgetcraft

Widget guidance was wrong.
FB

fixture_run optimize

assert_eq "$RUN_EXIT" "0" "the briefing succeeds"
assert_contains "$RUN_STDOUT" "[skills/widgetcraft] 1 entry" "a bare skill name is the skill's target"
assert_contains "$RUN_STDOUT" "Target: $skill_dir/SKILL.md" "skill source resolved as the target"
assert_not_contains "$RUN_STDOUT" "names no prompt or skill" "a known skill is not an unknown workflow"

proposal=$(fixture_write_proposal rules.md <<'RU'
## Rules

1. Measure twice before cutting.
RU
)
fixture_run_cli optimize apply --workflow skills/widgetcraft --rules "$proposal" --yes

assert_eq "$RUN_EXIT" "0" "applying to a skill succeeds"
assert_contains "$(cat "$skill_dir/SKILL.md")" "1. Measure twice before cutting." "the source SKILL.md is written"
assert_contains "$(cat "$skill_dir/SKILL.md")" "name: widgetcraft" "its frontmatter survives"
assert_contains "$RUN_STDOUT" "daisy install --update" "the run says installed copies are stale"
