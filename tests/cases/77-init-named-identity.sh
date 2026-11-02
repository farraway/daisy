# `daisy init <identity>` where the name is not a home but a file
# `<home>/identities/<identity>` in exactly one registered home: the
# workspace binds to that home (`.daisy/home`, the symlinks) and records the
# identity (`.daisy/identity`); user.name, user.email and, from `sshkey=`,
# core.sshCommand come from that file. Re-initialising with the home's own
# name afterwards returns to the default identity: `.daisy/identity` goes,
# core.sshCommand goes with it, and name/email are gitconfig's again.

printf 'name=Zebra Quillfeather\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"
key="$FIXTURE_HOME_PARENT/id_pelicanmoor"
touch "$key"
mkdir -p "$FIXTURE_HOME/identities"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$FIXTURE_HOME/identities/pelicanmoor"

fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "init with an identity name exits 0"
assert_contains "$RUN_STDOUT" "Home: testhome" "init names the home the identity belongs to"
assert_contains "$RUN_STDOUT" "Identity: pelicanmoor" "init names the identity it applied"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/home")" "testhome" ".daisy/home is the identity's home"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/identity")" "pelicanmoor" ".daisy/identity records the identity"
assert_eq "$(readlink "$FIXTURE_WORKSPACE/.daisy/tasks")" "$FIXTURE_HOME/tasks" "the symlinks point into that home"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Pelican Moorhen" "user.name comes from the identity file"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email)" "pelican@example.invalid" "user.email comes from the identity file"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -i $key -o IdentitiesOnly=yes" \
    "core.sshCommand pins the identity's key"
assert_not_contains "$RUN_STDOUT" "not found" "an existing key draws no warning"

fixture_run_cli status
assert_eq "$RUN_EXIT" "0" "status exits 0 under a named identity"
assert_contains "$RUN_STDOUT" "home: testhome | identity: pelicanmoor" "status prints home and identity"

fixture_run files
assert_eq "$RUN_EXIT" "0" "files exits 0 under a named identity"
assert_contains "$RUN_STDOUT" "identity: pelicanmoor" "files prints the identity"
assert_contains "$RUN_STDOUT" "$FIXTURE_HOME/identities/pelicanmoor" "files resolves the identity to its file"

# Back to the default identity of the same home.
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "re-init with the home name exits 0"
assert_contains "$RUN_STDOUT" "pelicanmoor → testhome" "init reports the identity switch"
[ -e "$FIXTURE_WORKSPACE/.daisy/identity" ] && kept=yes || kept=no
assert_eq "$kept" "no" ".daisy/identity is removed for the default identity"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/home")" "testhome" ".daisy/home is unchanged"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Zebra Quillfeather" "user.name is gitconfig's again"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email)" "fixture@example.invalid" "user.email is gitconfig's again"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "core.sshCommand is cleared"

fixture_run_cli status
assert_contains "$RUN_STDOUT" "home: testhome | identity: testhome" "status is back to the default identity"

# Re-running with the same identity is a no-op on the binding and reports it.
fixture_run_cli init pelicanmoor
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "re-init with the same identity exits 0"
assert_contains "$RUN_STDOUT" "Already initialized" "and says so"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/identity")" "pelicanmoor" ".daisy/identity still records the identity"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -i $key -o IdentitiesOnly=yes" "core.sshCommand still pins the key"

# An identity name that is neither a home nor an identity of one is refused
# before anything is written, and the error lists what exists.
mkdir -p "$FIXTURE_LOCAL/fresh-ws"
fixture_run_cli init nosuchone "$FIXTURE_LOCAL/fresh-ws"
assert_eq "$RUN_EXIT" "1" "init refuses an unknown identity"
assert_contains "$RUN_STDERR" "nosuchone" "the error names the identity"
assert_contains "$RUN_STDERR" "pelicanmoor" "and lists the identities that exist"
[ -e "$FIXTURE_LOCAL/fresh-ws/.daisy" ] && wrote=yes || wrote=no
assert_eq "$wrote" "no" "and writes nothing"

