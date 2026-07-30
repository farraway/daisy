# Check 6c scope: the publishable surface is what `migrate.sh` actually
# mirrors — git-tracked files plus untracked-but-not-ignored ones. A
# git-ignored file (per-machine workspace state, .env.sh secrets) never
# reaches the mirror, so naming an identifying term in one is not a leak and
# must not fail the healthcheck. Case 35 covers the positive direction; this
# case pins the boundary so the ignore-awareness can't degrade into a blanket
# suppression.

mkdir -p "$FIXTURE_ROOT/home/zebrahome"
printf 'ignored-leak.md\nper-machine/\n' > "$FIXTURE_ROOT/.gitignore"

echo "this doc mentions zebrahome by name" > "$FIXTURE_ROOT/ignored-leak.md"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck ignores a git-ignored file naming a real home"
assert_not_contains "$RUN_STDERR" "ignored-leak.md" \
    "healthcheck does not report a git-ignored file"

# An ignored *directory* is out of scope too — ls-files must not descend it.
mkdir -p "$FIXTURE_ROOT/per-machine"
echo "zebrahome lives here" > "$FIXTURE_ROOT/per-machine/state.txt"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck ignores files under a git-ignored directory"
assert_not_contains "$RUN_STDERR" "per-machine/state.txt" \
    "healthcheck does not report a file under a git-ignored directory"

# The boundary must still catch a real leak sitting beside the ignored ones,
# both tracked and merely untracked.
echo "zebrahome appears here too" > "$FIXTURE_ROOT/public-leak.md"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "1" "healthcheck still fails on an untracked, non-ignored leak"
assert_contains "$RUN_STDERR" "public-leak.md" \
    "healthcheck names the publishable leak"
assert_not_contains "$RUN_STDERR" "ignored-leak.md" \
    "healthcheck still excludes the git-ignored file"

git -C "$FIXTURE_ROOT" add -A >/dev/null 2>&1
git -C "$FIXTURE_ROOT" commit -q -m "track the leak" >/dev/null 2>&1

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "1" "healthcheck still fails once the leak is tracked"
assert_contains "$RUN_STDERR" "public-leak.md" \
    "healthcheck names the tracked leak"

rm -f "$FIXTURE_ROOT/public-leak.md"
git -C "$FIXTURE_ROOT" add -A >/dev/null 2>&1
git -C "$FIXTURE_ROOT" commit -q -m "remove the leak" >/dev/null 2>&1

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes again once the publishable leak is removed"
