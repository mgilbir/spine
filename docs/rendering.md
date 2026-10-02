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

Slide previews are strict by default: content outside the profile fails
preparation. Setting `render.Options.Warn` makes them best effort instead.
Each shape that cannot be drawn, on the slide, its layout or master, or in a
group, is reported to `Warn` and left out; a shape whose text cannot be laid
out is drawn without it; an unsupported background is drawn white; and
problems outside any shape (unknown slide content, alternate content,
extensions, charts and other graphic frames) are reported once. Animation and
transitions are ignored. Some details are drawn approximately instead, and
reported once per shape as such: shape, connector and text effects (shadows,
glows, 3-D) are left out; baseline shifts and character spacing are ignored;
text wider than its box, such as an overlong word, runs past it; and
arrowheads are drawn as described for connectors. The page is then
incomplete, and the rules below describe what is drawn. Cancellation, malformed parts and page-wide limits
still fail.

`slide.PrepareRender(ctx, render.Options{})` includes unsaved edits and returns
an independent page with PNG/SVG writers. A selected hidden slide is allowed.
The canvas starts white; the nearest of the slide, layout and master
backgrounds applies. A background is a solid or gradient fill, no fill, or a
theme background reference (`p:bgRef`) whose theme entry is one of those.
Rectangles, rounded rectangles (`roundRect` with a literal `adj` adjustment)
and ellipses may have a solid or gradient fill or none, and a solid outline or none. A shape's
style reference (`p:style`) supplies what it does not set itself: `fillRef`
selects a theme fill or background fill style, which must be solid, gradient or none, in
the reference's color; `lnRef` a theme line style beneath the shape's own
`a:ln`; `fontRef` the theme major or minor font and a text color, beneath the
shape's own list style; `effectRef` must select an effect style without
effects. Without a style an absent fill or outline is none, as DrawingML
defines. A `line` preset is drawn as a connector from corner to corner, its
flips choosing the corners. A solid outline
needs an explicit width and a single line; it is
centered on the boundary or inset (`algn="in"`) and painted as the even-odd ring
between the boundary's offsets, so rounded corners stay exact arcs. Sharp
corners need an explicit miter, bevel or round join, with a miter limit below
√2 beveling. Offsetting an ellipse does not yield an ellipse, so only circles
are outlined. Preset dashes (`dash`, `sysDot`, …) use the
ST_PresetLineDashVal patterns in line widths, starting where the preset path
starts and running clockwise; the pattern restarts there, so the last dash may
be short. Dashes have flat caps: the schema gives no default cap and PowerPoint's
shape styles use flat, so an absent cap is flat, and square or round caps fail,
as do custom dashes. A dash turning a sharp corner takes the line join. Each
dash is painted as its own path. Text lays out in the preset's text rectangle, which for rounded
rectangles and ellipses is inset from the corners. Embedded PNG/JPEG pictures with
rectangular geometry are supported, cropped by their source rectangle
(`a:srcRect`); a negative crop, which extends the picture, fails. Geometry is quantized to Forme's fixed-point
units during the EMU-to-CSS conversion.

Fill and background colors may be RGB, system colors (their recorded `lastClr`)
or theme scheme colors. Scheme colors, including inherited ones, resolve through
the slide's effective color map: the master map, replaced by any layout and
then slide override. The master's theme is read once per preparation, with
unsaved theme edits, and counts toward `MaxSourceBytes`/`MaxLayoutNodes`.
Color transforms apply in document order, clamping after each step: hue,
saturation and luminance transforms and `comp` in HSL; `tint`, `shade`, `gray`
and the red, green and blue transforms in linear RGB, as LibreOffice's import
does; `inv`, `gamma` and `invGamma` on the sRGB channels; and the alpha
transforms on opacity. Theme style entries with pattern or picture fills fail.

A gradient (`a:gradFill`) fills its shape's box. A linear gradient's angle
runs clockwise from the x axis and its stops span the box corner to corner
along it; `scaled` applies the angle to the box stretched to a square. A path
gradient's first stop lies at the box edge and its last at the centre of its
fill-to rectangle: `circle` exactly, and `rect` and `shape` approximately, as
ellipses, in best-effort mode only. Stop positions are clamped to 0–100%;
tiling and flipping fail, as best effort leaves them out. Outlines and text
with gradient or pattern fills fail; best effort draws them in a gradient's
middle stop color or a pattern's foreground. Unfilled text is invisible.
Spine's PNG and SVG writers paint single-tile linear and radial gradients with
linear blending.

