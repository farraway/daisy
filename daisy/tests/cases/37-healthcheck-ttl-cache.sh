# The healthcheck cache. The original mechanism was inert: healthcheck.sh
# exported DAISY_HEALTHCHECK_PASSED=1 one line before exiting, so the
# variable died with the process that set it and every run re-ran every
# check. A subprocess can only report back through a file, so the stamp is
# one: $DAISY_HOME/.healthcheck.ttl holds the unix time the pass expires.
#
# The cache must fail toward re-running. A cached *pass* is the dangerous
# direction — it can report a tree healthy using a result computed before it
# was edited — so a failure, a corrupt stamp, or a stamp beyond the TTL
# window must all drop back to a full run.

STAMP="$(fixture_home_dir)/.healthcheck.ttl"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "0" "clean fixture passes"
[ -f "$STAMP" ] && stamp_written=yes || stamp_written=no
assert_eq "$stamp_written" "yes" "a passing run writes the expiry stamp"

# Within the window, checks are skipped wholesale — proven by the absence of
# the per-component output a real run emits.
fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "0" "a cached run exits 0"
assert_not_contains "$RUN_STDOUT" "Component:" \
    "a cached run skips the component checks"

# --force must bypass the stamp and actually look.
mkdir -p "$FIXTURE_ROOT/home/zebrahome"
echo "this doc mentions zebrahome by name" > "$FIXTURE_ROOT/leak.md"

fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "0" "the stamp still suppresses the new leak (cache is honoured)"

fixture_run_cli healthcheck --force
assert_eq "$RUN_EXIT" "1" "--force ignores the stamp and finds the leak"
assert_contains "$RUN_STDERR" "Publication hygiene: 'zebrahome'" \
    "--force reports the leak"

# A failing run must clear the stamp: no stale pass may outlive a failure.
[ -f "$STAMP" ] && stamp_after_fail=yes || stamp_after_fail=no
assert_eq "$stamp_after_fail" "no" "a failing run clears the expiry stamp"

fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "1" "the next uncached run still fails"

rm -f "$FIXTURE_ROOT/leak.md"
rm -rf "$FIXTURE_ROOT/home/zebrahome"

fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "0" "passes again once the leak is gone"

# TTL=0 disables caching: the stamp expires the instant it is written.
export DAISY_HEALTHCHECK_TTL=0
fixture_run_cli healthcheck
assert_contains "$RUN_STDOUT" "Component:" \
    "TTL=0 re-runs the checks every time"
unset DAISY_HEALTHCHECK_TTL

# A corrupt stamp must not crash the numeric comparison under `set -e`, and
# must not be honoured.
fixture_run_cli healthcheck --force
echo "not-a-number" > "$STAMP"
fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "0" "a corrupt stamp does not break the run"
assert_contains "$RUN_STDOUT" "Component:" \
    "a corrupt stamp forces a full re-run"

# A stamp further out than the TTL permits is a bad write or a clock jump.
fixture_run_cli healthcheck --force
echo "9999999999" > "$STAMP"
fixture_run_cli healthcheck
assert_contains "$RUN_STDOUT" "Component:" \
    "a stamp beyond the TTL window is distrusted, not honoured forever"

# The documented external override still works — test 17 depends on it. It
# is checked with no stamp present, so the short-circuit can only be the
# variable's doing.
rm -f "$STAMP"
export DAISY_HEALTHCHECK_PASSED=1
fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "0" "the explicit env override short-circuits"
assert_not_contains "$RUN_STDOUT" "Component:" \
    "the explicit env override skips the checks"

# --force outranks the override — it is the way to say "look anyway".
fixture_run_cli healthcheck --force
assert_contains "$RUN_STDOUT" "Component:" \
    "--force overrides the env short-circuit"
unset DAISY_HEALTHCHECK_PASSED
