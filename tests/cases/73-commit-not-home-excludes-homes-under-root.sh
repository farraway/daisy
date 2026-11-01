# `daisy commit --not-home` commits the root repository minus every
# registered home that lies inside it — the registry defines "home", not a
# directory literally named home/. Until the split, the real homes are
# subdirectories of the root repository, so the exclusion must be derived
# from where the registry says they are, and the commit itself must be
# scoped: a bare `git commit` after a scoped `git add` would sweep a
# pre-staged home change into a root commit under a root message.

# A registered home inside the root repository, not under home/.
inner="$FIXTURE_ROOT/data/zebranest"
mkdir -p "$(dirname "$inner")"
cp -r "$REAL_DAISY_ROOT/templates/home" "$inner"
git -C "$FIXTURE_ROOT" add -A
git -C "$FIXTURE_ROOT" commit -q -m "root: a home inside the root"
fixture_register_home zebranest "$inner"

# Before the split, the top-level home/ holds the homes, and may hold a
# directory nothing registers; it is excluded as a whole while it exists.
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
fixture_run_cli commit --not-home "root change"
assert_eq "$RUN_EXIT" "0" "commit --not-home succeeds with a registered home inside the root"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$((before + 1))" "exactly one commit"
committed=$(git -C "$FIXTURE_ROOT" show --name-only --format= HEAD)
assert_contains "$committed" "code.txt" "the commit holds the root change"
assert_not_contains "$committed" "zebranest" "and not the pre-staged home change"
assert_not_contains "$committed" "stray" "nor anything under the top-level home/"
assert_eq "$(git -C "$FIXTURE_ROOT" diff --cached --name-only)" "data/zebranest/tasks/todo.txt" \
    "the home change stays staged for its own home commit"

# With only home changes pending, --not-home has nothing to commit.
fixture_run_cli commit --not-home "nothing outside the home"
assert_eq "$RUN_EXIT" "0" "a no-op is not an error"
assert_contains "$RUN_STDERR" "No changes to commit" "and says so"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$((before + 1))" "no commit was made"

git -C "$FIXTURE_ROOT" reset -q
git -C "$FIXTURE_ROOT" checkout -q -- home/stray/note.txt
fixture_unregister_home zebranest
rm -rf "$FIXTURE_ROOT/home"
git -C "$FIXTURE_ROOT" add -A
git -C "$FIXTURE_ROOT" commit -q -m "root: the homes have moved out"

# Once no registered home lies inside the root (the layout after the split),
# the commit is the whole repository, unscoped: a partial commit is refused
# during a merge, and a repository synced between machines sees merges.
echo "more" > "$FIXTURE_ROOT/more.txt"
git -C "$FIXTURE_ROOT" add more.txt
echo "later" > "$FIXTURE_ROOT/code.txt"
before=$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)
fixture_run_cli commit --not-home "whole root"
assert_eq "$RUN_EXIT" "0" "commit --not-home succeeds with no home inside the root"
assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$((before + 1))" "one commit"
committed=$(git -C "$FIXTURE_ROOT" show --name-only --format= HEAD)
assert_contains "$committed" "more.txt" "a pre-staged root change is committed too"
assert_contains "$committed" "code.txt" "with the unstaged one"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "" "nothing is left behind"
