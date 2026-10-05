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
	// MainXML, StylesXML, SettingsXML and ThemeXML serialize the document's
	// current in-memory state, including unsaved edits, to the XML the part
	// would be saved as. The styles, settings and theme functions return nil
	// bytes when the document has no such part. None of them mutates the
	// document.
	MainXML, StylesXML, SettingsXML, ThemeXML func() ([]byte, error)
	// FootnotesXML and EndnotesXML serialize the footnotes and endnotes parts
	// the same way, returning nil bytes when the document has none.
	FootnotesXML, EndnotesXML func() ([]byte, error)
	// HdrFtrXML serializes the header or footer part the main part references
	// with relationship id rid, as it would be saved. It returns nil bytes when
	// rid names no header or footer part of the document.
	HdrFtrXML func(rid string) ([]byte, error)
}

// DocumentOf snapshots d, a *docx.Document.
var DocumentOf func(d any) *Document
