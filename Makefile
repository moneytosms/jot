BINARY  := jot
CMD     := ./cmd/jot
BINDIR  ?= $(HOME)/.local/bin
GO      ?= go

# No git metadata (for example a source tarball) falls back to a dev version.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test vet fmt check install uninstall clean help

all: build

## build: compile bin/jot
build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)
	@echo "built bin/$(BINARY) ($(VERSION))"

## test: run the test suite
test:
	$(GO) test ./...

## vet: run go vet
vet:
	$(GO) vet ./...

## fmt: format every Go file
fmt:
	gofmt -w .

## check: vet + tests, the same checks CI would run
check: vet test

## install: build and copy the binary into BINDIR (default ~/.local/bin)
install: build
	mkdir -p $(BINDIR)
	install -m 0755 bin/$(BINARY) $(BINDIR)/$(BINARY)
	@echo "installed $(BINDIR)/$(BINARY)"
	@echo "run \`jot setup\` if this is your first install"

## uninstall: remove the binary (journal data is left alone)
uninstall:
	rm -f $(BINDIR)/$(BINARY)
	@echo "removed $(BINDIR)/$(BINARY); journal data is untouched"

## clean: remove build output
clean:
	rm -rf bin dist jot-export

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
