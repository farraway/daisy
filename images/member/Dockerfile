# The swarm's base member image: what one role run executes in
# (docs/swarm.md, section 7). Specified at Step 26, revised at Steps 38 and
# 40; Step 39 built and ran it by hand. It holds the three provider CLIs,
# git and the entrypoint, and nothing the spec does not require. It is a
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
# grok's version is pinned below with its checksum, not a build argument.

# grok, the xai adapter's CLI (docs/swarm.md, section 9): no npm package,
# a static binary the vendor publishes per version and platform (its
# installer, https://x.ai/cli/install.sh, fetches the same file and checks
# no checksum, so the build checks this one, taken from the vendor's file
# and equal to the release the maintainer runs). x86_64 only: an arm64
# host needs the -linux-aarch64 file and its checksum. Fetched in a stage
# of its own so that the final image holds one copy, without the download
# layer. Updating grok is a change to this file: the version in the URL
# and the checksum together.
FROM node:24.11.0-bookworm-slim@sha256:76d0ed0ed93bed4f4376211e9d8fddac4d8b3fbdb54cc45955696001a3c91152 AS grok
ADD https://x.ai/cli/grok-1.0.70-linux-x86_64 /grok
RUN echo "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9  /grok" | sha256sum -c - \
 && chmod 0755 /grok

# Base: Node.js, because two of the provider CLIs are npm packages; Debian slim so
# git comes from the distribution. Pinned to an exact release and its
# digest, the one the build host's image cache records for the tag
# (`docker image inspect --format '{{index .RepoDigests 0}}'`), on which
# daisy-member:1 was built; the grok stage above names the same. Updating
# the base is a change to both FROM lines: the tag and the digest together.
FROM node:24.11.0-bookworm-slim@sha256:76d0ed0ed93bed4f4376211e9d8fddac4d8b3fbdb54cc45955696001a3c91152

# git: the worker commits in /work, and every role reads the history and
# the diff there. ca-certificates: TLS roots for the codex CLI, a native
# binary that does not carry Node's bundled roots (Step 39: it reached its
# API with them; a run without them was not tried), and for grok, native
# too. --no-install-recommends
# keeps everything else out: no ssh client, since a member never reaches a
# git remote (apply pushes from the host).
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# The two npm provider CLIs (docs/swarm.md, section 9): `claude` for the
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

# grok, from its stage above.
COPY --from=grok /grok /usr/local/bin/grok

# The CLIs keep session state under $HOME. apply runs the container as the
# host user's uid (--user), which may have no passwd entry here, so HOME is
# a directory any uid can write (the entrypoint sets it again), and the
# passwd file is one any uid can append its entry to (entrypoint.sh). The
# updater is off because an ephemeral container's version is the image's.
ENV HOME=/home/member \
    DISABLE_AUTOUPDATER=1 \
    GROK_DISABLE_AUTOUPDATER=1
RUN mkdir -m 1777 /home/member && chmod 0666 /etc/passwd
# COPY then chmod, not COPY --chmod, which needs BuildKit: the file builds
# under the legacy builder too (a Docker without buildx; Step 5's smoke).
COPY entrypoint.sh /usr/local/bin/member-entrypoint
RUN chmod 0755 /usr/local/bin/member-entrypoint
ENTRYPOINT ["/usr/local/bin/member-entrypoint"]

# Never root, even when run without --user: the image's unprivileged user
# (claude refuses --permission-mode bypassPermissions as root).
USER node

# /work is where apply mounts the clone of the product repository, writable
# for every role (a reviewer's clone is a throwaway, never fetched).
WORKDIR /work
