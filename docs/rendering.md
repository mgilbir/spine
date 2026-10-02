# Native rendering

Implementation is in progress in stacked draft PRs. The `render` package can
prepare a caller-supplied Forme display list and write PNG or SVG. `pptx.Slide.PrepareRender` supports a first static slide profile. `xlsx.Sheet.PrepareRender` supports bounded range previews. `docx.Document.PrepareRender` prepares a selected physical page from a bounded
plain document flow.

```go
page, err := render.Prepare(ctx, dml.Inches(8.5), dml.Inches(11), ops, render.Limits{})
if err != nil { return err }
return page.WritePNG(ctx, writer, 144)
```

PNG is painted directly in Go; SVG is a sibling writer. There is no browser,
external process, SVG conversion or PDF intermediate. Forme is the only added
dependency. PDF/pdf0 has not been included.

Snapshots own copied geometry and normalized image pixels, are safe for concurrent
rendering, and survive later source edits. Fonts and image buffers must remain
immutable during preparation. Font parsing belongs to the caller's provider.
There is no ambient host-font lookup, external relationship loading or formula
execution. Unsupported operations fail even if their bounds fall off the page.

Supported drawing operations are solid rectangles, filled Forme paths, nested
path clips, horizontal positioned glyphs and contextual horizontal text, and
validated standard-library raster images. Color/bitmap/SVG fonts, vertical text,
letter spacing, tiling, gradients, strokes, effects and other operations fail
explicitly. PNG/JPEG decoding checks dimensions and bytes before decoding.
Sixteen-bit image sources are reduced to eight bits; EXIF orientation is not
applied. Text is outlined in SVG, so it is not selectable.

Both writers use CSS-pixel geometry (96 per inch) and upwards-rounded output
sizes; zero DPI means 96. PNG uses sRGB source-over with pixel-centre nearest-
neighbour bitmap sampling. Rectangle edges use analytic area coverage. Paths
use eight vertical samples and analytic horizontal intervals, with curves
flattened to a 1/16 output-pixel tolerance. Independent primitive antialiasing
can differ from a vector viewer or Office at touching/overlapping edges. SVG
keeps curves and requests pixelated image sampling; the viewer controls its
final antialiasing. No Office pixel identity is claimed.

Zero limit fields select these defaults; negative limits fail:

| Limit | Default |
|---|---:|
| Maximum output/source-image dimension | 8192 |
| Output pixels | 16 Mi |
| Drawing operations | 100000 |
| Pixel visits including overdraw | 64 Mi |
| Encoded output bytes | 32 MiB |
| Source/flattened path segments | 100000 each |
| Clip depth | 32 |
| Scanline/coverage work | 64 Mi units |
| Glyphs | 100000 |
| Logical text metadata | 1 MiB |
| Fonts / aggregate program bytes | 32 / 32 MiB |
| Charged text shaping work | 64 Mi units |
| Input bytes per shaped run, including context/features | 4096 |
| Unique source-image pixels | 4 Mi |
| Images / aggregate normalized PNG bytes | 32 / 32 MiB |

`Options.MaxSourceBytes` and `MaxLayoutNodes` default to 16 MiB and 100000 for
adapter capability inspection; node accounting includes attributes. Drawing/pixel limits remain independent.

Sizing and painting work are checked before allocating output pixels. Context
checks cover preparation, lookup steps, painting and output writes. Individual
font reads, Unicode preprocessing, image decoding and PNG compression are not
interruptible inside their routines; input/dimension/work bounds limit them.
A caller's blocking font resolver or writer must honour cancellation itself.
Output errors, byte limits or cancellation may leave a partial PNG/SVG. For
atomic output, use a caller-owned buffer under a separate byte budget.

## Static slide profile

