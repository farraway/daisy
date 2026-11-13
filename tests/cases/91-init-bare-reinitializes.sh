# A bare `daisy init`, run anywhere in a bound workspace, re-initializes it
# with the home `.daisy/home` names and the identity `.daisy/identity` names
# (else the home's default): `.daisy/identity`, the record of the applied
# identity (`.daisy/gitconfig`, `.daisy/gitconfig.before`) and the
# repository's keys stay as they were, and `.daisy/AGENTS.md` is rebuilt
# from the root as it is now. In a folder with no binding the identity is
# the missing argument: exit 2, the usage, and `daisy init <identity>`.

printf 'name=Zebra Quillfeather\nemail=fixture@example.invalid\n' > "$FIXTURE_HOME/gitconfig"
mkdir -p "$FIXTURE_HOME/identities"
printf 'name=Pelican Moorhen\nemail=pelican@example.invalid\n' > "$FIXTURE_HOME/identities/pelicanmoor"
git -C "$FIXTURE_WORKSPACE" init -q
git -C "$FIXTURE_WORKSPACE" config user.email hand@example.invalid

fixture_run_cli init pelicanmoor
assert_eq "$RUN_EXIT" "0" "init with a named identity exits 0"
identity_was=$(cat "$FIXTURE_WORKSPACE/.daisy/identity")
record_was=$(cat "$FIXTURE_WORKSPACE/.daisy/gitconfig")
before_was=$(cat "$FIXTURE_WORKSPACE/.daisy/gitconfig.before")
assert_contains "$before_was" "hand@example.invalid" "init records the key it found set by hand"

printf '\nA line the root gained after the workspace was bound.\n' >> "$FIXTURE_ROOT/prompts/daisy.md"
assert_not_contains "$(cat "$FIXTURE_WORKSPACE/.daisy/AGENTS.md")" "the root gained after" "AGENTS.md predates the root's change"

mkdir -p "$FIXTURE_WORKSPACE/sub"
FIXTURE_CWD="$FIXTURE_WORKSPACE/sub" fixture_run_cli init
assert_eq "$RUN_EXIT" "0" "bare init in a subdirectory of the workspace exits 0"
assert_contains "$RUN_STDOUT" "Initializing Daisy in: $FIXTURE_WORKSPACE" "it re-initializes the workspace, not the subdirectory"
assert_contains "$RUN_STDOUT" "Already initialized with identity: pelicanmoor" "with the binding's identity"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/identity")" "$identity_was" ".daisy/identity is unchanged"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/gitconfig")" "$record_was" "the applied gitconfig record is unchanged"
assert_eq "$(cat "$FIXTURE_WORKSPACE/.daisy/gitconfig.before")" "$before_was" "what the keys held before is unchanged"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.name)" "Pelican Moorhen" "user.name is still the identity's"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email)" "pelican@example.invalid" "user.email is still the identity's"
assert_contains "$(cat "$FIXTURE_WORKSPACE/.daisy/AGENTS.md")" "the root gained after" ".daisy/AGENTS.md is rebuilt from the root as it is now"
[ -e "$FIXTURE_WORKSPACE/sub/.daisy" ] && bound=yes || bound=no
assert_eq "$bound" "no" "the subdirectory is not bound"

# clean still puts back the key set by hand: the re-init kept the record.
fixture_run_cli clean -f
assert_eq "$RUN_EXIT" "0" "clean exits 0"
assert_eq "$(git -C "$FIXTURE_WORKSPACE" config --local user.email)" "hand@example.invalid" "clean puts back the hand-set key"

unbound="$FIXTURE_LOCAL/unbound"
mkdir -p "$unbound"
FIXTURE_CWD="$unbound" fixture_run_cli init
assert_eq "$RUN_EXIT" "2" "bare init in an unbound folder is a usage error"
assert_contains "$RUN_STDERR" "missing argument" "the identity is the missing argument"
assert_contains "$RUN_STDERR" "daisy init <identity>" "and the hint names the form that binds"
assert_contains "$RUN_STDERR" "Usage: daisy init" "the usage follows"
assert_eq "$(ls -A "$unbound")" "" "and nothing is written"
