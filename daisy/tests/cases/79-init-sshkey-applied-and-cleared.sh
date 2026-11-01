# `sshkey=` in an identity file pins the workspace's pushes to that key:
# core.sshCommand becomes `ssh -i <key> -o IdentitiesOnly=yes`. It is one
# of the three keys the identity owns, so `daisy clean` unsets it with the
# other two when it still matches, and leaves a hand-changed value alone.
# The identity file is read before anything is written: an identity without
# both name and email is refused, and a key path that does not exist is
# applied with a warning (the key may be created later; pushes say so).

key="$FIXTURE_HOME_PARENT/id_pelicanmoor"
touch "$key"
mkdir -p "$FIXTURE_HOME/identities"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$FIXTURE_HOME/identities/pelicanmoor"

fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "init with an sshkey identity exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -i $key -o IdentitiesOnly=yes" "core.sshCommand is applied"

fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "clean exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "clean unsets core.sshCommand"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name || true)" "" "clean unsets user.name"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email || true)" "" "clean unsets user.email"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name, user.email, core.sshCommand)" "clean names all three"

# A hand-changed sshCommand is not the identity's: left alone.
fixture_run_cli init pelicanmoor
git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand "ssh -v"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -v" "a hand-set core.sshCommand survives clean"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name, user.email)" "and the message omits it"
git -C "$FIXTURE_WORKSPACE" config --local --unset core.sshCommand

# A key that does not exist: applied (the file is the identity's statement
# of its key) but warned about, so a failing push is no mystery.
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$FIXTURE_HOME_PARENT/no-such-key" > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "init with a missing key still exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -i $FIXTURE_HOME_PARENT/no-such-key -o IdentitiesOnly=yes" "core.sshCommand is applied"
assert_contains "$RUN_STDERR" "no-such-key" "the warning, on stderr, names the key"
assert_contains "$RUN_STDERR" "not found" "and says it is missing"

# A `~` key path is kept verbatim for the shell git runs sshCommand through.
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=~/.ssh/id_pelicanmoor\n' > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -i ~/.ssh/id_pelicanmoor -o IdentitiesOnly=yes" "a ~ path is not expanded into the config"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "and is still recognised and unset by clean"

# An identity file missing name or email is refused before any write.
printf 'name=Pelican Moorhen\n' > "$FIXTURE_HOME/identities/pelicanmoor"
ws="$FIXTURE_LOCAL/incomplete-ws"
mkdir -p "$ws"
fixture_run_cli init pelicanmoor "$ws"
assert_eq "$RUN_EXIT" "1" "init refuses an incomplete identity"
assert_contains "$RUN_STDERR" "identities/pelicanmoor" "the error names the file"
assert_contains "$RUN_STDERR" "email" "and the missing key"
[ -e "$ws/.daisy" ] && wrote=yes || wrote=no
assert_eq "$wrote" "no" "and writes nothing"

# The same identity re-applied after its sshkey= line was removed: init
# unsets what it applied before (recorded in .daisy/gitconfig), so the old
# core.sshCommand goes even though the identity file no longer names a key.
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/gitconfig")" "$(printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s' "$key")" \
    "init records the identity it applied"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "re-init without the key exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "the previously applied core.sshCommand is cleared"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/gitconfig")" $'name=Pelican Moorhen\nemail=pelican@example.invalid' "the record follows"

# The identity file deleted after init: clean still unsets what init applied.
rm "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "clean exits 0 with the identity file gone"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email || true)" "" "and still unsets the applied identity"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name, user.email)" "and says so"

# The key goes into core.sshCommand unquoted, so it must be one plain shell
# word: whitespace or shell metacharacters are refused.
for bad in "$FIXTURE_HOME_PARENT/a key" '~/.ssh/id;echo' '$HOME/.ssh/id'; do
    printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$bad" > "$FIXTURE_HOME/identities/pelicanmoor"
    fixture_run_cli init pelicanmoor "$ws"
    assert_eq "$RUN_EXIT" "1" "init refuses sshkey '$bad'"
    assert_contains "$RUN_STDERR" "sshkey" "and names the key"
    [ -e "$ws/.daisy" ] && wrote=yes || wrote=no
    assert_eq "$wrote" "no" "and writes nothing"
