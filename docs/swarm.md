# The swarm — Specification

**Status.** Specified at Step 26 from the plan's agreed design; built in
Steps 27–30; nothing here runs yet. The maintainer confirms or amends this
document before Step 27 begins (the checkpoint), and section 14 lists the
places where it chose between readings of the plan. `docs/cli.md`'s
entry for the Swarm group points here: this document is the contract for the
five verbs, the configuration they read, the forge calls they make, the
containers they run and every file they write.

**Conventions.** As in [docs/cli.md](cli.md): tables, exact strings in
backticks, exit codes. `<…>` is a placeholder: `<ws>` the workspace root,
`<owner>/<repo>` the issues repository on the forge, `<forge-host>` its
host, `<n>` an issue number, `<m>` a pull request number, `<role>` one of
the five roles, `<state>` the swarm's state directory
`$DAISY_LOCAL/swarm/<forge-host>/<owner>/<repo>`, `<run>` one run's
directory `<state>/runs/<n>-<role>-<k>`. A row marked *to confirm at
Step N* is written from the plan and the tools' public documentation and
is checked against the real tool at that step. The commands named are the
agreed Swarm group (`swarm init [--defaults]`, `swarm apply [--once]`,
`swarm status`, `swarm pause`, `swarm resume`), not yet rows of the `cli`
table; `cmd/daisy/docs_test.go` admits exactly those five verbs and two
flags in this document until each verb joins the table.

## 1. Principle and scope

**The forge is the interface.** An issue is a job; its `swarm:*` label is
its stage; its comments are the conversation between the maintainer and
each persona; a pull request is the worker's output and the reviewers'
surface; the merge button is the gate. **Daisy is a reconciler:** `swarm
apply` reads the forge, runs the members whose turn it is, posts what they
wrote and moves the label. **Members have no memory:** each run is a fresh
container given the thread so far; the thread is the memory. All
durable state is on the forge; `<state>` holds only what one machine needs
to run members (a lock, a pause flag, a git mirror, the audit record of
each run).

| In version 1 | Not in version 1 |
|---|---|
| Five roles: owner, architect, worker, adversary, sympathetic | A swarm that merges: the merge is the maintainer's act on the forge (section 12) |
| Two forges: `gitea` (API v1), `github` (REST v3) | Webhooks: `apply` polls, by hand or on a user timer |
| Two providers: `anthropic` (`claude` CLI), `openai` (`codex` CLI) | `xai`: a persona naming it is a validation error until a non-interactive xAI agent CLI exists or the API adapter is built |
| Members as vendor agent CLIs in an ephemeral container | An API-loop provider adapter (Daisy's own tool-use loop); the `provider` seam is where it lands |
| One forge token; each comment marked with its role | One forge account per persona |
| Container limits: no mounted state, one key, a wall clock | Network egress filtering and further hardening |
| `apply` from the maintainer's terminal or a systemd user timer | `apply` inside an agent session: the agent's sandbox has no Docker socket, and the swarm must not depend on a chat being open |

## 2. Files

```
<ws>/swarm/                          # configuration, in the workspace repository; written by `swarm init` only
├── swarm.json                       # forge, repository, personas, image, limits (section 3)
└── prompts/<role>.md                # each persona's prompt (section 4)
<state>/                             # $DAISY_LOCAL/swarm/<forge-host>/<owner>/<repo>/, this machine's runtime state
├── lock                             # held (flock) for the life of one `swarm apply`; names the holder
├── paused                           # present while paused
├── owner-held                       # present while the owner's batch trigger is held (section 6)
├── git/                             # bare mirror of the product repository: the only repository host-side git writes
└── runs/<n>-<role>-<k>/             # one role run, kept for audit (the owner's batch run is <n> = 0)
    ├── argv                         # the `docker run` argv, one argument per line (no secret in it)
    ├── io/prompt.md                 # the prompt the member was sent (mounted at /work/.swarm)
    ├── io/out.md                    # what the member wrote
    ├── stdout, stderr               # the provider CLI's raw output
    ├── result                       # how the run ended and what was posted (below)
    └── work/                        # the member's clone; removed once the run is collected
```

Every file Daisy writes, and nothing else:

| Path | Holds | Written by | When |
|---|---|---|---|
| `swarm/swarm.json` | the configuration, two-space indented JSON, fields in section 3's order, trailing newline | `swarm init` | when the answers differ from the file (absent counts as different) |
| `swarm/prompts/<role>.md` | the default persona prompt (section 4) | `swarm init` | only when the file is absent; an existing prompt is never overwritten |
| `<state>/lock` | one line, the holder: `apply[ --once] since <RFC 3339 time> pid <pid>` | `swarm apply` | on taking the lock; the lock is released, not the file removed, at exit |
| `<state>/paused` | one line: `paused <RFC 3339 time>` | `swarm pause` (created), `swarm resume` (removed) | on the command |
| `<state>/owner-held` | one line: `held <RFC 3339 time>: <reason>` | `swarm apply` (created), `swarm resume` (removed) | when the owner's batch run fails or proposes nothing |
| `<state>/git/` | a bare repository: `refs/heads/*` as last fetched from the forge, plus a worker's branch before its push | `swarm apply` (`git init --bare`, `fetch`) | before each pass's first launch; after each worker run |
| `<run>/argv`, `<run>/io/prompt.md` | as above | `swarm apply` | before the launch |
| `<run>/io/out.md` | the member's output | the member (in the container) | during the run |
| `<run>/stdout`, `<run>/stderr` | the CLI's streams, as written | `swarm apply` (copying the container's streams) | when the run ends |
| `<run>/result` | `start <sha>` and `started <time>`; then the end line, `exited <code> <time>`, `timeout <time>` or `abandoned <time>`; then `failed: <reason>` when it failed (7.7); then one line per step done at collection (`creating <i>` before each batch proposal, `posted comment`, `pushed <sha>`, `created pull <m>`, `relabelled <from> <to>`, `stale <sha>`, …); last `collected`, which every run ends with, abandoned and stale ones included | `swarm apply` | at each event |
| `<run>/work/` | the clone the member works in | `swarm apply` (`git clone` from `<state>/git`), then the member | before the launch; removed when the run is collected or abandoned |
| the forge | comments, reviews, issues, a pull request, label changes, a pushed `swarm/<n>` branch, the seven labels | `swarm init` (labels), `swarm apply` (the rest) | sections 5–7 and 11 |

**Invariant.** Daisy writes nowhere else: not to a Daisy home, not to the
workspace outside `swarm/` (and there only at `swarm init`), not to the
product repository's working tree or `.git` (it reads its `origin` URL and
`core.sshCommand`, nothing more), not to `~/.config`, and never into a
container except through the two bind mounts of `<run>`. Step 30's claim
checks it on the real run.

## 3. `swarm.json`

The example, as `swarm init --defaults` would write it for a product
repository at `<ws>/app` whose `origin` is on a Gitea host, with
`docs/` present in the workspace:

```json
{
  "forge": {
    "kind": "gitea",
    "api": "https://<forge-host>/api/v1",
    "repo": "<owner>/<repo>",
    "token_env": "GITEA_TOKEN"
  },
  "repo": "app",
  "base": "main",
  "image": "daisy-member:1",
  "max_members": 3,
  "max_rounds": 3,
  "timeout": "45m",
  "interval": "5m",
  "owner": {"max_open_proposals": 5},
  "resources": ["docs/"],
  "keys": {"anthropic": "ANTHROPIC_API_KEY", "openai": "OPENAI_API_KEY"},
  "personas": {
    "owner":       {"provider": "anthropic", "model": "claude-opus-5-5",   "effort": "high"},
    "architect":   {"provider": "anthropic", "model": "claude-fable-5-1",  "effort": "high"},
    "worker":      {"provider": "anthropic", "model": "claude-sonnet-5-5", "effort": "medium"},
    "adversary":   {"provider": "openai",    "model": "",                  "effort": "high"},
    "sympathetic": {"provider": "anthropic", "model": "claude-opus-5-5",   "effort": "high"}
  }
}
```

`init` writes the layout above: `forge` and `personas` expanded, two spaces
per level, each persona on one line; `owner`, `keys` and `resources` on one
line each. `swarm.json` is read by every verb but
`init` reads it only for its defaults; it is re-read at the start of every
pass of `apply`, so an edit applies at the next pass.

