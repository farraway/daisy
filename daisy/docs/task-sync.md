# Task Synchronization Rules

**`tasks/todo.txt` is the canonical source of truth for all tasks.**

## Bidirectional Sync Requirements

**ANY task change must update BOTH `todo.txt` and `today.md`:**

1. **Adding a new task:**
   - Add to `todo.txt` with priority (A/B/C/D), creation date, tags
   - Add to `today.md` if priority is (A) or (B) and working on it today

2. **Completing a task:**
   - Mark [x] in `today.md`
   - Update in `todo.txt`: strip priority, add `x YYYY-MM-DD` prefix, move to end

3. **Changing priority:**
   - Update priority (A)/(B)/(C)/(D) in `todo.txt`
   - Update section in `today.md` (Now, Next, Inbox)

4. **Changing due dates:**
   - Update `due:YYYY-MM-DD` in `todo.txt`
   - Reflect urgency in `today.md` (add **OVERDUE** flag if past due)

5. **Starting a new day:**
   - Pull high-priority (A) and (B) tasks from `todo.txt` → `today.md`
   - Archive previous `today.md` → `journal.md`

## Common Sync Issues

**Priority mismatches:**
- Task is (A) in `today.md` but (B) in `todo.txt`
- **Fix:** Update `todo.txt` to match intended priority
- **Cause:** Priority changed in one file but not the other

**Completion status mismatches:**
- Task marked [x] in `today.md` but still active in `todo.txt`
- **Fix:** Update `todo.txt` with `x YYYY-MM-DD` prefix and move to end
- **Cause:** "done" command only updated one file

**Missing tasks:**
- Task exists in `todo.txt` but not in `today.md`
- **Fix:** Usually intentional (lower priority not pulled into today)
- **Cause:** Only (A) and (B) tasks are extracted during "new day"

## Sync Validation Algorithm

**Command:** "sync tasks" or "check sync"

```
1. Compare tasks in today.md vs todo.txt
2. For each task in today.md:
   a. Find matching task in todo.txt (by description substring)
   b. Check priority matches (section vs prefix)
   c. Check completion status matches (checkbox vs x prefix)
   d. Report discrepancies
3. Offer to synchronize automatically:
   - "Found 3 mismatches. Fix automatically?"
```

**Example Output:**
```
⚠️ Task sync issues found:

1. Priority mismatch:
   today.md: (A) Certificate training
   todo.txt: (B) Certificate training
   → Should be (A) in todo.txt

2. Completion mismatch:
   today.md: [x] PagerDuty migration
   todo.txt: (A) PagerDuty migration (still active)
   → Should be completed in todo.txt

Fix these automatically?
```
