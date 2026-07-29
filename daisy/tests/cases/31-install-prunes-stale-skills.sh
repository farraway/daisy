# daisy install --update must remove a previously-installed skill copy from
# $HOME/.claude/skills once its source no longer exists in $DAISY_ROOT/skills
# (defect 6a — daisy.sh:427-439 used to rm -rf only the names it was about to
# write, leaving deleted skills' copies behind forever).

fixture_setup_home_override

stale_dir="$FIXTURE_HOME_OVERRIDE/.claude/skills/definitely-not-a-real-skill"
mkdir -p "$stale_dir"
echo "---
name: definitely-not-a-real-skill
description: stale fixture skill, has no source counterpart
---
" > "$stale_dir/SKILL.md"

fixture_run_cli install --update
assert_eq "$RUN_EXIT" "0" "daisy install --update exits 0"

if [ -d "$stale_dir" ]; then
    echo "    ASSERT FAILED: stale skill directory survived install --update"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi

# A real source skill (writing-great-skills ships in $DAISY_ROOT/skills) must
# still be installed — pruning must not remove current skills.
real_skill_dir="$FIXTURE_HOME_OVERRIDE/.claude/skills/writing-great-skills"
if [ ! -d "$real_skill_dir" ]; then
    echo "    ASSERT FAILED: current skill 'writing-great-skills' was not installed"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi
