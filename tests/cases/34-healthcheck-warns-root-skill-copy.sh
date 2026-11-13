# Check 4c: daisy healthcheck warns (not fails) about a copy of a root skill
# that an earlier `daisy install` left under $HOME/.claude/skills, where
# install no longer writes (install --update removes it); a skill of the
# user's own there is not Daisy's and never warns (defect 6b once had 4c
# watch only the workspace's copies).

fixture_setup_home_override
mkdir -p "$FIXTURE_HOME_OVERRIDE/.claude/skills/definitely-not-a-real-skill"
echo "the user's" > "$FIXTURE_HOME_OVERRIDE/.claude/skills/definitely-not-a-real-skill/SKILL.md"
cp -r "$FIXTURE_ROOT/skills/testing" "$FIXTURE_HOME_OVERRIDE/.claude/skills/testing"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck still exits 0 (warn-only check)"
assert_contains "$RUN_STDERR" "/.claude/skills/testing is a copy of the root's skill 'testing'" \
    "healthcheck warns about the root skill's copy"
assert_contains "$RUN_STDERR" "run 'daisy install --update' to remove it" "and says how to remove it"
assert_not_contains "$RUN_STDERR" "definitely-not-a-real-skill" "a skill of the user's own is not named"
