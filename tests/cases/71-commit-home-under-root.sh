# Until the split, a real home is a subdirectory of the root repository. A
# home commit there must take only the home's paths: a bare `git commit`
# after a scoped `git add` would sweep anything already staged elsewhere in
# that repository into a commit with a Daisy log message. (The home's name
# must not be an English word: it becomes a publication-hygiene term.)

inner="$FIXTURE_ROOT/home/zebranest"
mkdir -p "$(dirname "$inner")"
cp -r "$REAL_DAISY_ROOT/templates/home" "$inner"
echo "code" > "$FIXTURE_ROOT/code.txt"
fixture_register_home zebranest "$inner"
echo "zebranest" > "$FIXTURE_WORKSPACE/.daisy/home"
seed_today_basic >/dev/null 2>&1 || true
cp "$FIXTURE_HOME/journal/today.md" "$inner/journal/today.md"
git -C "$FIXTURE_ROOT" add -A
git -C "$FIXTURE_ROOT" commit -q -m "root: a home inside the root"

# Something unrelated is already staged in the root repository.
echo "changed" > "$FIXTURE_ROOT/code.txt"
git -C "$FIXTURE_ROOT" add code.txt

before=$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)
fixture_run log "Logged from a zebranest home"
assert_eq "$RUN_EXIT" "0" "daisy log succeeds with the home inside the root repository"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$((before + 1))" "exactly one commit"
committed=$(git -C "$FIXTURE_ROOT" show --name-only --format= HEAD)
assert_contains "$committed" "home/zebranest/journal/today.md" "the commit holds the home's change"
assert_not_contains "$committed" "code.txt" "and not the pre-staged root change"
assert_eq "$(git -C "$FIXTURE_ROOT" diff --cached --name-only)" "code.txt" "the root change stays staged for its own commit"

fixture_unregister_home zebranest
