# Test Plan #21: `--done +proj` — done.txt matches only.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
EOF
fixture_write_done <<'EOF'
x 2026-07-01 2026-06-01 Old completed +proj
EOF

fixture_run tasks --done +proj

assert_eq "$RUN_EXIT" "0" "matches found"
assert_contains "$RUN_STDOUT" "Old completed +proj" "done.txt match present"
assert_not_contains "$RUN_STDOUT" "Task one" "todo.txt excluded under --done"
