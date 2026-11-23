# 0171. Review by risk, with an end: a fixed list of what blocks, trust boundaries, agreed probes, convergence, follow-ups and the escaped-defect measure

From Step R6-design of the plan implementing the 2026-11-19 swarm roadmap (R6, review by risk, with an end, and the parts of R5 it needs, 2026-11-22). Specified before its build: `docs/swarm.md` section 7.11, with the changes it makes marked in sections 4, 5, 6, 7.3, 7.4, 7.5, 7.6, 11 and 12; its open questions ruled on by the maintainer on 2026-11-22 (The rulings, below) and built a part at a time, as the As built notes at the end say. Amends decisions [15](0015-severity-per-finding.md) (what blocks is a fixed list), [63](0063-the-adversarys-fail-against-base-check-is-bounded.md) (the plan's probes beside the rollback), [144](0144-a-recurring-finding-asks-for-a-check.md) (a check finding reaches the owner as a follow-up), [156](0156-a-scorecard-per-merged-slice.md) (the scorecard's escaped-defect side) and [168](0168-a-delta-review.md) (from round 2, a claim the head fails blocks on any code, and nothing else new does).

**The maintainer's direction of 2026-11-22**, on the analysis below: "I'm
inclined to agree with your findings but I don't want to start making
brittle software. Similarly, I don't want the adversary making
outlandish demands." The approach he agreed: what blocks is a short,
fixed list, the same for every reviewer (the plan rules' own review
classes, `prompts/plan.md` rule 22, applied to the swarm); "realistic"
is defined by the product, once, as trust boundaries in each
repository's notes; probes are agreed in the plan, the architect naming
each test's breaking mutation and the worker recording it, and a mutant
the adversary invents that survives is non-blocking unless it exposes a
fault on the head; unsupported setups are listed per slice by the
architect; a finding is closed when its own reproduction now passes,
and from round 2 a new variant of a closed finding is a new finding,
non-blocking unless it breaks a claim, so that only an unclosed blocker
or a fault the revision introduced blocks; non-blocking findings are
collected into a follow-up issue the owner triages, so the adversary's
thoroughness still counts, as backlog and not as a veto; the balance is
measured, the escaped-defect side beside the cost, by configuration; the
round limit's outcome, a process rule, is his ruling; and the worker's
Task names which gates are Daisy's to run.

**The evidence.** 103 blocking findings of the live swarm's adversary
reviews, read one by one: 49% about tests or probes (a mutant the
reviewer invented survives; fixtures of one kind only); 24% hardening
against hostile or unlikely setups (a umask of `0477`, a planted `GOENV`
file, a percent-encoded path segment); 14% the slice's claim fails; 8%
real defects outside the claim; 4% wording; none shown to be a misread.
From round 2 on, 31 blocking findings: 18 new variants of a fixed
finding on code the revision touched, 7 faults the revision introduced,
5 weak new tests, 1 a true carry-over. Blocking findings per review fall
6.5, 2.0, 1.6, 0.5 by round, but any one blocks: 70% of round-3 reviews
still requested changes. Only 4 of 10 revision sets fixed a real defect;
their test lines outnumber their production lines about 5 to 1, and
about 85% of the production lines were hardening. The adversary's model
matters: round-1 request-changes rates were 31% on grok-4.7, 43% on
gpt-6.1-sol and 96% on gpt-6.2-astra at high effort. Workers never
declined a blocking finding, and nobody judged scope until the round
limit. The request of 2026-11-22 adds that twelve of 37 pull requests
took four or more rounds, five reaching the round limit within four
hours of a restart, and that a worker stopped with cause `environment`
for a database and a cache its container lacked, although Daisy's check
runs that gate.

**The rules**, quoted in the same words by `docs/swarm.md` section 7.11:

1. **What blocks.** A review finding is blocking only when it is one of
   four: the slice's claim fails (a claim of its Test Plan that the head
   does not show); a test the plan names for the slice is missing on the
   head or fails; a defect in the code the change adds or changes that a
   realistic user or input meets (rule 2); or a fault the revision
   introduced (behaviour that worked on a head reviewed before and fails
   on this one). Every other finding is non-blocking, however sound. The
   list is Daisy's fixed Task text, the same for both reviewers in every
   round.
