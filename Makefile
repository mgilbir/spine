.PHONY: build vet fetch fetch-strict fetch-cc test test-race test-corpus \
	lint fuzz fuzz-run cover cover-dark harvest-sweep harvest-batch

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
#
# Like test-race, this runs inside its own memory-capped systemd scope: it is
# the other memory-hungry pass CONTRIBUTING.md requires to be capped (thousands
# of wild Office files, several held in memory at once, some of which decompress
# very large). Without the scope an OOM kill lands in the terminal's own cgroup
# and systemd's default OOMPolicy=stop tears the whole terminal down. No
# GOMEMLIMIT squeeze here — that value exists to compensate for race shadow
# memory, which is outside GC accounting; a non-race run does not need it.
CORPUS_MEMMAX  ?= 12G
CORPUS_SWAPMAX ?= 1G
test-corpus:
	systemd-run --user --scope -p MemoryMax=$(CORPUS_MEMMAX) -p MemorySwapMax=$(CORPUS_SWAPMAX) \
		env SPINE_CC_FULL=1 go test ./cctest -count=1 -timeout 45m

# --- Coverage ---------------------------------------------------------------
#
# cover measures the whole module against the whole module (-coverpkg=./...),
# not each package against its own tests. Most of this library is exercised
# across package boundaries — cctest drives docx/xlsx/pptx over real documents,
# internal/symmetry drives all three — so per-package coverage understates it
# badly and points at the wrong gaps. Set SPINE_CC_FULL=1 (the default here) to
# include the corpus, which is what makes "never executed" mean never.
#
# It runs in a memory-capped scope for the same reason test-corpus does, and
# takes appreciably longer than a plain run because every package is
# instrumented.
#
# Treat the number as a *finder*, not a target. A test written to turn a line
# green is worse than no test: it costs maintenance and asserts nothing. What
# the profile is good for is pointing at code where a bug could exist today and
# nothing would notice — see CONTRIBUTING.md.
COVER_MEMMAX  ?= 14G
COVER_SWAPMAX ?= 2G
COVER_OUT     ?= coverage.out
cover:
	systemd-run --user --scope -p MemoryMax=$(COVER_MEMMAX) -p MemorySwapMax=$(COVER_SWAPMAX) \
		env SPINE_CC_FULL=1 go test ./... -coverpkg=./... -coverprofile=$(COVER_OUT) \
		-covermode=set -timeout 45m
	@go tool cover -func=$(COVER_OUT) | tail -1

# cover-dark lists the functions no test executes at all, which is the useful
# view. Library code only: examples/ and tools/ are demo and CLI programs whose
# coverage says nothing about the library.
cover-dark: $(COVER_OUT)
	@go tool cover -func=$(COVER_OUT) \
		| awk '$$NF == "0.0%"' \
		| grep -vE 'spine/(examples|tools)/' \
		| sed 's|github.com/mgilbir/spine/||' \
		| awk '{printf "%-64s %s\n", $$1, $$2}' \
		| sort
	@go tool cover -func=$(COVER_OUT) | awk '$$NF == "0.0%"' | grep -vcE 'spine/(examples|tools)/' \
		| xargs -I{} echo "-- {} library functions never executed"

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
#
# OOMPolicy=continue is load-bearing, and is the one place in this Makefile
# where the test-race reasoning inverts. systemd's default (DefaultOOMPolicy=
# stop) kills the WHOLE unit when the kernel OOM-kills any process in its
# cgroup — which for test-race is the containment we want, but here would kill
# the orchestrator along with the worker. The orchestrator records the kill
# AFTER the worker dies (ledger.Append on outcome resource/killed), so a stopped
# scope means the offending file is never ledgered and never counted as an
# attempt; the resume loop then re-selects the same file on every subsequent
# batch. That is a permanent livelock on one bad file, not a quarantine row.
# With continue, only the worker dies and the design in tools/ccrun's package
# doc ("one quarantine row, not a dead batch") actually holds. (C389)
#
# Measured on systemd 255.4 with a scope whose child blows a 128M MemoryMax:
# default policy -> the scope leader is SIGTERMed and never reaches its
# post-worker line (exit 143); with OOMPolicy=continue -> the leader observes
# the worker's 137 and carries on (exit 0).
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
		-p OOMPolicy=continue \
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
#
# Capped for the same reason as test-race, and more urgently: fuzzing *searches*
# for pathological input, so finding one is the success case — and for the
# parsers these targets drive, the historical failure mode is a single huge
# allocation. C360 sized a slice from an unvalidated CFB header field: a
# 512-byte file asked for 16 GiB, and buildFAT then appends into that slice, so
# the pages are really faulted in rather than merely reserved. Measured on a
# regression: 20.9 GB anon-RSS.
#
# Uncapped, that lands in the terminal's own cgroup, and on a machine with less
# RAM than the allocation it is a *global* OOM (CONSTRAINT_NONE) rather than a
# cgroup one. systemd's DefaultOOMPolicy=stop then tears down the whole terminal
# scope — observed, with a tmux session losing 16h of accumulated work. Inside
# this scope the same regression is contained: the kernel kills the test in its
# own cgroup and the scope alone fails with 'oom-kill'.
#
# The cap is deliberately far below the machine: a runaway should die fast and
# attributably rather than swap the box. Legitimate fuzz inputs are small.
#
# fuzz-run is the uncapped loop, exposed for environments that already impose a
# limit (CI containers) — invoking it directly outside one is what this target
# exists to prevent.
FUZZTIME ?= 30s
FUZZ_MEMMAX  ?= 4G
FUZZ_SWAPMAX ?= 0
fuzz:
	systemd-run --user --scope -p MemoryMax=$(FUZZ_MEMMAX) -p MemorySwapMax=$(FUZZ_SWAPMAX) \
		$(MAKE) fuzz-run FUZZTIME=$(FUZZTIME) FUZZ_PKGS="$(FUZZ_PKGS)"

# FUZZ_PKGS restricts the sweep to one or more packages, which is what lets CI
# run each package as its own job: the targets are heavy enough that sharing a
# single wall-clock budget across all of them leaves each with seconds. Empty
# means every package that has a target.
FUZZ_PKGS ?=
fuzz-run:
	@set -e; \
	pkgs="$(FUZZ_PKGS)"; \
	if [ -z "$$pkgs" ]; then \
		pkgs=$$(git grep -l '^func Fuzz' -- '*_test.go' | xargs -n1 dirname | sort -u); \
	fi; \
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
