.PHONY: build vet fetch fetch-strict test lint

build:
	go build ./...

vet:
	go vet ./...

fetch:
	bash testdata/fetch.sh

fetch-strict:
	bash testdata/fetch.sh --strict

test: fetch
	go test ./... -count=1

# Requires golangci-lint v2.x; a v1 binary rejects .golangci.yml.
lint:
	golangci-lint run ./...
