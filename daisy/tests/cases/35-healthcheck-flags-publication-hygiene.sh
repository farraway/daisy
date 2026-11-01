# Check 6c: daisy healthcheck must fail when a publishable file contains an
# identifying term — here, the name of a real
# registered home. Terms are derived at runtime, so the test registers its
# own distinctively-named home in the fixture rather than referencing any
# real name.

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes on a clean fixture"

fixture_register_home zebrahome
echo "this doc mentions zebrahome by name" > "$FIXTURE_ROOT/leak.md"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "1" "healthcheck fails when a publishable file names a real home"
assert_contains "$RUN_STDERR" "Publication hygiene: 'zebrahome'" \
    "healthcheck names the leaked term"
assert_contains "$RUN_STDERR" "leak.md" \
    "healthcheck names the offending file"

rm -f "$FIXTURE_ROOT/leak.md"
fixture_unregister_home zebrahome

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "healthcheck passes again once the leak is removed"
