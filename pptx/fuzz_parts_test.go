package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzbound"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/opc"
)

// This file holds the *targeted* pptx fuzzers: each one rewrites exactly one
// part inside an otherwise-valid package, so the PresentationML parser for that
// part sees hostile bytes directly. FuzzOpenPptx (fuzz_test.go) mutates the raw
// archive instead, and nearly every mutation there breaks the zip container
// before any part parser is reached.
//
// The oracles are deliberately not "did not panic". This library's defects are
// wrong output, not crashes, so every target asserts at least one of:
//
//   - honest errors: an opener returns a presentation or an error, never both
//     and never neither;
//   - our own output re-opens and re-saves to the same bytes (one round trip is
//     allowed to sanitize; a value that keeps moving is a parser/serializer
//     disagreement);
//   - the emitted package satisfies the structural invariants C363 broke —
//     every p:sldId resolves to a slide relationship of the right type, no id
//     collision is introduced, and the referenced part is present;
//   - a value read before a save is still there after it (the C598 shape: a
//     marshaler that replays only what the source happened to write).
//
// Three defects these targets found are recorded as skipped reproducers at the
// end of the file rather than fixed here; the carve-outs that keep the oracles
// green each name the reproducer they defer to.

// --- part names the targeted fuzzers rewrite ---

const (
	fuzzPartPresentation   = "ppt/presentation.xml"
	fuzzPartPresRels       = "ppt/_rels/presentation.xml.rels"
	fuzzPartAuthors        = "ppt/authors.xml"
	fuzzPartModernComment  = "ppt/comments/modernComment1.xml"
	fuzzPartLegacyComments = "ppt/comments/comment1.xml"
	fuzzPartLegacyAuthors  = "ppt/commentAuthors.xml"
	fuzzPartNotesSlide     = "ppt/notesSlides/notesSlide1.xml"
	fuzzPartLayout1        = "ppt/slideLayouts/slideLayout1.xml"
	fuzzPartMaster1        = "ppt/slideMasters/slideMaster1.xml"
	fuzzPartTableStyles    = "ppt/tableStyles.xml"
	fuzzPartPresProps      = "ppt/presProps.xml"
	fuzzPartViewProps      = "ppt/viewProps.xml"
	fuzzPartTheme          = "ppt/theme/theme1.xml"
	fuzzPartSlide1Rels     = "ppt/slides/_rels/slide1.xml.rels"
	fuzzPartContentTypes   = "[Content_Types].xml"
)

// --- fixture ---

// buildFuzzDeck builds the package every targeted pptx fuzzer rewrites one part
// of.
//
// It is built through the public API precisely so that each part it claims to
// carry is one the library really writes: AddComment produces the modern
// threaded comment part *and* ppt/authors.xml, SetNotes produces the notes
// slide. Legacy comments have no writer API at all, so they are grafted on
// afterwards as literal parts. TestFuzzDeckCarriesEveryFuzzedPart pins that the
// result actually contains all of them — a fuzzer that rewrites a part its
// fixture does not have mutates nothing and proves nothing.
func buildFuzzDeck(tb testing.TB) []byte {
	tb.Helper()
	p := Create()

	s1 := p.AddSlide()
	s1.AddTextBox().TextFrame().SetText("first slide")
	s1.SetNotes("speaker notes for slide one")
	c := s1.AddComment("Alice Author", "the original comment")
	if r := c.Reply("Bob Reader", "a threaded reply"); r == nil {
		tb.Fatal("building fuzz deck: Reply returned nil, so the fixture has no threaded reply")
	}
	c.SetResolved(true)

	s2 := p.AddSlide()
	s2.AddTextBox().TextFrame().SetText("second slide")

	out, err := p.SaveBytes()
	if err != nil {
		tb.Fatalf("building fuzz deck: %v", err)
	}
	return graftLegacyComments(tb, out)
}

// legacyCommentsXML and legacyCommentAuthorsXML are the pre-2018 comment
// mechanism, which no public API writes. They are grafted into the fixture so
// FuzzPptxLegacyCommentsXML has something to mutate.
const legacyCommentsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<p:cmLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
	`<p:cm authorId="1" dt="2024-01-02T03:04:05.678" idx="1"><p:pos x="1234" y="5678"/><p:text>a legacy comment</p:text></p:cm>` +
	`<p:cm authorId="2" dt="2024-01-02T03:04:06.000" idx="2"><p:pos x="10" y="20"/><p:text>a second legacy comment</p:text></p:cm>` +
	`</p:cmLst>`

const legacyCommentAuthorsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<p:cmAuthorLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
	`<p:cmAuthor id="1" name="Carol Legacy" initials="CL" lastIdx="1" clrIdx="0"/>` +
	`<p:cmAuthor id="2" name="Dave Legacy" initials="DL" lastIdx="1" clrIdx="1"/>` +
	`</p:cmAuthorLst>`

// graftLegacyComments adds a legacy comments part and its author list to pkg,
// wiring the slide and presentation relationships and the content-type
// overrides they need.
func graftLegacyComments(tb testing.TB, pkg []byte) []byte {
	tb.Helper()
	slideRels := string(fuzzseed.ZipEntry(pkg, fuzzPartSlide1Rels))
	presRels := string(fuzzseed.ZipEntry(pkg, fuzzPartPresRels))
	types := string(fuzzseed.ZipEntry(pkg, fuzzPartContentTypes))
	if slideRels == "" || presRels == "" || types == "" {
		tb.Fatal("grafting legacy comments: the base package is missing its relationship or content-type parts")
	}

	slideRels = insertBefore(tb, slideRels, "</Relationships>",
		`<Relationship Id="rId900" Type="`+opc.RelTypeComments+`" Target="../comments/comment1.xml"/>`)
	presRels = insertBefore(tb, presRels, "</Relationships>",
		`<Relationship Id="rId901" Type="`+relTypeCommentAuthors+`" Target="commentAuthors.xml"/>`)
	types = insertBefore(tb, types, "</Types>",
		`<Override PartName="/ppt/comments/comment1.xml" ContentType="`+opc.ContentTypePresentationComments+`"/>`+
			`<Override PartName="/ppt/commentAuthors.xml" ContentType="`+opc.ContentTypePresentationCommentAuthors+`"/>`)

	out := fuzzseed.EditZip(pkg, [][2]string{
		{fuzzPartSlide1Rels, slideRels},
		{fuzzPartPresRels, presRels},
		{fuzzPartContentTypes, types},
		{fuzzPartLegacyComments, legacyCommentsXML},
		{fuzzPartLegacyAuthors, legacyCommentAuthorsXML},
	})
	if out == nil {
		tb.Fatal("grafting legacy comments: the base package is not a readable zip")
	}
	return out
}

func insertBefore(tb testing.TB, doc, marker, insert string) string {
	tb.Helper()
	i := strings.LastIndex(doc, marker)
	if i < 0 {
		tb.Fatalf("building fuzz fixture: %q not found in part", marker)
	}
	return doc[:i] + insert + doc[i:]
}

// fuzzedParts is every part a targeted fuzzer in this file rewrites. It is the
// checklist TestFuzzDeckCarriesEveryFuzzedPart enforces.
var fuzzedParts = []string{
	fuzzPartPresentation,
	fuzzPartAuthors,
	fuzzPartModernComment,
	fuzzPartLegacyComments,
	fuzzPartLegacyAuthors,
	fuzzPartNotesSlide,
	fuzzPartLayout1,
	fuzzPartMaster1,
	fuzzPartTableStyles,
	fuzzPartPresProps,
	fuzzPartViewProps,
	fuzzPartTheme,
}

// fuzzSeedPart returns the fixture's current bytes for a part, failing when the
// part is absent. Seeding a targeted fuzzer from the real part is what makes
// the corpus start inside the grammar instead of outside it.
func fuzzSeedPart(tb testing.TB, pkg []byte, name string) []byte {
	tb.Helper()
	data := fuzzseed.ZipEntry(pkg, name)
	if data == nil {
		tb.Fatalf("fuzz fixture has no %s", name)
	}
	return data
}

// --- oracles ---

// pptxOpenHonest opens pkg and asserts the opener's contract: exactly one of a
// presentation and an error. It returns nil when the package was rejected.
func pptxOpenHonest(t *testing.T, pkg []byte) *Presentation {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	switch {
	case err != nil && p != nil:
		t.Fatalf("OpenReader returned both a presentation and an error: %v", err)
	case err == nil && p == nil:
		t.Fatal("OpenReader returned a nil presentation and a nil error")
	}
	return p
}

