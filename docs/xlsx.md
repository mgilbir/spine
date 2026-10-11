# Excel (XLSX)

This guide is the detailed reference for the `xlsx` package: creating and
modifying Excel workbooks. Reach for it when you need the full feature surface
(formulas, styles, pivot tables, conditional formatting, sparklines, tables,
form controls, page/print setup) or the code walkthroughs for commenting,
hyperlinking, protecting a sheet, adding images, merging workbooks, and
streaming large ones. The one-paragraph quick start lives in the
[README](../README.md#creating-an-excel-spreadsheet); this document carries
everything else.

## Common tasks

Open a workbook, read and write cells, add sheets and images, and save. The
walkthroughs below come first because they are what most programs do; the
topic sections after them cover large workbooks, merging, and text extraction,
and the [capability reference](#capabilities) closes the guide. To add a
chart, see [Charts in spreadsheets](charts.md#charts-in-spreadsheets).

### Opening and editing a workbook

`Open` reads an existing file; edits made afterwards are written back on
`Save`. `Sheet.Cell` returns a cell (creating it if it is empty) and
`Sheet.SetCellValue` writes one by reference.

```go
package main

import (
    "fmt"

    "github.com/mgilbir/spine/xlsx"
)

func main() {
    wb, err := xlsx.Open("book.xlsx")
    if err != nil {
        panic(err)
    }
    defer wb.Close()

    sheet, err := wb.Sheet(0)
    if err != nil {
        panic(err)
    }

    // Read a cell. Value returns it typed: string, float64, bool,
    // time.Time, xlsx.CellError, or nil for an empty cell.
    cell, err := sheet.Cell("A1")
    if err != nil {
        panic(err)
    }
    fmt.Printf("A1 = %v (%T)\n", cell.Value(), cell.Value())

    // Write: through the cell, or by reference.
    cell.SetValue("updated")
    if err := sheet.SetCellValue("B1", 42.5); err != nil {
        panic(err)
    }

    if err := wb.Save("book-updated.xlsx"); err != nil {
        panic(err)
    }
}
```

### Adding and removing sheets

`Workbook.AddSheet` appends a sheet; `Workbook.RemoveSheet(index)` removes one
(`DeleteSheet` is the deprecated older name for the same call).

`AddSheet` returns `(*Sheet, error)` and uses the name you give it or none at
all: an illegal name (see `ValidateSheetName`) or one already taken —
case-insensitively, as Excel compares them — is reported rather than silently
rewritten into something else. When a derived name is what you want, ask for it
explicitly:

```go
sheet, err := wb.AddSheet(wb.UniqueSheetName(userSuppliedName))
```

`UniqueSheetName` strips forbidden characters, trims to 31 runes, and appends a
`" (2)"`-style suffix until the name is free — exactly the coercion `AddSheet`
used to apply behind your back.

Removal is a cascade, not just a list edit: the sheet's preserved part, its
content-type override and its `.rels` part go, and so do the parts reachable
only from that sheet — its drawings, tables and comments, and transitively the
media and chart parts those own. A part still referenced from anywhere else is
kept. Workbook state that indexes sheets by position is adjusted too: the active
tab is shifted or clamped, and sheet-scoped defined names are re-pointed (names
scoped to the deleted sheet are dropped).

References to the removed sheet in the surviving sheets' formulas and in
defined names are rewritten to `#REF!`, as Excel does. The rewrite is
permanent: adding a sheet with the same name later does not restore them. If
other sheets' formulas must keep pointing at it, hide the sheet instead of
removing it.

One deliberate exception: pivot-table parts are *not* cascade-deleted, because
their caches are shared with the workbook and with other pivots.

```go
wb, _ := xlsx.Open("book.xlsx")
if err := wb.RemoveSheet(2); err != nil {
    log.Fatal(err)
}
```

### Adding images

`Sheet.AddImage(cell, data, opts)` anchors a PNG, JPEG, GIF, or SVG image (an SVG
gets a transparent raster fallback) with its top-left corner at a cell, on
created and opened workbooks alike. `ImageOptions` sets the size in pixels
(`WidthPx`/`HeightPx`, with `PreserveAspect` to scale the other axis), a
two-cell anchor (`ToCell`, so the image moves and resizes with the cells), and
the alt text. `AddImage` returns only an error; read images back with
`Sheet.Images()`.

```go
package main

import (
    "os"

    "github.com/mgilbir/spine/xlsx"
)

func main() {
    wb := xlsx.Create()
    sheet, err := wb.AddSheet("Report")
    if err != nil {
        panic(err)
    }

    logo, err := os.ReadFile("logo.png")
    if err != nil {
        panic(err)
    }
    // One-cell anchor at B2: 120 px wide, height scaled to keep the aspect.
    if err := sheet.AddImage("B2", logo, xlsx.ImageOptions{
        WidthPx: 120, PreserveAspect: true, AltText: "Company logo",
    }); err != nil {
        panic(err)
    }

    photo, err := os.ReadFile("photo.jpg")
    if err != nil {
        panic(err)
    }
    // Two-cell anchor: the image spans D2:H12 and moves and resizes with
    // those cells.
    if err := sheet.AddImage("D2", photo, xlsx.ImageOptions{ToCell: "H12"}); err != nil {
        panic(err)
    }

    if err := wb.Save("images.xlsx"); err != nil {
        panic(err)
    }
}
```

### Hyperlinks, images, and sheet protection

Spine reads and writes cell hyperlinks and sheet protection, and reads back
merged cells, freeze panes, auto-filter, and data validation, plus worksheet
images and conditional-formatting rules. `xlsx.Hyperlink` (`URL`, `Anchor`,
`Tooltip`, `SetTooltip`) and the image read accessors (`AltText`, `Data`,
`ContentType`) use the same method names as their `docx` and `pptx`
counterparts.

For an image added as SVG, `Data()` returns the **raster (PNG) fallback** that
was embedded alongside it for viewers that cannot render SVG —
`Image.SVGData()` returns the original SVG bytes. `SVGData()` is nil when the
image is not an SVG, and also when its SVG variant is not available (an image
read back from an opened file), so treat a nil result as "no SVG here" rather
than as an error.

```go
wb, _ := xlsx.Open("book.xlsx")
sheet, _ := wb.Sheet(0)

// Read hyperlinks, images, protection, merged cells, panes, and CF rules.
for _, h := range sheet.Hyperlinks() {
    fmt.Printf("%s -> url=%q anchor=%q\n", h.Ref(), h.URL(), h.Anchor())
}
for _, img := range sheet.Images() {
    fmt.Printf("image %s at %s alt=%q\n", img.ContentType(), img.AnchorCell(), img.AltText())
}
if p := sheet.Protection(); p != nil {
    fmt.Printf("protected (password=%v, sort locked=%v)\n", p.HasPassword(), p.Sort())
}
fmt.Println("merged:", sheet.MergedCells())
cols, rows, frozen := sheet.FrozenPanes()
fmt.Printf("frozen=%v cols=%d rows=%d\n", frozen, cols, rows)
for _, cf := range sheet.ConditionalFormats() {
    fmt.Printf("cf %s: %d rules\n", cf.SqRef, len(cf.Rules))
}

// Write an external hyperlink, an internal jump, and turn on sheet protection.
cell, _ := sheet.Cell("A1")
cell.SetHyperlink("https://example.com/").SetTooltip("Visit us")
nav, _ := sheet.Cell("A2")
nav.SetInternalHyperlink("Sheet2!A1")
// Protect fails with ErrNotWorksheet on a chartsheet, dialogsheet or macrosheet.
if err := sheet.Protect(xlsx.SheetProtectionOptions{Password: "secret", AllowSort: true}); err != nil {
    log.Fatal(err)
}

if err := wb.Save("book.xlsx"); err != nil {
    log.Fatal(err)
}
```

Sheet protection is a UI guard, not encryption: `Protect` uses Excel's documented
legacy 16-bit password hash, which is trivially removed. Every write persists on
both the `Create` and `Open` save paths, and an open→save with no edits writes
every part of a feature-bearing workbook byte-identical (the zip container
around the parts can still differ; see the
[round-trip fidelity model](architecture.md#round-trip-fidelity-model)).

### Reading and writing comments

The `xlsx` package reads and writes both comment mechanisms Excel uses — legacy
notes (`xl/comments*.xml` + a VML drawing) and modern threaded comments
(`xl/threadedComments/*` + a person list) — through one `xlsx.Comment` type.

```go
wb, _ := xlsx.Open("review.xlsx")
sheet, _ := wb.Sheet(0)

// Read every comment on the sheet (legacy notes and threaded comments unified).
for _, c := range sheet.Comments() {
    fmt.Printf("%s by %s: %s (resolved=%v)\n", c.Ref(), c.Author(), c.Text(), c.Resolved())
    for _, reply := range c.Replies() {
        fmt.Printf("  ↳ %s: %s\n", reply.Author(), reply.Text())
    }
}

// Add a threaded comment (Excel back-compat: a legacy note fallback is written
// too, so older Excel still renders the text). Then reply and resolve.
cell, _ := sheet.Cell("B2")
c := cell.AddComment("Ada Lovelace", "Please double-check this figure.")
c.Reply("Alan Turing", "Confirmed — updated.")
c.Resolve()

_ = wb.Save("review.xlsx")
```

`Sheet.AddComment(ref, author, text)`, `Cell.AddComment(author, text)`,
`Comment.Reply`, `Comment.Resolve`/`SetResolved`, and `Sheet.AddNote` (a
legacy-only note) are the write entry points; `Sheet.Comments()` and
`Cell.Comment()` read. An open→save with no edits writes every part of a
comment-bearing workbook byte-identical (the zip container around the parts can
still differ); only the touched sheet's comment parts are regenerated when a
comment is added.

## Writing large workbooks

The DOM path (`Create`, `AddSheet`, `Cell`, `Save`) keeps every cell in memory,
roughly 25× the file size in live heap (the `StreamWriter` godoc's estimate).
For big exports use `xlsx.NewStreamWriter`, which writes a row at a time
straight into the package. Its memory is bounded by one block of rows, however
many rows are written.

`StreamWriter.AddSheet` starts a sheet and finishes the previous one.
`StreamSheet.AppendRow` writes the next row and returns its 1-based number;
values convert as `Cell.SetValue` converts them, and a `nil` leaves a cell empty.
`WriteRowAt` writes at an explicit row number, which must be greater than the
last one written. `Close` completes the package.

```go
package main

import (
    "fmt"
    "os"

    "github.com/mgilbir/spine/xlsx"
)

func main() {
    f, err := os.Create("large.xlsx")
    if err != nil {
        panic(err)
    }

    w := xlsx.NewStreamWriter(f)
    sheet, err := w.AddSheet("Data")
    if err != nil {
        panic(err)
    }
    // Column widths go before the sheet's first row.
    if err := sheet.SetColWidth(2, 24); err != nil {
        panic(err)
    }
    if _, err := sheet.AppendRow("id", "name", "amount"); err != nil {
        panic(err)
    }
    for i := 1; i <= 100000; i++ {
        if _, err := sheet.AppendRow(i, fmt.Sprintf("item %d", i), float64(i)*1.5); err != nil {
            panic(err)
        }
    }
    // WriteRowAt jumps ahead, leaving the rows in between empty. It cannot
    // go back: the number must be greater than the last row written.
    if _, err := sheet.WriteRowAt(sheet.Rows()+2, "total", nil, 7500075000.0); err != nil {
        panic(err)
    }

    // Adding a sheet finishes the previous one for good.
    notes, err := w.AddSheet("Notes")
    if err != nil {
        panic(err)
    }
    if _, err := notes.AppendRow("generated by spine"); err != nil {
        panic(err)
    }

    // Close completes the package; without it the file is a truncated zip.
    if err := w.Close(); err != nil {
        panic(err)
    }
    if err := f.Close(); err != nil {
        panic(err)
    }
}
```

Streaming gives up these things:

- It only creates workbooks. You cannot open an existing one and stream into it.
- Rows go out in order and cannot be revisited.
- Column widths must be set before the sheet's first row.
- There is no `<dimension>` element (optional; Excel recalculates it).
- There is no `Validate` pass, because there is no model to check.
- There is no encryption, which needs the whole package.
- A writer abandoned without `Close` leaves a truncated archive.

## Merging and copying sheets

xlsx has no split counterpart to pptx's `ExtractSlides`. `Workbook.CopySheetFrom`
copies one sheet, and
`Workbook.AppendSheetsFrom` copies every worksheet of another workbook after
the existing sheets. Each copy lands under a unique name, with a `" (2)"`-style
suffix when the name is taken, so use the returned sheets rather than the
source names.

A copy carries cell values (shared strings resolved), styles, formulas, merged
ranges, column widths, row heights, images, and the sheet's visibility. A
hidden or very hidden sheet stays so, unless it would be the destination's only
visible sheet, which Excel refuses. It does not carry:

- charts: they are neither copied nor reported on the new sheet. The hidden
  data sheet `Sheet.AddChart` writes for each chart is a worksheet, so it is
  copied, and stays hidden;
- cross-sheet references: a copied formula such as `Lookup!A1*2` keeps its text
  and points at whatever sheet has that name in the destination;
- defined names and pivot tables.

A sheet that fails to copy stops `AppendSheetsFrom` with the earlier copies in
place: the append is not transactional. `AppendSheetsFrom` skips chartsheets,
dialogsheets, and macrosheets, which have no worksheet model to copy.

```go
package main

import (
    "fmt"

    "github.com/mgilbir/spine/xlsx"
)

func main() {
    dst, err := xlsx.Open("summary.xlsx")
    if err != nil {
        panic(err)
    }
    defer dst.Close()
    src, err := xlsx.Open("regions.xlsx")
    if err != nil {
        panic(err)
    }
    defer src.Close()

    // Every worksheet of src, in order, after dst's own. A name already
    // taken gets a " (2)"-style suffix, so use the returned sheets.
    added, err := dst.AppendSheetsFrom(src)
    if err != nil {
        panic(err)
    }
    for _, s := range added {
        fmt.Println("copied as", s.Name())
    }

    if err := dst.Save("combined.xlsx"); err != nil {
        panic(err)
    }
}
```

## Text extraction

`xlsx.Workbook.Text()` / `Sheet.Text()` return cell values row-major with shared
strings resolved, plus cell comments — plain concatenation, no markup,
deterministic ordering.

## Form controls and ActiveX

Excel form controls, Word form fields, and ActiveX controls (all three formats)
are covered in [Form controls and ActiveX](forms-and-objects.md#form-controls-and-activex).

## PNG and SVG range previews

`xlsxrender.PrepareRange(ctx, sheet, "A1:D10", render.Options{Fonts: resolver})`
prepares the supported range profile including unsaved values. Rendering lives
in the separate `xlsx/xlsxrender` package, so importing `xlsx` alone does not
compile the renderer or its dependencies. See the
[rendering guide](rendering.md) for supported values, fonts, sizing and resource
limits. Unsupported formatting returns an error; preparation creates no missing
cells and does not save the workbook.

## Capabilities

### Cells, formulas, and styling

- Create workbooks with multiple sheets, and remove one (`Workbook.RemoveSheet`) — see [Adding and removing sheets](#adding-and-removing-sheets)
- Read and write cell values. `Cell.Value()` returns the value typed: `string`, `float64`, `bool`, `time.Time` for a date-formatted cell, `xlsx.CellError` (the error literal, such as `#DIV/0!`) for an error cell, and `nil` for an empty one. For a formula cell it returns the *cached result*, typed the same way a literal cell of that type reads back (`=1+1` reads as `float64(2)`, not the string `"2"`); when the file carries no cached value it falls back to the formula text. `Cell.Type()` reports which case you are in
- Formula support, including array (`Cell.SetArrayFormula`), shared (`Cell.SetSharedFormula`, master + follower stubs over a range), and dynamic-array/spill (`Cell.SetDynamicArrayFormula`) authoring; saving a workbook with a new dynamic-array formula synthesizes `xl/metadata.xml` (the `XLDAPR` record) and tags the spill master cell with `cm`, so Excel shows the spill without a recalc
- Cell styling (fonts, fills, borders, number formats, alignment)
- Style depth — named/built-in cell styles (`StyleManager.AddNamedStyle`/`ApplyNamedStyle`/`Cell.SetNamedStyle` with `BuiltinStyle*` ids), gradient fills (`FillStyle.Gradient`), diagonal borders (`BorderStyle.Diagonal`/`DiagonalUp`/`DiagonalDown`), and alignment extras (`ShrinkToFit`, `JustifyLastLine`, `ReadingOrder`, `RelativeIndent`)
- Merged cells and named ranges
- Column widths and row heights
- Rich text (per-run formatting) within a cell
- Font depth: strikethrough, sub/superscript, and underline styles (single/double/accounting)
- Workbook structure/window protection and per-cell locked/hidden (`CellStyle.Protection`)

### Data, filters, and views

- Auto-filter criteria and sort state — read and write per-column value-list/custom-comparison filters (`Sheet.SetFilterColumn`/`FilterColumns`) and sort conditions (`Sheet.SetSortState`/`SortState`)
- Freeze panes, auto-filter, and data validation (including `errorStyle` and `imeMode`), with read accessors for each
- Sheet view & structure: sheet visibility (`Sheet.SetVisibility`/`Visible`, refusing to hide the last visible sheet), row/column hide (`SetRowHidden`/`SetColumnHidden`), view toggles (row/column headers, right-to-left, formulas, zeros, ruler, and normal/page-layout/page-break view), scrolling split panes (`Sheet.SplitPanes`, distinct from freeze), row/column grouping & outline levels (`GroupRows`/`GroupColumns`, collapsed flags, outline summary placement), and force-recalc-on-open (`Workbook.SetForceFullCalc`)
- External data: enumerate connections (`Workbook.Connections()` — name, type, connection string/command, web URL) and report the presence of a Power Pivot data model / Power Query content (`Workbook.DataModel()`); both are preserved verbatim. Authoring or refreshing a live query/mashup is deferred
- What-if scenarios — read and write (`Sheet.Scenarios`/`Sheet.AddScenario`): named substitute-value sets over a group of changing cells; existing scenarios round-trip byte-for-byte, authored ones emit from the typed model
- Search & replace (`Workbook.ReplaceText` / `Sheet.ReplaceText`, mirroring `docx` and `pptx`): replaces in string cells (shared-string cells convert to inline so the shared table is untouched) and across a rich cell's runs; formula cells are left alone, and a workbook with no matching text saves with every part byte-identical

### Graphics, tables, and pivots

- Embedded OLE objects — extract (`Workbook.OLEObjects`) and embed (`Sheet.AddOLEObject`): write an object as an embedding part with its worksheet `<oleObjects>` reference, a legacy VML Pict shape, an optional preview image, and the relationships/content types
- Embedded images anchored to cells (one- and two-cell anchors, SVG with a raster fallback), on both created and opened workbooks — see [Adding images](#adding-images)
- Read-only enumeration of worksheet images and conditional-formatting rules
- Tables (ListObjects) — read and write (`Sheet.Tables`/`Sheet.AddTable`): name, range, columns (with totals-row functions/labels and calculated-column formulas), header/totals rows, and built-in table style with row/column-stripe and first/last-column banding
- Pivot tables — read and create (`Sheet.PivotTables`/`Workbook.PivotTables`/`Sheet.AddPivotTable`): name, location, source range/cache, and the row/column/value/filter field layout with per-value aggregation (sum/count/average/min/max); creating one builds the pivot cache (definition + records, `refreshOnLoad`), the pivot table definition, the workbook `<pivotCaches>` entry, relationships and content-type overrides. Also supports calculated (formula) value fields, numeric range grouping of a field into value buckets, date grouping of a date/time field by year/quarter/month/day, discrete grouping that folds selected field items into named parent groups, and adding a pivot to a workbook that already has pivot caches (the new cache is allocated without disturbing existing pivots). Existing pivots round-trip byte-for-byte.
- Pivot slicers and timelines — read (`Sheet.Slicers`/`Workbook.Slicers`/`Sheet.Timelines`/`Workbook.Timelines`): each slicer/timeline's name, caption, source pivot field and controlled pivot tables, resolved through its slicer/timeline cache part. Slicers, timelines, their cache parts and the worksheet/workbook extension references round-trip byte-for-byte. Creating slicers and timelines is not yet supported (a slicer/timeline is an on-sheet drawing whose creation also requires injecting relationship-bearing x14/x15 extension lists into the shared workbook and worksheet parts at save time)
- Conditional formatting — read and write (`Sheet.AddConditionalFormat`): cell-value, color scales, data bars, icon sets, top/bottom, above-average, duplicate/unique, text, and formula rules
- Sparklines — read, write and mutate (`Sheet.Sparklines`/`Sheet.AddSparklineGroup`): line/column/win-loss groups with one or more (data range, location cell) mappings, stored in the worksheet extension list; live `SparklineGroup` handles set every color slot and per-group point toggles (markers, high/low/first/last/negative) and delete groups; unmodified sparklines round-trip byte-for-byte
- Charts (column, bar, line, pie, doughnut, radar, scatter, area, combo, bubble, stock, surface, pie-of-pie, and 3D variants) that reference the host workbook's cells — see [charts.md](charts.md)

### Comments, links, and protection

- Hyperlinks (external and internal), read and written through `xlsx.Hyperlink`, which has the same `URL`/`Anchor`/`Tooltip`/`SetTooltip` methods as the docx and pptx `Hyperlink` types plus `Ref`
- Sheet protection (read state and write with the legacy password hash)
- Comments: legacy notes and modern threaded comments (replies, resolve), both read and written through `xlsx.Comment` (its core methods have the same names as the docx and pptx comment types), with per-run rich text on notes (`Comment.RichText`/`Sheet.AddNoteRichText`/`Comment.SetRichText`, alongside the plain `Text()`)
- Theme read/write (`Workbook.Theme()`): color-scheme accents and major/minor Latin fonts, sharing the `dml.ThemeEditor` model with the docx and pptx theme APIs

### Page and print

- Page & print setup: orientation, scaling/fit, margins, headers/footers, print options, and print area/titles
