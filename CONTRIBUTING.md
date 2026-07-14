# Contributing

## Building and testing

```bash
make build         # go build ./...
make vet           # go vet ./...
make test          # fetches external fixtures (best-effort), then go test ./... -count=1
make lint          # golangci-lint run ./... — requires golangci-lint v2.x
```

The lint baseline is zero: `make lint` must exit clean before a change is
mergeable.

## Test fixtures

Round-trip tests run against real-world Office files that are not checked
in. `make fetch` downloads them (best-effort — unreachable fixtures are
skipped and the tests that need them skip silently); `make fetch-strict`
fails on any download error. A few fixtures have no known public URL and
always skip. See `testdata/README.md`, including how to obtain the
optional python-pptx fixture corpus.

## Fuzzing

Native Go fuzz targets cover the Open paths: `FuzzNewReader` and
`FuzzOpcMetadataXML` in `opc`, plus `FuzzOpenPptx`/`FuzzPptxSlideXML`,
`FuzzOpenDocx`/`FuzzDocxDocumentXML`, and
`FuzzOpenXlsx`/`FuzzXlsxWorksheetXML` in the format packages. Each opens
the fuzzed bytes as a package and, on success, walks a bounded slice of
the model and round-trips it (SaveBytes, then re-open). The `*XML`
variants pack the fuzzed bytes into the main part of an otherwise-valid
package so the XML parsers see hostile input directly instead of the
fuzzer having to invent whole zip archives. Errors are the expected
outcome on malformed input; any panic is a bug.

`make fuzz` is a short smoke run (30s per target; override with
`FUZZTIME=5m make fuzz`). Deeper fuzzing is `-fuzztime`-driven per
target, e.g.:

```bash
go test ./xlsx -run '^$' -fuzz '^FuzzXlsxWorksheetXML$' -fuzztime 30m
```

Seeds are generated in-process, so plain `go test` exercises them without
any fixtures. When the Common Crawl corpus is present (or
`SPINE_FUZZ_CORPUS` points at a directory with `docx`/`pptx`/`xlsx`
subdirectories), a handful of small real files are added as extra seeds
at runtime. Never commit corpus binaries or large seed files; minimized
crashers that Go writes under `testdata/fuzz/<Target>/` are tiny and
should be committed as regression seeds alongside the fix.

## Round-trip philosophy

The core invariant of this library is byte-identity for untouched
content: opening a document and saving it must reproduce every part
byte-for-byte, and mutating one part must not alter the bytes of any
other part. New features must come with round-trip tests that assert
this — mutate one thing, then compare every other part against the
original bytes.

A second invariant follows from it: **childOrder**. Parsed parts track
the order of their children (including unknown/raw-captured elements) so
they can be re-serialized in the original sequence. Every mutator that
appends to a typed slice on a parsed model must also maintain the
corresponding childOrder bookkeeping; a mutator that skips it will
serialize its element in the wrong place (or not at all) on files where
the element did not already exist.

## Commit style

One logical change per commit, message in the form
`pkg: imperative summary` (e.g. `xlsx: validate sheet renames`). When a
commit addresses an audit finding, cite its ID in the summary or body,
e.g. `(C76, C133)`.

## Audit reports

The findings cited from commit messages (C1–C235, D1–D30) live in
`docs/audits/`. The reports are historical records of the code at their
audit date; they are not updated as findings are fixed.
