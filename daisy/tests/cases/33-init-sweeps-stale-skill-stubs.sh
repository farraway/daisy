# daisy init must remove a workspace's .claude/skills/<name> copy and
# .cursor/rules/<name>.mdc pointer stub once that skill no longer exists in
# the home's skills/ source — otherwise a deleted home skill's stub lingers
# in every workspace forever (defect 6c).

mkdir -p "$FIXTURE_WORKSPACE/.claude/skills/definitely-not-a-real-skill"
echo "stale" > "$FIXTURE_WORKSPACE/.claude/skills/definitely-not-a-real-skill/SKILL.md"
mkdir -p "$FIXTURE_WORKSPACE/.cursor/rules"
cat > "$FIXTURE_WORKSPACE/.cursor/rules/definitely-not-a-real-skill.mdc" <<'EOF'
---
description: stale fixture skill stub
alwaysApply: false
---
EOF

fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "daisy init exits 0"

if [ -d "$FIXTURE_WORKSPACE/.claude/skills/definitely-not-a-real-skill" ]; then
    echo "    ASSERT FAILED: stale skill copy survived daisy init"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi
if [ -f "$FIXTURE_WORKSPACE/.cursor/rules/definitely-not-a-real-skill.mdc" ]; then
    echo "    ASSERT FAILED: stale skill .mdc stub survived daisy init"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi

# Core (non-skill) cursor rules must survive the same sweep.
if [ ! -f "$FIXTURE_WORKSPACE/.cursor/rules/daisy.mdc" ]; then
    echo "    ASSERT FAILED: core daisy.mdc rule was swept along with stale skill stubs"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi
