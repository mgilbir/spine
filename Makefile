.PHONY: build vet fetch fetch-strict fetch-cc test test-corpus lint fuzz \
	harvest-sweep harvest-batch

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

# Full Common Crawl corpus run (~15-20 min; plain `go test ./cctest` checks a
# fast deterministic subset instead). Regenerate the quarantine after a fix
# wave with SPINE_CC_UPDATE_QUARANTINE=1 instead of SPINE_CC_FULL=1.
test-corpus:
	SPINE_CC_FULL=1 go test ./cctest -count=1 -timeout 45m

# --- Batched multi-crawl harvest (see testdata/cc/README.md) ---------------
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
# (see the resume loop in testdata/cc/README.md). Everything is overridable.
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

# Fuzz smoke run: every fuzz target for a short fixed time. Deeper fuzzing is
# -fuzztime-driven; see CONTRIBUTING.md ("Fuzzing").
FUZZTIME ?= 30s
fuzz:
	go test ./opc -run '^$$' -fuzz '^FuzzNewReader$$' -fuzztime $(FUZZTIME)
	go test ./opc -run '^$$' -fuzz '^FuzzOpcMetadataXML$$' -fuzztime $(FUZZTIME)
	go test ./pptx -run '^$$' -fuzz '^FuzzOpenPptx$$' -fuzztime $(FUZZTIME)
	go test ./pptx -run '^$$' -fuzz '^FuzzPptxSlideXML$$' -fuzztime $(FUZZTIME)
	go test ./docx -run '^$$' -fuzz '^FuzzOpenDocx$$' -fuzztime $(FUZZTIME)
	go test ./docx -run '^$$' -fuzz '^FuzzDocxDocumentXML$$' -fuzztime $(FUZZTIME)
	go test ./xlsx -run '^$$' -fuzz '^FuzzOpenXlsx$$' -fuzztime $(FUZZTIME)
	go test ./xlsx -run '^$$' -fuzz '^FuzzXlsxWorksheetXML$$' -fuzztime $(FUZZTIME)

# Requires golangci-lint v2.x; a v1 binary rejects .golangci.yml.
# Lint covers the whole module, including tools/ccfetch and cctest.
lint:
	golangci-lint run ./...
