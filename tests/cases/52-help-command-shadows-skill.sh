# Test Plan #52: when a name is both a command and a skill, the command
# wins — but the shadowing is reported, so a skill that has become
# unreachable by name is visible rather than mysteriously missing.

skill_dir="$(fixture_home_dir)/skills/status"
mkdir -p "$skill_dir"
cat > "$skill_dir/SKILL.md" <<'EOF'
---
name: status
description: A skill that collides with a command name.
---

## Rules

1. Collide politely.
EOF

fixture_run_cli help status

assert_eq "$RUN_EXIT" "0" "help succeeds"
assert_not_contains "$RUN_STDOUT" "Collide politely" "the command's help is shown, not the skill's"
assert_contains "$RUN_STDOUT" "skill" "the shadowed skill is mentioned"
