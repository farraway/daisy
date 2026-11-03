# The swarm's base member image: what one role run executes in
# (docs/swarm.md, section 7). Specified at Step 26, revised at Step 38, and
# NOT YET BUILT OR RUN: Step 39 builds it first, by hand. It holds the two
# provider CLIs and git, and nothing the spec does not require. It is a
# base: a product's image starts `FROM daisy-member:1` and adds the
# product's toolchain and a warmed dependency cache, so the worker can run
# the product's `check` and Daisy can run it again in a container with no
# network (docs/swarm.md, section 7.2). The maintainer builds and tags that
# image and names it in swarm.json's `image`. No secret, token or key is in
# either image or its build context: a member receives its provider's key
# at `docker run` time as its one environment variable, and the check
# receives none (docs/swarm.md, section 8).
#
# Build, naming the CLI versions (recorded in the plan's Decisions):
#   docker build -t daisy-member:1 \
#     --build-arg CLAUDE_CODE_VERSION=<version> \
#     --build-arg CODEX_VERSION=<version> images/member

# Base: Node.js, because both provider CLIs are npm packages; Debian slim so
# git comes from the distribution. Pinned to an exact release; the first
# build appends the digest `docker pull` prints (FROM node:<tag>@sha256:<digest>).
FROM node:24.11.0-bookworm-slim

# git: the worker commits in /work, and every role reads the history and
# the diff there. ca-certificates: TLS roots for the codex CLI, a native
# binary that does not carry Node's bundled roots (to confirm at Step 39;
# dropped if codex reaches its API without it). --no-install-recommends
# keeps everything else out: no ssh client, since a member never reaches a
# git remote (apply pushes from the host).
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# The two provider CLIs (docs/swarm.md, section 9): `claude` for the
# anthropic adapter, `codex` for the openai adapter. Their versions are
# required build arguments with no default, so no image carries a version
# nobody chose.
ARG CLAUDE_CODE_VERSION
ARG CODEX_VERSION
RUN test -n "$CLAUDE_CODE_VERSION" && test -n "$CODEX_VERSION" \
 && npm install -g --no-fund --no-audit \
      "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
      "@openai/codex@${CODEX_VERSION}" \
 && npm cache clean --force

# The CLIs keep session state under $HOME. apply runs the container as the
# host user's uid (--user), which has no passwd entry here, so HOME is a
# directory any uid can write; it dies with the container (--rm). The
# updater is off because an ephemeral container's version is the image's.
ENV HOME=/home/member \
    DISABLE_AUTOUPDATER=1
RUN mkdir -m 1777 /home/member

# Never root, even when run without --user: the image's unprivileged user
# (claude refuses --permission-mode bypassPermissions as root).
USER node

# /work is where apply mounts the clone of the product repository, writable
# for every role (a reviewer's clone is a throwaway, never fetched).
WORKDIR /work
