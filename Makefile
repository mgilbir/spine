.PHONY: fetch test lint

fetch:
	bash testdata/fetch.sh

test: fetch
	go test ./... -count=1

lint:
	golangci-lint run ./...
