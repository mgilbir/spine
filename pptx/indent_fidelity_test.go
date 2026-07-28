package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// prettyPrint reindents an XML part the way a wild producer does: a newline
// plus depth-proportional indent before every tag that follows another tag.
// Character data is never touched, so <a:t>text</a:t> and every element whose
// only content is text keep their exact bytes — and no element gains a
// whitespace-only body, which would be character data rather than indentation.
func prettyPrint(t *testing.T, data []byte, unit string) []byte {
	t.Helper()
	var out bytes.Buffer
	depth := 0
	prevWasTag := false
	prevWasStart := false
	for i := 0; i < len(data); {
		if data[i] != '<' {
			j := i
			for j < len(data) && data[j] != '<' {
				j++
			}
			out.Write(data[i:j])
			prevWasTag, prevWasStart = false, false
			i = j
			continue
		}
		j := i + 1
		var quote byte
		for j < len(data) {
			c := data[j]
			if quote != 0 {
				if c == quote {
					quote = 0
				}
				j++
				continue
			}
			if c == '"' || c == '\'' {
				quote = c
			}
			j++
			if c == '>' {
				break
			}
		}
		tag := data[i:j]
		isDecl := len(tag) > 1 && (tag[1] == '?' || tag[1] == '!')
		isEnd := len(tag) > 1 && tag[1] == '/'
		isEmpty := len(tag) > 2 && tag[len(tag)-2] == '/'
		if isEnd {
			depth--
		}
		// An end tag right after its own start tag stays put: breaking it
		// would turn an empty element into one holding whitespace.
		if prevWasTag && !isDecl && (!isEnd || !prevWasStart) {
			out.WriteString("\n" + strings.Repeat(unit, depth))
		}
		out.Write(tag)
		if !isDecl && !isEnd && !isEmpty {
			depth++
		}
		prevWasTag = true
		prevWasStart = !isDecl && !isEnd && !isEmpty
		i = j
	}
	return out.Bytes()
}

// prettyPrintedDeck builds a deck through the API and reindents the three
// always-regenerated part kinds — presentation.xml, every slideLayout and every
// slideMaster — using a different indent unit for each kind, the way wild
// packages mix them (7330e2df353e31cc indents layouts with tabs and
// presentation.xml with single spaces).
func prettyPrintedDeck(t *testing.T) ([]byte, []string) {
	t.Helper()
	p := Create()
	slide := p.AddSlide()
	slide.AddTextBox().TextFrame().SetText("hello")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	var indented []string
	for _, file := range reader.File {
		src, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			t.Fatal(err)
		}
		unit := ""
		switch {
		case file.Name == "ppt/presentation.xml":
			unit = " "
		case strings.HasPrefix(file.Name, "ppt/slideLayouts/slideLayout"):
			unit = "\t"
		case strings.HasPrefix(file.Name, "ppt/slideMasters/slideMaster"):
			unit = "    "
		}
		if unit != "" {
			content = prettyPrint(t, content, unit)
			indented = append(indented, "/"+file.Name)
		}
		dst, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if len(indented) < 3 {
		t.Fatalf("expected presentation.xml, a layout and a master to reindent, got %v", indented)
	}
	return out.Bytes(), indented
}

// TestPrettyPrintedPartsRoundTripByteIdentically is the C587 regression: a deck
// whose presentation.xml, layouts and masters were pretty-printed by its
// producer must come back part-for-part byte-identical from a save that changed
// nothing. Those three part kinds are regenerated from the model on every save,
// so before the fix the regeneration deleted every indent run.
func TestPrettyPrintedPartsRoundTripByteIdentically(t *testing.T) {
	deck, indented := prettyPrintedDeck(t)
	orig := zipParts(t, deck)

	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	got := zipParts(t, saved)

	for name, want := range orig {
		have, ok := got[name]
		if !ok {
			t.Errorf("part %s missing from the saved package", name)
			continue
		}
		if !bytes.Equal(want, have) {
			t.Errorf("part %s is not byte-identical (%d bytes in, %d out)\n orig: %q\nsaved: %q",
				name, len(want), len(have), truncate(want), truncate(have))
		}
	}
	for name := range got {
		if _, ok := orig[name]; !ok {
			t.Errorf("save introduced part %s", name)
		}
	}
	// Guard the fixture itself: if prettyPrint ever stopped indenting, the
	// assertions above would pass for the wrong reason.
	for _, name := range indented {
		if !bytes.Contains(orig[name], []byte(">\n")) {
			t.Errorf("fixture part %s was not pretty-printed", name)
		}
	}
}

// TestProgrammaticDeckEmitsNoIndentation pins the other half of the contract:
// capture is per-instance, so a deck with no source part emits the canonical
// unindented form for the three regenerated part kinds, exactly as it did
// before C587. (Other parts — theme, viewProps, app.xml — come from indented
// templates and are outside this fix.)
func TestProgrammaticDeckEmitsNoIndentation(t *testing.T) {
	p := Create()
	p.AddSlide()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for name, content := range zipParts(t, data) {
		if !regeneratedPartKind(name) {
			continue
		}
		checked++
		body := content
		// Everything after the XML declaration must be one run of markup.
		if i := bytes.Index(body, []byte("?>")); i >= 0 {
			body = body[i+2:]
		}
		if bytes.Contains(body, []byte(">\n")) || bytes.Contains(body, []byte(">\t")) {
			t.Errorf("programmatically created deck gained indentation in %s: %q", name, truncate(body))
		}
	}
	if checked < 3 {
		t.Fatalf("expected presentation.xml, a layout and a master to check, got %d parts", checked)
	}
}

// regeneratedPartKind reports whether a zipParts key names one of the pptx
// parts that are rebuilt from the model on every save.
func regeneratedPartKind(name string) bool {
	return name == "/ppt/presentation.xml" ||
		strings.HasPrefix(name, "/ppt/slideLayouts/slideLayout") ||
		strings.HasPrefix(name, "/ppt/slideMasters/slideMaster")
}

// TestEditedLayoutRegeneratesInsteadOfReplayingSource pins that the source
// fallback is a fidelity shortcut, not a cache: once the model says something
// the source did not, the regenerated bytes must win.
func TestEditedLayoutRegeneratesInsteadOfReplayingSource(t *testing.T) {
	deck, _ := prettyPrintedDeck(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.slideLayouts) == 0 {
		t.Fatal("deck has no layouts")
	}
	layout := p.slideLayouts[0]
	layout.layoutXML.MatchingName = "C587 edited"
	layout.layoutXML.MatchingNamePresent = true

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	part := zipParts(t, saved)["/"+strings.TrimPrefix(layout.partName, "/")]
	if part == nil {
		t.Fatalf("layout part %s missing after save", layout.partName)
	}
	if !bytes.Contains(part, []byte(`matchingName="C587 edited"`)) {
		t.Errorf("edited layout replayed stale source bytes: %q", truncate(part))
	}
}

func truncate(b []byte) []byte {
	if len(b) > 400 {
		return b[:400]
	}
	return b
}
