# daisy install --update must remove a previously-installed skill copy from
# $HOME/.claude/skills once its source no longer exists in $DAISY_ROOT/skills
# (defect 6a — daisy.sh:427-439 used to rm -rf only the names it was about to
# write, leaving deleted skills' copies behind forever) — but never `synced`,
# where Claude Code's own skill sync writes (D47: pruned, Claude Code
# recreated it minutes later, and the health check warned forever).

fixture_setup_home_override

# install takes the root of the binary that runs (D47) and refuses a
# DAISY_ROOT naming another, so the fixture root gets its own binary: a
# copy, since a link resolves back to the real root. Excluded, as tests/ is.
mkdir -p "$FIXTURE_ROOT/bin"
cp "$(command -v daisy)" "$FIXTURE_ROOT/bin/daisy"
echo "/bin" >> "$FIXTURE_ROOT/.git/info/exclude"

stale_dir="$FIXTURE_HOME_OVERRIDE/.claude/skills/definitely-not-a-real-skill"
mkdir -p "$stale_dir"
echo "---
name: definitely-not-a-real-skill
description: stale fixture skill, has no source counterpart
---
" > "$stale_dir/SKILL.md"
synced_dir="$FIXTURE_HOME_OVERRIDE/.claude/skills/synced"
mkdir -p "$synced_dir/abc"
echo "Claude Code's" > "$synced_dir/abc/SKILL.md"

PATH="$FIXTURE_ROOT/bin:$PATH" fixture_run_cli install --update
assert_eq "$RUN_EXIT" "0" "daisy install --update exits 0"

if [ -d "$stale_dir" ]; then
    echo "    ASSERT FAILED: stale skill directory survived install --update"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi
assert_eq "$(cat "$synced_dir/abc/SKILL.md" 2>/dev/null)" "Claude Code's" \
    "synced is left as it was"

# A real source skill (testing ships in $DAISY_ROOT/skills) must
# still be installed — pruning must not remove current skills.
real_skill_dir="$FIXTURE_HOME_OVERRIDE/.claude/skills/testing"
if [ ! -d "$real_skill_dir" ]; then
    echo "    ASSERT FAILED: current skill 'testing' was not installed"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes after the install"
assert_not_contains "$RUN_STDERR" "skill '" "healthcheck warns about no skill, synced included"
