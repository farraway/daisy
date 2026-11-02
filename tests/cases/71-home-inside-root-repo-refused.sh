# A home commits in its own repository and nowhere else. A home that lies
# inside the root repository (the layout before the split) would put every
# journal entry, task and author into the published root's history, so it
# is refused: the health check fails, and so every mutation's pre-flight,
# before anything is written or committed. (The home's name must not be an
# English word: it becomes a publication-hygiene term.)

inner="$FIXTURE_ROOT/home/zebranest"
mkdir -p "$(dirname "$inner")"
cp -r "$REAL_DAISY_ROOT/templates/home" "$inner"
fixture_register_home zebranest "$inner"
echo "zebranest" > "$FIXTURE_WORKSPACE/.daisy/home"
seed_today_basic >/dev/null 2>&1 || true
cp "$FIXTURE_HOME/journal/today.md" "$inner/journal/today.md"
git -C "$FIXTURE_ROOT" add -A
git -C "$FIXTURE_ROOT" commit -q -m "root: a home inside the root"
before_today=$(cat "$inner/journal/today.md")

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "1" "healthcheck fails for a home inside the root repository"
assert_contains "$RUN_STDERR" "not in its own git repository" "and names the condition"

before=$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)
fixture_run_cli log "Logged from a zebranest home"
assert_eq "$RUN_EXIT" "1" "daisy log refuses with the home inside the root repository"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$before" "nothing is committed to the root"
assert_eq "$(cat "$inner/journal/today.md")" "$before_today" "and nothing is written"

fixture_run_cli commit "by hand"
assert_eq "$RUN_EXIT" "1" "daisy commit refuses it too"
assert_contains "$RUN_STDERR" "not in its own git repository" "naming the condition"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$before" "still nothing committed to the root"

fixture_unregister_home zebranest
