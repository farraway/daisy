# Test Plan #47: --all expands scope past the workspace's projects. It no
# longer means "search both files" — that is what naming no file filter
# does — so the two axes combine freely.

fixture_workspace_named "proj.260807"
fixture_add_project "proj"

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
(C) 2026-07-20 Task two +other
EOF
fixture_write_done <<'EOF'
x 2026-07-02 2026-06-02 Unrelated +other
EOF

fixture_run_cli tasks --all

assert_eq "$RUN_EXIT" "0" "listing succeeds"
assert_contains "$RUN_STDOUT" "Task one +proj" "workspace project still listed"
assert_contains "$RUN_STDOUT" "Task two +other" "other project's todo task now in scope"
assert_contains "$RUN_STDOUT" "Unrelated +other" "other project's done task now in scope"

# The axes are orthogonal: --all widens scope, --todo narrows files.
fixture_run_cli tasks --all --todo

assert_contains "$RUN_STDOUT" "Task two +other" "every project's todo tasks"
assert_not_contains "$RUN_STDOUT" "Unrelated +other" "done.txt excluded by --todo"
