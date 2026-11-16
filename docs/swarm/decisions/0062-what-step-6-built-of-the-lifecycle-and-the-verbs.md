# 0062. What Step 6 built of the lifecycle and the verbs

Moved verbatim from `docs/swarm.md` section 14, item 62, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**What Step 6 built of the lifecycle and the verbs.** A slot's login
is checked at the launch that takes it, as *slot refused*, not at
`apply`'s start: the Step 5 smoke had `apply --once` refuse a pass
that launched only the architect, an `anthropic` persona, because the
`openai` slot had no login; the check belongs where the slot is
needed, and section 8's slot rules already make a bad slot that
run's fault. The alternative, the start refusing only for the
providers the pass would launch, needs the pass's launches before
the pass. A worker run that ends in a swarm fault with commits keeps
them as its ref (7.6, step 2), as a failure does: the fault relaunches
the role from `base` or the branch, and the work would otherwise be
gone with `<run>/work`. The batch run records `propose <p>` (section
2), since its pacing can change before a retried collection. Two
faults in a row pause the swarm only when this process collected the
second (11, As built). The drain after a signal starts a worker run's
check, the push and the post depending on it. `Ops.Signals` is the
loop's one new effect. The member image's Dockerfile copies the
entrypoint and then sets its mode (`COPY` then `RUN chmod 0755`), not
`COPY --chmod`, which only BuildKit builds (the Step 5 smoke's
Docker had no buildx).
