# Test Plan #42: an explicit .daisy/project file is authoritative — it
# replaces the convention-derived default rather than extending it. The
# motivating case is a rename (daisy-dev -> +daisy), which extending cannot
# express without a second mechanism.

fixture_workspace_named "chessgrid.260807"
fixture_add_project "chessgrid"
fixture_add_project "jobsearch"

fixture_write_project_binding <<'EOF'
jobsearch
EOF

fixture_run files

assert_eq "$RUN_EXIT" "0" "files succeeds with an explicit binding"
assert_contains "$RUN_STDOUT" "+jobsearch" "explicit project reported"
assert_not_contains "$RUN_STDOUT" "+chessgrid" "directory-name default is replaced, not extended"
assert_contains "$RUN_STDOUT" ".daisy/project" "source of the binding is named"
