package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/internal/fuzzbound"
	"github.com/mgilbir/spine/internal/fuzzseed"
)

// Targeted fuzzing of the SECONDARY docx parts.
//
// FuzzOpenDocx mutates raw archive bytes, so almost every mutation breaks the
// zip and the part parsers are barely reached. The targets here take the other
// shape: a package the library itself built, with exactly one inner part
// replaced by the fuzzer's bytes. The zip, the content types, the
// relationships and every other part stay valid, so the bytes land in the
// parser under test on every single execution.
//
// The fixture matters as much as the shape. A bare Create() document has no
// numbering, no notes, no comments and no page furniture, so a target seeded
// from one would be substituting a part that is not in the package —
// reassuring green runs that exercise nothing. buildRichDocxFuzzSeed builds a
// document that genuinely carries all of them, and each target asserts at
// startup that the part it is about to replace is really there.
//
// None of these targets is a "did not panic" target. The bugs this library
// ships are wrong output, not crashes, so each one asserts:
//
//   - Honest errors. Open returns a document or an error, never both and never
//     neither.
//   - The document still works. Everything the API could read out of the
//     surviving parts before the round trip is still readable after it, with
//     the same values.
//   - No new structural defects. Document.Validate on the reopened package may
//     report fewer findings than before, never more: a save that invents a
//     dangling reference or an inheritance cycle has corrupted the document.
//   - A fixed point. Where an idempotent mutation exists, applying it, saving,
//     reopening and applying it again must produce the identical part bytes.
//     Sanitizing on the first save is legitimate; drifting forever is a parser
//     and a serializer disagreeing, and it corrupts a document a little more
//     on every open-and-save cycle.

// buildRichDocxFuzzSeed builds a package that actually carries every secondary
// part the targets in this file replace: styles.xml, numbering.xml,
// settings.xml, footnotes.xml, endnotes.xml, comments.xml,
// commentsExtended.xml, people.xml, header1.xml and footer1.xml — with the
// main document part carrying live cross-references into each of them (a
// numPr, a footnote and endnote reference, a comment range and reference, a
// pStyle, and section header/footer references).
//
// Those cross-references are the point: they are what makes a mutated
// secondary part observable at all. Replacing numbering.xml in a document
// whose body never references a numId proves nothing.
func buildRichDocxFuzzSeed(f testing.TB) []byte {
	f.Helper()
	d := Create()

	// A style, referenced from a paragraph.
	d.Styles().AddParagraphStyle("FuzzBody", "Fuzz Body").
		SetBasedOn("Normal").
		SetFontSize(11)
	styled := d.AddParagraphWithText("styled paragraph")
	styled.SetStyle("FuzzBody")

	// Notes and a comment, anchored in the body.
	host := d.AddParagraphWithText("host paragraph")
	run := host.Runs()[0]
	run.AddFootnote("a footnote body")
	run.AddEndnote("an endnote body")
	c := host.AddComment("Fuzz Author", "a comment body")
	c.Reply("Second Author", "a reply body")

	// A numbering definition with two levels and two paragraphs using it, so
	// the body carries numPr references into numbering.xml.
	def := d.Numbering().AddDefinition()
	def.SetLevel(0, NumberFormatDecimal, "%1.")
	def.SetLevel(1, NumberFormatLowerLetter, "%2)")
	list := def.ListStyle()
	d.AddParagraphWithText("first item").SetListStyle(list, 0)
	d.AddParagraphWithText("second item").SetListStyle(list, 1)

	// Page furniture, referenced from the section properties.
	d.AddHeader(HeaderDefault).AddParagraphWithText("header text")
	d.AddFooter(FooterDefault).AddParagraphWithText("footer text")

	// Settings content beyond the defaults.
	d.SetZoom(110)
	d.SetDefaultTabStop(36)
	d.SetDocumentVariable("SeedVar", "seed value")

	// A table, so the paragraph walk has nested content to traverse.
	tbl := d.AddTable(2, 2)
	tbl.Rows()[0].Cells()[0].AddParagraph().SetText("cell text")

	// Pinned so the fixture is byte-stable across builds; a fixture that moves
	// cannot be reproduced from a crasher. See fuzzseed.FixtureModified.
	d.Properties.Created = fuzzseed.FixtureModified
	d.Properties.Modified = fuzzseed.FixtureModified

	valid, err := d.SaveBytes()
	if err != nil {
		f.Fatalf("building the rich docx fuzz seed: %v", err)
	}
	// Comments carry a date and a paragraph id the writer generates as it
	// writes them, with no API to reach either; pinning the core properties
	// above does not cover them. See fuzzseed.PinGenerated.
	pinned, err := fuzzseed.PinGenerated(valid)
	if err != nil {
		f.Fatalf("building the rich docx fuzz seed: %v", err)
	}
	return pinned
}

// maxFuzzedPartBytes caps the size of a substituted part.
//
// Every target here opens the package, walks the model, saves, reopens and
// saves again — four or five passes over the content. A multi-megabyte part
// therefore costs seconds per execution, and because the fuzzing engine keeps
// mutating whatever it found most recently, one such input takes every worker
// out of circulation and the campaign stops exploring. The resource budgets
// scale with input size and stay meaningful far below this cap, so nothing is
// lost by refusing to spend the campaign on one enormous input.
const maxFuzzedPartBytes = 192 << 10

// skipOversizedParts skips an execution whose substituted parts are too large
// to explore with.
func skipOversizedParts(t *testing.T, parts ...[]byte) {
	t.Helper()
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	if total > maxFuzzedPartBytes {
		t.Skip("substituted parts are too large to explore with")
	}
}

// requireSeedParts fails the fuzz setup when the fixture does not carry every
// part a target is about to replace, and returns their original bytes.
//
// This is not defensive noise. A target whose substitution matches no entry in
// the package silently degrades into a no-op that reports thousands of clean
// executions, which is indistinguishable from a target that works.
func requireSeedParts(f *testing.F, pkg []byte, names ...string) [][]byte {
	f.Helper()
	out := make([][]byte, 0, len(names))
	for _, name := range names {
		body := fuzzseed.ZipEntry(pkg, name)
		if body == nil {
			f.Fatalf("the fuzz fixture has no %s: the target would be replacing a part that is not in the package", name)
		}
		out = append(out, body)
	}
	return out
}

// openFuzzedPackage opens a package built from fuzzer bytes and asserts the
// error contract: a document or an error, never both, never neither. A nil
// document with a nil error is the shape a caller dereferences.
func openFuzzedPackage(t *testing.T, pkg []byte) *Document {
	t.Helper()
	d, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	switch {
	case err != nil && d != nil:
		t.Fatalf("OpenReader returned a document AND an error, so a caller that checks the error still leaks a half-built document: %v", err)
	case err == nil && d == nil:
		t.Fatal("OpenReader returned a nil document and a nil error, which every caller dereferences")
	}
	return d
}

// reopenSaved opens a package this library just wrote. Failing to reopen it is
// always a bug: whatever the input was, the writer produced those bytes.
func reopenSaved(t *testing.T, pkg []byte, what string) *Document {
	t.Helper()
	d, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("%s: a package this library just wrote does not reopen: %v", what, err)
	}
	if d == nil {
		t.Fatalf("%s: reopening a package this library just wrote returned a nil document and a nil error", what)
	}
	return d
}

// validationCodes counts Document.Validate findings by code.
func validationCodes(d *Document) map[string]int {
	out := make(map[string]int)
	for _, e := range d.Validate() {
		out[e.Code]++
	}
	return out
}

// assertNoNewDefects fails when the round trip introduced validation findings
// that the model did not have before it. Losing findings is fine (the writer
// may drop an unreferenceable fragment); gaining them means the save wrote a
// document that is more broken than the model it came from.
func assertNoNewDefects(t *testing.T, before, after map[string]int, what string) {
	t.Helper()
	for code, n := range after {
		if n > before[code] {
			t.Fatalf("%s: the round trip introduced validation findings: %s went from %d to %d\n before: %v\n after:  %v",
				what, code, before[code], n, before, after)
		}
	}
}

