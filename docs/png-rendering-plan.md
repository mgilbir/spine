# Native rendering with Forme: PNG and SVG

Status: proposal; no renderer or PR has been published.

Rendering must run entirely in Go. Forme is the only permitted additional
dependency; other dependencies require an explicit decision by the maintainer.
Use Go standard-library facilities and github.com/mgilbir/forme. The local
Forme checkout inspected is at f443b39. It provides typography, paragraph layout
and drawing operations, but neither PNG output nor multi-page fragmentation.
Spine must still implement Office-specific inheritance/layout and DOCX pagination.
Forme requires Go 1.26 while Spine declares Go 1.25; adoption needs a pinned
version and an explicit toolchain/CI decision.

## SVG feasibility and decision

SVG export is easier than implementing a raster backend from scratch. Rectangles,
paths, transformations, gradients and clipping map to SVG primitives. Forme's
shape.Face.GlyphOutline provides move/line/quadratic/cubic segments, allowing
already-shaped glyphs to become paths with explicit placement. Use those paths
rather than SVG text for reproducible typography; browser shaping/font fallback
would otherwise move correctness out of Spine's control. Preserve contextual
shaping using layout.ShapedGlyphs and DrawText placement/style metadata.

Glyph outlines need font-unit scaling, Y-axis inversion, correct contour closure
and fill rules. Forme reports missing outlines; bitmap-only glyphs, color glyph
layers, standard faces without font programs and exhausted font budgets need
explicit policy/errors. Outlining loses selectable text and can enlarge files;
reuse bounded glyph definitions within the document where practical.

A generated SVG profile can contain paths, solid fills, gradients, transforms,
internal references, clips and embedded normalized bitmap data. Never copy raw
OOXML strings or embedded SVG markup into output. Escape through an XML encoder.
Exclude scripts, event attributes, foreignObject, external URLs, external fonts
and external stylesheets. Generate internal IDs and references ourselves. Bound
commands, path segments, nesting, embedded bytes and serialized bytes. Advanced
filters and color fonts remain explicit compatibility work.

However, SVG-to-PNG does not eliminate rasterization. A pure Go SVG converter
must support every generated feature and still enforce pixel/memory/work limits.
Under the dependency contract, that converter would have to be implemented in
Spine using only the standard library and Forme. A restricted generated subset
still needs XML/path parsing, gradients, clipping, compositing, bitmap sampling
and scan conversion. Serialization and reparsing add work and another failure
boundary without removing the difficult painting work.

Recommendation: retain a shared immutable drawing representation and render PNG
directly from it. Implement bounded raster primitives within Spine; PNG encoding
uses image/png. SVG is an optional serialization backend that can be delivered
earlier if valuable, without putting SVG parsing on the PNG path. Both backends
reuse the same Forme-shaped glyph geometry. No third-party rasterizer, SVG
converter, vector library or native/browser renderer is proposed.
No performance comparison has been benchmarked yet.

## Proposed PR stack

| PR | Scope | Acceptance evidence |
|----|-------|---------------------|
| 1 | Forme pin/toolchain decision, contracts, support matrix, immutable drawing data, geometry and resource limits | Forme API integration, checked sizing/overflow, explicit unsupported detection, ownership/source preservation |
| 2 | In-repository rasterization design/spike; optional bounded SVG serializer | Common fixtures for glyphs, nested clipping, transforms, gradients, alpha and images; standard library plus Forme only; bounded-work review; measured baseline |
| 3 | Bounded direct PNG backend using in-repository raster primitives | Independent pixel fixtures, output/pixel/work limits, cancellation, hostile images/paths, operation completeness |
| 4 | PPTX static-slide integration | Masters/layouts/themes, z-order, supported shapes/text/images, unsaved edits, strict unsupported errors, source preservation |
| 5 | XLSX bounded sheet previews | Explicit range, sparse cells, dimensions, hidden rows/columns, merges/styles/formatted cached values, cross-sheet dependencies |
| 6 | DOCX flow layout and real page fragmentation using Forme paragraphs | Supported style cascade, sections/margins, wrapping, breaks and keep constraints, tables/headers as declared, page geometry/count |
| 7 | Expanded compatibility corpus, fuzz/resource/performance gates and documentation | Independent references, pinned fonts, measured latency/memory/cancellation, explicit fidelity limits |

