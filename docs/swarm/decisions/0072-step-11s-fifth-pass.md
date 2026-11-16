# 0072. Step 11's fifth pass

Moved verbatim from `docs/swarm.md` section 14, item 72, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Step 11's fifth pass.** **`docker run`'s `--mount` fields are
CSV-quoted** (7.2), so a resource that resolves to a path holding a
comma (`d,src=/etc`) adds no mount option of its own; **a new
slice's branch leaves no host reflog either** (7.1 step 3); **init
takes back the recorded claims on the keys first**, then those with no
record, so a key gets back what a record says it held; **a
collection and the merge step read the issue's labels right before
each relabel** (section 5), the maintainer's move kept with the warning; **an orphan
check stopped at its timeout is said on stdout** (section 11); **a
value Go's JSON decoder cannot hold (`1e400`) is reported by its field**
(section 3).