2. **Realistic, by the trust boundaries.** Each repository's notes
   declare its trust boundaries once: what the product trusts (its own
   process's environment, umask, configuration and filesystem) and what
   it does not (requests, uploads, imports: what a user or a peer
   sends). An input across an untrusted boundary is realistic whatever
   its content, so hardening there is in scope and can block; a fault
   met only when something the product trusts is hostile or unusual is
   non-blocking.
3. **Probes agreed in the plan.** The plan's Test Plan names each
   test's probe, the change to the code under test that the test must
   fail on; the worker applies each and records the failure it saw, and
   the adversary checks those probes. A test that survives its named
   probe does not show its claim, and blocks as the claim's failure. A
   test that survives a probe the reviewer invents is non-blocking,
   unless the surviving change exposes a fault on the head, which is
   then judged as a defect.
4. **Unsupported setups per slice.** Each slice of the plan lists the
   setups it does not support (platforms, configurations,
   environments), or none, and a fault met only in one of them is
   non-blocking.
5. **Convergence.** A blocking finding gives its reproduction (a
   command, a test or steps), and it is closed when that reproduction
   now passes on the head, or answered with a reason the reviewer
   accepts. From round 2, a finding is blocking only when it is an
   earlier blocking finding not closed, a fault the revision introduced,
   or a claim of the slice the head does not show; any other new
   finding, a new variant of a closed one among them, is non-blocking.
6. **Follow-ups.** No non-blocking finding is lost: when a slice's pull
   request merges, Daisy files the non-blocking findings of its reviews,
   each once, as one follow-up issue, and the owner triages it in its
   next batch, proposing a job of what is worth doing; Daisy closes the
   issue on the owner's word.
7. **The balance, measured.** Beside each merged slice's cost (its
   runs, tokens, review rounds and stops), the scorecard reads its
   escapes (its merge reverted, a bug filed against it, a fix of it
   pushed forward by hand) and its follow-up items that a merged job
   later fixed, and shows both by configuration: the escapes against
   the reviewer configurations that approved the merged head, the
   follow-up items against those that raised them.
8. **The round limit.** The first time a pull request reaches the round
   limit, the architect judges each outstanding blocking finding against
   rules 1 and 5. When none qualifies it writes `OVERRULED:`; the
   sympathetic reviewer then reviews the head, its approval goes to the
   merge gate, and the overruled findings go into the follow-up. When
   one qualifies, the architect revises the plan. The second time, the
   issue is blocked. `max_rounds` is unchanged.
9. **Daisy's gates.** The worker's Task names the gates that are
   Daisy's to run (the declared check, in the repository's check
   environment with its test services, on the head the worker pushes,
   and the forge's required checks before the merge); a gate the
   worker's container cannot run goes under "Not established:", and is
   never a reason to stop.