Each PR includes its own tests/docs and bases on its predecessor. Split painting
or pagination further if necessary. The initial support matrix is still pending;
unsupported visible content fails explicitly instead of disappearing. Sheet print
layout is distinct from whole-sheet/range previews. Charts, SmartArt, effects,
equations and floating Word objects require their own work unless initially included.

## Contract and cross-cutting gates

Prepare a validated snapshot containing unsaved edits, with no mutation of source
parts or subsequent save behavior. Reuse layout for requested targets, keeping
later source mutations independent. Audit SaveBytes side effects before using it
for snapshots. Format adapters live in their format packages to inspect internal
models and raw preserved markup; a shared painter must not import these packages.

Select static slides, bounded sheet ranges, or one-based physical Word pages.
Define hidden targets, DPI/pixel-box sizing, aspect ratio, rounding, background,
blank targets and partial-writer behavior. Preflight unsupported visible/raw
content and fonts before writing. Expose errors for invalid requests, unsupported
features, limits, cancellation, layout and output failure with part/feature context.

Fonts are explicitly caller-supplied, with reported opt-in fallback. No network,
ambient host-font lookup, macros, executable objects, formula execution or external
relationship loading. Initially use cached formula values and fail when required
values are missing. Honor Forme's Refused verdict separately from its potentially
truncated Findings. Reject unknown drawing operations instead of dropping them.

Bound input/ZIP expansion, XML work/depth, visited cells, glyphs, fonts, decoded
images, path segments/subdivision, drawing commands, pagination passes/pages,
output dimensions/pixels/bytes and cache residency. Audit Forme's own limits and
cancellation entry points; a timeout goroutine does not stop layout. Check context
through each stage. Inspect image dimensions before decode and audit decoder
allocations. Higher-assurance deployments can run the same Go renderer in an
OS-limited worker without an external engine.

Use structural assertions plus independently reviewed visual references and
fixed licensed fonts. Forme's conformance tests do not validate OOXML adapters.
Verify source preservation for Create/Open and unsaved edits. Fuzz hostile fonts,
images, transforms, sparse ranges, nested groups and pathological pagination,
asserting bounded work rather than merely no panic.

Benchmark layout and painting separately; include repeated renders, typical and
large files, peak memory, allocation counts, output size and cancellation latency.
Prepare once and paint selected targets only. Bound per-session caches; race-test
shared fonts/sessions in the repository's required resource-capped scope.

## References

- Local Forme README.md, layout/paint.go, layout/path.go and shape/outline.go.
- [SVG specification](https://www.w3.org/TR/SVG2/)

## Potential PDF backend with pdf0

Exploratory only: pdf0 has not been approved or added as a dependency.

The inspected local pdf0 checkout already implements a Forme drawing-operation
backend in htmlpdf/canvas.go. Font adoption, glyph drawing, embedding/subsetting,
page creation, graphics and PDF serialization are reusable building blocks.
However, writePage is private and htmlpdf.Render accepts HTML/CSS rather than an
arbitrary prepared display list; exposing a bounded multi-page display-list API
upstream would avoid duplicating that backend in Spine. The shared representation
should retain logical text, shaping context/positioned glyphs and font identity,
not only outlines, so PDF can preserve extractable text while PNG/SVG use paths.
Selectable text is separate from tagged reading order and PDF/UA compliance.

PDF output could ship before direct PNG because much of its backend exists.
It does not solve Office layout or DOCX fragmentation, and pdf0 is not a general
PDF-page rasterizer: its htmlpdf visual tests use external Ghostscript/Poppler.
A PDF-to-PNG intermediate would introduce a PDF interpreter/rasterizer and is not
a shortcut under the entirely-in-Go dependency contract. Keep PDF, SVG and PNG
as sibling backends of shared prepared pages, with backend-specific capability
checks and the same source-preservation/resource guarantees.

Dependency policy needs a separate maintainer decision. The inspected pdf0
go.mod requires Forme, formalis, gopenjpeg, golittlecms and golang.org/x/text,
and Go 1.26. Adding pdf0 is not equivalent to adding only Forme; those requirements
and reachable package imports must be assessed. A separate adapter module can
keep pdf0 outside Spine's core dependency graph, or pdf0 could expose a leaner
writer module upstream. An ordinary optional package in Spine's existing module
would not isolate module requirements.
