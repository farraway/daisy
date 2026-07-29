# Test Plan #27: `done.txt` absent — not an error; todo.txt results still
# returned.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
EOF
rm -f "$(fixture_home_dir)/tasks/done.txt"

fixture_run tasks --all +proj

assert_eq "$RUN_EXIT" "0" "a missing done.txt is not an error"
assert_contains "$RUN_STDOUT" "Task one +proj" "todo.txt results still returned"