**Why a fixed list, in Daisy's text.** Decision 15 gave each finding a
severity because "a reviewer told to find fault always does"; the
severity was the reviewer's own call, by a persona sentence ("blocking
when the change is wrong without its fix") that every hardening finding
meets. The list moves the call from the reviewer's taste to four things
the head can be shown to do: fail a claim, lack or fail a named test,
ship a defect a realistic user meets, or break what worked. They are
`prompts/plan.md` rule 22's classes read for code (its "a document
contradicts another" is a claim the slice makes about its documents,
which the architect names among its claims). The list is in the Task
text, not the persona, so an edited persona cannot move the bar (section
4: a persona states a stance, the output contract is Daisy's), and it
is the same for both reviewers: the request's sympathetic reviewer
rarely blocks, but when it does it blocks by the same list. A blocking
finding names its kind and its reproduction, which costs the reviewer a
word and a line it should already give ("how to show it", section 4),
and gives the worker, the architect at the limit and the scorecard
something to read. Daisy parses the kind only to count it: it never
demotes a blocking finding itself, and a line without a kind is
counted `unnamed`, never refused, since a verdict Daisy overrides is
the brittleness the direction rules out. The alternatives: a severity
scale (critical, major, minor), which keeps the judgement where it was;
a bar in the persona alone, which is the live swarm's (below) and
which a swarm's edit can drop; Daisy demoting a blocking finding whose
kind is not on the list, an override of the reviewer's verdict that a
missing word would set off.

**The live swarm's change, weighed.** The swarm's own reviewer prompts
took, on 2026-11-21, a blocking bar (a claim broken, or a fault a
visitor, a member of staff or an operator meets in a configuration the
project supports; an unsupported or hostile environment non-blocking),
a convergence rule from round 2 (only an unclosed blocker, or a fault
the fix introduced that breaks a claim or what worked, blocks; a pull
request whose previous blockers are closed is approved) and a narrower
probe rule (a test that survives its probe blocks only when it is the
only test of a claim). Daisy's defaults take the first two, generalised:
the bar's "configuration the project supports" is each repository's
trust boundaries and each slice's unsupported setups, declared once
rather than in each reviewer's words; the convergence keeps a claim
that fails blocking on any code, which the swarm's "approved once the
previous blockers are closed" would let through. The probe rule is
replaced rather than taken: "the only test of a claim" still blocks on
a mutant the reviewer invents, half of the blocking findings above;
the plan's agreed probe is a test's own bar, set before the code is
written.

**Trust boundaries** (rule 2). A finding about a umask of `0477`, and
one about a malformed upload, read alike to a reviewer: each is an input
the code mishandles. What tells them apart is the product's own word on
what it trusts, which no persona can carry for every product: so each
repository's notes, the maintainer's word to every member working in it
(section 4), hold it under `## Trust boundaries`, and `swarm init`
seeds a default in the maintainer's own terms of 2026-11-22 (the
server's environment, umask, configuration and filesystem trusted; what
a user or a peer sends not). Daisy reads only whether the heading is
there, so the blocking sentence can state the default where it is not;
a repository whose notes say nothing more is judged by the default. A
declaration in `swarm.json` was the alternative: Daisy would read none
of it either, and the notes are where the maintainer already writes
what a member should know of a repository.

