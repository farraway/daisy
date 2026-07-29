# Test Plan #25: pattern starting with `-` — treated as a pattern, not a
# flag.

fixture_write_todo <<'EOF'
(B) 2026-07-27 -1 point deduction task +proj
EOF

fixture_run tasks --todo "-1 point"

assert_eq "$RUN_EXIT" "0" "a pattern beginning with '-' is not mistaken for a flag"
assert_contains "$RUN_STDOUT" "-1 point deduction task" "the dash-leading pattern matched"
