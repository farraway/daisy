# Test Plan #24: pattern with `.` / `[` / `/` — literal match only, no
# regex interpretation.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Release version 1.0 +proj
(B) 2026-07-27 Release version 1X0 +proj
(B) 2026-07-27 Fix a/b path +proj
EOF

fixture_run tasks --todo "version 1.0"

assert_eq "$RUN_EXIT" "0" "literal dot pattern matches"
assert_contains "$RUN_STDOUT" "version 1.0" "the literal match is present"
assert_not_contains "$RUN_STDOUT" "1X0" "'.' is not treated as regex any-char"

fixture_run tasks --todo "a/b path"
assert_eq "$RUN_EXIT" "0" "a pattern containing / matches literally, no syntax error"
assert_contains "$RUN_STDOUT" "a/b path" "the slash-containing match is present"
