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
resource-capped `systemd-run --user --scope` — that is what `make test-race`,
`make test-corpus` and `make harvest-batch` do. A bare run shares the terminal's
cgroup, and one kernel OOM kill there makes systemd (default `OOMPolicy=stop`)
tear down the whole terminal scope, killing your shell and everything in it.
Inside a capped scope the kill is contained to the test. Generic pattern for any
heavy one-off:

```bash
systemd-run --user --scope -p MemoryMax=12G -p MemorySwapMax=1G -- <command>
```

`OOMPolicy=stop` is the right default for a *test* run — containment is the
point. `make harvest-batch` is the one place that sets `OOMPolicy=continue`
instead, because there the OOM kill is an expected outcome the surviving
orchestrator must record; see the comment on that target.

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

**Resource-bounded targets** cover the encrypted-document parsers,
where "did it panic?" is not a strong enough oracle. `FuzzCFBContainer`
and `FuzzOpenEncrypted` in `opc`, and `FuzzAgileEncryptionInfo` /
`FuzzLegacyEncryptionInfo` in `common/crypto`, run every call inside an
`internal/fuzzbound.Budget`: a bound on bytes allocated and on wall clock,
each expressed as a floor plus a rate per input byte. The reason is C360,
where an unvalidated CFB header field made a 512-byte file allocate
16 GiB — an allocation that does not panic, and whose pages are never
touched, so it shows up neither as a crash nor as memory pressure (the
counter is `/gc/heap/allocs:bytes`, not RSS, for exactly that reason).
These targets also assert the API contract on every input: a malformed
container returns an error and never a partial success, and a strict
decrypt that accepts an agile package must have verified its HMAC (C361).
`FuzzSignatureXML` belongs to the same tier: it rewrites the signature
part of a validly signed package and asserts a coverage-containment
property — a fuzzer holds no private key, so the reported `CoveredParts`
may never exceed what the genuine signature covers (C362).
The budgets are checked against real encrypted documents — including one
large enough to need chained DIFAT sectors — by
`TestEncryptionFuzzBudgetsAllow*` and `TestCryptoFuzzBudgetAllows*`, so a
budget that would fire on a legitimate file fails in `go test` rather than
during a fuzz run.

In all tiers, errors are the expected outcome on malformed input; any
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

## Documentation

Godoc and the guides in `docs/` are not independent. When you change or qualify
a **behavior** — a call that becomes a no-op in some state, an argument that is
resolved late and can silently drop the operation, a side effect on an argument,
a return type that widens — write the caveat in the godoc **and** re-read the
guide section that covers the feature. Historically the caveat landed in godoc
and the guide was never touched: `AppendSlidesFrom` mutating its source,
out-of-range slide jumps being dropped, `Image.SVGData` versus `Data`,
`DeleteSheet`'s cascade and `Cell.Value` returning `time.Time` and typed formula
results all shipped that way (C579). The previous docs audit verified that every
guide snippet *executes*, which is exactly the check a caveat drift survives.

Two mechanical guards exist, and both are cheap to extend:

- `internal/docsguard` pins the README's validation-severity catalog to the
  severities the validators actually emit, and pins a list of caveat-bearing
  APIs to the guide sections that must keep documenting them. Add a row when you
  add a caveat.
- The README's validation table lives between `<!-- validation-catalog:begin -->`
  and `<!-- validation-catalog:end -->` markers; do not restructure it without
  updating the parser in that package.

## Mechanical guards

Several invariants in this repo are enforced by tests that read the source
rather than exercise it: the mutation-flag guards in `docx`, `xlsx` and `pptx`,
the capture-coverage guard in `pptx/internal/oxml`, the percent-type/schema diff
in `common/dml`, the relationship-allocator guards in `pptx`, the
map-iteration-order guard in `internal/maporder`, and the float-formatting guard
in `common/xml`. They exist because the same classes of omission kept recurring
— a mutator that edits a preserved part without flagging it, an attribute that
loses its captured spelling, an id allocator that ignores what the opened
document already contains, a map ranged in Go's randomized order on the way to
the output, a float printed in a notation Office never writes.

`internal/symmetry` guards a different axis: properties that are supposed to
hold *across* formats. Most of it compares API shape — which types exist and
what arguments they take — but shape was only ever a proxy. The
`dcterms:modified` rule was implemented once per format against three different
flag topologies and diverged behind identical signatures, which no per-format
suite could see because each only knows its own answer. If a rule spans formats,
assert it across formats.

