GO ?= go
COVERAGE_THRESHOLD ?= 95
GOFMT_TARGETS := cmd internal
CACHE_DIR ?= $(CURDIR)/.cache

export GOCACHE ?= $(CACHE_DIR)/go-build
export GOMODCACHE ?= $(CACHE_DIR)/gomod
export GOPATH ?= $(CACHE_DIR)/gopath
export XDG_CACHE_HOME ?= $(CACHE_DIR)/xdg

.PHONY: build fmt fmt-check lint typecheck deadcode test coverage check

build:
	$(GO) build -o git-real ./cmd/git-real

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
	$(GO) tool deadcode ./cmd/git-real

test:
	$(GO) test ./...

coverage:
	COVERAGE_THRESHOLD=$(COVERAGE_THRESHOLD) bash ./scripts/check-coverage.sh

check: fmt-check lint typecheck deadcode coverage
