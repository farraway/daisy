# Logging - Design & Reference

## Design Rationale

### Why "If You Helped DO Something, Log It"

The original design enumerated 11 specific trigger conditions for proactive logging (stakeholder interactions, starting work, key discoveries, design decisions, etc.). In practice, agents would lose track of the list mid-conversation and forget to log.

The simplified rule -- **"if you helped the user DO something, log it"** -- is a principle rather than a checklist. It's easier to remember, harder to miss, and covers the same ground. The distinction between "doing" and "discussing" prevents over-logging while ensuring meaningful work is captured.

## Detailed Logging Triggers

These are the specific situations that fall under the "helped DO something" rule. They are reference material for understanding intent, not a checklist for the agent to memorize:

1. **Stakeholder interactions** - User mentions meetings, discussions, decisions with colleagues
2. **Starting work** - User begins a task or asks for help with implementation
3. **Key discoveries** - Finding root causes, tracing bugs, identifying patterns
4. **Design decisions** - Choosing approaches, making architectural choices
5. **Implementations complete** - After writing/modifying code
6. **Blockers encountered** - When progress stops due to external factors
7. **Milestones reached** - PRs opened, tests passing, deployments complete
8. **Context switches** - Moving between tasks
9. **Questions raised** - Uncertainties that need resolution
10. **Technical debt identified** - Code that should be refactored or improved
11. **Learning moments** - Understanding new APIs, patterns, or legacy code

## Log Audit During Retrospective

The retrospective workflow includes a pre-step that compares completed tasks against log entries. This structural check catches logging gaps that the proactive rule missed. The audit works because:

- Completed tasks are always tracked (done.sh handles this mechanically)
- Log entries are sometimes missed (depends on agent behavior)
- The retrospective is a natural checkpoint where gaps can be filled

This creates a safety net: even if the agent forgets to log during the day, the retrospective will surface the gap before the day is archived.

## Abridged Archival

**Abridging only happens during weekly review**, never during daily archival.

When starting a new week, optionally curate quiet days in journal.md:

```
Goal: Create useful historical record without verbose minutiae (ONLY during weekly review)

Preserve (never lose):
- Stakeholder interactions: "Met with ~person", "~person decided"
- Task progress: "Completed", "Blocked by", "Started"
- Discoveries: "Found", "Discovered", "Traced to"
- Decisions: "Decided to", "Chose", "Approved"
- Milestones: "Opened PR", "Merged", "Released"

Condense:
- Multiple "working on" entries -> Time range + outcome
  Example: "0930, 1015, 1045 working on X" -> "0930-1200 - Investigated X, found Y"
- Routine status updates -> Omit if outcome is logged

Format:
- Time ranges for extended work: "0930-1200 - {activity and outcome}"
- Explicit times for events: "1130 - Met with ~person about X"
- Keep stakeholder aliases: ~person format preserved
```

### Archival Example

**Original today.md log:**
```
- 0930 - Started investigation of PROJ-1234
- 1015 - Still working on PROJ-1234
- 1045 - Making progress
- 1130 - Found race condition in adapter init
- 1215 - Met with ~jdoe about approach
- 1245 - Decided to use instance-based pattern
- 1445 - Implemented fix
- 1530 - PR#1545 opened
- 1545 - PR approved by ~jdoe
- 1600 - PR#1545 merged
```

**Abridged for journal.md:**
```
- 0930-1445 - Investigated PROJ-1234, found race condition in adapter init
- 1215 - Met with ~jdoe about approach, decided instance-based pattern
- 1530-1600 - PR#1545 opened, approved by ~jdoe, merged
```
