# Spine

A Go library for reading and writing Microsoft Office documents (PPTX, DOCX, XLSX) using the Open Packaging Conventions (OPC) standard.

## Features

- **OPC Package Support**: Low-level API for working with Open Packaging Convention packages
- **Round-Trip Preservation**: Byte-identical round-trip fidelity for unmodified parts across all formats
- **In-Memory I/O**: `SaveBytes` and `OpenReader` on all three formats
- **Charts**: A format-agnostic `chart` package builds DrawingML charts (column, bar, line, pie, doughnut, radar, scatter, area, and combination charts) and serializes a valid `chart.xml` with cached values, plus a matching embedded workbook. Series carry optional solid colors (`Series.SetColor`) and value data labels (`SetDataLabels`); combination charts (`NewCombo`) mix per-series types (`Series.SetType`) across a primary and secondary value axis (`Series.SetSecondaryAxis`). All three formats wire it in with symmetric `AddChart` / `Charts()` methods: `xlsx` references the host workbook's cells (`Sheet.AddChart` / `Sheet.Charts` / `Workbook.Charts`); `docx` (`Document.AddChart` / `Paragraph.AddChart` / `Document.Charts`) and `pptx` (`Slide.AddChart` / `Slide.Charts` / `Presentation.Charts`) embed the data workbook
- **PowerPoint (PPTX)**: Create and modify PowerPoint presentations
  - Create presentations from scratch or from templates
  - Add, remove, and reorder slides
  - Add shapes, text, tables, and images — including SVG images with a raster fallback
  - Slide placeholders, and read-only access to each master's and layout's placeholders and theme (color and font schemes)
  - Slide furniture: footers, auto-updating or fixed dates, and slide numbers on every slide
  - Auto shapes with solid/gradient fills, lines, and shadows
  - Slide transitions (fade, push, wipe, and more)
  - Hyperlinks on runs and shapes (external URLs, internal slide jumps, and `ppaction://` verbs), read and written through one unified `Hyperlink` type shared with the docx/xlsx APIs
  - Read every picture on a slide (`Slide.Pictures()`), with alt text, bytes, content type, and position/size
  - Rich text depth: text-frame autofit, auto-numbered bullets with bullet color/size/font, and paragraph indent/tab stops
  - Read the presentation's footer, slide-number, and date furniture set on slides
- **Word (DOCX)**: Create and modify Word documents
  - Create documents from scratch
  - Add headings, paragraphs, and tables
  - Rich text formatting (bold, italic, color, font, size)
  - Paragraph alignment, spacing, and indentation
  - Bullet and numbered lists
  - Headers and footers
  - Inline and floating (anchored) images — including SVG images with a raster fallback
  - Charts (column, bar, line, pie, doughnut, radar, scatter, area) inserted inline via `AddChart`, each carrying an embedded workbook so Office can edit the data; read back with `Charts()`
  - Fields (PAGE/NUMPAGES) and a table of contents
  - Page setup (size, margins), multi-column sections, page-number format/start, title-page, and section type; enumerate sections with `Document.Sections()`
  - Document protection (`Document.Protect`): read-only, comments-only, tracked-changes, or forms editing modes, with an optional password
  - Rich run formatting: highlight, super/subscript, caps/small-caps, underline style + color, character spacing/kerning, and paragraph tab stops
  - Tracked changes (revisions): enumerate insertions, deletions, and run/paragraph property changes with `Document.Revisions()` (author, date, type, text), then apply or discard them with `Revision.Accept()`/`Reject()` or `Document.AcceptAllRevisions()`/`RejectAllRevisions()`
