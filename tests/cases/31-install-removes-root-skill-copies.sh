# daisy install writes nothing under $HOME/.claude: skills and permissions
# are init's, per workspace. install --update takes back what earlier
# installs left there: a copy of a root skill, here one an older root
# installed (its SKILL.md a version in the root's history, not the current
# one), but never a user's own skill, a root-named one the user edited, or
# `synced`, where Claude Code's own skill sync writes (D47: pruned, Claude
# Code recreated it minutes later, and the health check warned forever).

fixture_setup_home_override

# install takes the root of the binary that runs (D47) and refuses a
# DAISY_ROOT naming another, so the fixture root gets its own binary: a
# copy, since a link resolves back to the real root. Excluded, as tests/ is.
mkdir -p "$FIXTURE_ROOT/bin"
cp "$(command -v daisy)" "$FIXTURE_ROOT/bin/daisy"
echo "/bin" >> "$FIXTURE_ROOT/.git/info/exclude"

claude="$FIXTURE_HOME_OVERRIDE/.claude"
skills="$claude/skills"
mkdir -p "$skills"
# The copy an older root installed; then the root moves on.
cp -r "$FIXTURE_ROOT/skills/testing" "$skills/testing"
echo "A later rule." >> "$FIXTURE_ROOT/skills/testing/SKILL.md"
git -C "$FIXTURE_ROOT" commit -q -am "fixture: a later testing skill"
cp -r "$FIXTURE_ROOT/skills/git" "$skills/git"
echo "My own rule." >> "$skills/git/SKILL.md"
mkdir -p "$skills/definitely-not-a-root-skill" "$skills/synced/abc"
printf -- '---\nname: definitely-not-a-root-skill\ndescription: the user'"'"'s own\n---\n' > "$skills/definitely-not-a-root-skill/SKILL.md"
echo "Claude Code's" > "$skills/synced/abc/SKILL.md"
printf '{\n  "model": "x",\n  "permissions": {"allow": ["Bash(ls:*)"]}\n}\n' > "$claude/settings.json"
find "$claude" -exec touch -h -d 2020-01-01 {} +
marker="$FIXTURE_HOME_OVERRIDE/marker"
touch -d 2021-01-01 "$marker"
snapshot() { (cd "$claude" && find . -path ./skills/testing -prune -o -type f -print0 | sort -z | xargs -0 sha256sum); }
before=$(snapshot)

PATH="$FIXTURE_ROOT/bin:$PATH" fixture_run_cli install --update
assert_eq "$RUN_EXIT" "0" "daisy install --update exits 0"
assert_contains "$RUN_STDOUT" "Removed 1 copy of a root skill from ~/.claude/skills (testing)" "names the copy it removed"
assert_contains "$RUN_STDOUT" "Kept ~/.claude/skills/git: named as a root skill but holding a file" "names the edited one it kept"

[ -e "$skills/testing" ] && left=yes || left=no
assert_eq "$left" "no" "the older root's copy is gone"
assert_eq "$(snapshot)" "$before" "every other file under ~/.claude is as it was"
assert_eq "$(find "$claude" -newer "$marker" -type f)" "" "no file under ~/.claude was written"
[ -e "$skills/synced/abc/SKILL.md" ] && [ -e "$skills/definitely-not-a-root-skill/SKILL.md" ] && [ -e "$skills/git/SKILL.md" ] && kept=yes || kept=no
assert_eq "$kept" "yes" "synced, the user's skill and the edited one stay"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes after the install"
assert_not_contains "$RUN_STDERR" "skill '" "healthcheck warns about no skill, synced included"
