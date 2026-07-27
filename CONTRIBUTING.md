# Contributing

## Building and testing

```bash
make build         # go build ./...
make vet           # go vet ./...
make test          # fetches external fixtures (best-effort), then go test ./... -count=1
make test-race     # go test -race, wrapped in a memory-capped systemd scope
make lint          # golangci-lint run ./... — requires golangci-lint v2.x
```

The lint baseline is zero: `make lint` must exit clean before a change is
mergeable.

Memory-hungry runs (`-race`, full-corpus passes) must go through a
resource-capped `systemd-run --user --scope` — that is what `make test-race`
and `make harvest-batch` do. A bare run shares the terminal's cgroup, and one
kernel OOM kill there makes systemd (default `OOMPolicy=stop`) tear down the
whole terminal scope, killing your shell and everything in it. Inside a capped
scope the kill is contained to the test. Generic pattern for any heavy
one-off:

```bash
systemd-run --user --scope -p MemoryMax=12G -p MemorySwapMax=1G -- <command>
```

## Test fixtures

Round-trip tests run against real-world Office files that are not checked
in. `make fetch` downloads them (best-effort — unreachable fixtures are
skipped and the tests that need them skip silently); `make fetch-strict`
fails on any download error. A few fixtures have no known public URL and
always skip. See `testdata/README.md`, including how to obtain the
optional python-pptx fixture corpus.

## Fuzzing

The fuzz surface has two tiers.

**Open-path targets** feed hostile bytes into the read path. These are
`FuzzNewReader` and `FuzzOpcMetadataXML` in `opc`, plus
`FuzzOpenPptx`/`FuzzPptxSlideXML`, `FuzzOpenDocx`/`FuzzDocxDocumentXML`,
and `FuzzOpenXlsx`/`FuzzXlsxWorksheetXML` in the format packages. Each
opens the fuzzed bytes as a package and, on success, walks a bounded
slice of the model and round-trips it (SaveBytes, then re-open). The
`*XML` variants pack the fuzzed bytes into the main part of an
otherwise-valid package so the XML parsers see hostile input directly
instead of the fuzzer having to invent whole zip archives.

**Write-path / API targets** exercise the authoring and transform APIs
with fuzzed inputs, then save and re-open the result to prove the
produced package survives a full round-trip. They fuzz the option
structs and arguments of the higher-level mutators — `FuzzDocxAddShape`,
`FuzzDocxWatermark`, `FuzzDocxMailMerge`, `FuzzDocxContentControl`,
`FuzzDocxSdtPr`, `FuzzDocxRevisions` in `docx`; `FuzzPptxAddAnimation`,
`FuzzPptxAddSection`, `FuzzPptxAddConnector`,
`FuzzPptxMasterLayoutSetters`, `FuzzPptxSmartArtData` in `pptx`; and
`FuzzXlsxAddTable`, `FuzzXlsxAddConditionalFormat`,
`FuzzXlsxAddPivotTable`, `FuzzXlsxAddSparklineGroup` in `xlsx`. A typical
target builds a document with `Create()`, drives the mutator with the
fuzzed values (reading the results back), then re-parses the saved bytes.
The revision/content-control fuzzers additionally inject a fuzzed XML
fragment into a valid package, open it, then run the transform API
(accept/reject revisions, edit controls) before saving.

In both tiers, errors are the expected outcome on malformed input; any
panic is a bug.

`make fuzz` is a short smoke run (30s per target; override with
`FUZZTIME=5m make fuzz`). It discovers the targets dynamically at run
time — it enumerates every `Fuzz*` function in the packages that have
them — so this list can never go stale: a newly added target is picked
up automatically. Deeper fuzzing is `-fuzztime`-driven per target, e.g.:

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