// pptxSaveHonest saves p and asserts that a failed save yields no bytes. It
// returns nil when the save was refused (the pre-save validation gate rejecting
// a structurally broken deck is a legitimate outcome, not a finding).
func pptxSaveHonest(t *testing.T, p *Presentation) []byte {
	t.Helper()
	out, err := p.SaveBytes()
	if err != nil {
		if out != nil {
			t.Fatalf("SaveBytes returned %d bytes alongside an error: %v", len(out), err)
		}
		return nil
	}
	if out == nil {
		t.Fatal("SaveBytes returned no bytes and no error")
	}
	return out
}

// pptxRoundTrip opens pkg, saves it, and asserts the two properties that hold
// of anything this library writes, whatever it was fed:
//
//   - the output re-opens. Emitting a package we cannot read back is a defect
//     regardless of how malformed the input was;
//   - the output is a fixed point. The first save may sanitize — dropping a
//     dangling reference, canonicalizing an id list — but a second pass over
//     our own output must reproduce it byte for byte. A value that keeps moving
//     means the parser and the serializer disagree about what the file says.
//
// It returns the first save's bytes (nil when the package was rejected at open
// or refused at save) so callers can layer further checks on them.
func pptxRoundTrip(t *testing.T, pkg []byte) []byte {
	t.Helper()
	p1 := pptxOpenHonest(t, pkg)
	if p1 == nil {
		return nil
	}
	defer func() { _ = p1.Close() }()

	if duplicateSlideRelID(p1) != "" {
		// Known unfixed defect: see TestKnownDefectDuplicateSldIDRelationship,
		// which holds the reproducer this fuzzer found. A deck whose sldIdLst
		// binds one relationship id to two slides grows by one orphan slide part
		// on every save, forever, so it can never be a fixed point. The carve-out
		// is stated as a property of the model rather than folded into the oracle
		// so that deleting it — once the defect is fixed — restores the check
		// with nothing else to unpick.
		return nil
	}

	out1 := pptxSaveHonest(t, p1)
	if out1 == nil {
		return nil
	}

	p2, err := OpenReader(bytes.NewReader(out1), int64(len(out1)))
	if err != nil {
		t.Fatalf("a package this library wrote does not re-open: %v", err)
	}
	defer func() { _ = p2.Close() }()

	out2, err := p2.SaveBytes()
	if err != nil {
		t.Fatalf("re-saving a package this library wrote failed: %v", err)
	}
	if !bytes.Equal(out1, out2) {
		t.Fatalf("save is not a fixed point: re-saving our own output changed it (%d bytes then %d bytes)\nfirst difference: %s",
			len(out1), len(out2), describeZipDiff(out1, out2))
	}
	return out1
}

// duplicateSlideRelID returns a presentation relationship id that more than one
// slide in the deck is bound to, or "" when every slide has its own. Two slides
// sharing one r:id is the C363 shape reached from a plain file read rather than
// from a merge.
func duplicateSlideRelID(p *Presentation) string {
	seen := make(map[string]bool, len(p.slides))
	for _, s := range p.slides {
		if s == nil || s.relID == "" {
			continue
		}
		if seen[s.relID] {
			return s.relID
		}
		seen[s.relID] = true
	}
	return ""
}

// describeZipDiff names the first entry that differs between two packages, for
// a failure message that points at a part instead of at a byte offset.
func describeZipDiff(a, b []byte) string {
	ea, eb := zipEntries(a), zipEntries(b)
	names := make([]string, 0, len(ea))
	for name := range ea {
		names = append(names, name)
	}
	for name := range eb {
		if _, ok := ea[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		x, okA := ea[name]
		y, okB := eb[name]
		switch {
		case !okA:
			return fmt.Sprintf("%s only in the second save", name)
		case !okB:
			return fmt.Sprintf("%s only in the first save", name)
		case !bytes.Equal(x, y):
			return fmt.Sprintf("%s (%d bytes then %d bytes)", name, len(x), len(y))
		}
	}
	return "no entry differs (container framing only)"
}

func zipEntries(pkg []byte) map[string][]byte {
	out := map[string][]byte{}
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return out
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		_ = rc.Close()
		out[f.Name] = buf.Bytes()
	}
	return out
}

// idRef is one entry of an id list in an emitted part: its numeric id and the
// relationship id it binds to.
type idRef struct{ ID, RID string }

// scanIDRefs returns every entry of the named id-list element in an emitted
// part, read straight from the bytes with a token scan.
//
// The scan is hand-rolled rather than decoded through struct tags because
// `xml:"id,attr"` matches on local name alone: it happily binds r:id to the
// numeric id field, so a checker written that way compares relationship ids to
// each other and silently passes a deck whose slide ids all collide. That is
// the trap this whole file exists to avoid — a check that matches nothing looks
// exactly like a check that holds. Attributes are therefore distinguished by
// namespace URI, which is the only thing that actually tells them apart.
func scanIDRefs(data []byte, elem string) ([]idRef, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out []idRef
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != elem {
			continue
		}
		var ref idRef
		for _, at := range se.Attr {
			if at.Name.Local != "id" {
				continue
			}
			switch at.Name.Space {
			case "":
				ref.ID = at.Value
			case relationshipsNS:
				ref.RID = at.Value
			}
		}
		out = append(out, ref)
	}
}

// relationshipsNS is the namespace r:id attributes live in.
const relationshipsNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// checkEmittedIDLists asserts, against the bytes of a package this library just
// wrote, the invariants C363 violated while Validate reported zero findings:
//
//  1. relationship ids are unique within ppt/_rels/presentation.xml.rels;
//  2. every p:sldId / p:sldMasterId / p:notesMasterId r:id resolves to a
//     relationship of the matching type — a slide id bound to a master's
//     relationship is the exact corruption C363 shipped;
//  3. every such relationship targets a part the package actually contains;
//  4. the save did not *introduce* a duplicate p:sldId id. PowerPoint keys
//     zooms and hyperlinks off those ids, so a collision silently redirects
//     references. A collision the input already carried is left alone: the
//     library preserves the source's ids, and demanding it invent new ones
//     would be a fidelity complaint, not a corruption one. A collision that
//     appears only in the output is the library's own — that is C363, and the
//     one-id-counter tension (T4) behind it.
//
// Because SaveTo runs Validate and refuses to write when it reports an error,
// any violation here is by construction a hole in Validate as well as a corrupt
// output.
func checkEmittedIDLists(t *testing.T, in, pkg []byte) {
	t.Helper()
	entries := zipEntries(pkg)
	presXML, ok := entries[fuzzPartPresentation]
	if !ok {
		t.Fatalf("emitted package has no %s", fuzzPartPresentation)
	}
	sldIDs, err := scanIDRefs(presXML, "sldId")
	if err != nil {
		t.Fatalf("emitted %s does not parse: %v", fuzzPartPresentation, err)
	}
	masterIDs, _ := scanIDRefs(presXML, "sldMasterId")
	notesMasterIDs, _ := scanIDRefs(presXML, "notesMasterId")

	rels, err := opc.UnmarshalRelationships(entries[fuzzPartPresRels])
	if err != nil {
		t.Fatalf("emitted %s does not parse: %v", fuzzPartPresRels, err)
	}
	byID := make(map[string]*opc.Relationship, len(rels))
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if prev, dup := byID[rel.ID]; dup {
			t.Fatalf("emitted %s uses relationship id %q twice (%s and %s)",
				fuzzPartPresRels, rel.ID, prev.Target, rel.Target)
		}
		byID[rel.ID] = rel
	}

	// An entry whose r:id does not resolve to the relationships namespace at all
	// is only a finding when the input did not already have one. A mutation that
	// misspells the xmlns:r URI unbinds every r:id in the part at once, and
	// re-emitting that verbatim is fidelity, not corruption.
	// Computed once, not per entry: an id list is attacker-sized, and rescanning
	// the input package for every entry in it is quadratic in the very input the
	// fuzzer is trying to grow.
	inPres := zipEntries(in)[fuzzPartPresentation]
	emptyPreexisting := map[string]bool{}
	for _, elem := range []string{"sldId", "sldMasterId", "notesMasterId"} {
		emptyPreexisting[elem] = idListHasEmptyRID(inPres, elem)
	}
	emptyOK := func(elem string) bool { return emptyPreexisting[elem] }
	check := func(what, relID, wantType, elem string) {
		if relID == "" {
			if emptyOK(elem) {
				return
			}
			t.Fatalf("emitted %s has a %s with no r:id, and the input's %s entries all had one",
				fuzzPartPresentation, what, elem)
		}
		rel, ok := byID[relID]
		if !ok {
			t.Fatalf("emitted %s: %s references relationship %q, which %s does not declare",
				fuzzPartPresentation, what, relID, fuzzPartPresRels)
		}
		if rel.Type != wantType {
			t.Fatalf("emitted %s: %s references relationship %q, whose type is %q, not %q (target %s)",
				fuzzPartPresentation, what, relID, rel.Type, wantType, rel.Target)
		}
		if rel.TargetMode == opc.TargetModeExternal {
			t.Fatalf("emitted %s: %s references external relationship %q", fuzzPartPresentation, what, relID)
		}
		target := strings.TrimPrefix(opc.ResolvePartName("/"+fuzzPartPresentation, rel.Target), "/")
		if _, ok := entries[target]; !ok {
			t.Fatalf("emitted %s: %s resolves to part %q, which the package does not contain",
				fuzzPartPresentation, what, target)
		}
	}

	alreadyDuplicated := duplicateSlideIDsIn(zipEntries(in)[fuzzPartPresentation])
	seen := map[string]bool{}
	for i, e := range sldIDs {
		check(fmt.Sprintf("sldId %d (id=%s)", i+1, e.ID), e.RID, opc.RelTypeSlide, "sldId")
		if seen[e.ID] && !alreadyDuplicated[e.ID] {
			t.Fatalf("emitted %s: slide id %s appears in sldIdLst more than once, and did not in the input",
				fuzzPartPresentation, e.ID)
		}
		seen[e.ID] = true
	}
	for i, e := range masterIDs {
		check(fmt.Sprintf("sldMasterId %d (id=%s)", i+1, e.ID), e.RID, opc.RelTypeSlideMaster, "sldMasterId")
	}
	for i, e := range notesMasterIDs {
		check(fmt.Sprintf("notesMasterId %d", i+1), e.RID, opc.RelTypeNotesMaster, "notesMasterId")
	}
}

