# `daisy init <home>` binds the workspace to the home's default identity:
# the `name=`/`email=` in `<home>/gitconfig` become the workspace
# repository's local user.name and user.email, no `.daisy/identity` is
# written (the default needs none), and `status`/`files` name the identity.
# `daisy clean` unsets exactly what still matches and says which keys.

printf 'name=Zebra Quillfeather\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"

fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "init with a home name exits 0"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/home")" "testhome" ".daisy/home names the home"
[ -e "$FIXTURE_WORKSPACE/.daisy/identity" ] && wrote=yes || wrote=no
assert_eq "$wrote" "no" "the default identity writes no .daisy/identity"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Zebra Quillfeather" "user.name comes from gitconfig"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email)" "fixture@example.invalid" "user.email comes from gitconfig"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "no sshkey, no core.sshCommand"
assert_contains "$RUN_STDOUT" "Identity: testhome" "init names the identity it applied"

fixture_run_cli status
assert_eq "$RUN_EXIT" "0" "status exits 0"
assert_contains "$RUN_STDOUT" "home: testhome | identity: testhome" "status prints the identity"

fixture_run files
assert_eq "$RUN_EXIT" "0" "files exits 0"
assert_contains "$RUN_STDOUT" "identity: testhome" "files prints the identity"
assert_contains "$RUN_STDOUT" "$FIXTURE_HOME/gitconfig" "files resolves the identity to its file"

# clean: every key that still matches the identity is unset, and the
# message names them rather than claiming a clear that did not happen.
fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "clean exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name || true)" "" "clean unsets user.name"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email || true)" "" "clean unsets user.email"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name, user.email)" "clean says which keys it cleared"

# A value changed by hand since init is not the identity's any more: left alone.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local user.email "other@example.invalid"
fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "clean exits 0 with a hand-edited identity"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email)" "other@example.invalid" "a hand-set email is left alone"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name || true)" "" "while the matching name is unset"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name)" "and the message names only the key it cleared"

# Nothing matching: no "cleared" claim at all.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local user.name "Someone Else"
git -C "$FIXTURE_WORKSPACE" config --local user.email "other@example.invalid"
fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "clean exits 0 when nothing matches"
assert_not_contains "$RUN_STDOUT" "Cleared local git identity" "clean does not claim to have cleared anything"
assert_contains "$RUN_STDOUT" "left as is" "clean says the identity was left"

# A linked worktree has no git config of its own: `git config --local` there
# writes the main checkout's. An identity bound in the worktree would be
# applied to, and later stripped from, a repository that is not this
# workspace, so init refuses and names where to run instead.
main="$FIXTURE_LOCAL/main"
git init -q "$main"
git -C "$main" -c user.name="Fixture Author" -c user.email="fixture@example.invalid" commit -q --allow-empty -m "main: initial"
fixture_run_cli init testhome "$main"
assert_eq "$RUN_EXIT" "0" "init in the main checkout exits 0"
wt="$FIXTURE_LOCAL/worktree"
git -C "$main" worktree add -q "$wt"
printf 'name=Other Person\nemail=other@example.invalid\n' > "$FIXTURE_HOME/gitconfig"  # so a write-through would show
fixture_run_cli init testhome "$wt"
assert_eq "$RUN_EXIT" "1" "init in a linked worktree is refused"
assert_contains "$RUN_STDERR" "linked worktree" "and says why"
assert_contains "$RUN_STDERR" "$main" "and names the main checkout"
[ -e "$wt/.daisy" ] && wrote=yes || wrote=no
assert_eq "$wrote" "no" "and writes nothing"
assert_eq "$(git -C "$main" config --local user.name)" "Zebra Quillfeather" "the main checkout's identity is untouched"
# A subdirectory of a worktree is not the worktree: it becomes its own repository.
mkdir -p "$wt/sub"
fixture_run_cli init testhome "$wt/sub"
assert_eq "$RUN_EXIT" "0" "init in a subdirectory of a worktree exits 0"
[ -d "$wt/sub/.git" ] && nested=yes || nested=no
assert_eq "$nested" "yes" "and makes it its own repository"
assert_eq "$(git -C "$main" config --local user.name)" "Zebra Quillfeather" "leaving the main checkout alone"

