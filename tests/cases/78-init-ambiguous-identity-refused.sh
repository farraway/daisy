# An identity name defined in two registered homes cannot be resolved: which
# home the workspace would bind to is exactly what the name was meant to
# decide. `daisy init` refuses, names both homes, and writes nothing. A
# registered home's own name is never ambiguous: it is that home's default
# identity even when another home defines an identity by the same name.

second="$FIXTURE_LOCAL/homes/second"
mkdir -p "$(dirname "$second")"
cp -r "$REAL_DAISY_ROOT/templates/home" "$second"
second=$(cd "$second" && pwd -P)
git init -q "$second"
git -C "$second" -c user.name="Fixture Author" -c user.email="fixture@example.invalid" commit -q --allow-empty -m "second: initial"
fixture_register_home second "$second"

mkdir -p "$FIXTURE_HOME/identities" "$second/identities"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$FIXTURE_HOME/identities/pelicanmoor"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$second/identities/pelicanmoor"

ws="$FIXTURE_LOCAL/ambiguous-ws"
mkdir -p "$ws"
fixture_run_cli init pelicanmoor "$ws"
assert_eq "$RUN_EXIT" "1" "init refuses an identity defined in two homes"
assert_contains "$RUN_STDERR" "identity 'pelicanmoor' is defined in more than one home: testhome, second" \
    "the error names the identity and both homes defining it"
[ -e "$ws/.daisy" ] && wrote=yes || wrote=no
assert_eq "$wrote" "no" "nothing is written to the workspace"
[ -e "$ws/.git" ] && repo=yes || repo=no
assert_eq "$repo" "no" "not even a repository"

# The existing workspace keeps its binding when a re-init is refused.
echo "testhome" > "$FIXTURE_WORKSPACE/.daisy/home"
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "1" "re-init with an ambiguous identity is refused too"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/home")" "testhome" "the workspace stays bound to its home"

# A home's name is its default identity, even if another home defines an
# identity by that name.
printf 'name=Imposter\nemail=imposter@example.invalid\n' > "$second/identities/testhome"
printf 'name=Zebra Quillfeather\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "init with a home name succeeds despite a same-named identity elsewhere"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/home")" "testhome" "the home wins"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Zebra Quillfeather" "and its gitconfig is applied"

# Once only one home defines the identity, it resolves.
rm "$second/identities/pelicanmoor"
fixture_run_cli init pelicanmoor "$ws"
assert_eq "$RUN_EXIT" "0" "init resolves an identity defined in one home"
assert_eq "$(cat "$ws/.daisy/home")" "testhome" "to that home"

# A switch of home that cannot finish (here, a corrupt settings file jq
# cannot read) leaves the binding whole: .daisy/home, the symlinks and the
# identity all still the old home's.
printf 'name=Second Person\nemail=second@example.invalid\n' > "$second/gitconfig"
fixture_run_cli init testhome
echo '{ not json' > "$FIXTURE_WORKSPACE/.claude/settings.local.json"
fixture_run_cli init second
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "a home switch that cannot finish exits non-zero"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/home")" "testhome" ".daisy/home still names the old home"
assert_eq "$(readlink "$FIXTURE_WORKSPACE/.daisy/tasks")" "$FIXTURE_HOME/tasks" "and the symlinks still point into it"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Zebra Quillfeather" "and its identity is still applied"
assert_contains "$(grep -m1 '^# Home:' "$FIXTURE_WORKSPACE/.daisy/AGENTS.md")" "testhome" "and AGENTS.md is still the old home's"
rm "$FIXTURE_WORKSPACE/.claude/settings.local.json"
fixture_run_cli init second
assert_eq "$RUN_EXIT" "0" "the switch succeeds once the obstacle is gone"
assert_eq "$(readlink "$FIXTURE_WORKSPACE/.daisy/tasks")" "$second/tasks" "and the symlinks follow"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Second Person" "as does the identity"

# A first init that cannot finish leaves no .daisy/ behind: a bare one would
# make the directory a workspace that resolves to the enclosing workspace's
# home, not the one asked for.
proj="$FIXTURE_WORKSPACE/proj"
mkdir -p "$proj/.claude"
echo '{ not json' > "$proj/.claude/settings.local.json"
fixture_run_cli init testhome "$proj"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "a first init that cannot finish exits non-zero"
[ -e "$proj/.daisy" ] && left=yes || left=no
assert_eq "$left" "no" "and leaves no .daisy/ behind"
FIXTURE_CWD="$proj"
fixture_run_cli status
FIXTURE_CWD=""
assert_contains "$RUN_STDOUT" "workspace: $FIXTURE_WORKSPACE" "so the directory is still part of the enclosing workspace"