// duplicateSlideIDsIn returns the p:sldId ids that the given presentation.xml
// already binds more than once — the collisions a save is allowed to preserve
// because it did not create them.
//
// A part that does not scan yields the empty set, i.e. "nothing was already
// duplicated". That is the strict answer rather than the permissive one, and it
// is safe: an input whose presentation.xml does not parse does not open, so no
// output exists to check.
func duplicateSlideIDsIn(presXML []byte) map[string]bool {
	refs, err := scanIDRefs(presXML, "sldId")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	dup := map[string]bool{}
	for _, e := range refs {
		if seen[e.ID] {
			dup[e.ID] = true
		}
		seen[e.ID] = true
	}
	return dup
}

// idListHasEmptyRID reports whether the named id-list element in an emitted or
// source part already carries an entry with no resolvable r:id.
func idListHasEmptyRID(partXML []byte, elem string) bool {
	refs, err := scanIDRefs(partXML, elem)
	if err != nil {
		return true
	}
	for _, e := range refs {
		if e.RID == "" {
			return true
		}
	}
	return false
}

// checkEmittedRelTargets asserts that every internal relationship in every
// .rels part of an emitted package targets a part the package actually
// contains.
//
// This is the dangling-reference class stated against the output. Validate
// answers the same question against the model — partExists consults the live
// objects and then the *source* reader, and answers an unconditional true for
// /ppt/presProps.xml, /ppt/viewProps.xml and /ppt/tableStyles.xml — so a part
// the source carries and the writer then declines to write is invisible to it
// by construction (audit tension T-A). Asking the bytes cannot be fooled that
// way.
//
// It is called from the targets where it currently holds. The presentation,
// master and aux-part targets do not call it yet: every one of them can drive
// the library into emitting a relationship to a part it dropped, which is
// recorded as TestKnownDefectDroppedPartKeepsItsRelationship rather than
// silently tolerated here.
func checkEmittedRelTargets(t *testing.T, pkg []byte) {
	t.Helper()
	entries := zipEntries(pkg)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !strings.HasSuffix(name, ".rels") || !strings.Contains(name, "_rels/") {
			continue
		}
		source := relsSourcePart(name)
		rels, err := opc.UnmarshalRelationships(entries[name])
		if err != nil {
			t.Fatalf("emitted %s does not parse: %v", name, err)
		}
		for _, rel := range rels {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := strings.TrimPrefix(opc.ResolvePartName(source, rel.Target), "/")
			if _, ok := entries[target]; !ok {
				t.Fatalf("emitted %s: relationship %s (%s) targets %q, which the package does not contain",
					name, rel.ID, shortRelType(rel.Type), target)
			}
		}
	}
}

// relsSourcePart maps a .rels entry name to the absolute part name its targets
// are relative to: "ppt/_rels/presentation.xml.rels" -> "/ppt/presentation.xml",
// and "_rels/.rels" -> "/".
func relsSourcePart(relsName string) string {
	dir, file, _ := strings.Cut(relsName, "_rels/")
	return "/" + dir + strings.TrimSuffix(file, ".rels")
}

// pptxPartBudget bounds what opening and saving one mutated package may cost.
//
// The floor covers the constants of a whole open-plus-save cycle over the
// ~30 KB fixture — decoder buffers, the layout and master models, the zip
// writer's compression state — measured at roughly 10 MB and given a wide
// margin, because the assertion has to be about amplification and not about
// allocator noise. The per-byte rate is the structural claim that costs scale
// with the part being parsed; an attacker-controlled count or size used as a
// make() argument breaks it by orders of magnitude, which is the shape of the
// worst bug this module has shipped (C360).
var pptxPartBudget = fuzzbound.Budget{
	What:              "pptx targeted part round trip",
	Bytes:             96 << 20,
	BytesPerInputByte: 4096,
	Time:              10 * time.Second,
	TimePerMiB:        20 * time.Second,
}

// fuzzPptxPart is the body every targeted fuzzer shares: substitute data for
// one part of the fixture, then run check on the resulting package under the
// resource budget.
func fuzzPptxPart(t *testing.T, pkg []byte, part string, data []byte, check func(*testing.T, []byte)) {
	t.Helper()
	if fuzzbound.Tripped() {
		t.Skip("a resource budget was already exceeded in this process")
	}
	wrapped := fuzzseed.ReplaceZipEntry(pkg, part, data)
	if wrapped == nil {
		t.Skip("fuzz fixture unreadable")
	}
	pptxPartBudget.Check(t, len(wrapped), func() { check(t, wrapped) })
}

// --- targets ---

// FuzzPptxPresentationXML rewrites ppt/presentation.xml, the part that binds
// slide ids to relationship ids. Malformed or self-contradictory sldIdLst and
// sldMasterIdLst content is the input class behind C363, where merge emitted
// p:sldId entries pointing at a notesMaster and a slideMaster with one rId
// bound twice and Validate reported the deck clean.
//
// Oracle: honest errors, the round trip is a fixed point, and — the part that
// would have caught C363 — the emitted id lists resolve to relationships of the
// right type, targeting parts that exist, with unique ids.
func FuzzPptxPresentationXML(f *testing.F) {
	pkg := buildFuzzDeck(f)
	orig := fuzzSeedPart(f, pkg, fuzzPartPresentation)

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("<p:presentation"))
	f.Add(orig[:len(orig)/2])
	const presOpen = `<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`
	// The C363 shape itself: a slide id bound to the master's relationship.
	f.Add([]byte(presOpen + `<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId4"/></p:sldMasterIdLst><p:sldIdLst><p:sldId id="256" r:id="rId4"/></p:sldIdLst></p:presentation>`))
	// One relationship id bound to two slides.
	f.Add([]byte(presOpen + `<p:sldIdLst><p:sldId id="256" r:id="rId5"/><p:sldId id="257" r:id="rId5"/></p:sldIdLst></p:presentation>`))
	// Duplicate slide ids behind distinct relationships.
	f.Add([]byte(presOpen + `<p:sldIdLst><p:sldId id="256" r:id="rId5"/><p:sldId id="256" r:id="rId6"/></p:sldIdLst></p:presentation>`))
	// A slide id pointing at nothing at all.
	f.Add([]byte(presOpen + `<p:sldIdLst><p:sldId id="256" r:id="rIdNope"/></p:sldIdLst></p:presentation>`))
	// Reserved and out-of-range slide ids (ST_SlideId is 256..2147483647).
	f.Add([]byte(presOpen + `<p:sldIdLst><p:sldId id="0" r:id="rId5"/><p:sldId id="4294967295" r:id="rId6"/></p:sldIdLst></p:presentation>`))
	// A custom show referring to slides that are not in the deck.
	f.Add([]byte(presOpen + `<p:sldIdLst><p:sldId id="256" r:id="rId5"/></p:sldIdLst><p:custShowLst><p:custShow name="show" id="0"><p:sldLst><p:sld r:id="rId5"/><p:sld r:id="rId4"/><p:sld r:id=""/></p:sldLst></p:custShow></p:custShowLst></p:presentation>`))
	// A very long id list, to see whether cost tracks the input. The ids are all
	// distinct and all dangling, which drives the parse-then-skip path at scale
	// without tripping the duplicate-r:id carve-out in pptxRoundTrip.
	var many strings.Builder
	many.WriteString(presOpen + `<p:sldIdLst>`)
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&many, `<p:sldId id="%d" r:id="rIdX%d"/>`, 256+i, i)
	}
	many.WriteString(`</p:sldIdLst></p:presentation>`)
	f.Add([]byte(many.String()))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzPptxPart(t, pkg, fuzzPartPresentation, data, func(t *testing.T, wrapped []byte) {
			if out := pptxRoundTrip(t, wrapped); out != nil {
				checkEmittedIDLists(t, wrapped, out)
			}
		})
	})
}

