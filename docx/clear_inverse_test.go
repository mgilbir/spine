package docx

import (
	"go/ast"
	"sort"
	"strings"
	"testing"
)

// A Clear* method has to *remove* its element, not set it to a zero value.
// Word does not treat the two the same: an explicit <w:i w:val="false"/> turns
// italics off even when the paragraph style asks for them, while an absent
// <w:i> lets the style through. Both spellings read back as false through the
// in-memory getter, so a Clear that wrote an explicit "off" would satisfy every
// getter-only test in the package while silently changing how the document
// renders.
//
// These tests therefore assert on the serialized part: after a Clear the
// element must not appear at all. They also assert that Clear on an
// already-clear value is a true no-op — it must not fabricate the container
// (an empty <w:rPr/>, <w:tcPr/>, or a settings.xml that did not exist).

// countElem counts opening tags of the named element in a serialized part.
// The character after the name must be a tag delimiter so that counting "w:b"
// does not also count "w:bCs" or "w:bdr", and closing tags ("</w:b>") are not
// counted because the search prefix is "<w:b" without the slash.
func countElem(part, tag string) int {
	n, open := 0, "<"+tag
	for i := 0; ; {
		j := strings.Index(part[i:], open)
		if j < 0 {
			return n
		}
		i += j + len(open)
		if i >= len(part) {
			return n
		}
		switch part[i] {
		case ' ', '/', '>', '\t', '\n', '\r':
			n++
		}
	}
}

func TestCountElem(t *testing.T) {
	const part = `<w:rPr><w:b/><w:bCs w:val="1"/><w:i w:val="false"></w:i><w:bdr/></w:rPr>`
	for _, c := range []struct {
		tag  string
		want int
	}{
		{"w:b", 1}, {"w:bCs", 1}, {"w:i", 1}, {"w:bdr", 1}, {"w:strike", 0}, {"w:rPr", 1},
	} {
		if got := countElem(part, c.tag); got != c.want {
			t.Errorf("countElem(%q) = %d, want %d", c.tag, got, c.want)
		}
	}
}

// clearSubject is one Clear* method and the scaffolding needed to exercise it.
type clearSubject struct {
	// method is the source-level name (RecvType.Method) the completeness guard
	// matches against.
	method string
	// part is the zip entry the element is written to.
	part string
	// elem is the element Clear must remove.
	elem string
	// scaffold builds the host object (a run, a cell, a section) without
	// setting the property under test.
	scaffold func(t *testing.T, d *Document)
	// set writes the property; clear removes it. Both run after scaffold.
	set func(t *testing.T, d *Document)
	// clear returns the method's report when it has one (Document's two note
	// clears report whether the element was present), nil otherwise.
	clear func(t *testing.T, d *Document) *bool
	// stillSet reads the in-memory getter, so the getter and the serialized
	// part are checked to agree.
	stillSet func(t *testing.T, d *Document) bool
}

func firstCell(t *testing.T, d *Document) *TableCell {
	t.Helper()
	tables := d.Tables()
	if len(tables) == 0 {
		t.Fatal("no tables")
	}
	return tables[0].Rows()[0].Cells()[0]
}

// scaffoldRun / scaffoldCell / scaffoldSection are shared by several subjects.
func scaffoldRun(t *testing.T, d *Document)     { t.Helper(); d.AddParagraph().AddRun().SetText("x") }
func scaffoldPara(t *testing.T, d *Document)    { t.Helper(); d.AddParagraph().SetText("x") }
func scaffoldSection(t *testing.T, d *Document) { t.Helper(); d.DefaultSection() }
func scaffoldCell(t *testing.T, d *Document)    { t.Helper(); d.AddTable(2, 2) }

func boolp(b bool) *bool { return &b }

var sampleBorder = &Border{Style: "single", Width: 1, Color: "FF00FF"}

