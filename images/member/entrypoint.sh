#!/bin/sh
# The member image's entrypoint (docs/swarm.md, section 7.2): it runs
# before every member and every check, as whatever uid `docker run --user`
# gave, then execs the argv it was given as the container's first process.
# It holds no credential logic: a mounted slot is named the CLI's home
# (CODEX_HOME, GROK_HOME) by `docker run --env`, not here.
set -eu

# A directory any uid can write; it dies with the container (--rm).
export HOME=/home/member

# The CLIs and git look the user up; a host uid the image lacks gets an
# entry (the file is writable for this; --security-opt no-new-privileges
# keeps a setuid binary from turning that into root).
if ! getent passwd "$(id -u)" >/dev/null; then
  printf 'member:x:%s:%s:Daisy swarm member:%s:/bin/sh\n' \
    "$(id -u)" "$(id -g)" "$HOME" >>/etc/passwd
fi

# A fallback identity for git outside the member's clone, whose own config
# names the role (section 7.1) and takes precedence.
git config --global user.name 'Daisy swarm'
git config --global user.email 'member@daisy-swarm.invalid'

exec "$@"
