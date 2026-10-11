// Package render prepares immutable drawing snapshots for Spine's rendering
// backends. It does not perform Office layout. Format adapters must resolve
// inheritance, reject unsupported source content, and honor Forme refusals
// before handing operations to this package.
//
// This implementation accepts solid rectangles, filled paths, path clips,
// single-tile linear and radial gradient fills, decoded raster images,
// positioned horizontal glyphs and horizontal text runs, including color
// glyphs. Other Forme operations fail explicitly, including operations
// outside the page. The public API is the render package, which wraps this
// one; the docxrender, pptxrender and xlsxrender adapters build its operations.
package render