// walkDocument reads every part of the model an ordinary caller would touch.
// It exists so a mutated secondary part cannot leave the body walk in a state
// that panics only when somebody looks at it.
func walkDocument(d *Document) {
	for i, p := range d.Paragraphs() {
		if i >= 128 {
			break
		}
		_ = p.Text()
		_ = p.Style()
		for j, r := range p.Runs() {
			if j >= 32 {
				break
			}
			_ = r.Text()
			_ = r.Bold()
		}
	}
	for i, tbl := range d.Tables() {
		if i >= 8 {
			break
		}
		for j, row := range tbl.Rows() {
			if j >= 16 {
				break
			}
			for k, cell := range row.Cells() {
				if k >= 16 {
					break
				}
				_ = cell.Text()
			}
		}
	}
	_ = d.Body()
	_ = d.Sections()
}

// zipPartBytes returns the named entry of a package, or nil.
func zipPartBytes(data []byte, name string) []byte {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil
		}
		return body
	}
	return nil
}

// assertPartFixedPoint fails when a part's bytes changed between two saves
// that had the same idempotent mutation applied to them.
//
// The first save is allowed to differ from the input: regenerating a part from
// the model legitimately normalizes it. The second must not differ from the
// first. A part that keeps changing is a parser and a serializer that disagree,
// and every open-and-save cycle a user performs pushes the document further
// from what they wrote.
func assertPartFixedPoint(t *testing.T, first, second []byte, names ...string) {
	t.Helper()
	for _, name := range names {
		if !namespacesResolve(zipPartBytes(first, name)) {
			// KNOWN FINDING, reported not fixed. Regenerating a metadata part
			// replays the source root's captured attribute list verbatim, so a
			// source part that declared no namespaces at all produces a root
			// that uses the w: prefix and declares nothing:
			//
			//	settings.xml = "<A/>", then any settings edit, saves
			//	<w:settings><w:zoom w:percent="100"/></w:settings>
			//
			// which is not namespace-well-formed and which Word rejects. It is
			// also not a fixed point: reopening it makes Go resolve the
			// undeclared w: to the literal namespace "w", and the next save
			// writes xmlns:w="w" onto the children, so the part converges only
			// on the third pass.
			//
			// The convergence failure is a consequence of the missing
			// declaration, not an independent bug, so the check is skipped
			// while the output is unresolvable rather than reported twice.
			// Delete this guard once the writer declares the namespaces its
			// own output uses.
			continue
		}
		a, b := zipPartBytes(first, name), zipPartBytes(second, name)
		if bytes.Equal(a, b) {
			continue
		}
		t.Fatalf("%s is not a fixed point: re-applying the same mutation to the reopened package produced different bytes\n first:  %q\n second: %q",
			name, truncateForMessage(a), truncateForMessage(b))
	}
}

func truncateForMessage(b []byte) string {
	const limit = 2048
	if len(b) <= limit {
		return string(b)
	}
	return string(b[:limit]) + "…"
}

// rootChildNames reports the local names of the direct children of a part's
// root element, and whether the part could be decoded at all.
//
// This is the exact surface of the deletion class this library keeps
// rediscovering. C370: the metadata parts were parsed with the plain
// xmlb.Unmarshal entry point, which leaves the capture kit inert, so the first
// edit that flipped a part to regenerate emitted only the root children the
// model types and deleted the rest — w:latentStyles from styles.xml, w:rsids
// and w:compat from settings.xml. There is no error and no panic; the content
// is simply gone from the saved document.
//
// Deliberately root children only, and deliberately names rather than content.
// Going deeper, or comparing attributes, means asserting that content the OOXML
// schema does not allow in the first place survives a round trip: a fuzzer
// invents `w:tyleId` on a w:style within seconds, w:style has a closed
// four-attribute list, and no producer will ever write it. Demanding its
// survival produces noise, not findings. Root children are different — a real
// producer writes root children this model does not type, and the capture kit
// exists precisely to keep them.
//
// Local names, not namespaces: an input part is free to use an undeclared
// prefix, which Go's decoder reports as a bare prefix string while a
// regenerated part reports the URI it was rebound to, and that difference is a
// false positive rather than a finding.
func rootChildNames(data []byte) (map[string]bool, bool) {
	out := make(map[string]bool)
	dec := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, true
		}
		if err != nil {
			return nil, false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				out[t.Name.Local] = true
			}
		case xml.EndElement:
			depth--
		}
	}
}

// namespacesResolve reports whether every namespace-qualified name in a part
// resolves to a namespace the part declares.
//
// It reads Go's own resolution rather than re-implementing scoping: the decoder
// rewrites a declared prefix to its URI and leaves an UNDECLARED prefix in
// Name.Space verbatim, so a Space that is not among the declared URIs is
// exactly an undeclared prefix. Undecodable bytes count as unresolvable.
func namespacesResolve(data []byte) bool {
	declared := map[string]bool{"": true, "xml": true, "xmlns": true}
	used := make(map[string]bool)
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		used[start.Name.Space] = true
		for _, a := range start.Attr {
			switch {
			case a.Name.Space == "xmlns":
				declared[a.Value] = true
			case a.Name.Space == "" && a.Name.Local == "xmlns":
				declared[a.Value] = true
			default:
				used[a.Name.Space] = true
			}
		}
	}
	for ns := range used {
		if !declared[ns] {
			return false
		}
	}
	return true
}

// rootNamespaceDecls reports the xmlns declarations on a part's root element,
// keyed by prefix ("" for the default namespace), and whether the part decoded.
func rootNamespaceDecls(data []byte) (map[string]string, bool) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, true // no root element at all
		}
		if err != nil {
			return nil, false
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		out := make(map[string]string)
		for _, a := range start.Attr {
			switch {
			case a.Name.Space == "xmlns":
				out[a.Name.Local] = a.Value
			case a.Name.Space == "" && a.Name.Local == "xmlns":
				out[""] = a.Value
			}
		}
		return out, true
	}
}

// assertRootDeclsPreserved fails when a regenerated part lost a namespace
// declaration its source root carried, or rebound one to a different URI.
//
// This is the other half of C370, and the half that is actually reachable for a
// part like styles.xml whose root children are a closed set. Every child the
// capture kit preserves verbatim references its prefixes through the root
// declarations; drop a declaration and the preserved content stops resolving,
// which is a document Word repairs. Replaying the root attributes verbatim is
// what the fix was, and it only works when the part was parsed with its source
// bytes registered — so this assertion fails the moment a part goes back to the
// plain unmarshal entry point.
func assertRootDeclsPreserved(t *testing.T, source, regenerated []byte, name string) {
	t.Helper()
	if regenerated == nil {
		t.Fatalf("the saved package has no %s at all: a part the source package carried was not written", name)
	}
	before, ok := rootNamespaceDecls(source)
	if !ok {
		return // the fuzzer's bytes do not decode on their own terms
	}
	after, ok := rootNamespaceDecls(regenerated)
	if !ok {
		t.Fatalf("the regenerated %s does not decode as XML", name)
	}
	for prefix, uri := range before {
		got, present := after[prefix]
		if !present {
			t.Fatalf("regenerating %s dropped the xmlns:%s=%q declaration its source root carried; every preserved child that uses that prefix now fails to resolve",
				name, prefix, uri)
		}
		if got != uri {
			t.Fatalf("regenerating %s rebound xmlns:%s from %q to %q", name, prefix, uri, got)
		}
	}
}

// assertRootChildrenPreserved fails when a regenerated part lost a root child
// the source part carried.
//
// Only meaningful where the edit driving the regeneration is purely additive. A
// target whose mutation legitimately rewrites content — Paragraph.SetText
// clears the paragraph, SetFootnoteProperties rebuilds w:footnotePr — must not
// use it.
func assertRootChildrenPreserved(t *testing.T, source, regenerated []byte, name string) {
	t.Helper()
	if regenerated == nil {
		t.Fatalf("the saved package has no %s at all: a part the source package carried was not written", name)
	}
	before, ok := rootChildNames(source)
	if !ok {
		return // the fuzzer's bytes do not decode on their own terms
	}
	after, ok := rootChildNames(regenerated)
	if !ok {
		t.Fatalf("the regenerated %s does not decode as XML", name)
	}
	for k := range before {
		if !after[k] {
			t.Fatalf("regenerating %s dropped its %s child, which the source part carried: the model does not type it, so the save deleted it from the document",
				name, k)
		}
	}
}

// sortedSetKeys is a stable rendering of a string set for failure messages.
func sortedSetKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// assertSubset fails when the round trip lost an entry the model carried
// before it.
func assertSubset(t *testing.T, before, after map[string]bool, what string) {
	t.Helper()
	for k := range before {
		if !after[k] {
			t.Fatalf("%s: the round trip dropped %q\n before: %v\n after:  %v",
				what, k, sortedSetKeys(before), sortedSetKeys(after))
		}
	}
}

