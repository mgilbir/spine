package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/enum"
)

// richRunBody is a text body in the shape PowerPoint writes for a run styled
// "Accent1, Lighter 40%" in a paragraph carrying properties this package's
// domain model does not represent. Every construct below is real PowerPoint
// output, and none of it is reachable through the pptx API:
//
//   - a:pPr    — defTabSz, rtl, fontAlgn, a:defRPr, a:buSzPts
//   - a:rPr    — lang, spc, kern, cap, and a solidFill whose schemeClr carries
//     the a:lumMod/a:lumOff pair that makes the tint
const richRunBody = `<p:txBody>` +
	`<a:bodyPr/><a:lstStyle/>` +
	`<a:p>` +
	`<a:pPr marL="342900" indent="-342900" algn="l" defTabSz="914400" rtl="0" fontAlgn="base">` +
	`<a:buSzPts val="1100"/><a:buFont typeface="Arial"/><a:buChar char="&#8226;"/>` +
	`<a:defRPr sz="1800" b="0"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr>` +
	`</a:pPr>` +
	`<a:r>` +
	`<a:rPr lang="en-GB" sz="1800" b="0" spc="-20" kern="1200" cap="none" dirty="0">` +
	`<a:solidFill><a:schemeClr val="accent1"><a:lumMod val="60000"/><a:lumOff val="40000"/></a:schemeClr></a:solidFill>` +
	`<a:latin typeface="Calibri"/>` +
	`</a:rPr>` +
	`<a:t>original</a:t>` +
	`</a:r>` +
	`</a:p>` +
	`</p:txBody>`

// deckWithRichRun builds a one-textbox deck and replaces the box's text body
// with richRunBody, so the deck reopens with a fully-styled parsed run.
func deckWithRichRun(t *testing.T) []byte {
	t.Helper()
	p := Create()
	slide := p.AddSlide()
	slide.AddTextBox().TextFrame().SetText("original")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		start := bytes.Index(xml, []byte("<p:txBody>"))
		end := bytes.Index(xml, []byte("</p:txBody>"))
		if start < 0 || end < 0 {
			t.Fatalf("slide1.xml has no txBody:\n%s", xml)
		}
		out := append([]byte{}, xml[:start]...)
		out = append(out, richRunBody...)
		return append(out, xml[end+len("</p:txBody>"):]...)
	})
}

// editFirstRun opens the deck, applies edit to the first run of the first
// paragraph of the first shape's text frame, and returns the saved slide XML.
func editFirstRun(t *testing.T, deck []byte, edit func(*Run)) string {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	tf := p.Slides()[0].Shapes()[0].(*TextBox).TextFrame()
	edit(tf.Paragraphs()[0].Runs()[0])
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return string(zipPart(t, saved, "ppt/slides/slide1.xml"))
}

// C380: editing a run's text must not restyle it. The flush used to replace the
// whole a:r with runToOxml's output, which knows only the modeled subset — so a
// SetText silently dropped the lumMod/lumOff theme tint (the text changed
// colour on screen), plus lang, spc, kern and cap.
func TestSetText_PreservesUnmodeledRunFormatting(t *testing.T) {
	out := editFirstRun(t, deckWithRichRun(t), func(r *Run) { r.SetText("edited") })
	run := runElement(t, out)

	if !strings.Contains(run, "<a:t>edited</a:t>") {
		t.Fatalf("the edit did not reach the XML:\n%s", out)
	}
	for _, want := range []string{
		`<a:lumMod val="60000"/>`,
		`<a:lumOff val="40000"/>`,
		`val="accent1"`,
		`lang="en-GB"`,
		`spc="-20"`,
		`kern="1200"`,
		`cap="none"`,
	} {
		if !strings.Contains(run, want) {
			t.Errorf("SetText dropped unmodeled run property %s\nrun: %s", want, run)
		}
	}
	// The tint must still be attached to the scheme colour, not orphaned.
	if !strings.Contains(run, `<a:schemeClr val="accent1"><a:lumMod val="60000"/><a:lumOff val="40000"/></a:schemeClr>`) {
		t.Errorf("the accent1 tint was not preserved intact:\n%s", run)
	}
}

