# 0030. Adapter additions

Moved verbatim from `docs/swarm.md` section 14, item 30, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Adapter additions.** The step named `PullComments`, `ReviewComments`,
`Pulls` (replacing `PullForIssue`, which read only the one open pull
request that closed the issue), `Merge` and a state for `Issues`. Two
more follow from the step's own text: `CloseIssue`, since "after the
last it closes the issue" is a forge call and every forge call is a
method; and `Login`, since `trusted` ends with the swarm's login, read
at `init`, and the marker's author check needs it in every verb that
reads threads. `Merge` takes the approved head as well as the pull
request, so a head pushed after the approvals is never merged.
Revision 4 adds `Board` (item 48), the sixteenth, and names a pull
request by its repository and number (`Ref`, item 49).
