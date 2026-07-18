package xlsx

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// openBytes opens a workbook from an in-memory xlsx package.
func openBytes(t *testing.T, data []byte) (*Workbook, error) {
	t.Helper()
	return OpenReader(bytes.NewReader(data), int64(len(data)))
}

// TestReadThreadedComments reads the crafted fixture carrying a legacy note,
// a threaded thread (root + reply) and a person list, and asserts the unified
// view.
func TestReadThreadedComments(t *testing.T) {
	wb, err := Open("testdata/threaded_comments.xlsx")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = wb.Close() }()

	s, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	comments := s.Comments()
	if len(comments) != 1 {
		t.Fatalf("want 1 top-level comment (legacy shadowed by thread), got %d", len(comments))
	}
	c := comments[0]
	if !c.Threaded() {
		t.Errorf("want threaded")
	}
	if c.Ref() != "A1" {
		t.Errorf("ref = %q, want A1", c.Ref())
	}
	if c.Author() != "Jane Doe" {
		t.Errorf("author = %q, want Jane Doe", c.Author())
	}
	if c.Text() != "First comment" {
		t.Errorf("text = %q, want First comment", c.Text())
	}
	if c.ID() == "" {
		t.Errorf("threaded comment should have an ID")
	}
	if c.Date().IsZero() {
		t.Errorf("threaded comment should have a date")
	}
	if got := len(c.Replies()); got != 1 {
		t.Fatalf("replies = %d, want 1", got)
	}
	reply := c.Replies()[0]
	if reply.Author() != "John Smith" || reply.Text() != "A reply" {
		t.Errorf("reply = %q/%q", reply.Author(), reply.Text())
	}
	if reply.Parent() != c {
		t.Errorf("reply parent not linked to root")
	}

	// Cell.Comment reaches the same comment.
	cell, _ := s.Cell("A1")
	if cell.Comment() == nil || cell.Comment().Ref() != "A1" {
		t.Errorf("Cell.Comment() did not find the A1 comment")
	}
}

// TestReadLegacyComments reads a real Excel/excelize file whose only comment
// mechanism is a legacy note.
func TestReadLegacyComments(t *testing.T) {
	path := "testdata/external/excelize_test.xlsx"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("external fixture absent")
	}
	wb, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wb.Close() }()
	s, _ := wb.Sheet(0)
	comments := s.Comments()
	if len(comments) != 1 {
		t.Fatalf("want 1 legacy comment, got %d", len(comments))
	}
	c := comments[0]
	if c.Threaded() {
		t.Errorf("legacy note should not be threaded")
	}
	if c.Ref() != "A22" {
		t.Errorf("ref = %q, want A22", c.Ref())
	}
	if !c.Date().IsZero() {
		t.Errorf("legacy note should have zero date")
	}
	if c.Author() != "Microsoft Office User" {
		t.Errorf("author = %q", c.Author())
	}
}

// TestCommentsRoundTripByteIdentical asserts a zero-modification open→save of a
// comment-bearing workbook preserves every part byte-for-byte.
func TestCommentsRoundTripByteIdentical(t *testing.T) {
	for _, name := range []string{"threaded_comments.xlsx"} {
		t.Run(name, func(t *testing.T) {
			src := filepath.Join("testdata", name)
			wb, err := Open(src)
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), name)
			if err := wb.Save(out); err != nil {
				t.Fatal(err)
			}
			_ = wb.Close()

			missing, extra, changed := testutil.CompareZipFiles(t, src, out)
			if len(missing) > 0 || len(extra) > 0 || len(changed) > 0 {
				t.Errorf("not byte-identical: missing=%v extra=%v changed=%v", missing, extra, changed)
			}
		})
	}
}

// TestAddThreadedComment adds a comment, a reply and resolves the thread, then
// reopens and asserts the threaded part, person list, legacy fallback and
// worksheet references are all present and correct.
func TestAddThreadedComment(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := wb.Sheet(0)
	cell, _ := s.Cell("C3")
	c := cell.AddComment("Ada Lovelace", "Look at this cell")
	if c == nil || !c.Threaded() {
		t.Fatal("AddComment returned nil or non-threaded")
	}
	reply := c.Reply("Alan Turing", "Interesting indeed")
	if reply == nil {
		t.Fatal("Reply returned nil")
	}
	c.Resolve()

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	_ = wb.Close()

	// Parts exist.
	for _, part := range []string{
		"xl/comments1.xml",
		"xl/drawings/vmlDrawing1.vml",
		"xl/threadedComments/threadedComment1.xml",
		"xl/persons/person1.xml",
	} {
		if !zipHasPart(t, data, part) {
			t.Errorf("missing part %s", part)
		}
	}
	// Worksheet gained a legacyDrawing reference.
	ws := readZipPart(t, data, "xl/worksheets/sheet1.xml")
	if !bytes.Contains(ws, []byte("<legacyDrawing")) {
		t.Errorf("worksheet missing <legacyDrawing>")
	}
	// Workbook gained a person relationship.
	wbRels := readZipPart(t, data, "xl/_rels/workbook.xml.rels")
	if !bytes.Contains(wbRels, []byte("relationships/person")) {
		t.Errorf("workbook rels missing person relationship")
	}

	// Reopen and verify the model.
	wb2, err := openBytes(t, data)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = wb2.Close() }()
	if rep := wb2.Validate(); len(rep) != 0 {
		t.Errorf("validate findings after add: %v", rep)
	}
	s2, _ := wb2.Sheet(0)
	comments := s2.Comments()
	if len(comments) != 1 {
		t.Fatalf("want 1 comment after reopen, got %d", len(comments))
	}
	got := comments[0]
	if got.Author() != "Ada Lovelace" || got.Text() != "Look at this cell" {
		t.Errorf("root = %q/%q", got.Author(), got.Text())
	}
	if !got.Resolved() {
		t.Errorf("thread should be resolved")
	}
	if len(got.Replies()) != 1 || got.Replies()[0].Author() != "Alan Turing" {
		t.Errorf("reply not persisted: %+v", got.Replies())
	}
}

