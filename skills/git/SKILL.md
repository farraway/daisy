---
name: git
description: Resolving git merge and rebase conflicts — recovering the original intent behind each side, resolving hunks without inventing behaviour, and finishing the operation. Use when a merge or rebase is in progress, when conflict markers appear in files, or when the user asks to resolve or finish a conflicted merge or rebase.
short_description: Resolve merge and rebase conflicts
---

A conflict is a question about **intent**, not about text. Both sides
compiled and passed review at some point; the resolution has to preserve
what each was for. Never `--abort` — abandoning the merge discards the
reconciliation work and leaves the same conflict waiting next time.

## 1. See the current state

Establish what operation is in progress and how far it got. Check the git
history on both sides and read the conflicting files in full — not just the
hunks, since surrounding code often shows why a change was made.

## 2. Find the primary sources

For each conflict, understand deeply why each change was made and what the
original intent was. Read the commit messages, then the PRs, then the
issues or tickets behind them. A hunk you resolve without knowing its
purpose is a guess.

## 3. Resolve each hunk

Preserve both intents where they can coexist. Where they genuinely conflict,
pick the one matching the merge's stated goal and note the trade-off. Do not
invent new behaviour to bridge the gap — a resolution that is neither side
is a change nobody reviewed.

## 4. Run the project's checks

Discover what the project actually runs — typically typecheck, then tests,
then format — and run them. Fix anything the merge broke. A conflict
resolved to something that compiles is not yet resolved.

## 5. Finish the operation

Stage everything and commit. If rebasing, continue until every commit is
replayed, resolving further conflicts the same way as they surface.