// commentFacts is everything the public comment API exposes about one deck,
// flattened so two reads can be compared.
type commentFacts []string

func readCommentFacts(p *Presentation) commentFacts {
	var out commentFacts
	for i, s := range p.Slides() {
		for _, c := range s.Comments() {
			out = append(out, describeComment(i, c, ""))
			for _, r := range c.Replies() {
				out = append(out, describeComment(i, r, "reply:"))
			}
		}
	}
	return out
}

func describeComment(slide int, c *Comment, prefix string) string {
	x, y := c.Position()
	anchor, anchored := c.AnchorShapeID()
	return fmt.Sprintf("%sslide=%d kind=%d id=%q author=%q text=%q date=%q resolved=%t pos=%d,%d anchor=%d/%t",
		prefix, slide, c.kind, c.ID(), c.Author(), c.Text(), c.Date().Format(time.RFC3339Nano), c.Resolved(), x, y, anchor, anchored)
}

// isModern reports whether a comment uses the 2018 threaded mechanism. The
// unexported field is read rather than inferred from the id shape because the
// fuzzer controls the id: a legacy comment whose idx renders as "{...}" would
// otherwise be misclassified and the resolved assertion below would invert.
func isModern(c *Comment) bool { return c.kind == commentModern }

// checkCommentsSurviveARewrite reads every comment, forces the comment part to
// be re-marshaled by editing the thread through the API, and asserts that
// everything the API reported before the edit still reads back the same way
// after a save and a re-open (bar the one field the edit changes).
//
// The forced edit is the point. Comment parts are otherwise preserved as raw
// bytes and re-emitted verbatim, so a round trip that never touches them
// exercises no marshaler at all. C598 was exactly this: a resolve was lost
// because the marshaler replayed only the attributes the source had written, a
// defect invisible to any test that did not both edit and reparse.
func checkCommentsSurviveARewrite(t *testing.T, pkg []byte) {
	t.Helper()
	p := pptxOpenHonest(t, pkg)
	if p == nil {
		return
	}
	defer func() { _ = p.Close() }()

	before := readCommentFacts(p)

	// Flip the resolved state of every thread, which routes each modern one
	// through ModernCommentPart.Marshal. wantResolved records what each thread's
	// flag must read as afterwards: flipped for a modern comment, unchanged for
	// a legacy one, whose SetResolved is a documented no-op.
	var wantResolved []bool
	for _, s := range p.Slides() {
		for _, c := range s.Comments() {
			was := c.Resolved()
			c.SetResolved(!was)
			if isModern(c) {
				wantResolved = append(wantResolved, !was)
			} else {
				wantResolved = append(wantResolved, was)
			}
			for range c.Replies() {
				wantResolved = append(wantResolved, wantResolved[len(wantResolved)-1])
			}
		}
	}

	out := pptxSaveHonest(t, p)
	if out == nil {
		return
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("a package this library wrote does not re-open: %v", err)
	}
	defer func() { _ = p2.Close() }()

	after := readCommentFacts(p2)
	if len(before) != len(after) {
		t.Fatalf("comment count changed across a save: %d before, %d after\nbefore: %v\nafter:  %v",
			len(before), len(after), before, after)
	}
	for i := range before {
		// Every field except the one the edit deliberately moved must be
		// unchanged: that is the C598 shape, where re-marshaling a thread
		// dropped data the source had not written explicitly.
		if normalizeResolved(before[i]) != normalizeResolved(after[i]) {
			t.Fatalf("comment %d changed across an edit and a save:\nbefore: %s\nafter:  %s", i, before[i], after[i])
		}
	}
	// And the field the edit *did* move must have moved, and stuck. C598 itself
	// was a lost resolve, so a check that only compared the untouched fields
	// would have watched the bug go past.
	got := resolvedFlags(p2)
	if len(got) != len(wantResolved) {
		t.Fatalf("comment count changed across a save: %d before, %d after", len(wantResolved), len(got))
	}
	for i := range got {
		if got[i] != wantResolved[i] {
			t.Fatalf("comment %d reads back resolved=%t after SetResolved(%t): the edit did not survive the save\nbefore: %s\nafter:  %s",
				i, got[i], wantResolved[i], before[i], after[i])
		}
	}
}

// resolvedFlags returns the resolved flag of every comment and reply in the
// deck, in the same order readCommentFacts walks them.
func resolvedFlags(p *Presentation) []bool {
	var out []bool
	for _, s := range p.Slides() {
		for _, c := range s.Comments() {
			out = append(out, c.Resolved())
			for _, r := range c.Replies() {
				out = append(out, r.Resolved())
			}
		}
	}
	return out
}

func normalizeResolved(s string) string {
	s = strings.Replace(s, "resolved=true", "resolved=?", 1)
	return strings.Replace(s, "resolved=false", "resolved=?", 1)
}

// FuzzPptxModernCommentXML rewrites the modern (2018 threaded) comment part.
// The part is dense with optional attributes and with children this library
// keeps verbatim rather than models, and its marshaler is the one C598 broke.
//
// Oracle: the round trip is a fixed point, and every field the comment API
// reports survives an edit that forces the part to be re-marshaled.
func FuzzPptxModernCommentXML(f *testing.F) {
	pkg := buildFuzzDeck(f)
	orig := fuzzSeedPart(f, pkg, fuzzPartModernComment)

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("<p188:cmLst"))
	f.Add(orig[:len(orig)/2])
	const cmOpen = `<p188:cmLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main">`
	// A thread with none of the optional attributes and no body.
	f.Add([]byte(cmOpen + `<p188:cm id="{00000000-0000-0000-0000-000000000000}"/></p188:cmLst>`))
	// A thread carrying attributes this library does not model, an empty reply
	// list, and an extLst — all of which must survive a rewrite.
	f.Add([]byte(cmOpen + `<p188:cm id="{1}" authorId="{2}" created="2024-01-01T00:00:00.000" status="resolved" startDate="2024-01-01" dueDate="2024-02-01" assignedTo="{3}" complete="50000" title="task"><p188:replyLst/><p188:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>body</a:t></a:r></a:p></p188:txBody><p188:extLst><p188:ext uri="{X}"><foo:bar xmlns:foo="urn:foo"/></p188:ext></p188:extLst></p188:cm></p188:cmLst>`))
	// status set to something other than "resolved".
	f.Add([]byte(cmOpen + `<p188:cm id="{1}" authorId="{2}" status="active"/></p188:cmLst>`))
	// Two top-level comments in one part: the model holds exactly one.
	f.Add([]byte(cmOpen + `<p188:cm id="{1}" authorId="{2}"/><p188:cm id="{1}" authorId="{2}"/></p188:cmLst>`))
	// Replies whose ids collide with the thread's, and a position marker.
	f.Add([]byte(cmOpen + `<p188:cm id="{1}" authorId="{2}"><p188:pos x="-9223372036854775808" y="9223372036854775807"/><p188:replyLst><p188:reply id="{1}" authorId="{2}"/><p188:reply id="{1}"/></p188:replyLst></p188:cm></p188:cmLst>`))
	// Deep reply nesting, to see whether cost tracks the input.
	var deep strings.Builder
	deep.WriteString(cmOpen + `<p188:cm id="{1}" authorId="{2}"><p188:replyLst>`)
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&deep, `<p188:reply id="{r%d}" authorId="{2}" created="2024-01-01T00:00:00.000"/>`, i)
	}
	deep.WriteString(`</p188:replyLst></p188:cm></p188:cmLst>`)
	f.Add([]byte(deep.String()))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzPptxPart(t, pkg, fuzzPartModernComment, data, func(t *testing.T, wrapped []byte) {
			if out := pptxRoundTrip(t, wrapped); out != nil {
				checkEmittedRelTargets(t, out)
			}
			checkCommentsSurviveARewrite(t, wrapped)
		})
	})
}