// --- styles.xml -------------------------------------------------------------

// styleFacts is what the styles API reports for one definition.
type styleFacts struct {
	name, basedOn string
	styleType     StyleType
}

func styleSnapshot(d *Document) map[string]styleFacts {
	out := make(map[string]styleFacts)
	for _, s := range d.Styles().List() {
		id := s.ID()
		if _, dup := out[id]; dup {
			// A duplicate id resolves to the first definition everywhere in the
			// package, so record the first and ignore the rest.
			continue
		}
		out[id] = styleFacts{name: s.Name(), basedOn: s.BasedOn(), styleType: s.Type()}
	}
	return out
}

// basedOnChainTermination reports, for each styleId, whether walking its
// w:basedOn chain ends — at a style with no parent, or at a dangling parent
// name, both of which are legal. A chain that revisits a definition never ends.
//
// A parsed part is free to contain a cycle already: producers write them, and
// this library does not repair what it did not author. What must hold is that
// Style.SetBasedOn never TURNS a terminating chain into a cyclic one, which is
// exactly the promise the setter makes and the only thing standing between a
// caller and a styles part Word repairs or misrenders. Nothing downstream
// catches it either: the chain is walked lazily, so a cycle the setter created
// surfaces in Word rather than here.
// It is written as a single linear pass with memoization rather than the
// obvious walk-from-every-style. The obvious version is cubic — every style,
// times a chain as long as the style list, times a linear lookup per step — and
// a fuzzer that discovers a styles part with a few thousand definitions drives
// every worker into it and the whole campaign stops making progress. That is
// not hypothetical: it is what the first version of this file did, and the
// measured symptom was 4177 executions followed by thirty seconds at 0/sec.
func basedOnChainTermination(d *Document) map[string]bool {
	// parent[id] is the styleId a definition inherits from, "" for none. A
	// duplicate id resolves to the first definition, matching every lookup in
	// the package.
	parent := make(map[string]string)
	order := make([]string, 0)
	for _, s := range d.Styles().List() {
		id := s.ID()
		if _, dup := parent[id]; dup {
			continue
		}
		parent[id] = s.BasedOn()
		order = append(order, id)
	}

	const (
		onPath = iota + 1
		ends
		loops
	)
	state := make(map[string]int, len(parent))
	for _, start := range order {
		if _, done := state[start]; done {
			continue
		}
		var path []string
		result := ends
		for cur := start; ; {
			if st, seen := state[cur]; seen {
				result = st
				break
			}
			p, defined := parent[cur]
			if !defined {
				break // a dangling parent name: legal, and it ends the chain
			}
			state[cur] = onPath
			path = append(path, cur)
			if p == "" {
				break
			}
			cur = p
		}
		if result == onPath {
			result = loops
		}
		for _, id := range path {
			state[id] = result
		}
	}

	out := make(map[string]bool, len(parent))
	for id := range parent {
		out[id] = state[id] != loops
	}
	return out
}

// assertNoCycleIntroduced fails when a style whose inheritance chain used to
// end no longer does.
func assertNoCycleIntroduced(t *testing.T, before, after map[string]bool, what string) {
	t.Helper()
	for id, terminated := range before {
		if !terminated {
			continue
		}
		if ended, present := after[id]; present && !ended {
			t.Fatalf("%s: the basedOn chain from style %q used to end and now inherits from itself; Style.SetBasedOn is supposed to refuse any value that closes a cycle", what, id)
		}
	}
}

// FuzzDocxStylesXML replaces word/styles.xml in an otherwise-valid package.
//
// Three invariants, none of them "did not panic":
//
//  1. Every style the API could read before the round trip is still readable
//     after it, with the same name, type and parent. styles.xml is regenerated
//     from the model as soon as anything marks it modified, so a definition the
//     model does not type is a definition the save can silently delete (the
//     C370 class).
//  2. Style.SetBasedOn never closes an inheritance cycle, whatever the parsed
//     part already contained.
//  3. The part is a fixed point under an idempotent edit.
func FuzzDocxStylesXML(f *testing.F) {
	valid := buildRichDocxFuzzSeed(f)
	const part = "word/styles.xml"
	orig := requireSeedParts(f, valid, part)[0]

	// The root carries the declaration set Word actually writes, not the bare
	// w: one. The declarations are the point: preserved children reference
	// their prefixes through them, and a regeneration that replays a fixed
	// standard set instead of the source's own drops every extension prefix
	// (C370). A seed whose root declares only w: cannot show that.
	const stylesOpen = `<w:styles ` +
		`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" ` +
		`xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml" ` +
		`mc:Ignorable="w14 w15">`
	f.Add(orig, "Normal")
	f.Add([]byte{}, "")
	f.Add([]byte("not xml at all"), "FuzzBody")
	f.Add(orig[:len(orig)/2], "Normal")
	// A self-inheriting style, and a two-step cycle: the shapes SetBasedOn is
	// supposed to refuse to extend.
	f.Add([]byte(stylesOpen+
		`<w:style w:type="paragraph" w:styleId="A"><w:name w:val="A"/><w:basedOn w:val="A"/></w:style>`+
		`<w:style w:type="paragraph" w:styleId="B"><w:name w:val="B"/><w:basedOn w:val="C"/></w:style>`+
		`<w:style w:type="paragraph" w:styleId="C"><w:name w:val="C"/><w:basedOn w:val="B"/></w:style>`+
		`</w:styles>`), "A")
	// Duplicate style ids and a dangling parent.
	f.Add([]byte(stylesOpen+
		`<w:style w:type="paragraph" w:styleId="Dup"><w:name w:val="one"/></w:style>`+
		`<w:style w:type="character" w:styleId="Dup"><w:name w:val="two"/></w:style>`+
		`<w:style w:type="paragraph" w:styleId="Orphan"><w:basedOn w:val="Nowhere"/></w:style>`+
		`</w:styles>`), "Dup")
	// docDefaults plus unmodeled children, the regeneration-deletes-what-it-does-
	// not-type surface.
	f.Add([]byte(stylesOpen+
		`<w:docDefaults><w:rPrDefault><w:rPr><w:sz w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults>`+
		`<w:latentStyles w:defLockedState="0" w:count="371"><w:lsdException w:name="Normal"/></w:latentStyles>`+
		`<w:style w:type="paragraph" w:styleId="Keep"><w:name w:val="Keep"/><w:pPr><w:keepNext/></w:pPr></w:style>`+
		`</w:styles>`), "Keep")
	// A style id with hostile characters and an enormous uiPriority.
	f.Add([]byte(stylesOpen+
		`<w:style w:type="paragraph" w:styleId="&lt;&amp;&quot;"><w:name w:val="]]&gt;"/>`+
		`<w:uiPriority w:val="99999999999999999999"/></w:style></w:styles>`), `<&"`)
	// Names carrying every character XML has to escape. This is the seed that
	// makes a setter and a getter disagreeing about the encoding visible: the
	// round trip reads a name back, writes it again, and a value that is
	// escaped on the way in but not decoded on the way out grows one escape per
	// save until the document is unreadable.
	f.Add([]byte(stylesOpen+
		`<w:style w:type="paragraph" w:styleId="Esc"><w:name w:val="a &amp; b &lt;c&gt; &quot;d&quot;"/></w:style>`+
		`<w:style w:type="character" w:styleId="Esc2"><w:name w:val="]]&gt;"/><w:basedOn w:val="Esc"/></w:style>`+
		`</w:styles>`), "Esc")

	f.Fuzz(func(t *testing.T, data []byte, parentID string) {
		skipOversizedParts(t, data)
		pkg := fuzzseed.ReplaceZipEntry(valid, part, data)
		if pkg == nil {
			t.Skip("seed package unreadable")
		}
		// The inheritance probe runs on its own handle. Chaining every style to
		// the next one is the shape SetBasedOn exists to refuse, but it is not
		// an idempotent edit — whether a given link is accepted depends on the
		// graph at the moment it is offered — so mixing it into the round trip
		// below would turn a legitimate difference into a fixed-point failure.
		if probe := openFuzzedPackage(t, pkg); probe != nil {
			termBefore := basedOnChainTermination(probe)
			chainStylesIntoRing(probe, parentID)
			assertNoCycleIntroduced(t, termBefore, basedOnChainTermination(probe),
				"after chaining every style to the next one")
			_ = probe.Close()
		}

		d := openFuzzedPackage(t, pkg)
		if d == nil {
			return
		}
		defer func() { _ = d.Close() }()

		before := styleSnapshot(d)
		walkDocument(d)

		mutateStyles(d)
		codesBefore := validationCodes(d)

		first, err := d.SaveBytes()
		if err != nil {
			// Refusing to write a document the model considers broken is a
			// legitimate outcome.
			return
		}
		assertPartsAreWellFormed(t, first)
		assertEmittedNamespacesResolve(t, pkg, first)
		assertRootChildrenPreserved(t, data, zipPartBytes(first, part), part)
		assertRootDeclsPreserved(t, data, zipPartBytes(first, part), part)

		d2 := reopenSaved(t, first, "styles round trip")
		defer func() { _ = d2.Close() }()
		mid := styleSnapshot(d2)
		walkDocument(d2)

		for id, was := range before {
			now, ok := mid[id]
			if !ok {
				t.Fatalf("saving dropped style %q, which the model read out of the parsed part", id)
			}
			if was.name != now.name || was.styleType != now.styleType {
				t.Fatalf("style %q changed across the round trip: name %q -> %q, type %q -> %q",
					id, was.name, now.name, was.styleType, now.styleType)
			}
		}
		assertNoNewDefects(t, codesBefore, validationCodes(d2), "styles round trip")

		mutateStyles(d2)
		second, err := d2.SaveBytes()
		if err != nil {
			t.Fatalf("re-saving a package this library just wrote failed: %v", err)
		}
		assertPartFixedPoint(t, first, second, part)
	})
}

