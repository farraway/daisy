# Test Plan #22: no flag — defaults to `--todo`.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
EOF
fixture_write_done <<'EOF'
x 2026-07-01 2026-06-01 Old completed +proj
EOF

fixture_run tasks +proj

assert_eq "$RUN_EXIT" "0" "matches found"
assert_contains "$RUN_STDOUT" "Task one +proj" "todo.txt match present by default"
assert_not_contains "$RUN_STDOUT" "Old completed" "done.txt excluded by default"
