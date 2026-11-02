# Test Plan #43: with no .daisy/project, the workspace's project is derived
# from the directory basename before the first dot. Workspaces are named
# {project}.{date}, so the convention already sits in the filesystem.

fixture_workspace_named "chessgrid.260807"
fixture_add_project "chessgrid"

fixture_run_cli files

assert_eq "$RUN_EXIT" "0" "files succeeds on the convention path"
assert_contains "$RUN_STDOUT" "+chessgrid" "project derived from directory name"
assert_contains "$RUN_STDOUT" "directory name" "source of the binding is named"

# Multiple projects: one per line, and order is preserved.
fixture_write_project_binding <<'EOF'
chessgrid
jobsearch
EOF
fixture_add_project "jobsearch"

fixture_run_cli files
assert_contains "$RUN_STDOUT" "+chessgrid +jobsearch" "several projects listed in file order"