// FuzzPptxAuthorsXML rewrites ppt/authors.xml, the shared modern author list a
// threaded comment's authorId resolves through. A comment whose author cannot
// be resolved shows as "Unknown" in PowerPoint, so silently losing or reordering
// authors is data loss with no error attached.
//
// Oracle: the round trip is a fixed point, comment fields survive a rewrite,
// and adding a comment for an author already in the list must reuse that
// author's identity rather than mint a second entry with the same name.
func FuzzPptxAuthorsXML(f *testing.F) {
	pkg := buildFuzzDeck(f)
	orig := fuzzSeedPart(f, pkg, fuzzPartAuthors)

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("<p188:authorLst"))
	f.Add(orig[:len(orig)/2])
	const authOpen = `<p188:authorLst xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main">`
	f.Add([]byte(authOpen + `</p188:authorLst>`))
	// An author with none of the optional attributes: nothing may be invented
	// for it on the way back out.
	f.Add([]byte(authOpen + `<p188:author id="{1}" name="Alice Author"/></p188:authorLst>`))
	// Two authors sharing an id, and two sharing a name.
	f.Add([]byte(authOpen + `<p188:author id="{1}" name="One"/><p188:author id="{1}" name="Two"/><p188:author id="{3}" name="One"/></p188:authorLst>`))
	// An author carrying unmodeled attributes and inner content.
	f.Add([]byte(authOpen + `<p188:author id="{1}" name="X" initials="" userId="" providerId="AD" unknownAttr="keep"><p188:extLst><p188:ext uri="{U}"/></p188:extLst></p188:author></p188:authorLst>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzPptxPart(t, pkg, fuzzPartAuthors, data, func(t *testing.T, wrapped []byte) {
			if out := pptxRoundTrip(t, wrapped); out != nil {
				checkEmittedRelTargets(t, out)
			}
			checkCommentsSurviveARewrite(t, wrapped)
			checkAuthorReuse(t, wrapped)
		})
	})
}

// checkAuthorReuse asserts that adding a comment authored by a name already in
// the author list resolves back to that same name. The author list is
// deduplicated by display name, so a parse that silently drops entries would
// show up here as a new comment whose author does not read back.
func checkAuthorReuse(t *testing.T, pkg []byte) {
	t.Helper()
	p := pptxOpenHonest(t, pkg)
	if p == nil {
		return
	}
	defer func() { _ = p.Close() }()
	slides := p.Slides()
	if len(slides) == 0 {
		return
	}
	const author = "Alice Author"
	added := slides[0].AddComment(author, "added by the fuzz oracle")
	if added == nil {
		t.Fatal("AddComment returned nil on a deck with at least one slide")
	}
	wantID := added.ID()

	out := pptxSaveHonest(t, p)
	if out == nil {
		return
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("a package this library wrote does not re-open: %v", err)
	}
	defer func() { _ = p2.Close() }()

	for _, s := range p2.Slides() {
		for _, c := range s.Comments() {
			if c.ID() != wantID {
				continue
			}
			if c.Author() != author {
				t.Fatalf("a comment added for author %q reads back with author %q: the author list did not survive the save",
					author, c.Author())
			}
			return
		}
	}
	t.Fatalf("a comment added through the API (id %s) is not present after a save and a re-open", wantID)
}

// FuzzPptxLegacyCommentsXML rewrites the pre-2018 comments part. Both comment
// mechanisms may coexist in one file and the legacy one is what every deck
// authored before 2018 carries, but no public API writes it, so it is reachable
// only by reading — which is exactly why it is worth fuzzing.
//
// Oracle: the round trip is a fixed point, comment fields survive an edit and a
// save, and Validate's author cross-check stays a warning rather than blocking
// a save.
func FuzzPptxLegacyCommentsXML(f *testing.F) {
	pkg := buildFuzzDeck(f)
	orig := fuzzSeedPart(f, pkg, fuzzPartLegacyComments)

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("<p:cmLst"))
	f.Add(orig[:len(orig)/2])
	const cmOpen = `<p:cmLst xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`
	f.Add([]byte(cmOpen + `</p:cmLst>`))
	// A comment whose author id is in no author list.
	f.Add([]byte(cmOpen + `<p:cm authorId="4294967295" idx="1"><p:text>orphan</p:text></p:cm></p:cmLst>`))
	// Positions at the edges of int64, a duplicated index, and a missing text.
	f.Add([]byte(cmOpen + `<p:cm authorId="1" idx="1"><p:pos x="-9223372036854775808" y="9223372036854775807"/></p:cm><p:cm authorId="1" idx="1" dt="not a date"><p:text></p:text></p:cm></p:cmLst>`))
	// Unmodeled attributes and an extLst that must survive.
	f.Add([]byte(cmOpen + `<p:cm authorId="1" idx="1" unknown="keep"><p:text>x</p:text><p:extLst><p:ext uri="{U}"><foo:bar xmlns:foo="urn:foo"/></p:ext></p:extLst></p:cm></p:cmLst>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzPptxPart(t, pkg, fuzzPartLegacyComments, data, func(t *testing.T, wrapped []byte) {
			if out := pptxRoundTrip(t, wrapped); out != nil {
				checkEmittedRelTargets(t, out)
			}
			checkCommentsSurviveARewrite(t, wrapped)
		})
	})
}