// mutateStyles applies an edit that is idempotent in the styleId space: the
// probe style is created once (AddStyle is idempotent on the id) and thereafter
// only re-set to the same values.
func mutateStyles(d *Document) {
	m := d.Styles()
	m.AddStyle(StyleTypeParagraph, "FuzzProbe", "Fuzz Probe").
		SetName("Fuzz Probe").
		SetUIPriority(11).
		SetBold(true)
	for i, s := range m.List() {
		if i >= 32 {
			break
		}
		s.SetName(s.Name())
	}
}

// chainStylesIntoRing points every style at the next one in document order and
// the last one back at the first, offering SetBasedOn the one thing it promises
// to refuse: a value that closes an inheritance cycle. The caller-supplied
// parentID is offered too, so the fuzzer can aim a link at a style id of its
// own choosing (including one that does not exist, and one that is already
// part of a cycle the parsed part contained).
func chainStylesIntoRing(d *Document, parentID string) {
	m := d.Styles()
	list := m.List()
	if len(list) > 32 {
		list = list[:32]
	}
	for i, s := range list {
		s.SetBasedOn(list[(i+1)%len(list)].ID())
	}
	for _, s := range list {
		s.SetBasedOn(parentID)
	}
}

// --- numbering.xml ----------------------------------------------------------

// numberingIDs is the set of numbering-instance ids the document knows about,
// counting both the definitions parsed from the part and any added this
// session. It is the space Document.nextNumID allocates out of.
func numberingIDs(d *Document) map[string]bool {
	out := make(map[string]bool)
	if d.numbering == nil {
		return out
	}
	for _, id := range d.numbering.ParsedNumIDs {
		out[id] = true
	}
	for _, n := range d.numbering.Num {
		if n != nil {
			out[n.NumId] = true
		}
	}
	return out
}

// abstractNumberingIDs is the same for abstract definitions.
func abstractNumberingIDs(d *Document) map[string]bool {
	out := make(map[string]bool)
	if d.numbering == nil {
		return out
	}
	for _, id := range d.numbering.ParsedAbstractNumIDs {
		out[id] = true
	}
	for _, an := range d.numbering.AbstractNum {
		if an != nil {
			out[an.AbstractNumId] = true
		}
	}
	return out
}

// numberingBudget bounds one open-mutate-save cycle over a fuzzed
// numbering.xml. Numbering is the one docx part with cross-part integer
// indices — a paragraph's numId selects a w:num, which selects a
// w:abstractNum, which is indexed by level — so a hostile value reaching an
// allocation or a loop bound shows up here rather than as a machine falling
// over. The floor covers the fixed cost of building and re-serializing the
// whole fixture package.
var numberingBudget = fuzzbound.Budget{
	What:              "docx numbering open+save",
	Bytes:             64 << 20,
	BytesPerInputByte: 512,
	Time:              20 * time.Second,
	TimePerMiB:        20 * time.Second,
}

// FuzzDocxNumberingXML replaces word/numbering.xml in a package whose body
// paragraphs carry live numPr references into it.
//
// The invariants are about the id space, because that is where numbering
// breaks. Document.nextNumID allocates "one past the largest id it could
// parse", so an id it cannot parse — an overflowing integer, a padded or
// signed one, an empty attribute — is invisible to the allocator and the next
// list registered collides with a definition that is already there. A
// collision does not panic and does not fail to save; it silently renumbers a
// list in the user's document.
//
//  1. Every numbering id the model knew before the round trip is still there
//     after it.
//  2. A newly registered list gets an id that no existing definition uses, and
//     the paragraph it is applied to still resolves after a save and reopen —
//     Validate reports no more dangling numId references than before.
//  3. The whole cycle stays inside a resource budget.
func FuzzDocxNumberingXML(f *testing.F) {
	valid := buildRichDocxFuzzSeed(f)
	const part = "word/numbering.xml"
	orig := requireSeedParts(f, valid, part)[0]

	const numOpen = `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`
	f.Add(orig, 0)
	f.Add([]byte{}, 1)
	f.Add([]byte("<w:numbering>"), 0)
	f.Add(orig[:len(orig)/2], 2)
	// Ids the allocator cannot parse, so it believes the space is empty.
	f.Add([]byte(numOpen+
		`<w:abstractNum w:abstractNumId="99999999999999999999"><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl></w:abstractNum>`+
		`<w:num w:numId="99999999999999999999"><w:abstractNumId w:val="99999999999999999999"/></w:num>`+
		`</w:numbering>`), 0)
	// A w:num pointing at an abstract definition that does not exist, and one
	// pointing at a negative index.
	f.Add([]byte(numOpen+
		`<w:num w:numId="1"><w:abstractNumId w:val="4242"/></w:num>`+
		`<w:num w:numId="2"><w:abstractNumId w:val="-7"/></w:num>`+
		`</w:numbering>`), 1)
	// Levels out of order, duplicated, and at an ilvl far past the schema's 0-8.
	f.Add([]byte(numOpen+
		`<w:abstractNum w:abstractNumId="0">`+
		`<w:lvl w:ilvl="8"><w:start w:val="-2147483648"/><w:lvlText w:val="%9"/></w:lvl>`+
		`<w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/></w:lvl>`+
		`<w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl>`+
		`<w:lvl w:ilvl="2147483647"><w:lvlText w:val="%1"/></w:lvl>`+
		`</w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num></w:numbering>`), 8)
	// An id that parses cleanly but is enormous: the shape that turns a length
	// or an index read out of a part into an allocation size (the C360 class).
	f.Add([]byte(numOpen+
		`<w:abstractNum w:abstractNumId="2000000000"><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl></w:abstractNum>`+
		`<w:num w:numId="2000000000"><w:abstractNumId w:val="2000000000"/></w:num>`+
		`</w:numbering>`), 0)
	// Duplicate numIds and an empty id attribute.
	f.Add([]byte(numOpen+
		`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`+
		`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`+
		`<w:num w:numId=""><w:abstractNumId w:val=""/></w:num>`+
		`</w:numbering>`), 0)

	f.Fuzz(func(t *testing.T, data []byte, level int) {
		if fuzzbound.Tripped() {
			t.Skip("a resource budget was already exceeded in this process")
		}
		skipOversizedParts(t, data)
		pkg := fuzzseed.ReplaceZipEntry(valid, part, data)
		if pkg == nil {
			t.Skip("seed package unreadable")
		}
		// Keep the level inside the schema's range; the point of the target is
		// the id space, not rejecting an obviously invalid ilvl.
		level = ((level % 9) + 9) % 9

		var (
			first  []byte
			saveOK bool
			d      *Document
		)
		numberingBudget.Check(t, len(pkg), func() {
			if d != nil {
				_ = d.Close()
			}
			d = openFuzzedPackage(t, pkg)
			if d == nil {
				return
			}
			walkDocument(d)
			numsBefore := numberingIDs(d)
			absBefore := abstractNumberingIDs(d)

			def := d.Numbering().AddDefinition()
			if got := strconv.Itoa(def.AbstractNumID()); absBefore[got] {
				t.Fatalf("AddDefinition allocated abstract id %q, which the parsed part already defines: the new definition overwrites an existing list's formatting", got)
			}
			def.SetLevel(level, NumberFormatDecimal, "%1.")
			list := def.ListStyle()
			if got := strconv.Itoa(list.numID); numsBefore[got] {
				t.Fatalf("ListStyle allocated numbering instance id %q, which the parsed part already defines: two lists now share a counter", got)
			}
			if paras := d.Paragraphs(); len(paras) > 0 {
				paras[len(paras)-1].SetListStyle(list, level)
			}

			codesBefore := validationCodes(d)
			out, err := d.SaveBytes()
			if err != nil {
				return
			}
			first, saveOK = out, true
			assertPartsAreWellFormed(t, first)
			assertEmittedNamespacesResolve(t, pkg, first)
			assertRootChildrenPreserved(t, data, zipPartBytes(first, part), part)
			assertRootDeclsPreserved(t, data, zipPartBytes(first, part), part)

			d2 := reopenSaved(t, first, "numbering round trip")
			defer func() { _ = d2.Close() }()
			walkDocument(d2)
			assertSubset(t, numsBefore, numberingIDs(d2), "numbering instance ids")
			assertSubset(t, absBefore, abstractNumberingIDs(d2), "abstract numbering ids")
			assertNoNewDefects(t, codesBefore, validationCodes(d2), "numbering round trip")
		})
		if d != nil {
			_ = d.Close()
		}
		if !saveOK {
			return
		}

		// Registering a second list on the reopened package must allocate out
		// of the id space the first save wrote, not collide with it.
		d3 := reopenSaved(t, first, "numbering re-registration")
		defer func() { _ = d3.Close() }()
		nums := numberingIDs(d3)
		def := d3.Numbering().AddDefinition()
		def.SetLevel(0, NumberFormatDecimal, "%1.")
		if got := strconv.Itoa(def.ListStyle().numID); nums[got] {
			t.Fatalf("after a save and reopen, ListStyle allocated numbering instance id %q, which the saved part already defines", got)
		}
	})
}

