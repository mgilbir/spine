# Rendering foundation

This internal package accepts prepared Forme solid rectangles only. It provides
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

Next layers must implement bounded paths/clipping and Forme glyph painting,
followed by Office-specific adapters. Preserve logical text and font identity
for a possible PDF sibling backend; pdf0 and its dependencies are not included.