# A registered home whose path is unavailable might define the identity too:
# the name cannot be resolved until the registry is whole.
fixture_register_home away "$FIXTURE_LOCAL/homes/not-mounted"
fixture_run_cli init pelicanmoor "$ws"
assert_eq "$RUN_EXIT" "1" "init refuses an identity while a registered home cannot be checked"
assert_contains "$RUN_STDERR" "away" "naming that home"
assert_contains "$RUN_STDERR" "does not exist" "and what is wrong with it"
fixture_unregister_home away
fixture_run_cli init testhome "$ws"
assert_eq "$RUN_EXIT" "0" "a home name still resolves by itself"
fixture_register_home away "$FIXTURE_LOCAL/homes/not-mounted"
fixture_run_cli init testhome "$ws"
assert_eq "$RUN_EXIT" "0" "even while another home cannot be checked"
fixture_unregister_home away

# A hand-edited registry name containing a space must not swallow a later
# home whose name is one of its words: the ambiguity is still seen.
spaced="$FIXTURE_LOCAL/homes/spaced"
mkdir -p "$spaced/identities" "$second/identities"
rm -f "$FIXTURE_HOME/identities/pelicanmoor"  # so only 'a second' and 'second' define it
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$spaced/identities/pelicanmoor"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$second/identities/pelicanmoor"
printf 'a second=%s\n' "$spaced" >> "$DAISY_CONFIG/homes"
fixture_run_cli init pelicanmoor "$ws"
assert_eq "$RUN_EXIT" "1" "init still refuses the identity defined in two homes"
assert_contains "$RUN_STDERR" "more than one home" "and says so"

# A first init that fails inside the final section (a stale lock stops the
# first git config call) must leave no .daisy/ either, and no identity.
proj2="$FIXTURE_WORKSPACE/proj2"
git init -q "$proj2"
touch "$proj2/.git/config.lock"
fixture_run_cli init testhome "$proj2"
rm "$proj2/.git/config.lock"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "a first init stopped by a config lock exits non-zero"
assert_contains "$RUN_STDERR" "did not finish" "and says it did not finish"
[ -e "$proj2/.daisy" ] && left=yes || left=no
assert_eq "$left" "no" "and leaves no .daisy/ behind"
assert_eq "$(git -C "$proj2" config --local user.name || true)" "" "and no identity"

# Likewise when the binding cannot be prepared (here, a directory sits where
# the record goes): init refuses before writing anything.
proj3="$FIXTURE_WORKSPACE/proj3"
mkdir -p "$proj3/.daisy/gitconfig"
fixture_run_cli init testhome "$proj3"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "an init that cannot write its record exits non-zero"
[ -e "$proj3/.git" ] && repo=yes || repo=no
assert_eq "$repo" "no" "and, refused before its first write, makes no repository to apply an identity to"
[ -e "$proj3/.daisy/home" ] && bound=yes || bound=no
assert_eq "$bound" "no" "and no binding is written"

# The state a kill between init's two renames leaves — .daisy/ aside and no
# .daisy/ — is not a directory inside the enclosing workspace: every command
# there refuses and names the repair, instead of resolving the parent's home
# while the repository carries the child's identity.
child="$FIXTURE_WORKSPACE/child"
mkdir -p "$child"
fixture_run_cli init second "$child"
mv "$child/.daisy" "$child/.daisy.previous.4242"
before=$(fixture_commit_count)
FIXTURE_CWD="$child"
fixture_run_cli status
assert_eq "$RUN_EXIT" "1" "status in a directory with an interrupted init refuses"
assert_contains "$RUN_STDERR" ".daisy.previous.4242" "naming the leftover"
assert_contains "$RUN_STDERR" "daisy init" "and the repair"
fixture_run_cli log "must not land anywhere"
assert_eq "$RUN_EXIT" "1" "a script run there refuses too"
FIXTURE_CWD=""
assert_eq "$(fixture_commit_count)" "$before" "and no commit was made in the parent's home"
assert_not_contains "$RUN_STDERR" "Not in a Daisy workspace" "and does not also claim there is no workspace"
mv "$child/.daisy.previous.4242" "$child/.daisy"

# A .daisy/ without its home file is an incomplete binding of this directory,
# not a reason to read the enclosing workspace's.
rm "$child/.daisy/home"
before=$(fixture_commit_count)
FIXTURE_CWD="$child"
fixture_run_cli status
assert_eq "$RUN_EXIT" "1" "status in a directory whose binding lacks .daisy/home refuses"
assert_contains "$RUN_STDERR" "daisy init" "and names the repair"
fixture_run_cli log "must not land anywhere either"
assert_eq "$RUN_EXIT" "1" "a script run there refuses too"
FIXTURE_CWD=""
assert_eq "$(fixture_commit_count)" "$before" "and no commit was made in the parent's home"
echo "second" > "$child/.daisy/home"

# A .daisy that is a plain file beside a leftover: nothing resolves upward.
mv "$child/.daisy" "$child/.daisy.previous.4343"
echo "junk" > "$child/.daisy"
FIXTURE_CWD="$child"
fixture_run_cli status
assert_eq "$RUN_EXIT" "1" "status refuses when .daisy is not a directory"
assert_contains "$RUN_STDERR" "not a directory" "and says so"
FIXTURE_CWD=""
rm "$child/.daisy"
mv "$child/.daisy.previous.4343" "$child/.daisy"

