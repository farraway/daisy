# `daisy home new` replaces create-home.sh: a template copy at a chosen
# location, its own git repository with a first commit under the identity
# prompted into gitconfig, and the name registered — so `daisy init` works
# against it at once.

target="$(cd "$FIXTURE_LOCAL" && pwd -P)/homes/fresh"  # the registry stores resolved paths
fixture_run_cli_with_input $'Fixture Person\nperson@example.invalid\n' home new fresh "$target"
assert_eq "$RUN_EXIT" "0" "home new exits 0"
assert_eq "$(cat "$target/gitconfig")" $'name=Fixture Person\nemail=person@example.invalid' \
    "the prompted identity is saved to gitconfig"
[ -f "$target/include.txt" ] && copied=yes || copied=no
assert_eq "$copied" "yes" "the template is copied to the chosen location"
assert_eq "$(git -C "$target" rev-list --count HEAD)" "1" \
    "the new home is its own repository with one commit"
assert_eq "$(git -C "$target" symbolic-ref HEAD)" "refs/heads/main" \
    "its repository starts on main (no init.defaultBranch is set)"
assert_eq "$(git -C "$target" config user.email)" "person@example.invalid" \
    "the repository commits as the home's identity"
assert_eq "$(git -C "$target" check-ignore .healthcheck.ttl AGENTS.md | wc -l | tr -d ' ')" "2" \
    "the home ignores its healthcheck stamp and built AGENTS.md"

fixture_run_cli home list
assert_contains "$RUN_STDOUT" "fresh" "the new home is registered"
assert_contains "$RUN_STDOUT" "$target" "at the chosen location"

fixture_run_cli_with_input $'\n\n' home new again "$target"
assert_eq "$RUN_EXIT" "1" "home new refuses a location that already exists"
assert_contains "$RUN_STDERR" "already exists" "and says so"

fixture_run_cli_with_input $'\n\n' home new fresh "$FIXTURE_LOCAL/homes/other"
assert_eq "$RUN_EXIT" "1" "home new refuses a name already registered"
[ -e "$FIXTURE_LOCAL/homes/other" ] && written=yes || written=no
assert_eq "$written" "no" "and writes nothing"

# With no path, the location is prompted with a default of $DAISY_LOCAL/homes/<name>.
fixture_run_cli_with_input $'\nA Person\na@example.invalid\n' home new prompted
assert_eq "$RUN_EXIT" "0" "home new with no path exits 0"
[ -d "$FIXTURE_LOCAL/homes/prompted" ] && defaulted=yes || defaulted=no
assert_eq "$defaulted" "yes" "an empty answer takes the default location"

fixture_run_cli init fresh
assert_eq "$RUN_EXIT" "0" "daisy init works against the new home"
assert_eq "$(readlink "$FIXTURE_WORKSPACE/.daisy/today.md")" "$target/journal/today.md" \
    "the workspace links into the new home"
