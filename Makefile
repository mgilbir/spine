.PHONY: build vet fetch fetch-strict fetch-cc test test-race test-corpus \
	lint fuzz harvest-sweep harvest-batch

build:
	go build ./...

vet:
	go vet ./...

fetch:
	bash testdata/fetch.sh

fetch-strict:
	bash testdata/fetch.sh --strict

# Download the Common Crawl OOXML corpus (gitignored; see testdata/cc/README.md).
# The corpus test (go test ./cctest) skips when the corpus is absent.
fetch-cc:
	go run ./tools/ccfetch -manifest testdata/cc -out testdata/corpus/cc -n 1000

test: fetch
	go test ./... -count=1

# Race-detector run. -race multiplies RSS several-fold and the corpus tests
# hold whole Office files in memory, so this runs inside its own systemd
# scope with a hard memory cap: if the kernel OOM-kills the test, only this
# scope dies. Never run it bare in the terminal's cgroup — systemd's default
# OOMPolicy=stop then tears down the entire terminal scope on any OOM kill
# inside it (this took out a whole tmux pane on 2026-07-27; peak was 25.8G).
# GOMEMLIMIT makes each test binary GC hard before the cap (race shadow
# memory is outside GC accounting, hence the low value); -p bounds concurrent
# package test binaries; SPINE_CC_PARALLEL trims corpus-file concurrency,
# the dominant race-memory driver, and SPINE_CC_SUBSET trims the corpus
# subset (races surface from path coverage, not volume — the 60/type default
# blows a 20m timeout under race at low parallelism).
RACE_MEMMAX    ?= 12G
RACE_SWAPMAX   ?= 1G
RACE_GOMEM     ?= 5GiB
RACE_P         ?= 2
RACE_CC_PAR    ?= 2
RACE_CC_SUBSET ?= 12
test-race:
	systemd-run --user --scope -p MemoryMax=$(RACE_MEMMAX) -p MemorySwapMax=$(RACE_SWAPMAX) \
		env GOMEMLIMIT=$(RACE_GOMEM) SPINE_CC_PARALLEL=$(RACE_CC_PAR) SPINE_CC_SUBSET=$(RACE_CC_SUBSET) \
		go test -race -p $(RACE_P) ./... -count=1 -timeout 20m

# Full Common Crawl corpus run (~15-20 min; plain `go test ./cctest` checks a
# fast deterministic subset instead). Regenerate the quarantine after a fix
# wave with SPINE_CC_UPDATE_QUARANTINE=1 instead of SPINE_CC_FULL=1.
test-corpus:
	SPINE_CC_FULL=1 go test ./cctest -count=1 -timeout 45m

# --- Batched multi-crawl harvest (see testdata/cc/HARVEST.md) ---------------
#
# harvest-sweep regenerates the committed 10k/type reference manifests by
# sweeping several recent crawls and deduplicating across them. Cheap DuckDB
# work; expect heavy CDN throttling (the script retries patiently). Override
# the crawl list, target, or output dir via the vars below.
HARVEST_CRAWLS ?=
HARVEST_TARGET ?= 10000
harvest-sweep:
	bash testdata/cc/sweep-multi.sh -t $(HARVEST_TARGET) $(HARVEST_CRAWLS)

# harvest-batch processes ONE batch of references under a resource-capped
# systemd-run scope: a memory-blowing file is OOM-killed by the kernel (the
# worker is the largest process) while the lightweight orchestrator survives
# and records it. Loop this target while the ledger still has unprocessed rows
# (see the resume loop in testdata/cc/HARVEST.md). Everything is overridable.
HARVEST_MEMMAX  ?= 2G
HARVEST_CPU     ?= 200%
HARVEST_BATCH   ?= 2000
HARVEST_WORKERS ?= 2
HARVEST_TIMEOUT ?= 90s
HARVEST_LEDGER  ?= testdata/corpus/cc-batch/ledger.tsv
HARVEST_SCRATCH ?= testdata/corpus/cc-batch/scratch
HARVEST_QUARANTINE ?= testdata/cc/batch-quarantine.tsv
harvest-batch: build
	go build -o testdata/corpus/cc-batch/ccrun ./tools/ccrun
	systemd-run --user --scope -p MemoryMax=$(HARVEST_MEMMAX) -p CPUQuota=$(HARVEST_CPU) \
		testdata/corpus/cc-batch/ccrun \
		-manifest testdata/cc \
		-ledger $(HARVEST_LEDGER) \
		-quarantine $(HARVEST_QUARANTINE) \
		-scratch $(HARVEST_SCRATCH) \
		-batch $(HARVEST_BATCH) -workers $(HARVEST_WORKERS) -timeout $(HARVEST_TIMEOUT)

# Fuzz smoke run: discovers every fuzz target at run time (open-path plus
# write-path/API fuzzers) and runs each for a short fixed time, so a newly
# added target is picked up automatically. Deeper fuzzing is -fuzztime-driven;
# see CONTRIBUTING.md ("Fuzzing").
FUZZTIME ?= 30s
fuzz:
	@set -e; \
	pkgs=$$(git grep -l '^func Fuzz' -- '*_test.go' | xargs -n1 dirname | sort -u); \
	for pkg in $$pkgs; do \
		for target in $$(go test -list '^Fuzz' ./$$pkg | grep '^Fuzz'); do \
			echo "==> ./$$pkg $$target"; \
			go test ./$$pkg -run '^$$' -fuzz "^$${target}$$" -fuzztime $(FUZZTIME) || exit 1; \
		done; \
	done

# Requires golangci-lint v2.x; a v1 binary rejects .golangci.yml.
# Lint covers the whole module, including tools/ccfetch and cctest.
lint:
	golangci-lint run ./...
