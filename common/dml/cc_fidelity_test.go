package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_EmbeddedWAVAudioFile (a:snd/p:snd) has a builtIn attribute (xsd:boolean,
// default false) the model did not type, so a built-in system sound
// (builtIn="1") was silently dropped on save. Found by Common Crawl validation
// (pptx 74c090).
func TestEmbeddedWAV_PreservesBuiltIn(t *testing.T) {
	input := `<snd xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" name="explode" builtIn="1"/>`
	var v EmbeddedWAVXML
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.BuiltIn != "1" {
		t.Fatalf("BuiltIn = %q, want %q (attribute dropped)", v.BuiltIn, "1")
	}
	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(nsA, "snd", &v)
	out := b.String()
	if !strings.Contains(out, `builtIn="1"`) {
		t.Errorf("builtIn dropped on re-marshal: %s", out)
	}
}

// A typed a14 image-adjustment effect must preserve an attribute the model does
// not type: producers emit off-spec attributes (an amount on
// a14:brightnessContrast) that would otherwise be lost through the typed struct.
// Found by Common Crawl validation (pptx a766e6).
func TestA14Brightness_PreservesUnmodeledAttr(t *testing.T) {
	input := `<extLst xmlns="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<ext uri="{BEBA8EAE-BF5A-486C-A8C5-ECC9F3942E4B}"><a14:imgProps><a14:imgLayer r:embed="rId2">` +
		`<a14:imgEffect><a14:brightnessContrast bright="20000" contrast="-40000" amount="5"/></a14:imgEffect>` +
		`</a14:imgLayer></a14:imgProps></ext></extLst>`

	var el ExtLst
	if err := xml.Unmarshal([]byte(input), &el); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(el.Ext) != 1 || el.Ext[0].ImgProps == nil || el.Ext[0].ImgProps.ImgLayer == nil {
		t.Fatalf("imgProps not parsed: %+v", el.Ext)
	}
	effects := el.Ext[0].ImgProps.ImgLayer.ImgEffects
	if len(effects) != 1 || effects[0].Brightness == nil {
		t.Fatalf("brightnessContrast not parsed: %+v", effects)
	}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(nsA, "extLst", &el)
	out := b.String()
	if !strings.Contains(out, `amount="5"`) {
		t.Errorf("off-spec amount attribute dropped on re-marshal: %s", out)
	}
	if !strings.Contains(out, `bright="20000"`) || !strings.Contains(out, `contrast="-40000"`) {
		t.Errorf("typed brightness/contrast attributes lost: %s", out)
	}
}
