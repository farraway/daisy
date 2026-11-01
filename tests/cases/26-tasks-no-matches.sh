# Test Plan #26: no matches — nothing on stdout, exit 1.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
EOF

fixture_run tasks --todo "Nonexistent Pattern"

assert_eq "$RUN_EXIT" "1" "no matches is a non-zero exit, grep-style"
assert_eq "$RUN_STDOUT" "" "nothing on stdout when there are no matches"
