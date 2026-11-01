# Test Plan #45: `daisy status` reports the workspace's projects too — it is
# the command an agent runs to orient itself, so the binding has to be
# visible there and not only in `daisy files`.

fixture_workspace_named "chessgrid.260807"
fixture_add_project "chessgrid"

fixture_run_cli status

assert_eq "$RUN_EXIT" "0" "status succeeds"
assert_contains "$RUN_STDOUT" "+chessgrid" "status names the workspace's project"