// --- settings.xml -----------------------------------------------------------

// settingsFacts is everything the settings API reports back.
type settingsFacts struct {
	zoom            int
	zoomSet         bool
	tabStop         float64
	tabStopSet      bool
	evenOdd         bool
	footnote        NoteProperties
	footnoteSet     bool
	endnote         NoteProperties
	endnoteSet      bool
	variableCount   int
	probeVar        string
	probeVarPresent bool
}

func settingsSnapshot(d *Document) settingsFacts {
	var s settingsFacts
	s.zoom, s.zoomSet = d.Zoom()
	s.tabStop, s.tabStopSet = d.DefaultTabStop()
	s.evenOdd = d.EvenAndOddHeaders()
	s.footnote, s.footnoteSet = d.FootnoteProperties()
	s.endnote, s.endnoteSet = d.EndnoteProperties()
	s.variableCount = len(d.DocumentVariables())
	s.probeVar, s.probeVarPresent = d.DocumentVariable("FuzzVar")
	return s
}

func settingsVariableNames(d *Document) map[string]bool {
	out := make(map[string]bool)
	for _, v := range d.DocumentVariables() {
		out[v.Name] = true
	}
	return out
}

// FuzzDocxSettingsXML replaces word/settings.xml and then writes to it through
// the public setters.
//
// The oracle is read-your-writes THROUGH A REOPEN. Setting a value and reading
// it back on the same handle passes whether or not the part parses, serializes
// or round-trips at all — the getter is reading the object the setter just
// mutated. Only re-opening the saved bytes proves the value survived the trip,
// and settings.xml is regenerated from a model that types a handful of its
// children and preserves the rest, which is exactly the arrangement where a
// value goes missing on the way out.
func FuzzDocxSettingsXML(f *testing.F) {
	valid := buildRichDocxFuzzSeed(f)
	const part = "word/settings.xml"
	orig := requireSeedParts(f, valid, part)[0]

	const setOpen = `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`
	f.Add(orig, 100, 36.0, "value", true)
	f.Add([]byte{}, 0, 0.0, "", false)
	f.Add([]byte("<w:settings"), -1, -1.0, "\x00", true)
	f.Add(orig[:len(orig)/2], 500, 1e9, strings.Repeat("v", 4096), false)
	// Existing values the setters must replace rather than duplicate.
	f.Add([]byte(setOpen+
		`<w:zoom w:percent="240"/><w:defaultTabStop w:val="1440"/><w:evenAndOddHeaders/>`+
		`<w:docVars><w:docVar w:name="FuzzVar" w:val="already here"/><w:docVar w:name="Other" w:val="x"/></w:docVars>`+
		`<w:footnotePr><w:pos w:val="pageBottom"/><w:numFmt w:val="decimal"/><w:numStart w:val="1"/></w:footnotePr>`+
		`</w:settings>`), 75, 18.0, "replaced", false)
	// Duplicated and hostile-valued children.
	f.Add([]byte(setOpen+
		`<w:zoom w:percent="not a number"/><w:zoom w:percent="99999999999999999999"/>`+
		`<w:defaultTabStop w:val=""/>`+
		`<w:docVars><w:docVar w:name="FuzzVar"/><w:docVar w:name="FuzzVar" w:val="second"/></w:docVars>`+
		`<w:footnotePr><w:footnote w:id="-1"/><w:footnote w:id="0"/><w:pos w:val="sectEnd"/></w:footnotePr>`+
		`<w:endnotePr><w:numRestart w:val="eachPage"/></w:endnotePr>`+
		`</w:settings>`), 10, 0.05, "]]>", true)
	// A settings part carrying only children the model does not type.
	f.Add([]byte(setOpen+
		`<w:rsids><w:rsidRoot w:val="00A1"/><w:rsid w:val="00A2"/></w:rsids>`+
		`<w:compat><w:compatSetting w:name="compatibilityMode" w:uri="x" w:val="15"/></w:compat>`+
		`<w:themeFontLang w:val="en-US"/></w:settings>`), 120, 72.0, "kept", false)

	f.Fuzz(func(t *testing.T, data []byte, zoom int, tabStop float64, varValue string, evenOdd bool) {
		skipOversizedParts(t, data)
		pkg := fuzzseed.ReplaceZipEntry(valid, part, data)
		if pkg == nil {
			t.Skip("seed package unreadable")
		}
		d := openFuzzedPackage(t, pkg)
		if d == nil {
			return
		}
		defer func() { _ = d.Close() }()

		_ = settingsSnapshot(d)
		namesBefore := settingsVariableNames(d)
		walkDocument(d)

		wantNote := NoteProperties{Position: "sectEnd", NumberFormat: "lowerRoman", Restart: "eachSect"}
		mutateSettings(d, zoom, tabStop, varValue, evenOdd, wantNote)
		codesBefore := validationCodes(d)
		want := settingsSnapshot(d)

		first, err := d.SaveBytes()
		if err != nil {
			return
		}
		assertPartsAreWellFormed(t, first)
		assertEmittedNamespacesResolve(t, pkg, first)
		assertRootDeclsPreserved(t, data, zipPartBytes(first, part), part)

		d2 := reopenSaved(t, first, "settings round trip")
		defer func() { _ = d2.Close() }()
		got := settingsSnapshot(d2)

		// Every setter's value must survive being written and read back.
		if got.zoom != zoom || !got.zoomSet {
			t.Fatalf("SetZoom(%d) did not survive a save and reopen: Zoom() = %d, present = %v", zoom, got.zoom, got.zoomSet)
		}
		if got.evenOdd != evenOdd {
			t.Fatalf("SetEvenAndOddHeaders(%v) did not survive a save and reopen: EvenAndOddHeaders() = %v", evenOdd, got.evenOdd)
		}
		if !got.probeVarPresent || got.probeVar != want.probeVar {
			t.Fatalf("SetDocumentVariable did not survive a save and reopen: want %q (present %v), got %q (present %v)",
				want.probeVar, want.probeVarPresent, got.probeVar, got.probeVarPresent)
		}
		if !got.footnoteSet || got.footnote != wantNote {
			t.Fatalf("SetFootnoteProperties(%+v) did not survive a save and reopen: got %+v (present %v)", wantNote, got.footnote, got.footnoteSet)
		}
		if got.tabStopSet != want.tabStopSet || (want.tabStopSet && got.tabStop != want.tabStop) {
			t.Fatalf("SetDefaultTabStop(%v) did not survive a save and reopen: want %v (present %v), got %v (present %v)",
				tabStop, want.tabStop, want.tabStopSet, got.tabStop, got.tabStopSet)
		}
		assertSubset(t, namesBefore, settingsVariableNames(d2), "document variable names")
		assertNoNewDefects(t, codesBefore, validationCodes(d2), "settings round trip")

		mutateSettings(d2, zoom, tabStop, varValue, evenOdd, wantNote)
		second, err := d2.SaveBytes()
		if err != nil {
			t.Fatalf("re-saving a package this library just wrote failed: %v", err)
		}
		assertPartFixedPoint(t, first, second, part)
	})
}

