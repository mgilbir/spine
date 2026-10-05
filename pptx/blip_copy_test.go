package pptx

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// A slide copied the way DuplicateSlide and the import paths copy one (marshal,
// then decode with the standard decoder, which registers no source) keeps a
// picture's repeated and unknown blip effects in place.
func TestDeepCopyKeepsRepeatedAndUnknownBlipEffects(t *testing.T) {
	const src = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/><p:pic><p:nvPicPr><p:cNvPr id="2" name="Picture 1"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"><a:lum bright="10000"/><a:grayscl/><a:grayscl/><a:futureEffect k="v"/><a:alphaModFix amt="50000"/></a:blip><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr/></p:pic></p:spTree></p:cSld></p:sld>`
	var orig oxml.Slide
	if err := xmlb.UnmarshalWithSource([]byte(src), &orig); err != nil {
		t.Fatal(err)
	}
	first, err := marshalSlide(&orig)
	if err != nil {
		t.Fatal(err)
	}
	const effects = `<a:lum bright="10000"/><a:grayscl/><a:grayscl/><a:futureEffect k="v"/><a:alphaModFix amt="50000"/>`
	if !strings.Contains(string(first), effects) {
		t.Fatalf("source-parsed slide lost effects:\n%s", first)
	}

	var cp oxml.Slide
	//xmlguard:lenient deep copy of the slide XML this library just marshaled, not a part read
	if err := xml.Unmarshal(first, &cp); err != nil {
		t.Fatal(err)
	}
	second, err := marshalSlide(&cp)
	if err != nil {
		t.Fatal(err)
	}
	// Without a source the second grayscl and the unknown effect are rebuilt
	// from tokens, each declaring the DrawingML namespace inline.
	const ns = ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
	want := `<a:lum bright="10000"/><a:grayscl/><a:grayscl` + ns + `/><a:futureEffect` + ns + ` k="v"/><a:alphaModFix amt="50000"/>`
	if !strings.Contains(string(second), want) {
		t.Fatalf("copied slide lost effects, want %s in:\n%s", want, second)
	}
}
