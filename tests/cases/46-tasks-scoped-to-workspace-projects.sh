# Test Plan #46: `daisy tasks` is scoped to the workspace's projects by
# default. An agent working in a single-project workspace should not have to
# read the entire cross-project backlog to find its own tasks.

fixture_workspace_named "proj.260807"
fixture_add_project "proj"

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
(C) 2026-07-20 Task two +other
EOF
fixture_write_done <<'EOF'
x 2026-07-01 2026-06-01 Old completed +proj
x 2026-07-02 2026-06-02 Unrelated +other
EOF

fixture_run tasks

assert_eq "$RUN_EXIT" "0" "listing succeeds"
assert_contains "$RUN_STDOUT" "Task one +proj" "workspace project's todo task listed"
assert_contains "$RUN_STDOUT" "Old completed +proj" "workspace project's done task listed"
assert_not_contains "$RUN_STDOUT" "+other" "other projects are out of scope"