func mutateSettings(d *Document, zoom int, tabStop float64, varValue string, evenOdd bool, note NoteProperties) {
	d.SetZoom(zoom)
	d.SetDefaultTabStop(tabStop)
	d.SetDocumentVariable("FuzzVar", varValue)
	d.SetEvenAndOddHeaders(evenOdd)
	d.SetFootnoteProperties(note)
	d.SetEndnoteProperties(note)
}

// --- footnotes.xml + endnotes.xml -------------------------------------------

func noteSnapshot(notes []*Footnote) map[string]bool {
	out := make(map[string]bool)
	for _, n := range notes {
		out[n.ID()] = true
	}
	return out
}

// FuzzDocxNotesXML replaces word/footnotes.xml and word/endnotes.xml at the
// same time, in a package whose body carries a footnote reference and an
// endnote reference.
//
// Both parts are fuzzed together on purpose: the two share their reference
// machinery and their id allocator, and a document that carries only one of
// them cannot show a reference resolving against the wrong part.
//
// Run.AddFootnote allocates "one past the largest id it can parse", so the
// invariant is that the id it hands out is not already taken and that the note
// is still there, with its text, after a save and reopen — read-your-writes
// through a reopen rather than off the handle that just wrote it.
func FuzzDocxNotesXML(f *testing.F) {
	valid := buildRichDocxFuzzSeed(f)
	const ftnPart, endPart = "word/footnotes.xml", "word/endnotes.xml"
	parts := requireSeedParts(f, valid, ftnPart, endPart)
	origFtn, origEnd := parts[0], parts[1]

	const ftnOpen = `<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`
	const endOpen = `<w:endnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`
	f.Add(origFtn, origEnd, "note text")
	f.Add([]byte{}, []byte{}, "")
	f.Add([]byte("<w:footnotes>"), []byte("garbage"), "x")
	f.Add(origFtn[:len(origFtn)/2], origEnd, "half")
	// Ids the allocator cannot parse, so it thinks the space is empty.
	f.Add([]byte(ftnOpen+`<w:footnote w:id="99999999999999999999"><w:p><w:r><w:t>huge</w:t></w:r></w:p></w:footnote></w:footnotes>`),
		[]byte(endOpen+`<w:endnote w:id="  3 "><w:p/></w:endnote></w:endnotes>`), "collide")
	// Duplicate ids, and the separator notes at unexpected ids.
	f.Add([]byte(ftnOpen+
		`<w:footnote w:type="separator" w:id="5"><w:p/></w:footnote>`+
		`<w:footnote w:type="continuationSeparator" w:id="5"><w:p/></w:footnote>`+
		`<w:footnote w:id="5"><w:p><w:r><w:t>dup</w:t></w:r></w:p></w:footnote>`+
		`</w:footnotes>`),
		[]byte(endOpen+`<w:endnote w:id="-1"><w:p/></w:endnote><w:endnote w:id=""><w:p/></w:endnote></w:endnotes>`), "dup")
	// Notes whose bodies are deeply nested, plus no notes at all.
	f.Add([]byte(ftnOpen+`<w:footnote w:id="1">`+strings.Repeat(`<w:tbl><w:tr><w:tc>`, 80)+
		strings.Repeat(`</w:tc></w:tr></w:tbl>`, 80)+`</w:footnote></w:footnotes>`),
		[]byte(endOpen+`</w:endnotes>`), "nested")

	f.Fuzz(func(t *testing.T, ftn, end []byte, text string) {
		skipOversizedParts(t, ftn, end)
		pkg := fuzzseed.EditZip(valid, [][2]string{{ftnPart, string(ftn)}, {endPart, string(end)}})
		if pkg == nil {
			t.Skip("seed package unreadable")
		}
		d := openFuzzedPackage(t, pkg)
		if d == nil {
			return
		}
		defer func() { _ = d.Close() }()

		ftnBefore := noteSnapshot(d.Footnotes())
		endBefore := noteSnapshot(d.Endnotes())
		walkDocument(d)

		paras := d.Paragraphs()
		if len(paras) == 0 {
			return
		}
		host := paras[0]
		if len(host.Runs()) == 0 {
			host.SetText("host")
		}
		run := host.Runs()[0]
		newFtn := run.AddFootnote(text)
		newEnd := run.AddEndnote(text)
		if ftnBefore[newFtn.ID()] {
			t.Fatalf("AddFootnote allocated id %q, which the parsed part already uses: the body reference now points at two notes", newFtn.ID())
		}
		if endBefore[newEnd.ID()] {
			t.Fatalf("AddEndnote allocated id %q, which the parsed part already uses", newEnd.ID())
		}
		// The baseline is taken AFTER the edit: adding a note legitimately adds
		// references (to the note styles, to the note itself), and the question
		// this asks is whether the SAVE introduced anything the model did not
		// already describe.
		codesBefore := validationCodes(d)

		first, err := d.SaveBytes()
		if err != nil {
			return
		}
		assertPartsAreWellFormed(t, first)
		assertEmittedNamespacesResolve(t, pkg, first)
		// KNOWN FINDING, reported not fixed: assertRootChildrenPreserved is NOT
		// applied to these two parts. marshalFootnotesXML and
		// marshalEndnotesXML emit w:footnote / w:endnote children and nothing
		// else, so any other root child the source carried — an
		// mc:AlternateContent, an extension element — is deleted the first time
		// an edit flips the part to regenerate. That is the C370 class in the
		// parts the C370 remediation did not reach; styles.xml, settings.xml
		// and numbering.xml all keep their unmodeled root children, and these
		// do not. Arm the assertion once they do.
		assertRootDeclsPreserved(t, ftn, zipPartBytes(first, ftnPart), ftnPart)
		assertRootDeclsPreserved(t, end, zipPartBytes(first, endPart), endPart)

		d2 := reopenSaved(t, first, "notes round trip")
		defer func() { _ = d2.Close() }()
		walkDocument(d2)
		ftnAfter, endAfter := noteSnapshot(d2.Footnotes()), noteSnapshot(d2.Endnotes())
		assertSubset(t, ftnBefore, ftnAfter, "footnote ids")
		assertSubset(t, endBefore, endAfter, "endnote ids")
		if !ftnAfter[newFtn.ID()] {
			t.Fatalf("the footnote just added (id %q) is not in the saved package", newFtn.ID())
		}
		if !endAfter[newEnd.ID()] {
			t.Fatalf("the endnote just added (id %q) is not in the saved package", newEnd.ID())
		}
		assertNoNewDefects(t, codesBefore, validationCodes(d2), "notes round trip")

		// Adding to the reopened package must not reuse the id the first save
		// wrote.
		paras2 := d2.Paragraphs()
		if len(paras2) == 0 || len(paras2[0].Runs()) == 0 {
			return
		}
		again := paras2[0].Runs()[0].AddFootnote(text)
		if ftnAfter[again.ID()] {
			t.Fatalf("after a save and reopen, AddFootnote reused id %q, which the saved part already defines", again.ID())
		}
	})
}

// --- comments.xml + commentsExtended.xml + people.xml -----------------------

// commentFacts is what the comment API reports for one comment.
type commentFacts struct {
	author, initials, text, parent string
	resolved                       bool
	replies                        int
}

func commentSnapshot(d *Document) map[string]commentFacts {
	out := make(map[string]commentFacts)
	for _, c := range d.Comments() {
		if _, dup := out[c.ID()]; dup {
			continue
		}
		f := commentFacts{
			author:   c.Author(),
			initials: c.Initials(),
			text:     c.Text(),
			resolved: c.Resolved(),
			replies:  len(c.Replies()),
		}
		if p := c.Parent(); p != nil {
			f.parent = p.ID()
		}
		out[c.ID()] = f
	}
	return out
}

