GO ?= go
FUZZTIME ?= 30s
BENCH ?= .
BENCHPKG ?= ./demux/

.PHONY: build vet fmt fmt-check test test-race golden bench fuzz ci

build:
	$(GO) build ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

test:
	$(GO) test -count=1 ./...

test-race:
	$(GO) test -race -count=1 ./...

# unit tests + the digest differential against real assets (owner machine)
golden:
	@test -n "$$GOLDEN_TS_DIR" || { echo "GOLDEN_TS_DIR is not set"; exit 1; }
	$(GO) test -count=1 ./...

bench:
	$(GO) test -run '^$$' -bench '$(BENCH)' -benchmem -count 5 $(BENCHPKG)

# runs every Fuzz* target for FUZZTIME each
fuzz:
	@for pkg in $$($(GO) list ./...); do \
		for f in $$($(GO) test -list '^Fuzz' $$pkg | grep '^Fuzz' || true); do \
			echo "--- $$pkg $$f"; \
			$(GO) test -run '^$$' -fuzz "^$$f$$" -fuzztime $(FUZZTIME) $$pkg || exit 1; \
		done; \
	done

ci: fmt-check build vet test-race