// FuzzPptxNotesSlideXML rewrites a notes slide. Speaker notes are a whole
// PresentationML shape tree in their own part, reachable only through
// Slide.Notes / SetNotes, and SetNotes rewrites the part by re-marshaling the
// parsed model — so a parse that loses a shape loses it permanently.
//
// Oracle: the round trip is a fixed point, and the notes text read before a
// SetNotes-driven rewrite of a *different* slide's notes is unchanged, while
// text written through SetNotes reads back verbatim.
func FuzzPptxNotesSlideXML(f *testing.F) {
	pkg := buildFuzzDeck(f)
	orig := fuzzSeedPart(f, pkg, fuzzPartNotesSlide)

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("<p:notes"))
	f.Add(orig[:len(orig)/2])
	const notesOpen = `<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`
	// No shape tree at all.
	f.Add([]byte(notesOpen + `<p:cSld/></p:notes>`))
	// A body placeholder with no text body, and a second body placeholder.
	f.Add([]byte(notesOpen + `<p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name=""/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp><p:sp><p:nvSpPr><p:cNvPr id="2" name=""/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:p><a:r><a:t>second</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:notes>`))
	// Explicit non-default showMasterSp, which must not be dropped as a zero.
	f.Add([]byte(`<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" showMasterSp="0" showMasterPhAnim="0"><p:cSld><p:spTree/></p:cSld></p:notes>`))
	// Text carrying characters that have to be escaped on the way out.
	f.Add([]byte(notesOpen + `<p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name=""/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:p><a:r><a:t>&lt;&amp;&gt;&#xD;&#x9;</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:notes>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzPptxPart(t, pkg, fuzzPartNotesSlide, data, func(t *testing.T, wrapped []byte) {
			if out := pptxRoundTrip(t, wrapped); out != nil {
				checkEmittedRelTargets(t, out)
			}
			checkNotesSurviveARewrite(t, wrapped)
		})
	})
}

// checkNotesSurviveARewrite asserts that reading a slide's notes, rewriting
// them through SetNotes, saving and re-opening yields exactly the text written
// — and that the *other* slides' notes are unaffected.
func checkNotesSurviveARewrite(t *testing.T, pkg []byte) {
	t.Helper()
	p := pptxOpenHonest(t, pkg)
	if p == nil {
		return
	}
	defer func() { _ = p.Close() }()
	slides := p.Slides()
	if len(slides) == 0 {
		return
	}

	if ns, _ := slides[0].loadNotesSlide(); ns == nil && slides[0].notesSlidePartName() != "" {
		// Known unfixed defect: see TestKnownDefectSetNotesOverAnUnparseableNotesPart,
		// which holds the reproducer this fuzzer found. When the slide's existing
		// notes part does not parse, SetNotes appends a *second* notesSlide
		// relationship rather than replacing the first, and Notes() keeps
		// resolving the original — so the text written is unreadable. The
		// carve-out is stated as the exact precondition of that defect so that
		// deleting it, once fixed, restores the check unchanged.
		return
	}

	othersBefore := make([]string, len(slides))
	for i, s := range slides {
		othersBefore[i] = s.Notes()
	}

	// Captured before the rewrite so the attribute check below can tell the
	// library's output apart from what the source handed it.
	notesBefore := p.rawPartData(slides[0].notesSlidePartName())

	const written = "rewritten by the fuzz oracle"
	slides[0].SetNotes(written)

	// The rewritten part must carry its text in DrawingML, and every element
	// name in it must resolve. Both were carved out here while the Builder
	// wrote names in namespaces the source root had not declared; both are now
	// assertions.
	part := slides[0].notesSlidePartName()
	rewritten := p.rawPartData(part)
	if !hasDrawingMLText(rewritten) {
		t.Fatalf("rewritten %s carries no DrawingML text, so its notes cannot be read back:\n%s", part, rewritten)
	}
	if !elementNamespacesResolve(rewritten) {
		t.Fatalf("rewritten %s has an element in an undeclared namespace:\n%s", part, rewritten)
	}
	// Attributes are held to the same standard only when the source's were:
	// the rewrite replays the root's captured attribute list verbatim, so a
	// source that carried an attribute in an undeclared prefix (the fuzzer
	// writes xmlni:p) gets it back. Preserving that is not the same defect as
	// emitting one, and blaming the library for it here would make the check
	// unfalsifiable rather than strict.
	if namespaceWellFormed(notesBefore) && !namespaceWellFormed(rewritten) {
		t.Fatalf("rewriting a namespace-well-formed %s made it ill-formed:\n%s", part, rewritten)
	}

	out := pptxSaveHonest(t, p)
	if out == nil {
		return
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("a package this library wrote does not re-open: %v", err)
	}
	defer func() { _ = p2.Close() }()

	after := p2.Slides()
	if len(after) != len(slides) {
		t.Fatalf("slide count changed across a save: %d before, %d after", len(slides), len(after))
	}
	if got := after[0].Notes(); got != written {
		t.Fatalf("notes written through SetNotes read back as %q, want %q", got, written)
	}
	for i := 1; i < len(after); i++ {
		if got := after[i].Notes(); got != othersBefore[i] {
			t.Fatalf("slide %d's notes changed while slide 1's were rewritten: %q before, %q after",
				i+1, othersBefore[i], got)
		}
	}
}

// hasDrawingMLText reports whether a part contains at least one text element
// that actually resolves to the DrawingML namespace. Note text lives in a:t, so
// a part with none of those carries no readable text however many a:t-looking
// elements it contains under some other binding.
func hasDrawingMLText(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if se, ok := tok.(xml.StartElement); ok &&
			se.Name.Local == "t" && se.Name.Space == xmlb.NSDrawingML {
			return true
		}
	}
}

// elementNamespacesResolve reports whether every element name in an XML
// document resolves to a declared namespace. It is namespaceWellFormed narrowed
// to element names: those are always the library's own to bind, while an
// attribute may have been replayed verbatim from a malformed source.
func elementNamespacesResolve(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if space := se.Name.Space; space != "" && space != "xml" && !strings.Contains(space, "://") {
			return false
		}
	}
}

// namespaceWellFormed reports whether every prefixed element and attribute name
// in an XML document resolves to a declared namespace.
//
// Go's decoder is deliberately lenient here: an undeclared prefix comes back
// with Name.Space set to the literal prefix rather than to a URI, and decoding
// carries on. That leniency is why a part can lose a namespace declaration and
// still round-trip through this library while being rejected by PowerPoint and
// read as empty by the library's own accessors — so the check is for the URI.
func namespaceWellFormed(data []byte) bool {
	resolved := func(space string) bool {
		return space == "" || space == "xml" || space == "xmlns" || strings.Contains(space, "://")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !resolved(se.Name.Space) {
			return false
		}
		for _, at := range se.Attr {
			if !resolved(at.Name.Space) {
				return false
			}
		}
	}
}

// FuzzPptxSlideLayoutXML rewrites a slide layout. Layouts are where placeholder
// geometry and text properties are inherited from, so a layout that parses into
// a subtly different model changes how every slide bound to it renders while
// nothing reports an error.
//
// Oracle: the round trip is a fixed point, the emitted id lists stay coherent,
// and every slide still resolves to a layout after the round trip if it did
// before.
func FuzzPptxSlideLayoutXML(f *testing.F) {
	pkg := buildFuzzDeck(f)
	orig := fuzzSeedPart(f, pkg, fuzzPartLayout1)

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("<p:sldLayout"))
	f.Add(orig[:len(orig)/2])
	const layoutOpen = `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	f.Add([]byte(layoutOpen + `><p:cSld><p:spTree/></p:cSld></p:sldLayout>`))
	// An unknown layout type and an empty name.
	f.Add([]byte(layoutOpen + ` type="notALayoutType" preserve="1"><p:cSld name=""><p:spTree/></p:cSld></p:sldLayout>`))
	// Placeholders with colliding ids and indices.
	f.Add([]byte(layoutOpen + `><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="a"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp><p:sp><p:nvSpPr><p:cNvPr id="2" name="a"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp></p:spTree></p:cSld></p:sldLayout>`))
	// Deep group nesting.
	f.Add([]byte(layoutOpen + `><p:cSld><p:spTree>` + strings.Repeat(`<p:grpSp><p:grpSpPr/>`, 100) + strings.Repeat(`</p:grpSp>`, 100) + `</p:spTree></p:cSld></p:sldLayout>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzPptxPart(t, pkg, fuzzPartLayout1, data, func(t *testing.T, wrapped []byte) {
			if out := pptxRoundTrip(t, wrapped); out != nil {
				checkEmittedIDLists(t, wrapped, out)
				checkEmittedRelTargets(t, out)
			}
			checkLayoutBindingsSurvive(t, wrapped)
		})
	})
}

// checkLayoutBindingsSurvive asserts that a slide which resolved to a layout
// before a save still resolves to one afterwards. Losing the binding silently
// re-parents a slide onto the default inheritance chain.
func checkLayoutBindingsSurvive(t *testing.T, pkg []byte) {
	t.Helper()
	p := pptxOpenHonest(t, pkg)
	if p == nil {
		return
	}
	defer func() { _ = p.Close() }()

	before := make([]bool, 0, len(p.Slides()))
	for _, s := range p.Slides() {
		before = append(before, s.Layout() != nil)
	}

	out := pptxSaveHonest(t, p)
	if out == nil {
		return
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("a package this library wrote does not re-open: %v", err)
	}
	defer func() { _ = p2.Close() }()

	after := p2.Slides()
	if len(after) != len(before) {
		t.Fatalf("slide count changed across a save: %d before, %d after", len(before), len(after))
	}
	for i, had := range before {
		if had && after[i].Layout() == nil {
			t.Fatalf("slide %d resolved to a layout before the save and to none after it", i+1)
		}
	}
}

// FuzzPptxSlideMasterXML rewrites a slide master. A master owns the
// sldLayoutIdLst, whose entries bind layout ids to the master's own
// relationships — the same dangling-reference class as the presentation's slide
// list, one level down, and the one case validateIDListReferences actually
// checks.
//
// Oracle: the round trip is a fixed point, the emitted id lists stay coherent,
// and a master's layout list in the output resolves through the master's own
// relationships.
func FuzzPptxSlideMasterXML(f *testing.F) {
	pkg := buildFuzzDeck(f)
	orig := fuzzSeedPart(f, pkg, fuzzPartMaster1)

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("<p:sldMaster"))
	f.Add(orig[:len(orig)/2])
	const masterOpen = `<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`
	f.Add([]byte(masterOpen + `<p:cSld><p:spTree/></p:cSld></p:sldMaster>`))
	// A layout id list pointing at relationships the master does not have.
	f.Add([]byte(masterOpen + `<p:cSld><p:spTree/></p:cSld><p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rIdNope"/><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`))
	// A colour map with values outside the enumeration.
	f.Add([]byte(masterOpen + `<p:cSld><p:spTree/></p:cSld><p:clrMap bg1="nope" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:sldMaster>`))
	// A long layout id list.
	var many strings.Builder
	many.WriteString(masterOpen + `<p:cSld><p:spTree/></p:cSld><p:sldLayoutIdLst>`)
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&many, `<p:sldLayoutId id="%d" r:id="rId1"/>`, 2147483649+i)
	}
	many.WriteString(`</p:sldLayoutIdLst></p:sldMaster>`)
	f.Add([]byte(many.String()))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzPptxPart(t, pkg, fuzzPartMaster1, data, func(t *testing.T, wrapped []byte) {
			if out := pptxRoundTrip(t, wrapped); out != nil {
				checkEmittedIDLists(t, wrapped, out)
				checkEmittedLayoutIDLists(t, wrapped, out)
			}
		})
	})
}