**Probes in the plan, and unsupported setups per slice** (rules 3 and
4; R5's part). Half the blocking findings were about tests, most of
them a mutant the reviewer had chosen: a test bar set after the code by
whoever reviews it, so each round could raise it again. The architect
names each test's probe in the Test Plan, the worker applies it and
reports what it showed, and the adversary checks that probe; one of its
own that a test survives is a finding the owner may take up, not a
veto, unless the surviving change shows the head is wrong, which is a
defect and judged as one. The adversary's rollback (decision 63) stays
beside it: it is Daisy's own named check, not an invented one, and
catches a test that passes without the change. A plan without probes
blocks on no invented one; the rollback alone is its named check, so
the plans written before the build are judged as they would be after.
The roadmap's R5(a) also had the architect apply each probe before
posting, with a probe table; that is left to R5's own design: a new
behaviour's test does not exist until the worker writes it, so the
worker's record is the first one possible. The unsupported setups are
the slice's own exclusions (a platform, a configuration), which the
trust boundaries, stated once per repository, do not reach.

**Convergence** (rule 5). From round 2, 18 of 31 blocking findings were
new variants of a finding already fixed: the reviewer read the fix,
found its edge, and raised it, so every revision bought a round. A
finding is closed by its own reproduction, the one its blocking line
gave, so a variant is a new finding; from round 2 it is non-blocking
unless it breaks a claim. Decision 168 already made a finding on
unchanged code non-blocking; this narrows changed code to the
revision's own faults and widens unchanged code to the one thing that
must not merge, a claim the head fails. The delta review's other text,
the round's rounds on an unchanged head and the count of rounds are
unchanged. A reviewer's first review in a later round (the sympathetic
reviewer's, decision 163) is bound by the round, not by its own count:
the pull request has had its rounds, and its findings beyond a failing
claim or the latest revision's fault go to the follow-up. The worker
answers, rather than fixes, a blocking finding that is none of the four
kinds, saying which it is not: decision 63 already let a worker answer
with a reason, and the evidence is that none did.

**Follow-ups** (rule 6). *When:* at the merge, so that what is filed is
about code that landed and the findings of every round, the approving
ones included, are in; a pull request closed unmerged files nothing.
*By whom:* Daisy, from the reviews on the forge, with no run, so the
filing costs no tokens and cannot fail as a member's output can.
*Deduplicated* three ways: the later round is told not to raise again
a non-blocking finding the previous round or its own last review
raised (it is given both since decision 168); the filing drops a finding whose text, its line
number aside, is already listed; and the owner takes findings of one
cause as one job. *Triaged* by the owner in its batch, which already
reads the open issues and proposes: its prompt gains `# Follow-ups`, a
job made of follow-up items says so on a `Follow-up:` line, and a
`Triaged:` line closes a follow-up whose items are all dealt with, on
the owner's word, as a `Met:` line reports a brief met. The issue
carries a new flag, `swarm:follow-up`, and no stage, so no role runs on
it and it holds no proposal's place. The alternatives: a follow-up at
`swarm:proposal-posted`, which the owner would answer in a run of its
own per merge (some 20 to 40 a day at the roadmap's rate, each with the
owner's system prompt) and the triage gate would then hold for the
maintainer one by one; filing at each round, which files findings about
code the next revision changes; one rolling issue per repository, whose
body grows without end and whose triage cannot close part of it; the
reviewers filing issues themselves, which gives a member the forge and
spends a run's tokens on it; and the maintainer closing each follow-up
(`Met:`'s way), which keeps him in a loop the direction aims to take him
out of. The trigger is the issue's pulls block not yet showing the pull
request merged, so the build files nothing for pull requests merged
before it. A pull request merged by `swarm merge` is filed by the verb,
under the maintainer's token, the one it holds, before its own listing
of the issue's pull requests writes the block showing the pull request
merged (ruling 4); the alternative, the verb leaving the block's
`merged` to the next pass for an issue it leaves open, files none for
an issue it closes. The follow-up of an issue closed by hand is not
filed: no pass lists a closed issue. The eighteenth label is created, as the sixteenth was, by an
owner of the organization where the swarm's account cannot.

**The balance** (rule 7). Cost is recorded already: runs, tokens,
rounds and stops per merged slice, by configuration (decisions 156 and
165). A lower bar trades rounds for escapes, and what escapes was not
counted beyond reverts (none so far). Three escapes are read from what
the swarm and the maintainer already write: a revert, as `status`
reads one (decision 141); a bug filed against the slice, an issue whose
body holds `Defect in: <name>#<m>`, which the owner's and the QA's batch
texts ask for when they find a failure's cause in a merged pull
request's change, and the maintainer writes in his own, which carries no swarm label, so the
scorecard's `Issues` takes an empty label as every issue of the state (a seam
change, section 10); and a fix
pushed forward by hand, a commit on `base` that is no swarm merge and
whose message holds the same line. The fourth figure is the follow-up
items a merged job names on its `Follow-up:` line: a non-blocking
finding that proved real. Escapes count against the reviewer runs whose
approval of the merged head stood, follow-up items against the runs
that raised them, so the adversary's models (31% to 96% at round 1)
read on both sides, by `config_hash`. Not established: a defect fixed
by a job that does not name it, or reverted by hand without a revert
commit. Alternatives: a window after the merge within which any hand
commit to the slice's files counts (a heuristic with a limit no one has
set, which would count refactors); a label for escapes (one more state
on the forge for the maintainer to keep).

**The round limit** (rule 8). The request of 2026-11-22 asks for an
outcome that needs no maintainer when every remaining adversary blocker
is new. Under decision 163 the sympathetic reviewer reviews only a head
the adversary approved, so "the sympathetic reviewer approves" and "the
adversary requests changes" do not meet on one head without a change.
The options put to the maintainer:

- (a) T11 as today: the first time, the architect revises the plan from
  the reviews, and the rounds count again from the revised plan; the
  second time, the issue is blocked. Rules 1 and 5 alone make fewer
  pull requests reach the limit.
- (b) The architect judges first: the first time, the architect's
  open-slice text asks it to judge each blocking finding of the last
  round against rules 1 and 5 (the trust boundaries and the slice's
  unsupported setups with them). When none qualifies it writes
  `OVERRULED: <why, finding by finding>` as its first line: Daisy posts
  it on the pull request, the head stands as the adversary's approval
  would, the issue returns to `swarm:review-1-in-progress` (a new
  transition), the sympathetic reviewer reviews the head as after an
  approval, and its approval goes to the merge gate as T9 does, the
  overruled findings filed in the follow-up marked `outstanding at the
  round limit`; its request for changes is a round as any other. When
  any finding qualifies, the architect revises as in (a). The second
  time, blocked as today. The plan's author judges its own claims, the
  sympathetic reviewer keeps a second key, and the maintainer still
  meets the head at the merge gate when it is on.
- (c) Merge when the sympathetic reviewer approves and every remaining
  blocker is new in the last round (the request's example): Daisy
  would tell "new" from the kinds the blocking lines name, and a new
  blocking finding of kind `claim` or `introduced` would still hold the
  merge. It needs the sympathetic reviewer launched at the limit on a
  head the adversary sent back, which decision 163's order does not do.
- (d) A limit per class (the roadmap's R6(d), 2 for routine and 3 for
  architecture), with any of (a) to (c): a limit, the maintainer's.

The maintainer ruled (b) (ruling 1, below), as recommended, with
`max_rounds` left as it is.

**Daisy's gates** (rule 9). A worker stopped for a database its
container lacked, the integration gate being Daisy's check's, which
runs the repository's test services (section 7.9). The worker's Task
now says which gates are Daisy's and that one it cannot run is listed,
not a stop. Its own `environment` stop, a tool or service its work
needs, is unchanged.

**The defaults** (section 4). The adversary's, the sympathetic
reviewer's, the architect's and the worker's default prompts take the
rules in their own words. The reviewers' also take the token rules the
live swarm's reviewers run under (the roadmap's R6(e), F47): neither
reruns the check Daisy ran; the adversary runs the probes and the tests
of the packages the change touches; the sympathetic reviewer reads and
runs nothing; the adversary stops once its scope is checked. The
sympathetic reviewer's reverses decision 168's note keeping its "try it
in your clone where that is cheap"; the maintainer ruled them in
(ruling 2, below).
The swarm's own reviewer part, which states its bar in its product's
words, is then the maintainer's to retire.

**What of R6 this leaves.** (a) One reviewer for routine jobs: decision
163 already launches the sympathetic reviewer only on a head the
adversary approved; whether a routine job needs it at all waits on the
week's figures. (b) The delta review: built at Step E2 (decision 168).
(c) The severity rule: this decision. (d) A round limit per class: a
limit, option (d) of the round limit, not taken. (e) The token rules: the defaults
above. And of R5: (a)'s architect run of each probe and its probe
table, (b) normative and illustrative plan text, (c) amendments in the
job's `plan.md`, and (d) the plan lint are R5's own design.

**The rulings** (the maintainer's, of 2026-11-22, on the questions this
design left open, each as recommended). 1. The round limit's outcome
(rule 8) is option (b): the architect judges the outstanding blocking
findings against rules 1 and 5; when none qualifies it writes
`OVERRULED:`, the sympathetic reviewer then reviews, and an approval
goes to the merge gate, the overruled findings going into the
follow-up; `max_rounds` is unchanged. 2. The token rules the live
swarm's reviewers run under go into the default reviewer prompts:
neither reviewer reruns the check; the adversary runs the probes and
stops once its scope is checked; the sympathetic reviewer reads and
runs nothing. 3. The default trust boundaries, stated where a
repository's notes declare none, stand as written. 4. `swarm merge`
files the follow-up of the pull request it merges under the
maintainer's token, the issue then the maintainer's; a pass listing
closed issues, for an issue closed by hand, was the other option.

**As built: the blocking sentence (Step R6-A of the plan implementing
the 2026-11-19 swarm roadmap).** The list is the Task's, after the row's
text and before the slice sentence, as paragraphs of its own with a list
item per kind, not a sentence run on in the row's paragraph: a sentence
in the middle of a paragraph is what decision
[167](0167-the-system-prompt-stable-blocks-first.md)'s amendment found a
smaller model to miss. Each item's kind stands before it, in place of a
sentence naming the four after. The `# Output` block stays one text per
role: its sentence on when to request changes defers to the Task's list
rather than leaving the block, so the block still says it, and its
template's blocking line shows the kind and the reproduction; the
alternative, the template left as it was, would show a form the Task
contradicts. The verdict's statement and its reading are unchanged. The
reproduction is asked for on the finding's own line, since a round's
file keeps only the lines that begin with a marker. The notes' heading
is named `"Trust boundaries"`, without its `##`, as the prompt shifts a
note's headings. The text says that Daisy files the non-blocking
findings, which Step R6-B builds. Tests: `TestTaskTextBlockingSentence`,
`TestTaskTextBlockingSentenceWithoutTrustBoundaries`.

**As built: convergence (Step R6-A of the plan implementing the
2026-11-19 swarm roadmap).** The convergence sentence, and the sentence
of a reviewer whose first review falls in a later round, are the
paragraph after the list of what blocks, not the row's last sentence
(the delta's) nor the round's addition at the end: each narrows the
list, and read before it the list of four would widen it again. "That
round" and "your review" read "the previous round" and "your last
review", away from the sentence they pointed to. The rounds on an
unchanged head keep their sentences, at the end. The worker's sentence
is a paragraph of its own after its `review-3-changes-requested` text.
Tests: `TestTaskTextConvergenceSentence`,
`TestTaskTextLateFirstReviewIsBoundByTheRound`,
`TestTaskTextWorkerAnswersAFindingThatDoesNotBlock`.

**As built: the probes and the unsupported setups (Step R6-A of the
plan implementing the 2026-11-19 swarm roadmap).** The two lines are
asked for in a list of their own after the text that asks for the plan,
not in the headings sentence's parentheses, where they would be two more
clauses of a sentence of some 1,400 characters; and in each text that
asks for a plan, the revision's and the accepted plan's as well as the
`proposal-accepted` row's, since a revised plan is written whole by a
run that does not see that row (the one row, which would leave a revised
plan's probes to the persona, was the alternative). The worker's and the
adversary's are paragraphs of their own, the worker's on a slice of a
plan alone, the fast lane having no Test Plan. Tests:
`TestTaskTextPlanAsksForProbesAndUnsupported`,
`TestTaskTextWorkerReportsProbes`,
`TestTaskTextAdversaryChecksTheNamedProbes`.

**As built: Daisy's gates (Step R6-A of the plan implementing the
2026-11-19 swarm roadmap).** The gates sentence is a paragraph of its
own after the check sentence, in every worker text, the fast lane's
among them, and the "Not established:" sentence begins the next. Test:
`TestTaskTextWorkerGatesSentence`.

**As built: the defaults and the seed (Step R6-A of the plan
implementing the 2026-11-19 swarm roadmap).** The four defaults are
section 4's designed texts, but for the architect's sentence `The
reviewers check those probes; a probe that cannot fail fails the slice's
review, not the worker's.`, which reads `The adversary checks those
probes: a test that survives its own holds the slice's review.`: only
the adversary runs them. The reviewers' Task offered both a clone `you
may build and test in`, which the sympathetic reviewer's default now
rules out; its Task says `a throwaway clone`, the last word it reads
being the Task's (the Task left as it was, its permission against the
persona's rule, was the alternative). The seed's heading is the one the
blocking sentence looks for. `swarm prompt build` brings a swarm still
at an earlier default to these; an edited prompt is kept. Tests:
`TestDefaultPromptsAreSectionFours`,
`TestDefaultHistoryHoldsEachCurrentDefault`, `TestInitSeedsTrustBoundaries`,
`TestTaskTextSympatheticReadsItsClone`.

**As built: `Defect in:` (Step R6-A of the plan implementing the
2026-11-19 swarm roadmap).** The sentence is a paragraph of its own in
the owner's and the QA's batch texts, after the row's text and before
the protected paths; nothing reads the line until Step R6-C's scorecard.
Test: `TestTaskTextsAskForDefectIn`.

**As built: the label and `Issues(state, "")` (Step R6-B of the plan
implementing the 2026-11-19 swarm roadmap).** `swarm:follow-up` is the
seventh flag, so no stage is read from it and an issue carrying it
alone is no swarm issue; `init` and `apply` ensure it with the other
seventeen. An empty label leaves Gitea's `labels` query out and the
fake's filter off; the GitHub adapter, not yet built, takes it with its
build. Test: `TestIssuesWithoutLabel`, and the label counts of the
`init` and `apply` tests.

**As built: the follow-up's findings and its filing (Step R6-B of the
plan implementing the 2026-11-19 swarm roadmap).** One function lists
an issue's pull requests for the pass, the merge step and `swarm merge`
alike (`ListPulls`), so the filing sits there, between the read of the
body again and the block's write, for each pull request merged whose
item the body does not show `merged`; the verb gives it the
maintainer's adapter, the pass the swarm's. The findings are read by
round (section 6's rounds, the reopenings without a commit among
them), each review whole where Daisy cut it on the forge, as decision
[149](0149-daisy-reads-a-text-it-cut-whole.md) reads every text it
cut: a finding past the cut would otherwise be lost, which rule 6 rules
out (the posted text alone, which section 7.11's `Reviews(m)` could be
read as, was the alternative). The deduplication's key is the line with
the `:<digits>` after a reference's file dropped and every space and
case ignored. A filing that fails is the listing's failure, a warning,
the block left unwritten, so the next listing files it; the lists of
follow-ups are read once per pass, or per verb, and grow with each
one filed. The follow-up's marker is read as the reconciler's, as the
gate note's is. This answers the triage's L378 (a round-1
non-blocking finding not repeated in the approving round reached no
one): it reaches the owner in the follow-up, the merge gate's note left
as section 5 has it. Tests: `TestFollowUpFindings`,
`TestSwarmFollowUpFiledAtTheMerge`, `TestSwarmFollowUpFiledOnce`,
`TestSwarmFollowUpNotFiledForPastMerges`.

**As built: the owner's batch reads the follow-ups (Step R6-B of the
plan implementing the 2026-11-19 swarm roadmap).** The batch reads the
open follow-ups once it is due, after its pacing, so they neither hold
nor wake it; `# Follow-ups` reads `None.` when there are none, and the
Task's sentence is there in every owner batch, as the section quotes
it, rather than only with a follow-up open (the alternative, which
would make the owner's `prompt_hash` vary with the forge). The QA's
prompt and Task have neither. Test: `TestSwarmOwnerBatchReadsFollowUps`.

Tests, named for the build: `TestTaskTextBlockingSentence`,
`TestTaskTextBlockingSentenceWithoutTrustBoundaries`,
`TestTaskTextConvergenceSentence`,
`TestTaskTextLateFirstReviewIsBoundByTheRound`,
`TestTaskTextAdversaryChecksTheNamedProbes`,
`TestTaskTextPlanAsksForProbesAndUnsupported`,
`TestTaskTextWorkerGatesSentence`, `TestInitSeedsTrustBoundaries`,
`TestDefaultPromptsAreSectionFours` and
`TestDefaultHistoryHoldsEachCurrentDefault` with the new defaults;
`TestFollowUpFindings` (the dedup),
`TestSwarmFollowUpFiledAtTheMerge`, `TestSwarmFollowUpFiledOnce`,
`TestSwarmFollowUpNotFiledForPastMerges`,
`TestSwarmOwnerBatchReadsFollowUps`,
`TestSwarmTriagedClosesTheFollowUp`; `TestEscapes`,
`TestScorecardEscapesAndFollowUps` and
`TestScorecardEscapesByConfiguration`.
