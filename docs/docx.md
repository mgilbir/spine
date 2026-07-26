# Word (DOCX)

This guide is the detailed reference for the `docx` package: creating and
modifying Word documents. Reach for it when you need the full feature surface
(styles, tables, tracked changes, comments, footnotes, mail merge, content
controls, fields) or the code walkthroughs for commenting, hyperlinking, and
annotating a document. The one-paragraph quick start lives in the
[README](../README.md#creating-a-word-document); this document carries
everything else.

## Capabilities

### Structure and text

- Create documents from scratch
- Add headings, paragraphs, and tables
- Rich text formatting (bold, italic, color, font, size)
- Paragraph alignment, spacing, and indentation
- Bullet and numbered lists, plus custom numbering definitions via `Document.Numbering()` (per-level format, level text, start, and indent/hanging)
- Style definitions via `Document.Styles()`: create and modify paragraph and character styles (id/name, based-on, next, link, quick-format, and the style's font/size/bold/italic/color, alignment, spacing, and indentation)
- Headers and footers
- Rich run formatting: highlight, super/subscript, caps/small-caps, underline style + color, character spacing/kerning, and paragraph tab stops
- Paragraph borders and shading; run character styles (`Run.SetStyle`) and symbol glyphs (`Run.AddSymbol`)

### Graphics and objects

- Watermarks: text (`SetTextWatermark`) and washed-out image (`SetImageWatermark`) watermarks in the page header, read back with `Watermark()` and cleared with `RemoveWatermark()`; target a specific section with `SetSectionTextWatermark`/`SetSectionImageWatermark` for distinct per-section watermarks, or emit a DrawingML text box (with a VML fallback) via `WatermarkOptions.DrawingML`
- Inline and floating (anchored) images — including SVG images with a raster fallback
- Text boxes and basic shapes (rectangle, rounded rectangle, ellipse, line), inline or anchored, with fill/border and text as real WordprocessingML paragraphs — `Paragraph.AddTextBox`/`AddShape` and `Document.AddTextBox`/`AddShape`; read every box (DrawingML and legacy VML) back with `Document.TextBoxes()`. Text boxes can optionally carry a down-level VML fallback (`TextBoxOptions.VMLFallback`, an `mc:AlternateContent` Choice+Fallback pair)
- WordArt (`Paragraph.AddWordArt`/`Document.AddWordArt`) — a DrawingML text effect with a solid fill and an optional preset text warp (arch, circle, inflate, wave, ...), inline or anchored
- Shape groups (`Paragraph.AddShapeGroup`/`Document.AddShapeGroup`) — several shapes/text boxes combined into one `wpg:wgp` group, each member positioned in the group's coordinate space
- Embedded OLE objects (`Paragraph.AddOLEObject`/`Document.AddOLEObject`) — embed an object stream as a package part with its content type, `oleObject` relationship, a presentation-icon image, and a `w:object` reference declaring the ProgID
- Charts (column, bar, line, pie, doughnut, radar, scatter, area, combo, bubble, stock, surface, pie-of-pie, and 3D variants) inserted inline via `AddChart`, each carrying an embedded workbook so Office can edit the data; read back with `Charts()` — see [charts.md](charts.md)

### Navigation and references

- Fields (PAGE/NUMPAGES) and a table of contents
- Hyperlinks: external and internal (bookmark-anchored) links, read and written through the `Hyperlink` type shared with the pptx/xlsx APIs (`Document.Hyperlinks`, `Paragraph.AddHyperlink`/`AddInternalHyperlink`)
- Bookmarks: read (`Document.Bookmarks`) and add (`Paragraph.AddBookmark`, `Document.AddBookmarkOnRange`) named bookmarks — the anchors internal hyperlinks target
- Footnotes and endnotes: enumerate (`Document.Footnotes`/`Endnotes`) and author (`Run.AddFootnote`/`AddEndnote`) notes
- Bibliography and citations: read/write bibliography sources (`word/bibliography/sources.xml`, `b:Sources`) with `Document.AddSource`/`Sources()`/`RemoveSource` (tag, type, author, title, year, city, publisher), and cite a source with `Paragraph.AddCitation(tag)`, which emits a `CITATION` field with a formatted placeholder

### Forms, merge, and signatures

- Legacy form fields: enumerate every text/checkbox/dropdown form field (`Document.FormFields()`, walking the body, tables, headers, and footers) with its name, value, checkbox state, and dropdown entries; author new ones with `Paragraph.AddFormField(FormFieldOptions{...})`, which emits the `w:fldChar` begin/separate/end sequence with a `w:ffData` definition. Fields read from a file round-trip byte-identical
- Mail merge: read/write the merge configuration (`Document.MailMerge`/`SetMailMerge`) — main-document type, data source, and `w:odso` field mappings — plus `Paragraph.AddMergeField` and `Document.MergeFields()` for MERGEFIELD fields
- Signature lines: insert a visible "Microsoft Office Signature Line" placeholder with `Document.AddSignatureLine`/`Paragraph.AddSignatureLine(SignatureLineOptions{Signer, Title, Email, Instructions})` and read them back with `Document.SignatureLines()` (the in-document signature request, distinct from signing the package with `opc.SignPackage`)

### Sections and page setup

- Page setup (size, margins), multi-column sections, page-number format/start, title-page, and section type; enumerate sections with `Document.Sections()`
- Section depth: page borders, line numbering, vertical alignment, paper source, document grid, and per-section footnote/endnote numbering (position, format, start, restart)
- Document settings: default tab stop, even/odd headers, zoom, document variables (`Document.SetDocumentVariable` and friends), and document-level footnote/endnote numbering defaults (`Document.SetFootnoteProperties`/`SetEndnoteProperties`, complementing the per-section variant)

### Tables

- Table depth: vertical cell merge (`TableCell.SetVerticalMerge`, complementing horizontal `SetGridSpan`), table look/layout/indent/alignment, and read accessors for table and cell borders, width, and shading

### Review, controls, and structure

- Document protection (`Document.Protect`): read-only, comments-only, tracked-changes, or forms editing modes, with an optional password
- Tracked changes (revisions): enumerate insertions, deletions, run/paragraph property changes, and tracked moves (`w:moveFrom`/`w:moveTo`) — across the main body and every header/footer part — with `Document.Revisions()` (author, date, type, text; `Revision.MoveName` links the two halves of a move), then apply or discard them with `Revision.Accept()`/`Reject()` or `Document.AcceptAllRevisions()`/`RejectAllRevisions()`; author new ones with `Paragraph.AddInsertedRun`, `Run.MarkInserted`, `Run.MarkDeleted`, and `Paragraph.AddMoveFromRun`/`AddMoveToRun` (each assigns a unique `w:id`; `...WithDate` variants pin the timestamp)
- Comments: read and write threaded comments with replies and resolve (`Document.Comments`, `Paragraph.AddComment`, `Document.AddCommentOnRange`, `Comment.Reply`/`Resolve`), through the `Comment` type shared with the pptx/xlsx APIs
- Content controls (structured document tags): read and edit tag, alias, type, value, and drop-down options through `Document.ContentControls()`; insert new ones with `Document.AddContentControl` / `Paragraph.AddContentControl` (block-level and inline); bind a control to a custom-XML node with `ContentControl.SetDataBinding(xpath, storeItemID)`
- Document structure parts: read and author building blocks / AutoText in the glossary (`Document.BuildingBlocks()` — name, gallery, category, types, guid — and `Document.AddBuildingBlock(BuildingBlockDef{...})`, which creates the glossary part, its relationship, and content-type override or appends a `w:docPart` to an existing one, preserving the existing entries verbatim), read and add custom-XML data parts (`Document.CustomXMLParts()` / `Document.AddCustomXMLPart` — the latter generates the itemProps, relationships, and content-type override), and read and author the web-layout frameset tree (`Document.Frameset()` and `Document.SetFrameset(FramesetDef{...})` — nested framesets and leaf frames with their source documents, wired as external frame relationships). Glossary, custom-XML, and frameset parts round-trip byte-for-byte when untouched; an authored building block's body is a placeholder paragraph (the API models building-block metadata, not body content)
- Theme read/write (`Document.Theme()`): color-scheme accents and major/minor Latin fonts, sharing the `dml.ThemeEditor` model with the xlsx and pptx theme APIs
- Search & replace (`Document.ReplaceText`, mirroring `pptx` and `xlsx`): replaces across the body, tables, structured document tags, and every header/footer, including matches Word has split across multiple `w:r` runs — the replacement inherits the first affected run's formatting while surrounding runs keep theirs; documents with no matching text round-trip byte-for-byte

## Merging and appending documents

Combine or divide packages with automatic id, part-name, and relationship
remapping (no dangling references or duplicate parts). `Document.Append` appends
another document's body content with its images, remapped styles/numbering
(including numbering preserved as raw XML in an opened source), and the
header/footer parts referenced by the section breaks it copies.

## Text extraction

`docx.Document.Text()` returns body paragraphs incl. hyperlink/inserted-run/content-control
text, tables cell-by-cell, headers/footers, footnotes/endnotes, and text boxes,
in document order — plain concatenation, no markup, deterministic ordering. This
is the docx half of the symmetric, read-only "give me all the text" API shared
with `xlsx.Workbook.Text()` / `Sheet.Text()` and `pptx.Presentation.Text()` /
`Slide.Text()`.

## Document properties

Core (`docProps/core.xml`), extended/app, and custom (`docProps/custom.xml`)
properties are supported across all three formats. `CustomProperties()`,
`SetCustomProperty(name, value)`, and `RemoveCustomProperty(name)` on
`docx.Document`, `xlsx.Workbook`, and `pptx.Presentation` read and write
user-defined properties typed as string, int64, float64, bool, or time.Time; the
part, its content type, and package relationship are created on demand, and
existing properties round-trip byte-identically when untouched.

## Reading and writing comments

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

## Hyperlinks, images, bookmarks, and footnotes

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
