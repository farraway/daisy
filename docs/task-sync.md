# Task Synchronization Rules

**`tasks/todo.txt` is the canonical source of truth for all tasks.**

## Bidirectional Sync Requirements

**ANY task change must update BOTH `todo.txt` and `today.md`:**

1. **Adding a new task:**
   - `daisy add "<description>" A|B|C|D|none` adds it to `todo.txt` with its creation date and, when it is (A) or (B), to `today.md`

2. **Completing a task:**
   - Mark [x] in `today.md`
   - Update in `todo.txt`: strip priority, add `x YYYY-MM-DD` prefix, move to end

3. **Changing priority:**
   - `daisy priority "<pattern>" A|B|C|D|none` changes it in `todo.txt` and moves the task to its section in `today.md`, or out of it

4. **Changing due dates:**
   - Update `due:YYYY-MM-DD` in `todo.txt`
   - Reflect urgency in `today.md` (add **OVERDUE** flag if past due)

5. **Starting a new day:**
   - Pull high-priority (A) and (B) tasks from `todo.txt` → `today.md`
   - Archive previous `today.md` → `journal.md`

## Common Sync Issues

**Priority mismatches:**
- Task is (A) in `today.md` but (B) in `todo.txt`
- **Fix:** `todo.txt` wins: move the task to its section in `today.md`. When `today.md` has the intended priority, `daisy priority "<pattern>" A|B|C|D|none`
- **Cause:** Priority changed in one file but not the other

**Completion status mismatches:**
- Task marked [x] in `today.md` but still active in `todo.txt`
- **Fix:** Update `todo.txt` with `x YYYY-MM-DD` prefix and move to end
- **Cause:** "done" command only updated one file

**Missing tasks:**
- Task exists in `todo.txt` but not in `today.md`
- **Fix:** An open (A) or (B) task belongs in `today.md` (`prompts/daisy.md` rule 2): add it to its section (Now, Next, or GitHub PRs for one with `@git`). Any other may be absent: "new day" brings in an inbox task and one with `@git`, and leaves out a (C) or (D) without `@git`
- **Cause:** `todo.txt` was edited without `today.md`

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
   → todo.txt wins: (B) in today.md (or, if (A) is intended: daisy priority "Certificate training" A)

2. Completion mismatch:
   today.md: [x] PagerDuty migration
   todo.txt: (A) PagerDuty migration (still active)
   → Should be completed in todo.txt

Fix these automatically?
```
