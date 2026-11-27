# `swarm init --defaults` through the binary over the fake forge
# DAISY_FORGE names (docs/swarm.md section 10, The fake forge's file), in
# a folder holding the product's clone: the maintainer's token creates
# acme/swarm-widget and gives swarmbot write access, init clones it beside
# the product's, commits and pushes its files as the maintainer, and the
# swarm's token makes the eighteen labels, every write held in the file;
# a second run, from the clone, adopts the repository, writes nothing and
# creates no label.

fixture_fake_forge
clone="$FIXTURE_WORKSPACE/swarm-widget"

fixture_run_cli swarm init --defaults
assert_eq "$RUN_EXIT" "0" "init exits 0"
assert_contains "$RUN_STDOUT" "✓ Maintainer maintainer: acme/swarm-widget created" "the maintainer's token creates the swarm repository"
assert_contains "$RUN_STDOUT" "✓ Maintainer maintainer: swarmbot may write to acme/swarm-widget" "and gives the swarm's account write access"
assert_contains "$RUN_STDOUT" "✓ Cloned acme/swarm-widget into $clone" "init clones it beside the product's"
assert_contains "$RUN_STDOUT" "✓ Token \$GITEA_TOKEN reads acme/swarm-widget as swarmbot" "the swarm's token reads it"
assert_contains "$RUN_STDOUT" "✓ Token \$GITEA_TOKEN: labels 18 present (18 created, 0 renamed)" "the swarm's token makes the labels"
assert_eq "$(git -C "$FIXTURE_UPSTREAMS/acme/swarm-widget.git" log --format='%an %s' main)" "Fixture Maintainer swarm: written by init" "one commit is pushed, as the maintainer"
assert_contains "$(cat "$FIXTURE_FORGE")" '"CreatedBy": "maintainer"' "the file holds the repository, created by the maintainer"
assert_contains "$(cat "$FIXTURE_FORGE")" '"swarmbot": "write by maintainer"' "and the swarm's access to it"
assert_contains "$(cat "$FIXTURE_FORGE")" '"swarm:proposal-posted"' "and the labels"
[ -f "$clone/swarm.json" ] && written=yes || written=no
assert_eq "$written" "yes" "the clone holds swarm.json"

FIXTURE_CWD="$clone"
fixture_run_cli swarm init --defaults
assert_eq "$RUN_EXIT" "0" "a second init exits 0"
assert_contains "$RUN_STDOUT" "  Maintainer maintainer: acme/swarm-widget exists" "the second run adopts the repository"
assert_contains "$RUN_STDOUT" "  Nothing to commit" "and commits nothing"
assert_contains "$RUN_STDOUT" "labels 18 present (0 created, 0 renamed)" "and creates no label"
assert_eq "$(git -C "$FIXTURE_UPSTREAMS/acme/swarm-widget.git" rev-list --count main)" "1" "and pushes nothing"