func clearSubjects() []clearSubject {
	return []clearSubject{
		{
			method: "Run.ClearBold", part: "word/document.xml", elem: "w:b",
			scaffold: scaffoldRun,
			set:      func(t *testing.T, d *Document) { firstRun(t, d).SetBold(false) },
			clear:    func(t *testing.T, d *Document) *bool { firstRun(t, d).ClearBold(); return nil },
			stillSet: func(t *testing.T, d *Document) bool { return firstRun(t, d).Bold() },
		},
		{
			method: "Run.ClearItalic", part: "word/document.xml", elem: "w:i",
			scaffold: scaffoldRun,
			set:      func(t *testing.T, d *Document) { firstRun(t, d).SetItalic(false) },
			clear:    func(t *testing.T, d *Document) *bool { firstRun(t, d).ClearItalic(); return nil },
			stillSet: func(t *testing.T, d *Document) bool { return firstRun(t, d).Italic() },
		},
		{
			method: "Run.ClearStrike", part: "word/document.xml", elem: "w:strike",
			scaffold: scaffoldRun,
			set:      func(t *testing.T, d *Document) { firstRun(t, d).SetStrike(false) },
			clear:    func(t *testing.T, d *Document) *bool { firstRun(t, d).ClearStrike(); return nil },
			stillSet: func(t *testing.T, d *Document) bool { return firstRun(t, d).Strike() },
		},
		{
			method: "Run.ClearCaps", part: "word/document.xml", elem: "w:caps",
			scaffold: scaffoldRun,
			set:      func(t *testing.T, d *Document) { firstRun(t, d).SetCaps(false) },
			clear:    func(t *testing.T, d *Document) *bool { firstRun(t, d).ClearCaps(); return nil },
			stillSet: func(t *testing.T, d *Document) bool { return firstRun(t, d).Caps() },
		},
		{
			method: "Run.ClearSmallCaps", part: "word/document.xml", elem: "w:smallCaps",
			scaffold: scaffoldRun,
			set:      func(t *testing.T, d *Document) { firstRun(t, d).SetSmallCaps(false) },
			clear:    func(t *testing.T, d *Document) *bool { firstRun(t, d).ClearSmallCaps(); return nil },
			stillSet: func(t *testing.T, d *Document) bool { return firstRun(t, d).SmallCaps() },
		},
		{
			method: "Paragraph.ClearBorders", part: "word/document.xml", elem: "w:pBdr",
			scaffold: scaffoldPara,
			set: func(t *testing.T, d *Document) {
				d.Paragraphs()[0].SetBorders(ParagraphBorders{Top: sampleBorder, Bottom: sampleBorder})
			},
			clear: func(t *testing.T, d *Document) *bool { d.Paragraphs()[0].ClearBorders(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.Paragraphs()[0].Borders()
				return ok
			},
		},
		{
			method: "Paragraph.ClearShading", part: "word/document.xml", elem: "w:shd",
			scaffold: scaffoldPara,
			set:      func(t *testing.T, d *Document) { d.Paragraphs()[0].SetShading("C0FFEE") },
			clear:    func(t *testing.T, d *Document) *bool { d.Paragraphs()[0].ClearShading(); return nil },
			stillSet: func(t *testing.T, d *Document) bool { return d.Paragraphs()[0].Shading() != "" },
		},
		{
			method: "Paragraph.ClearTabStops", part: "word/document.xml", elem: "w:tabs",
			scaffold: scaffoldPara,
			set: func(t *testing.T, d *Document) {
				d.Paragraphs()[0].AddTabStop(TabStop{Position: 36, Alignment: TabAlignRight})
			},
			clear:    func(t *testing.T, d *Document) *bool { d.Paragraphs()[0].ClearTabStops(); return nil },
			stillSet: func(t *testing.T, d *Document) bool { return len(d.Paragraphs()[0].Tabs()) > 0 },
		},
		{
			method: "Section.ClearPageNumbering", part: "word/document.xml", elem: "w:pgNumType",
			scaffold: scaffoldSection,
			set: func(t *testing.T, d *Document) {
				start := 7
				d.DefaultSection().SetPageNumbering(PageNumbering{Format: PageNumberLowerRoman, Start: &start})
			},
			clear: func(t *testing.T, d *Document) *bool { d.DefaultSection().ClearPageNumbering(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.DefaultSection().PageNumbering()
				return ok
			},
		},
		{
			method: "Section.ClearColumns", part: "word/document.xml", elem: "w:cols",
			scaffold: scaffoldSection,
			set:      func(t *testing.T, d *Document) { d.DefaultSection().SetColumns(Columns{Count: 3, Spacing: 18}) },
			clear:    func(t *testing.T, d *Document) *bool { d.DefaultSection().ClearColumns(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.DefaultSection().Columns()
				return ok
			},
		},
		{
			method: "Section.ClearPageBorders", part: "word/document.xml", elem: "w:pgBorders",
			scaffold: scaffoldSection,
			set: func(t *testing.T, d *Document) {
				d.DefaultSection().SetPageBorders(PageBorders{OffsetFrom: "page", Top: sampleBorder})
			},
			clear: func(t *testing.T, d *Document) *bool { d.DefaultSection().ClearPageBorders(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.DefaultSection().PageBorders()
				return ok
			},
		},
		{
			method: "Section.ClearLineNumbering", part: "word/document.xml", elem: "w:lnNumType",
			scaffold: scaffoldSection,
			set: func(t *testing.T, d *Document) {
				d.DefaultSection().SetLineNumbering(LineNumbering{CountBy: 5, Start: 1, Restart: "newPage"})
			},
			clear: func(t *testing.T, d *Document) *bool { d.DefaultSection().ClearLineNumbering(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.DefaultSection().LineNumbering()
				return ok
			},
		},
		{
			method: "Section.ClearPaperSource", part: "word/document.xml", elem: "w:paperSrc",
			scaffold: scaffoldSection,
			set:      func(t *testing.T, d *Document) { d.DefaultSection().SetPaperSource(3, 4) },
			clear:    func(t *testing.T, d *Document) *bool { d.DefaultSection().ClearPaperSource(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, _, ok := d.DefaultSection().PaperSource()
				return ok
			},
		},
		{
			method: "Section.ClearDocumentGrid", part: "word/document.xml", elem: "w:docGrid",
			scaffold: scaffoldSection,
			set: func(t *testing.T, d *Document) {
				d.DefaultSection().SetDocumentGrid(DocumentGrid{Type: "lines", LinePitch: 312, CharSpace: 100})
			},
			clear: func(t *testing.T, d *Document) *bool { d.DefaultSection().ClearDocumentGrid(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.DefaultSection().DocumentGrid()
				return ok
			},
		},
		{
			method: "Section.ClearFootnoteProperties", part: "word/document.xml", elem: "w:footnotePr",
			scaffold: scaffoldSection,
			set: func(t *testing.T, d *Document) {
				d.DefaultSection().SetFootnoteProperties(NoteProperties{Position: "sectEnd", NumberFormat: "lowerRoman"})
			},
			clear: func(t *testing.T, d *Document) *bool { d.DefaultSection().ClearFootnoteProperties(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.DefaultSection().FootnoteProperties()
				return ok
			},
		},
		{
			method: "Section.ClearEndnoteProperties", part: "word/document.xml", elem: "w:endnotePr",
			scaffold: scaffoldSection,
			set: func(t *testing.T, d *Document) {
				d.DefaultSection().SetEndnoteProperties(NoteProperties{Position: "docEnd", NumberFormat: "upperLetter"})
			},
			clear: func(t *testing.T, d *Document) *bool { d.DefaultSection().ClearEndnoteProperties(); return nil },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.DefaultSection().EndnoteProperties()
				return ok
			},
		},
		{
			method: "Document.ClearFootnoteProperties", part: "word/settings.xml", elem: "w:footnotePr",
			scaffold: scaffoldPara,
			set: func(t *testing.T, d *Document) {
				d.SetFootnoteProperties(NoteProperties{Position: "beneathText", NumberFormat: "chicago"})
			},
			clear: func(t *testing.T, d *Document) *bool { return boolp(d.ClearFootnoteProperties()) },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.FootnoteProperties()
				return ok
			},
		},
		{
			method: "Document.ClearEndnoteProperties", part: "word/settings.xml", elem: "w:endnotePr",
			scaffold: scaffoldPara,
			set: func(t *testing.T, d *Document) {
				d.SetEndnoteProperties(NoteProperties{Position: "sectEnd", NumberFormat: "decimal"})
			},
			clear: func(t *testing.T, d *Document) *bool { return boolp(d.ClearEndnoteProperties()) },
			stillSet: func(t *testing.T, d *Document) bool {
				_, ok := d.EndnoteProperties()
				return ok
			},
		},
		{
			method: "TableCell.ClearVerticalMerge", part: "word/document.xml", elem: "w:vMerge",
			scaffold: scaffoldCell,
			set:      func(t *testing.T, d *Document) { firstCell(t, d).SetVerticalMerge(VerticalMergeRestart) },
			clear:    func(t *testing.T, d *Document) *bool { firstCell(t, d).ClearVerticalMerge(); return nil },
			stillSet: func(t *testing.T, d *Document) bool { return firstCell(t, d).VerticalMerge() != "" },
		},
	}
}

// buildClearDoc runs the listed stages on a fresh document and returns the
// document plus its serialized parts.
func buildClearDoc(t *testing.T, s clearSubject, stages ...func(t *testing.T, d *Document)) (*Document, map[string]string) {
	t.Helper()
	doc := Create()
	s.scaffold(t, doc)
	for _, stage := range stages {
		stage(t, doc)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, _ := docParts(t, data)
	return doc, parts
}

// TestClearRemovesTheElement is the core assertion: a Clear must delete the
// element from the written part, not write it with a zero value.
func TestClearRemovesTheElement(t *testing.T) {
	for _, s := range clearSubjects() {
		t.Run(s.method, func(t *testing.T) {
			// The fixture must actually write the element, or every assertion
			// below would pass on a Clear that did nothing at all.
			_, setParts := buildClearDoc(t, s, s.set)
			if n := countElem(setParts[s.part], s.elem); n < 1 {
				t.Fatalf("fixture is broken: after the setter, %s contains no <%s>", s.part, s.elem)
			}

			doc, clearedParts := buildClearDoc(t, s, s.set, func(t *testing.T, d *Document) {
				t.Helper()
				if rep := s.clear(t, d); rep != nil && !*rep {
					t.Error("Clear reported the element was absent, but the setter had just written it")
				}
			})
			if n := countElem(clearedParts[s.part], s.elem); n != 0 {
				t.Errorf("after Clear, %s still contains %d <%s> element(s); "+
					"Clear must remove the element, not set it to a zero value "+
					"(an explicit off and an absent element are different documents to Word)",
					s.part, n, s.elem)
			}
			if s.stillSet(t, doc) {
				t.Error("the getter still reports the property as set after Clear")
			}
		})
	}
}

// clearNoOpIgnoredParts are excluded from the byte comparison below.
//
// docProps/core.xml carries dcterms:modified, and the Section clears stamp it
// even when they change nothing: Section.touch() runs unconditionally, before
// the method knows whether the element was there. (The Run clears do not, so
// the two families disagree.) That is the modification-tracking contract this
// package deliberately enforces — TestMutationsFlagTheirPart requires every
// mutator to reach its flag — not a property leaking into the document body,
// so it is out of scope here rather than a failure.
var clearNoOpIgnoredParts = map[string]bool{"docProps/core.xml": true}

// TestClearOnAlreadyClearIsANoOp pins the other half of the contract: clearing
// a property that was never set must not fabricate anything — no empty <w:rPr/>
// or <w:tcPr/>, and no settings.xml conjured into existence. The comparison is
// over every written part (bar the modification stamp), so a container created
// as a side effect fails even when the element under test is correctly absent.
func TestClearOnAlreadyClearIsANoOp(t *testing.T) {
	for _, s := range clearSubjects() {
		t.Run(s.method, func(t *testing.T) {
			_, bare := buildClearDoc(t, s)
			_, cleared := buildClearDoc(t, s, func(t *testing.T, d *Document) {
				t.Helper()
				if rep := s.clear(t, d); rep != nil && *rep {
					t.Error("Clear reported the element was present on a document that never set it")
				}
			})
			if n := countElem(cleared[s.part], s.elem); n != 0 {
				t.Errorf("Clear on an unset property wrote %d <%s> into %s", n, s.elem, s.part)
			}
			for name, want := range bare {
				if clearNoOpIgnoredParts[name] {
					continue
				}
				got, ok := cleared[name]
				if !ok {
					t.Errorf("Clear on an unset property dropped part %s", name)
					continue
				}
				if got != want {
					t.Errorf("Clear on an unset property changed %s:\n bare:    %s\n cleared: %s", name, want, got)
				}
			}
			for name := range cleared {
				if _, ok := bare[name]; !ok {
					t.Errorf("Clear on an unset property fabricated part %s", name)
				}
			}
		})
	}
}

// clearNotPropertyRemoval names the Clear* methods that are not
// property-removal inverses and so are out of scope for the two tests above.
// Each entry has to say why, and the guard fails when one stops existing, so
// the list cannot quietly become a place to park an untested method.
var clearNotPropertyRemoval = map[string]string{
	"Paragraph.Clear": "empties the paragraph's content (runs), not a single formatting property",
	"Run.Clear":       "empties the run's content (text/breaks), not a single formatting property",
}

// TestClearSubjectsAreComplete derives the roster of Clear* methods from the
// package source, so a Clear added tomorrow fails this test until it is either
// covered above or explicitly declared out of scope.
func TestClearSubjectsAreComplete(t *testing.T) {
	covered := map[string]bool{}
	for _, s := range clearSubjects() {
		if covered[s.method] {
			t.Errorf("duplicate subject %s", s.method)
		}
		covered[s.method] = true
	}

	files, _ := parseGoDir(t, ".")
	declared := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
				continue
			}
			if !strings.HasPrefix(fd.Name.Name, "Clear") || !fd.Name.IsExported() {
				continue
			}
			declared[baseTypeName(fd.Recv.List[0].Type)+"."+fd.Name.Name] = true
		}
	}
	if len(declared) == 0 {
		t.Fatal("found no Clear* methods in the package — the source scan is broken")
	}

	var missing []string
	for name := range declared {
		if covered[name] || clearNotPropertyRemoval[name] != "" {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	for _, name := range missing {
		t.Errorf("%s is a Clear* method with no clearSubject entry: add one, or declare it out of scope in clearNotPropertyRemoval with a reason", name)
	}

	for name := range covered {
		if !declared[name] {
			t.Errorf("clearSubjects names %q, which no longer exists — drop the entry", name)
		}
	}
	for name := range clearNotPropertyRemoval {
		if !declared[name] {
			t.Errorf("clearNotPropertyRemoval names %q, which no longer exists — drop the entry", name)
		}
	}
}