| Field | Type | Default (`init`) | Rule |
|---|---|---|---|
| `forge.kind` | string | detected from the product repository's `origin` (section 10) | `gitea` or `github`: a kind with an adapter |
| `forge.api` | string | `https://<host>/api/v1` (gitea), `https://api.github.com` (github) | an absolute `http` or `https` URL, no user information, no query, no trailing `/` |
| `forge.repo` | string | `<owner>/<repo>` from `origin` | `^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`; the issues repository; it must equal `origin`'s `<owner>/<repo>`, since `swarm/<n>` is pushed to `origin` and the pull request is opened in `forge.repo` (section 14, item 17) |
| `forge.token_env` | string | `GITEA_TOKEN` (gitea), `GITHUB_TOKEN` (github) | a variable name, `^[A-Za-z_][A-Za-z0-9_]*$`; the token itself never appears in any file Daisy writes |
| `repo` | string | the product repository found from `<ws>` (section 11, the wizard) | a relative path inside `<ws>` (`.` when the workspace is the product) naming a git repository with an `origin` remote; an `http(s)` origin URL carries no user information at all (a token as user name included); an SSH URL's `user@` is allowed, a password in it is not |
| `base` | string | `main` | a branch name: `^[A-Za-z0-9._][A-Za-z0-9._/-]*$`, no `..`, not ending in `/` or `.lock`; the branch pull requests target and clones start from; it must be the forge's default branch, since both forges close an issue named by `Closes #<n>` only on a merge into it (checked with host git, `git ls-remote --symref <origin-url> HEAD`, by `init` and by `apply`'s Start only: not by `status`, which runs read-only, nor by a pass's re-validation; that Gitea closes only on the default branch is *to confirm at Step 27*) |
| `image` | string | `daisy-member:1` | non-empty; a local image (`docker run --pull never`) |
| `max_members` | integer | 3 | 1 to 16: containers of this repository running at once |
| `max_rounds` | integer | 3 | 1 to 10: review rounds before `blocked` |
| `timeout` | string | `45m` | a Go duration, 1m to 24h: one run's wall clock |
| `interval` | string | `5m` | a Go duration, 1m to 24h: between passes of `apply` without `--once`, and the timer's period |
| `owner.max_open_proposals` | integer | 5 | 1 to 20: the most issues one batch run creates |
| `resources` | array of strings | `["docs/"]` when `<ws>/docs` is a directory, else `[]` | each a relative path; resolved through every symbolic link (`Ops.FS().EvalSymlinks`), it must lie inside `<ws>` resolved the same way, exist, not be `<ws>` itself, not lie under `swarm/`, and neither be nor contain `.env.sh`, `.daisy` or `.git`; no two the same; the resolved path is what is mounted, read-only, at `/resources/<path>` (a link inside a mounted tree resolves inside the container, where its target is absent) |
| `keys` | object | `{"anthropic": "ANTHROPIC_API_KEY", "openai": "OPENAI_API_KEY"}` | provider name → the variable holding its key on the host; every provider a persona uses must have one |
| `personas` | object | the five roles as in the example | exactly the keys `owner`, `architect`, `worker`, `adversary`, `sympathetic` |
| `personas.<role>.provider` | string | as in the example | a provider with an adapter: `anthropic` or `openai` |
| `personas.<role>.model` | string | as in the example | passed through, but for one check: no whitespace (it is one argv word; section 14, item 8); `""` means the CLI's own default (no model flag) |
| `personas.<role>.effort` | string | as in the example | passed through, but must match `^[a-z]+$` (the "bad effort" Step 27 tests); the vendor's list is not checked |
| (each persona) | — | — | `swarm/prompts/<role>.md` exists and is non-empty |

Validation runs before anything else in every verb but `pause` and
`resume` (which read only the forge fields, to find `<state>`). The first
failure is reported and the verb exits 1; the message is
`Error: swarm.json: <field>: <problem>` on stderr, `<field>` a dotted path:

| Problem | Message (after `Error: `) |
|---|---|
| file absent | `swarm.json: not found at <ws>/swarm/swarm.json; run 'daisy swarm init'` |
| not JSON, or a field of the wrong type | `swarm.json: <field>: <what encoding/json reports>` (a syntax error names the byte offset) |
| unknown field | `swarm.json: <field>: unknown field` |
| required field absent | `swarm.json: <field>: missing` |
| forge kind without an adapter | `swarm.json: forge.kind: "gitlab" has no adapter` |
| provider without an adapter | `swarm.json: personas.adversary.provider: "xai" has no adapter` |
| not one of the five roles | `swarm.json: personas.tester: not a role (owner, architect, worker, adversary, sympathetic)` |
| missing prompt | `swarm.json: personas.architect: prompt swarm/prompts/architect.md is missing or empty` |
| bad effort | `swarm.json: personas.worker.effort: "Very High" is not one lowercase word` |
| model with whitespace | `swarm.json: personas.worker.model: "a b" contains whitespace` |
| `repo` not a repository | `swarm.json: repo: "app" is not a git repository` |
| no `origin` | `swarm.json: repo: "app" has no origin remote` |
| credentials in `origin` | `swarm.json: repo: origin URL carries user information; use SSH or a credential helper` |
| `forge.repo` not `origin`'s | `swarm.json: forge.repo: "<value>" is not origin's <owner>/<repo>` |
| a number or duration out of range | `swarm.json: max_members: 0 is not between 1 and 16` (each with its own range) |
| a resource refused | `swarm.json: resources[1]: "." <reason>` (`is the workspace`, `does not exist`, `is outside the workspace` (also through a symbolic link), `contains .env.sh`, `contains .daisy`, `contains .git`, `is under swarm/`, `is listed twice`) |
| `base` not the default branch (`init` and `apply`'s Start only) | `swarm.json: base: "main" is not the forge's default branch "trunk"` |
| a provider without a key name | `swarm.json: keys: no variable for provider "openai"` |
| bad URL, repo or variable name | `swarm.json: forge.api: "<value>" is not an absolute http(s) URL without credentials`, `swarm.json: forge.repo: "<value>" is not <owner>/<repo>`, `swarm.json: forge.token_env: "<value>" is not a variable name` |

**Secret values.** A variable named by `forge.token_env` or `keys` is
resolved as `daisy check-secrets` resolves one: the assignment in
`<ws>/.env.sh` (read, never run), else the process environment (the
plan says `.env.sh`; the fallback lets a timer's `Environment=` supply a
value; section 14, item 8). The
example's names belong in `<ws>/.env.sh` (gitignored; the template is
`templates/env.sh.template`). A verb that needs a value and finds it empty
exits 1: `Error: swarm: $GITEA_TOKEN is not set (in <ws>/.env.sh or the
environment)`. `status` needs the token; `apply` needs the token and the
key of every provider a persona uses; `pause` and `resume` need neither.

## 4. Personas and prompts

| Role | Runs in | Receives (section 7, the prompt file) | Writes to `out.md` | Its output becomes |
|---|---|---|---|---|
| owner | no issue (the batch run); `swarm:proposed` | its prompt; for the batch, the open swarm issues by stage; on an issue, the issue and its thread | the batch: proposals, each `## <title>` and a body; on an issue: a reply | new issues labelled `swarm:proposed`; a comment |
| architect | `swarm:ready`, `swarm:planning` | its prompt, the issue and its thread | the whole plan | a comment |
| worker | `swarm:planned`, `swarm:reviewed` | its prompt, the issue and its thread; in `reviewed`, the pull request and its reviews | the pull request's description; in `reviewed`, what it changed; and commits on `swarm/<n>` in the clone | a pushed branch and a pull request (`Closes #<n>`); a comment |
| adversary | `swarm:in-review` | its prompt, the issue and its thread, the pull request and its reviews; the change is `git diff <base>...HEAD` in its read-only clone | `VERDICT: approve` or `VERDICT: request-changes`, then the review | a pull request review |
| sympathetic | `swarm:in-review` | as the adversary | as the adversary | a pull request review |

Every role may instead write `BLOCKED: <reason>` as its first line
(section 7.5). Comments in the thread carry their author's forge login and
time; a comment the swarm posted is shown as its role and run instead
(section 7.3). The maintainer talks to the swarm on the issue; comments on
the pull request are not read.

The default prompts follow. `swarm init` writes each to
`swarm/prompts/<role>.md` when absent. They state a persona's stance only:
what to do and where to write it is Daisy's fixed Task section of the
prompt file (section 7.4), so an edited persona prompt cannot break the
output contract.

### owner

```markdown
# Owner

You are the product owner of this repository. You decide what is worth
doing next, in what order, and why.

- Read the resources under /resources and the code under /work before you
  propose anything. Propose only what they support, and name the document
  or file that supports each proposal.
- A proposal is one job a single worker can finish in one pull request.
  Its title names the outcome. Its body says what is true when the job is
  done, why it is next, and what it depends on.
- Prefer the smallest job that moves the next milestone. Never propose
  what an open issue already covers.
- In a thread, answer the question asked. When the maintainer's comment
  changes the job, restate the whole job as it now stands, so that the
  thread's last word is all of it.
- You do not plan the implementation; the architect does.
```

### architect

```markdown
# Architect

You are the architect. You turn an agreed job into a plan a worker can
follow without asking.

- Read the code under /work and the resources under /resources before you
  plan. Follow the design decisions they record; where the job needs one
  that is not recorded, state it as a decision and give the alternative.
- The plan names the files to change, the approach, the tests that will
  show the job is done (and where they go), and the risks.
- Keep to the job as the thread states it. If the job is unclear or too
  large for one pull request, say so and propose how to cut it.
- When the maintainer replies, revise the plan and write it out whole; the
  worker reads only the latest plan.
- You do not write the code.
```

### worker

```markdown
# Worker

You are the worker. You implement the latest plan in the thread, on the
branch checked out in /work, and nothing else.

- Write the tests the plan names first, see them fail, then make them
  pass. Run the repository's own checks before you finish.
- Make small commits with messages that say why. Do not push; Daisy pushes.
- Change only what the plan needs. If the plan is wrong or cannot be
  followed, stop and say why instead of improvising.
- When reviews request changes, address each point or say why you did
  not; do not rewrite what the reviewers did not question.
- The description you write is read by the reviewers and the maintainer:
  what changed, how it was tested, and anything left undone.
```

### adversary

```markdown
# Adversary

You are the adversarial reviewer. Your job is to find what is wrong with
this change before it is merged.

- Read the whole diff and the code around it. Check it against the plan
  in the thread and the job in the issue.
- Look for defects: wrong behaviour, a missed case, a test that cannot
  fail, a claim in the description the change does not support, a pattern
  fixed in one place and not its siblings.
- Report only what you can point at: the file and line, what is wrong,
  and how to show it. A preference is not a finding.
- Request changes when any finding is real; approve when none is.
```

### sympathetic

