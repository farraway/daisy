# Check 3: every Daisy mutation commits in the home's repository and root
# code commits in the root's, so the health check requires both. A home with
# no repository would make every `daisy done` and `daisy log` fail at the
# commit, after the files were already written; the check says so first and
# names the fix. A home with no repository of its own inside some other
# repository (a dotfiles-managed $HOME) is the same failure in a worse form:
# every commit would land, silently, in that other repository. A registered home that lies inside the root repository is
# the pre-split layout: it works, and it is reported as a warning so the
# tree is not called clean once the homes should have moved out.

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes with the home as its own repository"
assert_not_contains "$RUN_STDERR" "lies inside" "no warning when no home is inside the root"

# The home loses its repository.
mv "$FIXTURE_HOME/.git" "$FIXTURE_HOME_PARENT/git-aside"
fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "1" "healthcheck fails when the home is not in a git repository"
assert_contains "$RUN_STDERR" "not in its own git repository" "the error names the condition"
assert_contains "$RUN_STDERR" "git -C $(printf '%q' "$FIXTURE_HOME") init" "and the command that fixes it"
mv "$FIXTURE_HOME_PARENT/git-aside" "$FIXTURE_HOME/.git"

# The home's directory lies inside a repository that is neither its own nor
# the root's.
git init -q "$FIXTURE_HOME_PARENT"
mv "$FIXTURE_HOME/.git" "$FIXTURE_HOME_PARENT/git-aside"
fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "1" "healthcheck fails when the home is inside a foreign repository"
assert_contains "$RUN_STDERR" "not in its own git repository" "the error names the condition"
mv "$FIXTURE_HOME_PARENT/git-aside" "$FIXTURE_HOME/.git"
rm -rf "$FIXTURE_HOME_PARENT/.git"
fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes again with the home's own repository restored"

# A registered home inside the root repository: healthy, with a warning.
inner="$FIXTURE_ROOT/data/zebranest"
mkdir -p "$(dirname "$inner")"
cp -r "$REAL_DAISY_ROOT/templates/home" "$inner"
git -C "$FIXTURE_ROOT" add -A
git -C "$FIXTURE_ROOT" commit -q -m "root: a home inside the root"
fixture_register_home zebranest "$inner"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "a registered home inside the root is not a failure"
assert_contains "$RUN_STDERR" "Home 'zebranest' lies inside the root repository" \
    "but it is warned about by name"

fixture_unregister_home zebranest
