# Check 4c: daisy healthcheck must warn (not fail) about a stale root-installed
# skill under $HOME/.claude/skills with no matching source in
# $DAISY_ROOT/skills — Check 4b only ever covered the per-workspace
# .claude/skills directory, never the root one where `daisy install` actually
# puts things (defect 6b).

fixture_setup_home_override
mkdir -p "$FIXTURE_HOME_OVERRIDE/.claude/skills/definitely-not-a-real-skill"
echo "stale" > "$FIXTURE_HOME_OVERRIDE/.claude/skills/definitely-not-a-real-skill/SKILL.md"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck still exits 0 (warn-only check)"
assert_contains "$RUN_STDERR" "Root-installed skill 'definitely-not-a-real-skill' has no matching source" \
    "healthcheck warns about the stale root skill"