done

# A record that exists but cannot be read is no record: clean falls back to
# the home's identity file rather than leaving the identity behind.
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
: > "$FIXTURE_WORKSPACE/.daisy/gitconfig"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "clean unsets the key with an empty record"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.name, user.email, core.sshCommand)" "against the home's file"

# Multi-valued keys: git refuses to overwrite them with a plain set, so apply
# replaces all values, and unset treats the key as the identity's only when
# every value is.
rm -rf "$FIXTURE_WORKSPACE/.daisy"
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.name || true
git -C "$FIXTURE_WORKSPACE" config --local --add user.name "One"
git -C "$FIXTURE_WORKSPACE" config --local --add user.name "Two"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "init succeeds over a multi-valued user.name"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local --get-all user.name)" "Pelican Moorhen" "and replaces every value"
git -C "$FIXTURE_WORKSPACE" config --local --add user.name "Hand Added"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local --get-all user.name | tr '\n' ',')" "Pelican Moorhen,Hand Added," \
    "a key holding a value that is not the identity's is left whole"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.email)" "and only the matching key is reported"

# A clean whose unset fails (a stale lock from a crashed git) must stop
# before removing the record it would need to try again, and say so.
fixture_run_cli init pelicanmoor
touch "$FIXTURE_WORKSPACE/.git/config.lock"
fixture_run_cli clean -f
rm "$FIXTURE_WORKSPACE/.git/config.lock"
assert_eq "$RUN_EXIT" "1" "clean fails when the identity cannot be unset"
assert_contains "$RUN_STDERR" "could not unset" "and says why"
assert_not_contains "$RUN_STDOUT" "left as is" "and does not call it 'nothing matched'"
[ -f "$FIXTURE_WORKSPACE/.daisy/gitconfig" ] && kept=yes || kept=no
assert_eq "$kept" "yes" "the record is kept for the retry"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Pelican Moorhen" "and the identity is still in place"
fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "the retry succeeds"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name || true)" "" "and unsets it"

# No readable record and an identity file that has since lost its sshkey:
# init unsets against the file as it is now and says so on stderr; the key
# of Daisy's own form that the file no longer claims is still Daisy's, so it
# goes too, and status has nothing to report.
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
rm "$FIXTURE_WORKSPACE/.daisy/gitconfig"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "re-init without a record exits 0"
assert_contains "$RUN_STDERR" "No readable .daisy/gitconfig record" "and warns that it had no record of the previous identity"
assert_contains "$RUN_STDOUT" "core.sshCommand" "and reports clearing the key of Daisy's form the file no longer claims"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "which is gone"
fixture_run_cli status
assert_eq "$RUN_STDERR" "" "and status has nothing to report"

# A core.sshCommand in Daisy's own form is Daisy's, whatever record survived:
# clean takes it even under a key-less identity, while a hand-set command of
# another form stays the user's.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand "ssh -i $FIXTURE_HOME_PARENT/orphan -o IdentitiesOnly=yes"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "clean unsets an unclaimed key of Daisy's form"
assert_contains "$RUN_STDOUT" "core.sshCommand" "and reports it"
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand "ssh -v"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -v" "while a command of another form is left"
git -C "$FIXTURE_WORKSPACE" config --local --unset core.sshCommand

# Daisy's form is judged per value: a user's value of another form between
# two of Daisy's keeps the whole key out of clean's hands.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local --add core.sshCommand "ssh -i /k/a -o IdentitiesOnly=yes"
git -C "$FIXTURE_WORKSPACE" config --local --add core.sshCommand "ssh -i /k/m -o ProxyCommand=corp-jump"
git -C "$FIXTURE_WORKSPACE" config --local --add core.sshCommand "ssh -i /k/z -o IdentitiesOnly=yes"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local --get-all core.sshCommand | wc -l | tr -d ' ')" "3" "a key holding a value of another form is left whole"
git -C "$FIXTURE_WORKSPACE" config --local --unset-all core.sshCommand