- **Excel (XLSX)**: Create and modify Excel workbooks
  - Create workbooks with multiple sheets
  - Read and write cell values (strings, numbers, booleans)
  - Formula support
  - Cell styling (fonts, fills, borders, number formats, alignment)
  - Freeze panes, auto-filter, and data validation, with read accessors for each
  - Merged cells and named ranges
  - Column widths and row heights
  - Hyperlinks (external and internal), read and written through the `Hyperlink` type shared with the docx/pptx APIs
  - Sheet protection (read state and write with the legacy password hash)
  - Read-only enumeration of worksheet images and conditional-formatting rules
  - Embedded images anchored to cells (one- and two-cell anchors, SVG with a raster fallback), on both created and opened workbooks
  - Rich text (per-run formatting) within a cell
  - Comments: legacy notes and modern threaded comments (replies, resolve), read and written through one unified `Comment` type
  - Page & print setup: orientation, scaling/fit, margins, headers/footers, print options, and print area/titles
  - Conditional formatting — read and write (`Sheet.AddConditionalFormat`): cell-value, color scales, data bars, icon sets, top/bottom, above-average, duplicate/unique, text, and formula rules
  - Workbook structure/window protection and per-cell locked/hidden (`CellStyle.Protection`)
  - Font depth: strikethrough, sub/superscript, and underline styles (single/double/accounting)

Runnable programs for all three formats live in [`examples/`](examples/).

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

### Reading and Writing Comments (Word)

Comments support the full review flow: read the feedback on a document, add
comments, reply in threads, and resolve. The core method set (`ID`, `Author`,
`Text`, `Date`, `Resolved`, `Replies`, `Parent`, `AddComment`, `Reply`,
`Resolve`) is shared with the `xlsx` and `pptx` comment APIs.

```go
package main

import (
    "fmt"

    "github.com/mgilbir/spine/docx"
)

func main() {
    doc, err := docx.Open("document.docx")
    if err != nil {
        panic(err)
    }
    defer doc.Close()

    // Read existing comments and their threads.
    for _, c := range doc.Comments() {
        fmt.Printf("%s on %q: %s (resolved=%v)\n",
            c.Author(), c.AnchorText(), c.Text(), c.Resolved())
        for _, reply := range c.Replies() {
            fmt.Printf("  ↳ %s: %s\n", reply.Author(), reply.Text())
        }
    }

    // Add a comment over a whole paragraph, reply to it, and resolve the thread.
    p := doc.AddParagraphWithText("The quick brown fox.")
    c := p.AddComment("Reviewer", "Please rephrase.")
    reply := c.Reply("Author", "Done.")
    reply.Resolve()

    // Range-precise anchors are also available:
    //   run.AddComment(author, text)
    //   doc.AddCommentOnRange(startRun, endRun, author, text)

    if err := doc.Save("reviewed.docx"); err != nil {
        panic(err)
    }
}
```

Spine writes and round-trips the modern comment parts (`comments.xml`,
`commentsExtended.xml` for threading/resolved state, and `people.xml` for the
author registry) and preserves `commentsIds.xml`/`commentsExtensible.xml`
verbatim. A zero-modification open→save of a comment-bearing document is
byte-identical.

### Hyperlinks, Images, Bookmarks, and Footnotes (Word)

Spine reads and writes hyperlinks, inline and floating images, bookmarks, and
footnotes/endnotes. The `Hyperlink` type (`URL`, `Anchor`, `Tooltip`) and the
image read accessors are shared with the `xlsx` and `pptx` APIs; bookmarks and
footnotes/endnotes are Word-specific.

```go
package main

import (
    "fmt"

    "github.com/mgilbir/spine/docx"
)

func main() {
    doc, err := docx.Open("document.docx")
    if err != nil {
        panic(err)
    }
    defer doc.Close()

    // Read every hyperlink, image, bookmark, and footnote.
    for _, h := range doc.Hyperlinks() {
        fmt.Printf("link %q -> url=%q anchor=%q\n", h.Text(), h.URL(), h.Anchor())
    }
    for _, img := range doc.Images() {
        fmt.Printf("image %s (%s) %.0fx%.0fpt alt=%q floating=%v\n",
            img.PartName(), img.ContentType(), img.Width(), img.Height(),
            img.AltText(), img.Floating())
    }
    for _, b := range doc.Bookmarks() {
        fmt.Printf("bookmark %q -> %q\n", b.Name(), b.Text())
    }
    for _, f := range doc.Footnotes() {
        fmt.Printf("footnote %s: %s\n", f.ID(), f.Text())
    }

    // Write: an external and an internal hyperlink, a bookmark they can target,
    // and a footnote anchored on a run.
    p := doc.AddParagraph()
    p.AddRun().SetText("See ")
    link := p.AddHyperlink("our site", "https://example.com/")
    link.SetTooltip("Visit us")

    target := doc.AddParagraphWithText("Chapter One")
    target.AddBookmark("chap1")
    doc.AddParagraph().AddInternalHyperlink("go to chapter", "chap1")

    note := doc.AddParagraphWithText("A claim.")
    note.Runs()[0].AddFootnote("Supporting evidence.")

    if err := doc.Save("annotated.docx"); err != nil {
        panic(err)
    }
}
```

