package docx

import (
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/docx/internal/view"
)

// The page renderer, docx/docxrender, reads the model through
// docx/internal/view; see that package.
func init() {
	view.DocumentOf = func(v any) *view.Document { return v.(*Document).view() }
}

// view snapshots the document for the renderer.
func (d *Document) view() *view.Document {
	main := d.mainPart()
	return &view.Document{Reader: d.reader, MainPart: main, StylesPart: d.stylesPartName(), Relationships: d.relationships[main],
		PreservedParts: d.preservedParts, Styles: d.styles, Settings: d.settings, Load: func() *oxml.CT_Document { return d.doc() }}
}