Rotated or
flipped shapes/pictures other than lines, SVGs, groups, connectors, charts, tables,
SmartArt, effects, animation and alternate/raw drawing content fail explicitly.
Master and layout shapes are drawn beneath the slide's, master first, in their
document order: shapes and pictures through the same profile as slide content,
their text and colors resolved as the slide's. Placeholders on masters and
layouts are prompts and are never drawn. A slide with `showMasterSp="0"` hides
its layout's and master's shapes, and a layout with it hides its master's;
hidden layers still supply backgrounds. Their shapes are checked against the
source profile only when drawn, so unsupported content in a placeholder or a
hidden layer does not fail the slide. Inherited groups, tables and connectors
fail when drawn. Original
slide, layout and master XML is checked for unsupported content before a lossy
model projection can hide it. Metadata that cannot change painted output is
accepted: shape and slide creation ids, the decorative accessibility flag,
editor guide lists, the picture local-DPI storage flag, run language, proofing,
smart-tag and bookmark attributes, `rtlCol` on the single-column body,
master/layout header-footer flags (footer placeholders themselves still fail),
editor locks and resize preferences, `userDrawn`, an extension list's `mod`
flag, a picture fill's `rotWithShape` and `dpi`, and `bwMode`, which applies
only to black-and-white output. Best effort also skips animation and
transitions in layouts and masters.
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

Plain horizontal left-to-right text is supported in shapes, placeholders and
text boxes. Provide `render.Options.Fonts`; the renderer performs no ambient
font discovery. The renderer lays out the text body a save would write,
including pending edits. A non-placeholder body inherits nothing, so absent
attributes take their DrawingML defaults: top anchoring, square wrapping, and
0.1"/0.05" left-right/top-bottom insets. Top, middle and bottom anchoring place
the text block, whose height spans its paragraphs' spacing and full line
heights. Without wrapping (`wrap="none"`) each line keeps its natural
width, aligned in the box as wrapped text is. Justified and distributed
anchoring, and vertical, rotated, clipped or multi-column text fail; best
effort draws them top anchored, horizontal, whole and in one column, and
ignores `anchorCtr`, WordArt warps and 3-D text. Shape autofit (`spAutoFit`) and unscaled
normal autofit render at the stored extent PowerPoint fitted, and a line that
measures below it is still drawn; scaled autofit fails. A fixed frame fails
when a line that draws glyphs measures past the bottom inset; an empty line may
hang below unseen. A body without characters paints nothing and is not laid
out, so its fonts need not resolve.
Slide placeholders inherit from the layout placeholder with the same `idx`
(of the same type when several share it, or the only one of the same type when
none does) and that from the master placeholder of its base type (`title` for
title and centered title, the same type for date, footer, slide number and
header, `body` otherwise); more than one candidate fails. Geometry, fill,
outline, effects and body attributes merge property by property, nearest
first, and the list styles chain slide, layout and master placeholder, then
the master's title, body or other text style. Whether the presentation default
text style follows is unspecified, so the result must agree with and without
it. A matched layout or master placeholder must be free of unsupported content
outside its prompt paragraphs; placeholders with field
text fail, and a placeholder with no geometry anywhere is invalid. An edited
placeholder renders as a save writes it. A spacing percentage of zero is
accepted as no spacing.

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
behavior), left/right margins within the box, and left-to-right Latin word
breaking. A first-line indent needs a character bullet, which hangs in it: the
bullet is drawn at the margin plus the (negative) indent on the first baseline
and every line's text starts at the margin, so the indent must hold the
bullet; a bullet past it would push the text to a tab stop this profile does
not place. Bullet character, font (`buFont`, or the text's), size (`buSzPct`,
`buSzPts`, or the text's) and color (`buClr`, or the first run's) inherit
separately; a bullet taller than its line fails, numbered and picture bullets
fail, and an empty paragraph shows no bullet. Runs may differ in font family, size (1–4000
pt), bold/italic setting and kerning; consecutive runs that agree on these shape
as one span. Break opportunities come from the whole paragraph, while shaping
context stays within a span, so no glyph or contextual form crosses a change of
font or size; the paragraph's shaping budget is shared evenly between its
faces. Each line's box takes the largest ascent, descent and line gap on that
line, with percentage line spacing scaling the whole, and every span sits on
the shared baseline. Each run has its own solid color and optional highlight;
a glyph standing for characters of two runs, such as a ligature, fails, since
PowerPoint shapes runs apart. A highlight fills the run's advance from its
font's ascent to its descent about the baseline, beneath the text, with
touching spans of one color painted as one. Underline, strike, capitals, baseline shift, character spacing, outline,
effects and hyperlinks fail. Kerning applies from the resolved `kern` size,
and is off when it is absent or zero. An empty paragraph takes its line box from
its end-of-paragraph properties; a paragraph with runs takes it from them, as
LibreOffice's import does. A line break (`a:br`) starts a new line; an empty
line takes its box from the break's properties, or after a trailing break
from the end-of-paragraph properties. A field (`a:fld`) is drawn with the text
it was saved with, which a viewer may update, such as a date. Rich styles, tabs, bidi,
unresolved fonts fail. Text may use Latin, Greek and Cyrillic letters,
combining diacritics, Latin-1, general punctuation, currency and letterlike
symbols, arrows, mathematical operators and geometric shapes, which DrawingML
draws with the Latin font. PowerPoint may draw symbols of ambiguous East Asian
width with the East Asian font in Chinese, Japanese or Korean text, so a run
whose resolved language or alternate language is one of those may hold only
ASCII. Other characters, including
right-to-left and East Asian text, soft hyphens, controls and format characters,
fail.