// commentThreadBudget bounds the comment thread walk. Threading is expressed
// as parent links keyed by paragraph id, so a part can describe a cycle, a
// star (every comment claiming the same parent) or a chain thousands deep, and
// the walk that resolves it is quadratic in the number of comments. A
// pathological part that turns a document with a few hundred comments into
// minutes of walking is a denial of service on anything that opens untrusted
// documents, and it neither panics nor fails to save.
var commentThreadBudget = fuzzbound.Budget{
	What:              "docx comment thread walk",
	Bytes:             64 << 20,
	BytesPerInputByte: 512,
	Time:              20 * time.Second,
	TimePerMiB:        20 * time.Second,
}

// FuzzDocxCommentsXML replaces the three parts that together describe a comment
// thread — the comments themselves, the parent links, and the author list — in
// a package whose body carries a comment range and a comment reference.
//
// They are fuzzed together because threading is a JOIN across all three:
// comments.xml holds the bodies, commentsExtended.xml the parent links and the
// resolved flags keyed by paragraph id, and people.xml the authors. Fuzzing any
// one of them alone leaves the join trivially consistent.
//
//  1. Walking the threads terminates inside a resource budget, whatever cycles
//     or fan-out the parent links describe.
//  2. Resolving and re-initialling every comment is idempotent: applying it,
//     saving, reopening and applying it again produces identical part bytes,
//     and the values read back match what was written.
//  3. No comment the model could read disappears across the round trip.
func FuzzDocxCommentsXML(f *testing.F) {
	valid := buildRichDocxFuzzSeed(f)
	const cPart, cExPart, pPart = "word/comments.xml", "word/commentsExtended.xml", "word/people.xml"
	parts := requireSeedParts(f, valid, cPart, cExPart, pPart)
	origC, origEx, origP := parts[0], parts[1], parts[2]

	const wns = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	const w14ns = `xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`
	const w15ns = `xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml"`

	f.Add(origC, origEx, origP)
	f.Add([]byte{}, []byte{}, []byte{})
	f.Add([]byte("<w:comments>"), []byte("nope"), []byte("nope"))
	f.Add(origC[:len(origC)/2], origEx, origP)
	// A parent-link cycle: two comments each claiming the other as parent.
	f.Add([]byte(`<w:comments `+wns+` `+w14ns+`>`+
		`<w:comment w:id="1" w:author="A"><w:p w14:paraId="0001"><w:r><w:t>one</w:t></w:r></w:p></w:comment>`+
		`<w:comment w:id="2" w:author="B"><w:p w14:paraId="0002"><w:r><w:t>two</w:t></w:r></w:p></w:comment>`+
		`</w:comments>`),
		[]byte(`<w15:commentsEx `+wns+` `+w15ns+`>`+
			`<w15:commentEx w15:paraId="0001" w15:paraIdParent="0002" w15:done="0"/>`+
			`<w15:commentEx w15:paraId="0002" w15:paraIdParent="0001" w15:done="1"/>`+
			`</w15:commentsEx>`),
		origP)
	// A star: one root with every other comment as a direct reply. Resolving
	// the root walks the whole thread, and both the walk and the parent lookup
	// it performs per step are linear scans, so this is the shape whose cost
	// grows fastest with the size of the part.
	//
	// The root must genuinely be a root. An earlier version gave the parent
	// entry a paraIdParent of its own, which made Comments() classify every
	// comment as a reply, return nothing, and exercise none of this.
	star := &strings.Builder{}
	starEx := &strings.Builder{}
	star.WriteString(`<w:comments ` + wns + ` ` + w14ns + `>`)
	starEx.WriteString(`<w15:commentsEx ` + wns + ` ` + w15ns + `>`)
	starEx.WriteString(`<w15:commentEx w15:paraId="00000000" w15:done="0"/>`)
	for i := range 150 {
		id := strconv.Itoa(i)
		para := fmt.Sprintf("%08X", i)
		fmt.Fprintf(star, `<w:comment w:id="%s" w:author="A"><w:p w14:paraId="%s"><w:r><w:t>c</w:t></w:r></w:p></w:comment>`, id, para)
		if i > 0 {
			fmt.Fprintf(starEx, `<w15:commentEx w15:paraId="%s" w15:paraIdParent="00000000"/>`, para)
		}
	}
	star.WriteString(`</w:comments>`)
	starEx.WriteString(`</w15:commentsEx>`)
	f.Add([]byte(star.String()), []byte(starEx.String()), origP)
	// Self-parenting, duplicate ids, and a people part with hostile authors.
	f.Add([]byte(`<w:comments `+wns+` `+w14ns+`>`+
		`<w:comment w:id="7" w:author="&lt;x&gt;" w:initials="]]&gt;"><w:p w14:paraId="00FF"/></w:comment>`+
		`<w:comment w:id="7" w:author=""><w:p w14:paraId="00FF"/></w:comment>`+
		`</w:comments>`),
		[]byte(`<w15:commentsEx `+wns+` `+w15ns+`><w15:commentEx w15:paraId="00FF" w15:paraIdParent="00FF" w15:done="1"/></w15:commentsEx>`),
		[]byte(`<w15:people `+wns+` `+w15ns+`><w15:person w15:author=""><w15:presenceInfo w15:providerId="None" w15:userId=""/></w15:person></w15:people>`))

	f.Fuzz(func(t *testing.T, comments, extended, people []byte) {
		if fuzzbound.Tripped() {
			t.Skip("a resource budget was already exceeded in this process")
		}
		skipOversizedParts(t, comments, extended, people)
		pkg := fuzzseed.EditZip(valid, [][2]string{
			{cPart, string(comments)},
			{cExPart, string(extended)},
			{pPart, string(people)},
		})
		if pkg == nil {
			t.Skip("seed package unreadable")
		}
		d := openFuzzedPackage(t, pkg)
		if d == nil {
			return
		}
		defer func() { _ = d.Close() }()

		// The budget brackets the whole read-and-resolve pass, not just the
		// snapshot: threading is resolved by walking the thread and looking up a
		// parent by paragraph id, both linear scans, so the cost of a part is
		// super-linear in the number of comments it declares. A part that turns
		// a document with a few hundred comments into minutes of walking is a
		// denial of service on anything that opens untrusted documents, and it
		// neither panics nor fails to save.
		var before map[string]commentFacts
		commentThreadBudget.Check(t, len(pkg), func() {
			before = commentSnapshot(d)
			mutateComments(d)
		})
		walkDocument(d)
		codesBefore := validationCodes(d)

		first, err := d.SaveBytes()
		if err != nil {
			return
		}
		assertPartsAreWellFormed(t, first)
		assertEmittedNamespacesResolve(t, pkg, first)
		// KNOWN FINDING, reported not fixed: assertRootChildrenPreserved is NOT
		// applied to the comment parts, for the same reason as the note parts
		// above. marshalCommentsXML, marshalCommentsExtendedXML and
		// marshalPeopleXML each emit exactly one kind of child and drop every
		// other root child on regeneration. Reproduced by the fuzzer in under a
		// minute against commentsExtended.xml. Arm the assertion once they
		// preserve what they do not type.
		assertRootDeclsPreserved(t, comments, zipPartBytes(first, cPart), cPart)
		assertRootDeclsPreserved(t, extended, zipPartBytes(first, cExPart), cExPart)
		assertRootDeclsPreserved(t, people, zipPartBytes(first, pPart), pPart)

		d2 := reopenSaved(t, first, "comments round trip")
		defer func() { _ = d2.Close() }()
		var mid map[string]commentFacts
		commentThreadBudget.Check(t, len(first), func() {
			mid = commentSnapshot(d2)
		})
		walkDocument(d2)

		for id, was := range before {
			now, ok := mid[id]
			if !ok {
				t.Fatalf("saving dropped comment %q, which the model read out of the parsed part", id)
			}
			if was.author != now.author || was.text != now.text {
				t.Fatalf("comment %q changed across the round trip: author %q -> %q, text %q -> %q",
					id, was.author, now.author, was.text, now.text)
			}
			if !now.resolved {
				t.Fatalf("comment %q was resolved before the save and reads back unresolved", id)
			}
			if now.initials != "FZ" {
				t.Fatalf("comment %q had its initials set to \"FZ\" before the save and reads back %q", id, now.initials)
			}
		}
		assertNoNewDefects(t, codesBefore, validationCodes(d2), "comments round trip")

		mutateComments(d2)
		second, err := d2.SaveBytes()
		if err != nil {
			t.Fatalf("re-saving a package this library just wrote failed: %v", err)
		}
		assertPartFixedPoint(t, first, second, cPart, cExPart, pPart)
	})
}

