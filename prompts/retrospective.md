## Trigger

Read the full `daisy/prompts/retrospective.md` when:
- User says "Daisy, help me with my retrospective"
- User asks about successes, misses, or "what would a sage do"
- Starting a new week (weekly retrospective section)
- User asks for reflection prompts or feels stuck/overwhelmed

# Daily and Weekly Retrospective Guide

This prompt helps structure end-of-day and end-of-week reflections.

## Daily Retrospective Format

The daily retrospective in `today.md` uses three core questions:

### 1. Successes
**What went well today?**

Focus on:
- Tasks completed successfully
- Problems solved effectively
- Good decisions made
- Skills applied or developed
- Positive interactions or collaborations
- Efficient processes followed

**Examples:**
- "Deep-dive investigation saved from implementing dead code"
- "Pair programming session accelerated understanding of legacy system"
- "Proactive communication prevented merge conflict"
- "Test-driven approach caught edge case early"

### 2. Misses
**What could have been better?**

Reflect on:
- Tasks that took longer than expected (why?)
- Mistakes or missteps (what can you learn?)
- Opportunities missed
- Processes that were inefficient
- Communication gaps
- Assumptions that proved wrong

**Frame constructively:**
❌ "Wasted time on X"
✅ "Underestimated complexity of X - learned Y for next time"

❌ "Bad code review delayed me"
✅ "Review cycle highlighted need for earlier sync on architecture"

### 3. What Would a Sage Do Next?
**What's the wise, thoughtful next action?**

The "Sage" framework encourages:
- **Strategic thinking** - Consider long-term impact
- **Learning orientation** - What skills to develop?
- **Relationship building** - Who to connect with?
- **Process improvement** - What systems to refine?
- **Self-care** - When to rest and recharge?

This question shifts from "what must I do?" to "what would be most valuable?"

**Examples:**
- "Document architectural findings for team knowledge base"
- "Schedule pairing session with ~colleague to share context"
- "Refactor duplicated logic before it spreads further"
- "Block focus time tomorrow for complex problem"
- "Take morning to recharge before starting next sprint"

## Weekly Retrospective Format

At week's end or week's start, reflect on the bigger picture:

### Weekly Review Questions

**Accomplishments:**
- What did you ship or complete?
- What progress did you make on larger initiatives?
- What did you learn?

**Patterns:**
- What themes emerged this week?
- What kept coming up as a blocker?
- Where did you spend most of your time?
- What energized you? What drained you?

**Relationships:**
- Who did you collaborate with effectively?
- Who do you need to follow up with?
- What communication could have been better?

**Process:**
- What workflows worked well?
- What felt inefficient or frustrating?
- What tools or practices helped?
- What would you change for next week?

### Resolutions Section

The weekly journal template includes a "Resolutions" section:

**Who would you like to be?**

This is an identity-based goal-setting question. Instead of "what do I want to do?", ask "who do I want to be?"

**Examples:**
- "Someone who proactively documents complex decisions"
- "A developer who ships working code confidently"
- "A teammate who unblocks others generously"
- "An engineer who balances perfectionism with pragmatism"
- "A professional who maintains work-life boundaries"

Then derive specific resolutions from that identity:
- To be a proactive documenter: "Write ADR for every significant architectural choice"
- To ship confidently: "Expand test coverage to 80% before deploying"
- To unblock others: "Respond to all code reviews within 24 hours"

## Reflection Prompts by Situation

### After Completing a Major Project

- What was the hardest part? How did you overcome it?
- What would you do differently if starting over?
- What did you learn about your strengths and weaknesses?
- What systems or skills developed through this work?
- How did you grow as an engineer or professional?

### After Missing a Deadline or Goal

- What factors were within your control? What weren't?
- What did you learn about estimation or planning?
- How could you have communicated the risk earlier?
- What support or resources would have helped?
- How will you approach similar situations differently?

### After a Difficult Interaction

- What was the underlying need or concern (yours and theirs)?
- What assumptions did you make?
- How could you have communicated more clearly?
- What can you learn about this person's working style?
- What boundaries or norms need to be established?

### When Feeling Stuck or Unmotivated

- What's actually draining your energy?
- What would you work on if you had complete freedom?
- When was the last time you felt energized? What were you doing?
- What small win could you achieve today?
- Who could you talk to for perspective?

### When Feeling Overwhelmed

- What can you delegate or defer?
- What's actually urgent vs. just loud?
- What's the minimum viable version?
- Who can you ask for help?
- What would you tell a friend in this situation?

## Integration with Daisy System

### End-of-Day Workflow

1. **Review completed tasks** in today.md - Check off `[x]` items
2. **Review log entries** - What actually happened today?
3. **Fill retrospective bullets** - Use the three questions
4. **Identify tomorrow's focus** - What's most important?

### End-of-Week Workflow

1. **Archive completed tasks** - Run "start a new week" to move done tasks
2. **Review week's journal entries** - Skim through the week in journal.md
3. **Identify patterns** - What themes emerged?
4. **Set weekly resolutions** - Who do you want to be this week?
5. **Plan week ahead** - What are the key priorities?

## Best Practices

- **Be honest** - Your journal is for you, not performance reviews
- **Be specific** - "Improved testing" is less useful than "Added 15 integration tests"
- **Be constructive** - Frame challenges as learning opportunities
- **Be consistent** - Daily reflection builds self-awareness over time
- **Be kind to yourself** - Progress isn't linear, some days are just hard
- **Be forward-looking** - Retrospectives inform action, not just recording
