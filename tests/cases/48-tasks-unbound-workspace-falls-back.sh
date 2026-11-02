# Test Plan #48: a workspace bound to no project falls back to every task
# rather than to none. Returning nothing would be a silent lie — the tasks
# exist — so the fallback is announced on stderr and stdout stays bare.

fixture_workspace_named "unbound.260101"

fixture_write_todo <<'EOF'
(B) 2026-07-27 Task one +proj
(C) 2026-07-20 Task two +other
EOF

fixture_run_cli tasks

assert_eq "$RUN_EXIT" "0" "listing succeeds"
assert_contains "$RUN_STDOUT" "Task one +proj" "unscoped fallback lists everything"
assert_contains "$RUN_STDOUT" "Task two +other" "including other projects"
assert_contains "$RUN_STDERR" "no project" "the fallback is announced, not silent"
