# Native rendering

Implementation is in progress in stacked draft PRs. The `render` package can
prepare a caller-supplied Forme display list and write PNG or SVG. `pptx.Slide.PrepareRender` supports a first static slide profile. `xlsx.Sheet.PrepareRender` supports bounded range previews. Word page
preparation is still being implemented.

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
The canvas starts white; explicit opaque RGB backgrounds inherit from master to
layout to slide. Rectangles and ellipses require explicit RGB/no-fill styling
and an explicit absent outline (`AutoShape.SetNoLine`). Uncropped embedded
PNG/JPEG pictures with rectangular geometry are supported. Geometry is quantized
to Forme's fixed-point units during the EMU-to-CSS conversion.

Theme/style references, color transforms, gradients, strokes, rotated or
flipped shapes/pictures, image crops/SVGs, groups, connectors, charts, tables,
SmartArt, effects, animation and alternate/raw drawing content fail explicitly.
Visible inherited master/layout shapes fail. Title/body placeholder definitions
are not independently painted; other inherited placeholder types fail. Original
slide, layout and master XML is checked for unsupported content before a lossy
model projection can hide it. These source checks can conservatively reject a
feature removed by a pending edit. Source inspection and drawing budgets are
separate, and decoded pictures are bounded cumulatively before decode.

Preparation does not call SaveBytes or synchronize shapes to source XML. Do not
race source edits with preparation. Returned snapshots can be rendered concurrently.

Plain horizontal ASCII text is supported in non-placeholder rectangles and
explicitly styled text boxes (`TextBox.SetNoLine` suppresses their outline). Provide
`render.Options.Fonts`; the renderer performs no ambient font discovery. The
frame must have top anchoring, square wrapping, explicit insets and no autofit.
Each paragraph must explicitly select no bullet, left/center/right alignment and
positive percentage line spacing and explicit before/after spacing (including
zero). Runs in a paragraph must share an explicit
literal font family, size (12–4096 pt), bold/italic setting, opaque RGB color, and
explicit no-underline/no-strike settings. Rich styles, hard breaks, tabs, bidi,
non-ASCII characters, unresolved fonts and overflowing text fail. Empty paragraphs
need a styled run so their height is defined.

Forme measures wrapping and shapes final lines under cumulative budgets. Native
line metrics use the supplied font's hhea ascent, descent and line gap; baseline
placement and percentage spacing use those metrics. This is a defined native
layout profile, not a claim of identical PowerPoint line placement. DrawingML
[percentage line spacing](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.drawing.linespacing)
scales with text size; fixed-point line spacing and percentage before/after
paragraph spacing are not supported by this first adapter.

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