External hyperlinks allocate an `External` relationship in the part's rels;
internal ones use `w:anchor`. Adding a footnote or endnote creates
`word/footnotes.xml` / `word/endnotes.xml` (with the mandatory separator notes,
the relationship, and the content-type override) on first use. A
zero-modification open→save of a document using any of these features is
byte-identical, and the parts are regenerated only when that feature is
modified.

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

### Reading and Writing Excel Comments

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

### Hyperlinks, Images, and Sheet Protection (Excel)

Spine reads and writes cell hyperlinks and sheet protection, and reads back the
previously write-only feature surface (merged cells, freeze panes, auto-filter,
data validation) plus worksheet images and conditional-formatting rules. The
`Hyperlink` type (`URL`, `Anchor`, `Tooltip`, `SetTooltip`) and the image read
accessors (`AltText`, `Data`, `ContentType`) are shared with the `docx` and
`pptx` APIs.

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

### Charts (preview)

The `chart` package builds a DrawingML `chart.xml` part (`c:chartSpace`) that is
independent of any one document format. Pick a chart type, set categories, add
series, and serialize:

```go
package main

import (
	"os"

	"github.com/mgilbir/spine/chart"
)

func main() {
	c := chart.NewColumn().
		SetTitle("Quarterly Sales").
		SetCategories([]string{"Q1", "Q2", "Q3", "Q4"}).
		SetAxisTitles("Quarter", "USD").
		SetLegend(chart.LegendRight).
		SetDataLabels(true) // render each point's value on the chart
	c.AddSeries("North", []float64{10, 20, 30, 40}).SetColor("#1F77B4")
	c.AddSeries("South", []float64{5, 15, 25, 35}).SetColor("#FF7F0E")

	// chart.xml with cached values so it renders without a live data source.
	xmlBytes, _ := c.MarshalChartXML()
	_ = os.WriteFile("chart1.xml", xmlBytes, 0o644)

	// The workbook docx/pptx charts embed so Office can edit the data. The
	// returned layout's cell ranges line up with the chart's c:f references.
	wbBytes, layout, _ := c.EmbeddedWorkbook()
	_ = wbBytes
	_ = layout

	// Read a chart.xml back into the model.
	parsed, _ := chart.Parse(xmlBytes)
	_ = parsed
}
```

Supported types: `NewColumn`, `NewBar` (horizontal), `NewLine`, `NewPie`,
`NewDoughnut`, `NewRadar`, `NewScatter` (via `AddXYSeries`), `NewArea`, and
`NewCombo` (combination). Cached values (`c:numCache` / `c:strCache`) are
populated from the supplied data; `c:f` references are built against a
configurable `DataRef` sheet (default `Sheet1`).

Formatting is opt-in and symmetric with the constructors:

- `SetDataLabels(true)` emits `c:dLbls` (showVal) so each point's value renders
  on the chart. It round-trips through `Charts()` (`Chart.DataLabels()`).
- `Series.SetColor("#1F77B4")` gives a series a solid RGB fill
  (`c:spPr` / `a:solidFill` / `a:srgbClr`); the leading `#` is optional and the
  value is recovered on read as `Series.Color`.

A **combination chart** mixes series types on a shared category axis. Give each
series a plot type with `Series.SetType` (`KindColumn`, `KindLine`, or
`KindArea`) and, optionally, move it to the right-hand secondary value axis with
`Series.SetSecondaryAxis(true)`:

