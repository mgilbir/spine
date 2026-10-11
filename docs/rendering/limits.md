# Rendering limits

This page lists the bounds every render works within: the fields of
`render.Limits` with their defaults, the source budgets of
`render.Options`, and the extra budgets of Word documents. It is for
callers who render untrusted files or need to raise a bound; the command
line raises several of them ([spine-render](cli.md)).
It is part of the [rendering guide](../rendering.md), which has the quick
start and the entry points.

The fields of `render.Limits` bound each prepared page, slide or sheet range.
Zero fields select these defaults; negative limits fail with
`render.ErrInvalid`, and exceeding one fails with `render.ErrLimit`:

| Limit | `render.Limits` field | Default |
|---|---|---:|
| Maximum output/source-image dimension | `MaxDimension` | 8192 |
| Output pixels | `MaxPixels` | 16 Mi |
| Drawing operations | `MaxOperations` | 100000 |
| Pixel visits including overdraw | `MaxPixelVisits` | 64 Mi |
| Encoded output bytes | `MaxOutputBytes` | 32 MiB |
| Source/flattened path segments | `MaxPathSegments` | 100000 each |
| Clip depth | `MaxClipDepth` | 32 |
| Scanline/coverage work | `MaxEdgeChecks` | 64 Mi units |
| Glyphs | `MaxGlyphs` | 100000 |
| Logical text metadata | `MaxTextBytes` | 1 MiB |
| Fonts | `MaxFonts` | 32 |
| Aggregate font program bytes | `MaxFontBytes` | 32 MiB |
| Charged text shaping work | `MaxShapeWork` | 64 Mi units |
| Input bytes per shaped run, including context/features | `MaxRunBytes` | 4096 |
| Unique source-image pixels | `MaxImagePixels` | 4 Mi |
| Images | `MaxImages` | 32 |
| Encoded image input / aggregate normalized PNG bytes | `MaxImageBytes` | 32 MiB |

## `MaxShapeWork`

`MaxShapeWork` measures Forme's shaping work: each subtable, ligature and rule
tried and each glyph a match walks over (about 50 units a byte of Latin text,
300 of Devanagari, 350 of Nastaliq); it is not a duration. The default, 64 Mi
units a slide, page or sheet, covers ordinary documents many times over while a
font's runaway rules are still charged in full.

## Source budgets

`Options.MaxSourceBytes` and `MaxLayoutNodes` default to 16 MiB and 100000 for
adapter capability inspection; node accounting includes attributes. Drawing/pixel limits remain independent.

## DOCX budgets

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