# An identity is a file name in identities/: a name that is a path is not one.
fixture_run_cli init ../gitconfig "$FIXTURE_LOCAL/fresh-ws"
assert_eq "$RUN_EXIT" "1" "init refuses an identity name that is a path"
assert_contains "$RUN_STDERR" "not a valid identity name" "and says why"
[ -e "$FIXTURE_LOCAL/fresh-ws/.daisy" ] && wrote=yes || wrote=no
assert_eq "$wrote" "no" "and writes nothing"

# A failure in the middle of a re-init (here, a corrupt settings file jq
# cannot read) must leave the workspace as it was: still bound to, and
# committing as, the previous identity. Binding and identity are the last
# step, unset and apply together.
echo "testhome" > "$FIXTURE_WORKSPACE/.daisy/home"
fixture_run_cli init pelicanmoor
echo '{ not json' > "$FIXTURE_WORKSPACE/.claude/settings.local.json"
fixture_run_cli init testhome
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "a re-init that cannot finish exits non-zero"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/identity")" "pelicanmoor" "the binding still names the previous identity"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Pelican Moorhen" "and its user.name is still applied"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -i $key -o IdentitiesOnly=yes" "and its key"
assert_eq "$(head -1 "$FIXTURE_WORKSPACE/.daisy/gitconfig")" "name=Pelican Moorhen" "and the record still matches what is applied"
rm "$FIXTURE_WORKSPACE/.claude/settings.local.json"
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "the re-init succeeds once the obstacle is gone"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "and the previous key is cleared"

# An empty .daisy/identity is the default identity, not an identity named "".
: > "$FIXTURE_WORKSPACE/.daisy/identity"
fixture_run_cli status
assert_eq "$RUN_EXIT" "0" "status exits 0 with an empty .daisy/identity"
assert_contains "$RUN_STDOUT" "identity: testhome" "and reads it as the default identity"

# The record is init's own file: one made read-only by hand is replaced, not
# an obstacle.
fixture_run_cli init pelicanmoor
chmod a-w "$FIXTURE_WORKSPACE/.daisy/gitconfig"
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "a read-only record does not stop a re-init"
assert_eq "$(head -1 "$FIXTURE_WORKSPACE/.daisy/gitconfig")" "name=Zebra Quillfeather" "and is rewritten"

# No ordering of writes can make every failure impossible, so status and
# files check the record against the repository and the bound identity
# file and say when they disagree.
git -C "$FIXTURE_WORKSPACE" config --local user.email "other@example.invalid"
fixture_run_cli status
assert_contains "$RUN_STDERR" "differs" "status warns that the applied identity differs from the record"
assert_contains "$RUN_STDERR" "daisy init" "naming the fix"
fixture_run_cli init testhome
fixture_run_cli status
assert_not_contains "$RUN_STDERR" "differs" "after a clean re-init there is no warning"

# The identity file edited after init: the record no longer matches it.
printf 'name=Zebra Quillfeather\nemail=changed@example.invalid\n' > "$FIXTURE_HOME/gitconfig"
fixture_run_cli status
assert_contains "$RUN_STDERR" "does not match what init recorded" "status warns that the identity file no longer matches the record"
printf 'name=Zebra Quillfeather\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"

# A key the record never claimed is not drift: a hand-set core.sshCommand
# under a key-less identity is the user's, and status does not nag.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand "ssh -v"
fixture_run_cli status
assert_eq "$RUN_STDERR" "" "a hand-set core.sshCommand under a key-less identity draws no warning"
git -C "$FIXTURE_WORKSPACE" config --local --unset core.sshCommand

# A failure while the new binding is being prepared (a directory sits where
# .daisy/identity goes) leaves the binding and the identity untouched: the
# binding is swapped in whole or not at all.
fixture_run_cli init testhome
mkdir "$FIXTURE_WORKSPACE/.daisy/identity"
fixture_run_cli init pelicanmoor
rmdir "$FIXTURE_WORKSPACE/.daisy/identity"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "an init that cannot prepare its binding exits non-zero"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/home")" "testhome" ".daisy/home is unchanged"
assert_eq "$(readlink "$FIXTURE_WORKSPACE/.daisy/tasks")" "$FIXTURE_HOME/tasks" "the links are unchanged"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Zebra Quillfeather" "and so is the identity"
assert_eq "$(head -1 "$FIXTURE_WORKSPACE/.daisy/gitconfig")" "name=Zebra Quillfeather" "and the record"
fixture_run_cli status
assert_eq "$RUN_STDERR" "" "and status sees nothing out of place"

