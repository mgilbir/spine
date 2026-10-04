// Package view exposes the docx model's unexported state to the page
// renderer, docx/docxrender, without the docx package importing it. docx
// sets DocumentOf at init.
package view

import (
	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// Document is a document's state as the renderer reads it. Load parses the
// main part on first use; the renderer calls it only after checking the
// part's source bytes.
type Document struct {
	Reader         *opc.ReadCloser
	MainPart       string
	StylesPart     string
	Relationships  []*opc.Relationship
	PreservedParts map[string]*coxml.RawPart
	Styles         *oxml.CT_Styles
	Settings       *oxml.CT_Settings
	Load           func() *oxml.CT_Document
}

// DocumentOf snapshots d, a *docx.Document.
var DocumentOf func(d any) *Document
