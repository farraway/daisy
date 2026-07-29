# Test Plan #16: nothing completed in either file — exits non-zero, no ✅,
# no success line. (Covered structurally by resolver's no-match case, but
# asserted here as its own explicit contract check.)

seed_basic

fixture_run done "Totally Absent Pattern"

assert_eq "$RUN_EXIT" "1" "a run that completes nothing exits non-zero"
assert_not_contains "$RUN_STDOUT" "✅" "no success marker when nothing completed"