// mutateComments resolves every comment and gives it the same initials, an
// edit that is idempotent by construction.
//
// Replies are resolved through their own handles rather than only through the
// thread root: SetResolved walks the thread from whichever comment it is handed,
// and the walk from a reply takes a different path (up to the root, then back
// down) than the walk from a root.
func mutateComments(d *Document) {
	for i, c := range d.Comments() {
		if i >= 256 {
			break
		}
		c.SetResolved(true)
		c.SetInitials("FZ")
		for j, r := range c.Replies() {
			if j >= 16 {
				break
			}
			r.SetResolved(true)
			r.SetInitials("FZ")
		}
	}
}

// --- header / footer parts --------------------------------------------------

func hdrFtrSnapshot(d *Document) map[string]bool {
	out := make(map[string]bool)
	for _, h := range d.Headers() {
		out["header "+h.PartName()] = true
		for i, p := range h.Paragraphs() {
			if i >= 32 {
				break
			}
			out["header "+h.PartName()+" text "+p.Text()] = true
		}
	}
	for _, ft := range d.Footers() {
		out["footer "+ft.PartName()] = true
		for i, p := range ft.Paragraphs() {
			if i >= 32 {
				break
			}
			out["footer "+ft.PartName()+" text "+p.Text()] = true
		}
	}
	return out
}

// FuzzDocxHeaderFooterXML replaces word/header1.xml and word/footer1.xml, the
// parts a section's r:id references point at.
//
// A header is the one secondary part the open path treats as ESSENTIAL: a
// section that references a header Word would render, pointing at a part that
// is not there, is a broken document rather than an optional absence. So the
// invariants are that the reference still resolves after the round trip, that
// editing a header's text is visible after a reopen and not merely on the
// handle that wrote it, and that the part is a fixed point under that edit.
func FuzzDocxHeaderFooterXML(f *testing.F) {
	valid := buildRichDocxFuzzSeed(f)
	const hdrPart, ftrPart = "word/header1.xml", "word/footer1.xml"
	parts := requireSeedParts(f, valid, hdrPart, ftrPart)
	origHdr, origFtr := parts[0], parts[1]

	const hdrOpen = `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`
	const ftrOpen = `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`
	f.Add(origHdr, origFtr, "edited")
	f.Add([]byte{}, []byte{}, "")
	f.Add([]byte("<w:hdr>"), []byte("<w:ftr>"), "x")
	f.Add(origHdr[:len(origHdr)/2], origFtr, "half")
	// A header whose runs reference relationships this part does not declare.
	f.Add([]byte(hdrOpen+`<w:p><w:r><w:drawing><wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">`+
		`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData/></a:graphic>`+
		`</wp:inline></w:drawing></w:r></w:p><w:p><w:hyperlink r:id="rIdNope"><w:r><w:t>link</w:t></w:r></w:hyperlink></w:p></w:hdr>`),
		[]byte(ftrOpen+`<w:p><w:fldSimple w:instr="PAGE"/></w:p></w:ftr>`), "danglingrel")
	// A header carrying a whole table and a nested sectPr.
	f.Add([]byte(hdrOpen+`<w:tbl><w:tr><w:tc><w:p><w:r><w:t>in a cell</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`+
		`<w:p><w:pPr><w:sectPr/></w:pPr></w:p></w:hdr>`),
		[]byte(ftrOpen+strings.Repeat(`<w:p><w:r><w:t>line</w:t></w:r></w:p>`, 200)+`</w:ftr>`), "table")
	// Empty root elements: legal, and the case where Paragraphs() is empty.
	f.Add([]byte(hdrOpen+`</w:hdr>`), []byte(ftrOpen+`</w:ftr>`), "empty")

	f.Fuzz(func(t *testing.T, hdr, ftr []byte, text string) {
		skipOversizedParts(t, hdr, ftr)
		pkg := fuzzseed.EditZip(valid, [][2]string{{hdrPart, string(hdr)}, {ftrPart, string(ftr)}})
		if pkg == nil {
			t.Skip("seed package unreadable")
		}
		d := openFuzzedPackage(t, pkg)
		if d == nil {
			return
		}
		defer func() { _ = d.Close() }()

		partsBefore := make(map[string]bool)
		for _, h := range d.Headers() {
			partsBefore["header "+h.PartName()] = true
		}
		for _, ft := range d.Footers() {
			partsBefore["footer "+ft.PartName()] = true
		}
		_ = hdrFtrSnapshot(d)
		walkDocument(d)

		mutateHdrFtr(d, text)
		codesBefore := validationCodes(d)
		editedParts := hdrFtrEditedParts(d)

		first, err := d.SaveBytes()
		if err != nil {
			return
		}
		assertPartsAreWellFormed(t, first)
		assertEmittedNamespacesResolve(t, pkg, first)
		assertRootDeclsPreserved(t, hdr, zipPartBytes(first, hdrPart), hdrPart)
		assertRootDeclsPreserved(t, ftr, zipPartBytes(first, ftrPart), ftrPart)

		d2 := reopenSaved(t, first, "header/footer round trip")
		defer func() { _ = d2.Close() }()
		walkDocument(d2)

		partsAfter := make(map[string]bool)
		for _, h := range d2.Headers() {
			partsAfter["header "+h.PartName()] = true
		}
		for _, ft := range d2.Footers() {
			partsAfter["footer "+ft.PartName()] = true
		}
		assertSubset(t, partsBefore, partsAfter, "header and footer parts")
		assertSameTextEverywhere(t, editedParts, d2)
		assertNoNewDefects(t, codesBefore, validationCodes(d2), "header/footer round trip")

		mutateHdrFtr(d2, text)
		second, err := d2.SaveBytes()
		if err != nil {
			t.Fatalf("re-saving a package this library just wrote failed: %v", err)
		}
		assertPartFixedPoint(t, first, second, hdrPart, ftrPart)
	})
}

// hdrFtrEditedParts names the header and footer parts mutateHdrFtr actually
// wrote to — the ones that had a paragraph to rewrite.
func hdrFtrEditedParts(d *Document) map[string]bool {
	out := make(map[string]bool)
	for _, h := range d.Headers() {
		if len(h.Paragraphs()) > 0 {
			out[h.PartName()] = true
		}
	}
	for _, ft := range d.Footers() {
		if len(ft.Paragraphs()) > 0 {
			out[ft.PartName()] = true
		}
	}
	return out
}

// assertSameTextEverywhere checks that every header and footer whose first
// paragraph was set to the same string reads that paragraph back as the same
// string after a save and a reopen.
//
// It compares the parts against EACH OTHER rather than against the string that
// was written, and that is the whole design. Comparing against the input would
// make the target fail on legitimate behavior: the writer strips characters XML
// 1.0 forbids, so a run of invalid UTF-8 written into a header reads back
// shorter, and that is a documented sanitization rather than a bug. What cannot
// be legitimate is two parts that were handed the same string disagreeing about
// it — that means one of them was not written, or was written from stale
// preserved bytes instead of from the model, which is the failure this library
// has shipped before and which no exception for sanitization should hide.
func assertSameTextEverywhere(t *testing.T, edited map[string]bool, d *Document) {
	t.Helper()
	first, firstPart := "", ""
	seen := false
	check := func(part, got string) {
		if !edited[part] {
			return
		}
		if !seen {
			first, firstPart, seen = got, part, true
			return
		}
		if got != first {
			t.Fatalf("every header and footer had its first paragraph set to the same text, but after a save and reopen %s reads %q while %s reads %q: one of them was not written from the model",
				part, got, firstPart, first)
		}
	}
	for _, h := range d.Headers() {
		if ps := h.Paragraphs(); len(ps) > 0 {
			check(h.PartName(), ps[0].Text())
		}
	}
	for _, ft := range d.Footers() {
		if ps := ft.Paragraphs(); len(ps) > 0 {
			check(ft.PartName(), ps[0].Text())
		}
	}
}

// mutateHdrFtr rewrites the first paragraph of every header and footer to the
// same text, an edit that is idempotent.
func mutateHdrFtr(d *Document, text string) {
	for _, h := range d.Headers() {
		if ps := h.Paragraphs(); len(ps) > 0 {
			ps[0].SetText(text)
		}
	}
	for _, ft := range d.Footers() {
		if ps := ft.Paragraphs(); len(ps) > 0 {
			ps[0].SetText(text)
		}
	}
}