```go
c := chart.NewCombo().SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
c.AddSeries("Revenue", []float64{100, 120, 140, 160}).SetType(chart.KindColumn)
c.AddSeries("Margin %", []float64{12, 15, 14, 18}).
    SetType(chart.KindLine).SetSecondaryAxis(true)
```

`Charts()` reads a combo back as `KindCombo` with each series' `PlotType` and
`SecondaryAxis` recovered.

All types (including combination charts) flow through every format's `AddChart`
and are read back by `Charts()`.

This package is the shared core. Each format wires it in with an `AddChart`
method and a `Charts()` reader (xlsx references the host sheet; docx and pptx
embed the workbook from `EmbeddedWorkbook`).

#### Charts in spreadsheets

In `xlsx`, `Sheet.AddChart` anchors a chart on a sheet and `Sheet.Charts` /
`Workbook.Charts` read the charts back:

```go
package main

import (
	"bytes"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/xlsx"
)

func main() {
	wb := xlsx.Create()
	sheet := wb.AddSheet("Sales")
	_ = sheet.SetCellValue("A1", "Region")

	c := chart.NewColumn().
		SetTitle("Quarterly Sales").
		SetCategories([]string{"Q1", "Q2", "Q3", "Q4"}).
		SetLegend(chart.LegendRight)
	c.AddSeries("North", []float64{10, 20, 30, 40})
	c.AddSeries("South", []float64{5, 15, 25, 35})

	// Anchor the chart at E2 (a single cell places a default-sized chart; a
	// range like "E2:L20" sizes it to that block).
	_ = sheet.AddChart("E2", c)

	data, _ := wb.SaveBytes()

	wb2, _ := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
	s2, _ := wb2.SheetByName("Sales")
	for _, got := range s2.Charts() {
		_ = got.Title()      // "Quarterly Sales"
		_ = got.Categories() // ["Q1" "Q2" "Q3" "Q4"]
		_ = got.SeriesList() // series names + values
	}
}
```

An xlsx chart references cells in the host workbook rather than an embedded
workbook. `AddChart` writes the chart's data (categories and each series) into a
dedicated hidden worksheet — one per chart — and points the chart's `c:f`
references at it, so Excel's "Edit Data" opens real cells while the cached
values keep the chart rendering standalone; the sheet's own cells are never
touched. Charts and images coexist in one drawing part per sheet. `AddChart`
persists on both the `Create` and `Open` save paths, and a zero-modification
open→save of a chart-bearing workbook stays byte-identical (the chart and
drawing parts are preserved verbatim unless a chart is added or modified).

#### Charts in Word documents

`docx` inserts a chart inline in the text flow, like an inline image. The
chart's data is written to an embedded workbook (`word/embeddings/…xlsx`) that
the chart part references, so Office can open and edit the values — a docx has
no host worksheet. Sizes are in EMUs (914400 per inch):

```go
doc := docx.Create()
doc.AddParagraphWithText("Quarterly revenue:")

c := chart.NewColumn().
	SetTitle("Revenue by Quarter").
	SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
c.AddSeries("North", []float64{10, 20, 30, 40})

// Appends a paragraph holding the chart (~5.5in x 3in). Use
// Paragraph.AddChart to place a chart inline among other runs.
if err := doc.AddChart(c, 5029200, 2743200); err != nil {
	log.Fatal(err)
}
_ = doc.Save("charted.docx")

// Read charts back from an opened document.
opened, _ := docx.Open("charted.docx")
for _, ch := range opened.Charts() {
	fmt.Println(ch.Title(), ch.Categories())
}
```

A zero-modification open→save of a chart-bearing document is byte-identical:
the chart and embedded-workbook parts round-trip verbatim, and are regenerated
only when a chart is added.

#### Charts in PowerPoint

`Slide.AddChart` places a chart on a slide. Because a presentation has no host
workbook, the chart's data is embedded as a small `.xlsx` package that Office
can open to edit the data; the chart part, the embedded workbook, the wiring
relationships, and the content-type overrides are all created for you. Position
and size are given in EMUs.

