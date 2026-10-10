# Native rendering

Implementation is in progress in stacked draft PRs. The `render` package can
prepare a caller-supplied Forme display list and write PNG or SVG. `pptxrender.PrepareSlide` (package `pptx/pptxrender`) supports a first static slide profile. `xlsxrender.PrepareRange` (package `xlsx/xlsxrender`) supports bounded range previews. `docxrender.Prepare` and `docxrender.PreparePage` (package `docx/docxrender`) lay out a Word document and prepare its physical pages.

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
a color glyph's coverage. DOCX and XLSX previews have no best effort and
refuse what cannot be drawn exactly. A font with bitmap strikes (`EBDT`, `EBLC`, and Apple's `bdat`, `bloc`)
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

## Static slide profile

Slide previews are strict by default: content outside the profile fails
preparation. Setting `render.Options.Warn` makes them best effort instead.
Each shape that cannot be drawn, on the slide, its layout or master, or in a
group, is reported to `Warn` and left out; a shape whose text cannot be laid
out is drawn without it; an unsupported background is drawn white; and
problems outside any shape (unknown slide content, alternate content,
extensions, charts and other graphic frames) are reported once. Animation and
transitions are ignored. Some details are drawn approximately instead, and
reported once per shape as such: the effects of a shape, connector or
picture, its own or its style's theme effects. A sharp outer shadow is drawn
exactly, beneath the shape as its drawing in the shadow color, offset by the
shadow's distance and direction. Effects that blur are drawn from a raster of
the shape at up to three pixels per CSS pixel (one million pixels an effect,
eight million a slide), blurred with an approximate Gaussian of half each
radius: a blurred outer shadow, the shape's coverage offset and blurred, with
scaling and skewing left out; a reflection, the shape flipped below its box,
faded from its start to its end opacity and blurred; a glow, the coverage
blurred and doubled in the glow color beneath the shape; soft edges, the
shape's own pixels faded over the radius inside its edge; an inner shadow,
the uncovered area offset, blurred and kept inside the shape; and a blur
effect, the shape itself blurred, past its box unless `grow` is off. Office
does not document how it draws them. Fill overlays, preset shadows, effect
graphs, 3-D and text effects are left out;
text wider than its box, such as a single character, runs past it; and
arrowheads are drawn as described for connectors. The page is then
incomplete, and the rules below describe what is drawn. Cancellation, malformed parts and page-wide limits
still fail.

`pptxrender.PrepareSlide(ctx, slide, render.Options{})` includes unsaved edits and returns
an independent page with PNG/SVG writers. A selected hidden slide is allowed.
The canvas starts white; the nearest of the slide, layout and master
backgrounds applies. A background is a solid or gradient fill, no fill, or a
theme background reference (`p:bgRef`) whose theme entry is one of those.
Pattern fills (`a:pattFill`) on shapes and backgrounds are drawn exactly, in
strict mode too: the 8 by 8 bitmaps of all 54 presets are measured from
PowerPoint's own rendering of them, in the foreground color over the
background color (black over white when absent). A pattern pixel is one
point, 4/3 CSS pixels, and the tiling is anchored to the slide's origin, not
the shape's, so shapes at different places show one pattern; a shape's
position includes its group's mapping as drawn. The fill is composed at 3
image pixels per CSS pixel (4 per pattern pixel), or 1.5 or 1 when the box is
too large for that within the tiled-fill pixel budget, as a slide-sized
background is. At 3 and 1.5, output at 144 and 288 DPI lands pattern edges on
whole pixels; at 96 DPI a pattern pixel is 1.33 pixels, so edges blur
whatever the scale. The drawing is placed to 1/64 CSS pixel. A pattern is
never turned or mirrored: a rotated or flipped shape, or one in a rotated or
flipped group, shows the slide's upright pattern under it, as PowerPoint
draws it, so the fill is composed again over the turned shape's bounds and
clipped to its outline. Best effort reports problems with a background as the background's, and draws a
background it cannot draw white.
Rectangles, rounded rectangles (`roundRect` with a literal `adj` adjustment)
and ellipses may have a solid, gradient or picture fill or none, and a solid outline or none. A shape's
style reference (`p:style`) supplies what it does not set itself: `fillRef`
selects a theme fill or background fill style, which must be solid, gradient or none, in
the reference's color; `lnRef` a theme line style beneath the shape's own
`a:ln`; `fontRef` the theme major or minor font and a text color, beneath the
shape's own list style; `effectRef` must select an effect style without
effects. Without a style an absent fill or outline is none, as DrawingML
defines. A `line` preset is drawn as a connector from corner to corner, its
flips choosing the corners. A solid outline
needs an explicit width and a single line (best effort draws a line without
one, which DrawingML draws as thin as the device allows, a pixel wide); it is
centered on the boundary or inset (`algn="in"`) and painted as the even-odd ring
between the boundary's offsets, so rounded corners stay exact arcs. Sharp
corners need an explicit miter, bevel or round join, with a miter limit below
√2 beveling; best effort miters them. Offsetting an ellipse does not yield an
ellipse, so only circles are outlined; best effort draws other ellipses' rings
between ellipses with offset radii. Preset dashes (`dash`, `sysDot`, …) use the
ST_PresetLineDashVal patterns in line widths, starting where the preset path
starts and running clockwise; the pattern restarts there, so the last dash may
be short. Dashes have flat caps: the schema gives no default cap and PowerPoint's
shape styles use flat, so an absent cap is flat; square or round dash caps
on shape outlines fail, and best effort draws them flat. Custom dashes fail. A dash turning a sharp corner takes the line join. Each
dash is painted as its own path. Text lays out in the preset's text rectangle, which for rounded
rectangles and ellipses is inset from the corners. Embedded PNG, JPEG and GIF pictures with
rectangular geometry are supported, cropped by their source rectangle
(`a:srcRect`); a negative crop extends the picture's box past its image,
which shows the image smaller with the rest of the box empty. A picture's
flips mirror its pixels and quarter turns rotate them, turning its box about
its centre; best effort draws other rotations unrotated, and leaves out
picture effects. A picture's solid outline runs around its box. An SVG picture
draws its raster fallback, as Office versions without SVG support show it. A
picture's color effects, and a picture fill's, apply in document order to
its pixels: grayscale (`grayscl`) and bi-level by Rec. 601 luminance;
`duotone` between its two colors by that luminance; color replacement
(`clrRepl`) and change (`clrChange`, exact matches, with `useA`); `hsl`
shifts; the alpha effects (`alphaModFix`, `alphaRepl`, `alphaBiLevel`,
`alphaCeiling`, `alphaFloor`, `alphaInv`); and solid fill overlays in their
blend mode. Brightness and contrast (`lum`), which the standard does not
define, follow LibreOffice; tint and gradient overlays are approximated, as
is blur, an approximate Gaussian of half its radius kept within the picture's
box; alpha masks (`alphaMod`) are left out. Strict mode refuses those, and best
effort reports them. A picture is downscaled, by averaging, to at most
four pixels per CSS pixel it is drawn at. A picture placeholder without its
own geometry takes its layout's, or master's, placeholder geometry. A picture
background (`a:blipFill` in `p:bgPr`) is stretched over the slide or tiled,
and cropped by its source rectangle. A shape's picture
fill (`a:blipFill` in `p:spPr`) is stretched over its box, inset by its fill
rectangle (`a:fillRect`), or tiled over it, cropped by its source rectangle
and clipped to the shape; it flips and turns with the shape, and its fixed
color effects apply as a picture's. A tile is the picture's natural size, at the
fill's `dpi`, the file's resolution (PNG `pHYs`, JPEG JFIF density) or 96
DPI, scaled by `sx`/`sy`; the first tile sits at its alignment in the box,
moved by `tx`/`ty`, and the rest repeat from it, every other one mirrored on
each axis `flip` names. Tiles are composed into one image of the picture's
density, up to four pixels per CSS pixel and four million pixels in all.
Best effort stretches a picture fill with neither fill mode, draws a
negative source inset as none. Hidden shapes
(`hidden` on `cNvPr`) are not drawn.

EMF and WMF pictures, as picture shapes, picture fills and picture
backgrounds, are drawn onto a transparent raster of the size the picture is
drawn at, up to two pixels per CSS pixel and never below half; the raster then
goes through the picture machinery above (crop, flips, effects, tiling). The
records are played by [gowemf](https://github.com/mgilbir/gowemf)'s `Play`,
which keeps the GDI device context and the GDI+ graphics state and resolves
every drawing record into paths, paints, images and text runs in raster
coordinates. `internal/metafile` draws those with the page painter's scan
conversion (eight vertical samples, exact horizontal coverage) rather than
through the display list, which fills only by the even-odd rule and has no
strokes or arbitrary clip regions. A metafile fills its picture area (an EMF's
frame, a placeable WMF's bounds, else a WMF's window) over the picture's box.

The raster is the device GDI draws on, as when Office plays a picture onto a
page: GDI lines run through its pixel centres, a pen is never narrower than one
of its pixels and a hairline is one pixel wide. EMF shapes include their right
and bottom edges, as Windows plays them; WMF keeps GM_COMPATIBLE's rule
(MS-EMF 2.1.16) and leaves them out. Drawn exactly, in the sense of this page:
everything gowemf plays (its `COVERAGE.md` is the inventory), among it the
mapping modes, window, viewport and world transforms, saved states; pens with
their caps, joins, user dashes and compound bands; solid, hatched, pattern and
DIB brushes, and in EMF+ GDI+'s 53 hatches, textures, linear and path
gradients; paths, filled, stroked and as clips, and region clips; bitmaps
copied, inverted, alpha-blended or color-keyed, and a black-and-white mask
followed by its sprite as one image with alpha; gradient fills; EMF+ metafile
images; and text in the fonts the caller's `Fonts` resolver supplies. Text is
decoded from Unicode, the Windows code pages and the double-byte character
sets; DEFAULT_CHARSET text is read as a Western system writes it, in
Windows-1252. Each run of one bidirectional level is shaped by forme in that
level's direction, mirrored at odd levels, without the optional ligatures,
contextual alternates and kerning ExtTextOut does not apply to simple text
(complex scripts and right-to-left runs keep the font's rules, as Uniscribe
applies them). Glyph-index text is drawn only in the face it indexes, matched by
family or PostScript name.

An EMF+ file is drawn from its EMF+ records, as GDI+ draws it, when every one
of them can be drawn within the limits; an EMF+ Only file is always drawn from
them. A Dual file whose EMF+ records cannot all be drawn is drawn from the GDI
records it carries for readers that cannot, when gowemf can read those, and the
substitution is reported.
Strict mode fails with `render.ErrUnsupported` for anything not drawn exactly;
best effort draws the rest and reports each detail once, as approximated or left
out: what gowemf reports it cannot play (its `UnsupportedOperation`s, such as
EMF+ custom line caps, which stay off), preset pen dashes (drawn at Windows'
display lengths), a font's set character width (drawn at its own width),
characters a face lacks and glyph indexes in a substitute face (left out), and
raster operations that combine with the destination beyond a mask and sprite
pair (left out: a picture's destination is the page under it, which its raster
does not have). Malformed files fail; best effort stops at a record gowemf
cannot read and keeps what came before it. All budgets apply: the raster's
pixels count against the unique-image pixel budget; drawing operations,
flattened and stroked segments, scan work, pixel visits, clip masks (sixteen
rasters), glyphs, fonts and text shaping (`MaxShapeWork`, `MaxRunBytes`) are
charged to the render limits; gowemf's path points and decoded bitmap pixels
are bounded by `MaxPathSegments` and `MaxImagePixels` across the metafile; and
cancellation is checked in every drawing call and between rows.

Other preset geometries draw from the standard's definitions
(`presetShapeDefinitions.xml` of ECMA-376 Part 1, embedded with only their
guides, text rectangles and paths) through the custom geometry engine below,
the shape's adjustments replacing their defaults; eight guides the standard
writes with a stray operand are corrected. Bent and curved connectors draw
their definitions unfilled.

Custom geometry (`a:custGeom`) evaluates its guide formulas (ECMA-376
§20.1.10.36) over the shape's built-in guides and draws each path:
`moveTo`, `lnTo`, `arcTo` (its angles as seen on the ellipse), and Bézier
curves flattened into 16 segments. Paths fill with the even-odd rule unless
their fill is `none`; lightened and darkened path fills draw plain in best
effort. An outline of a single segment is exact; longer ones fail, and best
effort draws them segment by segment with round joins, dashes restarting at
each segment. The `a:rect` guides give the text rectangle.

Rotation and flips turn and mirror a shape about its box's centre; arcs are
then flattened in 5° steps. A gradient keeps its direction under rotation,
which best effort reports. Text lays out in the unturned shape and turns
with its rotation, and over with a vertical flip, but is never mirrored; a
body's own `rot` turns it further. Turned glyphs are drawn as their outlines,
flattened to a sixteenth of a pixel at 384 DPI, so each costs path segments
(around 60 at body sizes) against `MaxPathSegments`. Outlines are filled
even-odd; a glyph whose contours overlap, which the font fills nonzero, is
reported and drawn even-odd. Geometry is quantized to Forme's fixed-point
units during the EMU-to-CSS conversion.

Fill and background colors may be RGB, system colors (their recorded `lastClr`),
preset colors (CSS color names, `dk`, `lt` and `med` abbreviating `dark`,
`light` and `medium`) or theme scheme colors. Scheme colors, including inherited ones, resolve through
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

Alternate content (`mc:AlternateContent`) in a slide's shape tree draws its
fallback where it stands among the shapes, as a reader without the choices'
extensions shows it; the source check skips the choices and checks the
fallback as if its shapes stood in the shape tree. Alternate content on
layouts and masters draws its fallback the same way, its placeholders
prompts. At the root of a slide, layout or master, alternate content wraps
newer transitions and animation: a fallback of `p:transition` or `p:timing`
only, or none, changes nothing on a static page, and is treated as a
transition, which strict mode refuses and best effort leaves out; other root
fallback content is reported. Charts, SmartArt,
effects, animation and raw drawing content fail explicitly.
Master and layout shapes are drawn beneath the slide's, master first, in their
document order: shapes and pictures through the same profile as slide content,
their text and colors resolved as the slide's. Placeholders on masters and
layouts are prompts and are never drawn. A slide with `showMasterSp="0"` hides
its layout's and master's shapes, and a layout with it hides its master's;
hidden layers still supply backgrounds. Their shapes are checked against the
source profile only when drawn, so unsupported content in a placeholder or a
hidden layer does not fail the slide. Inherited groups, connectors, tables
and charts are drawn as the slide's are; an inherited chart names its part
through its layout's or master's relationships. Original
slide, layout and master XML is checked for unsupported content before a lossy
model projection can hide it. Metadata that cannot change painted output is
accepted: shape and slide creation ids, the decorative accessibility flag,
editor guide lists, the picture local-DPI storage flag, proofing,
smart-tag and bookmark attributes, `rtlCol` on the single-column body,
master/layout header-footer flags (footer placeholders themselves still fail),
editor locks and resize preferences, `userDrawn`, an extension list's `mod`
flag, a picture fill's `rotWithShape` and `dpi`, and `bwMode`, which applies
only to black-and-white output, hidden fills, lines and effects kept for
older editors, the shadow-obscured and Mac text-box flags, and the Designer
element flag. Best effort also skips animation and
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
width, aligned in the box as wrapped text is. Vertical text (`vert`,
`eaVert`, whose Latin characters turn as `vert`'s do, and `vert270`) lays out
across its text rectangle turned a quarter about its centre, the insets
turning with their sides, and turns back with it: `vert` clockwise, its first
line along the right side, and `vert270` anticlockwise. `eaVert` sets East
Asian characters upright, which is not drawn: text of it holding any fails,
and best effort turns those characters with their line, as `vert` does.
Best effort draws `mongolianVert` and the WordArt vertical types as `vert`. Upright text
(`upright`) does not turn with its shape or body rotation. Text in columns
(`numCol`, at most 16) lays out at the column width, the content width less
the spaces between (`spcCol`) shared evenly, and fills each column down to
the bottom before the next, left to right or, with `rtlCol`, right to left;
text that fits one column keeps its anchoring, and text over several is
anchored at the top, which best effort reports. Columns wider than their
box fail, and best effort draws one. Justified and distributed anchoring,
and clipped text fail; best effort draws them top anchored and whole, and
ignores `anchorCtr` and 3-D text. A WordArt warp (`prstTxWarp`) shapes the
text to the paths of its preset, from the standard's
`presetTextWarpDefinitions.xml`, evaluated for the content box with the
warp's adjustments: paths in pairs bound a band of lines each, top and
bottom, and single paths each carry a band of lines, hanging inwards from
the path towards the box's middle; lines go to the band holding their
middle, and each band's text is stretched along its paths. Glyphs are drawn
as outlines, their edges divided to bend with the warp. Office does not
document how it fits text to a warp, so it fails, and best effort reports it.
Shape autofit (`spAutoFit`) and normal
autofit render at the stored extent PowerPoint fitted, and a line that
measures below it is still drawn. Normal autofit's stored `fontScale` scales
every run's size, rounded to hundredths of a point, and its `lnSpcReduction`
reduces percentage line spacing. A fixed frame fails when a line that draws
glyphs measures past the bottom inset, and best effort draws the text past
it; an empty line may hang below unseen. A body without characters paints nothing and is not laid
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
it; best effort uses the reading without it, and the first of several
matching layout or master placeholders. A matched layout or master placeholder must be free of unsupported content
outside its prompt paragraphs; field placeholders fail, and a placeholder with no geometry anywhere is invalid. An edited
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

The resolved paragraph must be left, centered, right, justified or
distributed with percentage line spacing, point-based space before and after
(none before the first paragraph, whose treatment depends on undocumented
`spcFirstLastPara` behavior). Justified lines widen their spaces to fill the
line, except a paragraph's last line and lines ended by a break, as
LibreOffice draws them; distributed lines, the last included, widen every
gap between characters. Spaces ending a line hang and are not widened, and a
tab stop holds what comes before it, so only spaces after a line's last tab
widen. Justified lines also widen the gaps after East Asian characters, which
PowerPoint may do otherwise: such a line fails, and best effort spreads the
characters evenly. Thai distribution draws as distributed, from which it
differs only in Thai, which this profile does not draw. Low kashida
justification elongates Arabic letters, which is not drawn: a line of it with
Arabic fails, and best effort widens its spaces as justified lines do, and
Hebrew and other lines draw as justified. Best effort draws an exact line height (`spcPts`) with the line's glyphs keeping their
ascent-to-descent proportion; space in percent of a line as that share of the
first or last line's height; and space before the first paragraph as given.
The paragraph also needs left/right margins within the box, and left-to-right Latin word
breaking. A right-to-left paragraph (`rtl`) has its lines start at the right.
Its alignment is physical, as PowerPoint draws it (measured from its own
export, not LibreOffice's, which swaps them): with `rtl` an absent `algn` and
`l` put the text against the left edge, `r` against the right, and `ctr`
centres it. Its margins and indent are logical: `marL` is the margin at the
right, the start, and `marR` at the left, so the line box spans from the left
inset to the right inset less `marL`. A hanging bullet sits at the start of its
own line, the right end of its text, the indent (the hanging gap) right of the
text's start edge, so with left alignment the bullet travels with the text and
is right of it. A justified line is stretched edge to edge, with the spaces that
end it hanging past the left, and a line that cannot be stretched, such as the
last, sits at the right; distributed lines are all stretched. The text of a paragraph of either
direction is ordered by the Unicode bidirectional algorithm (UAX #9), with
the paragraph's direction as base: lines break in logical order, and each is cut
by embedding level, white space ending it taking the paragraph's level, and
drawn in visual order, brackets facing the right way. Arabic letters take
their joined forms within a span, as the font's shaping gives them, and a
span is one font and size, so a change of color in a word does not break the
joining while a change of font does. Tabs in a right-to-left paragraph are measured from the line's start, at its right
end, and a stop aligns the text after the tab physically, as PowerPoint's export
showed: a left stop (`l`) puts the left edge of that text at the stop and a right
stop (`r`) its right edge; the whole line is then placed by the paragraph's
alignment, so with an absent `algn` it hugs the left inset. A centred or decimal
stop, a tab past the explicit stops, and where the line is placed otherwise
were not measured: they fail, and best effort draws them by the same reading
(a centred stop symmetric about the stop, a decimal one as a right stop). Tabs
fail in a left-to-right paragraph that holds right-to-left text, and a run's
own direction (`a:rtl`) fails. East Asian text wraps by the Unicode line breaking rules (UAX #14):
a line may break between ideographs, kana and hangul syllables, a closing
mark, stop or comma may not begin a line and an opening mark may not end one.
PowerPoint's kinsoku lists are not documented, so this is the profile's
reading of them. A paragraph with East Asian text needs East Asian line
breaking (`eaLnBrk`, which absent is on, as PowerPoint writes it), whose
absence wraps by rules PowerPoint does not document: it fails, and best
effort wraps as above. With hanging punctuation (`hangingPunct`, absent on)
an ideographic or fullwidth stop or comma that would not fit hangs past the end
of its line, which is aligned and justified without it; off, it takes the
character before it to the next line, and a paragraph that runs right to left
or holds right-to-left text does not hang it. Hanging matches PowerPoint's own export of one
Japanese sentence at 14 pt in boxes 1.3 to 1.3875 in wide, 0.9 pt apart, on and
off (tested at the line-breaking level; a font with East Asian kerning, such as
Hiragino Sans, can fit one more character than PowerPoint does). A word too wide for its line breaks between characters, as a last
resort, as PowerPoint breaks it. A first-line indent needs a character bullet, which hangs in it: the
bullet is drawn at the margin plus the (negative) indent on the first baseline
and every line's text starts at the margin, so the indent must hold the
bullet; a bullet past it would push the text to a tab stop this profile does
not place. Bullet character, font (`buFont`, or the text's), size (`buSzPct`,
`buSzPts`, or the text's) and color (`buClr`, or the first run's) inherit
separately; a bullet taller than its line or wider than its indent fails, as
does a first-line indent without a bullet, and best effort draws them as
they would hang, and the text at the margin. Numbered bullets (`buAutoNum`)
show Arabic or Roman numerals or Latin letters with a period, parentheses or
nothing, counting from `startAt` over consecutive numbered paragraphs of a
level: a paragraph resets the counts of deeper levels, and an unnumbered one
its own; best effort draws other schemes as Arabic numerals and picture
bullets as "•". An empty paragraph shows no bullet. Runs may differ in font family, size (1–4000
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
touching spans of one color painted as one. Capitals (`cap="all"`) draw
the uppercase text. Character spacing (`spc`) follows each character, in
measuring and drawing. Underline, strike, small capitals, baseline shift,
outline, effects and hyperlinks fail. Best effort draws raised and lowered
text (`baseline`) shifted by its share of the run's size and drawn at 58% of
it, as LibreOffice's import does, and draws
underlines a tenth of an em below the baseline and strikes three tenths above
it, a twentieth of an em thick (doubled lines twice, styled ones solid),
small capitals as capitals, hyperlinks as plain text, and leaves out
characters the profile or the run's font cannot draw. Kerning applies from the resolved `kern` size,
and is off when it is absent or zero. An empty paragraph takes its line box from
its end-of-paragraph properties; a paragraph with runs takes it from them, as
LibreOffice's import does. A line break (`a:br`) starts a new line; an empty
line takes its box from the break's properties, or after a trailing break
from the end-of-paragraph properties. A field (`a:fld`) is drawn with the text
it was saved with, which a viewer may update, such as a date. A tab advances
to the next explicit stop (`a:tabLst`, inherited as a whole) past it, and
past those to the next default tab stop (`defTabSz`, inherited, 1" by
default), measured from the start of the line. Explicit stops are positioned
from the text box's inset edge, so stops at or before the paragraph's left
margin are passed over; the text after a tab, up to the next tab or the
line's end, starts, ends or centres at its stop, or puts its first full stop
there (decimal, which without one ends the text there). Text that would
start before its tab leaves the tab no advance. Lines break with every tab
measured to the default stops: a paragraph that wraps after explicit stops
moved a tab fails, and best effort keeps those breaks. Unresolved fonts fail. Text may use Latin, Greek and Cyrillic letters,
combining diacritics, Latin-1, general punctuation, currency and letterlike
symbols, arrows, mathematical operators and geometric shapes, which DrawingML
draws with the Latin font (`a:latin`), and East Asian text: ideographs, kana,
hangul and bopomofo, CJK symbols and punctuation, enclosed and compatibility
forms, and halfwidth and fullwidth forms, which it draws with the East Asian
font (`a:ea`). A run is cut into spans by the font class of each character,
which depends on the character alone; ASCII, including digits and spaces,
uses the Latin font even inside East Asian text, and each span is drawn with
its own face, so nothing shapes across a change of class. The East Asian font
may be a family or a theme reference (`+mn-ea`, `+mj-ea`); a theme font left
empty, as PowerPoint writes it, takes the theme font list's entry
(`a:font script`) for the run's language (`ja` Jpan, `ko` Hang, `zh` Hans, or
Hant for Taiwan, Hong Kong, Macau and the Hant script), as the alternate
language of a run with no language does. Text with no East Asian font fails,
and best effort draws it with the Latin font, which the resolver may leave
without the characters. A problem with the East Asian slot matters only to a
run with East Asian characters. PowerPoint may draw symbols of ambiguous East
Asian width with the East Asian font in Chinese, Japanese or Korean text, so
a run whose resolved language or alternate language is one of those may hold
only ASCII and East Asian characters among them: other symbols fail, and best
effort draws them with the Latin font. Right-to-left text: Hebrew, Arabic,
Syriac, Thaana and N'Ko with their presentation forms, the Arabic comma,
tatweel and digits, and the joiners and left-to-right, right-to-left and
Arabic letter marks set among them, is drawn with the complex-script font
(`a:cs`), chosen as the East Asian font is: a family, a `+mn-cs` or
`+mj-cs` theme reference, or a theme font left empty taking the theme font
list's entry for the run's language (`ar`, `fa`, `ur` and others Arab, `he`
and `yi` Hebr, `dv` Thaa, `syr` Syrc, `nqo` Nkoo) or, where the language names
none, for the script of the text. The joiners and marks take the font of the
text they are among. ASCII other than Latin letters (spaces, digits, punctuation
and symbols) uses the Latin font, except in a run whose language is written in a
complex script (the languages above): there, as PowerPoint's export showed for
`he-IL` and `ar-SA` with `a:latin` Courier New and `a:cs` Arial, it uses the
complex-script font, where with `en-US` it uses the Latin one. Other punctuation of
the European repertoire uses the Latin font even inside right-to-left text. Text with no
complex-script font fails, and best effort draws it with the Latin font. Other
scripts, such as Thai or Devanagari, soft hyphens, controls, the explicit
embedding, override and isolate controls, and other format characters, fail.

Tables (`a:tbl` in a graphic frame) render when they have no diagonal borders
or table-level fill, when their table style is absent or one of PowerPoint's
built-in styles (below), and when they are parsed
from the file without pending edits: the save path rewrites the domain model's
cells, which preparation must not do, so a new or edited table fails until it
is saved and reopened. Column widths come from the grid; a row is as tall as
its stored height or its tallest cell's text plus margins, whichever is
larger. Cell fills, then borders, then cell text are painted. A border is a
solid line centered on its grid line, drawn as one line along cells that give
it alike, and extended half the width of a border that ends at its end, so
corners close; where a border runs through, the line stops at its centre. Where adjacent cells set a shared edge
differently, or differing borders meet at a grid point, rendering fails: the
precedence is undocumented. Cell text uses the slide text profile, laid out
with the cell's margins and anchor; a cell `a:bodyPr` may only repeat them. A merged cell (`gridSpan`, `rowSpan`) draws its fill and text over the
grid cells it spans, which are marked `hMerge` or `vMerge` and draw nothing;
edges inside a merge have no border, and a cell spanning rows grows its last
row to hold its text.

Groups draw their shapes and pictures with geometry mapped from the group's
child space to its frame; text sizes and line widths do not scale, as
PowerPoint draws them. A rotated or flipped group turns and mirrors its
shapes about its centre, and turns their text with it. Pictures flip and
turn by quarters about their own centres, by their pixels. Best effort moves
pictures turned by other angles, and the text of a flipped group, with it,
upright and unturned. A group's fill paints nothing itself; shapes inside
whose fill is their group's (`a:grpFill`) take the nearest group fill. Best
effort leaves group effects out. Tables and charts inside groups are drawn
with their frames mapped; a chart scales with its group, and a table keeps its
own column widths and row heights, which best effort reports when the group
scales. A slide's placeholder inside a group draws with its layout's
inheritance at its own geometry, mapped into the group; one without its own
geometry fails. Grouped placeholders on layouts and masters are prompts and
are not drawn. Groups are drawn only
from their parsed form without pending edits.

Straight connectors (`straightConnector1` or `line`) draw their stored
geometry, flipped and rotated about their middle; bindings to other
shapes move a connector only when those shapes move. The line comes from the
theme line style the connector's `lnRef` selects, its `phClr` taking the
reference color, with any property the connector's own `a:ln` sets winning; an
effect reference must select an empty theme effect style. Lines take flat,
square or round caps and preset dashes, each dash taking the caps. Arrowheads fail: the
specification names their sizes but not their geometry. In best-effort mode
every head is drawn as a filled triangle whose tip is the line's end, sized
like LibreOffice's (small, medium and large are two, three and five line
widths, at least a pixel each), with the line stopping halfway into it. Bent and curved
connectors draw as their preset geometry, their several segments approximate, and like tables, a slide's connectors are drawn only from
their parsed form without pending edits. Connectors on layouts and masters
and in groups are drawn too.

Tables without a table style are drawn unstyled. This is provisional:
`tableStyles.xml` names a default style, and whether PowerPoint applies it to a
table without `a:tableStyleId` is undocumented. A style the renderer cannot
resolve is left out, and best effort draws the table unstyled and says so:
a style the deck defines in `ppt/tableStyles.xml` (those definitions are not
read yet), an inline `a:tableStyle`, and an id that no built-in style has.

PowerPoint's 74 built-in table styles (Themed Style 1 and 2, Light Style 1
to 3, Medium Style 1 to 4 and Dark Style 1 and 2, plain and with their
accents) are drawn when the file names one by its GUID and does not define it.
PowerPoint does not write their definitions into `tableStyles.xml`, so the
renderer carries them, each as the `a:tblStyle` it stands for: parts
(`wholeTbl`, `band1H`, `firstRow`, ...) with theme colors, tint, shade and alpha,
borders and bold. They go through the same resolution as any table style. A
cell takes its fill, text color and weight from the part that ranks highest
among those that apply to it, in the schema's order from `wholeTbl` up to the
corner cells, and its borders likewise, side by side: a part's `left`, `top`,
`right` and `bottom` apply at the edge of its region, and its `insideH` and
`insideV` between cells of it. Where two cells give a shared edge different
borders, the one from the higher part wins, and where borders cross the higher
is painted over the lower. A cell's own fill, text properties and borders
outrank the style's. The header and total rows are not counted in banding. A
compound line (`dbl`) is two strokes a third of its width each, a third apart.
A style's background takes its fill from the theme's fill styles (Themed Styles
paint the theme's gradients behind the table, over its whole height) and its
shadow from the theme's effect styles.

The definitions were measured against PowerPoint's own export of a table in each
style, 3 columns by 4 rows with header, total, first and last column and banded
rows on, drawn with the default theme: of the 74 styles, 68 match it in the fill
of every cell, the color and width of every border (within a pixel) and the
color and weight of every cell's text, with colors within 8 of 255 per channel
and no more than a few hundred of some 230,000 pixels outside the text off, at
the antialiased corners. The other six, Themed Style 2 with an accent, differ
in the soft shadow around the table, which is drawn as the approximation any
blurred shadow is. A strict render fails for these six, as it does for any
blurred shadow, and for banded columns, which were not compared; best effort
draws them and reports the approximation. Where a cell's own border meets a
style's at a corner, which of the two shows is undocumented: strict rendering
fails and best effort draws it.

Forme measures wrapping and shapes final lines under cumulative budgets. Native
line metrics use the supplied font's hhea ascent, descent and line gap; baseline
placement and percentage spacing use those metrics. This is a defined native
layout profile, not a claim of identical PowerPoint line placement. DrawingML
[percentage line spacing](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.drawing.linespacing)
scales with text size; fixed-point line spacing and percentage before/after
paragraph spacing are not supported by this adapter.

### Charts

A chart frame (`c:chart` in a graphic frame) is drawn when
`render.Options.Charts` supplies a chart renderer, and otherwise fails, or is
left out with a warning. The chart part is read from its cached values and
written as a [Vega](https://vega.github.io/vega/) specification the size of
the frame in CSS pixels; the renderer returns an image of it at two pixels
per CSS pixel, which is stretched over the frame under the slide's image
budget. The specification carries the chart's values, labels and colors as
data, and no expressions from the document.

Column and bar charts (clustered, stacked and percent stacked, with their gap
width and overlap), line charts (with or without markers), area charts
(standard and stacked), pie and doughnut charts (first slice angle, hole size,
varied colors) and scatter charts are drawn. Series and point colors resolve
through the slide's theme like shapes': a series' own fill or line, or else
the theme's accents in turn, then darker and lighter rounds of them. Axes
follow `delete`, `tickLblPos`, `majorGridlines`, `scaling` minimum, maximum and
orientation, `crossBetween` and common number formats (`General`, `0`,
`0.0`, `0.00`, `#,##0` and their decimals, and percentages); titles,
automatic titles, legends and their positions, and shown values are drawn,
with the chart space's text size and color. Best effort draws 3-D charts flat, a combination chart as one of its
types, a secondary or date axis as the primary one, other number formats as
General, smoothed lines straight and dashed lines solid, and leaves out
trendlines, error bars and legend entry formatting. Turned chart frames fail.

Formatting a chart leaves out is Office's automatic look, which differs from
the modern look of charts PowerPoint writes with explicit formatting on every
element: text, gridlines and axis lines are black, the axes' tick marks point
out (those of a category axis fall between categories), the legend sits at the
right, centred along its side, and a pie's slices have no outline. Text is
10pt and a title 18pt, or 1.2 times the chart space's text size when it sets
one. A line series is the theme's first line style (0.75pt without one) times
3, or 5 or 7 for the heavier `c:style` values (9 to 24 and 33 to 48, or 25
to 32). Explicit
`spPr`, `txPr`, `majorTickMark` and `legendPos` win, including their absence of
a line (`a:ln` with `a:noFill`) or of tick marks (`none`); an axis' own text
formatting styles only that axis, the legend's its entries and a data label's
its values. A tick mark that crosses the axis is drawn pointing out, with a
warning in best effort. A legend beside a horizontal bar chart lists its
series last first, as the bars stack.

A value axis follows Office's automatic scale unless `c:min`, `c:max` or
`c:majorUnit` set it. It starts at zero when the data's range exceeds a sixth
of its largest magnitude, and else half a range below the data; it ends above
the data by a twentieth of its range; a percent stacked axis is 0 to 100%.
The major unit is the least 1, 2 or 5 times a power of ten that keeps the steps
apart by at least 1.2 times the axis' text size on a vertical axis, and 6
times on a horizontal one, estimating the plot's length from the frame, and
divides the axis into at most ten steps; both ends round out to a multiple of
it. Data up to 5 runs 0 to 6 in steps of 1 however tall the plot. Data up to 4.5 over a plot 3 inches tall
runs 0 to 5 in steps of 0.5, and over one 6 inches wide, in steps of 1, as
PowerPoint draws them. Radar and surface charts step by their own nice
rule.

Bubble charts draw each point as a circle on value axes, its area (or with
`sizeRepresents="w"` its width) its size's share of the largest, which spans
a quarter of the plot's smaller side times `bubbleScale`; bubbles of no or
negative size are left out, and best effort draws 3-D bubbles flat. Radar
charts draw a spoke per category clockwise from 12 o'clock, rings at the
value axis' nice steps and each series a closed line, with markers for the
`marker` style and filled for `filled`. Stock charts draw their prices as a
line chart, with high-low lines from each category's lowest to highest value
and up-down bars from the first series' value to the last's, in the up or
down bar's fill. Pie-of-pie and bar-of-pie charts move the points their split
selects (`pos`, `val`, `percent` or `cust`) to a second pie or stacked bar
`secondPieSize` of the first, joined by series lines to a grey slice facing
it; best effort moves the last third of the points for the `auto` split,
which Office does not document. Surface charts fail, and best effort draws
them from above as a banded contour: values interpolated across each cell,
in bands of the value axis' steps colored by the theme's accents, with a
legend of the bands. Charts allow at
most 256 series of 4096 points.

## Sheet range profile

`xlsxrender.PrepareRange(ctx, sheet, "A1:D10", opts)` snapshots that logical range at 96 CSS
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

`docxrender.Prepare(ctx, document, opts)` lays out and paginates the complete
document once and returns `*Pages`: `Count()` is the number of **physical
pages** and `Page(ctx, n)` prepares a **1-based** page (a page beyond `Count`
returns `render.ErrInvalid` wrapping `docxrender.ErrPageOutOfRange`). `Pages`
is immutable and safe for concurrent use. `docxrender.PreparePage(ctx, document, n,
opts)` is `Prepare` followed by `Page` and repeats the layout on every call;
`spine-render` uses `Prepare`. The document is never saved or changed, and
unsaved edits are drawn. `Count` and `Page` are physical pages; the number a page
shows (`PAGE`) follows the sections' `w:pgNumType`.

### How a document is drawn

The renderer reads the parts as the document would save them, so nothing the
model preserved is invisible to the checks. Each section's body is translated
into generated HTML and CSS for [forme](https://github.com/mgilbir/forme), the
layout engine under the other renderers, and laid out once at the section's text
width with unbounded height. forme sets a document on one sheet and does not
break flow across pages, so spine paginates: it cuts that layout at block and
line-box boundaries, applying Word's rules, and gives each page the display-list
operations of the lines it holds (text by its baseline; fills and rules by the
line they decorate, with block-level fills clipped at the cut). Everything is
validated or escaped: the only strings from the document that reach forme are
text nodes, escaped as text. Font families are replaced by synthetic names in the
generated CSS and asked of `render.Options.Fonts` as `render.FontRequest`s
(bold and italic included, one request per distinct family, weight and slope
and per document, bounded by `Limits.MaxFonts`); colours and lengths are
formatted from numbers; nothing is loaded from the document.

With `render.Options.Warn` unset (strict mode) anything that is not drawn
exactly fails the call with `render.ErrUnsupported`, wherever in the document
it is. With `Warn` set (best effort) content that is not supported is left out
and reported once per kind, as an error wrapping `render.ErrUnsupported`, and
content that is drawn approximately is reported once per kind as an error
wrapping `render.ErrApproximated`; the rest of the page still draws.

### Drawn exactly

- **Sections.** Page size and margins (and a gutter, added to the left margin)
  per section; section breaks `nextPage`, `continuous` (when page size and
  top/bottom margins match; a different geometry starts a page and is reported as
  approximated), `evenPage` and `oddPage` (by the page number the next page
  shows, with a blank page where needed; a blank page is numbered and has a
  header and footer). A section without an explicit page size or margins fails;
  best effort draws it on Letter with one-inch margins, the page Word's US Normal
  template sets, and reports it.
- **Styles.** Document defaults; paragraph and character styles with `basedOn`
  chains (a cycle in a style the document uses is invalid) and the default
  paragraph style; toggle properties combine across style levels by exclusive or
  (a bold character style on bold text is not bold) and direct formatting
  overrides; theme fonts (`asciiTheme` and the other slots), theme colours with
  `themeTint` and `themeShade` (scaling HSL luminance) through the settings
  colour mapping. Unreferenced styles are never read.
- **Paragraphs.** Alignment (`left`, `start`, `center`, `right`, `end`, `both`);
  left, right, first-line and hanging indents (`start` and `end` as left and
  right); space before and after (kept on the first page, dropped at the top
  of a page that a soft break, a manual page break or `pageBreakBefore`
  started, and kept after a section break, as Word's PDF output shows), the
  larger of one paragraph's space after and the next one's space before
  separating them, also across a section break; a page break that ends its
  paragraph keeps the paragraph mark with it, so no empty line opens the next
  page; with `contextualSpacing` and HTML automatic spacing
  (14 pt); line spacing `auto` (a multiple of the font's line height, 240ths of a
  line), `exact` and `atLeast`. A font's line height is its Windows ascent and
  descent (`usWinAscent` + `usWinDescent`), as Word sets lines, or its ascent,
  descent and line gap when it states none. Text is placed in its line as Word
  places it, line by line by the line's tallest text (measured against Word's
  PDF output): a multiple's extra space below the text (the baseline at the
  Windows ascent), an at-least height's above it, an exact height's baseline
  at four fifths of the line. Approximated: fonts of different proportions on
  one line, and a font whose own ascent or descent is not its Windows one
  beside an inline picture or at an at-least height; `pageBreakBefore`, `keepNext` (a chain taller
  than a page is ignored), `keepLines`, `widowControl` (on by default; the first
  two lines stay together and two lines move to the next page); manual page and
  column breaks (the text after a break is the paragraph's continuation on the
  next page and takes no first-line indent); line breaks; tab stops: default stops
  (`w:defaultTabStop`, from the page margin, with the left indent as the stop of
  a hanging indent's first line), and custom left, center and right stops with
  `dot`, `hyphen` and `underscore` leaders, cleared and inherited through
  styles, which are set by measuring the text between tabs, so they are exact
  while each line between manual breaks is one line of text. The paragraph mark's formatting sizes the last line
  only when it has no text (an empty paragraph, the line after a line break
  that ends the paragraph, or a line of pictures); a line with text is sized by
  its runs, whatever the mark's size, as Word's PDF output shows.
- **Numbering and lists.** `w:numPr` set on a paragraph or through its
  paragraph style (and the styles it is based on) names a `w:num` of
  `numbering.xml`; the level is `w:ilvl`, or the level whose `w:pStyle` names the
  paragraph's style. Counting follows Word: counters belong to the abstract
  numbering (several `w:num` of one `w:abstractNum` continue each other's count),
  a `w:startOverride` restarts its level at the first paragraph of that `w:num`
  there, a `w:lvlOverride` with a `w:lvl` replaces the level, a level restarts
  after a paragraph of a higher level (or after the level `w:lvlRestart` names,
  or never when it is 0), a paragraph that skips levels counts the skipped ones
  as used at their start value, and paragraphs without numbering, other content
  and section breaks do not affect the count. Levels: `w:start`, `w:numFmt`
  (decimal, decimalZero, upper and lower roman and letter, chicago, the enclosed
  and full-width decimals, bullet, none), `w:lvlText` with `%1` to `%9` (each in
  its own level's format, in decimal under `w:isLgl`), `w:suff` (tab, space,
  nothing), `w:lvlJc` (the marker begins at, is centred on, or ends at the
  position where the first line starts), the level's indents and tab stops, and
  its run formatting over the paragraph mark's (font, size, colour, bold...). The
  tab after the marker reaches the hanging indent, a custom stop or the next
  default stop through the tab machinery above, so a paragraph that wraps is
  still exact. With the numbering set directly on the paragraph the level's
  indents replace its style's; with it set by the style the style's indents
  win; direct indents always win. `w:numStyleLink` and `w:styleLink` resolve
  through the numbering style (at most 64 links deep). A paragraph whose list
  is not defined (no numbering part, `w:num`, abstract numbering or level)
  draws without a marker, as Word draws it; `w:numId` 0 removes inherited
  numbering. Bullets in the symbol fonts (Symbol, Wingdings, Wingdings 2 and 3,
  Webdings) are drawn from the private-use code point in that font when the
  host's font resolver provides a face that has it.
- **Runs.** Multiple runs per paragraph; font family per script slot (ASCII,
  high ANSI, East Asian and complex script characters pick the matching
  `rFonts` slot), size, bold, italic (complex-script text uses `bCs`, `iCs` and
  `szCs`); underline `single`, `double` and `thick` (in the underline's colour),
  strikethrough; caps; text colour and `auto`; highlight; solid or clear
  shading; `vertAlign` superscript and subscript (65 % size, raised by a third
  and lowered by an eighth of the run's size) and `position`; hidden text
  (`vanish`, not drawn); soft and non-breaking hyphens; kerning from the `w:kern`
  size (off otherwise); text beyond the margin that has no break opportunity
  wraps at the margin. Optional OpenType ligatures, contextual alternates and
  kerning are disabled unless asked for, matching Word's default.
  Text is drawn in the scripts forme shapes; characters the supplied fonts lack
  fail (`render.ErrUnsupported`).
- **Headers and footers.** `w:headerReference` and `w:footerReference` of the
  types `default`, `first` (with `w:titlePg`, on the first page a section opens)
  and `even` (with `w:evenAndOddHeaders`, by the displayed page number); a type
  a section does not reference is inherited from the previous section, and one
  nobody defines is blank. The content is translated by the body's code
  (paragraphs and every other block kind the profile draws), laid out at the
  section's text width, and drawn from the `w:pgMar` header and footer
  distances. A header that reaches past the top margin moves the text area down,
  and a footer past the bottom margin moves it up, and pagination uses the
  reduced height. Blank pages inserted for even and odd section breaks have
  them too.
- **Page numbers.** `w:pgNumType` `start` (restart at the section's first page)
  and `fmt` (`decimal`, `decimalZero`, `upperRoman`, `lowerRoman`,
  `upperLetter`, `lowerLetter`, `chicago`). In headers and footers `PAGE`,
  `NUMPAGES`, `SECTIONPAGES` and `SECTION` fields and the `w:pgNum` placeholder
  show the real values, in the section's format for `PAGE` and with the
  `\* Arabic`, `roman`, `ROMAN`, `alphabetic` and `ALPHABETIC` switches
  (`MERGEFORMAT` is accepted). `NUMPAGES` and `SECTIONPAGES` are only known
  after pagination, and the page count can change a header's height (a wider
  number wraps): pagination is repeated, at most four times, while the heights
  it assumed differ from the result; where it does not settle the last
  pagination is kept and reported as approximated. Other fields in headers
  and footers show their cached result.
- **Fields and links.** The cached result of simple and complex fields, across
  paragraphs, is drawn and the instruction is not; hyperlink text is drawn with
  its run formatting. In the body the cached result of `PAGE` and the other
  page-number fields is drawn, as Word saved it, and checked against the
  value the field has on the page it lands on: a result that differs is
  reported as approximated.
- **Footnotes.** `w:footnoteReference` draws the note's mark (its number in the
  `w:footnotePr` `numFmt`, from `numStart`, restarting by `numRestart`:
  `continuous`, `eachSect` or `eachPage`; properties from the settings part and
  the section) in the run's own formatting, and the note's text (translated by
  the body's code, with its `w:footnoteRef` mark) goes to the bottom of the text
  area of the page that holds the reference's line, under the separator
  (144 pt from the start of the line, from the separator note) in the order of the
  references. The area reduces the text area for pagination: a line is placed
  only if it and its notes fit. A note that does not fit whole is cut at a line
  and continues on the next page under the continuation separator (also 144 pt
  long, as Word draws it), before that page's own notes. Footnote text that restarts numbering on
  every page is translated again with the numbers pagination gives (at most
  three times). `w:customMarkFollows` references use the text that follows as the
  mark.
- **Endnotes.** `w:endnoteReference` marks (lower-case Roman by default) and the
  notes, which follow the body text of the document, or of each section for
  `w:pos` `sectEnd`, after the endnote separator, as ordinary flow.
- **Pictures.** A `w:drawing` that holds a picture (`pic:pic`) is drawn from
  the image its `r:embed` relationship names in the part being translated: PNG,
  JPEG and GIF (the first frame), and EMF and WMF, which are played onto a
  raster of about twice the frame's pixels (never below half, within the image
  budget) with the metafile playback `internal/metafile` shares with PowerPoint,
  described under PowerPoint pictures above: gowemf plays the records, the page
  painter's scan conversion draws them, and strict mode fails on anything not
  drawn exactly while best effort reports what it approximates or leaves out
  and draws the rest. An EMF+ file is drawn from its EMF+ records when they can
  all be drawn, else from its GDI records. Playback is charged to the render
  limits and checks cancellation in every drawing call. A metafile is drawn once, at the
  size of the first picture to use it. The image is read from the package, never from
  a path or URL the document writes; linked pictures (`r:link`, external
  relationships) are not loaded. Each image part is decoded once, under
  `Limits.MaxImageBytes` and `MaxImagePixels`, and a picture is cut, filled,
  flipped, rotated and given its colour effects into one raster, sized to what is
  drawn (at most four image pixels per drawn pixel) and shared by equal pictures.
  Supported: the frame (`wp:extent`), `a:srcRect` crops (negative values extend
  the picture with transparent pixels), `a:fillRect` insets, `flipH`, `flipV` and
  any rotation (resampled, with antialiased edges), `wp:effectExtent` (the room
  around the frame, so rotated pictures take their bounding box), picture colour
  effects (grayscale, bi-level, duotone, colour change and replacement, HSL
  shifts, alpha effects and fill overlays, the pixel code `pptxrender` uses) with
  sRGB, theme and system colours, a solid picture outline (`a:ln`, drawn centred
  on the frame's edge for unrotated pictures), SVG pictures through their PNG
  fallback, hidden drawings (`wp:docPr hidden`, not drawn).
  - **Inline** pictures (`wp:inline`) sit on the text line like a large glyph,
    their bottom on the baseline: the line grows to hold them, they move with
    their line across pages, and a picture taller than the page is clipped by it.
  - **Anchored** pictures (`wp:anchor`) belong to the page of their anchor line.
    Without wrapping (`wrapNone`, in front of or `behindDoc` behind the text) the
    picture is drawn at its position: `relativeFrom` page, margin, column, left,
    right, inside and outside margins, paragraph, line, character and
    top and bottom margins, with `posOffset` or `align`, or `simplePos`; text is
    unaffected, as in Word, and `relativeHeight` orders overlapping pictures.
    Square, tight, through and top-and-bottom wrapping around a picture positioned
    from its paragraph or line (offset zero or aligned to the top) is a float in
    the layout: text wraps around it on the side `wrapText` names (`left`,
    `right`, `largest`, and `bothSides` where the picture touches an edge of the
    text) at the wrap distances, and it moves with its paragraph. The wrapped
    text around a float is laid out where the float is, so a float and the
    paragraph's lines stay together on one page. In a table cell, where Word
    places the picture relative to the cell (`layoutInCell`), which is not
    implemented, an anchored picture is drawn on its line, where the same
    picture inline would be: strict mode fails, best effort reports it.
- Content controls, smart tags and custom XML wrappers draw their content.
- **Tables.** The table grid and the columns it gives (fixed and autofit
  layouts: an autofit table is drawn by the grid Word stored, which is what Word
  computed), `w:tblW` as points or a percentage of the text width (of the cell,
  in a nested table), alignment and indent (the table's edge sits at the indent
  in compatibility mode 15, and the first cell's text at it in older modes, as in
  Word), cell margins from the cell, the row exception (`w:tblPrEx`), the table
  and its style, measured from the grid line, and vertical alignment. Borders:
  the table's outer and inside borders, table style layers and cell borders, as
  single, double, dotted and dashed lines with their width and colour, where
  neighbouring cells of the same size share an edge by Word's rule (the heavier
  line, then the darker colour, then the first cell). Shading from the cell,
  the table and the style (clear and solid fills, theme colours). `gridSpan`,
  `vMerge`, the legacy `hMerge`, `gridBefore` and `gridAfter`, row heights
  (`atLeast`, and `exact` while the text fits), hidden rows, and nested tables.
  Table styles: `basedOn` chains, the default table style, the whole-table layer
  and every conditional layer (first and last row and column, banded rows and
  columns with their band sizes, corner cells) selected by `w:tblLook`, with the
  style's paragraph and run formatting beneath paragraph styles and direct
  formatting. Cells hold paragraphs (with tab stops) and nested tables.
  Pagination cuts a table between rows; a row without `w:cantSplit` is also cut
  between the lines of its cells, where no line is crossed; a `cantSplit` row
  moves to the next page whole unless it is taller than a page; header rows
  (`w:tblHeader`, from the first row) are drawn again on each page the table
  continues on and are not left alone at the foot of a page; rows whose
  paragraphs all keep with next stay with the next row, and a keep-with-next
  paragraph before a table stays with its first row; the borders at a cut are
  drawn whole on both pages. Rows a vertical merge joins are cut only between
  the lines of the merged cell.

### Drawn approximately

Reported with `render.ErrApproximated` in best effort and refused in strict
mode: tab stops in a paragraph whose lines wrap (the tab widths assume the text
between manual breaks fits on one line), decimal stops (set as right stops), bar
stops (not drawn), the heavy and middle-dot leaders (drawn with an underscore and
a dot); justified
`distribute` and kashida alignment (justified); a justified line before a manual
line break (Word stretches it, this does not); character spacing (`w:spacing`, not
applied: the shared rasterizer draws no letter spacing); small caps (synthesized
by forme); underline styles other than single, double and thick (drawn as a
single line, as forme draws only solid lines) and double strikethrough
(single); shading patterns (not drawn); right-to-left paragraphs and runs (laid
out with the bidirectional algorithm, but alignment of `left` and `right` is not
mirrored as Word does); character-unit and line-unit indents and spacing (not
applied); tracked changes (insertions shown, deletions dropped, as the final
text); settings that change layout and are not modelled (mirrored margins,
automatic hyphenation, book fold, ...); multiple text columns and the document
grid (laid out as one column); a continuous section break across different page
geometries; footnote text that continues for more than half a page (a page
takes at most half its text area of continued note text and the rest goes to the
next page); a footnote continuation notice that has text (not drawn), footnotes
beneath the text or at the end of a section or document (drawn at the page
bottom), and endnotes that continue on another page (the continuation separator
is not drawn); footnote numbers that restart on every page and do not settle; a
body page number field whose saved result differs from the page it is on (or
has none to compare); a
page number format other than those listed (decimal is drawn),
chapter numbers in page numbers, other field switches on page number fields, a
page break inside a header or footer, and a header and footer that leave no room
for text on the page (the text area keeps the margins); list markers in a symbol font the host does not provide (a
Unicode bullet of similar look is drawn in the paragraph's font), picture
bullets (a bullet character), number formats other than those listed above
(Hebrew, Arabic, Thai, Japanese and Chinese counting, ...) and values a format
cannot express (roman numerals above 3999 or below 1, letters below 1,
enclosed circles above 20), which draw in decimal, the English text formats
(`ordinal`, `cardinalText`, `ordinalText`), which Word writes in the document's
language, and marker text that refers to a deeper level; floating tables
(`w:tblpPr`, drawn in the text flow), cell spacing
(drawn without), right-to-left tables, a table width or cell widths (`w:tcW`)
that differ from the grid (drawn by the grid, or, for an autofit table, scaled
to the table width), a grid without column widths, an autofit column too narrow
for an unbreakable word (the word overflows; Word widens the column), `noWrap`
cells in autofit tables, text taller than an `exact` row height (drawn, not
clipped), page breaks in a cell (`pageBreakBefore` and manual breaks are not
applied), borders in other styles than the four above (drawn solid), border
shadows, shading patterns, row alignment, vertical text in cells, `tcFitText`,
justified vertical alignment, tracked changes to rows and cells, and borders
shared by cells of different sizes whose lines differ (forme's rule decides); pictures: tight and through wrapping (drawn around the picture's
box, not its outline), `bothSides` wrapping of a picture with text room on both
sides (the larger side), a wrapped picture offset below the top of its paragraph
(text beside the space above it is displaced), a wrapped picture positioned from
the page or margin (drawn there, without text wrapping around it), wrapping that
crosses a page boundary, inside and outside positions (drawn as left and right),
a picture placed from its character with wrapping (from the column), a picture
in a line of exact height (Word clips it; the line grows), tab stops beside a
floating picture, picture shapes other than a rectangle, a tiled picture fill,
shadow, glow and 3-D picture effects (not drawn), picture artistic effects, a
dashed picture outline (solid) and a rotated picture's outline (not drawn),
brightness and contrast (drawn as LibreOffice does), tint, blur and gradient
overlays, picture colour effects with colour transforms (the effects are skipped)
and effects colours other than sRGB, theme and system colours.

### Left out

Reported as unsupported in best effort and refused in strict mode: diagonal cell
borders, table, row and cell properties and elements this profile does not know,
content of merged cells that a merge hides, pictures that cannot be drawn
(linked, missing, corrupt, in another format such as EMF and WMF, or too large
to decode: an inline one keeps its space), legacy VML pictures (`w:pict`),
charts, SmartArt, shapes, text boxes, groups and other non-picture drawings (an
inline one keeps its space), alternate content, notes the document does not
have, notes in headers, footers and notes, comments, equations,
embedded objects, symbols, form fields, paragraph and run borders, paragraph
shading, frames, page borders, line numbering, vertical page alignment, text
direction other than left to right, page background, text effects, and every
element or property this profile does not know.

### Defined by the profile

Native line metrics (the font's own ascent, descent and line gap, with leading
split equally above and below a line; Word distributes extra leading
differently), the superscript and subscript scale and offset, the double
underline's spacing, automatic hyphenation (none), and the shaping engine decide
the exact placement; identical Word pagination is not promised. Pagination
follows the rules above on those metrics.

Tables are laid out by forme as CSS tables with collapsing borders and fixed
columns; the renderer computes what Word decides (columns, margins as padding,
every cell's borders, shading, heights) and hands them over as numbers. Where
that differs from Word the profile decides: the text inset of a cell is its
margin from the grid line (the half of a border wider than the margin is the
inset instead), a row that only holds the continuation of a vertical merge takes
its height from the other rows, a row aligned other than to the top is kept whole
rather than re-aligned in each part when it is cut, and only table placement
differs between compatibility modes.

### Budgets

The main, styles, numbering, settings, theme, header, footer, footnote and
endnote parts are bounded by `MaxSourceBytes` and
`MaxLayoutNodes` (elements, attributes and every generated block, span, row and
cell; a table is bounded to 256 columns, 16 nested levels and a million grid
slots);
list counters by Word's own limit of 32767 (`render.ErrLimit`) and level text by
256 characters; emitted text by eight times `Limits.MaxTextBytes`; pages by
`Limits.MaxOperations`; the whole document's display list by sixty-four times
`Limits.MaxOperations`; fonts by `Limits.MaxFonts`; page dimensions and pixels by
`MaxDimension` and `MaxPixels`. Pictures: at most 16384 drawings per
document; images decoded once each, within four times `Limits.MaxImageBytes` and
`MaxImagePixels` in all (a single image over the limits is refused in strict mode
and left out in best effort), and the pixels made from them within four times
`MaxImagePixels`; extents and offsets beyond a million pixels are limits or are
clamped. A page's own operations, glyphs and shaping work are checked when
`Page` prepares it, under the shared `render.Limits`, which include the page's
pictures: `MaxImages` distinct rasters (32 by default) and `MaxImagePixels` on
a page. The layout
engine is not interruptible inside one call; cancellation is checked between
parts, sections, blocks and pages.

## Runnable previews

Run `go run ./examples/render_previews -out render-previews` to write PNG and SVG
previews of a slide, a sheet range and two physical document pages at 144 DPI.
The example uses Forme's embedded Noto Sans and explicitly substitutes it for
the sheet's Calibri request. It needs no host fonts or external processes.

`MaxShapeWork` measures Forme's shaping work: each subtable, ligature and rule
tried and each glyph a match walks over (about 50 units a byte of Latin text,
300 of Devanagari, 350 of Nastaliq); it is not a duration. The default, 64 Mi
units a slide, page or sheet, covers ordinary documents many times over while a
font's runaway rules are still charged in full.

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
are searched.

A TrueType or OpenType collection (`.ttc`, `.otc`, or one wrapped as WOFF 2)
holds several fonts in one file, and a mapping picks one of its faces by its
index from zero after a `#`: `-font 'Cambria=fonts/cambria.ttc#0'`,
`-font 'Cambria Math=fonts/cambria.ttc#1'`. The index is the digits after the
last `#` of the file name; a file whose own name ends in `#` and digits takes
an index of its own as well (`a#12` is `a#12#0`), and other `#`s are part of
the name. A mapping to a collection of several faces without an index fails,
naming the first eight faces with their indices, family and style, and
PostScript names, so that the face is never a guess; a single font, or a
collection of one face, loads without one, and an index past its faces fails.
Several faces mapped from one file read and count it once toward the 32 MiB
(it is found by its cleaned path; two paths to one file count twice). Faces
load at their default instance, and share the file's bytes. Charts take one
font per file, so for a chart each mapped face of a collection is copied out
into a font of its own, which together may hold up to a further 32 MiB (the
tables the faces share are copied for each); past that, and only when charts
are drawn, the command fails and `-charts=false` leaves them out. `-fallback-noto` explicitly substitutes embedded Noto Sans for
unresolved regular faces. Strict, bold and italic faces require mappings;
otherwise a missing bold or italic face is drawn with the family's regular
face, or the regular fallback, with a warning. Substitution can
change wrapping and overflow.

Text is shaped at its size: a font with an AAT tracking table (`trak`, with a
`STAT` table, as HarfBuzz requires), such as Apple's newer system fonts, is
tracked at each run's size in CSS pixels, the size a browser hands HarfBuzz,
rather than at CoreText's default of 12 points. Office's own fonts have no
tracking table.

PPTX charts are drawn with [aster](https://github.com/mgilbir/aster), an
embedded Vega renderer: the library writes each chart as a Vega
specification and aster draws it, loading nothing outside the specification,
with the `-font` mappings and otherwise its embedded Liberation Sans. Each
chart is bounded to 256 MiB of renderer memory and to `-timeout`, which cannot
interrupt a chart once started. `-charts=false` leaves charts out, with a
warning.

`-image-pixels` bounds decoded image pixels per slide, page or sheet; it
defaults to 64 Mi, where the library's default of 4 Mi is less than one phone
photo.

`-shape-work` bounds shaping per slide, page or sheet in the work units
described above and defaults to the library's 64 Mi, which covers more than a
million bytes of Latin text; `-timeout` bounds the whole command. Likewise
`-edge-checks` bounds path painting per output (scanline edge tests and
coverage samples) and defaults to 4 Gi instead of 64 Mi: a slide of text,
circles and outlined boxes needed up to 256 Mi at 144 DPI and painted in about
a quarter of a second, and the work grows with the square of the DPI.

`-dpi` defaults to 288, sharp on a display of twice the standard density,
where a widescreen slide is 3840 by 2160 pixels; `-dpi 144` or `-dpi 96`
writes smaller files faster, and SVG output stays sharp at any zoom. For it
the command allows 64 Mi pixels and 16384 pixels a side per page, 1 Gi pixel
visits and 256 MiB of output, where the library defaults suit about 144 DPI.
`-max-pages` defaults to 100 (maximum 10000), and
`-timeout` defaults to one minute. Interrupt cancels rendering. Library package,
source, shaping, pixel and output limits still apply. DOCX is laid out once, and
its pages are prepared from that layout; the layout cannot be interrupted, so
`-timeout` takes effect between sections and pages. The command raises the path
segment budget to 8 Mi per page, since a page of body text in a complete font
exceeds the library's 100,000, and the document node budget to 8 Mi, since a
document of about fifteen pages exceeds the library's 100,000. The timeout and
page cap bound this command's processing.

The CLI draws slides and documents best effort by default: each piece of content
it cannot draw prints a `warning:` line naming the slide (or the document), and
the rest is drawn. A
summary counts content left out and content drawn approximately, which
warnings wrap with `render.ErrApproximated`. Use
`-strict` to fail instead. It does not expand the supported formatting. Errors identify the failing page, slide, sheet or document. Existing outputs
are never overwritten. A failed output file is removed; completed files from
earlier pages or the other format remain available after a later error.

`-keep-going` reports each slide or sheet that cannot be prepared on standard
error, skips it, renders the rest, and then exits with an error naming the
skipped pages. A cancelled or timed-out run still stops, and write failures are
never skipped. DOCX lays out one document for every page, so its failures
repeat across pages and stop the run as before.

## Measuring fidelity

`tools/fidelity` compares spine's slide renders with a reference render of the
same deck, such as PowerPoint's PNG export (File > Export, as PNG, every
slide) or LibreOffice's. Render at the reference's size, or let the tool
resample, then compare:

```sh
./spine-render -input deck.pptx -out ours -font 'Calibri=fonts/Calibri.ttf'
go run ./tools/fidelity -ours ours -ref reference -out report
```

Files pair by the last number in their names (`slide-0001.png` with
`Slide1.png`). Each slide gets its structural similarity (SSIM of luma over
8 by 8 windows; 1 is identical), mean absolute channel difference, the share
of pixels differing by more than `-threshold` (32 by default), and a diff
image of the faded reference with those pixels red. `report.md` and
`report.json` collect them. Fonts matter most: map the deck's fonts to the
files the reference used, or differences in text dominate.
