# Architecture

How spine is put together, drawn rather than narrated. For the round-trip
contract that underlies all of it, see
[CONTRIBUTING § Round-trip philosophy](../CONTRIBUTING.md#round-trip-philosophy).

## Package layering

```mermaid
flowchart TD
    pptx["pptx"]
    docx["docx"]
    xlsx["xlsx"]
    chart["chart (public chart builder)"]
    validate["common/validate"]
    models["common/* models: dml (+chart, diagram), oxml, omml, enum, vml"]
    opc["opc (parts, rels, content types, signatures, encrypted containers)"]
    crypto["common/crypto (agile/standard AES, RC4 decrypt, XML-DSig)"]
    xmlb["common/xml (Builder + reflection marshaler)"]

    pptx --> opc & models & validate & chart
    docx --> opc & models & validate & chart & crypto
    xlsx --> opc & models & validate & chart & crypto
    chart --> models & xmlb
    validate --> opc
    opc --> crypto & xmlb
    models --> xmlb
```

- **opc** is the OPC container: parts, relationships, `[Content_Types].xml`,
  package signatures, and the CFB-wrapped encrypted containers. It depends only
  on `common/crypto` and `common/xml`.
- **common/xml** is the write substrate — the `Builder` and the reflection
  marshaler that every XML part is serialized through. It has no spine
  dependencies (Go stdlib only), which is why it sits at the bottom.
- **The shared models** (`common/dml` and its `chart`/`diagram` children,
  `oxml`, `omml`, plus the leaf packages `enum` and `vml`) are the typed OOXML
  structs. The ones that marshal (dml, oxml, omml, dml/chart, dml/diagram)
  depend on `common/xml`; `enum` and `vml` are pure leaves.
- **common/validate** is the pre-save checker. It depends on `opc` (it inspects
  the assembled package), so it is not a bottom-level leaf.
- **chart** is the public chart builder; it composes `common/dml`(/chart) values
  and writes them through `common/xml`.
- **pptx / docx / xlsx** are the format packages. Each imports `opc`, the
  models, `validate`, and `chart`; `docx` and `xlsx` additionally import
  `common/crypto` directly for their `OpenEncrypted` wrappers (pptx reaches
  encryption through `opc`). Each also has an `internal/oxml` package for its
  format-specific structs.

## Save pipeline

```mermaid
flowchart TD
    A["Save / SaveAs / SaveTo / SaveBytes"] --> V{"Validate()"}
    V -- "error-severity finding" --> E["refused: Report returned as error<br/>(escape hatch: SaveToUnvalidated)"]
    V -- "clean or warnings only" --> O{"opened from a file?"}
    O -- "Create()" --> N["saveNew: every part generated from the model"]
    O -- "Open() / OpenReader()" --> R["saveRoundTrip"]
    R --> P["untouched parts: original bytes verbatim<br/>(incl. never-accessed lazy slides / sheets / body)"]
    R --> G["touched parts: regenerated from the model,<br/>plus the OPC furniture that references them"]
```

`SaveTo` runs `Validate()` first and returns the `Report` as an error on any
error-severity finding, so a structurally corrupt package is never written;
`SaveToUnvalidated` is the escape hatch. (`xlsx` also refuses an empty workbook
with `ErrNoSheets` ahead of the gate.) The origin decides the rest: a document
built with `Create` regenerates every part (`saveNew`), while one opened from a
file takes `saveRoundTrip`, writing untouched parts verbatim and regenerating
only the ones the session touched.

## Lazy-parse part lifecycle

```mermaid
stateDiagram-v2
    [*] --> Raw: Open() validates the part, then discards the model
    Raw --> Parsed: first read (Slide shapes / Sheet cells / Document body)
    Parsed --> Dirty: any mutation
    Raw --> [*]: Save writes original bytes verbatim
    Parsed --> [*]: Save writes byte-identical output (reads never mark dirty)
    Dirty --> [*]: Save regenerates the part from the model
```

Slides (`pptx`), sheets (`xlsx`), and the document body (`docx`) are validated
once at `Open` and then discarded; the model is re-parsed only on first access.
Reading never marks a part dirty, so an accessed-but-unedited part still
round-trips byte-identically. The two ways that happens differ: `xlsx` re-emits
a clean parsed sheet's *original bytes* (it regenerates only when `dirty`),
while `pptx` and `docx` *regenerate* a materialized part from its model — still
byte-identical, but produced by re-marshaling. A part never accessed at all
keeps its model nil and always passes through verbatim.

One nuance: `pptx`'s `ReplaceText` materializes each scanned slide's model (its
docx/xlsx text-scan counterparts do not), so those slides move to *Parsed* and
are regenerated on save — byte-identical where no key matched, but no longer on
the never-accessed verbatim-bytes path.

## Round-trip fidelity model

The invariant behind every diagram above is **byte-identity for untouched
content**: opening and saving reproduces every part byte-for-byte, and mutating
one part never disturbs another's bytes. A second invariant follows —
**childOrder**: parsed parts record the order of their children (including
unknown, raw-captured elements) so they re-serialize in the original sequence,
and every mutator that appends to a typed slice must maintain that bookkeeping.
Markup the typed model does not represent is captured verbatim and replayed on
save, so unknown content survives a round trip untouched. Both invariants and
the tests that enforce them are described in
[CONTRIBUTING § Round-trip philosophy](../CONTRIBUTING.md#round-trip-philosophy).
