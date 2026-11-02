# `daisy commit --root` commits the root repository minus every registered
# home that lies inside it — the registry defines "home", not a directory
# literally named home/. A home may still be placed under the root's path,
# so the exclusion is derived from where the registry says the homes are,
# and the commit itself is scoped: a bare `git commit` after a scoped `git
# add` would sweep a pre-staged home change into a root commit under a root
# message.

# A registered home inside the root repository, not under home/.
inner="$FIXTURE_ROOT/data/zebranest"
mkdir -p "$(dirname "$inner")"
cp -r "$REAL_DAISY_ROOT/templates/home" "$inner"
git -C "$FIXTURE_ROOT" add -A
git -C "$FIXTURE_ROOT" commit -q -m "root: a home inside the root"
fixture_register_home zebranest "$inner"

# A top-level home/ that nothing registers is root content like any other:
# the pre-split exclusion of it is gone.
mkdir -p "$FIXTURE_ROOT/home/stray"
echo "v1" > "$FIXTURE_ROOT/home/stray/note.txt"
git -C "$FIXTURE_ROOT" add -A
git -C "$FIXTURE_ROOT" commit -q -m "root: an unregistered directory under home/"

# A root change, a home change that is already staged, and a change under
# the unregistered home/ directory.
echo "code" > "$FIXTURE_ROOT/code.txt"
echo "- [ ] a task" >> "$inner/tasks/todo.txt"
git -C "$FIXTURE_ROOT" add "data/zebranest/tasks/todo.txt"
echo "v2" > "$FIXTURE_ROOT/home/stray/note.txt"

before=$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)
fixture_run_cli commit --root "root change"
assert_eq "$RUN_EXIT" "0" "commit --root succeeds with a registered home inside the root"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$((before + 1))" "exactly one commit"
committed=$(git -C "$FIXTURE_ROOT" show --name-only --format= HEAD)
assert_contains "$committed" "code.txt" "the commit holds the root change"
assert_contains "$committed" "home/stray/note.txt" "and the unregistered home/ directory, which is the root's"
assert_not_contains "$committed" "zebranest" "and not the pre-staged home change"
assert_eq "$(git -C "$FIXTURE_ROOT" diff --cached --name-only)" "data/zebranest/tasks/todo.txt" \
    "the home change stays staged for its own home commit"

# With only home changes pending, --root has nothing to commit.
fixture_run_cli commit --root "nothing outside the home"
assert_eq "$RUN_EXIT" "0" "a no-op is not an error"
assert_contains "$RUN_STDERR" "No changes to commit in root (excluding data/zebranest)" "and says so, naming the exclusion"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$((before + 1))" "no commit was made"

# The removed forms are refused, never taken as a message.
fixture_run_cli commit --not-home "x"
assert_eq "$RUN_EXIT" "2" "commit --not-home is a usage error"
fixture_run_cli commit --home
assert_eq "$RUN_EXIT" "2" "commit --home is not a message"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$((before + 1))" "and neither committed"

git -C "$FIXTURE_ROOT" reset -q
fixture_unregister_home zebranest
rm -rf "$inner"
git -C "$FIXTURE_ROOT" add -A
git -C "$FIXTURE_ROOT" commit -q -m "root: the home has moved out"

# Once no registered home lies inside the root, the commit is the whole
# repository, unscoped: a partial commit is refused during a merge, and a
# repository synced between machines sees merges.
echo "more" > "$FIXTURE_ROOT/more.txt"
git -C "$FIXTURE_ROOT" add more.txt
echo "later" > "$FIXTURE_ROOT/code.txt"
before=$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)
fixture_run_cli commit --root "whole root"
assert_eq "$RUN_EXIT" "0" "commit --root succeeds with no home inside the root"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$((before + 1))" "one commit"
committed=$(git -C "$FIXTURE_ROOT" show --name-only --format= HEAD)
assert_contains "$committed" "more.txt" "a pre-staged root change is committed too"
assert_contains "$committed" "code.txt" "with the unstaged one"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "" "nothing is left behind"
