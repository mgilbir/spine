# Rendering foundation

This internal package accepts Forme solid rectangles, filled paths and nested
path clips, and positioned horizontal glyphs (`DrawGlyphs`) and horizontal `DrawText` runs, plus decoded raster `DrawImage` operations. It provides
no page, slide or sheet layout API yet. Other operations fail explicitly, even
when off-page; raw-source capability checks remain the format adapter's job.

`Prepare` copies commands and clips them in CSS-pixel coordinates to a physical
EMU page. `Size` rounds output dimensions upwards at the requested DPI, checking
dimension, pixel count and address-space limits before four-byte pixel allocation.
Zero DPI is 96. Zero limit fields select documented implementation defaults;
negative limits fail. Prepared pages are safe to render concurrently.

`WritePNG` paints directly to transparent black using source-over in sRGB, then
uses standard-library PNG encoding. Fractional edges use independent per-rectangle
area coverage. This is an antialiasing approximation: touching or overlapping
fractional edges may differ from a vector renderer that retains subpixel coverage
through compositing. The initial primitive backend does not claim pixel identity
with an SVG viewer or Office. Pixel visits are checked cumulatively before
allocation, including repeated overdraw; output bytes are capped during encoding.

`WriteSVG` writes generated geometry through an XML encoder, using the same
size/coordinates/colors and painter order. It introduces no script, font, source
markup, filesystem access or external references. SVG is a sibling backend,
not an intermediate on the PNG path.

Both writers can leave partial output on encoding, byte-limit, cancellation or
writer failure. Callers needing atomic output must buffer within a separate
budget. Context checks bound painting intervals and output writes, but cannot
interrupt a caller's blocking writer. PNG compression runs between writer checks;
the configured pixel bound limits that work. No cancellation goroutine is used.

Paths use even-odd filling, with half-open vertex handling. Ellipse arcs are
flattened per DPI with a 1/16 pixel sagitta bound; source and flattened segment
counts are capped separately by the same limit (100,000 by default). Nested
clips are intersected geometrically before computing coverage, with a default
depth limit of 32. PNG scanline sorting/checking and coverage work is bounded
before pixel allocation (64 million units by default). Filled paths use eight
vertical samples with analytic horizontal coverage; this is documented numerical
approximation, distinct from SVG's exact arc serialization.

Glyphs use Forme's actual quadratic/cubic outlines with nonzero filling,
baseline/Y-axis conversion and thousandths-of-em positioning. Each glyph is
painted separately to preserve translucent overdraw. Preparation clones faces,
copies resolved geometry and retains logical text once per run with a font
fingerprint. No mutable source face or glyph slice survives in the snapshot.
Defaults cap glyphs at 100,000, text metadata at 1 MiB, font count at 32 and
aggregate unique font-program bytes at 32 MiB. Font programs must remain immutable
during preparation; parsing fonts and their decoded input budgets remain the
font provider's responsibility. Color glyphs are drawn from their COLR, CBDT and
sbix data (see docs/rendering.md); vertical glyph placement and Apple bitmap
fonts fail explicitly instead of silently drawing blank glyphs.
Missing outlines and Forme's font-layout truncation findings also fail.

Bezier flattening checks a 1/16 output-pixel control-hull distance and caps
recursion at 24; SVG preserves the original quadratic/cubic curves. Forme's
outline routine is bounded internally but is not context-interruptible inside
one glyph; context is checked when segments are delivered and between glyphs.
`DrawText` uses Forme's bounded contextual shaping API, retaining resolved RTL
direction, merging neighbours, kerning and feature settings. Defaults bound
aggregate charged shaping work at 64 million units and each run's text, context
and feature strings at 4096 bytes. Missing characters, lookup/recursion
exhaustion and reported font truncation fail before a page is returned. Horizontal
width scaling is supported; vertical text and letter spacing fail explicitly.
Font parsing and Unicode preprocessing retain Forme's own bounds and are not
interrupted within a single phase.

Next layers must implement images and Office-specific adapters. Preserve logical text and font identity
for a possible PDF sibling backend; pdf0 and its dependencies are not included.

Images are copied to an eight-bit RGBA snapshot and normalized to PNG for SVG
embedding. Only validated standard-library image storage is accepted; source
keys and markup are never serialized. Pixel-centre nearest-neighbour sampling
preserves mapping through rectangular and path clips. SVG requests pixelated
sampling, though a viewer controls its final sampling behaviour. Defaults cap
aggregate unique source pixels at 4 million, normalized image bytes at 32 MiB
and images at 32. Even off-page images undergo capability and budget checks.
`DecodeImage` checks encoded bytes and dimensions before standard-library PNG
or JPEG decoding; it loads no external resources and applies no EXIF orientation.
Decoders are not interruptible within a single call. Image pixels and programs
must remain immutable during preparation. Normalization intentionally reduces
sixteen-bit sources to eight bits. Tiling remains unsupported.
