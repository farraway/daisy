# Check 3: every Daisy mutation commits in the home's repository and root
# code commits in the root's, so the health check requires both. A home with
# no repository would make every `daisy done` and `daisy log` fail at the
# commit, after the files were already written; the check says so first and
# names the fix. A home with no repository of its own inside some other
# repository (a dotfiles-managed $HOME) is the same failure in a worse form:
# every commit would land, silently, in that other repository — the root's
# included (case 71). Another registered home placed inside the root is
# not this workspace's to check: a root commit and the hygiene scan leave
# it out, and the pre-split warning about it (Check 3b) is gone.

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes with the home as its own repository"

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

# Another registered home placed under the root's directory as its own
# repository (untracked by the root): healthy, no warning — a root commit
# and the scan leave it out. (Tracked by the root, its files would be the
# root's, published and scanned: TestTrackedUnderAHomeIsScanned.)
inner="$FIXTURE_ROOT/data/zebranest"
mkdir -p "$(dirname "$inner")"
cp -r "$REAL_DAISY_ROOT/templates/home" "$inner"
git -C "$inner" init -q
fixture_register_home zebranest "$inner"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "a registered home of its own under the root is not a failure"
assert_not_contains "$RUN_STDERR" "zebranest" "and not warned about"

fixture_unregister_home zebranest