# GIT_DIR in the environment (as inside any git hook) must not steer the
# identity at a foreign repository: every git call resolves from its
# directory alone.
fresh="$FIXTURE_LOCAL/fresh"
mkdir -p "$fresh"
RUN_STDOUT=$(cd "$fresh" && GIT_DIR="$main/.git" daisy init testhome 2>/dev/null); RUN_EXIT=$?
assert_eq "$RUN_EXIT" "0" "init under an inherited GIT_DIR exits 0"
[ -d "$fresh/.git" ] && own=yes || own=no
assert_eq "$own" "yes" "the target gets its own repository"
assert_eq "$(git -C "$fresh" config --local user.name)" "Other Person" "and the identity"
assert_eq "$(git -C "$main" config --local user.name)" "Zebra Quillfeather" "while the GIT_DIR repository is untouched"
printf 'name=Zebra Quillfeather\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"

# A home whose gitconfig is still the template has no identity: init binds
# the workspace, applies nothing, records nothing, and says so.
cp "$REAL_DAISY_ROOT/daisy/templates/home/gitconfig" "$FIXTURE_HOME/gitconfig"
fixture_run_cli clean -f
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.name   # the hand-set values above
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.email
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "init with a template gitconfig exits 0"
assert_contains "$RUN_STDERR" "no complete default identity" "and warns, on stderr, that the home has no identity"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name || true)" "" "no placeholder is applied as a git identity"
[ -e "$FIXTURE_WORKSPACE/.daisy/gitconfig" ] && rec=yes || rec=no
assert_eq "$rec" "no" "and nothing is on record"
fixture_run_cli status
assert_contains "$RUN_STDERR" "$FIXTURE_HOME/gitconfig" "status keeps saying the bound identity file is not a complete identity"

# clean with nothing to unset against — no record, home unregistered — says
# so instead of skipping the identity silently.
printf 'name=Zebra Quillfeather\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"
fixture_run_cli init testhome
rm "$FIXTURE_WORKSPACE/.daisy/gitconfig"
fixture_unregister_home testhome
fixture_run_cli clean -f
fixture_register_home testhome "$FIXTURE_HOME"
assert_eq "$RUN_EXIT" "0" "clean exits 0 with no record and the home unregistered"
assert_contains "$RUN_STDOUT" "No readable .daisy/gitconfig record" "and says it had nothing to unset against"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Zebra Quillfeather" "leaving it in place"
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.name
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.email

# A workspace bound before records existed (no .daisy/gitconfig) falls back
# to the home's identity file, as clean always did.
fixture_run_cli init testhome
rm "$FIXTURE_WORKSPACE/.daisy/gitconfig"
fixture_run_cli clean -f
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name, user.email)" "clean unsets against the home's file when there is no record"

# A record without .daisy/home (init interrupted between the two writes, or
# the file removed by hand): clean still unsets what the record says.
fixture_run_cli init testhome
rm "$FIXTURE_WORKSPACE/.daisy/home"
fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "clean exits 0 with a record and no .daisy/home"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name, user.email)" "and unsets against the record"

# The binding is six symlinks as well as the identity; status says when one
# is missing rather than letting edits land in a plain file.
fixture_run_cli init testhome
rm "$FIXTURE_WORKSPACE/.daisy/today.md"
fixture_run_cli status
assert_eq "$RUN_EXIT" "0" "status still runs with a link missing"
assert_contains "$RUN_STDERR" "today.md" "and names the missing link"
assert_contains "$RUN_STDERR" "daisy init" "and the fix"
fixture_run_cli init testhome
fixture_run_cli status
assert_eq "$RUN_STDERR" "" "a re-init restores it and status is quiet"