# .daisy that is not a directory is refused before anything happens, and kept.
plain="$FIXTURE_LOCAL/plain-ws"
mkdir -p "$plain"
echo "user data" > "$plain/.daisy"
fixture_run_cli init testhome "$plain"
assert_eq "$RUN_EXIT" "1" "init refuses a .daisy that is not a directory"
assert_contains "$RUN_STDERR" "not a directory" "and says so"
assert_eq "$(cat "$plain/.daisy")" "user data" "and leaves the file alone"
[ -e "$plain/.git" ] && repo=yes || repo=no
assert_eq "$repo" "no" "having written nothing"

# The final step failing after the previous identity was taken back (the
# new identity's apply, forced through a PATH-injected git) rolls everything
# back: the identity cleared is put back, .daisy/ is untouched, and nothing
# is left beside .daisy/. (The swap's own rename failing is the binary's
# TestSwapFailureRollsBack: no PATH command stands in for a rename.)
fixture_run_cli init pelicanmoor
echo "alpha" > "$FIXTURE_WORKSPACE/.daisy/project"
fakebin=$(mktemp -d)
real_git=$(command -v git)
cat > "$fakebin/git" <<EOF2
#!/usr/bin/env bash
# Fail the apply; pass everything else on, the rollback's own restore included.
case " \$* " in *" --replace-all user.name "*) echo "git: forced failure" >&2; exit 1 ;; esac
exec "$real_git" "\$@"
EOF2
chmod +x "$fakebin/git"
RUN_STDOUT=$(cd "$FIXTURE_WORKSPACE" && PATH="$fakebin:$PATH" daisy init testhome 2>"$fakebin/err"); RUN_EXIT=$?
RUN_STDERR=$(cat "$fakebin/err")
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "an init whose final step fails exits non-zero"
assert_contains "$RUN_STDERR" "did not finish" "and says it did not finish"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/identity")" "pelicanmoor" ".daisy/ is untouched, still bound to the previous identity"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/project")" "alpha" "with the project binding intact"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Pelican Moorhen" "the previous identity is applied again"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -i $key -o IdentitiesOnly=yes" "with its key"
assert_eq "$(ls -d "$FIXTURE_WORKSPACE"/.daisy.* 2>/dev/null | wc -l | tr -d ' ')" "0" "and nothing is left beside .daisy/"
rm "$FIXTURE_WORKSPACE/.daisy/project"  # (the fixture has no project 'alpha'; status would say so)
fixture_run_cli status
assert_eq "$RUN_STDERR" "" "and status sees nothing out of place"
rm -rf "$fakebin"

# What a kill between the two renames would leave — .daisy/ aside and no
# .daisy/ — is reclaimed by the next init before it does anything else.
mv "$FIXTURE_WORKSPACE/.daisy" "$FIXTURE_WORKSPACE/.daisy.previous.12345"
mkdir "$FIXTURE_WORKSPACE/.daisy.stage.abcdef"
# Only init's own shapes are its leftovers: a user's directory, a file or a
# symlink that happens to resemble one is left alone.
mkdir "$FIXTURE_WORKSPACE/.daisy.previous.bak"
echo "my notes" > "$FIXTURE_WORKSPACE/.daisy.previous.bak/notes"
ln -s "$FIXTURE_LOCAL" "$FIXTURE_WORKSPACE/.daisy.previous.777"
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "init after an interrupted swap exits 0"
assert_contains "$RUN_STDERR" ".daisy.previous.12345" "and says what it reclaimed"
assert_contains "$RUN_STDOUT" "Already initialized with identity: pelicanmoor" "the reclaimed binding is the one re-initialised"
[ -e "$FIXTURE_WORKSPACE/.daisy.stage.abcdef" ] && stage=yes || stage=no
assert_eq "$stage" "no" "and its staging leftover is gone"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy.previous.bak/notes")" "my notes" "a user's look-alike directory is untouched"
[ -L "$FIXTURE_WORKSPACE/.daisy.previous.777" ] && link=yes || link=no
assert_eq "$link" "yes" "and so is a symlink"
assert_eq "$(stat -c %a "$FIXTURE_WORKSPACE/.daisy")" "755" "a reclaimed .daisy/ keeps its mode"
rm -rf "$FIXTURE_WORKSPACE/.daisy.previous.bak" "$FIXTURE_WORKSPACE/.daisy.previous.777"