Tables (`a:tbl` in a graphic frame) render when they have no table style,
merged cells, diagonal borders or table-level fill, and when they are parsed
from the file without pending edits: the save path rewrites the domain model's
cells, which preparation must not do, so a new or edited table fails until it
is saved and reopened. Column widths come from the grid; a row is as tall as
its stored height or its tallest cell's text plus margins, whichever is
larger. Cell fills, then borders, then cell text are painted. A border is a
solid line centered on its grid line, extended half its width where another
border meets it so corners close. Where adjacent cells set a shared edge
differently, or differing borders meet at a grid point, rendering fails: the
precedence is undocumented. Cell text uses the slide text profile, laid out
with the cell's margins and anchor; a cell `a:bodyPr` may only repeat them.

Groups draw their shapes and pictures with geometry mapped from the group's
child space to its frame; text sizes and line widths do not scale, as
PowerPoint draws them. Rotated or flipped groups, group fills and effects, and
placeholders, tables and connectors inside groups fail; groups are drawn only
from their parsed form without pending edits.

Straight connectors (`straightConnector1` or `line`) draw their stored
geometry, horizontally or vertically flipped but not rotated; bindings to other
shapes move a connector only when those shapes move. The line comes from the
theme line style the connector's `lnRef` selects, its `phClr` taking the
reference color, with any property the connector's own `a:ln` sets winning; an
effect reference must select an empty theme effect style. Lines take flat,
square or round caps and preset dashes with flat caps. Arrowheads fail: the
specification names their sizes but not their geometry. In best-effort mode
every head is drawn as a filled triangle whose tip is the line's end, sized
like LibreOffice's (small, medium and large are two, three and five line
widths, at least a pixel each), with the line stopping halfway into it. Bent and curved
connectors fail, and like tables, connectors are drawn only from their parsed
form without pending edits.

Tables without a table style are drawn unstyled. This is provisional:
`tableStyles.xml` names a default style, often a built-in Office style the file
does not define, and whether PowerPoint applies it to a table without
`a:tableStyleId` is undocumented. Explore that, and built-in style definitions,
before supporting styled tables.

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
change wrapping and overflow.

`-shape-work` bounds shaping per slide, page or sheet in the conservative lookup
units described above. The command defaults to 16 Gi units instead of the
library's 64 Mi, which suits a single paragraph: complete fonts charge about a
million units per byte of text (embedded Noto Sans) while shaping it in
microseconds, so 64 Mi stops a slide after about 60 characters. The document
controls only the amount of text, the fonts are the caller's, and `-timeout`
bounds the whole command; lower the budget for fonts you do not trust. Likewise
`-edge-checks` bounds path painting per output (scanline edge tests and
coverage samples) and defaults to 1 Gi instead of 64 Mi: a slide of text,
circles and outlined boxes needed up to 256 Mi at 144 DPI and painted in about
a quarter of a second.

`-dpi` defaults to 144; `-max-pages` defaults to 100 (maximum 10000), and
`-timeout` defaults to one minute. Interrupt cancels rendering. Library package,
source, shaping, pixel and output limits still apply. DOCX currently lays out the
whole document for each selected page, so large documents repeat layout work.
The timeout and page cap bound this command's processing.

The CLI draws slides best effort by default: each piece of content it cannot
draw prints a `warning:` line naming the slide, and the rest is drawn. Use
`-strict` to fail instead. It does not expand the supported formatting. Errors identify the failing page, slide or sheet. Existing outputs
are never overwritten. A failed output file is removed; completed files from
earlier pages or the other format remain available after a later error.

`-keep-going` reports each slide or sheet that cannot be prepared on standard
error, skips it, renders the rest, and then exits with an error naming the
skipped pages. A cancelled or timed-out run still stops, and write failures are
never skipped. DOCX lays out one document for every page, so its failures
repeat across pages and stop the run as before.
