# Excel (XLSX)

This guide is the detailed reference for the `xlsx` package: creating and
modifying Excel workbooks. Reach for it when you need the full feature surface
(formulas, styles, pivot tables, conditional formatting, sparklines, tables,
form controls, page/print setup) or the code walkthroughs for commenting,
hyperlinking, and protecting a sheet. The one-paragraph quick start lives in the
[README](../README.md#creating-an-excel-spreadsheet); this document carries
everything else.

## Capabilities

### Cells, formulas, and styling

- Create workbooks with multiple sheets, and delete one (`Workbook.DeleteSheet`) — see [Adding and deleting sheets](#adding-and-deleting-sheets)
- Read and write cell values. `Cell.Value()` returns the value typed: `string`, `float64`, `bool`, and `time.Time` for a date-formatted cell. For a formula cell it returns the *cached result*, typed the same way a literal cell of that type reads back (`=1+1` reads as `float64(2)`, not the string `"2"`); when the file carries no cached value it falls back to the formula text. `Cell.Type()` reports which case you are in
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
- Search & replace (`Workbook.ReplaceText` / `Sheet.ReplaceText`, mirroring `docx` and `pptx`): replaces in string cells (shared-string cells convert to inline so the shared table is untouched) and across a rich cell's runs; formula cells are left alone, and workbooks with no matching text round-trip byte-for-byte

### Graphics, tables, and pivots

- Embedded OLE objects — extract (`Workbook.OLEObjects`) and embed (`Sheet.AddOLEObject`): write an object as an embedding part with its worksheet `<oleObjects>` reference, a legacy VML Pict shape, an optional preview image, and the relationships/content types
- Embedded images anchored to cells (one- and two-cell anchors, SVG with a raster fallback), on both created and opened workbooks
- Read-only enumeration of worksheet images and conditional-formatting rules
- Tables (ListObjects) — read and write (`Sheet.Tables`/`Sheet.AddTable`): name, range, columns (with totals-row functions/labels and calculated-column formulas), header/totals rows, and built-in table style with row/column-stripe and first/last-column banding
- Pivot tables — read and create (`Sheet.PivotTables`/`Workbook.PivotTables`/`Sheet.AddPivotTable`): name, location, source range/cache, and the row/column/value/filter field layout with per-value aggregation (sum/count/average/min/max); creating one builds the pivot cache (definition + records, `refreshOnLoad`), the pivot table definition, the workbook `<pivotCaches>` entry, relationships and content-type overrides. Also supports calculated (formula) value fields, numeric range grouping of a field into value buckets, date grouping of a date/time field by year/quarter/month/day, discrete grouping that folds selected field items into named parent groups, and adding a pivot to a workbook that already has pivot caches (the new cache is allocated without disturbing existing pivots). Existing pivots round-trip byte-for-byte.
- Pivot slicers and timelines — read (`Sheet.Slicers`/`Workbook.Slicers`/`Sheet.Timelines`/`Workbook.Timelines`): each slicer/timeline's name, caption, source pivot field and controlled pivot tables, resolved through its slicer/timeline cache part. Slicers, timelines, their cache parts and the worksheet/workbook extension references round-trip byte-for-byte. Creating slicers and timelines is not yet supported (a slicer/timeline is an on-sheet drawing whose creation also requires injecting relationship-bearing x14/x15 extension lists into the shared workbook and worksheet parts at save time)
- Conditional formatting — read and write (`Sheet.AddConditionalFormat`): cell-value, color scales, data bars, icon sets, top/bottom, above-average, duplicate/unique, text, and formula rules
- Sparklines — read, write and mutate (`Sheet.Sparklines`/`Sheet.AddSparklineGroup`): line/column/win-loss groups with one or more (data range, location cell) mappings, stored in the worksheet extension list; live `SparklineGroup` handles set every color slot and per-group point toggles (markers, high/low/first/last/negative) and delete groups; unmodified sparklines round-trip byte-for-byte
- Charts (column, bar, line, pie, doughnut, radar, scatter, area, combo, bubble, stock, surface, pie-of-pie, and 3D variants) that reference the host workbook's cells — see [charts.md](charts.md)

### Comments, links, and protection

- Hyperlinks (external and internal), read and written through the `Hyperlink` type shared with the docx/pptx APIs
- Sheet protection (read state and write with the legacy password hash)
- Comments: legacy notes and modern threaded comments (replies, resolve), read and written through one unified `Comment` type, with per-run rich text on notes (`Comment.RichText`/`Sheet.AddNoteRichText`/`Comment.SetRichText`, alongside the plain `Text()`)
- Theme read/write (`Workbook.Theme()`): color-scheme accents and major/minor Latin fonts, sharing the `dml.ThemeEditor` model with the docx and pptx theme APIs

### Page and print

- Page & print setup: orientation, scaling/fit, margins, headers/footers, print options, and print area/titles

## Form controls and ActiveX

Read/enumerate interactive controls across formats — Word legacy form fields
(`docx.Document.FormFields()`: FORMTEXT/FORMCHECKBOX/FORMDROPDOWN with name,
value, checkbox state, and dropdown entries) with basic authoring via
`docx.Paragraph.AddFormField`; Excel form controls (`xlsx.Sheet.FormControls()`:
buttons, checkboxes, dropdowns, list boxes, option buttons, spinners, scroll
bars, each with its linked cell); and ActiveX controls (`ActiveXControls()` on
all three formats, reporting each control's COM class id, persistence mode, and
`activeXN.bin` binary). All control parts are preserved verbatim on save;
authoring ActiveX controls is out of scope.

Excel's own legacy form controls read back through `Sheet.FormControls()` —
buttons, checkboxes, dropdowns, list boxes, option buttons, spinners, and scroll
bars — reading each control's type, linked cell (`x:FmlaLink`), source range,
checkbox state, and its VML/`ctrlProps` parts; the control parts are preserved
verbatim on save.

## Adding and deleting sheets

`Workbook.AddSheet` appends a sheet; `Workbook.DeleteSheet(index)` removes one.
Deletion is a cascade, not just a list edit: the sheet's preserved part, its
content-type override and its `.rels` part go, and so do the parts reachable
only from that sheet — its drawings, tables and comments, and transitively the
media and chart parts those own. A part still referenced from anywhere else is
kept. Workbook state that indexes sheets by position is adjusted too: the active
tab is shifted or clamped, and sheet-scoped defined names are re-pointed (names
scoped to the deleted sheet are dropped).

Two deliberate exceptions: pivot-table parts are *not* cascade-deleted, because
their caches are shared with the workbook and with other pivots; and hiding
rather than deleting is the safer edit when other sheets' formulas reference the
one you are about to remove — spine does not rewrite formulas.

```go
wb, _ := xlsx.Open("book.xlsx")
if err := wb.DeleteSheet(2); err != nil {
    log.Fatal(err)
}
```

## Merging and copying sheets

Combine or divide packages with automatic id, part-name, and relationship
remapping (no dangling references or duplicate parts). `Workbook.CopySheetFrom`
copies a sheet with its resolved cell values, styles, merges, and embedded
images (from created or opened sources) under a unique name.

## Text extraction

`xlsx.Workbook.Text()` / `Sheet.Text()` return cell values row-major with shared
strings resolved, plus cell comments — plain concatenation, no markup,
deterministic ordering. This is the xlsx half of the symmetric, read-only "give
me all the text" API shared with `docx.Document.Text()` and
`pptx.Presentation.Text()` / `Slide.Text()`.

## Reading and writing comments

The `xlsx` package reads and writes both comment mechanisms Excel uses — legacy
notes (`xl/comments*.xml` + a VML drawing) and modern threaded comments
(`xl/threadedComments/*` + a person list) — through one unified `Comment` type.

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
`Cell.Comment()` read. A zero-modification open→save preserves comment-bearing
workbooks byte-for-byte; only the touched sheet's comment parts are regenerated
when a comment is added.

## Hyperlinks, images, and sheet protection

Spine reads and writes cell hyperlinks and sheet protection, and reads back the
previously write-only feature surface (merged cells, freeze panes, auto-filter,
data validation) plus worksheet images and conditional-formatting rules. The
`Hyperlink` type (`URL`, `Anchor`, `Tooltip`, `SetTooltip`) and the image read
accessors (`AltText`, `Data`, `ContentType`) are shared with the `docx` and
`pptx` APIs.

For an image added as SVG, `Data()` returns the **raster (PNG) fallback** that
was embedded alongside it for viewers that cannot render SVG —
`Image.SVGData()` returns the original SVG bytes. `SVGData()` is nil when the
image is not an SVG, and also when its SVG variant is not available (an image
read back from an opened file), so treat a nil result as "no SVG here" rather
than as an error.

```go
wb, _ := xlsx.Open("book.xlsx")
sheet, _ := wb.Sheet(0)

// Read hyperlinks, images, protection, and the write-only feature surface.
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
sheet.Protect(xlsx.SheetProtectionOptions{Password: "secret", AllowSort: true})

_ = wb.Save("book.xlsx")
```

Sheet protection is a UI guard, not encryption: `Protect` uses Excel's documented
legacy 16-bit password hash, which is trivially removed. Every write persists on
both the `Create` and `Open` save paths, and a zero-modification open→save of a
feature-bearing workbook stays byte-identical.
