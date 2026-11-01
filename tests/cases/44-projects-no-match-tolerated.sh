# Test Plan #44: a workspace need not map to any project. Deriving by
# convention must tolerate a miss silently (plenty of workspaces have no
# project), while a *declared* project that does not exist is a typo worth
# warning about — but never fatal.

fixture_workspace_named "nosuchthing.260101"

fixture_run files
assert_eq "$RUN_EXIT" "0" "no match is not an error"
assert_contains "$RUN_STDOUT" "none" "reports that nothing is bound"
assert_eq "$RUN_STDERR" "" "an unmatched convention default warns about nothing"

fixture_write_project_binding <<'EOF'
typoed-name
EOF

fixture_run files
assert_eq "$RUN_EXIT" "0" "a declared project that does not exist is still not fatal"
assert_contains "$RUN_STDERR" "typoed-name" "the unknown declared project is named"