```go
package main

import (
	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/pptx"
)

func main() {
	p := pptx.Create()
	slide := p.AddSlide()

	c := chart.NewColumn().
		SetTitle("Quarterly Revenue").
		SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
	c.AddSeries("2024", []float64{10, 20, 15, 25})
	c.AddSeries("2025", []float64{12, 18, 22, 30})

	// x, y, width, height in EMUs (914400 EMU = 1 inch).
	if err := slide.AddChart(c, 914400, 1828800, 5486400, 3657600); err != nil {
		panic(err)
	}
	_ = p.Save("charts.pptx")
}
```

Read charts back with `Slide.Charts()` or `Presentation.Charts()`, which return
the parsed `*chart.Chart` definitions (type, title, categories, and series).
A chart added this way coexists with the slide's existing shapes, and opening
and re-saving a chart-bearing deck without changes preserves the chart and
embedding parts byte-for-byte.

### Working with Documents in Memory

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

### Opening and Modifying a Presentation

```go
package main

import (
    "github.com/mgilbir/spine/pptx"
)

func main() {
    // Open an existing presentation
    p, err := pptx.Open("existing.pptx")
    if err != nil {
        panic(err)
    }
    defer p.Close()

    // Modify properties
    p.Properties.Title = "Updated Title"

    // Add a new slide
    p.AddSlide()

    // Save to a new file
    if err := p.SaveAs("modified.pptx"); err != nil {
        panic(err)
    }
}
```

### Creating from a Template

```go
package main

import (
    "github.com/mgilbir/spine/pptx"
)

func main() {
    // Create a new presentation based on a template
    // This preserves the template's masters, layouts, and themes
    p, err := pptx.CreateFromTemplate("template.pptx")
    if err != nil {
        panic(err)
    }
    defer p.Close()

    // Add slides using template layouts
    layout := p.GetLayoutByType(pptx.LayoutTitleAndContent)
    _ = p.AddSlideFromLayout(layout)

    // Save the new presentation
    if err := p.Save("from_template.pptx"); err != nil {
        panic(err)
    }
}
```

### Reading and Writing Slide Comments

`pptx` reads both PowerPoint comment mechanisms — legacy per-slide comments and
modern (2018) threaded comments — through one `*Comment` type, and writes modern
threaded comments (the mechanism current PowerPoint emits, and the only one that
supports replies and resolution). The API mirrors the `docx` and `xlsx` comment
APIs; the anchor (a slide plus an optional position or shape) is pptx-specific.

```go
package main

import (
    "fmt"

    "github.com/mgilbir/spine/pptx"
)

func main() {
    p, err := pptx.Open("deck.pptx")
    if err != nil {
        panic(err)
    }
    defer p.Close()

    // Read every comment across the deck.
    for _, c := range p.Comments() {
        fmt.Printf("%s: %s\n", c.Author(), c.Text())
        for _, reply := range c.Replies() {
            fmt.Printf("  ↳ %s: %s\n", reply.Author(), reply.Text())
        }
    }

    // Add a threaded comment, reply to it, and resolve it. The author is
    // registered in the author list (deduplicated by name).
    slide, _ := p.Slide(0)
    c := slide.AddComment("Reviewer", "Please tighten this section.")
    c.Reply("Author", "Done in the next revision.")
    c.Resolve()

    // Precise placement (EMUs) is available via AddCommentAt.
    slide.AddCommentAt(4572000, 2286000, "Reviewer", "Anchored here.")

    if err := p.Save("deck.pptx"); err != nil {
        panic(err)
    }
}
```

Newly added comments always use the modern threaded mechanism, even on a deck
whose existing comments are legacy (both may coexist in one file). Legacy
comments carry no threading or resolved state, so `Reply` is a no-op returning
`nil` and `Resolve`/`SetResolved` are no-ops on them. A zero-modification
open→save of a comment-bearing deck is byte-identical; only the parts a comment
write touches are regenerated.

### Hyperlinks and Pictures