# Nothing on record and no bound identity file: status reports what the
# repository holds rather than asserting nothing is applied; clean still takes
# Daisy's key; a re-init switching away does too.
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
rm "$FIXTURE_WORKSPACE/.daisy/gitconfig" "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli status
assert_contains "$RUN_STDERR" "Pelican Moorhen" "status names the applied identity nothing claims"
assert_not_contains "$RUN_STDERR" "no git identity is applied" "and does not claim none is applied"
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "a re-init with nothing on record exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "and still takes the key of Daisy's form"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Fixture Author" "while applying the new identity"
git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand "ssh -i $key -o IdentitiesOnly=yes"
rm "$FIXTURE_WORKSPACE/.daisy/gitconfig"
mv "$FIXTURE_HOME/gitconfig" "$FIXTURE_HOME/gitconfig.away"
fixture_run_cli clean -f
mv "$FIXTURE_HOME/gitconfig.away" "$FIXTURE_HOME/gitconfig"
assert_eq "$RUN_EXIT" "0" "clean with nothing on record exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "and takes the key of Daisy's form"
assert_contains "$RUN_STDOUT" "core.sshCommand" "saying so"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Fixture Author" "while a name nothing claims is left for the user"
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.name
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.email

# Every value counts, the empty string included: a key holding the identity's
# value and an empty one is not wholly the identity's.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local --add user.name ""
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local --get-all user.name | wc -l | tr -d ' ')" "2" "a key with an added empty value is left whole"
assert_contains "$RUN_STDOUT" "Cleared local git identity (user.email)" "and only the matching key is reported"
git -C "$FIXTURE_WORKSPACE" config --local --unset-all user.name

# status judges Daisy's form per value too, and promises no removal clean
# would not make.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local --add core.sshCommand "ssh -i /k/a -o IdentitiesOnly=yes"
git -C "$FIXTURE_WORKSPACE" config --local --add core.sshCommand "ssh -i /k/m -o ProxyCommand=corp-jump"
fixture_run_cli status
assert_not_contains "$RUN_STDERR" "removes it" "status does not call a mixed key Daisy's"
git -C "$FIXTURE_WORKSPACE" config --local --unset-all core.sshCommand

# A key of Daisy's form is Daisy's even when the claim has moved on: with no
# record and an identity file whose sshkey changed since init, clean still
# takes the key init applied.
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init pelicanmoor
rm "$FIXTURE_WORKSPACE/.daisy/gitconfig"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$FIXTURE_HOME_PARENT/key-b" > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand || true)" "" "clean takes the applied key though the file now names another"
assert_contains "$RUN_STDOUT" "core.sshCommand" "and reports it"

# A first init over a repository already carrying a key of Daisy's form (an
# init elsewhere, or one that left no binding) takes it with the rest.
first="$FIXTURE_LOCAL/first-ws"
git init -q "$first"
git -C "$first" config --local core.sshCommand "ssh -i /k/orphan -o IdentitiesOnly=yes"
fixture_run_cli init testhome "$first"
assert_eq "$RUN_EXIT" "0" "a first init over an orphan key exits 0"
assert_eq "$(git -C "$first" config --local core.sshCommand || true)" "" "and the orphan key of Daisy's form is gone"

# Daisy's form is exactly what init writes — one plain key path between
# `ssh -i` and `-o IdentitiesOnly=yes`; a user's command that merely ends the
# same way, or carries more options, is the user's.
fixture_run_cli init testhome
git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand "ssh -i ~/.ssh/work -o ProxyJump=bastion.example -o IdentitiesOnly=yes"
fixture_run_cli clean -f
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand)" "ssh -i ~/.ssh/work -o ProxyJump=bastion.example -o IdentitiesOnly=yes" \
    "a key-pinning command with other options is left"
git -C "$FIXTURE_WORKSPACE" config --local --unset core.sshCommand