```markdown
# Sympathetic reviewer

You are the sympathetic reviewer. You read the change as its future
maintainer will, assuming it is meant to work.

- Read the whole diff and the code around it, against the plan and the job.
- Judge whether the change does what the job asked, whether it fits the
  code around it, and whether the next person can read and change it.
- Name what is good as well as what is missing, briefly. Point at files
  and lines.
- Request changes when the change does not do the job or will mislead its
  next reader; approve otherwise.
```

## 5. Lifecycle

An issue's stage is its `swarm:*` label; `swarm init` creates the seven
labels and an open issue carries at most one. An issue with two is skipped
by `apply` with a warning until the maintainer removes one (it is
reachable only by a relabel interrupted between its two calls, section 10).

| Label | Meaning | Role that runs (section 6) |
|---|---|---|
| `swarm:proposed` | a job the owner proposed, awaiting triage | owner, to answer the thread |
| `swarm:ready` | triaged: plan it | architect |
| `swarm:planning` | a plan is in the thread, under discussion | architect, to answer the thread |
| `swarm:planned` | the plan is accepted: build it | worker |
| `swarm:in-review` | a pull request awaits both reviews of its head | adversary, then sympathetic |
| `swarm:reviewed` | both reviews of the head are in | worker, when one requested changes |
| `swarm:blocked` | waiting for the maintainer; the reason is the last comment | nobody |

**Transitions.** Every change of an issue's stage, with the one condition
that causes it (the trigger) and the one party that sets the new label
(the setter). "Daisy" is `swarm apply` collecting the run named in the
trigger (section 7.6), with the calls named there. The maintainer's rows
launch nothing: their trigger is the maintainer's decision.

| # | From | To | Trigger | Setter |
|---|---|---|---|---|
| T1 | (no issue) | `swarm:proposed` | the owner's batch trigger fires (section 6) and its run proposes at least one job | Daisy: `CreateIssue(title, body, ["swarm:proposed"])` per proposal |
| T2 | `swarm:proposed` | `swarm:ready` | the maintainer accepts the job | the maintainer, on the forge |
| T3 | `swarm:ready` | `swarm:planning` | the role rule launches the architect on a `ready` issue, and its run succeeds | Daisy: `Relabel(n, "swarm:ready", "swarm:planning")` |
| T4 | `swarm:planning` | `swarm:planned` | the maintainer accepts the plan | the maintainer, on the forge |
| T5 | `swarm:planned` | `swarm:in-review` | the role rule launches the worker on a `planned` issue, and its run succeeds (commits pushed, pull request open) | Daisy: `Relabel(n, "swarm:planned", "swarm:in-review")` |
| T6 | `swarm:in-review` | `swarm:reviewed` | a reviewer's run succeeds and its review completes the pair on the head, with both approving or the rounds below `max_rounds` | Daisy: `Relabel(n, "swarm:in-review", "swarm:reviewed")` |
| T7 | `swarm:in-review` | `swarm:blocked` | a reviewer's run succeeds and its review completes the pair on the head, with a request for changes and the rounds at `max_rounds` or more | Daisy: `Comment(n, <reconciler note>)`, then `Relabel(n, "swarm:in-review", "swarm:blocked")` |
| T8 | `swarm:reviewed` | `swarm:in-review` | the role rule launches the worker on a `reviewed` issue, and its run succeeds (fixes pushed) | Daisy: `Relabel(n, "swarm:reviewed", "swarm:in-review")` |
| T9 | any label with a role (all but `swarm:blocked`) | `swarm:blocked` | a run on the issue ends in a failure whose forge column in 7.7 is T9, or writes `BLOCKED:` | Daisy: `Comment`, then `Relabel(n, <label>, "swarm:blocked")` |
| T10 | `swarm:blocked` | any `swarm:*` label | the maintainer resumes the issue, at the stage of their choosing | the maintainer, on the forge |
| T11 | open, any label | closed | the maintainer merges a pull request whose body says `Closes #<n>` | the forge, on the merge |
| T12 | open, any label | closed | the maintainer abandons the job | the maintainer, on the forge |

The maintainer may also relabel any issue to any stage at any time; the
next pass reads the new label and the role rule decides from there. Daisy
never removes a label it did not find (a relabel whose `from` label is
gone by collection time is skipped, section 7.6) and never closes,
reopens or merges anything.

**Runs that do not move the label.** The owner on a `proposed` issue and
the architect on a `planning` issue post a comment and leave the label;
the first reviewer of a pair posts its review and leaves `in-review`; an
issue `reviewed` with both approvals runs nobody and waits for the merge.

## 6. The trigger rule and the marker

**The marker.** Every issue body, comment, pull request body and review
Daisy posts begins with two lines:

```
<!-- daisy-swarm role=<role> run=<k> -->
**<role>:**
```

then a blank line and the text. `<role>` is one of the five roles, or
`reconciler` for Daisy's own notes (a failure, the round limit), which carry
the number of the run they report on. A text is the swarm's exactly when
its first line, without a trailing `\r`, matches
`^<!-- daisy-swarm role=(owner|architect|worker|adversary|sympathetic|reconciler) run=([1-9][0-9]*) -->$`.
Authorship is never read from the forge's author field: every post uses
the one token, so it bears the maintainer's account. The marker is the
only test, and a table test asserts it (Step 28).

`<k>`, a run's number, counts per issue and role: one more than the
largest among `<state>/runs/<n>-<role>-*` and the markers `role=<role>` on
the issue and its pull request. The owner's batch runs are issue 0.

**The rule.** One rule decides every launch. Role R runs on open issue n
when all of these hold:

1. n carries exactly one `swarm:*` label, and R is that label's role
   (section 5);
2. no container labelled `daisy.swarm=<owner>/<repo>#<n>/…` is running,
   whatever its role, and no run on n has an end line without `collected`
   (a run whose posting failed is retried, never run again);
3. R has something to answer: the newest entry of the thread R reads is
   not R's own (table below);
4. the swarm is not paused, and fewer than `max_members` containers
   labelled for `<owner>/<repo>` are running.

| Label | R | The thread R reads | R has something to answer when |
|---|---|---|---|
| `swarm:proposed` | owner | the issue body, then its comments, oldest first | the newest is not marked `role=owner` |
| `swarm:ready` | architect | the same | the newest is not marked `role=architect` |
| `swarm:planning` | architect | the same | the newest is not marked `role=architect` |
| `swarm:planned` | worker | the same | the newest is not marked `role=worker` |
| `swarm:in-review` | adversary, then sympathetic | the swarm's reviews of the open pull request that closes n and whose head branch is `swarm/<n>` (any other is ignored) | R has no review marked `role=R` on the pull request's head commit |
| `swarm:reviewed` | worker | the same | the pair on the head holds a `request-changes` and the head has not moved since |

So the human speaking last wakes a member, and so does a stage the
maintainer sets: in `ready` the newest entry is the owner's or the
maintainer's, never the architect's, so the architect runs without a
comment. A run that fails leaves its role's words out of the thread (the
note is `role=reconciler`), so relabelling a blocked issue runs the role again.
Within a pass, an issue's candidate roles are taken in the table's order
and at most one launches, so the adversary reviews a head before the
sympathetic reviewer does. An issue in a review stage with no open pull
request from `swarm/<n>` that closes it is skipped with a warning.

A **round** is a head commit both reviewers have reviewed; the rounds of a
pull request are the number of distinct commits with a review marked
`role=adversary` and one marked `role=sympathetic`.

**The owner's batch trigger.** The owner has one more trigger, having no
issue to wait on: it runs as issue 0 when no open issue carries
`swarm:proposed`, no container labelled `#0/owner` is running, no
batch run (`0-owner-*`) has an end line without `collected`,
`<state>/owner-held` is absent, and rule 4 holds. It proposes at most
`owner.max_open_proposals` jobs, in its priority order, each body saying
why it is next. A batch run that fails, writes `BLOCKED:` or proposes
nothing writes `<state>/owner-held` with the reason and creates nothing;
the trigger then waits for `swarm resume`, so a backlog with nothing left
to propose does not relaunch the owner every pass. The maintainer's triage
of the open proposals is what wakes the next batch.

## 7. Members

A member is one role run: one container, started by `apply`, that reads a
prompt file and a clone and writes one output file. It holds no forge
token, no git credentials and no state beyond the run.

### 7.1 Preparing a run

On the host, in order, before `docker run`:

