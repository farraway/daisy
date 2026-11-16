# 0122. A dependency step and the swarm's cache

Moved verbatim from `docs/swarm.md` section 14, item 122, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A dependency step and the swarm's cache (Step 2 of the plan for
the inbox requests F27–F35, 2026-11-14; requests F28 and F30, by
F30's (a), the maintainer's choice).** The check runs offline, so a
product image had to hold every dependency each head needs: gridapp's
was rebuilt six times as jobs adopted libraries, and the cache it
held was not writable by the check's uid (F28). Now a repository
declares `deps` (section 3), and before each check Daisy runs it on
the fetched head with the network, writing `<state>/deps/<name>`;
the check reads that cache read-only, still offline (7.2; 7.6, step
4). **The schema** is an object, `{"run": [...], "env": {...}}`, not
the bare argv the plan named: the cache's place inside a container is
the tool's to be told (`GOMODCACHE`, `npm_config_cache`,
`PIP_CACHE_DIR`), so Daisy fixes the mount point, `/deps`, and the
product names its tool's variables, which both containers are
given; an argv alone would have had Daisy know Go, and an `env`
beside `deps` could exist without it. A declared mount point was
the other reading; mounting at the image's own cache path would hide
the image's cache and take its owner. **A host directory, not a
Docker volume** (the plan's `daisy-swarm-<owner>-<repo>-<name>-deps`):
the daemon makes a new volume's root root's, or copies the owner of
the image's directory at the mount point, so the step, run as the
host user, could not write it, F28's failure again; a directory
under `<state>` is the host user's, per swarm and per repository by
its path, needs no name made safe for Docker and no `docker volume
create`, and the maintainer prunes it with `rm -rf` (Go's
`-modcacherw` keeps its files deletable). **No member gets the
cache**, though the plan ran the step before a worker's run too: a
member is on Docker's default bridge already (7.2), so the worker
fetches what it needs; the cache read-only with `env` would refuse
its own fetch of a new dependency, and read-write would let a member
write what other jobs' checks read. F30's (a) names the check alone.
**No `GOCACHE`** is set by Daisy: the check's own filesystem is per
run (`--rm`), and the entrypoint's `HOME`, where Go's build cache
lies by default, is writable by any uid; an image that sets an
unwritable one is given another through `env`. **The network** is
the members' and the dependency step's, never the check's. **The
head's clone is read-only** to the step, so that it writes the cache
alone and the check sees the head as fetched. **Two steps at once**
on one repository are not serialized by Daisy: Go's module cache is
safe for concurrent writers (the go command's modfetch package: a
`lockedfile` mutex per module version, extraction behind a
`.partial` file that readers treat as absent), and a tool whose
cache is not says so in its own `run` (`flock`), which knows the
tool, where a lock in Daisy would be held across asynchronous
containers and lost with a dead `apply`. **A step that fails** or
times out is the check's result, named as the dependency step's in
the check block and the reviewers' `# Check`, so a module that could
not be fetched is not read as a failing test (F28's "Also"); one
Docker cannot run (125–127) is a swarm fault, as a check's is.
**The wizard asks nothing**: `check` is not asked either, and an
argv is edited in the file; it keeps the file's `deps`. Tests:
`TestSwarmDepsBeforeTheCheck`, `TestSwarmCheckWithoutDepsUnchanged`,
`TestSwarmDepsFailure`, `TestSwarmDepsRestartedAfterItsApplyDied`,
`TestSwarmApplyWorkFailures` (a secret in `deps.out`), `TestArgs`,
`TestValidateNamesTheField`, `TestParseNamesTheField`,
`TestFormatDeps` and `TestWizardKeepsTheFilesDeps`.
