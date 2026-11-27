# `swarm status` through the binary over the fake forge DAISY_FORGE names
# (docs/swarm.md section 10, The fake forge's file), once init has made
# the swarm: an issue the file holds at proposal-posted is counted and
# waits for the maintainer's triage; `--waiting` prints that line alone;
# status writes nothing to the file; and a token the file does not name
# (the swarm's, once the file no longer takes it) is the forge's 401,
# exit 1 with nothing on stdout.

fixture_fake_forge
clone="$FIXTURE_WORKSPACE/swarm-widget"
fixture_run_cli swarm init --defaults
assert_eq "$RUN_EXIT" "0" "init exits 0"

# The issue as the owner's run would have posted it: the file's next
# number in acme/swarm-widget, under swarmbot.
issue='"items": [{"repo": "swarm-widget", "n": 1, "title": "A job", "author": "swarmbot", "open": true, "labels": ["swarm:proposal-posted"]}],
  "next": {"swarm-widget": 1},'
awk -v issue="$issue" '/^  "items": null,$/ { next } /^  "next": / { print issue; next } { print }' "$FIXTURE_FORGE" > "$FIXTURE_FORGE.new" && mv "$FIXTURE_FORGE.new" "$FIXTURE_FORGE"
before=$(cat "$FIXTURE_FORGE")

FIXTURE_CWD="$clone"
fixture_run_cli swarm status
assert_eq "$RUN_EXIT" "0" "status exits 0"
assert_eq "$RUN_STDERR" "" "status writes nothing to stderr"
assert_contains "$RUN_STDOUT" "Swarm: acme/swarm-widget on forge.example.invalid (gitea), repositories widget" "status names the swarm"
assert_contains "$RUN_STDOUT" "  proposal-posted            1  #1" "the issue is counted at its stage"
assert_contains "$RUN_STDOUT" "Running: none" "the docker that lists none runs nothing"
assert_contains "$RUN_STDOUT" "Waiting for you: triage #1" "and waits for the maintainer's triage"

fixture_run_cli swarm status --waiting
assert_eq "$RUN_STDOUT" "Waiting for you: triage #1" "--waiting prints that line alone"
assert_eq "$(cat "$FIXTURE_FORGE")" "$before" "status writes nothing to the forge"

sed 's/"tok-swarm"/"tok-other"/' "$FIXTURE_FORGE" > "$FIXTURE_FORGE.new" && mv "$FIXTURE_FORGE.new" "$FIXTURE_FORGE"
fixture_run_cli swarm status
assert_eq "$RUN_EXIT" "1" "a token the file does not name exits 1"
assert_contains "$RUN_STDERR" "401" "as the forge's 401"
assert_eq "$RUN_STDOUT" "" "with nothing on stdout"