// checkEmittedLayoutIDLists asserts that every sldLayoutId in every emitted
// master resolves to a slideLayout relationship of that master, targeting a
// part the package contains. This is validateIDListReferences' invariant
// checked against the output rather than against the model that produced it —
// audit tension T-A says those are not the same question.
func checkEmittedLayoutIDLists(t *testing.T, in, pkg []byte) {
	t.Helper()
	entries := zipEntries(pkg)
	inEntries := zipEntries(in)
	for name, data := range entries {
		if !strings.HasPrefix(name, "ppt/slideMasters/") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		layoutIDs, err := scanIDRefs(data, "sldLayoutId")
		if err != nil {
			t.Fatalf("emitted %s does not parse: %v", name, err)
		}
		if len(layoutIDs) == 0 {
			continue
		}
		relsName := "ppt/slideMasters/_rels/" + strings.TrimPrefix(name, "ppt/slideMasters/") + ".rels"
		rels, err := opc.UnmarshalRelationships(entries[relsName])
		if err != nil {
			t.Fatalf("emitted %s does not parse: %v", relsName, err)
		}
		byID := map[string]*opc.Relationship{}
		for _, rel := range rels {
			if rel != nil {
				byID[rel.ID] = rel
			}
		}
		inputHadEmpty := idListHasEmptyRID(inEntries[name], "sldLayoutId")
		for i, e := range layoutIDs {
			if e.RID == "" && inputHadEmpty {
				continue
			}
			rel, ok := byID[e.RID]
			if !ok {
				t.Fatalf("emitted %s: sldLayoutId %d (id=%s) references relationship %q, which %s does not declare",
					name, i+1, e.ID, e.RID, relsName)
			}
			if rel.Type != opc.RelTypeSlideLayout {
				t.Fatalf("emitted %s: sldLayoutId %d references relationship %q, whose type is %q, not a slide layout",
					name, i+1, e.RID, rel.Type)
			}
			target := strings.TrimPrefix(opc.ResolvePartName("/"+name, rel.Target), "/")
			if _, ok := entries[target]; !ok {
				t.Fatalf("emitted %s: sldLayoutId %d resolves to part %q, which the package does not contain",
					name, i+1, target)
			}
		}
	}
}

// fuzzAuxParts are the presentation-level parts with no dedicated target of
// their own: they are small, they carry no references, and their failure mode
// is losing settings rather than corrupting structure. One target covers them
// with a selector byte so the fuzzer can steer between them.
var fuzzAuxParts = []string{
	fuzzPartTableStyles,
	fuzzPartPresProps,
	fuzzPartViewProps,
	fuzzPartTheme,
	fuzzPartLegacyAuthors,
}

