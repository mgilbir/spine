package docx

import (
	"path"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/docx/internal/view"
	"github.com/mgilbir/spine/opc"
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
		PreservedParts: d.preservedParts,
		PartData:       func(name string) []byte { _, data := d.partBytes(name); return data },
		Styles:         d.styles, Settings: d.settings, Load: func() *oxml.CT_Document { return d.doc() },
		MainXML: func() ([]byte, error) { return marshalDocumentXML(d.doc()) },
		StylesXML: func() ([]byte, error) {
			if d.styles == nil {
				return nil, nil
			}
			return marshalStylesXML(d.styles)
		},
		NumberingXML: func() ([]byte, error) {
			if d.numbering == nil {
				return nil, nil
			}
			return marshalNumberingXML(d.numbering)
		},
		SettingsXML: func() ([]byte, error) {
			if d.settings == nil {
				return nil, nil
			}
			return marshalSettingsXML(d.settings)
		},
		FootnotesXML: func() ([]byte, error) {
			if d.footnotes == nil {
				return nil, nil
			}
			return marshalFootnotesXML(d.footnotes)
		},
		EndnotesXML: func() ([]byte, error) {
			if d.endnotes == nil {
				return nil, nil
			}
			return marshalEndnotesXML(d.endnotes)
		},
		HdrFtrXML: d.hdrFtrXML,
		ThemeXML: func() ([]byte, error) {
			if _, data := d.regeneratedThemePart(); data != nil {
				return data, nil
			}
			_, data := d.themePart()
			return data, nil
		}}
}

// hdrFtrXML serializes the header or footer part the main part references with
// relationship id rid.
func (d *Document) hdrFtrXML(rid string) ([]byte, error) {
	var target string
	for _, rel := range d.relationships[d.mainPart()] {
		if rel.ID == rid && (rel.Type == opc.RelTypeHeader || rel.Type == opc.RelTypeFooter) && rel.TargetMode != opc.TargetModeExternal {
			target = rel.Target
			break
		}
	}
	if target == "" {
		return nil, nil
	}
	name := target
	if !strings.HasPrefix(name, "/") {
		name = path.Join(path.Dir(d.mainPart()), name)
	}
	if hp, ok := d.headers[name]; ok && hp.hdr != nil {
		return marshalHdrFtrXML(hp.hdr, "hdr")
	}
	if fp, ok := d.footers[name]; ok && fp.ftr != nil {
		return marshalHdrFtrXML(fp.ftr, "ftr")
	}
	return nil, nil
}