// C380: a property the caller does set must still win, without taking the rest
// of the rPr with it.
func TestSetBold_OverlaysWithoutClobberingTheRest(t *testing.T) {
	out := editFirstRun(t, deckWithRichRun(t), func(r *Run) { r.SetBold(true) })
	run := runElement(t, out)

	if !strings.Contains(run, `b="1"`) {
		t.Errorf("SetBold(true) did not reach the run:\n%s", run)
	}
	for _, want := range []string{`<a:lumMod val="60000"/>`, `spc="-20"`, `kern="1200"`} {
		if !strings.Contains(run, want) {
			t.Errorf("SetBold dropped unmodeled run property %s\nrun: %s", want, run)
		}
	}
}

// runElement returns the first <a:r>...</a:r> of the slide XML. The assertions
// below must look only inside the run: the paragraph's a:defRPr carries its own
// b="0", so a whole-document substring match would pass without the fix.
func runElement(t *testing.T, slideXML string) string {
	t.Helper()
	start := strings.Index(slideXML, "<a:r>")
	end := strings.Index(slideXML, "</a:r>")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("no a:r element in slide XML:\n%s", slideXML)
	}
	return slideXML[start : end+len("</a:r>")]
}

// C518: a run explicitly marked b="0" inside a bold-inheriting placeholder must
// stay unbolded after an unrelated edit. The domain model folds b="0" into
// bold=false and the old emitter wrote nothing for false, so the attribute
// vanished and the run re-inherited bold.
func TestEditingUnboldedRun_DoesNotReBoldIt(t *testing.T) {
	out := editFirstRun(t, deckWithRichRun(t), func(r *Run) { r.SetText("edited") })
	run := runElement(t, out)

	if !strings.Contains(run, `b="0"`) {
		t.Errorf("explicit b=\"0\" was dropped from the run, so it re-inherits bold:\n%s\nfull: %s", run, out)
	}
}

// C518: SetBold(false) must be expressible on a run that has no b attribute —
// it means "not bold", which only an explicit b="0" can say when the
// placeholder inherits bold.
func TestSetBoldFalse_EmitsExplicitZero(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tf := s.AddTextBox().TextFrame()
	tf.SetText("hello")
	tf.Paragraphs()[0].Runs()[0].SetBold(false)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(out, `b="0"`) {
		t.Errorf(`SetBold(false) emitted nothing, so the run inherits bold:\n%s`, out)
	}
}

// C517: a paragraph property edit must not replace the whole a:pPr with the
// modeled subset. Realigning a paragraph used to drop its defRPr, rtl,
// fontAlgn, defTabSz and buSzPts.
func TestSetAlignment_PreservesUnmodeledParagraphProperties(t *testing.T) {
	deck := deckWithRichRun(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	tf := p.Slides()[0].Shapes()[0].(*TextBox).TextFrame()
	tf.Paragraphs()[0].SetAlignment(enum.TextAlignCenter)

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if !strings.Contains(out, `algn="ctr"`) {
		t.Fatalf("the alignment edit did not reach the XML:\n%s", out)
	}
	for _, want := range []string{
		`defTabSz="914400"`,
		`rtl="0"`,
		`fontAlgn="base"`,
		`<a:buSzPts val="1100"/>`,
		`<a:defRPr sz="1800"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SetAlignment dropped unmodeled paragraph property %s\n%s", want, out)
		}
	}
}

// C517: a paragraph that never carried lvl must not gain lvl="0" just because
// some other property was set on it.
func TestParagraphEdit_DoesNotInventLevelZero(t *testing.T) {
	deck := deckWithRichRun(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	tf := p.Slides()[0].Shapes()[0].(*TextBox).TextFrame()
	tf.Paragraphs()[0].SetAlignment(enum.TextAlignRight)

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(out, `lvl="0"`) {
		t.Errorf(`a paragraph edit invented lvl="0":\n%s`, out)
	}
}

// C517/C518: a paragraph built from scratch must still emit everything the
// caller asked for, including an explicit zero space-before.
func TestNewParagraph_EmitsExplicitZeroSpacing(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tf := s.AddTextBox().TextFrame()
	tf.SetText("hello")
	tf.Paragraphs()[0].SetSpaceBefore(0)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(out, `<a:spcBef><a:spcPts val="0"/></a:spcBef>`) {
		t.Errorf("SetSpaceBefore(0) emitted nothing, so the paragraph inherits spacing:\n%s", out)
	}
}
