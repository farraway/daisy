# Test Plan #51: `daisy help <skill>` surfaces usage detail beyond `daisy
# list`'s one-liner, so an agent can decide whether a skill applies without
# grepping SKILL.md files off disk itself.

skill_dir="$(fixture_home_dir)/skills/widgetcraft"
mkdir -p "$skill_dir"
cat > "$skill_dir/SKILL.md" <<'EOF'
---
name: widgetcraft
description: Making widgets well, including when to stop making them.
short_description: Widget craft.
---

## Rules

1. Measure before cutting.

## Worked example

A widget, measured.
EOF

fixture_run_cli help widgetcraft

assert_eq "$RUN_EXIT" "0" "help succeeds for a skill"
assert_contains "$RUN_STDOUT" "when to stop making them" "full description, not the short one"
assert_contains "$RUN_STDOUT" "Measure before cutting" "the skill's rules are shown"
assert_contains "$RUN_STDOUT" "Worked example" "remaining sections are outlined"
