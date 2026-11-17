# 0147. Screenshots reach the pull request and the reviewers

From Step 38 of the plan remediating the 2026-11-15 design review (talk A16, 2026-11-17; wording approved by the maintainer 2026-11-17).

**A worker that changes a page saves screenshots, which Daisy attaches
to the pull request and gives the reviewers (Step 38 of the plan
remediating the 2026-11-15 design review, 2026-11-17; talk A16).** The
reviewers judged a change to a page from its diff alone, and the
maintainer read the pull request the same way: what the page shows was
seen by no one before the merge. **Every worker's Task text** asks for a
screenshot of each page the change alters, saved under
`/work/.swarm/artifacts/` (section 7.4); the worker alone knows whether
its change alters a page, so the sentence is always there and the
member decides, rather than a declaration in `swarm.json` that the
maintainer would have to keep true. How the product is served and a page
rendered is the product's: the image holds the browser and the
repository notes say how, so Daisy names neither. **The collection**
reads the directory as untrusted, since the member wrote it (section
7.6, The screenshots): `Lstat` on the directory and on each entry, so no
link is followed and no subdirectory descended; a name of one path
element from a fixed character set that cannot begin with `.` or `-`;
the content sniffed as its extension's image type; the member's secrets
searched for, as in `out.md`. Images only (PNG, JPEG, WebP): what the
reviewers are asked to look at, and what Gitea's default
`ALLOWED_TYPES` lets through; a skipped entry is warned of and recorded,
never a failure, since a screenshot is evidence beside the work, not
the work. **The upload is a forge-seam method**, `Attach(pr, name,
data)` (section 10), as every forge call is: Gitea's
`issueCreateIssueAttachment`, a pull request being an issue there, and
the Fake's. A refusal by the forge (its `413` beyond `MAX_SIZE`, `422`
outside `ALLOWED_TYPES`, `404` with attachments off) is `ErrNotAttached`,
recorded and warned of, never a failure. **GitHub** has no endpoint that
uploads a file to an issue or a pull request: a release asset would need
a release of the product's own, and a commit would write the declared
repository's tree, which Daisy never does (section 2). Its adapter, when
built, answers `Attach` with `ErrNotAttached`, so on GitHub the
screenshots reach the reviewers from the host and the pull request does
not show them; that is stated rather than worked around. **The
reviewers' prompt** lists the screenshots of the worker run that pushed
the head under `# Artifacts`, path and URL, and each file is copied into
the reviewer's own `/work/.swarm/artifacts/`: a member holds no forge
credential (section 8), so a URL on a private repository is no image to
it, while a file in its mount is one its CLI can open. Its Task text
adds a sentence only when the section is there, so a run without
screenshots reads as before. **No bound of Daisy's own** is set on the
files' number or size: a limit is a process rule, the maintainer's to
set; the forge's own limits refuse a file. Alternatives: a `swarm.json`
flag per repository naming it a product with pages (a declaration to
keep true, and a worker of a library that renders an example page would
be told nothing); listing the URLs alone (the reviewers could not open
them); mounting the worker run's directory into the reviewer's container
(a new mount, section 7.2, for files a copy delivers through the mount
the run has); attaching the files to the worker's comment
(`/issues/comments/{id}/assets`, which needs the comment's id that
`Comment` does not return); any file type (a reviewer is asked to look at
a page). Tests: `TestSwarmWorkerArtifactsReachThePullAndTheReviewers`,
`TestSwarmWorkerWithoutArtifacts`, `TestSwarmWorkerArtifactsUntrusted`,
`TestSwarmWorkerArtifactsDirectoryLink`, `TestSwarmWorkerArtifactRefused`,
`TestSwarmApplyWorkFailures` (a secret in a screenshot) and
`TestGiteaAttach`.
