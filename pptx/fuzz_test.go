package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// buildValidPptxFuzzSeed creates a small valid presentation in-process so no
// corpus binaries need committing.
func buildValidPptxFuzzSeed(f *testing.F) []byte {
	f.Helper()
	p := Create()
	slide := p.AddSlide()
	tb := slide.AddTextBox()
	tb.TextFrame().SetText("fuzz seed")
	valid, err := p.SaveBytes()
	if err != nil {
		f.Fatalf("building valid pptx seed: %v", err)
	}
	return valid
}

// fuzzExercisePptx opens the bytes as a presentation and, on success, walks
// a bounded slice of the model and round-trips it (SaveBytes then re-open).
// Any panic is a bug; errors are expected and fine.
func fuzzExercisePptx(data []byte) {
	p, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return
	}
	defer func() { _ = p.Close() }()

	for i, s := range p.Slides() {
		if i >= 8 {
			break
		}
		_ = s.Name()
		_ = s.Layout()
		for j, sh := range s.Shapes() {
			if j >= 32 {
				break
			}
			_ = sh.Name()
			_, _ = sh.Position()
			_, _ = sh.Size()
		}
		for j, ph := range s.Placeholders() {
			if j >= 32 {
				break
			}
			_ = ph.Name()
		}
	}

	out, err := p.SaveBytes()
	if err != nil {
		return
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		return
	}
	_ = p2.Slides()
	_ = p2.Close()
}

// FuzzOpenPptx feeds arbitrary bytes to the presentation opener.
func FuzzOpenPptx(f *testing.F) {
	valid := buildValidPptxFuzzSeed(f)

	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	f.Add(valid[:len(valid)/2])
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)/2] ^= 0xFF
	f.Add(corrupt)
	// A package that claims to be a presentation but whose main part is
	// truncated garbage, so the PML parse paths see hostile XML.
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`},
		{"ppt/presentation.xml", `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst><p:sldId id="256"`},
	}))
	// A handful of small real files when the gitignored corpus is present;
	// never committed.
	for _, seed := range fuzzseed.CorpusSeeds("pptx", 5, 256<<10) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzExercisePptx(data)
	})
}

// FuzzPptxSlideXML packs the fuzz bytes into the first slide part of an
// otherwise-valid presentation, so the PML parsers see hostile XML directly
// instead of the fuzzer having to invent whole valid zip archives.
func FuzzPptxSlideXML(f *testing.F) {
	valid := buildValidPptxFuzzSeed(f)
	const slidePart = "ppt/slides/slide1.xml"
	orig := fuzzseed.ZipEntry(valid, slidePart)
	if orig == nil {
		f.Fatalf("valid pptx seed has no %s", slidePart)
	}

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add(orig[:len(orig)/2])
	f.Add([]byte(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>` +
		strings.Repeat(`<p:grpSp><p:grpSpPr/>`, 200) + strings.Repeat(`</p:grpSp>`, 200) +
		`</p:spTree></p:cSld></p:sld>`))
	f.Add([]byte(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:spPr><a:xfrm><a:off x="99999999999999999999" y="-99999999999999999999"/><a:ext cx="0" cy="0"/></a:xfrm></p:spPr></p:sp></p:spTree></p:cSld></p:sld>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		wrapped := fuzzseed.ReplaceZipEntry(valid, slidePart, data)
		if wrapped == nil {
			t.Skip("seed package unreadable")
		}
		fuzzExercisePptx(wrapped)
	})
}

// fuzzReparsePptx saves a presentation and re-opens the bytes, walking the
// slides, their animations and the deck's sections. Any panic is a bug; errors
// are expected and fine.
func fuzzReparsePptx(p *Presentation) {
	out, err := p.SaveBytes()
	if err != nil {
		return
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		return
	}
	defer func() { _ = p2.Close() }()
	for _, s := range p2.Slides() {
		for _, a := range s.Animations() {
			_ = a.ShapeID()
			_ = a.Effect()
			_ = a.Trigger()
			_ = a.ByParagraph()
		}
	}
	for _, sec := range p2.Sections() {
		_ = sec.Name()
		_ = sec.ID()
		_ = sec.Slides()
	}
}

// FuzzPptxAddAnimation fuzzes Slide.AddAnimation: the target shape id (including
// ids that address no shape on the slide), the effect and trigger enum values
// (including out-of-range ones), and the build-by-paragraph flag. It creates a
// slide with a multi-paragraph text box, adds the animation, then saves and
// re-opens.
func FuzzPptxAddAnimation(f *testing.F) {
	f.Add(uint32(2), int8(1), int8(0), false, "one\ntwo\nthree")
	f.Add(uint32(0), int8(0), int8(0), true, "")
	f.Add(uint32(999999), int8(-5), int8(99), true, "solo")
	f.Add(uint32(2), int8(6), int8(2), true, "a\nb")
	f.Add(uint32(2), int8(11), int8(1), false, "x")

	f.Fuzz(func(t *testing.T, shapeID uint32, effect, trigger int8, byPara bool, text string) {
		p := Create()
		slide := p.AddSlide()
		tb := slide.AddTextBox()
		tb.TextFrame().SetText(text)

		// Prefer a real shape id half the time so the timing tree actually
		// targets something; otherwise use the fuzzed (possibly absent) id.
		id := shapeID
		if shapeID&1 == 0 {
			for _, sh := range slide.Shapes() {
				if ided, ok := sh.(interface{ ID() uint32 }); ok {
					id = ided.ID()
					break
				}
			}
		}

		a := slide.AddAnimation(id, AnimationEffect(effect), AnimationTrigger(trigger))
		a.SetByParagraph(byPara)
		// A second animation exercises the append-into-existing-sequence path.
		slide.AddAnimation(shapeID, AnimationEffect(effect+1), AnimationTrigger(trigger+1))
		fuzzReparsePptx(p)
	})
}

// FuzzPptxAddSection fuzzes Presentation.AddSection and section membership: the
// section name, how many slides the deck holds, and a per-slide assignment
// selector. It creates the slides and sections, assigns membership, then saves
// and re-opens.
func FuzzPptxAddSection(f *testing.F) {
	f.Add("Intro", uint8(3), "Body", []byte{0, 1, 2})
	f.Add("", uint8(0), "", []byte{})
	f.Add("Only", uint8(1), "Only", []byte{9})
	f.Add("A", uint8(5), "B", []byte{0, 0, 0, 1, 1})

	f.Fuzz(func(t *testing.T, name1 string, nSlides uint8, name2 string, assign []byte) {
		p := Create()
		// Cap the slide count so a single fuzz iteration stays cheap.
		n := int(nSlides % 8)
		slides := make([]*Slide, 0, n)
		for i := 0; i < n; i++ {
			slides = append(slides, p.AddSlide())
		}

		sec1 := p.AddSection(name1)
		sec2 := p.AddSection(name2)
		secs := []*Section{sec1, sec2}

		for i, sl := range slides {
			if i >= len(assign) {
				break
			}
			switch assign[i] % 3 {
			case 0:
				sec1.AddSlide(sl)
			case 1:
				secs[assign[i]%uint8(len(secs))].AddSlide(sl)
			case 2:
				p.MoveSlideToSection(sl, nil)
			}
		}
		sec1.SetName(name2)
		fuzzReparsePptx(p)
	})
}