`slide.PrepareRender(ctx, render.Options{})` includes unsaved edits and returns
an independent page with PNG/SVG writers. A selected hidden slide is allowed.
The canvas starts white; the nearest of the slide, layout and master
backgrounds applies. A background is a solid fill, no fill, or a theme
background reference (`p:bgRef`) whose theme entry is a solid fill or no fill.
Rectangles, rounded rectangles (`roundRect` with a literal `adj` adjustment)
and ellipses may have a solid fill or none, and a solid outline or none. A shape
has no style reference (`p:style`, which fails), so an absent fill or outline,
or an outline without a fill, is none, as DrawingML defines. A solid outline
needs an explicit width, a single line, and no dashes beyond `solid`; it is
centered on the boundary or inset (`algn="in"`) and painted as the even-odd ring
between the boundary's offsets, so rounded corners stay exact arcs. Sharp
corners need an explicit miter, bevel or round join, with a miter limit below
√2 beveling. Offsetting an ellipse does not yield an ellipse, so only circles
are outlined. Text lays out in the preset's text rectangle, which for rounded
rectangles and ellipses is inset from the corners. Uncropped embedded PNG/JPEG pictures with
rectangular geometry are supported. Geometry is quantized to Forme's fixed-point
units during the EMU-to-CSS conversion.

Fill and background colors may be RGB, system colors (their recorded `lastClr`)
or theme scheme colors. Scheme colors, including inherited ones, resolve through
the slide's effective color map: the master map, replaced by any layout and
then slide override. The master's theme is read once per preparation, with
unsaved theme edits, and counts toward `MaxSourceBytes`/`MaxLayoutNodes`.
`lumMod` and `lumOff` apply in HSL in document order with clamping after each
step; other transforms, alpha, and theme style entries with gradient, pattern
or picture fills fail.

Other theme style references, gradients, strokes, rotated or
flipped shapes/pictures, image crops/SVGs, groups, connectors, charts, tables,
SmartArt, effects, animation and alternate/raw drawing content fail explicitly.
Visible inherited master/layout shapes fail. Title/body placeholder definitions
are not independently painted; other inherited placeholder types fail. Original
slide, layout and master XML is checked for unsupported content before a lossy
model projection can hide it. Metadata that cannot change painted output is
accepted: shape and slide creation ids, the decorative accessibility flag,
editor guide lists, the picture local-DPI storage flag, run language, proofing,
smart-tag and bookmark attributes, `rtlCol` on the single-column body, and
master/layout header-footer flags (footer placeholders themselves still fail).
Any other extension URI, or a known URI under a different owner, fails. Empty
effect lists, which PowerPoint writes where the schema requires effect
properties, paint nothing; any effect in them fails. These source checks can conservatively reject a
feature removed by a pending edit. Source inspection and drawing budgets are
separate, and decoded pictures are bounded cumulatively before decode. A
slide decodes each distinct image (media part, or image bytes set through the
API) once and charges its bytes and pixels once; every picture still counts
toward `MaxImages`.

Preparation does not call SaveBytes or synchronize shapes to source XML. Do not
race source edits with preparation. Returned snapshots can be rendered concurrently.

Plain horizontal ASCII text is supported in non-placeholder rectangles and
text boxes. Provide `render.Options.Fonts`; the renderer performs no ambient
font discovery. The renderer lays out the text body a save would write,
including pending edits. A non-placeholder body inherits nothing, so absent
attributes take their DrawingML defaults: top anchoring, square wrapping, and
0.1"/0.05" left-right/top-bottom insets. Top, middle and bottom anchoring place
the text block, whose height spans its paragraphs' spacing and full line
heights; justified and distributed anchoring, other wrapping, and vertical,
rotated or multi-column text fail. Shape autofit (`spAutoFit`) and unscaled
normal autofit render at the stored extent PowerPoint fitted, and a line that
measures below it is still drawn; scaled autofit fails. A fixed frame fails
when its text measures past the bottom inset.
Paragraph and run properties are inherited: the paragraph's own properties,
then the shape's list style at the paragraph's level, then the document
defaults, then DrawingML defaults (left alignment, 100% line spacing, no space,
no bullet, 18 pt, no bold, italic, underline or strike). Whether PowerPoint
consults the master's other-text style before the presentation default text
style is not specified and implementations differ, so each property is resolved
both ways and must agree; PowerPoint writes the two alike. A list style's
`defPPr` and a paragraph's own `defRPr`, which PowerPoint may ignore, must also
agree with the value that applies without them. Disagreement fails. A deck
built in code saves the library's default styles, whose level 2–9 sizes
disagree. Theme font references (`+mn-lt`, `+mj-lt`) resolve through the
master's theme and colors through the slide's color map. Inherited styles are
checked for unsupported content only when a slide has text; the presentation
default text style counts toward the source budget.

