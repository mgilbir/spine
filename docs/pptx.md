# PowerPoint (PPTX)

This guide is the detailed reference for the `pptx` package: creating and
modifying PowerPoint presentations. Reach for it when you need the full feature
surface (shapes, tables, charts, animations, transitions, SmartArt, comments,
sections, embedded media) or the code walkthroughs for opening, templating,
commenting, and hyperlinking a deck. The one-paragraph quick start lives in the
[README](../README.md#creating-a-powerpoint-presentation); this document carries
everything else.

## Capabilities

### Slides and structure

- Create presentations from scratch or from templates
- Add, remove, reorder, and duplicate slides (`Slide.Duplicate` deep-copies the slide's shapes, slide-level relationships, notes slide, and any generated auto-play/animation timing into a new slide)
- Slide sections (the thumbnail-pane groups): read (`Presentation.Sections()`), create (`AddSection`), and assign slides (`Section.AddSlide` / `Presentation.MoveSlideToSection`), stored in the `p14:sectionLst` extension
- Zoom objects (Slide, Section, and Summary zooms) — read (`Slide.ZoomLinks`/`Presentation.ZoomLinks`): each zoom's kind, hosting shape, and target slide id or section GUID(s); zoom frames round-trip byte-for-byte. Creating a zoom is not supported (a zoom embeds a rendered thumbnail image of its target that the library cannot generate)

### Shapes, text, and tables

- Add shapes, text, tables, and images — including SVG images with a raster fallback
- Slide placeholders, and read-only access to each master's and layout's placeholders and theme (color and font schemes)
- Auto shapes with solid/gradient fills, lines, and shadows
- Connectors (`Slide.AddConnector`/`Slide.Connectors`, and `GroupShape.AddConnector`/`GroupShape.Connectors` inside a group): straight, elbow, and curved connection shapes bound to two shapes' connection sites (ids resolved on save) or drawn between free points, with line width/color/dash; decks with existing connectors round-trip byte-for-byte
- Shape effects on auto shapes and text boxes — glow (`SetGlow`/`Glow`), reflection (`SetReflection`), soft edge (`SetSoftEdge`), and a basic 3D bevel (`SetBevel`), each read back and written to `a:effectLst`/`a:sp3d`
- Slide, master, and layout background fills (`SetBackgroundFill`/`BackgroundColor`/`HasBackground`/`ClearBackground`), reusing the shared `dml.Fill` (solid, gradient, or pattern), plus image (blip) backgrounds (`SetBackgroundImage(data, contentType)`) that embed a media part and stretch it behind the slide/layout/master
- Table styling: built-in/theme table-style reference (`Table.SetStyleID`) and per-cell text insets (`TableCell.SetMargins`)
- Rich text depth: text-frame autofit, auto-numbered bullets with bullet color/size/font, and paragraph indent/tab stops
- Read every picture on a slide (`Slide.Pictures()`), with alt text, bytes, content type, and position/size

### Slide furniture

- Slide furniture: footers, auto-updating or fixed dates, and slide numbers on every slide
- Read the presentation's footer, slide-number, and date furniture set on slides

### Embedded media

- Embedded video and audio (`Slide.AddVideo`/`AddAudio`): store the clip as a `/ppt/media` part wired by both a Microsoft `media` embed and an OOXML video/audio link relationship, with a generated poster preview; media plays on click by default, and `SetPlayMode(PlayAutomatically)` generates the autoplay timing tree. An omitted content type is sniffed from the data

### Masters, layouts, and fonts

- Slide master / layout editing: per-level master text styles (`SlideMaster.TitleStyle`/`BodyStyle`/`OtherStyle` with `SetLevelFont`/`SetLevelFontSize`/`SetLevelBold`/`SetLevelItalic`/`SetLevelColor`/`SetLevelBullet`, including adding a level absent from the source in correct schema order), master/layout placeholder geometry (`EditablePlaceholders`/`EditablePlaceholder` with `SetPosition`/`SetSize`), and adding a layout under a master (`SlideMaster.AddLayout`) — unedited masters and layouts round-trip byte-for-byte and an edit touches only its own part
- Embedded fonts (`Presentation.EmbeddedFonts`/`SetEmbeddedFonts` to reference existing font parts, `EmbedFont(name, regular, bold, italic, boldItalic)` to create the font-data parts and relationships from raw bytes) and custom slide shows (`Presentation.CustomShows`/`AddCustomShow`), read and written

### Transitions and animations

- Slide transitions (fade, push, wipe, circle, comb, newsflash, pull, random-bar, strips, wedge, zoom, and more) with direction/orientation, wheel spokes, through-black, and sound actions — a stop-previous action and a start sound authored from raw audio bytes (`TransitionSound.StartSoundData`) or read back from a file; the PowerPoint 2016+ **morph** transition (`TransitionMorph` with `MorphOption` for object/word/character morphing), written as the `p159:morph` extension wrapped in `mc:AlternateContent` with a fade fallback and read back through `Transition()`
- Slide animations: entrance (appear, fade, fly-in, wipe, zoom), emphasis (pulse, spin, grow/shrink), and exit (disappear, fade, fly-out) effects with on-click / with-previous / after-previous sequencing and optional build-by-paragraph, authored via `Slide.AddAnimation` and read back with `Slide.Animations()`

### SmartArt and diagrams

- Read SmartArt / diagrams (`Slide.SmartArt()` / `Presentation.SmartArt()`): each graphic's text nodes and their hierarchy from the diagram data part (`dgm:dataModel`), returned as a `SmartArtNode` tree via `SmartArt.Nodes()`; the raw diagram parts round-trip byte-for-byte
- Create SmartArt (`Slide.AddSmartArt(kind, nodes...)`) for the list (`SmartArtList`), hierarchy/org-chart (`SmartArtHierarchy`), left-to-right process (`SmartArtProcess`), and radial cycle (`SmartArtCycle`) kinds: generates all four definition parts (`dgm:dataModel`/`layoutDef`/`styleDef`/`colorsDef`), the content-type overrides, slide relationships, and the `dgm:relIds` graphicFrame so Office renders the diagram; returns a `SmartArt` whose `Nodes()` reads the outline back

### Notes, comments, hyperlinks, and search

- Speaker notes: read (`Slide.Notes()`) and write (`Slide.SetNotes`) the notes-slide body text; editing rewrites only the affected notes part while untouched notes slides round-trip byte-for-byte
- Comments: read both legacy per-slide comments and modern threaded comments, and write threaded comments with replies and resolve (`Presentation.Comments`, `Slide.AddComment`/`AddCommentAt`, `Comment.Reply`/`Resolve`), through the `Comment` type shared with the docx/xlsx APIs
- Hyperlinks on runs and shapes (external URLs, internal slide jumps, and `ppaction://` verbs), read and written through one unified `Hyperlink` type shared with the docx/xlsx APIs
- Search & replace (`Presentation.ReplaceText` / `Slide.ReplaceText` / `Slide.ReplaceTextInShape`, mirroring `docx` and `xlsx`): exact-match replacement across shapes, table cells, and nested groups — including matches split across multiple runs — rewriting the XML in place to preserve formatting; speaker notes are not touched

## Merging and splitting decks

Combine or divide packages with automatic id, part-name, and relationship
remapping (no dangling references or duplicate parts). `Presentation.AppendSlidesFrom`
/ `Presentation.ExtractSlides` copy slides and their media/charts/embeddings
between decks, carrying each slide's own layout, master, theme (deduplicating
identical masters/layouts), notes slide, and the source deck's notes master and
handout master (when the destination has none).

## Text extraction

`pptx.Presentation.Text()` / `Slide.Text()` return all shapes incl. groups and
tables, plus speaker notes and comments — plain concatenation, no markup,
deterministic ordering. This is the pptx half of the symmetric, read-only "give
me all the text" API shared with `docx.Document.Text()` and
`xlsx.Workbook.Text()` / `Sheet.Text()`.

## Embedded OLE objects

Read embedded OLE objects (`embeddings/oleObjectN.bin`) with `OLEObjects()`,
returning each object's name, content type, raw bytes, and best-effort ProgID;
embed new objects with `docx.Paragraph.AddOLEObject` or
`pptx.Slide.AddOLEObject(data, progID, opts…)` (which writes the object part, a
`p:oleObj` graphic frame with a fallback preview picture, and the wiring
relationships); unmodified objects round-trip verbatim.

## Ink annotations and 3D models

Read/extract pen-stroke ink annotations and embedded 3D models.
`Slide.InkAnnotations()`/`Presentation.InkAnnotations()` (pptx) and
`Document.InkAnnotations()` (docx) enumerate ink — the InkML content parts
(`application/inkml+xml`, referenced by a `contentPart` element through a
`customXml` relationship) — reporting each part's name, content type, raw InkML
bytes, and referencing relationship id. `Slide.Model3D()`/`Presentation.Model3D()`
(pptx) and `Document.Model3D()` (docx) extract the opaque glTF-binary 3D model
parts (`model/gltf-binary`, e.g. `media/*.glb`, referenced by an `am3d:model3D`
element), returning each part's name, content type, and raw bytes. Extraction is
read-only and both kinds of part round-trip byte-for-byte; spine never parses
stroke geometry or model data. Authoring new ink strokes and embedding new 3D
models are not yet supported.

## Opening and modifying a presentation

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

## Creating from a template

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

## Reading and writing slide comments

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

## Hyperlinks and pictures

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

// Connect two shapes with an elbow connector (ids are bound on save).
other := pptx.NewAutoShape(pptx.PresetEllipse)
_ = slide.AddShape(other)
conn := slide.AddConnector(pptx.ConnectorElbow)
conn.Connect(shape, 3, other, 1)          // bind start/end to connection sites
conn.SetLineWidth(2)
conn.SetLineColor(dml.NewRGB(0xC0, 0x00, 0x00).ToColor())
// Or draw a free-floating line between two points:
// conn := slide.AddConnector(pptx.ConnectorStraight)
// conn.SetPoints(0, 0, dml.Inches(3), dml.Inches(2))
```

Writing an external or slide-jump link allocates the backing relationship in the
slide's rels on save; `ppaction://` verbs need none. A zero-modification
open→save of a hyperlink- or picture-bearing deck is byte-identical, and setting
a hyperlink on a run in an opened slide patches that slide in place without
disturbing the others.

## Slide layouts

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

The exact `Layout*` constant names are the source of truth in [`pptx/layout.go`](../pptx/layout.go).