1. The mirror: `<state>/git` is created by `git init --bare` when absent,
   and brought up to date once per pass before the pass's first launch:
   `git -C <state>/git fetch --prune <origin-url> '+refs/heads/*:refs/heads/*'`.
   `<origin-url>` is the product repository's `remote.origin.url`; when the
   product repository sets `core.sshCommand` (an identity's key), every
   host git command that reaches the forge passes it as
   `-c core.sshCommand=<value>`. These are the host's credentials; they are
   used only by host-side git.
2. `<run>` and `<run>/io` are created, and the prompt file written to
   `<run>/io/prompt.md` (7.3).
3. The clone: `git clone --no-local --quiet --branch <branch> <state>/git <run>/work`,
   `<branch>` being `swarm/<n>` when the mirror has it, else `<base>`
   (the owner's batch run: `<base>`). The worker on a `planned` issue with
   no `swarm/<n>` then runs `git -C <run>/work checkout -b swarm/<n>`. Then,
   before any container sees the clone: a local `<base>` branch is made
   when the clone has none (`git -C <run>/work branch <base> origin/<base>`),
   so `git diff <base>...HEAD` works; `git -C <run>/work remote remove origin`;
   `user.name` `<role> (Daisy swarm)` and `user.email`
   `<role>@daisy-swarm.invalid` are set in the clone's config;
   `/.swarm/` is appended to `<run>/work/.git/info/exclude`; and
   `<run>/work/.swarm` is created as the mount point. The head commit is
   recorded in `<run>/result` as `start <sha>`.
4. `<run>/argv` is written and `started <time>` appended to `<run>/result`.

After the container starts, host-side git never runs inside
`<run>/work`: the member controls its `.git` (hooks, config), so the only
git command that reads it is a `fetch` from it into the mirror (7.6), and
the directory is then deleted.

### 7.2 The container contract

```
docker run --rm --pull never
  --name daisy-swarm.<owner>.<repo>.<n>.<role>.<k>
  --label daisy.swarm=<owner>/<repo>#<n>/<role>
  --user <uid>:<gid>
  --stop-timeout 30
  --env-file /dev/stdin
  --mount type=bind,src=<run>/work,dst=/work[,readonly]
  --mount type=bind,src=<run>/io,dst=/work/.swarm
  --mount type=bind,src=<resolved resource>,dst=/resources/<resource>,readonly
  --workdir /work
  <image>
  <the provider's argv (section 9)>
```

| Argument | Rule |
|---|---|
| `--rm` | no container, and so no volume, survives the run |
| `--pull never` | the image is local; a missing image fails the run (exit 125) instead of pulling a public image of the same name |
| `--name` | unique per run; `apply` stops the container by it at the wall clock |
| `--label` | the running set: `docker ps --filter label=daisy.swarm` is what rule 2, `max_members` and `status` read |
| `--user` | the host user's uid and gid (`id -u`, `id -g`, read once per `apply`), so the bind-mounted files stay the host user's and the member is never root |
| `--stop-timeout 30` | the grace between `docker stop`'s signal and its kill; the wall clock itself is `timeout`, enforced by `apply` (below) |
| `--env-file /dev/stdin` | the one environment variable, read by the docker CLI from its standard input, which `apply` writes as the single line `<VAR>=<key>` and closes; `<VAR>` is the provider's container variable (section 9). The key is never on an argv nor in any file Daisy writes; Docker itself keeps it in the container's configuration (`docker inspect`) for the container's life, which `--rm` ends (*to confirm at Step 28*) |
| `/work` | the clone, read-write; `,readonly` for the adversary and the sympathetic reviewer |
| `/work/.swarm` | `<run>/io`, read-write for every role: the prompt file in, `out.md` out |
| `/resources/<resource>` | one read-only mount per `resources` entry, its source the path resolved at this pass's validation (section 3), never `<ws>/<resource>` as written; none when the list is empty |
| `--workdir /work` | the CLIs start in the clone |

Nothing else is mounted, passed or published: no Docker socket, no home
directory, no `<ws>` beyond the resources, no `.env.sh`, no SSH agent, no
port. The container's network is Docker's default bridge (the CLIs reach
their APIs; egress filtering is deferred).

**The wall clock.** `apply` starts the container with `Ops.Start` and a
timer of `timeout`; when it fires, `apply` runs
`docker stop --time 30 daisy-swarm.<owner>.<repo>.<n>.<role>.<k>`
and the run is a timeout whatever it exits with. The container's standard
output and error are copied to `<run>/stdout` and `<run>/stderr` when the
run ends.

### 7.3 The prompt file

`<run>/io/prompt.md`, seen by the member as `/work/.swarm/prompt.md`.
Times are RFC 3339 in UTC; a swarm text appears without its two marker
lines, under its role and run.

```markdown
<!-- daisy-swarm prompt: <owner>/<repo>#<n> <role> run <k>, <time> -->

# Persona

<swarm/prompts/<role>.md, verbatim>

# Task

<the Task text for the role and label, 7.4>

# Repository

- Forge repository: <owner>/<repo> on <forge-host>
- Clone: /work, branch <branch>, base <base>[, read-only]
- Resources (read-only): /resources/<path>, … (or: none)

# Issue #<n>: <title>

Opened by <login> at <time>. Labels: <labels, comma-separated>.

<body>

# Thread

## <login> at <time>

<comment>

## <role>, run <k>, at <time>

<swarm comment>

# Pull request #<m>: <title>

Branch swarm/<n> into <base>; head <sha>.

<body>

## Reviews

### <role>, run <k>, at <time>, on <sha>: <approve|request-changes>

<review>
```

`# Thread` holds the comments oldest first (`None.` when there are none).
`# Pull request` is present for the reviewers and for the worker on a
`reviewed` issue, or on a `planned` issue that already has a pull request;
`## Reviews` lists every review on it, oldest first (a review that is not
the swarm's shows its login and the forge's state). The owner's batch
prompt has no issue, thread or pull request; in their place:

```markdown
# Open swarm issues

- #<n> [<stage>] <title>
```

listing every open issue with a `swarm:*` label (`None.` when there are none).

### 7.4 The Task texts

Daisy's fixed text, by role and label; `<…>` filled in. Every text ends
with the same paragraph:

> Write your output to /work/.swarm/out.md; that file is the only thing
> read back, and nothing you write elsewhere outside /work survives. If you
> cannot do this without an answer from the maintainer, write
> `BLOCKED: <your question>` as the first line of /work/.swarm/out.md
> instead.

| Role, label | Task text (before the closing paragraph) |
|---|---|
| owner, batch (issue 0) | `Propose up to <max_open_proposals> new jobs for <owner>/<repo>, most important first. Start each with a line "## <title>" and follow it with the job's body. Write nothing before the first "## " line. If there is nothing worth proposing, write "BLOCKED: <why>" as the only line.` |
| owner, `swarm:proposed` | `Answer the latest comment in the thread of issue #<n>.` |
| architect, `swarm:ready` | `Write the plan for issue #<n>: the files to change, the approach, the tests that will show it works, and the risks.` |
| architect, `swarm:planning` | `Revise your plan for issue #<n> in answer to the latest comment. Write the whole revised plan, not the changes to it.` |
| worker, `swarm:planned` | `Implement the latest plan in the thread of issue #<n> on the branch checked out in /work (swarm/<n>). Commit with git; do not push. Write the pull request's description.` |
| worker, `swarm:reviewed` | `The latest reviews of pull request #<m> request changes. Address them on the branch checked out in /work; commit with git, do not push. Write what you changed, point by point.` |
| adversary or sympathetic, `swarm:in-review` | `Review pull request #<m> for issue #<n>. The change is "git diff <base>...HEAD" in /work, which is read-only. The first line of your output must be exactly "VERDICT: approve" or "VERDICT: request-changes", followed by your review.` |

### 7.5 The output file

`apply` reads `<run>/io/out.md` (the member's `/work/.swarm/out.md`) once
the container has exited. Its first line is read without a trailing `\r`
or trailing spaces.

| The file | Means |
|---|---|
| absent, or only whitespace | the failure *no output* |
| first line begins `BLOCKED:` | the member asks the maintainer: posted as the role's comment, then T9 (the batch run: `owner-held`, reason the rest of the line) |
| a reviewer's, first line exactly `VERDICT: approve` or `VERDICT: request-changes` | the verdict; the whole file is the review's text, verdict line included |
| a reviewer's, any other first line | the failure *bad verdict line* |
| the batch run's | proposals: each line matching `^## (.+)$` starts one, its title the match trimmed (1 to 200 characters), its body the lines up to the next such line, blank lines trimmed at both ends; text before the first is ignored; the first `max_open_proposals` are taken; none is *proposed nothing* |
| anything else | the text to post |

A text longer than 60 000 bytes is posted cut at the last line break
before that length, followed by
`[truncated by Daisy; the whole output is in runs/<n>-<role>-<k>]`.

### 7.6 Collecting a run

A run is collected when its container has exited. Each step first checks
the forge and is skipped when already done there (a comment or review
marked with this role and run exists; an open pull request whose body
carries this run's marker exists; the label already moved; for the
batch run, a `creating <i>` line for proposal i is in `<run>/result`,
written before its `CreateIssue`, so a proposal is created at most once
even when the reply was lost), so a collection interrupted by a forge or network error is
retried whole at the next pass without posting twice. Each step done is
appended to `<run>/result`; `collected` ends it, and `<run>/work` is then
removed. Before any step, `Issue(n)` is read again: an issue closed
meanwhile gets nothing posted (`#<n> closed during the run; nothing
posted`, the output kept in `<run>`, `collected` written), and a `Relabel` whose `from` label
the issue no longer carries is skipped with a warning (the maintainer
moved it during the run). A reviewer's run whose pull request is gone, or
whose head is no longer the run's `start` commit, posts nothing: `stale`
is recorded and the role rule relaunches at k+1 on the new head.

| Run | Steps, in order |
|---|---|
| owner, batch | `CreateIssue(<title>, <marked body>, ["swarm:proposed"])` for each proposal, in order (T1) |
| owner, `proposed` | `Comment(n, <marked text>)` |
| architect, `ready` | `Comment(n, <marked text>)`; `Relabel(n, "swarm:ready", "swarm:planning")` (T3) |
| architect, `planning` | `Comment(n, <marked text>)` |
| worker, `planned` | `git -C <state>/git fetch --no-tags <run>/work '+refs/heads/swarm/<n>:refs/heads/swarm/<n>'`; the fetched commit equal to `start` is the failure *no commits*; `git -C <state>/git push <origin-url> refs/heads/swarm/<n>:refs/heads/swarm/<n>` (never forced; a rejection is the failure *push rejected*); `PullForIssue(n)`: none, `CreatePull("swarm/<n>", <base>, <issue title>, <marked text> + "\n\nCloses #<n>")`; one already open, `Comment(n, <marked text>)` (both skipped when its body carries this run's marker); `Relabel(n, "swarm:planned", "swarm:in-review")` (T5) |
| worker, `reviewed` | fetch, the *no commits* check and push as above; `Comment(n, <marked text>)`; `Relabel(n, "swarm:reviewed", "swarm:in-review")` (T8) |
| adversary or sympathetic, `in-review` | `PullForIssue(n)`; `Review(m, <verdict>, <marked text>)`; `Reviews(m)`: when both roles have a review on the head, both approving, or the rounds below `max_rounds`, `Relabel(n, "swarm:in-review", "swarm:reviewed")` (T6); else `Comment(n, <reconciler note: review round <r> of <max_rounds> requested changes>)` and `Relabel(n, "swarm:in-review", "swarm:blocked")` (T7) |
| any role, `BLOCKED:` | `Comment(n, <marked text>)`; `Relabel(n, <label>, "swarm:blocked")` (T9) |

`<marked text>` is the marker for the role and run, then the output. A
reconciler note is `<!-- daisy-swarm role=reconciler run=<k> -->`, `**reconciler:**`, a
blank line, and one sentence: `<role> run <k> failed: <reason>. Relabel
the issue to resume.` or the round-limit sentence; it never quotes the
member's streams.

### 7.7 Failure paths

| Failure | Detected by | `<run>/result` | On the forge |
|---|---|---|---|
| timeout | the wall clock fires (`docker stop` sent) | `timeout <time>`, `failed: timeout after <timeout>` | T9: reconciler note, `Relabel(n, <label>, "swarm:blocked")` |
| launch fails | `Ops.Start` returns an error (no `docker`, no socket) | `failed: launch: <error>` | T9 |
| non-zero exit | the exit status of `docker run` (125–127: Docker's own, such as a missing image) | `exited <code> <time>`, `failed: exit <code>` | T9 |
| no output | 7.5 | `failed: no output` | T9 |
| bad verdict line | 7.5 | `failed: bad verdict line` | T9 |
| no commits (worker) | 7.6 | `failed: no commits` | T9 |
| push rejected (worker) | 7.6 | `failed: push rejected` | T9 |
| abandoned | a pass finds a run not its own with `started`, no end line and no container by the run's name (the `apply` that started it died) | `abandoned <time>`, `collected` | none: the role rule relaunches at run k+1 |
| orphan over time | a pass finds a container by the run's name, the run not its own, `started` more than `timeout` ago | `docker stop --time 30 <name>` sent; then abandoned as above | none |
| stale (reviewer) | 7.6: the pull request is gone or its head moved during the run | `stale <sha>` and `collected` | none: relaunched on the new head |
| the batch run, any of the above, or *proposed nothing* | as above | as above | none: `<state>/owner-held` written |
| a forge or git error while collecting | 7.6 | the steps done so far; no end line | none further this pass; retried at the next (`--once`: exit 1) |

## 8. The credential rule

1. **One secret per member.** A container receives exactly one secret, its
   provider's key, as one environment variable (`ANTHROPIC_API_KEY` or
   `OPENAI_API_KEY`, section 9), through `--env-file /dev/stdin` (7.2). No
   other variable is passed.
2. **The forge token never enters a container.** It is read by `apply`,
   `status` and `init` into memory, sent only in the `Authorization` header
   of `Ops.HTTP` requests to `forge.api`, and never written to a file, an
   argv, a prompt, a comment or an error message (an adapter error names
   the method, endpoint and status, never the request's headers).
3. **The host's git credentials never enter a container.** Fetch and push
   run on the host, in `<state>/git`, with the host's SSH keys, agent or
   credential helper and the product repository's `core.sshCommand`. The
   clone a member sees has no remote (7.1), so no URL reaches it; user
   information in an `http(s)` `origin` URL is refused at validation
   (section 3), and an SSH URL's password likewise, so no credential can
   reach a clone's reflog or config either.
4. **Names, not values.** `swarm.json` names the variables
   (`forge.token_env`, `keys`); the values live in `<ws>/.env.sh`
   (gitignored) or the environment. A resource may not contain `.env.sh`,
   `.daisy` or `.git` (section 3), so no mount carries them.
5. **What `init` checks about the token:** that its variable is set; that
   it can read the issues repository (`Issues("swarm:proposed")` succeeds);
   and, by `EnsureLabels`, that it can write labels. It cannot check the
   token's other write permissions without writing; the first `apply`
   that posts finds a missing one as a forge error.
6. **The record holds no secret Daisy put there.** `<run>/argv` holds the argv, which holds
   no value; `<run>/io/prompt.md` holds the thread; `<run>/stdout` and
   `<run>/stderr` hold what the CLI printed, which Daisy does not post.
7. **What the rule does not cover.** A member holds its own key, and any
   comment on the issue reaches its prompt (authorship is the marker, not
   the login), so a comment written to make a member print its key could
   get it into `out.md`, which is posted, or into `<run>/stdout`. Version 1
   accepts this for a repository whose issues only the maintainer and
   trusted users can comment on; filtering comments by author is deferred.

## 9. The provider seam

```go
// internal/swarm/provider
type Adapter interface {
    // Command is the argv run inside the container for one persona,
    // the prompt file being at promptPath (/work/.swarm/prompt.md).
    Command(persona declare.Persona, promptPath string) []string
}

// The adapter table: name → adapter, and the variable its CLI reads its
// key from inside the container.
var Providers = map[string]struct {
    Adapter Adapter
    KeyVar  string
}{ "anthropic": …, "openai": … }
```

| Provider | `KeyVar` | `Command(persona, P)` |
|---|---|---|
| `anthropic` | `ANTHROPIC_API_KEY` | `claude -p --model <M> --effort <E> --system-prompt-file <P> --permission-mode bypassPermissions --output-format json "Do the task in your system prompt. Write your output to /work/.swarm/out.md."` |
| `openai` | `OPENAI_API_KEY` | `codex exec --model <M> -c model_reasoning_effort=<E> --dangerously-bypass-approvals-and-sandbox --cd /work "Read <P> and do the task it describes. Write your output to /work/.swarm/out.md."` |

The `anthropic` row is the plan's argv with the one positional prompt
`claude -p` needs; the `openai` row is the plan's "`codex exec` with its
model flag" made whole. `<M>` and `<E>` are the persona's `model` and
`effort`, passed through unvalidated (section 3's syntax rules aside); a
model of `""` drops `--model <M>`. Both rows are *to confirm at Step 28*
against the CLIs the image installs (the flag spellings, `--effort`'s
existence in print mode, and whether `codex exec` reads `OPENAI_API_KEY`
or `CODEX_API_KEY`); a correction there changes this table, not the seam.
The container is the sandbox, which is why both CLIs run with their own
permission checks off. `xai` has no row: a persona naming it fails
validation (section 3).

## 10. The forge seam

```go
// internal/swarm/forge, over Ops.HTTP()
type Adapter interface {
    EnsureLabels(names []string) error
    Issues(label string) ([]Issue, error)            // open issues carrying the label
    Issue(n int) (Issue, error)                      // with comments, oldest first
    CreateIssue(title, body string, labels []string) (int, error)
    Comment(n int, body string) error
    Relabel(n int, remove, add string) error
    PullForIssue(n int) (*Pull, error)               // the open PR whose body closes n
    CreatePull(head, base, title, body string) (int, error)
    Reviews(pr int) ([]Review, error)
    Review(pr int, verdict Verdict, body string) error
}

type Issue struct {
    Number   int
    Title    string
    Body     string
    Author   string    // login
    Created  time.Time
    Open     bool
    Labels   []string
    Comments []Comment // filled by Issue(n) only
}
type Comment struct{ Author, Body string; Created time.Time }
type Pull struct{ Number int; Title, Body, Head, HeadSHA, Base string }
type Review struct {
    Author    string
    Body      string
    State     Verdict   // the forge's state; the swarm's reviews are Commented (below)
    Commit    string    // the head the review was made on
    Submitted time.Time
}
type Verdict int // Approve, RequestChanges, Commented
```

These ten methods are every call Daisy makes to the forge; the only other
traffic with it is host-side git (fetch and push, 7.1 and 7.6).

**Semantics common to both adapters.** `EnsureLabels` lists the labels and
creates each missing name with colour `#ededed` and no description; it
never edits or deletes one, so a second call creates nothing. `Issues`
excludes pull requests and is sorted by number. `PullForIssue` reads the
open pull requests and returns the one whose head branch (`head.ref`) is
`swarm/<n>` and whose body has a line equal to `Closes #<n>` (any case),
or nil; a pull request on another branch is never the swarm's. `Relabel` adds `add` and then
removes `remove` (an interruption between them leaves two labels, which
`apply` reports and skips, section 5); `add` is one of the seven labels `init`
ensured (GitHub's add call would create a missing one; Gitea's needs its id).
**`Review` always posts a comment-type review**: one token both opens and
reviews the pull request, and both forges refuse an approval or a request
for changes on one's own pull request, so the verdict travels in the
body's first line after the marker (`VERDICT: …`), which is where
`reconcile` reads the swarm's verdicts; the `verdict` argument is kept for
an adapter with one account per persona (deferred). A list call reads
every page (until a page is shorter than the limit). A `429` or `5xx`
response is retried after its `Retry-After` (else 10 s), at most three
times. An error reads `forge: <METHOD> <path>: <status> <message from the
body>`.

**Detection** (the wizard's defaults): from the product repository's
`remote.origin.url` in any of the forms `git@<host>:<owner>/<repo>.git`,
`ssh://[<user>@]<host>[:<port>]/<owner>/<repo>[.git]` and
`https://<host>[:<port>]/<owner>/<repo>[.git]`: host `github.com` gives
kind `github`, API `https://api.github.com`; any other host gives kind
`gitea`, API `https://<host>/api/v1` (an SSH port is not the web port and
is dropped). **`<forge-host>`**, the first part of `<state>`, is
`github.com` for kind `github`, else `forge.api`'s host (with `:<port>`
when it has one).

### Gitea (API v1)

Header `Authorization: token <token>`; `<base>` is `forge.api`; `{o}/{r}`
is `forge.repo`. Label endpoints take ids: the adapter lists the labels
(`GET /repos/{o}/{r}/labels`) to map names to ids before `CreateIssue` and
`Relabel`.

| Method | HTTP | Endpoint | Request body | Response fields read |
|---|---|---|---|---|
| `EnsureLabels` | `GET`, then `POST` per missing name | `/repos/{o}/{r}/labels?page=<p>&limit=50`; `/repos/{o}/{r}/labels` | —; `{"name", "color": "#ededed", "description": ""}` | `[].id`, `[].name`; — |
| `Issues` | `GET` | `/repos/{o}/{r}/issues?state=open&type=issues&labels=<label>&page=<p>&limit=50` | — | `[].number`, `title`, `body`, `user.login`, `created_at`, `state`, `labels[].name` |
| `Issue` | `GET`, `GET` | `/repos/{o}/{r}/issues/{n}`; `/repos/{o}/{r}/issues/{n}/comments?page=<p>&limit=50` | — | as `Issues`; `[].body`, `[].user.login`, `[].created_at` |
| `CreateIssue` | `POST` | `/repos/{o}/{r}/issues` | `{"title", "body", "labels": [<ids>]}` | `number` |
| `Comment` | `POST` | `/repos/{o}/{r}/issues/{n}/comments` | `{"body"}` | — |
| `Relabel` | `POST`, then `DELETE` | `/repos/{o}/{r}/issues/{n}/labels`; `/repos/{o}/{r}/issues/{n}/labels/{id}` | `{"labels": [<add id>]}`; — | — |
| `PullForIssue` | `GET` | `/repos/{o}/{r}/pulls?state=open&page=<p>&limit=50` | — | `[].number`, `title`, `body`, `head.ref`, `head.sha`, `base.ref` |
| `CreatePull` | `POST` | `/repos/{o}/{r}/pulls` | `{"head", "base", "title", "body"}` | `number` |
| `Reviews` | `GET` | `/repos/{o}/{r}/pulls/{pr}/reviews?page=<p>&limit=50` | — | `[].body`, `[].state` (`APPROVED`, `REQUEST_CHANGES`, `COMMENT`), `[].commit_id`, `[].user.login`, `[].submitted_at` |
| `Review` | `POST` | `/repos/{o}/{r}/pulls/{pr}/reviews` | `{"event": "COMMENT", "body"}` | — |

*To confirm at Step 27* against the instance's `/api/swagger`: the
`labels` filter taking names, the comments endpoint's paging, the review
`event` spelling, and the refusal of a self-approval.

### GitHub (REST v3)

Headers `Authorization: Bearer <token>`, `Accept: application/vnd.github+json`,
`X-GitHub-Api-Version: 2022-11-28`; `<base>` is `https://api.github.com`.
Labels are named in paths and bodies (a name in a path is escaped:
`swarm%3Aready`).

| Method | HTTP | Endpoint | Request body | Response fields read |
|---|---|---|---|---|
| `EnsureLabels` | `GET`, then `POST` per missing name | `/repos/{o}/{r}/labels?per_page=100&page=<p>`; `/repos/{o}/{r}/labels` | —; `{"name", "color": "ededed", "description": ""}` | `[].name`; — |
| `Issues` | `GET` | `/repos/{o}/{r}/issues?state=open&labels=<label>&per_page=100&page=<p>` | — | `[].number`, `title`, `body`, `user.login`, `created_at`, `state`, `labels[].name`; an item with `pull_request` is dropped |
| `Issue` | `GET`, `GET` | `/repos/{o}/{r}/issues/{n}`; `/repos/{o}/{r}/issues/{n}/comments?per_page=100&page=<p>` | — | as `Issues`; `[].body`, `[].user.login`, `[].created_at` |
| `CreateIssue` | `POST` | `/repos/{o}/{r}/issues` | `{"title", "body", "labels": [<names>]}` | `number` |
| `Comment` | `POST` | `/repos/{o}/{r}/issues/{n}/comments` | `{"body"}` | — |
| `Relabel` | `POST`, then `DELETE` | `/repos/{o}/{r}/issues/{n}/labels`; `/repos/{o}/{r}/issues/{n}/labels/{name}` | `{"labels": [<add>]}`; — | — |
| `PullForIssue` | `GET` | `/repos/{o}/{r}/pulls?state=open&per_page=100&page=<p>` | — | `[].number`, `title`, `body`, `head.ref`, `head.sha`, `base.ref` |
| `CreatePull` | `POST` | `/repos/{o}/{r}/pulls` | `{"title", "head", "base", "body"}` | `number` |
| `Reviews` | `GET` | `/repos/{o}/{r}/pulls/{pr}/reviews?per_page=100&page=<p>` | — | `[].body`, `[].state` (`APPROVED`, `CHANGES_REQUESTED`, `COMMENTED`), `[].commit_id`, `[].user.login`, `[].submitted_at` |
| `Review` | `POST` | `/repos/{o}/{r}/pulls/{pr}/reviews` | `{"event": "COMMENT", "body"}` | — |

`Relabel`'s `POST` adds without replacing (the `PUT` form would replace
every label). *To confirm at Step 27* against GitHub's REST reference: the
recorded pair per method.

### `forge.Fake`

An in-memory forge implementing `Adapter` (a compile-time
`var _ Adapter = (*Fake)(nil)` for both real adapters and the Fake): labels,
issues, comments, pull requests and reviews in maps; numbers from one
counter shared by issues and pull requests, as on both forges; times from
the `Ops` clock it is built with. It applies the same semantics as the real
adapters (the label must exist for `Relabel`, `Review` stores a commented
review on the pull request's current head, `Issues` excludes pull
requests), and has helpers the real forge does not need: `Say(n, login,
body)` (the maintainer comments), `SetLabel(n, label)`, `Push(pr, sha)`
(moves a head), `Merge(pr)` (closes the pull request and the issue it
closes) and `Close(n)`. Tests assert on the Fake's state after a pass,
never on which methods were called.

## 11. The verbs

Every verb runs in a Daisy workspace (`<ws>`, found as every workspace
command finds it); outside one it prints `Not in a Daisy workspace` and
exits 1, as `status` does. Any argument a verb does not take is a usage
error: its usage line on stderr, exit 2. None commits: `swarm/` is the
workspace repository's, committed by the maintainer.

### daisy swarm init

| Field | Value |
|---|---|
| Usage | `daisy swarm init [--defaults]` |
| Reads | `<ws>/swarm/swarm.json` when present (its values are the defaults), the product repository's `origin` and the workspace tree (detection), `<ws>/.env.sh` (the token) |
| Asks | the wizard below, through `Ops.Prompt`, questions on stderr; `--defaults` asks nothing and takes every default |
| Forge | `Issues("swarm:proposed")` (the token can read the repository), then `EnsureLabels` with the seven labels |
| Writes | `swarm/swarm.json` when the answers differ from it; each `swarm/prompts/<role>.md` that is absent. Nothing under `$DAISY_LOCAL` |
| Order | answers; validation as section 3 (the prompt files excepted, which this run writes); the token set; the read check; the files; `EnsureLabels`; the unit files printed. A failure stops there, so a failed read check writes nothing |
| Stdout | `✓ Wrote swarm/swarm.json` or `  Unchanged swarm/swarm.json`; per role `✓ Wrote swarm/prompts/<role>.md` or `  Kept swarm/prompts/<role>.md (exists)`; `✓ Token $<VAR> reads <owner>/<repo>`; `✓ Labels: 7 present (<c> created)`; a blank line, then the two unit files and the install line (below) |
| Stderr | the questions; a refused answer's validation message (without `Error: `), then the same question again; `Cancelled — no answer on stdin (EOF); nothing written.` |
| Exit codes | 0 written (or unchanged), or cancelled at EOF; 1 outside a workspace, an invalid file or default (`Error: swarm.json: …`), the token unset, the read check failing (`Error: swarm: $<VAR> cannot read <owner>/<repo>: <forge error>`), a write failing, `EnsureLabels` failing (the files stay written; a re-run completes it); 2 usage |
| Ops | Real |
| Idempotence | `init --defaults` run twice: the second run writes no file and creates no label (Step 27's claim) |

The wizard, in order. An empty answer takes the default shown in
brackets; defaults come from the existing `swarm.json`, else detection.

| # | Question | Default without a `swarm.json` |
|---|---|---|
| 1 | `Product repository (path in the workspace) [<d>]: ` | `.` when `<ws>` is a git repository with `origin`; else the one directory directly under `<ws>` that is (several or none: `.`) |
| 2 | `Forge kind (gitea, github) [<d>]: ` | detected (section 10) |
| 3 | `Forge API [<d>]: ` | detected |
| 4 | `Issues repository [<d>]: ` | `<owner>/<repo>` of `origin` |
| 5 | `Token variable [<d>]: ` | `GITEA_TOKEN` or `GITHUB_TOKEN` |
| 6 | `Base branch [<d>]: ` | `main` |
| 7–21 | for each role in the order owner, architect, worker, adversary, sympathetic: `<role> provider (anthropic, openai) [<d>]: `, `<role> model (- for the CLI's default) [<d>]: `, `<role> effort [<d>]: ` | section 3's example |
| 22 | `Image [<d>]: ` | `daisy-member:1` |
| 23 | `Max members [<d>]: ` | 3 |
| 24 | `Max rounds [<d>]: ` | 3 |
| 25 | `Timeout [<d>]: ` | `45m` |
| 26 | `Interval [<d>]: ` | `5m` |
| 27 | `Resources (comma-separated workspace paths, - for none) [<d>]: ` | `docs/` when `<ws>/docs` is a directory, else none |

`owner.max_open_proposals` and `keys` are not asked: they keep the
existing file's values or section 3's defaults, and are edited in the file.

**The unit files.** `init` prints them for the maintainer to install;
`<daisy>` is the running binary's path (`Ops.Executable`), `<interval>`
the configured interval in systemd's notation (`5min`), and `<instance>`
the workspace path escaped as `systemd-escape --path` escapes it:

```
# ~/.config/systemd/user/daisy-swarm@.service
[Unit]
Description=Daisy swarm: one apply pass for the workspace %I

[Service]
Type=oneshot
WorkingDirectory=%I
ExecStart=<daisy> swarm apply --once

# ~/.config/systemd/user/daisy-swarm@.timer
[Unit]
Description=Daisy swarm: apply every <interval> for the workspace %I

[Timer]
OnActiveSec=1min
OnUnitInactiveSec=<interval>

[Install]
WantedBy=timers.target

Install (your step):
  systemctl --user daemon-reload && systemctl --user enable --now daisy-swarm@<instance>.timer
```

A changed `interval` takes effect on the timer only when the unit is
printed and installed again. Under the timer, `apply` has no terminal: the
push needs credentials that work without one (an SSH key without a
passphrase prompt, or an agent the user manager can reach).

### daisy swarm apply

| Field | Value |
|---|---|
| Usage | `daisy swarm apply [--once]` |
| Reads | `swarm/swarm.json` and the prompts (each pass), `<ws>/.env.sh` (token and keys), the product repository's `origin` and `core.sshCommand`, `<state>` |
| Start | validation (with the `base` check of section 3); the token and every used provider's key set; `id -u` and `id -g`; `docker ps` once (failing: `Error: swarm: docker: <message>`, exit 1); `<state>` created when absent; the lock (below) |
| A pass | (1) re-read and validate `swarm.json` (without `--once`, a failure is printed and the pass skipped); (2) collect this process's finished runs, and retry every run with an end line but no `collected` (7.6); (3) the running set: `docker ps --filter label=daisy.swarm --format '{{.Names}} {{.Label "daisy.swarm"}}'`, keeping this repository's; of the runs this process did not start, one with `started`, no end line and no container is marked `abandoned` (then `collected`) and its `work/` removed, and one whose container has outlived `timeout` is stopped (7.7); a `docker ps` failure here ends the pass without launching; (4) when `paused` exists, print it and launch nothing; (5) for each label in section 5's order, `Issues(label)`, and for each issue by number `Issue(n)` (and `PullForIssue(n)` in `planned` and the review stages, `Reviews(m)` in the review stages); a forge error here ends the pass's launching, with `Error: …` on stderr: the role rule (section 6) decides, and launches run until `max_members` are running; the mirror is fetched before the pass's first launch; (6) the owner's batch trigger; (7) the summary line |
| `--once` | one pass; then it waits for every run it launched, collects each, and exits |
| Without `--once` | a pass every `interval`, start to start, until SIGINT or SIGTERM: then no launch, its running members collected as they exit, exit 0; a second signal exits at once (130), leaving its members to finish and be found abandoned by a later pass |
| Forge | reads: `Issues`, `Issue`, `PullForIssue`, `Reviews`; writes: `CreateIssue`, `Comment`, `Relabel`, `CreatePull`, `Review` (7.6); host git fetches from and pushes `swarm/<n>` to the forge |
| Writes | `<state>/lock`, `<state>/owner-held`, `<state>/git/`, `<state>/runs/` (section 2); nothing else |
| Stdout | `#<n> <role> run <k>: launched`; at collection one line per step done, `#<n> <role> run <k>: ` then `posted comment`, `posted review (<approve\|request-changes>)`, `pushed swarm/<n> <short sha>`, `created pull #<m>`, `<from> → <to>`; a failure `#<n> <role> run <k>: failed (<reason>)` and then its steps; the batch run `owner run <k>: launched`, `owner run <k>: proposed #<n> <title>` per issue, or `owner run <k>: proposed nothing; batch held (<state>/owner-held)`; `Paused since <time> (<state>/paused): launching nothing`; each pass ends `Pass <HH:MM:SS>: launched <a>, collected <c>, running <r>` |
| Stderr | warnings `⚠ #<n>: carries <label> and <label>; skipped`, `⚠ #<n>: <label> but no open pull request from swarm/<n> closes it; skipped`, `⚠ #<n>: relabelled to <label> during the run; not relabelled`, `⚠ #<n> closed during the run; nothing posted`; errors `Error: …` |
| Lock | `Ops.FS().Lock(<state>/lock, "apply[ --once] since <time>")` takes an exclusive `flock` without waiting and writes the holder line (adding `pid <pid>`); the kernel releases it when the process exits, however it exits. Held: `Error: swarm apply is already running for <owner>/<repo> (<holder line>); lock: <state>/lock`, exit 1, nothing read from the forge |
| Exit codes | 0 the pass or passes ran (a member's failure is reported on the forge, not in the exit code); 1 outside a workspace, invalid `swarm.json` at start, a token or key unset, `docker` unavailable (`Error: swarm: docker: <message>`), the lock held, and with `--once` a forge, git or `docker ps` error during the pass (without `--once` such an error is printed and the next pass runs); 2 usage |
| Ops | Real |

### daisy swarm status

| Field | Value |
|---|---|
| Usage | `daisy swarm status` |
| Reads | `swarm/swarm.json`, `<ws>/.env.sh` (the token), `<state>/paused`, `<state>/owner-held`; the forge: `Issues(label)` for each of the seven labels; `docker ps` as `apply` runs it |
| Writes | nothing, anywhere |
| Stdout | `Swarm: <owner>/<repo> on <forge-host> (<kind>), product repository <repo>`; `State: <state>`, then ` — paused since <time>` and ` — owner batch held: <reason>` when so; one line per label, `  %-12s %d` and the issue numbers (`  proposed     2  #20 #21`); `Running: #<n> <role>, …` or `Running: none` or `Running: unknown (docker: <message>)`; `Personas:` and one line per role, `  %-12s %-10s %s %s` (provider, model or `(default)`, effort) |
| Exit codes | 0 (also when Docker cannot be reached: the running set is `unknown`); 1 outside a workspace, invalid `swarm.json`, the token unset, a forge call failing; 2 with any argument |
| Ops | ReadOnly, extended at Step 27 to let through an `HTTP` request whose method is `GET` or `HEAD` and the `Exec` of `docker ps` (section 14, item 3); every other write, `Exec` and `Start` stays refused |

### daisy swarm pause and daisy swarm resume

| Field | `pause` | `resume` |
|---|---|---|
| Usage | `daisy swarm pause` | `daisy swarm resume` |
| Reads | `swarm/swarm.json`'s forge fields only (to name `<state>`); no secret, no forge call | the same |
| Writes | `<state>/paused` (and `<state>` when absent) | removes `<state>/paused` and `<state>/owner-held` |
| Stdout | `Paused: no new member will launch (<state>/paused); running members finish and are posted`; already paused: `Already paused since <time>` | `Resumed: members launch at the next pass`, and `Owner batch released` when `owner-held` was present; neither present: `Not paused` |
| Effect | the next pass of every `apply` (a running loop included) launches nothing; collection continues | the next pass launches as the role rule says; the owner's batch trigger is re-armed |
| Exit codes | 0; 1 outside a workspace, invalid forge fields, the write failing; 2 with any argument | the same |
| Ops | Real | Real |

## 12. The merge gate

- **Merging is the maintainer's act on the forge.** Daisy has no call that
  merges, closes or reopens (section 10's interface has none), and no
  command that does.
- **What a merge does.** The pull request's body ends `Closes #<n>`; on the
  merge into the default branch (which `base` must be, section 3) both
  forges close issue n (T11). A closed issue is in no
  `Issues(label)` list, so nothing runs for it again; its label stays as it
  was (`swarm:reviewed`) as the record of the stage it was merged from.
  The branch `swarm/<n>` is the maintainer's to delete.
- **Merging early.** The maintainer may merge at any stage; the issue
  closes and the swarm stops on it. A run already in flight is collected
  with nothing posted (7.6).
- **An issue relabelled `swarm:blocked` with a pull request open.** Nothing
  runs on it; the pull request stays open and untouched. The maintainer
  resumes it by relabelling (T10): to `swarm:in-review` for a fresh pair of
  reviews of the current head (only reviewers without one on that head
  run), to `swarm:reviewed` for one more fix round, or to `swarm:planned`
  for the worker to continue on the same branch and pull request (it pushes
  to `swarm/<n>` and comments; no second pull request). The maintainer may
  instead close the pull request and the issue.
- **A `Closes #<n>` line removed by hand** from the swarm's pull request
  makes it invisible to `PullForIssue`; a worker run on the issue then
  fails to open a second one for the same branch, stays uncollected and
  holds the issue (rule 2) until the maintainer restores the line or
  closes that pull request.
- **Approval.** Both approvals put the issue in `swarm:reviewed` and run
  nobody: the issue waits for the merge, which the swarm neither makes nor
  needs.

## 13. Seams for Steps 27–29

Each step tests at these seams first, against the Fakes, and only then
writes the code (the plan's Phase F rule). `ops.Fake` stands in for the
network (`HTTPFn`), processes, the clock and the prompt; `forge.Fake` and
`runner.Fake` stand in for the forge and Docker.

| Step | Seam | Tested against | What it pins |
|---|---|---|---|
| 27 | `declare`: `swarm.json` validation | real temp workspaces | every row of section 3's error table, one case each, the message byte for byte |
| 27 | `declare`: the wizard | a scripted `Ops.Prompt` | `--defaults` takes every default; each override is honoured; a refused answer asks again; EOF cancels and writes nothing |
| 27 | `forge`: `gitea` and `github` | `ops.Fake` `HTTPFn` with one recorded request and response per method | section 10's tables: method, endpoint, body; pagination; `EnsureLabels` idempotent; error text without the token |
| 27 | `status` | `ops.ReadOnly` over a Fake | it runs read-only (section 14, item 3) and prints section 11's lines; zero issues per label and the declared personas after `init` |
| 27 | smoke | the Gitea instance, a scratch repository | `init --defaults` creates the seven labels; a second run creates none |
| 28 | `provider` | table test | section 9's argv for each adapter, model `""` included |
| 28 | `runner.Docker` | `ops.Fake` `Exec`/`Start` and the `After` hook (item 20) | section 7.2's argv (mounts, readonly, label, name, user, env-file, stop-timeout), the key on stdin only, `docker stop` at the wall clock |
| 28 | the role rule | table over (label, newest entry's marker, running set) | launch or not, per section 6's table, the marker regex included |
| 28 | `reconcile`, `ready → planning` | `forge.Fake`, `runner.Fake` | the architect runs once; its output is a marked comment; T3; a second pass launches nothing; `Say` from the maintainer runs it again |
| 28 | smoke | a real container from `images/member/Dockerfile`, run by the maintainer with `daisy swarm apply --once` | one comment on the real issue, the label moved, `docker ps` empty, the prompt under `runs/` |
| 29 | `reconcile`, the lifecycle | `forge.Fake`, `runner.Fake` | T1–T9: an issue from `proposed` to `reviewed` with two approvals through all five roles in two rounds; `max_rounds` then T7; every failure row of 7.7 then T9; the batch trigger and `owner-held` |
| 29 | `pause`, `resume`, the lock | temp `<state>`, `forge.Fake`, `runner.Fake` | `pause` mid-lifecycle stops launches and `resume` continues; two concurrent `apply --once` launch each role once, the second exiting with the holder's line |

`runner.Fake` implements the runner interface Step 28 defines (start a
run, list the running set, stop one): it records each launch's spec,
writes the canned `out.md` (and, for the worker, commits in the clone)
that the test scripted for (issue, role, run), and reports the canned exit.

## 14. Open decisions for the maintainer

Each is a place where this document chose between readings of the plan,
or filled a gap the plan left; the choice is in force unless amended at
the checkpoint. Items 1–6 and 17–20 change behaviour the plan describes.

1. **The trigger rule, read per role.** Chosen: "the newest entry of the
   thread is not R's own", with the issue body as the first entry, and,
   in `in-review`, "R has not reviewed the head". Alternative: the plan's
   literal "the last comment is not the swarm's", under which the
   architect never runs on `ready` after the owner spoke last, and the
   second reviewer never runs after the first posted.
2. **Reviews are comment-type, the verdict in the body.** Chosen: `Review`
   posts `event: COMMENT` with `VERDICT: …` as the first line after the
   marker, because one token opens and reviews the pull request and both
   forges refuse approving one's own. Alternative: real approve and
   request-changes events, which need a second forge account (one account
   per persona is a Non-goal).
3. **`status` runs under a widened `ReadOnly`.** Today `ops.ReadOnly`
   refuses every HTTP request and every `Exec` but git reads, so the plan's
   "`status` runs under `ReadOnly`" cannot read the forge or `docker ps`.
   Chosen: Step 27 lets `GET`/`HEAD` requests and `docker ps` through
   `ReadOnly`. Alternative: `status` under `Real`.
4. **Host-side git works in a bare mirror under `<state>`.** Chosen: the
   member's clone is cloned from `<state>/git`, fetched back into it, and
   pushed from it; the product repository is only read. Alternative: the
   plan's "fetches the branch from the mounted clone into the host
   repository", which writes refs and objects into the product repository
   (the workspace itself when `repo` is `.`, against Step 30's claim that
   nothing beyond `swarm/` is written there). Both designs fetch from the
   member's clone; neither runs any other git command in it.
5. **The wall clock is `apply`'s, not `--stop-timeout`'s.** Docker's
   `--stop-timeout` is the grace between `docker stop`'s signal and its
   kill, not a run limit. Chosen: `apply` runs `docker stop --time 30` at
   `timeout`; `--stop-timeout 30` stays on the argv. Alternative: the
   plan's `--stop-timeout` set to `timeout`, which would bound nothing.
6. **The key reaches the container through `--env-file /dev/stdin`.**
   Chosen: `Ops.Start`'s stdin carries `<VAR>=<key>`, so the key is never
   on an argv nor in any file Daisy writes (Docker keeps it in the
   container's configuration while it lives; to confirm at Step 28).
   Alternative: `Ops.Exec`/`Start` gain an environment argument and the
   argv says `-e <VAR>`.
7. **The lock is a new `Ops.FS().Lock`** (an `flock` the kernel releases
   on exit) added at Step 29. Alternative: an exclusive `Mkdir`, which a
   crashed `apply` leaves behind for the maintainer to remove.
8. **Additions to `swarm.json`:** `base` (default `main`), `interval`
   (default `5m`; the plan names an interval but no field), `keys` (the
   host variable per provider; the plan says the names are declared but
   its example has none). Effort is checked as one lowercase word (the
   plan's "bad effort" and "passes through" reconciled); a model may be
   `""` for the CLI's default, which is the adversary's default since no
   OpenAI model name is fixed here; a model with whitespace is refused
   (the plan validates only adapters; one argv word is the reason). A
   secret is resolved from `.env.sh`, then the process environment (the
   plan names `.env.sh` only; the fallback serves a timer's
   `Environment=`). The default sympathetic reviewer is
   `anthropic` (the plan's example has `xai`, which has no adapter).
9. **Additions to `<state>`:** `owner-held` (the batch trigger's brake
   after a failed or empty batch, released by `resume`) and `git/` (item 4);
   `runs/` also holds `argv`, `stdout`, `stderr` and `result`. Alternative
   for the brake: pause the whole swarm, which would stop issues in flight
   when the backlog runs dry.
10. **The `reconciler` marker role** for Daisy's own notes, so a failure note is
    no role's words and a relabel from `blocked` runs the role again.
11. **Output conventions:** the owner's batch writes `## <title>` per
    proposal; any role may write `BLOCKED: <reason>` to set `blocked`
    (the plan's "set by any member" given a mechanism); a text over
    60 000 bytes is truncated with a pointer to `runs/`.
12. **The container:** run as the host user's uid (`--user`), `--pull
    never`, a per-run `--name`, the prompt and output in `<run>/io`
    mounted at `/work/.swarm` (read-write even when `/work` is read-only),
    resources at `/resources/<path>`; the worker's commits authored
    `worker (Daisy swarm) <worker@daisy-swarm.invalid>`.
13. **Sequential reviewers.** Rule 2 ("no member running for that issue")
    is kept, so the adversary reviews a head before the sympathetic
    reviewer does, and the second sees the first's review. Alternative:
    rule 2 per role, running both at once, each blind to the other.
14. **The conversation is on the issue.** Comments on the pull request are
    not read; the worker's notes after the first round go on the issue.
15. **The member image** pins `node:24.11.0-bookworm-slim` by tag (the
    digest is added at Step 28, when Docker is at hand), installs the CLIs
    at versions given as required build arguments, and adds
    `ca-certificates` for `codex` (to confirm at Step 28).
16. **The docs test admits the five agreed verbs** in this document until
    each is a row of the `cli` table, and fails once all five are, so the
    exemption cannot outlive the build.
17. **`forge.repo` must be `origin`'s repository.** The worker's branch is
    pushed to `origin` and the pull request opened in `forge.repo`, so the
    two cannot differ. The wizard keeps the plan's question but refuses
    another answer. Alternative: push to `forge.repo`'s repository, which
    needs a second URL and credentials.
18. **Paused, `apply` still collects.** The plan's "stop if `paused`"
    is read as "stop launching": members already running are collected
    and posted, so a pause loses no work.
19. **The round limit is set by the reviewer's collection** (T7: the pair
    completing on round `max_rounds` with a request for changes moves
    `in-review` to `blocked`). The plan's table puts it on the worker's
    `reviewed` row; there it would need a worker launch to decide not to
    launch.
20. **A timer in `Ops`.** The wall clock, the 429 wait and the loop
    interval need one, and nothing above `Ops` may call `time` itself:
    Step 28 adds `Ops.After(d) <-chan time.Time`, with a Fake hook, as
    item 7 adds `Lock`.
21. **Limits kept from the plan:** the container label and name carry
    `<owner>/<repo>` without the forge host, so two forges holding the
    same `<owner>/<repo>` on one machine would share a running set.
    `base` must be the default branch for T11; it is checked by host git
    (`ls-remote --symref`), not by an `Adapter` method, since the
    interface has none that reads it.