# Two of init's own leftovers and no .daisy/: init cannot know which binding
# is the real one and refuses rather than guess.
mv "$FIXTURE_WORKSPACE/.daisy" "$FIXTURE_WORKSPACE/.daisy.previous.100"
mkdir "$FIXTURE_WORKSPACE/.daisy.previous.200"
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "1" "init refuses with two leftovers and no .daisy/"
assert_contains "$RUN_STDERR" ".daisy.previous.100" "naming the first"
assert_contains "$RUN_STDERR" ".daisy.previous.200" "and the second"
[ -d "$FIXTURE_WORKSPACE/.daisy.previous.100" ] && kept=yes || kept=no
assert_eq "$kept" "yes" "and removes neither"
rmdir "$FIXTURE_WORKSPACE/.daisy.previous.200"
mv "$FIXTURE_WORKSPACE/.daisy.previous.100" "$FIXTURE_WORKSPACE/.daisy"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" check-ignore .daisy.previous.1 .daisy.stage.x | wc -l | tr -d ' ')" "2" \
    "and such leftovers are ignored by the workspace repository"

# Files init owns are its own: a symlink standing where .daisy/home goes is
# not written through, and a .daisy that is itself a symlink is refused.
fixture_run_cli init testhome
mv "$FIXTURE_WORKSPACE/.daisy/home" "$FIXTURE_WORKSPACE/realhome"
echo "stale" > "$FIXTURE_WORKSPACE/realhome"
ln -s ../realhome "$FIXTURE_WORKSPACE/.daisy/home"
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "init over a symlinked .daisy/home exits 0"
[ -L "$FIXTURE_WORKSPACE/.daisy/home" ] && link=yes || link=no
assert_eq "$link" "no" ".daisy/home is a file of init's own again"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/home")" "testhome" "holding the home"
assert_eq "$(cat "$FIXTURE_WORKSPACE/realhome")" "stale" "and the link's target was not written through"
rm "$FIXTURE_WORKSPACE/realhome"
shared="$FIXTURE_LOCAL/shared-binding"
mv "$FIXTURE_WORKSPACE/.daisy" "$shared"
ln -s "$shared" "$FIXTURE_WORKSPACE/.daisy"
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "1" "init refuses a .daisy that is a symlink"
assert_contains "$RUN_STDERR" "symlink" "and says so"
assert_eq "$(cat "$shared/identity")" "pelicanmoor" "leaving the target as it was"
rm "$FIXTURE_WORKSPACE/.daisy"
mv "$shared" "$FIXTURE_WORKSPACE/.daisy"

# Leftover shapes are judged by name alone: an ancestor directory that
# resembles one does not hide a real leftover, and a digit-less
# `.daisy.previous.` is nobody's leftover.
odd="$FIXTURE_LOCAL/.daisy.previous.5/ws"
mkdir -p "$odd"
fixture_run_cli init testhome "$odd"
echo "alpha" > "$odd/.daisy/project"
mv "$odd/.daisy" "$odd/.daisy.previous.12345"
mkdir "$odd/.daisy.previous."
echo "notes" > "$odd/.daisy.previous./notes.txt"
fixture_run_cli init testhome "$odd"
assert_eq "$RUN_EXIT" "0" "init under an odd ancestor exits 0"
assert_contains "$RUN_STDERR" ".daisy.previous.12345" "and reclaims the real leftover"
assert_eq "$(cat "$odd/.daisy/project")" "alpha" "with its project binding"
assert_eq "$(cat "$odd/.daisy.previous./notes.txt")" "notes" "while the digit-less directory is left alone"