# A dangling .daisy symlink is a broken binding of this directory, not a
# reason to read the enclosing workspace's.
mv "$child/.daisy" "$child/.daisy.aside"
ln -s /nonexistent "$child/.daisy"
FIXTURE_CWD="$child"
fixture_run_cli status
assert_eq "$RUN_EXIT" "1" "status refuses when .daisy is a dangling symlink"
assert_contains "$RUN_STDERR" "symlink" "and says so"
FIXTURE_CWD=""
rm "$child/.daisy"
mv "$child/.daisy.aside" "$child/.daisy"

# The Daisy root is itself a directory named .daisy under the user's home:
# it is never a workspace binding, so a walk up from beside it finds no
# workspace rather than an incomplete one — and clean there removes nothing.
root2="$FIXTURE_LOCAL/home2/.daisy"
mkdir -p "$root2" "$FIXTURE_LOCAL/home2/bin"
mkdir -p "$root2/templates"
RUN_STDERR=$(cd "$FIXTURE_LOCAL/home2/bin" && DAISY_ROOT="$root2" daisy status 2>&1 >/dev/null); RUN_EXIT=$?
assert_eq "$RUN_EXIT" "1" "status beside the Daisy root finds no workspace"
assert_contains "$RUN_STDERR" "Not in a Daisy workspace" "and says so"
assert_not_contains "$RUN_STDERR" "incomplete" "without mistaking the root for a binding"
RUN_STDOUT=$(cd "$FIXTURE_LOCAL/home2/bin" && DAISY_ROOT="$root2" daisy clean -f 2>/dev/null); RUN_EXIT=$?
assert_eq "$RUN_EXIT" "1" "clean beside the Daisy root refuses"
[ -e "$root2/templates" ] && kept=yes || kept=no
assert_eq "$kept" "yes" "and the root is untouched"
# Nor may init bind the directory holding the root: its .daisy IS the root,
# and the swap would move it aside and remove it.
RUN_STDERR=$(cd "$FIXTURE_LOCAL/home2" && DAISY_ROOT="$root2" daisy init testhome 2>&1 >/dev/null); RUN_EXIT=$?
assert_eq "$RUN_EXIT" "1" "init refuses the directory whose .daisy is the Daisy root"
assert_contains "$RUN_STDERR" "Daisy root" "and says why"
[ -e "$root2/templates" ] && kept=yes || kept=no
assert_eq "$kept" "yes" "and the root is still there"
[ -e "$FIXTURE_LOCAL/home2/.git" ] && repo=yes || repo=no
assert_eq "$repo" "no" "having written nothing"

# A root reached through a symlink is still the root, not a symlinked binding
# to move aside.
mkdir -p "$FIXTURE_LOCAL/home3/bin"
ln -s "$root2" "$FIXTURE_LOCAL/home3/.daisy"
RUN_STDERR=$(cd "$FIXTURE_LOCAL/home3/bin" && DAISY_ROOT="$FIXTURE_LOCAL/home3/.daisy" daisy status 2>&1 >/dev/null); RUN_EXIT=$?
assert_eq "$RUN_EXIT" "1" "status beside a symlinked root finds no workspace"
assert_contains "$RUN_STDERR" "Not in a Daisy workspace" "and says so"
assert_not_contains "$RUN_STDERR" "move it aside" "without advising to move the root"
RUN_STDERR=$(cd "$FIXTURE_LOCAL/home3" && DAISY_ROOT="$FIXTURE_LOCAL/home3/.daisy" daisy init testhome 2>&1 >/dev/null); RUN_EXIT=$?
assert_eq "$RUN_EXIT" "1" "init beside a symlinked root is refused"
assert_contains "$RUN_STDERR" "Daisy root" "as the root, not as a symlink to move aside"

# A stage leftover in a directory that is no longer a repository root belongs
# to no repository init may touch: the enclosing repository's identity stays.
enc="$FIXTURE_LOCAL/enclosing"
git init -q "$enc"
git -C "$enc" config --local user.name "Pelican Moorhen"
git -C "$enc" config --local user.email "pelican@example.invalid"
mkdir -p "$enc/sub/.daisy.stage.abcdef"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$enc/sub/.daisy.stage.abcdef/gitconfig"
fixture_run_cli init testhome "$enc/sub"
assert_eq "$RUN_EXIT" "0" "init in a subdirectory with a stage leftover exits 0"
assert_eq "$(git -C "$enc" config --local user.name)" "Pelican Moorhen" "the enclosing repository's identity is untouched"
assert_not_contains "$RUN_STDOUT" "Cleared the git identity" "and no clearing is claimed"
[ -e "$enc/sub/.daisy.stage.abcdef" ] && left=yes || left=no
assert_eq "$left" "no" "while the leftover is removed"

fixture_unregister_home second
