# Check 6c: once homes are their own repositories, the author each home
# commits as is an identifying term like the root's own, and must not appear
# in a publishable file. Terms are derived at runtime from the registered
# homes' git configuration, so the test gives its home a distinctive author.

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes on a clean fixture"

git -C "$FIXTURE_HOME" config user.name "Zebra Quillfeather"
echo "written by Zebra Quillfeather" > "$FIXTURE_ROOT/leak.md"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "1" "healthcheck fails when a publishable file names a home's author"
assert_contains "$RUN_STDERR" "Publication hygiene: 'Zebra Quillfeather'" \
    "healthcheck names the leaked term"
assert_contains "$RUN_STDERR" "leak.md" "and the offending file"

rm -f "$FIXTURE_ROOT/leak.md"
fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes again once the leak is removed"