Hyperlinks are read and written through one `*Hyperlink` type shared with the
`docx` and `xlsx` APIs (`URL`, `Anchor`, `Tooltip`, `SetTooltip`). In pptx a
link lives on a run (`a:hlinkClick` in the run properties) or on a shape
(`p:cNvPr`); the anchor of an internal link is a destination slide number (a
slide jump) or a `ppaction://` verb. `Slide.Pictures()` returns every picture on
a slide with its alt text, bytes, content type, and frame geometry.

```go
p, _ := pptx.Open("deck.pptx")
defer p.Close()

// Read: every hyperlink and every picture across the deck.
for _, h := range p.Hyperlinks() {
    fmt.Printf("url=%q anchor=%q tip=%q\n", h.URL(), h.Anchor(), h.Tooltip())
}
for _, pic := range p.Pictures() {
    fmt.Printf("%s %s (%d bytes)\n", pic.AltText(), pic.ContentType(), len(pic.Data()))
}

// Write: an external link on a run, an internal slide jump on a shape.
slide, _ := p.Slide(0)
run := slide.AddTextBox().TextFrame().AddParagraph().AddRun()
run.SetText("Our site")
run.SetHyperlink("https://example.com").SetTooltip("Open the site")

shape := pptx.NewAutoShape(pptx.PresetRect)
shape.SetSize(914400, 914400)
_ = slide.AddShape(shape)
shape.SetHyperlinkToSlide(2)              // jump to slide 3 (0-based index)
// shape.SetActionHyperlink(pptx.ActionNextSlide) // or a ppaction:// verb
```

Writing an external or slide-jump link allocates the backing relationship in the
slide's rels on save; `ppaction://` verbs need none. A zero-modification
open→save of a hyperlink- or picture-bearing deck is byte-identical, and setting
a hyperlink on a run in an opened slide patches that slide in place without
disturbing the others.

## Opening vs. Creating Documents

`Create` builds a new document from scratch; `Open`/`OpenReader` parse an existing file. Both return the same types with the same mutation API, and edits made after `Open` persist on save: document properties, cell values, text edits, and added slides, sheets, or paragraphs are all written back, while parts you did not touch are preserved byte-for-byte. Known asymmetries that remain:

- pptx: `Create()` produces a 4:3 deck (use `CreateWithOptions` with `SlideSizeWidescreen`, or `CreateWidescreen()`, for 16:9). The baked master and layouts size their placeholders to the slide, so both aspect ratios are internally consistent.
- docx: markup the library does not model is captured raw when a document is opened and preserved verbatim on save, but it is opaque to the API — `Text()` does not see text inside it and `SetText`/`ReplaceText` cannot edit it.
- pptx: master and layout `Placeholders()` and `Theme()` are read-only views; mutating the returned values does not change the saved parts.

## Validation

Every top-level type — `pptx.Presentation`, `docx.Document`, `xlsx.Workbook` — has a `Validate()` method that inspects the current in-memory model (without saving) and returns a `validate.Report`: a slice of structured findings. Each finding carries a stable `Code`, a `Severity` (error or warning), the `Part` it concerns, and a human-readable `Detail`, so callers can triage programmatically rather than parse a string.

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

If a finding is advisory for your use case, `SaveToUnvalidated` writes without the pre-save check.

## Supported Flavors

Each format family comes in several ECMA-376 flavors, distinguished only by the main part's content type. `Open` accepts all of them and a save re-emits the flavor the file was opened with — a slideshow stays a slideshow, and a macro-enabled workbook keeps both its `vbaProject.bin` (preserved verbatim) and its macro-enabled content type. The `Flavor()` accessor on `Presentation`, `Document`, and `Workbook` reports the main part's content type (`opc.ContentType*` constants).

| Package | Flavors opened and round-tripped |
|---------|----------------------------------|
| `pptx`  | presentation (.pptx), slideshow (.ppsx), template (.potx), macro-enabled presentation (.pptm), slideshow (.ppsm), and template (.potm) |
| `docx`  | document (.docx), template (.dotx), macro-enabled document (.docm), and template (.dotm) |
| `xlsx`  | workbook (.xlsx), template (.xltx), macro-enabled workbook (.xlsm), template (.xltm), and add-in (.xlam) |

