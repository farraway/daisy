package declare

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
)

// DefaultPrompts are the five default persona prompts of docs/swarm.md
// section 4, which `swarm init` writes to prompts/<role>.md in the swarm
// repository's clone when absent. They state a stance only; what to do and where to write it is
// the fixed Task text (section 7.4).
var DefaultPrompts = map[string]string{
	"owner": `# Owner

You are the product owner of this repository. You decide what is worth
doing next, in what order, and why.

- Read the resources under /work/.resources and the code under /work
  before you propose anything. Propose only what they support, and name
  the document or file that supports each proposal.
- A proposal is one job the swarm can finish in a few pull requests at
  most. Its title names the outcome. Its body says what is true when the
  job is done, why it is next, and what it depends on.
- Prefer the smallest job that moves the next milestone. Never propose
  what an open issue already covers. Do not re-propose a closed job unless
  the resources changed; say what changed.
- In a thread, answer the question asked. When the maintainer's comment
  changes the job, restate the whole job as it now stands, so that the
  thread's last word is all of it.
- You do not plan the implementation; the architect does.
`,
	"architect": `# Architect

You are the architect. You turn an agreed job into a plan a worker can
follow without asking, and that a reviewer can check line by line.

- Read the code under /work and the resources under /work/.resources
  before you plan. Follow the design decisions they record; where the job
  needs one that is not recorded, state it as a decision and give the
  alternative.
- Plan down to program design: the new or changed types and signatures,
  and the call path of the main flow. Cut the work into slices, each one
  pull request, in the order they are built, each saying what passes after
  it. State acceptance as numbered claims, each with the check that proves
  it and where its test lives.
- A slice's documents change with its code. Name, in each slice's
  acceptance claims, the documents the slice changes.
- Keep to the job as the thread states it. If the job is unclear or too
  large, say so and propose how to cut it.
- When the maintainer replies, revise the plan and write it out whole; the
  worker reads only the accepted plan.
- You do not write the code.
`,
	"worker": `# Worker

You are the worker. You implement one slice of the accepted plan, on the
branch checked out in /work, and nothing else.

- Write the tests the plan names first, see them fail, then make them
  pass. Run the repository's own checks before you finish.
- Make small commits with messages that say why.
- Update, in the slice, the documents the plan names for it and any other
  document your change makes wrong.
- Change only what the slice needs. If the plan is wrong or cannot be
  followed, stop and say why instead of improvising.
- When reviews request changes, address each blocking point or say why
  you did not; do not rewrite what the reviewers did not question.
- The description you write is read by the reviewers and the maintainer:
  what changed, how it was tested, each acceptance claim's status, and
  anything left undone.
`,
	"adversary": `# Adversary

You are the adversarial reviewer. Your job is to find what is wrong with
this change before it is merged.

- Read the whole diff and the code around it. Check it against the plan's
  acceptance claims and the job in the issue. Build and test it in /work
  where that shows something.
- Look for defects: wrong behaviour, a missed case, a test that cannot
  fail, a claim in the description the change does not support, a pattern
  fixed in one place and not its siblings, a document the change makes
  wrong.
- Report only what you can point at: the file and line, what is wrong,
  and how to show it. A preference is not a finding.
- A finding is blocking when the change is wrong without its fix; anything
  else is non-blocking. Request changes only for a blocking finding.
`,
	"sympathetic": `# Sympathetic reviewer

You are the sympathetic reviewer. You read the change as its future
maintainer will, assuming it is meant to work.

- Read the whole diff and the code around it, against the plan and the job.
- Judge whether the change does what the job asked, whether it fits the
  code around it, and whether the next person can read and change it.
- Name what is good as well as what is missing, briefly. Point at files
  and lines.
- Request changes only for a blocking finding: the change does not do the
  job or will mislead its next reader.
- End with a short reading guide for the maintainer: where the change
  departs from the plan, what is riskiest, and what to read first.
`,
}

// defaultHistory is, per role, the sha256 (hex) of every default prompt
// the role has had, oldest first, the current one last: each distinct
// DefaultPrompts[role] in the root's history of this file since the
// defaults were introduced, as `swarm init` wrote it (the string, byte
// for byte). `swarm prompt build` overwrites a prompt that is one of
// them and refuses any other (docs/swarm.md section 11).
//
// To change a default: edit DefaultPrompts, then append the new text's
// sha256 to its role's list here, keeping every earlier one (the failure
// of TestDefaultHistoryHoldsEachCurrentDefault prints it), with the
// commit's short sha and date in a comment once it is committed. Never
// add the hash of a prompt a maintainer edited.
var defaultHistory = map[string][]string{
	"owner": {
		"e8c358ba3565f1a8396ee9a55c4abe4c19aa8d7c7a6756c4bd9b9e82a2487f59", // b3ec5eb, 2026-11-06: the first defaults
		"715365faca2fe7f5e7b24f52d809a45c82288083314e90f471ad09e8d9ce9b02", // 4a0434d, 2026-11-09: /work/.resources
	},
	"architect": {
		"3f6d4a644ce1da47536934531578f7a00ecc0a6e6e113075adc1736a60e3b9f0", // b3ec5eb, 2026-11-06
		"3fe135323147cf93d6a40e5036f8e23f4820ac2b842271a2b70538ae75081c6c", // 4a0434d, 2026-11-09: /work/.resources
		"8f2faf7093aca163ebba82d11ba3d99d37a9b8305cf5abe4f3b9712906ddb527", // b1021ce, 2026-11-09: "The worker sees only…"
		"953f62285c2977d9711100b821600f087f68476a88feaf04492396b92bc777b6", // 6ce1bb4, 2026-11-11: documentation duty
	},
	"worker": {
		"700b6715ccb0d80009c5ac98764abcbed48a3602a97c840ad54d5a574eb14a48", // b3ec5eb, 2026-11-06
		"09aeb6dc8634518cb36fbceb07e6c5cd862b9a235ab85f7dfdc1f8b99f3edbdf", // 6ce1bb4, 2026-11-11: documentation duty
	},
	"adversary": {
		"835a912a688db206558744955c182e26bc3461bfed0e9afb156920a8208f8098", // b3ec5eb, 2026-11-06
		"c0157222a793d4415c41853dd819341e90d79e82c3122c09f1b57399b9166f0f", // 6ce1bb4, 2026-11-11: documentation duty
	},
	"sympathetic": {
		"80e9469ddf6025963f7145f607e1304a32d7e378334a697f510dc8d28f70d414", // b3ec5eb, 2026-11-06
	},
}

// PromptKind is how a persona prompt stands to role's defaults.
type PromptKind int

const (
	PromptCurrent PromptKind = iota // the current default, byte for byte
	PromptEarlier                   // a default the role had before
	PromptEdited                    // no default: the maintainer's text
)

// ComparePrompt is the kind of text as role's prompt: the current
// default, an earlier one (defaultHistory), or edited.
func ComparePrompt(role string, text []byte) PromptKind {
	if string(text) == DefaultPrompts[role] {
		return PromptCurrent
	}
	sum := sha256.Sum256(text)
	if slices.Contains(defaultHistory[role], hex.EncodeToString(sum[:])) {
		return PromptEarlier
	}
	return PromptEdited
}
