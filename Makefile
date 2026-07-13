.PHONY: build vet fetch fetch-strict fetch-cc test lint

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

# Requires golangci-lint v2.x; a v1 binary rejects .golangci.yml.
# Lint covers the whole module, including tools/ccfetch and cctest.
lint:
	golangci-lint run ./...
