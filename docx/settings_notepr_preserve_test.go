package docx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// settingsWith wraps children in a settings part bound to the WML namespace,
// plus a second namespace for the re-homing case.
func settingsWith(children string) []byte {
	return []byte(`<w:settings ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:v="urn:schemas-microsoft-com:vml">` + children + `</w:settings>`)
}

// openWithSettings builds a package whose settings part is the given bytes.
func openWithSettings(t *testing.T, settings []byte) *Document {
	t.Helper()
	pkg := fuzzseed.ReplaceZipEntry(buildRichDocxFuzzSeed(t), "word/settings.xml", settings)
	if pkg == nil {
		t.Fatal("could not build the fixture package")
	}
	d, err := OpenReader(strings.NewReader(string(pkg)), int64(len(pkg)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return d
}

// emittedSettings rewrites the note properties — the mutation that regenerates
// w:footnotePr and replays the children it does not own — and returns the part.
func emittedSettings(t *testing.T, d *Document) string {
	t.Helper()
	d.SetFootnoteProperties(NoteProperties{Position: "sectEnd", NumberFormat: "lowerRoman", Restart: "eachSect"})
	out, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return string(zipPartBytes(out, "word/settings.xml"))
}

// A preserved child whose local name cannot simply be pasted after a prefix must
// not be rebuilt as one. <:pos/> is reported by the decoder with the local name
// ":pos"; concatenating "w:" with it produced <w::pos/>, which has two colons,
// is not a QName, and made the emitted part unparseable — the library writing a
// settings part it cannot read back (FuzzDocxSettingsXML).
func TestNotePropsPreservesAnAwkwardChildNameVerbatim(t *testing.T) {
	d := openWithSettings(t, settingsWith(
		`<w:footnotePr><:pos w:val="pageBottom"/><w:numFmt w:val="decimal"/></w:footnotePr>`))
	defer func() { _ = d.Close() }()

	got := emittedSettings(t, d)
	if strings.Contains(got, "w::pos") {
		t.Errorf("the preserved child was rebuilt into a name with two colons:\n%s", got)
	}
	if !strings.Contains(got, `<:pos w:val="pageBottom"/>`) {
		t.Errorf("the preserved child was not carried across verbatim:\n%s", got)
	}
	// The rewrite this mutation exists to perform still happened.
	if !strings.Contains(got, `<w:pos w:val="sectEnd"/>`) {
		t.Errorf("the numbering rewrite did not take:\n%s", got)
	}
}

// A preserved child in another namespace must keep it. Forcing "w:" onto every
// child moved it into WordprocessingML — the same silent re-homing the pptx
// comment parts had, reached by a different route.
func TestNotePropsDoesNotRehomeAForeignChild(t *testing.T) {
	d := openWithSettings(t, settingsWith(
		`<w:footnotePr><v:custom v:flag="1"/><w:numFmt w:val="decimal"/></w:footnotePr>`))
	defer func() { _ = d.Close() }()

	got := emittedSettings(t, d)
	if strings.Contains(got, "<w:custom") {
		t.Errorf("a child in the VML namespace was re-homed into WordprocessingML:\n%s", got)
	}
	if !strings.Contains(got, `<v:custom v:flag="1"/>`) {
		t.Errorf("the foreign-namespace child was not preserved:\n%s", got)
	}
}

// A preserved child with content keeps it, rather than being flattened to an
// empty tag by a rebuild that only ever looked at the start element.
func TestNotePropsPreservesChildContent(t *testing.T) {
	d := openWithSettings(t, settingsWith(
		`<w:footnotePr><w:footnote w:id="1"><w:inner w:val="kept"/></w:footnote></w:footnotePr>`))
	defer func() { _ = d.Close() }()

	got := emittedSettings(t, d)
	// The nesting is the assertion, not the mere presence of the inner element:
	// the rebuild this replaced walked every start element in the subtree and
	// emitted each as an empty tag, so the child's content came out as its
	// *sibling*. Checking only that <w:inner> appears somewhere passes on both.
	if !strings.Contains(got, `<w:footnote w:id="1"><w:inner w:val="kept"/></w:footnote>`) {
		t.Errorf("the preserved child's subtree was flattened rather than carried across:\n%s", got)
	}
}
