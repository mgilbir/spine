# Spine

A Go library for reading and writing Microsoft Office documents (PPTX, DOCX, XLSX) using the Open Packaging Conventions (OPC) standard.

## Features

Each bullet links to the guide that carries the detail.

- **OPC package support** — a low-level API for working with Open Packaging Convention packages.
- **Round-trip preservation** — byte-identical round-trip fidelity for unmodified parts across all formats.
- **In-memory I/O** — `SaveBytes` and `OpenReader` on all three formats; see [Working with Documents in Memory](#working-with-documents-in-memory).
- **Merge, append & split** — combine or divide packages with automatic id, part-name, and relationship remapping (no dangling references or duplicate parts): [pptx](docs/pptx.md#merging-and-splitting-decks), [docx](docs/docx.md#merging-and-appending-documents), [xlsx](docs/xlsx.md#merging-and-copying-sheets).
- **Password encryption & digital signatures** — read/write real AES-encrypted documents (agile and standard schemes), and sign/verify OPC package signatures; see [Encryption and signing](docs/encryption-and-signing.md).
- **VBA macros** — extract, inject/replace, and remove `vbaProject.bin`; see the [trust caveat](docs/encryption-and-signing.md#vba-macros).
- **Charts** — a format-agnostic builder for column, bar, line, pie, scatter, combo, bubble, and 3D charts wired into all three formats; see [Charts](docs/charts.md).
- **Text extraction** — a symmetric, read-only "give me all the text" API across all three formats for search, indexing, and LLM ingestion: [docx](docs/docx.md#text-extraction), [xlsx](docs/xlsx.md#text-extraction), [pptx](docs/pptx.md#text-extraction).
- **Document & custom properties** — core, extended, and custom document properties on all three formats; see [Document properties](docs/docx.md#document-properties).
- **Embedded OLE objects** — read with `OLEObjects()` and embed new objects in [docx](docs/docx.md), [pptx](docs/pptx.md#embedded-ole-objects), and [xlsx](docs/xlsx.md); unmodified objects round-trip verbatim.
- **Form controls, ActiveX, ink & 3D models** — read Word/Excel form controls and ActiveX across formats ([details](docs/xlsx.md#form-controls-and-activex)), and extract [ink annotations and 3D models](docs/pptx.md#ink-annotations-and-3d-models).
- **PowerPoint (PPTX)** — create and modify presentations: shapes, tables, images, charts, animations, transitions, SmartArt, media, sections, and comments; see [docs/pptx.md](docs/pptx.md).
- **Word (DOCX)** — create and modify documents: styles, tables, tracked changes, comments, footnotes, mail merge, content controls, and fields; see [docs/docx.md](docs/docx.md).
- **Excel (XLSX)** — create and modify workbooks: formulas, styles, pivot tables, conditional formatting, sparklines, tables, and page/print setup; see [docs/xlsx.md](docs/xlsx.md).

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

    // Add a sheet
    sheet := wb.AddSheet("Sales")

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

`Create` builds a new document from scratch; `Open`/`OpenReader` parse an existing file. Both return the same types with the same mutation API, and edits made after `Open` persist on save: document properties, cell values, text edits, and added slides, sheets, or paragraphs are all written back, while parts you did not touch are preserved byte-for-byte. Known asymmetries that remain:

- pptx: `Create()` produces a 4:3 deck (use `CreateWithOptions` with `SlideSizeWidescreen`, or `CreateWidescreen()`, for 16:9). The baked master and layouts size their placeholders to the slide, so both aspect ratios are internally consistent.
- docx: markup the library does not model is captured raw when a document is opened and preserved verbatim on save, but it is opaque to the API — `Text()` does not see text inside it and `SetText`/`ReplaceText` cannot edit it.
- pptx: master and layout `Placeholders()` and `Theme()` are read-only views; mutating the returned values does not change the saved parts. To edit placeholder geometry use `EditablePlaceholders()`/`EditablePlaceholder()`, which write back to the part.

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
    sheet := wb.AddSheet("Export")
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

For xlsx, `WriteToBuffer` remains as a convenience wrapper around `SaveBytes` that returns a `*bytes.Buffer`.

## Validation

Every top-level type — `pptx.Presentation`, `docx.Document`, `xlsx.Workbook` — has a `Validate()` method that inspects the current in-memory model (without saving) and returns a `validate.Report` (from `github.com/mgilbir/spine/common/validate`): a slice of structured findings. Each finding carries a stable `Code`, a `Severity` (error or warning), the `Part` it concerns, and a human-readable `Detail`, so callers can triage programmatically rather than parse a string.

`Save`, `SaveBytes`, and `SaveTo` run `Validate()` first and refuse to write when any **error-severity** finding is present, so a structurally corrupt package is never produced. Warnings never block a save. The findings are sound — no error-severity finding fires on a file the corresponding Office app accepts.

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

Error-severity checks include duplicate shape ids within a slide, dangling `sldLayoutId`/sheet/header/footer relationship references, orphaned shared-formula followers, duplicate `sheetId`, out-of-range `definedName` scope, overlapping merged ranges, and a `numPr` that references an undefined numbering definition. Conditions that Office tolerates — a relationship whose target part is missing, a part with no content type, a dangling image/hyperlink reference, an undefined style reference — are reported as warnings.

If a finding is advisory for your use case, `SaveToUnvalidated` writes without the pre-save check. When a save is refused or a file misbehaves, see [Troubleshooting](docs/troubleshooting.md).

## Supported Flavors

Each format family comes in several ECMA-376 flavors, distinguished only by the main part's content type. `Open` accepts all of them and a save re-emits the flavor the file was opened with — a slideshow stays a slideshow, and a macro-enabled workbook keeps both its `vbaProject.bin` (preserved verbatim) and its macro-enabled content type. The `Flavor()` accessor on `Presentation`, `Document`, and `Workbook` reports the main part's content type (`opc.ContentType*` constants).

| Package | Flavors opened and round-tripped |
|---------|----------------------------------|
| `pptx`  | presentation (.pptx), slideshow (.ppsx), template (.potx), macro-enabled presentation (.pptm), slideshow (.ppsm), and template (.potm) |
| `docx`  | document (.docx), template (.dotx), macro-enabled document (.docm), and template (.dotm) |
| `xlsx`  | workbook (.xlsx), template (.xltx), macro-enabled workbook (.xlsm), template (.xltm), and add-in (.xlam) |

Documents built with `Create` always save as the regular flavor. Converting a file from one flavor to another (e.g. saving an `.xlsm` as a plain `.xlsx`, which would also need its macro parts stripped) is not supported.

Opening an ISO-Strict (ISO/IEC 29500 Strict) package — a valid but as-yet-unread OOXML dialect that uses the `purl.oclc.org/ooxml` namespaces instead of the transitional ones — returns `opc.ErrStrictOOXML`, a distinct signal that the file is a genuine Office document in an unsupported dialect rather than a corrupt or non-Office file.

## Thread safety

A `pptx.Presentation`, `docx.Document`, or `xlsx.Workbook` — and everything reached through it (slides, sheets, paragraphs, shapes) — is not safe for concurrent use and must be confined to one goroutine, or all access guarded by external synchronization. In particular `Save`/`SaveBytes`/`SaveTo` mutate shared state while serializing, so they must not run concurrently with each other or with any mutation of the same value. Distinct values may be used from different goroutines.

## Resource limits

To bound memory against decompression ("zip bomb") attacks, an opened package is capped by `opc.MaxDecompressedPartSize` (per part, default 1 GiB) and `opc.MaxDecompressedPackageSize` (total across the package, default 4 GiB); `opc.OpenEncrypted` additionally limits its input with `opc.MaxEncryptedInputSize` (default 2 GiB). Raise a limit before opening a legitimately larger file, or set a decompression bound to 0 to disable it. These are plain package-level variables captured when a reader is constructed, so set them during program setup — mutating one concurrently with an open in another goroutine is a data race. To override the two decompression limits for a single reader without touching the globals, pass `opc.ReaderOptions` to `opc.NewReaderWithOptions` / `opc.OpenReaderWithOptions`.

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
  - `omml/` - Office Math Markup Language types
  - `oxml/` - Shared Office XML types
  - `validate/` - Structured validation vocabulary (`Report`, `Error`) shared by the format packages
  - `vml/` - Vector Markup Language types
  - `xml/` - XML namespace handling and Builder-based serialization
- `chart/` - Public, format-agnostic chart builder, serialization, and reader
- `pptx/` - PowerPoint document support
- `docx/` - Word document support
- `xlsx/` - Excel document support

## Documentation

- [docs/](docs/README.md) — the documentation index, routed by reader question, plus the per-format guides ([pptx](docs/pptx.md), [docx](docs/docx.md), [xlsx](docs/xlsx.md)), [charts](docs/charts.md), [encryption and signing](docs/encryption-and-signing.md), and [troubleshooting](docs/troubleshooting.md).
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
- [`docx_mailmerge`](examples/docx_mailmerge) — author a mail-merge form letter: mail-merge configuration with a data source, MERGEFIELD placeholders, a floating text-box callout, a "DRAFT" text watermark, and author-side tracked changes — reopened to read the merge fields, text boxes, watermark, and revisions back.

## Testing

Unit tests run against small synthetic fixtures (committed) and larger real-world Office files that are fetched on demand (`make fetch`, `make fetch-cc`) and skip silently when absent. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full build/test/lint/fuzz flow, [`testdata/README.md`](testdata/README.md) for the external and python-pptx fixtures, and [`testdata/cc/README.md`](testdata/cc/README.md) for the Common Crawl corpus. To run the full suite: `make test`.

## Requirements

- Go 1.25 or later

Spine is pre-1.0 (module `v0.x`): the API may change between minor versions, per the Go module versioning conventions. All non-internal packages — including `opc`, `chart`, `common/crypto`, `common/dml`, and `common/validate` — are part of the public API surface, since the user-facing examples import them directly.

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test, and fixture instructions, and please feel free to submit issues and pull requests.
