.PHONY: build vet fetch fetch-strict fetch-cc test test-corpus lint

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

# Requires golangci-lint v2.x; a v1 binary rejects .golangci.yml.
# Lint covers the whole module, including tools/ccfetch and cctest.
lint:
	golangci-lint run ./...