# A partial apply (the second git config call fails, forced through a
# PATH-injected git) is rolled back like a whole one.
fakebin=$(mktemp -d)
real_git=$(command -v git)
cat > "$fakebin/git" <<EOF2
#!/usr/bin/env bash
case " \$* " in *" --replace-all user.email "*) echo "git: forced failure" >&2; exit 1 ;; esac
exec "$real_git" "\$@"
EOF2
chmod +x "$fakebin/git"
part="$FIXTURE_LOCAL/partial-ws"
mkdir -p "$part"
RUN_STDOUT=$(cd "$part" && PATH="$fakebin:$PATH" daisy init testhome 2>"$fakebin/err"); RUN_EXIT=$?
RUN_STDERR=$(cat "$fakebin/err")
rm -rf "$fakebin"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "an init whose apply fails part-way exits non-zero"
assert_eq "$(git -C "$part" config --local user.name || true)" "" "and the key it did set is taken back"
[ -e "$part/.daisy" ] && left=yes || left=no
assert_eq "$left" "no" "and no .daisy/ is left"

# The rollback restores the three keys exactly as they were, a hand-set value
# included — the identity's own rule (a hand-set value is the user's) holds
# for the way back too.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local user.email "handset@example.invalid"
fakebin=$(mktemp -d)
cat > "$fakebin/git" <<EOF2
#!/usr/bin/env bash
case " \$* " in *" --replace-all user.name "*) echo "git: forced failure" >&2; exit 1 ;; esac
exec "$real_git" "\$@"
EOF2
chmod +x "$fakebin/git"
RUN_STDOUT=$(cd "$FIXTURE_WORKSPACE" && PATH="$fakebin:$PATH" daisy init pelicanmoor 2>"$fakebin/err"); RUN_EXIT=$?
rm -rf "$fakebin"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "the forced failure exits non-zero"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email)" "handset@example.invalid" "and the hand-set email is exactly as it was"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Zebra Quillfeather" "as is the name"
git -C "$FIXTURE_WORKSPACE" config --local user.email "fixture@example.invalid"

# The rollback puts values back byte for byte, a newline or a tab included.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local user.email $'multi\nline@example.invalid'
git -C "$FIXTURE_WORKSPACE" config --local --add user.name $'\tTabbed Name'
fakebin=$(mktemp -d)
cat > "$fakebin/git" <<EOF2
#!/usr/bin/env bash
case " \$* " in *" --replace-all user.name "*) echo "git: forced failure" >&2; exit 1 ;; esac
exec "$real_git" "\$@"
EOF2
chmod +x "$fakebin/git"
RUN_STDOUT=$(cd "$FIXTURE_WORKSPACE" && PATH="$fakebin:$PATH" daisy init pelicanmoor 2>"$fakebin/err"); RUN_EXIT=$?
rm -rf "$fakebin"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "the forced failure exits non-zero"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local --get-all user.email | wc -l | tr -d ' ')" "2" "a value with a newline is one value again (two lines)"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local --get-all user.email)" $'multi\nline@example.invalid' "and byte for byte"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local --get-all user.name | tail -1)" $'\tTabbed Name' "a leading tab survives too"
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.name
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.email
fixture_run_cli init testhome

# Values are values, never options: a name that looks like an echo flag is
# applied, recognised and taken back like any other; a .daisy/identity that
# looks like one is refused like any other non-name.
printf 'name=-n\nemail=dash@example.invalid\n' > "$FIXTURE_HOME/identities/dashy"
dash="$FIXTURE_LOCAL/dash-ws"
mkdir -p "$dash"
fixture_run_cli init dashy "$dash"
assert_eq "$RUN_EXIT" "0" "init with a name of '-n' exits 0"
assert_eq "$(git -C "$dash" config --local user.name)" "-n" "and applies it"
FIXTURE_CWD="$dash"
fixture_run_cli clean -f
FIXTURE_CWD=""
assert_eq "$(git -C "$dash" config --local user.name || true)" "" "and clean takes it back"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name, user.email)" "reporting both keys"
rm "$FIXTURE_HOME/identities/dashy"
fixture_run_cli init testhome
printf -- '-n' > "$FIXTURE_WORKSPACE/.daisy/identity"
fixture_run_cli status
assert_eq "$RUN_EXIT" "1" "status refuses a .daisy/identity of '-n'"
assert_contains "$RUN_STDERR" "not an identity name" "as not a name"
rm "$FIXTURE_WORKSPACE/.daisy/identity"