Documents built with `Create` always save as the regular flavor. Converting a file from one flavor to another (e.g. saving an `.xlsm` as a plain `.xlsx`, which would also need its macro parts stripped) is not supported.

## Package Structure

- `opc/` - Open Packaging Conventions implementation
- `common/` - Shared types and utilities
  - `dml/` - DrawingML types (colors, geometry, fills, lines)
    - `chart/` - Chart types
    - `diagram/` - Diagram types
  - `enum/` - Common enumerations
  - `omml/` - Office Math Markup Language types
  - `oxml/` - Shared Office XML types
  - `vml/` - Vector Markup Language types
  - `xml/` - XML namespace handling and Builder-based serialization
- `chart/` - Public, format-agnostic chart builder, serialization, and reader
- `pptx/` - PowerPoint document support
- `docx/` - Word document support
- `xlsx/` - Excel document support

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

## Slide Layouts

The following standard slide layout types are supported:

| Type | Description |
|------|-------------|
| `LayoutTitle` | Title slide |
| `LayoutTitleAndContent` | Title and content |
| `LayoutSectionHeader` | Section header |
| `LayoutTwoContent` | Two content areas |
| `LayoutComparison` | Comparison layout |
| `LayoutTitleOnly` | Title only |
| `LayoutBlank` | Blank slide |
| `LayoutContentWithCaption` | Content with caption |
| `LayoutPictureWithCaption` | Picture with caption |
| `LayoutTitleAndVerticalText` | Title and vertical text |
| `LayoutVerticalTitleAndText` | Vertical title and text |

## Testing

Unit tests run against both small synthetic fixtures (committed to git) and a set of real-world Office files sourced from the internet. The external files are used for round-trip compatibility testing: each file is parsed, serialized back, and compared byte-for-byte against the original.

External fixtures are not checked into the repository. To download them:

```bash
make fetch
```

This reads `testdata/external.txt` (a list of destination paths and URLs) and downloads any missing files. Fetching is best-effort by default: unreachable fixtures are reported and skipped rather than failing the run. Use `make fetch-strict` (or `bash testdata/fetch.sh --strict`) to treat any download failure as an error, and `bash testdata/fetch.sh --force` to re-download everything. Four fixtures have no known public URL (commented out in `external.txt` with `# URL unknown`) and cannot be fetched at all.

Tests that depend on an external fixture skip silently when the file is absent, so a green run on a fresh clone exercises fewer cases than one with all fixtures fetched. A few pptx tests additionally use fixtures from the python-pptx test suite; see [`testdata/README.md`](testdata/README.md) for how to obtain that optional corpus.

A much larger optional corpus of real-world files harvested from Common Crawl is available via `make fetch-cc`: committed manifests pin thousands of candidate documents per format from a single crawl, the fetcher materializes up to 1000 of each locally (gitignored, never redistributed) — plus, optionally, files above Common Crawl's 1 MiB truncation limit refetched from their origin behind a DNS-over-HTTPS blocklist gate — and `go test ./cctest` runs the open/save/reopen/part-fidelity discipline over them, with known failures cataloged in a quarantine file. A plain `go test ./cctest` checks a fast deterministic subset (60 files per format) so the whole suite stays inside Go's default package timeout; `make test-corpus` runs the complete corpus (~15-20 minutes). See [`testdata/cc/README.md`](testdata/cc/README.md) for the pipeline, politeness, and licensing details.

To run the full test suite (fetches external files first):

```bash
make test
```

Native Go fuzz targets cover the Open paths of `opc`, `pptx`, `docx`, and `xlsx` (malformed zip archives and hostile XML inside otherwise-valid packages must produce errors, never panics). `make fuzz` runs a short smoke pass over every target; see [CONTRIBUTING.md](CONTRIBUTING.md) for deeper `-fuzztime`-driven runs.

To lint (requires golangci-lint v2.x):

```bash
make lint
```

## Requirements

- Go 1.25 or later

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test, and fixture instructions, and please feel free to submit issues and pull requests.
