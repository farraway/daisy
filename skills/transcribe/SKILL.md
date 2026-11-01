---
name: transcribe
description: Turning a meeting or conversation transcript into a structured written record — what was said and who said it, kept strictly apart from what you are inferring, with each inference paired to the cheapest test that would settle it. Use when a transcript, recording, or call notes need summarising, when preparing a follow-up for the other party to correct, or when the user asks what a conversation actually established.
short_description: Turn a transcript into a structured record
---

A transcript summary fails in one particular way: it launders inference into
fact. "He is avoiding the work" reads exactly like "he said he had not got to
it", and once written down the difference is unrecoverable. Everything here
follows from keeping those two apart.

The deliverable is one file — plus a second, addressed to the other party,
when there is one to send it to.

## Rules

1. **Attribute every claim.** A statement belongs to whoever made it. If you
   cannot point to where in the transcript it was said, it is an inference,
   not a confirmation.
2. **Pair each inference with its cheapest test.** An untested assumption is
   worth recording only alongside what would settle it and by when. An
   inference with no test is a verdict wearing a hypothesis's clothes.
3. **Report the observation; leave the interpretation.** Write what happened
   and what was said. Do not diagnose motive, name a cause only to dismiss
   it, or offer two options as though they were exhaustive.
4. **Write the shareable file only when there is someone to send it to.** A
   lecture, a talk, or a research transcript has no counterparty; producing
   an addressee-less "for you to correct" document is worse than producing
   nothing. State which call you made.
5. **Suggest tasks; never create them.** Commitments made in a conversation
   become a written list with owners and dates. Where `.daisy/` is present,
   offer to add them to `todo.txt` — offer, and wait for an answer.
6. **Disclose provenance in anything shared.** A document derived from a
   recording of someone says so, in the sender's own words, at the top.
7. **Preserve the words that carry weight.** Quote verbatim where the exact
   phrasing is the evidence; paraphrase everywhere else.
8. **No proposal in the shareable file.** Its job is to be corrected, not to
   persuade. Proposals need the corrections first.

## The internal record

Always written. Name it `{slug}-notes.md` beside the transcript.

```markdown
# {Subject} — notes (internal)

**Basis:** why this conversation happened, and what it is not.
**Source:** transcripts/{file}, {date}, {duration}

## Confirmed — stated directly

- ~alias: what they said, in compressed form.
- ~alias: another statement, attributed.

## Inferred — untested

| Assumption | Cheapest test | By |
|---|---|---|
| What you are reading in | The question or observation that settles it | date |

## The constraint

The thing the conversation is actually about, stated once, without
dramatising it. Often both parties already know it.

## Weighting

What the transcript itself says about which of the above matters most —
quoting the lines that carry it.

## Suggested tasks

- [ ] ~owner: action — due YYYY-MM-DD

## Next

The specific next step, and what it depends on.
```

## The shareable record

Written only when there is a counterparty. Name it `{slug}-summary.md`.

```markdown
# {Subject} — what I heard

{Your name} · {date} · for {them}, to correct

> **How this was made:** a transcript of our conversation was analysed to
> generate a summary.

This is my understanding after {when}, written down so you can tell me where
I have got it wrong. It isn't a proposal, and I have deliberately not put
one in.

## The situation, as I understand it

- Their position, in their own framing, not yours.

## What you've already tried

What they have done, taken seriously rather than as a preamble to advice.

## What I still don't know

1. A real question, whose answer would change what happens next.

## Next

The concrete next step, and what each party gets out of it.
```

## Deciding whether there is a counterparty

Ask who could correct the record. A meeting, a call, or an interview has
someone with standing to say "that isn't what I meant" — write both files.
A talk, a podcast, a lecture, or a video you transcribed for research has no
such person — write the internal record alone, and say that is what you did.

Infer this from the participants and say which way you decided, so a wrong
call costs one reply to fix.
