# `daisy home` manages the registry at $DAISY_CONFIG/homes — the one place a
# home name becomes a path. Resolution never derives a path from
# $DAISY_ROOT, so a home can live anywhere, in its own repository, and a
# workspace bound to a name nobody registered fails loudly and names the fix.

fixture_run_cli home list
assert_eq "$RUN_EXIT" "0" "home list exits 0"
assert_contains "$RUN_STDOUT" "testhome" "home list names the fixture home"
assert_contains "$RUN_STDOUT" "$FIXTURE_HOME" "home list shows where it lives"

# add: a second home, a template copy outside the root and the first home
second="$FIXTURE_LOCAL/homes/second"
mkdir -p "$(dirname "$second")"
cp -r "$REAL_DAISY_ROOT/daisy/templates/home" "$second"
second=$(cd "$second" && pwd -P)  # the registry stores resolved paths
git init -q "$second"
git -C "$second" -c user.name="Fixture Author" -c user.email="fixture@example.invalid" commit -q --allow-empty -m "second: initial"

fixture_run_cli home add second "$second"
assert_eq "$RUN_EXIT" "0" "home add registers an existing home directory"
fixture_run_cli home list
assert_contains "$RUN_STDOUT" "second" "home list shows the added home"

fixture_run_cli home add second "$second"
assert_eq "$RUN_EXIT" "1" "home add refuses a name already registered"
assert_contains "$RUN_STDERR" "already registered" "home add says why"

not_a_home=$(mktemp -d)
fixture_run_cli home add nothome "$not_a_home"
assert_eq "$RUN_EXIT" "1" "home add refuses a directory that is not a home"
assert_contains "$RUN_STDERR" "include.txt" "home add names what a home must contain"
rmdir "$not_a_home"

fixture_run_cli home add "bad=name" "$second"
assert_eq "$RUN_EXIT" "1" "home add refuses a name the registry format cannot hold"

fixture_run_cli home add missing "$FIXTURE_LOCAL/homes/nowhere"
assert_eq "$RUN_EXIT" "1" "home add refuses a path that does not exist"

touch "$FIXTURE_ROOT/include.txt"
fixture_run_cli home add rooty "$FIXTURE_ROOT"
assert_eq "$RUN_EXIT" "1" "home add refuses the root itself"
assert_contains "$RUN_STDERR" "cannot be a home" "and says why"
rm -f "$FIXTURE_ROOT/include.txt"

# A hand-edited entry at the root is refused wherever a home is resolved.
fixture_register_home rooty "$FIXTURE_ROOT"
echo "rooty" > "$FIXTURE_WORKSPACE/.daisy/home"
fixture_run_cli status
assert_eq "$RUN_EXIT" "1" "a workspace bound to a home at the root fails"
assert_contains "$RUN_STDERR" "cannot be a home" "and the error says why"
echo "testhome" > "$FIXTURE_WORKSPACE/.daisy/home"
fixture_unregister_home rooty

# Nor can a home hold the root: a home commit there would commit the root.
problem=$(bash -c "source '$DAISY_ROOT/daisy/scripts/common.sh'; daisy_home_path_problem '$(dirname "$FIXTURE_ROOT")'")
assert_contains "$problem" "contains the Daisy root" "a directory holding the root cannot be a home"
problem=$(bash -c "source '$DAISY_ROOT/daisy/scripts/common.sh'; daisy_home_path_problem 'relative/path'")
assert_contains "$problem" "not an absolute path" "a relative registry path cannot be a home"
problem=$(bash -c "source '$DAISY_ROOT/daisy/scripts/common.sh'; daisy_home_path_problem '$FIXTURE_HOME'")
assert_eq "$problem" "" "the fixture home has no problem"
# Spellings of the same directory: a POSIX-special leading `//`, and `/`.
problem=$(bash -c "source '$DAISY_ROOT/daisy/scripts/common.sh'; daisy_home_path_problem '/$FIXTURE_ROOT'")
assert_contains "$problem" "is the Daisy root" "a double-slash spelling of the root is still the root"
problem=$(bash -c "source '$DAISY_ROOT/daisy/scripts/common.sh'; daisy_home_path_problem '/'")
assert_contains "$problem" "contains the Daisy root" "the filesystem root contains every Daisy root"

# A workspace bound to the second home resolves into its registered path.
fixture_run_cli init second
assert_eq "$RUN_EXIT" "0" "daisy init binds the workspace to a registered home"
assert_eq "$(readlink "$FIXTURE_WORKSPACE/.daisy/tasks")" "$second/tasks" \
    "the .daisy symlinks point into the registered path"
fixture_run_cli status
assert_eq "$RUN_EXIT" "0" "status resolves the second home"
assert_contains "$RUN_STDOUT" "home: second" "status names the second home"

# remove: unregisters the name and leaves the files alone
fixture_run_cli home remove second
assert_eq "$RUN_EXIT" "0" "home remove exits 0"
[ -f "$second/include.txt" ] && files_kept=yes || files_kept=no
assert_eq "$files_kept" "yes" "home remove leaves the home's files in place"
fixture_run_cli home list
assert_not_contains "$RUN_STDOUT" "second" "home list no longer shows the removed home"

fixture_run_cli status
assert_eq "$RUN_EXIT" "1" "a workspace bound to an unregistered home fails"
assert_contains "$RUN_STDERR" "not registered" "the error says the name is unregistered"
assert_contains "$RUN_STDERR" "daisy home add second" "the error names the command that fixes it"

fixture_run_cli home remove nosuch
assert_eq "$RUN_EXIT" "1" "home remove refuses an unregistered name"
