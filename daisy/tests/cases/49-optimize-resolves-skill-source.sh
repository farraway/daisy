# Test Plan #49: a workflow naming a Daisy-authored skill resolves to that
# skill's *source* SKILL.md. Placement follows runtime coupling, so the
# learning loop has to follow content into the craft layer — but it must
# rewrite the source Daisy owns, never an installed copy that `daisy install`
# would overwrite on its next run.

skill_dir="$(fixture_home_dir)/skills/widgetcraft"
mkdir -p "$skill_dir"
cat > "$skill_dir/SKILL.md" <<'EOF'
---
name: widgetcraft
description: Making widgets well.
---

## Rules

1. Measure before cutting.
EOF

fixture_write_feedback <<'EOF'
# Feedback Log

---

## 2026-08-01 1000
**Workflow:** widgetcraft

Widget guidance was wrong.
EOF

fixture_run_without_api_key optimize --dry-run

assert_eq "$RUN_EXIT" "0" "run succeeds"
assert_contains "$RUN_STDOUT" "skills/widgetcraft/SKILL.md" "skill source resolved as the target"
assert_not_contains "$RUN_STDERR" "Unknown workflow" "a known skill is not an unknown workflow"
