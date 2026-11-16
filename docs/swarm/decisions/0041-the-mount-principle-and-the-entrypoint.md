# 0041. The mount principle and the entrypoint

Moved verbatim from `docs/swarm.md` section 14, item 41, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The mount principle and the entrypoint** (section 8) replace
revision 2's "no home directory". The passwd file is writable by any
uid so the entrypoint can add an entry; `no-new-privileges` keeps a
setuid binary from making that root. Alternative: `nss_wrapper`
through `LD_PRELOAD`, which a statically linked binary ignores.