The resolved paragraph must be left, centered or right aligned with percentage
line spacing, point-based space before and after (none before the first
paragraph, whose treatment depends on undocumented `spcFirstLastPara`
behavior), no bullet, no indent, left/right margins within the box, and
left-to-right Latin word breaking. Runs in a paragraph must resolve to one font
family, size (1–4000 pt), bold/italic setting and kerning, and so shape alike;
each run has its own solid color and optional highlight. A paragraph wraps as
one string and each line is cut back into runs; a glyph standing for
characters of two runs, such as a ligature, fails, since PowerPoint shapes runs
apart. A highlight fills the run's advance from the line's ascent to its
descent beneath the text, with touching spans of one color painted as one.
Underline, strike, capitals, baseline shift, character spacing, outline,
effects and hyperlinks fail. Kerning applies from the resolved `kern` size,
and is off when it is absent or zero. An empty paragraph takes its line box from
its end-of-paragraph properties; a paragraph with runs takes it from them, as
LibreOffice's import does. Rich styles, hard breaks, fields, tabs, bidi,
non-ASCII characters and unresolved fonts fail.

Forme measures wrapping and shapes final lines under cumulative budgets. Native
line metrics use the supplied font's hhea ascent, descent and line gap; baseline
placement and percentage spacing use those metrics. This is a defined native
layout profile, not a claim of identical PowerPoint line placement. DrawingML
[percentage line spacing](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.drawing.linespacing)
scales with text size; fixed-point line spacing and percentage before/after
paragraph spacing are not supported by this adapter.

## Sheet range profile

`sheet.PrepareRender(ctx, "A1:D10", opts)` snapshots that logical range at 96 CSS
pixels per inch, without UI headers or print pagination. The first profile
requires the library's default stylesheet and explicit widths on every selected
column. Supply the Normal font through `opts.Fonts` (Calibri, 11 pt); substitution
is controlled by the caller. The native maximum digit advance is rounded to a
96-DPI pixel, then the [SpreadsheetML column width formula](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.spreadsheet.column)
is applied. Row heights use their explicit or sheet-default point sizes; the
created-sheet fallback is 15 pt. Hidden rows/columns are omitted. Grid lines
use a fixed light gray and respect `showGridLines`.

Plain ASCII strings may extend through blank neighbours inside the range.
General integers with fewer than ten decimal digits are right aligned; booleans
are centred. Supported cached formula values follow the same rules, without
evaluation. Other numbers, number formats, rich strings, alternate styles,
merges, conditional formatting, drawings and other worksheet features fail.
Horizontal/vertical overflow fails instead of silently clipping a value or
changing its formatting. Glyph overhang is clipped to the permitted cell span.

Preparation checks original worksheet/styles/shared-string XML, bounds source
indexing and range size, and creates no missing cells. The snapshot includes
unsaved values. Original source checks remain conservative for pending edits.
Native font/grid metrics do not promise Excel pixel identity.

## Word physical-page profile