`internal/maporder` is the one that needs type information, and it is the worked
example of how to get it: `golang.org/x/tools/go/packages` — the module's only
requirement, and the first reason it has a `go.sum`. `go/importer`'s source mode
is the dependency-free alternative and was tried first; it is four times slower
(2.5s against 0.6s) because it re-type-checks every transitive dependency, and
it type-checks each package in isolation, so a call from `docx` into `opc`
resolves to a *different* `*types.Func` than the one indexed from `opc`'s own
syntax and the cross-package call graph silently does not work.

A guard that resolves nothing reports a clean repository forever, so
`TestGuardSeesEnough` asserts floors on packages, files, functions, typed
expressions and map ranges, and that four named landmark loops were still
analysed and still came out clean. `Load` also refuses to return a package that
carries any load error, rather than pressing on with partial types.

Two properties make one of these worth having, and both are easy to lose:

- **It must fail on a *new* omission, not pin today's known set.** A table of
  known-good cases goes stale the moment someone adds a method; a guard that
  derives its subject list from the AST or from reflection covers the new method
  by construction. Where a hand-maintained list is unavoidable, add a
  completeness check that fails when the list and the source disagree.
- **Someone must have watched it fail.** Add the violation the guard exists to
  catch — a mutator that skips its flag, an attribute without capture — confirm
  the guard names it, then remove it. A guard whose detection has never been
  observed is indistinguishable from one that matches nothing, and this repo has
  shipped both a scaling check that failed on noise instead of regressions and a
  test dismissed as flaky for three audits while it was reporting a real bug.

Exemptions belong in code with their reason, next to the guard, and a **stale**
exemption should fail the test too — otherwise the list only grows.

### Coverage is a finder, not a target

`make cover` measures the whole module against the whole module
(`-coverpkg=./...`) with the corpus enabled; `make cover-dark` lists the library
functions no test executes at all. Per-package coverage is close to useless
here — most of this library is exercised across package boundaries, `cctest`
drives all three formats over real documents and `internal/symmetry` drives them
together — so a package measured against only its own tests understates itself
and points at the wrong gaps.

**Do not write tests to raise the number.** A test that calls a function so a
line turns green asserts nothing, costs maintenance forever, and makes the
number lie about how well the code is checked. There is no coverage floor in CI
on purpose: a floor is satisfied by exactly those tests.

What the profile is genuinely good for is pointing at code where a bug could
exist today and nothing would notice. The patterns worth acting on, in the order
they have paid off here:

- **A sibling accessor left behind.** `WidthEMU` covered and `HeightEMU` not, on
  the same type. The bug this predicts — one returning the other's value — is
  invisible any other way, and a fixture whose width and height are equal will
  pass under exactly that bug.
- **A truncated family.** Accents 1 and 2 tested, 3 through 6 not. Test the whole
  family in one table with a *distinct* value per member, so a setter writing
  into its neighbour's slot fails; derive the member list where you can, so the
  next member is covered by construction.
- **An inverse operation.** The `Set` side tested and the `Clear` side not.
  Assert on the serialized part, since a `Clear` that writes a zero value rather
  than removing the element produces a different document and the in-memory
  getter cannot tell those apart.
- **A fallback path.** Case-insensitive or lenient branches that only fire on
  inputs no fixture contains. These have already produced a defect here (C596),
  and `lookupFileLinear`/`lookupRawLinear` sat dead until a coverage pass found
  them.
- **A size-triggered branch.** `writeDIFATSectors` only runs above ~7 MB of CFB
  payload, so no fixture reached it. Where such a branch produces a file another
  implementation must read, validate it against one — a symmetric misreading
  between our own writer and reader passes a round trip and still ships files
  Office cannot open.

The bar for any test added this way: **mutate the function so it is wrong and
watch the test fail.** If a mutation you would expect to be caught is not, the
test is decorative. This is the same rule as the guards above, and the same
reason.

### Testing wall-clock behaviour

`dcterms:modified` is a wall-clock value serialized at RFC3339 (one-second)
resolution, so a stamp written in the same second as the baseline cannot be told
from no stamp at all. Tests that pin the "stamp iff the content changed" rule
therefore have to let a second elapse — which used to mean sleeping for real.

They run under `testing/synctest` (Go 1.25) instead. `synctest.Test` runs the
body in a bubble whose clock is fake: it starts at 2000-01-01T00:00:00Z and
jumps instantly when every goroutine in the bubble is durably blocked. It
reaches `time.Now()` inside the library too, because a bubble captures
goroutines rather than packages, so the sleeps stay and cost nothing.