# The author and committer a git hook exports to its environment must not
# override the identity: every Daisy commit is the home's.
seed_basic
export GIT_AUTHOR_NAME="Hook Author" GIT_AUTHOR_EMAIL="hook@example.invalid" GIT_COMMITTER_NAME="Hook Author" GIT_COMMITTER_EMAIL="hook@example.invalid"
fixture_run log "Logged from inside a hook"
unset GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL
assert_eq "$RUN_EXIT" "0" "log succeeds under a hook's environment"
assert_eq "$(git -C "$FIXTURE_HOME" log -1 --format='%an <%ae> %cn')" "Fixture Author <fixture@example.invalid> Fixture Author" \
    "and the commit carries the home repository's author, not the hook's"

# `git -c key=value` reaches hooks as GIT_CONFIG_PARAMETERS (and the
# GIT_CONFIG_COUNT family): the same class as the author variables.
export GIT_CONFIG_PARAMETERS="'user.name=Hook Author' 'user.email=hook@example.invalid'"
fixture_run log "Logged under a hook's -c parameters"
unset GIT_CONFIG_PARAMETERS
assert_eq "$RUN_EXIT" "0" "log succeeds under GIT_CONFIG_PARAMETERS"
assert_eq "$(git -C "$FIXTURE_HOME" log -1 --format='%an <%ae>')" "Fixture Author <fixture@example.invalid>" \
    "and the commit carries the home repository's author"
export GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=user.name GIT_CONFIG_VALUE_0="Hook Author"
fixture_run log "Logged under a hook's config count"
unset GIT_CONFIG_COUNT GIT_CONFIG_KEY_0 GIT_CONFIG_VALUE_0
assert_eq "$(git -C "$FIXTURE_HOME" log -1 --format='%an')" "Fixture Author" "likewise under GIT_CONFIG_COUNT"

# A bound identity that is not applied (here: the record removed and the
# config cleared by hand) is reported by status even without a record.
fixture_run_cli init testhome
rm "$FIXTURE_WORKSPACE/.daisy/gitconfig"
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.email
fixture_run_cli status
assert_contains "$RUN_STDERR" "user.email" "status says which key is not the bound identity's"
assert_contains "$RUN_STDERR" "daisy init" "and the fix"
fixture_run_cli init testhome

# A first init makes .daisy/ with the usual directory mode, not mktemp's.
fresh2="$FIXTURE_LOCAL/fresh2"
mkdir -p "$fresh2"
fixture_run_cli init testhome "$fresh2"
assert_eq "$(stat -c %a "$fresh2/.daisy")" "755" "a fresh .daisy/ is 755"
assert_eq "$(stat -c %a "$fresh2/.daisy/AGENTS.md")" "644" "and AGENTS.md is readable like the files beside it"

# Re-init on a .gitignore from before the leftover patterns existed adds them
# under the existing header, not a second header.
grep -v '^\.daisy\.' "$fresh2/.gitignore" > "$fresh2/.gitignore.tmp" && mv "$fresh2/.gitignore.tmp" "$fresh2/.gitignore"
echo ".daisy/" >> "$fresh2/.gitignore"
fixture_run_cli init testhome "$fresh2"
assert_eq "$(grep -c '^# Daisy workspace config' "$fresh2/.gitignore")" "1" "one Daisy header in .gitignore after a re-init"
assert_eq "$(grep -c '^\.daisy\.previous\.\*$' "$fresh2/.gitignore")" "1" "and the leftover pattern is present once"

# clean removes init's own leftovers too, so no record lingers untracked and
# un-ignored in the workspace repository.
mkdir "$fresh2/.daisy.previous.555"
cp "$fresh2/.daisy/gitconfig" "$fresh2/.daisy.previous.555/"
FIXTURE_CWD="$fresh2"
fixture_run_cli clean -f
FIXTURE_CWD=""
assert_eq "$RUN_EXIT" "0" "clean exits 0 with a leftover present"
[ -e "$fresh2/.daisy.previous.555" ] && left=yes || left=no
assert_eq "$left" "no" "and removes the leftover"
assert_contains "$RUN_STDOUT" ".daisy.previous.555" "saying so"