`document.PrepareRender(ctx, 1, opts)` selects a **1-based physical page** after
laying out the complete document under shared font/text/glyph/work budgets. The
first profile supports one section with explicit page size and nonnegative
margins, plain ASCII paragraphs, one explicit run style per paragraph,
left/center/right alignment, exact line spacing and zero before/after paragraph
spacing. Runs require explicit font family, size, bold, italic, strike-off and
RGB color. Supplied fonts determine native hhea ascent/descent; leading is split
equally above/below the line. Font metrics must fit the chosen line and page.

The flow honours `pageBreakBefore` and [widow/orphan control](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.widowcontrol),
including its enabled default. A paragraph that cannot satisfy those constraints
on the given page fails. Paragraph keeps, indentation, rich styles, tables,
headers/footers, columns, tracked changes, fields, drawings and unsupported
source markup fail. Document settings and inherited/default style formatting
outside the profile also fail. Unreferenced non-default named styles are ignored.
The first profile keeps optional OpenType ligatures, contextual alternates and
kerning disabled, matching [Word's default OpenType-feature setting](https://learn.microsoft.com/en-us/openspecs/office_standards/ms-docx/116847ff-9af6-45a7-a21f-0e0ff2eef41d).

Page selection does not skip validation or layout of later paragraphs. Unsupported
document content on another page still returns an error. Preparation checks the
original main-part stream before lazy projection and never saves the document.
Returned snapshots include unsaved changes. Physical-page numbering is unrelated
to visible page-number fields. Native line placement and pagination are defined
by this profile; identical Word pagination is not promised.

## Runnable previews

Run `go run ./examples/render_previews -out render-previews` to write PNG and SVG
previews of a slide, a sheet range and two physical document pages at 144 DPI.
The example uses Forme's embedded Noto Sans and explicitly substitutes it for
the sheet's Calibri request. It needs no host fonts or external processes.

`MaxShapeWork` measures conservative lookup-work units, including lookup subtable
bytes; it is not a duration. This example explicitly raises that budget for its
known embedded font. Untrusted input retains the default limit unless the caller
chooses a different bounded budget.

## File-rendering CLI

Build `go build -o spine-render ./cmd/spine-render`, then supply an input file and
output directory. PNG is the default; `-format both` also writes SVG.

```sh
./spine-render -input deck.pptx -out previews -font 'Calibri=fonts/Calibri.ttf'
./spine-render -input report.docx -out previews -format both -font 'Calibri=fonts/Calibri.ttf'
./spine-render -input workbook.xlsx -out previews -range A1:D20 -sheet Data -font 'Calibri=fonts/Calibri.ttf'
```

PPTX renders every slide. DOCX renders every physical page, with 1-based output
names. XLSX requires an explicit range to bound its canvas; the range applies to
all sheets unless `-sheet` selects one by exact name. Sheet filenames use their
original 1-based workbook indices, avoiding unsafe names from source content.

Repeat `-font` for additional families or styles, e.g.
`-font 'Calibri:bold=fonts/Calibri-Bold.ttf'`. Files are loaded explicitly through
Forme, with at most 32 mappings and 32 MiB aggregate font input. No host fonts
are searched. `-fallback-noto` explicitly substitutes embedded Noto Sans for
unresolved regular faces; bold/italic still require mappings. Substitution can
change wrapping and overflow. For known fonts needing a larger conservative
work budget, choose e.g. `-shape-work 1073741824`.

`-dpi` defaults to 144; `-max-pages` defaults to 100 (maximum 10000), and
`-timeout` defaults to one minute. Interrupt cancels rendering. Library package,
source, shaping, pixel and output limits still apply. DOCX currently lays out the
whole document for each selected page, so large documents repeat layout work.
The timeout and page cap bound this command's processing.

The CLI uses the strict profiles above; it does not expand their supported
formatting. Errors identify the failing page, slide or sheet. Existing outputs
are never overwritten. A failed output file is removed; completed files from
earlier pages or the other format remain available after a later error.
