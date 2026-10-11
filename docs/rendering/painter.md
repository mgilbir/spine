# The painter

This page explains how spine paints a prepared page: the display list
`render.Prepare` accepts, the drawing operations it supports, how PNG and
SVG output is antialiased and sampled, how cancellation reaches it, and how
color fonts are drawn. It is for readers who draw their own content or need
to know why output differs from Office at the pixel level.
It is part of the [rendering guide](../rendering.md), which has the quick
start and the entry points.

At the lowest level, `render.Prepare` takes a Forme display list and page size
and returns the same `render.Page`. The [format renderers](../rendering.md) build that list
for you; call it directly only to draw your own content:

```go
page, err := render.Prepare(ctx, dml.Inches(8.5), dml.Inches(11), ops, render.Limits{})
if err != nil { return err }
return page.WritePNG(ctx, writer, 144)
```

PNG is painted directly in Go; SVG is a sibling writer. There is no browser,
external process, SVG conversion or PDF intermediate. Rendering adds
[Forme](https://github.com/mgilbir/forme) (layout and text) and
[gowemf](https://github.com/mgilbir/gowemf) (EMF/WMF pictures) to the module's
dependencies; `cmd/spine-render` also links aster, which draws charts and
brings pdf0 in indirectly. The library writes no PDF.

Snapshots own copied geometry and normalized image pixels, are safe for concurrent
rendering, and survive later source edits. Fonts and image buffers must remain
immutable during preparation. Font parsing belongs to the caller's provider.
There is no ambient host-font lookup, external relationship loading or formula
execution. Unsupported operations fail even if their bounds fall off the page.

Supported drawing operations are solid rectangles, filled Forme paths, nested
path clips, single-tile linear and radial gradient fills, horizontal
positioned glyphs and contextual horizontal text, and
validated standard-library raster images. Vertical text,
letter spacing, tiling, strokes, effects and other operations fail
explicitly; color fonts are drawn as described under Color fonts. PNG, JPEG and
GIF (its first frame) decoding checks dimensions and bytes before decoding.
Sixteen-bit image sources are reduced to eight bits; EXIF orientation is not
applied. Text is outlined in SVG, so it is not selectable.

Both writers use CSS-pixel geometry (96 per inch) and upwards-rounded output
sizes; zero DPI means 96. PNG uses sRGB source-over. A bitmap is filtered over
each output pixel's square: averaged by area along an axis where the pixel
spans more than one bitmap pixel, and interpolated linearly between the two
nearest bitmap pixels where it spans less, in premultiplied alpha, its edges
extending outwards. Rectangle edges use analytic area coverage. Paths
use eight vertical samples and analytic horizontal intervals, with curves
flattened to a 1/16 output-pixel tolerance. Independent primitive antialiasing
can differ from a vector viewer or Office at touching/overlapping edges. SVG
keeps curves and requests pixelated image sampling (not for a glyph's bitmap); the viewer controls its
final antialiasing. No Office pixel identity is claimed.

Sizing and painting work are checked before allocating output pixels. Context
checks cover preparation, lookup steps, painting and output writes. Individual
font reads, Unicode preprocessing, image decoding and PNG compression are not
interruptible inside their routines; input/dimension/work bounds limit them.
A caller's blocking font resolver or writer must honour cancellation itself.
Output errors, byte limits or cancellation may leave a partial PNG/SVG. For
atomic output, use a caller-owned buffer under a separate byte budget.

## Color fonts

A glyph with colors of its own is drawn in them, from the tables Forme paints:
COLRv0 layers and COLRv1 paint graphs, with CPAL's first palette and the
text's color for what the font fills in the foreground (its alpha is the
text's times the font's, and the text's alpha touches nothing else); and CBDT
and sbix PNG strikes, from the smallest strike at least as large as the font
size in CSS pixels, or the largest, scaled to the glyph's box and smoothed.
Other glyphs of the same font, and glyphs of fonts with no such table, are
outlines in the text color as before. Each fill of a color glyph is a drawing
of the page inside the glyph's clips, which are outlines or boxes carried
through the paint graph's transforms, so PNG and SVG output draw it as they
draw any clipped fill; outlines clip by the nonzero rule, as fonts fill them.

These are drawn exactly: solid fills, with the font's alpha; linear gradients
under any transform, with padded, repeated and reflected color lines, stops
in any order and past either end; radial gradients between circles about one
center (any radii, in either direction, in any extend mode) under a transform
that keeps their circles circles or axis-aligned ellipses (a uniform scale, a
turn, a stretch along the axes, and their products); and groups composited
source-over, or whose backdrop alone shows. A repeating color line is laid out
as the stops its area reaches, up to 4,096 of them. Gradients blend
premultiplied in PNG output, as the font specification has it; SVG viewers
blend as SVG does.

What cannot be drawn exactly is refused with `render.ErrUnsupported` in strict
preparation. In best effort (`render.PrepareBestEffort`, and `render.Options.Warn`
for slides), each of these is drawn approximately and reported once, wrapping
`render.ErrApproximated`:

- a sweep gradient, a color line with no extent that repeats or reflects, a
  degenerate gradient (circles of one radius, a line along its own side) and a
  color line repeating more than 4,096 stops over its area, as the average of
  its stops' colors, premultiplied and weighted by their stretch;
- a radial gradient between circles of different centers, about the end
  circle's center, and one turned or skewed into a tilted ellipse, with
  axis-aligned radii of the same extents;
- a glyph composited in any other mode (multiply, screen, source-in and so on),
  a glyph of an SVG table, and a bitmap glyph whose image is not a readable PNG
  or that is turned or skewed, as its outline in the text color (nothing at all,
  for a font with no outlines);
- a monochrome or greyscale bitmap glyph (a mask of a bitmap font) scaled from the
  strike it is drawn from, drawn scaled;
- turned or warped text in a bitmap font, whose glyphs have no outline to turn,
  left out.

A glyph is drawn whole in one of these ways, never part by part. Text turned
or warped in a slide is drawn as outlines, so its color glyphs are outlines in
the text color, and reported as such; a slide's shadows and other effects take
a color glyph's coverage. Only slides and `render.PrepareBestEffort` draw
color glyphs approximately. XLSX previews have no best effort at all. A DOCX page is drawn strictly even
when `Warn` is set: `docxrender.Prepare` lays the document out best effort,
but `Pages.Page` prepares each page's drawing with strict `render.Prepare`, so
a color glyph that can only be drawn approximately fails that whole page with
`render.ErrUnsupported`, not wrapped in `render.ErrApproximated`, where a slide
would draw it approximately and warn. A font with bitmap strikes (`EBDT`, `EBLC`, and Apple's `bdat`, `bloc`)
and outlines (`glyf`, `CFF `, `CFF2`), as system fonts such as Courier New that
carry strikes for small screen sizes are, is drawn from its outlines. A font
with strikes and no outlines (forme's `BitmapOnly`) is drawn from them: each
glyph's monochrome or greyscale bitmap is a coverage mask painted in the text's
color, with the text's alpha times the coverage, through the path color bitmaps
take. Strikes of 1, 2, 4 and 8 bits are read as forme reads them, and a glyph
with no ink (a space) draws nothing. Turned or warped text is outlines, which
such a font has none of: strict preparation refuses it with
`render.ErrUnsupported`, and best effort leaves those glyphs out, reports it
once wrapping `render.ErrApproximated`, and draws the rest of the slide.

A mask glyph of a bitmap font is drawn exactly only when it is not resampled:
its strike is the size asked for (`shape.Image.Exact`: the font size in CSS
pixels, rounded up, must be a strike's pixels per em, so a size with a fraction
is never exact), the text is not stretched, and it is drawn at one device pixel
to the CSS pixel (96 DPI output; PNG at another DPI and SVG at any size resample
the pixels, as they do for any image). A glyph that has to be scaled is refused
with `render.ErrUnsupported` in strict preparation and drawn scaled and
smoothed, reported once wrapping `render.ErrApproximated`, in best effort. The
two kinds of strike differ on purpose: a monochrome or greyscale strike is a
pixel design for one size, which scaling distorts, while a CBDT or sbix strike
is made to be scaled (Noto Color Emoji has one strike, of 109 pixels), so
scaled color bitmaps are drawn smoothed, in strict mode too, and are not
reported. Reading a strike is bounded by forme's work budget for each glyph,
and its image counts against the image pixel and byte limits, once for each
glyph, size and text color.

Color glyphs are bounded as every drawing is. Forme refuses a paint graph
deeper than 64 paints, with more than 16,384 paint edges or past its work
budget before drawing any of it, and that is `render.ErrLimit`. The drawings a
glyph makes count as operations, its clips and gradient stops as path segments,
and its clips against the clip depth; its clips and gradients cost edge checks
and pixel visits when painted. A glyph's PNG strike is decoded under the image
limits (dimensions, pixels and bytes) once per page, glyph and size, and counts
toward unique image pixels and bytes but not toward the count of pictures, so a
slide of many emoji is bounded by its pixels. A strike is chosen for the size at
preparation, not for the DPI a page is painted at.
