GO ?= go
COVERAGE_THRESHOLD ?= 95
GOFMT_TARGETS := cmd internal
MAIN_PKG := ./cmd/git-real
CACHE_DIR ?= $(CURDIR)/.cache

export GOCACHE ?= $(CACHE_DIR)/go-build
export GOMODCACHE ?= $(CACHE_DIR)/gomod
export GOPATH ?= $(CACHE_DIR)/gopath
export XDG_CACHE_HOME ?= $(CACHE_DIR)/xdg

# Reproducible-build inputs. Override on the command line for releases:
#   make build VERSION=v1.2.3 COMMIT=$(git rev-parse HEAD) DATE=$(git log -1 --format=%cI)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse HEAD 2>/dev/null || echo none)
DATE    ?= $(shell git log -1 --format=%cI 2>/dev/null || echo unknown)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: build fmt fmt-check lint typecheck deadcode test test-race coverage vuln actionlint check

build:
	$(GO) build -trimpath -buildvcs=true -ldflags='$(LDFLAGS)' -o git-real $(MAIN_PKG)

fmt:
	$(GO) fmt ./...

fmt-check:
	files="$$(find $(GOFMT_TARGETS) -name '*.go' -print)"; \
	if [ -z "$$files" ]; then \
		exit 0; \
	fi; \
	unformatted="$$(gofmt -l $$files)"; \
	if [ -n "$$unformatted" ]; then \
		printf '%s\n' "$$unformatted"; \
		exit 1; \
	fi

lint:
	$(GO) vet ./...
	$(GO) tool staticcheck ./...

typecheck:
	$(GO) test -run '^$$' ./...

deadcode:
	$(GO) tool deadcode $(MAIN_PKG)

test:
	$(GO) test ./...

test-race:
	$(GO) test -race -shuffle=on ./...

coverage:
	COVERAGE_THRESHOLD=$(COVERAGE_THRESHOLD) bash ./scripts/check-coverage.sh

vuln:
	$(GO) tool govulncheck ./...

actionlint:
	$(GO) tool actionlint

check: fmt-check lint typecheck deadcode coverage
