# Daisy — the Go binary. `make build` is what `daisy install` needs;
# `make check` is the gate before a root commit; `make test` is `daisy
# test` (build, go test, then the acceptance suite) run against this root;
# `make install-dev` installs this root as its remote has it, checked.
GO ?= go
BIN := bin/daisy

.PHONY: build test check clean install-dev

build:
	$(GO) build -o $(BIN) ./cmd/daisy

test: build
	DAISY_ROOT="$(CURDIR)" $(BIN) test

check: build
	@test -z "$$(gofmt -l .)" || { echo "gofmt: these files need formatting:"; gofmt -l .; exit 1; }
	$(GO) vet ./...
	$(GO) mod tidy -diff
	$(GO) test -race ./...

clean:
	rm -rf bin

# install-dev is the one install command, run from anywhere as `make -C
# <root> install-dev`: a fast-forward pull, the build (in Go's cache when
# it is writable, else in $TMPDIR/gocache, and says so), `daisy install
# --update`, then a failure unless ~/bin/daisy's `daisy version` is the
# root's HEAD exactly, so neither a build that left an old binary in place
# nor one from a tree with changes (`<sha> (modified)`) can pass.
install-dev:
	git -C "$(CURDIR)" pull --ff-only
	@cache="$$($(GO) env GOCACHE)"; \
	if ! { mkdir -p "$$cache" && test -w "$$cache"; } 2>/dev/null; then \
		cache="$${TMPDIR:-/tmp}/gocache"; \
		echo "install-dev: Go's build cache is not writable here; building with GOCACHE=$$cache"; \
	fi; \
	GOCACHE="$$cache" $(MAKE) --no-print-directory build
	DAISY_ROOT="$(CURDIR)" "$(CURDIR)/$(BIN)" install --update
	@head="$$(git -C "$(CURDIR)" rev-parse HEAD)" || exit 1; \
	got="$$("$$HOME/bin/daisy" version)" || { echo "Error: ~/bin/daisy reports no commit (above); the root's HEAD is $$head" >&2; exit 1; }; \
	changes="$$(git -C "$(CURDIR)" status --porcelain)" || exit 1; \
	if [ "$$got" = "$$head" ]; then \
		echo "install-dev: ~/bin/daisy is $$head, the root's HEAD"; \
	elif [ "$$got" = "$$head (modified)" ] && [ -n "$$changes" ]; then \
		echo "Error: ~/bin/daisy is $$head built from a tree with changes HEAD does not hold, so it may not be HEAD's code:" >&2; \
		printf '%s\n' "$$changes" >&2; \
		echo "  Remove them or set them aside (git stash -u; an untracked file counts), then re-run 'make -C $(CURDIR) install-dev'" >&2; exit 1; \
	else \
		echo "Error: ~/bin/daisy is $$got, but the root's HEAD is $$head: the build did not install HEAD (see above)" >&2; exit 1; \
	fi
