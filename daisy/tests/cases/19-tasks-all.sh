# Test Plan #19: `--all +proj` — matching lines from both todo.txt and
# done.txt.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
(C) 2026-07-20 Task two +other
EOF
fixture_write_done <<'EOF'
x 2026-07-01 2026-06-01 Old completed +proj
x 2026-07-02 2026-06-02 Unrelated +other
EOF

fixture_run tasks --all +proj

assert_eq "$RUN_EXIT" "0" "matches found"
assert_contains "$RUN_STDOUT" "Task one +proj" "todo.txt match present"
assert_contains "$RUN_STDOUT" "Old completed +proj" "done.txt match present"
assert_not_contains "$RUN_STDOUT" "+other" "unrelated tag excluded"