Four conventions, each of which is easy to get wrong and two of which fail in
ways that look like a library bug:

- **Assert the exact instant, not that it moved forward.** A deterministic clock
  makes the instant a save will stamp knowable. `Modified.After(baseline)`
  accepts *any* later value, so a stamp at the wrong time reads as success;
  compare for equality and treat anything else as its own failure.
- **`Modified.After(baseline)` is not merely weak here, it is wrong.** The bubble
  epoch is the year 2000 and fixtures are newer (`pptx/testdata/test.pptx` is
  2012, `docx/testdata/chart.docx` is 2022), so a *correctly* stamping save
  produces a timestamp earlier than the fixture's and the assertion fails.
- **Sleep in whole seconds.** A sub-second instant does not survive the RFC3339
  round trip. The bubble clock advances only by these sleeps, since the library
  sets no timers of its own.
- **Put the bubble inside `t.Run`, not around it.** `t.Run` and `t.Parallel`
  panic inside a bubble.

Converting a timing test can quietly make it vacuous, which is worse than the
sleeps it removed. Re-run the fail-before toggles *after* converting — for this
rule, "never stamp" and "stamp every save" — and confirm nothing passes under
both.

### Exploratory sweeps

`ast-grep` is a good way to *find* a new class before you know whether it is
real. Rules are quick to write and run without any project setup:

```
ast-grep scan --inline-rules '...' xlsx/          # ad-hoc
ast-grep scan -r some-rule.yml xlsx/              # from a file
```

There is deliberately no `sgconfig.yml` and no committed rule set, and ast-grep
is not wired into CI. A rule that lives outside the test suite is a guard nobody
watches, and every invariant currently worth enforcing is already a Go test that
runs on every push — `TestSheetDirtyIsOnlySetByMarkDirty` covers the same ground
as the obvious ast-grep rule and got the Go `method_declaration` /
`function_declaration` distinction right, which the first draft of that rule did
not: it silently excluded nothing and reported a false positive. The same slip in
the other direction reports "clean" forever.

`semgrep` is the heavier option and earns its weight on one specific kind of
question. Both tools are effectively intra-procedural, so neither can answer
"this getter reads a field the materializer never assigns" — that spans two
functions. But semgrep can express **negation and helper-call awareness within
one function**, which ast-grep's relational rules cannot, and that is often
enough once the question is restated.

That restatement is the transferable part. The getter-versus-materializer
property was undecidable as posed; rewritten as *a materializer that reads the
parsed node's `SpPr` but never mirrors it into the overlay* it fits inside a
single function, and a rule with one `pattern` plus three `pattern-not`s — the
third covering the helper form `$HELPER(&$DST.spPr, ...)` — retro-found both
buggy materializers behind C599. Two false positives came with them, both
dismissed in seconds because the types had no overlay field at all.

**When a cross-function property blocks you, look for an equivalent
single-function restatement.** That is what turned "cannot be automated" into two
true positives.

Know the limits before reaching for either:

- ast-grep matched C599's eight buggy methods only by their *formatting* —
  one-line delegating getters. `Connector.line()` is correct and has a body, so
  it did not match; reformat either and the answer flips in both directions. It
  found the right set for the wrong reason.
- Some questions are not in the Go source at all. Whether a
  `RawAttrListOverride` call site is safe depends on whether the attribute is
  *schema-required*, which no rule over the code can see. There the guard does
  not analyse — it forces the decision to be written down per call site and
  fails when a new site appears undeclared.

So: sweep with ast-grep, reach for semgrep when the question needs negation,
then promote anything real to a Go test. Prefer
`go/ast` and `go/types` for the promoted version — they are in the standard
library, this module has no third-party requirements, and the questions that
matter here ("does this method transitively reach `markDirty`?", "are these map
keys sorted before use?") need types and a call graph, which a syntactic matcher
cannot answer.

## Commit style

One logical change per commit, message in the form
`pkg: imperative summary` (e.g. `xlsx: validate sheet renames`). When a
commit addresses an audit finding, cite its ID in the summary or body,
e.g. `(C76, C133)`.

## Audit reports

The findings cited from commit messages (C1–C235, D1–D30) live in
`docs/audits/`. The reports are historical records of the code at their
audit date; they are not updated as findings are fixed.
