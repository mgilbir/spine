// Package render prepares immutable drawing snapshots for Spine's rendering
// backends. It does not perform Office layout. Format adapters must resolve
// inheritance, reject unsupported source content, and honor Forme refusals
// before handing operations to this package.
//
// This initial implementation accepts solid rectangles only. All other Forme
// operations fail explicitly, including operations outside the page. Rendering
// APIs are internal until the format adapters establish their support contracts.
package render
