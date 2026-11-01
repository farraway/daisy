# Daisy — the Go binary. `make build` is what `daisy test` and `daisy
# install` need; `make check` is the gate before a root commit.
GO ?= go
BIN := bin/daisy

.PHONY: build test check clean

build:
	$(GO) build -o $(BIN) ./cmd/daisy

test:
	$(GO) test -race ./...

check: build
	@test -z "$$(gofmt -l .)" || { echo "gofmt: these files need formatting:"; gofmt -l .; exit 1; }
	$(GO) vet ./...
	$(GO) mod tidy -diff
	$(GO) test -race ./...

clean:
	rm -rf bin
