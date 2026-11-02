# install takes the root of the binary that runs; a DAISY_ROOT naming another
# root (a stale export) is refused before any write, in both forms, with both
# paths on stderr and the fix: plain install, which writes the rc line the
# export came from (D47: a stale export had sent an install, and ~/bin/daisy,
# to the old root). Here DAISY_ROOT is the fixture root and the binary is the
# real root's.

fixture_setup_home_override
mkdir -p "$FIXTURE_HOME_OVERRIDE/bin" "$FIXTURE_HOME_OVERRIDE/.claude/skills/old-skill"
ln -s /nowhere/bin/daisy "$FIXTURE_HOME_OVERRIDE/bin/daisy"
echo "export DAISY_ROOT=\"/nowhere\"" > "$FIXTURE_HOME_OVERRIDE/.zshenv"
snapshot() { (cd "$FIXTURE_HOME_OVERRIDE" && find . -printf '%p %l\n' | sort && cat .zshenv); }
home_before=$(snapshot)
root_before=$(git -C "$FIXTURE_ROOT" status --porcelain)

for form in "install" "install --update"; do
    # shellcheck disable=SC2086
    SHELL=/bin/zsh fixture_run_cli $form
    assert_eq "$RUN_EXIT" "1" "$form: exits 1"
    assert_contains "$RUN_STDERR" "DAISY_ROOT is $FIXTURE_ROOT, but this binary" "$form: names DAISY_ROOT"
    assert_contains "$RUN_STDERR" "DAISY_ROOT=$REAL_DAISY_ROOT $REAL_DAISY_ROOT/bin/daisy install  (" "$form: names the binary's root and the fix"
    assert_eq "$RUN_STDOUT" "" "$form: prints nothing on stdout"
done

assert_eq "$(snapshot)" "$home_before" "HOME (~/bin, skills, rc file) is unchanged"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "$root_before" "the fixture root is unchanged"
