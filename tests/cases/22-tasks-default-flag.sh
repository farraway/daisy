# Test Plan #22: no file flag — both files. --todo/--done select files, so
# naming neither means neither is excluded. (Before the rescope the default
# was --todo; scope and file-selection were conflated on one flag.)

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
EOF
fixture_write_done <<'EOF'
x 2026-07-01 2026-06-01 Old completed +proj
EOF

fixture_run tasks +proj

assert_eq "$RUN_EXIT" "0" "matches found"
assert_contains "$RUN_STDOUT" "Task one +proj" "todo.txt searched by default"
assert_contains "$RUN_STDOUT" "Old completed +proj" "done.txt searched by default too"
