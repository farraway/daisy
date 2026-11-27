# The swarm verbs that need no forge, run through the binary in a scratch
# swarm repository's clone (swarm.json and .git at its top, no origin, so
# nothing can reach a network): `swarm pause` writes <state>/paused under
# $DAISY_LOCAL/swarm/<forge-host>/<owner>/<repo>, a second pause says
# since when, `swarm list --json` reports the swarm paused and here,
# `swarm resume` removes the file and list reports it unpaused;
# `swarm prompt build --dry-run` names the five prompts it would write and
# writes none; outside a clone, pause refuses and writes no state.

clone="$FIXTURE_WORKSPACE/swarm-widget"
mkdir -p "$clone"
git -C "$clone" init -q
cat > "$clone/swarm.json" <<'EOF'
{
  "forge": {"kind": "gitea", "api": "https://forge.example.invalid/api/v1", "repo": "acme/swarm-widget", "token_env": "GITEA_TOKEN", "commit_email": "swarm@example.invalid"},
  "repos": [{"name": "widget", "repo": "acme/widget", "check": [], "image": "daisy-member:1", "protected": ["AGENTS.md"]}],
  "base": "main",
  "max_members": 3,
  "max_rounds": 5,
  "timeout": "45m",
  "interval": "5m",
  "owner": {"max_open_proposals": 5},
  "resources": ["briefs/", "docs/"],
  "trusted": ["maintainer", "swarmbot"],
  "gates": {"triage": true, "plan": true, "merge": true},
  "keys": {"anthropic": {"env": "CLAUDE_CODE_OAUTH_TOKEN"}},
  "personas": {
    "owner": {"provider": "anthropic", "model": "model-a", "effort": "high"},
    "architect": {"provider": "anthropic", "model": "model-a", "effort": "high"},
    "worker": {"provider": "anthropic", "model": "model-a", "effort": "medium"},
    "adversary": {"provider": "anthropic", "model": "model-a", "effort": "high"},
    "sympathetic": {"provider": "anthropic", "model": "model-a", "effort": "high"}
  }
}
EOF
state="$DAISY_LOCAL/swarm/forge.example.invalid/acme/swarm-widget"
FIXTURE_CWD="$clone"

fixture_run_cli swarm list --json
assert_eq "$RUN_EXIT" "0" "swarm list exits 0 before any state"
assert_contains "$RUN_STDOUT" '"swarms": []' "no swarm is listed before any state"

fixture_run_cli swarm pause
assert_eq "$RUN_EXIT" "0" "swarm pause exits 0"
assert_eq "$RUN_STDOUT" "Paused: no new member will launch ($state/paused); running members finish and are posted" "pause names the state file it wrote"
[ -f "$state/paused" ] && paused=yes || paused=no
assert_eq "$paused" "yes" "<state>/paused is written under DAISY_LOCAL"
since=$(sed -n 's/^paused //p' "$state/paused")

fixture_run_cli swarm pause
assert_eq "$RUN_EXIT" "0" "a second pause exits 0"
assert_eq "$RUN_STDOUT" "Already paused since $since" "a second pause says since when"

fixture_run_cli swarm list --json
assert_eq "$RUN_EXIT" "0" "swarm list exits 0"
assert_contains "$RUN_STDOUT" '"repository": "acme/swarm-widget"' "list names the swarm"
assert_contains "$RUN_STDOUT" '"host": "forge.example.invalid"' "list names its forge host"
assert_contains "$RUN_STDOUT" "\"since\": \"$since\"" "list reports it paused, since the pause"
assert_contains "$RUN_STDOUT" "\"here\": \"$clone\"" "list reports the clone the working folder is in"

fixture_run_cli swarm resume
assert_eq "$RUN_EXIT" "0" "swarm resume exits 0"
assert_eq "$RUN_STDOUT" "Resumed: members launch at the next pass" "resume says members launch again"
[ -e "$state/paused" ] && paused=yes || paused=no
assert_eq "$paused" "no" "<state>/paused is removed"

fixture_run_cli swarm resume
assert_eq "$RUN_STDOUT" "Not paused" "a second resume says it was not paused"

fixture_run_cli swarm list --json
assert_contains "$RUN_STDOUT" '"paused": null' "list reports the swarm unpaused"

fixture_run_cli swarm prompt build --dry-run
assert_eq "$RUN_EXIT" "0" "prompt build --dry-run exits 0"
for role in owner architect worker adversary sympathetic; do
    assert_contains "$RUN_STDOUT" "Would write prompts/$role.md" "the dry run names $role's prompt"
done
assert_contains "$RUN_STDOUT" "Dry run: nothing written" "the dry run says it wrote nothing"
[ -e "$clone/prompts" ] && written=yes || written=no
assert_eq "$written" "no" "and no prompt is written"
assert_eq "$(git -C "$clone" status --porcelain)" "?? swarm.json" "the clone holds only its swarm.json"

rm -rf "$DAISY_LOCAL/swarm"
FIXTURE_CWD="$FIXTURE_WORKSPACE"
fixture_run_cli swarm pause
assert_eq "$RUN_EXIT" "1" "pause outside a clone exits 1"
assert_contains "$RUN_STDERR" "swarm.json: not found in $FIXTURE_WORKSPACE" "it names where it looked"
[ -e "$DAISY_LOCAL/swarm" ] && wrote=yes || wrote=no
assert_eq "$wrote" "no" "and writes no state"
