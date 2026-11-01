# `daisy home new` must leave nothing behind when it cannot finish: a
# half-made directory would block the retry ("already exists") and could be
# registered by hand as a home that is not one. A failing pre-commit hook,
# installed through a private global git config, makes the first commit
# fail after the template has been copied.

hooks=$(mktemp -d)
printf '#!/bin/sh\nexit 1\n' > "$hooks/pre-commit"
chmod +x "$hooks/pre-commit"
gitcfg=$(mktemp)
printf '[core]\n\thooksPath = %s\n' "$hooks" > "$gitcfg"

target="$FIXTURE_LOCAL/homes/doomed"
# run.sh sources every case into one shell: restore the developer's value.
saved_git_config_global="${GIT_CONFIG_GLOBAL-}"
had_git_config_global="${GIT_CONFIG_GLOBAL+set}"
export GIT_CONFIG_GLOBAL="$gitcfg"
fixture_run_with_input $'Fixture Person\nperson@example.invalid\n' home new doomed "$target"
if [ -n "$had_git_config_global" ]; then
    export GIT_CONFIG_GLOBAL="$saved_git_config_global"
else
    unset GIT_CONFIG_GLOBAL
fi

assert_eq "$RUN_EXIT" "1" "home new fails when the first commit fails"
assert_contains "$RUN_STDERR" "was removed and nothing is registered" "and says what it did about it"
[ -e "$target" ] && left=yes || left=no
assert_eq "$left" "no" "the copied template is removed"
fixture_run_cli home list
assert_not_contains "$RUN_STDOUT" "doomed" "nothing is registered"

# A directory that is not a repository is refused by `home add` for the same
# reason: every mutation commits in the home's repository.
plain="$FIXTURE_LOCAL/homes/plain"
mkdir -p "$(dirname "$plain")"
cp -r "$REAL_DAISY_ROOT/templates/home" "$plain"
fixture_run_cli home add plain "$plain"
assert_eq "$RUN_EXIT" "1" "home add refuses a home that is not in a git repository"
assert_contains "$RUN_STDERR" "not in its own git repository" "and says so"

rm -rf "$hooks" "$gitcfg"
