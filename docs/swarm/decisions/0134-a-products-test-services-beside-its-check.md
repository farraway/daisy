# 0134. A product's test services beside its check

Moved verbatim from `docs/swarm.md` section 14, item 134, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A product's test services beside its check (Step 16 of the plan
for the inbox requests F27–F35, 2026-11-15; request F29).** gridapp's
contract tests against a real MongoDB and Redis read two variables
and skip without them, so the check, with no network, never ran
them, and a head that failed one passed its check and both reviews
and failed CI at merge. **The product declares them**, not the swarm
(the maintainer: "it should be attached to gridapp rather than the
swarm"): a Compose file in the product's repository,
`compose.test.yaml` at its root unless `repos[].services` names
another, read at the commit run, so a job may add a service with
the change that needs it; `swarm.json` gains only that optional
path. **Docker, not Compose**: Daisy runs a stated subset itself
(`image`, `tmpfs`, `environment`, `command`, and `x-daisy-env` for
the variables), refusing every other key by name, so a file is run
whole or not at all and nothing it names (a port, a mount, a
privileged flag) reaches the host; Compose would be a second
dependency of every swarm host, with its own project naming and
network rules to keep apart from the swarm's labels. **No YAML
library**: the root is the standard library alone (AGENTS.md), so
`declare` reads a YAML subset (block mappings and lists, one-line
flow collections, plain and quoted scalars, comments) and refuses
the rest by line (anchors, tags, block scalars, a second document,
tabs); a JSON-only file was the other reading, but a Compose file
is YAML to every other reader of it. **Networks**: the check's is
`--internal`, made for it alone, joined in place of `none`, so it
still reaches nothing outside; the worker's is a bridge network of
its own, not internal, because its CLI must reach its API: it is put
on that network alone, one `--network` with the default bridge's
route out, which works on every Docker version,
where joining the default bridge and an internal network at once
takes two `--network` flags (Docker 25 and later) or a `docker
network connect` after the container has started (a race with the
CLI's first request). The worker's services share its route out;
the check's have none. **Readiness**: none in Daisy. Its tests
wait for their server as they do beside CI's services, which have
no health check either; a wait with a cap in Daisy would be an
invented limit, and `healthcheck` stays out of the subset until a
product needs it. **Removal** is a step of the collection, the
first once no container of the run runs, whatever its outcome, by
label (`daisy.services=<net>`) so that it is the same whether made
once or twice: the result's `services <step> up` lines, written
before anything is made, against its `services <step> down` lines
say what remains, so services an `apply` that died left are removed
by the next pass's collection, or before the check is started again.
Services carry no `daisy.swarm` label, so they count in neither
`max_members` nor `status`. **Failures**: a file refused at the head
is the check's result (the worker can fix it; the reviewers see
why), while at a worker's launch it is a warning and the run goes
on without services (a fault there would relaunch the worker into
the same refusal on its own branch, never letting it fix the file);
an image not on the host is a swarm fault (`--pull never`, as for
the members' images: the maintainer pulls `mongo:7`; Daisy never
pulls an image a file a worker edits names), two of which pause the
swarm with the reason. Not run beside a reviewer, whose clone may
build and test too: not asked for. Tests:
`TestSwarmServicesBesideTheCheck`, `TestSwarmNoServicesFile`,
`TestSwarmServicesRefused`, `TestSwarmServicesCannotStart`,
`TestSwarmServicesRestartedAfterTheirApplyDied`,
`TestSwarmCheckWithoutDepsUnchanged`, `TestParseServices`,
`TestParseServicesRefused`, `TestArgs`, `TestServicesArgs`,
`TestDockerServices`, `TestValidateNamesTheField`, `TestFormatDeps`
and `TestWizardKeepsTheFilesDeps`.

**Amended by decision [160](0160-one-declared-check-environment.md)
(Step 10 of the plan implementing the 2026-11-19 swarm roadmap, R3),
built at its Steps R3-2 and R3-3.** The subset gains `ulimits` (run as
`--ulimit`) and, in its YAML, literal and folded block scalars; and a
reviewer's run is given the services and `x-daisy-env` as a worker's
is, read at the head it reviews.