// FuzzPptxAuxPartXML rewrites one of the presentation-level parts that carry
// settings rather than structure: the table style list, the presentation and
// view properties, the theme, and the legacy comment author list. Which part is
// rewritten is chosen by a fuzzed selector, so one target covers all five and
// the engine steers between them by coverage.
//
// Oracle: the round trip is a fixed point and the emitted id lists stay
// coherent — a malformed theme must not take the deck's structure with it.
func FuzzPptxAuxPartXML(f *testing.F) {
	pkg := buildFuzzDeck(f)

	for i, name := range fuzzAuxParts {
		orig := fuzzSeedPart(f, pkg, name)
		f.Add(uint8(i), orig)
		f.Add(uint8(i), []byte{})
		f.Add(uint8(i), orig[:len(orig)/2])
	}
	f.Add(uint8(0), []byte(`<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def=""/>`))
	f.Add(uint8(1), []byte(`<p:presentationPr xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:showPr loop="1"><p:present/></p:showPr></p:presentationPr>`))
	f.Add(uint8(2), []byte(`<p:viewPr xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" lastView="notAView"><p:normalViewPr><p:restoredLeft sz="-1" autoAdjust="0"/></p:normalViewPr></p:viewPr>`))
	f.Add(uint8(3), []byte(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name=""><a:themeElements/></a:theme>`))
	f.Add(uint8(4), []byte(`<p:cmAuthorLst xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cmAuthor id="1" name="Carol Legacy" clrIdx="0"/><p:cmAuthor id="1" name="Other"/></p:cmAuthorLst>`))
	f.Add(uint8(200), []byte("selector out of range"))

	f.Fuzz(func(t *testing.T, which uint8, data []byte) {
		part := fuzzAuxParts[int(which)%len(fuzzAuxParts)]
		fuzzPptxPart(t, pkg, part, data, func(t *testing.T, wrapped []byte) {
			if out := pptxRoundTrip(t, wrapped); out != nil {
				checkEmittedIDLists(t, wrapped, out)
			}
		})
	})
}

// --- reproducers for the defects these fuzzers found ---
//
// Both are skipped, not deleted and not weakened into a test of the broken
// behaviour. They are the inputs the targeted fuzzers produced, minimized, kept
// in the tree so the carve-outs above point at something executable and so
// whoever fixes the defect has the reproducer already written. Removing the
// t.Skip is the first step of that fix.

// TestKnownDefectDuplicateSldIDRelationship reproduces the defect found by
// FuzzPptxPresentationXML: ppt/presentation.xml whose sldIdLst binds one
// relationship id to two p:sldId entries.
//
// Opening such a deck yields two Slide objects that share a part and a
// relationship id. Validate reports it clean — including
// validatePresentationRelIDs, which exists to catch exactly "the same
// relationship id twice" but reads the *rebuilt* relationship set, where the
// duplicate has already collapsed into one entry. On save the writer allocates
// a fresh part name for the second slide, writes its content there, and then
// emits an sldIdLst that still names the shared r:id twice — so the new part is
// referenced by nothing. The count therefore rises by one on every save of an
// unedited deck, without bound: this is C363's shape reached by reading a file
// rather than by merging two.
func TestKnownDefectDuplicateSldIDRelationship(t *testing.T) {
	t.Skip("reproduces an unfixed defect: a duplicated sldId r:id makes every save add an orphan slide part")

	pkg := buildFuzzDeck(t)
	orig := string(fuzzSeedPart(t, pkg, fuzzPartPresentation))
	open := strings.Index(orig, "<p:sldIdLst>")
	closed := strings.Index(orig, "</p:sldIdLst>")
	if open < 0 || closed < 0 {
		t.Fatal("fixture presentation.xml has no sldIdLst")
	}
	mutated := orig[:open] +
		`<p:sldIdLst><p:sldId id="256" r:id="rId5"/><p:sldId id="257" r:id="rId5"/></p:sldIdLst>` +
		orig[closed+len("</p:sldIdLst>"):]

	cur := fuzzseed.ReplaceZipEntry(pkg, fuzzPartPresentation, []byte(mutated))
	counts := make([]int, 0, 3)
	for gen := 0; gen < 3; gen++ {
		p, err := OpenReader(bytes.NewReader(cur), int64(len(cur)))
		if err != nil {
			t.Fatalf("generation %d does not open: %v", gen, err)
		}
		if report := p.Validate(); report.HasErrors() {
			t.Logf("generation %d validate: %v", gen, report)
		}
		out, err := p.SaveBytes()
		_ = p.Close()
		if err != nil {
			t.Fatalf("generation %d does not save: %v", gen, err)
		}
		n := 0
		for name := range zipEntries(out) {
			if strings.HasPrefix(name, "ppt/slides/slide") {
				n++
			}
		}
		counts = append(counts, n)
		cur = out
	}
	for i := 1; i < len(counts); i++ {
		if counts[i] != counts[0] {
			t.Fatalf("saving an unedited deck changed the slide part count: %v", counts)
		}
	}
}

// TestKnownDefectSetNotesOverAnUnparseableNotesPart reproduces the defect found
// by FuzzPptxNotesSlideXML: a slide whose notesSlide relationship points at a
// part that does not parse.
//
// loadNotesSlide collapses "no relationship", "no part" and "the part does not
// parse" into the same (nil, "") result, so SetNotes takes its create-a-notes-
// slide branch and *appends* a second notesSlide relationship. The slide now
// carries two, which CT_Slide does not permit; notesSlidePartName returns the
// first, which is still the unparseable one; and Notes() therefore reports ""
// for text that SetNotes accepted without complaint. Validate reports the deck
// clean. Replacing the part the existing relationship already points at would
// fix all three.
func TestKnownDefectSetNotesOverAnUnparseableNotesPart(t *testing.T) {
	t.Skip("reproduces an unfixed defect: SetNotes over an unparseable notes part silently loses the text")

	pkg := buildFuzzDeck(t)
	broken := fuzzseed.ReplaceZipEntry(pkg, fuzzPartNotesSlide, []byte("<p:notes"))
	p, err := OpenReader(bytes.NewReader(broken), int64(len(broken)))
	if err != nil {
		t.Fatalf("deck with an unparseable notes part does not open: %v", err)
	}
	defer func() { _ = p.Close() }()

	slide := p.Slides()[0]
	const written = "notes written over a broken part"
	slide.SetNotes(written)

	notesRels := 0
	for _, rel := range p.relationships[slide.partName] {
		if rel != nil && rel.Type == opc.RelTypeNotesSlide {
			notesRels++
		}
	}
	if notesRels != 1 {
		t.Errorf("slide carries %d notesSlide relationships after SetNotes, want 1", notesRels)
	}

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = p2.Close() }()
	if got := p2.Slides()[0].Notes(); got != written {
		t.Errorf("Notes() reads back %q after SetNotes(%q)", got, written)
	}
}

// minimalNotesSlide is a notes slide that is valid PresentationML, namespace
// well-formed, and uses no DrawingML — so its root has no reason to declare the
// a: prefix, and does not.
const minimalNotesSlide = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<p:notes xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
	`<p:cSld><p:spTree>` +
	`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` +
	`</p:spTree></p:cSld></p:notes>`

// TestNotesRewriteBindsTheDrawingMLPrefix pins the fix for the defect
// FuzzPptxNotesSlideXML found, and it is the one that needed no malformed input
// at all: the notes slide below is valid.
//
// SetNotes has to build a body placeholder, whose text lives in DrawingML. The
// marshaler declares xmlns:a on each a:-prefixed element it happens to open —
// a:spLocks, a:bodyPr, a:lstStyle — but a declaration on a:lstStyle does not
// scope to its *sibling* a:p, so the paragraph, run and text elements are
// emitted with the a: prefix undeclared. The part is no longer namespace
// well-formed; PowerPoint reports the file as needing repair, and the library's
// own Notes() reads back "" for text SetNotes accepted without complaint.
//
// Go's decoder hides it: an undeclared prefix decodes with Name.Space set to
// the literal prefix instead of erroring, so nothing downstream notices.
//
// The fuzzer found a second way into the same hole, which is what makes this a
// binding bug rather than a declaration bug: a source root that binds a: to
// some URI other than DrawingML. The part stayed well-formed, the rewrite still
// wrote its text as a:t, and the text was again outside DrawingML. Both came
// from assuming the prefix means DrawingML instead of ensuring the DrawingML
// namespace is bound to whatever prefix the text is written under, which is now
// what the Builder does for every name it writes (xmlb.Builder.declareInline).
func TestNotesRewriteBindsTheDrawingMLPrefix(t *testing.T) {
	pkg := buildFuzzDeck(t)
	wrapped := fuzzseed.ReplaceZipEntry(pkg, fuzzPartNotesSlide, []byte(minimalNotesSlide))
	p, err := OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
	if err != nil {
		t.Fatalf("a valid minimal notes slide does not open: %v", err)
	}
	defer func() { _ = p.Close() }()

	slide := p.Slides()[0]
	const written = "hello world"
	slide.SetNotes(written)

	part := slide.notesSlidePartName()
	if !namespaceWellFormed(p.rawPartData(part)) {
		t.Errorf("rewritten %s is not namespace well-formed:\n%s", part, p.rawPartData(part))
	}
	if got := slide.Notes(); got != written {
		t.Errorf("Notes() reads back %q immediately after SetNotes(%q)", got, written)
	}

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = p2.Close() }()
	if got := p2.Slides()[0].Notes(); got != written {
		t.Errorf("Notes() reads back %q after a save and a re-open", got)
	}
}

// TestKnownDefectDroppedPartKeepsItsRelationship reproduces the defect found by
// FuzzPptxPresentationXML, FuzzPptxSlideMasterXML and FuzzPptxAuxPartXML: when
// a part is not loaded — because nothing in the id lists references it, or
// because its bytes do not parse — the writer omits the part but keeps the
// relationship that points at it, so the emitted package declares a target it
// does not contain.
//
// The three cases below share one root cause and one blind spot. Validate's
// CheckRelationshipTargets is supposed to catch exactly this, but it asks
// partExists, which falls through to the *source* reader (where the part is
// still sitting) and answers an unconditional true for presProps, viewProps and
// tableStyles. So the report is clean for all three. Either the relationship
// should be dropped with the part or the part should be written with the
// relationship; emitting one without the other is what PowerPoint reports as a
// file needing repair.
func TestKnownDefectDroppedPartKeepsItsRelationship(t *testing.T) {
	t.Skip("reproduces an unfixed defect: a part that is dropped on save keeps the relationship that points at it")

	pkg := buildFuzzDeck(t)
	presXML := string(fuzzSeedPart(t, pkg, fuzzPartPresentation))
	masterXML := string(fuzzSeedPart(t, pkg, fuzzPartMaster1))

	cases := []struct {
		name string
		part string
		body []byte
	}{{
		// No sldMasterIdLst: the master and every layout under it go unloaded,
		// and are then omitted while presentation.xml.rels still names the
		// master and every slide still relates to its layout.
		name: "presentation without a slide master id list",
		part: fuzzPartPresentation,
		body: []byte(dropElement(t, presXML, "<p:sldMasterIdLst>", "</p:sldMasterIdLst>")),
	}, {
		// No sldLayoutIdLst: the layouts go unloaded and are omitted while the
		// master's own rels still name every one of them.
		name: "slide master without a layout id list",
		part: fuzzPartMaster1,
		body: []byte(dropElement(t, masterXML, "<p:sldLayoutIdLst>", "</p:sldLayoutIdLst>")),
	}, {
		// An empty part: dropped on save, relationship retained.
		name: "empty tableStyles part",
		part: fuzzPartTableStyles,
		body: []byte{},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fuzzseed.ReplaceZipEntry(pkg, tc.part, tc.body)
			p, err := OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			if report := p.Validate(); report.HasErrors() {
				t.Logf("validate: %v", report)
			}
			out, err := p.SaveBytes()
			_ = p.Close()
			if err != nil {
				t.Fatalf("save: %v", err)
			}
			checkEmittedRelTargets(t, out)
		})
	}
}

// dropElement removes the first open..close element from an XML document.
func dropElement(tb testing.TB, doc, openTag, closeTag string) string {
	tb.Helper()
	i := strings.Index(doc, openTag)
	j := strings.Index(doc, closeTag)
	if i < 0 || j < 0 {
		tb.Fatalf("building fuzz fixture: %s not found", openTag)
	}
	return doc[:i] + doc[j+len(closeTag):]
}

// --- fixture guard ---

// TestFuzzDeckCarriesEveryFuzzedPart is the guard against the failure mode that
// makes a targeted fuzzer worthless without ever failing: rewriting a part the
// fixture does not contain. ReplaceZipEntry then matches nothing, every
// execution round-trips the same untouched package, and the target reports
// hundreds of thousands of reassuring execs while exercising no parser at all.
//
// It also pins that the grafted legacy comment machinery is really wired up —
// that the deck reads back two legacy comments with their authors resolved —
// so a change to how unknown parts are carried across a round trip is caught
// here rather than quietly hollowing out FuzzPptxLegacyCommentsXML.
func TestFuzzDeckCarriesEveryFuzzedPart(t *testing.T) {
	pkg := buildFuzzDeck(t)
	entries := zipEntries(pkg)
	for _, name := range fuzzedParts {
		if len(entries[name]) == 0 {
			t.Errorf("the fuzz fixture has no %s, so any target rewriting it substitutes nothing", name)
		}
	}
	for _, name := range fuzzAuxParts {
		if len(entries[name]) == 0 {
			t.Errorf("the fuzz fixture has no aux part %s", name)
		}
	}

	p, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("the fuzz fixture does not open: %v", err)
	}
	defer func() { _ = p.Close() }()

	var legacy, modern int
	authors := map[string]bool{}
	for _, s := range p.Slides() {
		for _, c := range s.Comments() {
			authors[c.Author()] = true
			if strings.HasPrefix(c.ID(), "{") {
				modern++
			} else {
				legacy++
			}
			for _, r := range c.Replies() {
				authors[r.Author()] = true
				modern++
			}
		}
	}
	if legacy != 2 {
		t.Errorf("fuzz fixture carries %d legacy comments, want 2 — the grafted part is not being read", legacy)
	}
	if modern != 2 {
		t.Errorf("fuzz fixture carries %d modern comments and replies, want 2", modern)
	}
	for _, want := range []string{"Alice Author", "Bob Reader", "Carol Legacy", "Dave Legacy"} {
		if !authors[want] {
			t.Errorf("fuzz fixture does not resolve comment author %q; got %v", want, sortedAuthorNames(authors))
		}
	}
	if got := p.Slides()[0].Notes(); got == "" {
		t.Error("fuzz fixture slide 1 has no speaker notes")
	}
}

func sortedAuthorNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
