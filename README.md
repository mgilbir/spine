# Spine

A Go library for reading and writing Microsoft Office documents (PPTX, DOCX, XLSX) using the Open Packaging Conventions (OPC) standard.

## Features

Each bullet links to the guide that carries the detail.

- **OPC package support** — a low-level API for working with Open Packaging Convention packages.
- **Round-trip preservation** — open, edit, and save without disturbing what you did not edit: a part your code never reads is written back byte-for-byte; see [the round-trip model](docs/architecture.md#round-trip-fidelity-model) for what reading or editing a part changes.
- **In-memory I/O** — `SaveBytes` and `OpenReader` on all three formats; see [Working with Documents in Memory](#working-with-documents-in-memory).
- **Merge, append & split** — combine or divide packages with automatic id, part-name, and relationship remapping (no dangling references or duplicate parts): [pptx](docs/pptx.md#merging-and-splitting-decks), [docx](docs/docx.md#merging-and-appending-documents), [xlsx](docs/xlsx.md#merging-and-copying-sheets).
- **Password encryption & digital signatures** — write real AES-encrypted documents (agile and standard schemes) across all three formats, read them back into a document model in every format (pass `opc.WithPassword` to the ordinary `Open`), and sign/verify OPC package signatures; see [Encryption and signing](docs/encryption-and-signing.md).
- **VBA macros** — extract, inject/replace, and remove `vbaProject.bin`; see the [trust caveat](docs/encryption-and-signing.md#vba-macros).
- **Charts** — a format-agnostic builder for column, bar, line, pie, doughnut, area, radar, scatter, bubble, stock, surface, pie-of-pie, combo, and 3D charts wired into all three formats; see [Charts](docs/charts.md).
- **Text extraction** — a symmetric "give me all the text" API (it changes nothing, though it loads what it reads; see the round-trip model) across all three formats for search, indexing, and LLM ingestion: [docx](docs/docx.md#text-extraction), [xlsx](docs/xlsx.md#text-extraction), [pptx](docs/pptx.md#text-extraction).
- **Document & custom properties** — core, extended, and custom document properties on all three formats; see [Document properties](docs/properties.md).
- **Embedded OLE objects** — read with `OLEObjects()` and embed new objects in docx, pptx, and xlsx; unmodified objects round-trip verbatim; see [Embedded OLE objects](docs/forms-and-objects.md#embedded-ole-objects).
- **Form controls, ActiveX, ink & 3D models** — read Word/Excel form controls and ActiveX across formats ([details](docs/forms-and-objects.md#form-controls-and-activex)), and extract [ink annotations and 3D models](docs/forms-and-objects.md#ink-annotations-and-3d-models).
- **PowerPoint (PPTX)** — create and modify presentations: shapes, tables, images, charts, animations, transitions, SmartArt, media, sections, and comments; see [docs/pptx.md](docs/pptx.md).
- **Word (DOCX)** — create and modify documents: styles, tables, tracked changes, comments, footnotes, mail merge, content controls, and fields; see [docs/docx.md](docs/docx.md).
- **Excel (XLSX)** — create and modify workbooks: add and delete sheets (deletion cascades to the parts only that sheet owned), formulas, styles, pivot tables, conditional formatting, sparklines, tables, and page/print setup; see [docs/xlsx.md](docs/xlsx.md).
- **Rendering** — draw slides, Word pages, and sheet ranges to PNG or SVG in pure Go (`pptxrender`, `docxrender`, `xlsxrender`), or from the command line with `spine-render`; see [Rendering](docs/rendering.md).

## Installation

```bash
go get github.com/mgilbir/spine
```

## Quick Start

### Creating a PowerPoint Presentation

```go
package main

import (
    "github.com/mgilbir/spine/common/dml"
    "github.com/mgilbir/spine/pptx"
)

func main() {
    // Create a new presentation
    p := pptx.Create()
    p.Properties.Title = "My Presentation"
    p.Properties.Creator = "Author Name"

    // Add a slide
    slide := p.AddSlide()
    slide.SetName("Introduction")

    // Add a text box
    textBox := slide.AddTextBox()
    textBox.SetPosition(dml.Inches(1), dml.Inches(1))
    textBox.SetSize(dml.Inches(8), dml.Inches(1))
    textBox.SetText("Hello, World!")

    // Save the presentation
    if err := p.Save("presentation.pptx"); err != nil {
        panic(err)
    }
}
```

### Creating a Word Document

```go
package main

import (
    "github.com/mgilbir/spine/docx"
)

func main() {
    // Create a new document
    doc := docx.Create()
    doc.Properties.Title = "My Document"

    // Add a heading
    doc.AddHeading("Welcome", 1)

    // Add a paragraph
    doc.AddParagraphWithText("This is a simple document created with Spine.")

    // Add formatted text
    p := doc.AddParagraph()
    bold := p.AddRun()
    bold.SetText("Bold text")
    bold.SetBold(true)

    // Save the document
    if err := doc.Save("document.docx"); err != nil {
        panic(err)
    }
}
```

### Creating an Excel Spreadsheet

```go
package main

import (
    "fmt"
    "github.com/mgilbir/spine/xlsx"
)

func main() {
    // Create a new workbook
    wb := xlsx.Create()

    // Add a sheet. AddSheet reports an illegal or already-taken name rather
    // than quietly renaming the sheet.
    sheet, err := wb.AddSheet("Sales")
    if err != nil {
        panic(err)
    }

    // Set cell values
    sheet.SetCellValue("A1", "Product")
    sheet.SetCellValue("B1", "Revenue")
    sheet.SetCellValue("A2", "Widgets")
    sheet.SetCellValue("B2", 1500.0)
    sheet.SetCellValue("A3", "Gadgets")
    sheet.SetCellValue("B3", 3200.0)

    // Add a formula
    cell, _ := sheet.Cell("B4")
    cell.SetFormula("SUM(B2:B3)")

    // Save the workbook
    if err := wb.Save("spreadsheet.xlsx"); err != nil {
        panic(err)
    }

    fmt.Printf("Sheets: %d\n", wb.SheetCount())
}
```

For the full per-format feature set and code walkthroughs (opening, templating,
comments, hyperlinks, images, protection), see the guides:
[pptx](docs/pptx.md), [docx](docs/docx.md), [xlsx](docs/xlsx.md), and
[charts](docs/charts.md).

## Opening vs. Creating Documents

`Create` builds a new document from scratch; `Open`/`OpenReader` parse an existing file. Both return the same types with the same mutation API, and edits made after `Open` persist on save: document properties, cell values, text edits, and added slides, sheets, or paragraphs are all written back, while parts your code never read are preserved byte-for-byte. A part that was read but not changed is re-emitted: identical for xlsx, and for pptx and docx nearly always identical, but not guaranteed (see [the round-trip model](docs/architecture.md#round-trip-fidelity-model)). Known asymmetries that remain:

- pptx: `Create()` produces a 4:3 deck (use `CreateWithOptions` with `SlideSizeWidescreen`, or `CreateWidescreen()`, for 16:9). The baked master and layouts size their placeholders to the slide, so both aspect ratios are internally consistent.
- docx: markup the library does not model is captured raw when a document is opened and preserved verbatim on save, but it is opaque to the API — `Text()` does not see text inside it and `SetText`/`ReplaceText` cannot edit it.
- pptx: master and layout `Placeholders()` are read-only views; mutating the returned values does not change the saved parts. To edit placeholder geometry use `EditablePlaceholders()`/`EditablePlaceholder()`, which write back to the part. `Presentation.Theme()` and `SlideMaster.Theme()`, by contrast, are read/write: edits to the theme are saved (see [docs/pptx.md](docs/pptx.md)).

## Working with Documents in Memory

All three formats can be saved to and opened from memory. `SaveBytes` exists on `pptx.Presentation`, `docx.Document`, and `xlsx.Workbook`; each package also provides `OpenReader`:

```go
package main

import (
    "bytes"
    "github.com/mgilbir/spine/xlsx"
)

func main() {
    wb := xlsx.Create()
    sheet, err := wb.AddSheet("Export")
    if err != nil {
        panic(err)
    }
    if err := sheet.SetCellValue("A1", "hello"); err != nil {
        panic(err)
    }

    data, err := wb.SaveBytes()
    if err != nil {
        panic(err)
    }

    wb2, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
    if err != nil {
        panic(err)
    }
    defer wb2.Close()
}
```

For xlsx, `WriteToBuffer` still works but is deprecated: it is `SaveBytes` wrapped in a `*bytes.Buffer`, and neither docx nor pptx has a counterpart.

## Validation

Every top-level type — `pptx.Presentation`, `docx.Document`, `xlsx.Workbook` — has a `Validate()` method that inspects the current in-memory model (without saving) and returns a `validate.Report` (from `github.com/mgilbir/spine/common/validate`): a slice of structured findings. Each finding carries a stable `Code`, a `Severity` (error or warning), the `Part` it concerns, and a human-readable `Detail`, so callers can triage programmatically rather than parse a string.

`Save`, `SaveBytes`, and `SaveTo` run `Validate()` first and refuse to write when any **error-severity** finding is present, so an edit cannot introduce a structural error. Warnings never block a save. The findings are sound — no error-severity finding fires on a file the corresponding Office app accepts.

The gate checks what the program has loaded, not the whole input file. A pptx slide or the docx body that was never read is not parsed for validation: it is written back byte-for-byte, defects included. xlsx checks every sheet. So a damaged input file can save without complaint, and the same save is refused once your code reads the damaged slide or body, even if it changes nothing. See [Troubleshooting](docs/troubleshooting.md#save-refused-after-only-reading-a-file).

```go
p, _ := pptx.Open("deck.pptx")
// ... mutate ...

// Inspect findings without saving.
for _, f := range p.Validate() {
    fmt.Printf("%s [%s] %s: %s\n", f.Code, f.Severity, f.Part, f.Detail)
}

// SaveBytes validates first and returns the report as an error if any
// finding is error-severity; nothing is written in that case.
if _, err := p.SaveBytes(); err != nil {
    log.Fatal(err)
}
```

This is the complete catalog. Error severity means the save is refused; warning
severity means the finding is reported and the save proceeds, because the
corresponding Office app accepts the structure. The table is the single source
of truth for these severities and is verified against the validators themselves
by `internal/docsguard` — a severity that changes in code without changing here
fails `go test`. The guard compares codes and severities only; the "Fires when"
text is maintained by hand.

<!-- validation-catalog:begin -->

| Code | Severity | Fires when |
|------|----------|------------|
| `dangling-rel` | error and warning | A reference names a relationship id that does not exist. Error where the structure cannot survive it — a section's `headerReference`/`footerReference` (docx), a workbook `sheet` or a relationship to a part removed this session (xlsx), a master's `sldLayoutId` or a presentation's `sldId`/`custShow` entry (pptx). Warning for the references Word opens anyway: a drawing's image or chart embed, and a hyperlink (docx). |
| `defined-name-scope` | error | A `definedName` is scoped to a `localSheetId` that is out of range for the workbook. |
| `duplicate-part-name` | error | Two part names collide case-insensitively, which OPC forbids. |
| `rel-id-dup` | error | Two relationships in one part's `.rels` share an `Id`, so a reference resolves to whichever the reader sees first (pptx). |
| `merge-overlap` | error | Two merged cell ranges on a sheet overlap. |
| `shape-id-dup` | error | Two shapes on one slide share a `cNvPr` id. |
| `shared-formula-orphan` | error | A shared-formula follower cell (`t="shared"`) has no master defining that `si`. |
| `sheet-id-dup` | error | Two sheets share a `sheetId`. |
| `bookmark-missing` | warning | A `bookmarkEnd` has no matching `bookmarkStart` (or the reverse). |
| `chart-no-rel` | warning | A slide's chart reference has no matching relationship (pptx). |
| `chart-target-missing` | warning | A drawing's chart relationship points at a part that is not in the package (xlsx). |
| `comment-missing` | warning | A `commentReference` names a comment id that comments.xml does not define (docx). |
| `comment-no-author` | warning | A comment references an author id with no entry in the author list; PowerPoint shows it as "Unknown". |
| `comment-person-orphan` | warning | A threaded comment references a person id that persons.xml does not define (xlsx). |
| `comment-ref-invalid` | warning | A comment's cell reference is not a valid cell address (xlsx). |
| `content-type-missing` | warning | A part has no content type — no `Default` extension mapping and no `Override`. |
| `data-validation-range` | warning | A data-validation `sqref` is malformed or out of range. |
| `hyperlink-no-rel` | warning | A slide hyperlink's `r:id` has no matching relationship (pptx). |
| `hyperlink-rel-missing` | warning | A cell hyperlink's `r:id` has no matching relationship (xlsx). |
| `merge-malformed` | warning | A merged-range reference does not parse as a range. |
| `note-missing` | warning | A footnote/endnote reference names an id the notes part does not define (docx). |
| `numbering-missing` | warning | A paragraph's `numPr` references a `numId` with no matching `w:num`, or the document has no numbering part at all. **Warning, not error**: real documents in the wild reference a `numId` while shipping an empty or partial numbering part, and Word opens them — blocking the save would reject a file Word accepts. |
| `sldid-rel-type` | error | A `p:sldId` or `p:custShow` entry resolves to a relationship whose type is not a slide (pptx). |
| `rel-target-missing` | warning | A relationship's target part is not present in the package. |
| `style-cycle` | warning | A style's `basedOn` chain forms a cycle or points at itself (docx). |
| `style-missing` | warning | A paragraph or run references a style id that styles.xml does not define (docx). |
| `zoom-no-target` | warning | A slide or section zoom names a target id that the deck does not contain (pptx). |
| `styles-empty` | warning | The workbook has a styles part with no formatting records (xlsx). |

<!-- validation-catalog:end -->

If a finding is advisory for your use case, `SaveToUnvalidated` writes without the pre-save check. When a save is refused or a file misbehaves, see [Troubleshooting](docs/troubleshooting.md).

## Supported Flavors

Each format family comes in several ECMA-376 flavors, distinguished only by the main part's content type. `Open` accepts all of them and a save re-emits the flavor the file was opened with — a slideshow stays a slideshow, and a macro-enabled workbook keeps both its `vbaProject.bin` (preserved verbatim) and its macro-enabled content type. The `Flavor()` accessor on `Presentation`, `Document`, and `Workbook` reports the main part's content type (`opc.ContentType*` constants).

| Package | Flavors opened and round-tripped |
|---------|----------------------------------|
| `pptx`  | presentation (.pptx), slideshow (.ppsx), template (.potx), macro-enabled presentation (.pptm), slideshow (.ppsm), and template (.potm) |
| `docx`  | document (.docx), template (.dotx), macro-enabled document (.docm), and template (.dotm) |
| `xlsx`  | workbook (.xlsx), template (.xltx), macro-enabled workbook (.xlsm), template (.xltm), and add-in (.xlam) |

Adding or removing a VBA project changes the flavor: `SetVBAProject` switches the main part to its macro-enabled content type (`.xlsx` → `.xlsm`, `.docx` → `.docm`, `.pptx` → `.pptm`), and `RemoveVBAProject` switches it back, removing the macro parts — including on a document built with `Create`, which otherwise saves as the regular flavor. Other conversions, such as template to document or slideshow to presentation, are not supported.

Opening an ISO-Strict (ISO/IEC 29500 Strict) package — a valid but as-yet-unread OOXML dialect that uses the `purl.oclc.org/ooxml` namespaces instead of the transitional ones — returns `opc.ErrStrictOOXML`, a distinct signal that the file is a genuine Office document in an unsupported dialect rather than a corrupt or non-Office file.

## Thread safety

A `pptx.Presentation`, `docx.Document`, or `xlsx.Workbook` **may be read from several goroutines at once**. The accessors that build part of the model on first use — the worksheet or slide or main-part parse, comments, sparklines, notes, shape lists, author lists — are synchronized, so concurrent readers of one value are safe even when they race to be the first to touch a sheet or slide.

**Modifying one is not safe**, and internal locking would not make it so. A mutating call hands back a handle — a `*Cell`, `*Slide`, `*Paragraph` — that outlives the call and is not covered by any lock the call took; and a read-modify-write spanning two calls (`Rows()` then a loop over the count, `Sheet(i)` then use of the result) has nothing to hold it together. Only the caller knows where its own operation begins and ends, so confine modification to one goroutine, or guard it with external synchronization at that granularity.

`Save`/`SaveBytes`/`SaveTo` mutate shared state while serializing, so they count as modification: they must not run concurrently with each other, with a mutation, or with a read of the same value. Distinct values may be used from different goroutines.

## Resource limits

To bound memory against decompression ("zip bomb") attacks and other resource exhaustion, an opened package is capped by five limits, each with a default and an option that overrides it for one reader:

| Option | Default | Bounds |
| --- | --- | --- |
| `opc.WithMaxDecompressedPartSize` | 1 GiB | bytes any single part may decompress to |
| `opc.WithMaxDecompressedPackageSize` | 4 GiB | bytes decompressed across the whole package |
| `opc.WithMaxPackageEntries` | 65536 | zip entries in the package |
| `opc.WithMaxNestingDepth` | 1000 | element nesting depth in any XML part |
| `opc.WithMaxEncryptedInputSize` | 2 GiB | bytes read from an encrypted input before parsing its container |

```go
// One reader, raised bounds; nothing global changes.
doc, err := docx.Open(path, opc.WithMaxDecompressedPartSize(4<<30))
```

A bound of zero or less disables it. The defaults are the `opc.Default*` constants, and `opc.DefaultReaderOptions()` returns them as a value you can adjust wholesale and pass with `opc.WithReaderOptions`. Configuration is resolved per open, so concurrent opens with different limits need no coordination — there is no global state to mutate, and no window in which one open sees another's settings.

## Units

Positions and sizes take EMUs (English Metric Units), the standard unit in Office Open XML. Helper functions are provided for common conversions:

```go
import "github.com/mgilbir/spine/common/dml"

// Convert from inches to EMUs
width := dml.Inches(10.5)

// Convert from centimeters to EMUs
height := dml.Centimeters(5.0)

// Convert from points to EMUs (e.g. for line widths)
lineWidth := dml.Points(2)
```

Font sizes are the exception: `SetFontSize` takes plain points, not EMUs — use `run.SetFontSize(12)` for 12pt text.

## Package Structure

- `opc/` - Open Packaging Conventions implementation
- `common/` - Shared types and utilities
  - `crypto/` - Office password encryption (agile/standard AES, RC4 decrypt) and OPC XML-DSig signing (`crypto.ErrWrongPassword` and friends)
  - `dml/` - DrawingML types (colors, geometry, fills, lines)
    - `chart/` - Chart types
    - `diagram/` - Diagram types
  - `enum/` - Common enumerations
  - `options/` - The functional-option mechanism behind `opc.WithPassword`, the `WithMax*` limits, and the other per-call options
  - `omml/` - Office Math Markup Language types
  - `oxml/` - Shared Office XML types
  - `validate/` - Structured validation vocabulary (`Report`, `Error`) shared by the format packages
  - `vml/` - Vector Markup Language types
  - `xml/` - XML namespace handling and Builder-based serialization
- `chart/` - Public, format-agnostic chart builder, serialization, and reader
- `pptx/` - PowerPoint document support
  - `pptxrender/` - Render a slide to a `render.Page`
- `docx/` - Word document support
  - `docxrender/` - Lay out and paginate a document into `render.Page`s
- `xlsx/` - Excel document support
  - `xlsxrender/` - Render a cell range to a `render.Page`
- `render/` - Page snapshots and the PNG/SVG writers shared by the three renderers
- `cmd/spine-render/` - Command-line renderer for .pptx, .docx, and .xlsx files

## Documentation

- [docs/](docs/README.md) — the documentation index, routed by reader question, plus the per-format guides ([pptx](docs/pptx.md), [docx](docs/docx.md), [xlsx](docs/xlsx.md)), [charts](docs/charts.md), [encryption and signing](docs/encryption-and-signing.md), [rendering](docs/rendering.md), and [troubleshooting](docs/troubleshooting.md).
- [docs/architecture.md](docs/architecture.md) — how the library is put together: package layering, the save pipeline, and the lazy-parse part lifecycle, drawn as diagrams.
- [CHANGELOG.md](CHANGELOG.md) — the release history, including the 0.1.0 lazy-parse behavior.
- [CONTRIBUTING.md](CONTRIBUTING.md) — build, test, lint, fuzz, and the round-trip philosophy.

Runnable programs for all three formats live in [`examples/`](examples/):

- [`create_presentation`](examples/create_presentation) — build a PowerPoint deck.
- [`pptx_diagram`](examples/pptx_diagram) — build a diagram deck: a connector bound to two shapes, slide-master text-style and slide-layout background editing, speaker notes, and the SmartArt read path, then reopen to verify the round-trip.
- [`pptx_deck`](examples/pptx_deck) — build a rich PowerPoint deck: a native chart with an auto-embedded data workbook, a table with an in-text hyperlink, an auto shape with layered effects (shadow, glow, reflection) and an entrance animation, Zoom/Wheel transitions, sections, and a threaded comment — saved in two phases (so shape ids materialize) and reopened to read the sections, animations, charts, and comments back.
- [`create_spreadsheet`](examples/create_spreadsheet) — build an Excel workbook.
- [`create_document`](examples/create_document) — build a Word document (page setup, lists, table, image).
- [`docx_report`](examples/docx_report) — author a rich Word report: custom paragraph/character styles, a custom numbered list, a table of contents, a table with a vertical cell merge, an inline image, an embedded chart, threaded comments, a content control, a two-column page-numbered section, and document protection.
- [`docx_review`](examples/docx_review) — review a Word document: list tracked changes and comment threads, then accept all revisions and save a clean copy.
- [`xlsx_report`](examples/xlsx_report) — a guided tour of the newer XLSX authoring features: a table with a totals row, conditional formatting, an embedded chart, page/print setup, freeze panes, named styles and sheet/workbook protection.
- [`xlsx_dashboard`](examples/xlsx_dashboard) — build a sales dashboard: a pivot table cross-tabulating regions against months, per-row and per-column sparklines, and a line chart, then reopen the file to read the pivot layout and sparkline groups back.
- [`render_previews`](examples/render_previews) — render a slide, a sheet range, and the pages of a Word document to PNG and SVG.
- [`docx_mailmerge`](examples/docx_mailmerge) — author a mail-merge form letter: mail-merge configuration with a data source, MERGEFIELD placeholders, a floating text-box callout, a "DRAFT" text watermark, and author-side tracked changes — reopened to read the merge fields, text boxes, watermark, and revisions back.

## Testing

Unit tests run against small synthetic fixtures (committed) and larger real-world Office files that are fetched on demand (`make fetch`, `make fetch-cc`) and skip silently when absent. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full build/test/lint/fuzz flow, [`testdata/README.md`](testdata/README.md) for the external and python-pptx fixtures, and [`testdata/cc/README.md`](testdata/cc/README.md) for the Common Crawl corpus. To run the suite: `make test`, which fetches the fixtures first and checks a fast subset of the corpus; `make test-corpus` runs the whole corpus (15–20 minutes, in a memory-capped scope).

## Requirements

- Go 1.26 or later

Spine is pre-1.0 (module `v0.x`): the API may change between minor versions, per the Go module versioning conventions. All non-internal packages are part of the public API surface. Some are imported directly by the user-facing examples (`chart`, `common/dml`, `common/enum`, `opc`, `render`, `pptx/pptxrender`, `docx/docxrender`, `xlsx/xlsxrender`); the rest are reachable through the format packages' own signatures — `Validate()` returns a `common/validate.Report`, and the encrypted open surfaces `common/crypto`'s errors and options — so they are equally part of the contract. Anything under an `internal/` path is not.

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test, and fixture instructions, and please feel free to submit issues and pull requests.
