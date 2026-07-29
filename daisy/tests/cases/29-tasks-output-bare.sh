# Test Plan #29: output is bare — matching lines only, no headers, no
# decoration, no summary count.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
(C) 2026-07-20 Task two +proj
EOF

fixture_run tasks --todo +proj

assert_eq "$RUN_EXIT" "0" "matches found"
assert_not_contains "$RUN_STDOUT" "Active projects" "no header text"
line_count=$(printf '%s\n' "$RUN_STDOUT" | grep -c .)
assert_eq "$line_count" "2" "exactly the matching lines, nothing decorative added"