// TestAddCommentPreservesExistingDrawing verifies that adding a comment to a
// sheet that already has an image/drawing does not clobber the drawing: the
// DrawingML drawing (images) and the VML/legacy drawing (comments) coexist.
func TestAddCommentPreservesExistingDrawing(t *testing.T) {
	path := "testdata/external/excelize_test.xlsx"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("external fixture absent")
	}
	wb, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := wb.Sheet(0)
	before := len(s.Comments())
	s.AddComment("E5", "Reviewer", "Please check")

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	_ = wb.Close()

	// Existing image drawing + media survive alongside the new comment parts.
	for _, part := range []string{
		"xl/drawings/drawing1.xml",
		"xl/media/image1.jpeg",
		"xl/threadedComments/threadedComment1.xml",
		"xl/drawings/vmlDrawing1.vml",
	} {
		if !zipHasPart(t, data, part) {
			t.Errorf("missing part %s", part)
		}
	}

	wb2, err := openBytes(t, data)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = wb2.Close() }()
	if rep := wb2.Validate(); len(rep) != 0 {
		t.Errorf("validate findings: %v", rep)
	}
	s2, _ := wb2.Sheet(0)
	if got := len(s2.Comments()); got != before+1 {
		t.Errorf("comment count = %d, want %d", got, before+1)
	}
}

// TestAddNote adds a legacy-only note (no threaded comment or person entry).
func TestAddNote(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := wb.Sheet(0)
	s.AddNote("B2", "Noter", "Just a note")

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	_ = wb.Close()

	if !zipHasPart(t, data, "xl/comments1.xml") {
		t.Errorf("missing comments part")
	}
	if zipHasPart(t, data, "xl/persons/person1.xml") {
		t.Errorf("legacy note should not create a person list")
	}

	wb2, err := openBytes(t, data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wb2.Close() }()
	s2, _ := wb2.Sheet(0)
	comments := s2.Comments()
	if len(comments) != 1 || comments[0].Threaded() || comments[0].Text() != "Just a note" {
		t.Fatalf("note not read back correctly: %+v", comments)
	}
}

// TestValidateCommentPersonOrphan asserts the optional validate check flags a
// threaded comment whose personId has no matching person.
func TestValidateCommentPersonOrphan(t *testing.T) {
	w := &Workbook{}
	w.persons = &oxml.CT_PersonList{Persons: []oxml.CT_Person{{DisplayName: "Known", ID: "{good}"}}}
	w.personsLoaded = true
	s := &Sheet{workbook: w}
	s.comments = &sheetComments{
		loaded:       true,
		threadedPart: "/xl/threadedComments/threadedComment1.xml",
		threaded: &oxml.CT_ThreadedComments{Comments: []oxml.CT_ThreadedComment{
			{Ref: "A1", PersonID: "{good}", ID: "{c1}"},
			{Ref: "A2", PersonID: "{missing}", ID: "{c2}"},
		}},
	}
	w.sheets = []*Sheet{s}

	c := validate.New()
	w.validateComments(c)
	rep := c.Report()
	if len(rep) != 1 {
		t.Fatalf("want 1 orphan finding, got %d: %v", len(rep), rep)
	}
	if rep[0].Code != codeCommentPersonOrphan {
		t.Errorf("code = %q, want %q", rep[0].Code, codeCommentPersonOrphan)
	}
}

// TestGeneratedVMLWellFormed asserts the VML drawing built for a comment is
// well-formed XML.
func TestGeneratedVMLWellFormed(t *testing.T) {
	c := &oxml.CT_Comments{
		Authors: []string{"A"},
		Comments: []oxml.CT_Comment{
			{Ref: "A1", AuthorID: 0, Text: oxml.NewCommentText("x")},
			{Ref: "Z100", AuthorID: 0, Text: oxml.NewCommentText("y")},
		},
	}
	assignShapeIDs(c)
	vml := buildCommentVML(c)
	if err := xml.Unmarshal(vml, new(struct {
		XMLName xml.Name
	})); err != nil {
		t.Fatalf("generated VML is not well-formed: %v\n%s", err, vml)
	}
}
