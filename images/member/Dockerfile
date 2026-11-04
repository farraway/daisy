# The swarm's base member image: what one role run executes in
# (docs/swarm.md, section 7). Specified at Step 26, revised at Steps 38 and
# 40; Step 39 built and ran it by hand. It holds the two provider CLIs, git
# and the entrypoint, and nothing the spec does not require. It is a
# base: a product's image starts `FROM daisy-member:1` and adds the
# product's toolchain and a warmed dependency cache, so the worker can run
# the product's `check` and Daisy can run it again in a container with no
# network (docs/swarm.md, section 7.2). The maintainer builds and tags that
# image and names it in swarm.json's `image`, keeping this entrypoint. No
# secret, token or key is in either image or its build context: a member
# receives its credential at `docker run` time, one environment variable
# or one mounted login slot, and the check receives none (docs/swarm.md,
# section 8).
#
# Build, naming the CLI versions (recorded in the plan's Decisions):
#   docker build -t daisy-member:1 \
#     --build-arg CLAUDE_CODE_VERSION=<version> \
#     --build-arg CODEX_VERSION=<version> images/member

# Base: Node.js, because both provider CLIs are npm packages; Debian slim so
# git comes from the distribution. Pinned to an exact release; the next
# build appends the digest `docker pull` prints (FROM node:<tag>@sha256:<digest>),
# which Step 39's build did not record.
FROM node:24.11.0-bookworm-slim

# git: the worker commits in /work, and every role reads the history and
# the diff there. ca-certificates: TLS roots for the codex CLI, a native
# binary that does not carry Node's bundled roots (Step 39: it reached its
# API with them; a run without them was not tried). --no-install-recommends
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
# host user's uid (--user), which may have no passwd entry here, so HOME is
# a directory any uid can write (the entrypoint sets it again), and the
# passwd file is one any uid can append its entry to (entrypoint.sh). The
# updater is off because an ephemeral container's version is the image's.
ENV HOME=/home/member \
    DISABLE_AUTOUPDATER=1
RUN mkdir -m 1777 /home/member && chmod 0666 /etc/passwd
COPY --chmod=0755 entrypoint.sh /usr/local/bin/member-entrypoint
ENTRYPOINT ["/usr/local/bin/member-entrypoint"]

# Never root, even when run without --user: the image's unprivileged user
# (claude refuses --permission-mode bypassPermissions as root).
USER node

# /work is where apply mounts the clone of the product repository, writable
# for every role (a reviewer's clone is a throwaway, never fetched).
WORKDIR /work
