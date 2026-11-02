# Daisy — the Go binary. `make build` is what `daisy install` needs;
# `make check` is the gate before a root commit; `make test` is `daisy
# test` (build, go test, then the acceptance suite) run against this root.
GO ?= go
BIN := bin/daisy

.PHONY: build test check clean

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