# An init killed between apply and swap leaves its staging directory, whose
# gitconfig is the only record of what it applied. Both reclaims — the next
# init's and clean's — unset against it before removing it.
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$FIXTURE_HOME/identities/pelicanmoor"
fixture_run_cli init testhome
stage="$FIXTURE_WORKSPACE/.daisy.stage.k1lled"
mkdir "$stage"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=%s\n' "$key" > "$stage/gitconfig"
git -C "$FIXTURE_WORKSPACE" config --local user.name "Pelican Moorhen"
git -C "$FIXTURE_WORKSPACE" config --local user.email "pelican@example.invalid"
git -C "$FIXTURE_WORKSPACE" config --local core.sshCommand "ssh -i $key -o IdentitiesOnly=yes"
fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "clean after a killed init exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name || true)" "" "and takes the identity the killed init applied"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email || true)" "" "email included"
for k in user.name user.email core.sshCommand; do
    assert_contains "$RUN_STDOUT" "$k" "reporting $k"
done

fixture_run_cli init testhome
mkdir "$stage"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$stage/gitconfig"
git -C "$FIXTURE_WORKSPACE" config --local user.name "Pelican Moorhen"
git -C "$FIXTURE_WORKSPACE" config --local user.email "pelican@example.invalid"
printf 'name=\n' > "$FIXTURE_HOME/gitconfig"  # the home has no complete identity: nothing will be applied over it
fixture_run_cli init testhome
printf 'name=Fixture Author\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"
assert_eq "$RUN_EXIT" "0" "init after a killed init exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name || true)" "" "and takes the identity the killed init applied before removing its stage"
[ -e "$stage" ] && left=yes || left=no
assert_eq "$left" "no" "which is gone"
fixture_run_cli init testhome

# A stage leftover is consulted in the final step, under the rollback's
# cover: when its unset fails the init stops, keeps the stage for the retry,
# and leaves the identity as it was.
fixture_run_cli init testhome
mkdir "$stage"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$stage/gitconfig"
git -C "$FIXTURE_WORKSPACE" config --local user.name "Pelican Moorhen"
git -C "$FIXTURE_WORKSPACE" config --local user.email "pelican@example.invalid"
touch "$FIXTURE_WORKSPACE/.git/config.lock"
fixture_run_cli init testhome
rm "$FIXTURE_WORKSPACE/.git/config.lock"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "an init whose reclaim cannot unset exits non-zero"
assert_contains "$RUN_STDERR" "could not unset" "and says why"
[ -d "$stage" ] && kept=yes || kept=no
assert_eq "$kept" "yes" "the stage is kept for the retry"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Pelican Moorhen" "and the identity is as it was"
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "the retry succeeds"
[ -e "$stage" ] && kept=yes || kept=no
assert_eq "$kept" "no" "and the stage is gone"

# A stage whose gitconfig copies the live record (an init killed between
# staging and applying) must not strip the live identity when the next init
# cannot finish: the final step's rollback puts it back.
cp -a "$FIXTURE_WORKSPACE/.daisy" "$stage"
echo '{ not json' > "$FIXTURE_WORKSPACE/.claude/settings.local.json"
fixture_run_cli init testhome
rm "$FIXTURE_WORKSPACE/.claude/settings.local.json"
[ "$RUN_EXIT" -ne 0 ] && failed=yes || failed=no
assert_eq "$failed" "yes" "the init that cannot finish exits non-zero"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Fixture Author" "and the live identity is still applied"
rm -rf "$stage"

# A leftover that cannot be removed after a completed init is reported, and
# the init still reports success: the binding is in place.
fixture_run_cli init testhome
mkdir -p "$FIXTURE_WORKSPACE/.daisy/locked"
touch "$FIXTURE_WORKSPACE/.daisy/locked/f"
chmod 555 "$FIXTURE_WORKSPACE/.daisy/locked"
fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "an init whose aside cannot be removed still exits 0"
assert_contains "$RUN_STDERR" "could not be removed" "and says what is left"
assert_contains "$RUN_STDOUT" "Bound .daisy/" "having bound the workspace"
chmod 755 "$FIXTURE_WORKSPACE"/.daisy.previous.*/locked 2>/dev/null; chmod 755 "$FIXTURE_WORKSPACE/.daisy/locked" 2>/dev/null
rm -rf "$FIXTURE_WORKSPACE"/.daisy.previous.* "$FIXTURE_WORKSPACE/.daisy/locked"
