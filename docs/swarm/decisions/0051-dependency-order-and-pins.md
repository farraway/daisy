# 0051. Dependency order and pins

Moved verbatim from `docs/swarm.md` section 14, item 51, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Dependency order and pins** (from the same project: a consumer's
pointer moves only once the upstream change is reviewed and merged).
Chosen: slices are built in the plan's order, one at a time, the next
after the one before is merged (T13, T14); `# Earlier pull requests`
gives each merged slice's merge commit, and the worker's Task text
has a pin name it, or `BLOCKED:` when the upstream slice is not
merged (a slice the maintainer closed unmerged). Alternative: `apply`
refusing the launch, which needs a transition no run triggers. The
check runs without a network (7.2), so a check that fetches the newly
pinned upstream fails for that slice unless the product's image or
tree provides it; Step 30 of the plan that specified this document
records which (section 13, Established by the first real run).

**Amended by decision [169](0169-integration-control.md)** (R2, specified ahead of its build, Step 12 of the plan implementing the 2026-11-19 swarm roadmap): slice order stays dependency order within a job; between jobs, a `Depends:` declaration holds a job while an issue it names is open, and a slice's `Touches:` patterns hold its first build beside an overlapping open slice (`docs/swarm.md` section 12.1).
