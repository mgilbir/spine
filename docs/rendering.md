# Native rendering

Implementation is in progress in stacked draft PRs. The `render` package can
prepare a caller-supplied Forme display list and write PNG or SVG. `pptxrender.PrepareSlide` (package `pptx/pptxrender`) supports a first static slide profile. `xlsxrender.PrepareRange` (package `xlsx/xlsxrender`) supports bounded range previews. `docxrender.PreparePage` (package `docx/docxrender`) prepares a selected physical page from a bounded
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
  for a font with no outlines).

A glyph is drawn whole in one of these ways, never part by part. Text turned
or warped in a slide is drawn as outlines, so its color glyphs are outlines in
the text color, and reported as such; a slide's shadows and other effects take
a color glyph's coverage. DOCX and XLSX previews have no best effort and
refuse what cannot be drawn exactly. Bitmap fonts (`EBDT`, `EBLC`, and Apple's
`bdat`, `bloc`) are refused unless they also have outlines (`glyf`, `CFF `, `CFF2`), as system fonts
such as Courier New that carry strikes for small screen sizes do; those are
drawn from their outlines.

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
Pattern fills (`a:pattFill`) on shapes and backgrounds fail, since the
standard pictures its preset patterns without giving their pixels. Best
effort draws each from its description, 8 by 8 CSS pixels tiled from the
box's corner in its foreground over its background (black over white when
absent): a percentage as an ordered dither of that density, and lines,
grids, checks and figures as their names say. Best effort reports problems
with a background as the background's, and draws a background it cannot
draw white.
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

Tables (`a:tbl` in a graphic frame) render when they have no table style,
diagonal borders or table-level fill, and when they are parsed
from the file without pending edits: the save path rewrites the domain model's
cells, which preparation must not do, so a new or edited table fails until it
is saved and reopened. Column widths come from the grid; a row is as tall as
its stored height or its tallest cell's text plus margins, whichever is
larger. Cell fills, then borders, then cell text are painted. A border is a
solid line centered on its grid line, extended half its width where another
border meets it so corners close. Where adjacent cells set a shared edge
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
`tableStyles.xml` names a default style, often a built-in Office style the file
does not define, and whether PowerPoint applies it to a table without
`a:tableStyleId` is undocumented. Explore that, and built-in style definitions,
before supporting styled tables. Best effort draws styled tables unstyled.

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
with the chart space's text size and color. Value axes step about every 60
pixels. Best effort draws 3-D charts flat, a combination chart as one of its
types, a secondary or date axis as the primary one, other number formats as
General, smoothed lines straight and dashed lines solid, and leaves out
trendlines, error bars and legend entry formatting. Turned chart frames fail.

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

`docxrender.PreparePage(ctx, document, 1, opts)` selects a **1-based physical page** after
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

`-shape-work` bounds shaping per slide, page or sheet in the conservative lookup
units described above. The command defaults to 16 Gi units instead of the
library's 64 Mi, which suits a single paragraph: complete fonts charge about a
million units per byte of text (embedded Noto Sans) while shaping it in
microseconds, so 64 Mi stops a slide after about 60 characters. The document
controls only the amount of text, the fonts are the caller's, and `-timeout`
bounds the whole command; lower the budget for fonts you do not trust. Likewise
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
source, shaping, pixel and output limits still apply. DOCX currently lays out the
whole document for each selected page, so large documents repeat layout work.
The timeout and page cap bound this command's processing.

The CLI draws slides best effort by default: each piece of content it cannot
draw prints a `warning:` line naming the slide, and the rest is drawn. A
summary counts content left out and content drawn approximately, which
warnings wrap with `render.ErrApproximated`. Use
`-strict` to fail instead. It does not expand the supported formatting. Errors identify the failing page, slide or sheet. Existing outputs
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
