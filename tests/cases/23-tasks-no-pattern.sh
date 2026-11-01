# Test Plan #23: no pattern — every line of the selected file(s).

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
(C) 2026-07-20 Task two +other
EOF

fixture_run tasks --todo

assert_eq "$RUN_EXIT" "0" "listing succeeds"
line_count=$(printf '%s\n' "$RUN_STDOUT" | grep -c .)
assert_eq "$line_count" "2" "every todo.txt line is listed when no pattern is given"
