package xlsx

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// commentDateLayout is the ISO-8601 form Excel writes in a threaded comment's
// dT attribute (UTC, second precision, trailing Z).
const commentDateLayout = "2006-01-02T15:04:05Z"

// Comment is a cell comment: either a modern threaded comment (a resolvable
// discussion thread with dated, authored entries and replies) or a legacy note
// (a single unauthored-by-identity annotation rendered in a VML box). The read
// and write surface is shared across the docx, xlsx, and pptx packages; the
// xlsx-specific anchor is the cell reference, exposed by Ref.
//
// A Comment returned by Sheet.Comments or Cell.Comment is a snapshot taken at
// the call; mutating methods (Reply, Resolve, SetResolved) write through to the
// workbook so a subsequent save persists them, but do not retroactively update
// other snapshots.
type Comment struct {
	sheet    *Sheet
	threaded bool
	ref      string
	id       string // threaded comment id ("{GUID}"); "" for a legacy note
	rootID   string // id of the thread root (== id for a root); "" for legacy
	author   string
	text     string
	rich     []TextRun // per-run formatting for a legacy note; nil when plain
	date     time.Time
	resolved bool
	parent   *Comment
	replies  []*Comment
}

// ID returns the comment's stable identifier. For a threaded comment this is
// its GUID; a legacy note has no identifier and returns "".
func (c *Comment) ID() string { return c.id }

// Author returns the display name of the comment's author.
func (c *Comment) Author() string { return c.author }

// Text returns the comment's plain-text body. Rich (per-run) formatting is
// flattened; use RichText to read the runs with their formatting intact.
func (c *Comment) Text() string { return c.text }

// RichText returns the comment body as formatting runs. A legacy note carries
// per-run formatting (bold labels, colored text); a threaded comment and an
// unformatted note return a single run holding the plain text. It returns nil
// for an empty body. Text continues to return the flattened plain text.
func (c *Comment) RichText() []TextRun {
	if len(c.rich) > 0 {
		return append([]TextRun(nil), c.rich...)
	}
	if c.text == "" {
		return nil
	}
	return []TextRun{{Text: c.text}}
}

// Date returns the comment's creation time. It is the zero time.Time for a
// legacy note, which carries no timestamp.
func (c *Comment) Date() time.Time { return c.date }

// Resolved reports whether the comment's thread is marked resolved (done). A
// legacy note is never resolved.
func (c *Comment) Resolved() bool { return c.resolved }

// Replies returns the thread's replies to this comment, in order. It is nil for
// a reply or a legacy note.
func (c *Comment) Replies() []*Comment { return c.replies }

// Parent returns the comment this one replies to, or nil for a top-level
// comment or legacy note.
func (c *Comment) Parent() *Comment { return c.parent }

// Ref returns the cell reference the comment is anchored to (e.g. "A1"). This
// is the xlsx-specific anchor, an addition to the shared comment surface.
func (c *Comment) Ref() string { return c.ref }

// Threaded reports whether this is a modern threaded comment (true) or a legacy
// note (false). This is an xlsx-specific addition to the shared surface.
func (c *Comment) Threaded() bool { return c.threaded }

// ---------------------------------------------------------------------------
// In-memory comment model
// ---------------------------------------------------------------------------

// sheetComments holds a sheet's parsed comment model plus the part names and
// relationship ids needed to regenerate the parts on save (reusing the
// existing names/ids of a comment-bearing sheet so its relationship structure
// stays stable).
type sheetComments struct {
	legacy   *oxml.CT_Comments
	threaded *oxml.CT_ThreadedComments

	commentsPart string
	vmlPart      string
	threadedPart string
	commentsRID  string
	vmlRID       string
	threadedRID  string

	loaded  bool
	mutated bool // a comment was added/replied/resolved; regenerate on save
}

// hasComments reports whether the sheet actually carries any comment (a legacy
// note or a threaded comment), whether parsed from the opened package or added
// this session. Unlike a s.comments != nil check it stays false after a
// read-only Comments()/Cell.Comment() call on a comment-free sheet, which
// lazily creates an empty s.comments (C284): a mere inspection must not disable
// writers (e.g. AddOLEObject) that guard on the sheet owning its legacy VML.
func (s *Sheet) hasComments() bool {
	sc := s.comments
	if sc == nil {
		return false
	}
	if sc.mutated {
		return true
	}
	if sc.legacy != nil && len(sc.legacy.Comments) > 0 {
		return true
	}
	if sc.threaded != nil && len(sc.threaded.Comments) > 0 {
		return true
	}
	return false
}

// loadComments parses the sheet's existing comment parts (legacy, threaded) and
// resolves the related part names and relationship ids. It is idempotent.
func (s *Sheet) loadComments() {
	if s.comments != nil && s.comments.loaded {
		return
	}
	sc := &sheetComments{}
	if s.workbook != nil && s.partName != "" {
		for _, rel := range s.workbook.relationships[s.partName] {
			if rel == nil {
				continue
			}
			target := opc.ResolvePartName(s.partName, rel.Target)
			switch rel.Type {
			case opc.RelTypeComments:
				sc.commentsPart = target
				sc.commentsRID = rel.ID
				if part, ok := s.workbook.preservedParts[target]; ok {
					if parsed, err := oxml.ParseComments(part.Data); err == nil {
						sc.legacy = parsed
					}
				}
			case opc.RelTypeVMLDrawing:
				sc.vmlPart = target
				sc.vmlRID = rel.ID
			case opc.RelTypeThreadedComment:
				sc.threadedPart = target
				sc.threadedRID = rel.ID
				if part, ok := s.workbook.preservedParts[target]; ok {
					if parsed, err := oxml.ParseThreadedComments(part.Data); err == nil {
						sc.threaded = parsed
					}
				}
			}
		}
	}
	sc.loaded = true
	s.comments = sc
	if s.workbook != nil {
		s.workbook.loadPersons()
	}
}

// loadPersons parses the workbook's person-list part (shared by all sheets). It
// is idempotent.
func (w *Workbook) loadPersons() {
	if w.personsLoaded {
		return
	}
	w.personsLoaded = true
	w.persons = &oxml.CT_PersonList{}
	main := w.mainPart()
	for _, rel := range w.relationships[main] {
		if rel == nil || rel.Type != opc.RelTypePerson {
			continue
		}
		target := opc.ResolvePartName(main, rel.Target)
		w.personsPartName = target
		if part, ok := w.preservedParts[target]; ok {
			if parsed, err := oxml.ParsePersonList(part.Data); err == nil {
				w.persons = parsed
			}
		}
		return
	}
}

// ---------------------------------------------------------------------------
// Read API
// ---------------------------------------------------------------------------

// Comments returns every comment on the sheet as a unified list, merging modern
// threaded comments and legacy notes. Threaded comments appear as top-level
// entries with their Replies attached; a legacy note on a cell that also has a
// threaded comment is treated as the threaded comment's back-compat fallback
// and is not reported separately.
func (s *Sheet) Comments() []*Comment {
	s.loadComments()

	var out []*Comment
	threadedRefs := make(map[string]bool)

	// Threaded comments: group roots and replies. Excel stores replies flat,
	// each parentId pointing at the thread root.
	if s.comments.threaded != nil {
		byID := make(map[string]*Comment)
		var roots []*Comment
		// First pass: roots.
		for i := range s.comments.threaded.Comments {
			tc := &s.comments.threaded.Comments[i]
			threadedRefs[tc.Ref] = true
			if tc.ParentID != "" {
				continue
			}
			c := s.threadedToComment(tc)
			c.rootID = tc.ID
			byID[tc.ID] = c
			roots = append(roots, c)
		}
		// Second pass: replies.
		for i := range s.comments.threaded.Comments {
			tc := &s.comments.threaded.Comments[i]
			if tc.ParentID == "" {
				continue
			}
			root := byID[tc.ParentID]
			c := s.threadedToComment(tc)
			c.rootID = tc.ParentID
			c.parent = root
			c.resolved = root != nil && root.resolved
			if root != nil {
				root.replies = append(root.replies, c)
			}
		}
		out = append(out, roots...)
	}

	// Legacy notes not shadowed by a threaded comment on the same cell.
	if s.comments.legacy != nil {
		for i := range s.comments.legacy.Comments {
			lc := &s.comments.legacy.Comments[i]
			if threadedRefs[lc.Ref] {
				continue
			}
			author := ""
			if lc.AuthorID >= 0 && lc.AuthorID < len(s.comments.legacy.Authors) {
				author = s.comments.legacy.Authors[lc.AuthorID]
			}
			out = append(out, &Comment{
				sheet:    s,
				threaded: false,
				ref:      lc.Ref,
				author:   author,
				text:     lc.PlainText(),
				rich:     rstToTextRuns(&lc.Text),
			})
		}
	}
	return out
}

// threadedToComment builds a Comment snapshot from a threaded-comment entry,
// resolving the author display name from the workbook person list.
func (s *Sheet) threadedToComment(tc *oxml.CT_ThreadedComment) *Comment {
	author := ""
	if s.workbook != nil && s.workbook.persons != nil {
		if p := s.workbook.persons.FindByID(tc.PersonID); p != nil {
			author = p.DisplayName
		}
	}
	var date time.Time
	if tc.DT != "" {
		if t, err := time.Parse(commentDateLayout, tc.DT); err == nil {
			date = t
		} else if t, err := time.Parse(time.RFC3339, tc.DT); err == nil {
			date = t
		}
	}
	return &Comment{
		sheet:    s,
		threaded: true,
		ref:      tc.Ref,
		id:       tc.ID,
		author:   author,
		text:     tc.Text,
		date:     date,
		resolved: tc.Done,
	}
}

// Comment returns the top-level comment anchored to the cell, or nil if the
// cell has none. A threaded comment takes precedence over a legacy note.
func (c *Cell) Comment() *Comment {
	for _, cm := range c.sheet.Comments() {
		if strings.EqualFold(cm.ref, c.cell.R) {
			return cm
		}
	}
	return nil
}

// canonicalCellRef validates a cell reference and returns it in canonical
// upper-case A1 form (e.g. "a1" -> "A1"). It reports ok=false for a
// syntactically invalid ref, so the comment-authoring API can reject one rather
// than emit a note with an unparseable or ref-less anchor.
func canonicalCellRef(ref string) (string, bool) {
	row, col, err := ParseCellRef(ref)
	if err != nil {
		return "", false
	}
	return FormatCellRef(row, col), true
}

// removeCommentsAt drops every legacy note and threaded comment (a thread root
// and its replies) anchored to ref, matched case-insensitively, so a new
// comment on that cell replaces rather than duplicates the existing one.
func (s *Sheet) removeCommentsAt(ref string) {
	if s.comments == nil {
		return
	}
	if lc := s.comments.legacy; lc != nil {
		kept := lc.Comments[:0]
		for _, cm := range lc.Comments {
			if !strings.EqualFold(cm.Ref, ref) {
				kept = append(kept, cm)
			}
		}
		lc.Comments = kept
	}
	if tc := s.comments.threaded; tc != nil {
		kept := tc.Comments[:0]
		for _, cm := range tc.Comments {
			if !strings.EqualFold(cm.Ref, ref) {
				kept = append(kept, cm)
			}
		}
		tc.Comments = kept
	}
}

// ---------------------------------------------------------------------------
// Write API
// ---------------------------------------------------------------------------

// AddComment adds a threaded comment authored by author to the cell, returning
// the new comment. Excel's back-compat behavior is matched: a threaded comment
// is created (so modern Excel shows the thread) together with a legacy note
// fallback (so older Excel still renders the text). The author is registered as
// a person, deduplicated by display name.
func (c *Cell) AddComment(author, text string) *Comment {
	return c.sheet.addComment(c.cell.R, author, text)
}

// AddComment adds a threaded comment authored by author to the cell at ref (see
// Cell.AddComment).
func (s *Sheet) AddComment(ref, author, text string) *Comment {
	return s.addComment(ref, author, text)
}

func (s *Sheet) addComment(ref, author, text string) *Comment {
	canon, ok := canonicalCellRef(ref)
	if !ok {
		return nil
	}
	ref = canon
	s.loadComments()
	if s.comments.threaded == nil {
		s.comments.threaded = &oxml.CT_ThreadedComments{}
	}
	if s.comments.legacy == nil {
		s.comments.legacy = &oxml.CT_Comments{}
	}

	// Replace any comment already anchored to this cell so a repeated AddComment
	// (or one using a case-variant ref such as "a1" then "A1") does not emit two
	// <comment ref="A1"> entries and two threaded roots on one cell (C288).
	s.removeCommentsAt(ref)

	person := s.workbook.ensurePerson(author)
	id := newGUID()

	s.comments.threaded.Comments = append(s.comments.threaded.Comments, oxml.CT_ThreadedComment{
		Ref:      ref,
		DT:       time.Now().UTC().Format(commentDateLayout),
		PersonID: person.ID,
		ID:       id,
		Text:     text,
	})

	// Legacy fallback note: one per thread root, so old Excel renders something.
	authorID := s.comments.legacy.AuthorIndex(author)
	s.comments.legacy.Comments = append(s.comments.legacy.Comments, oxml.CT_Comment{
		Ref:      ref,
		AuthorID: authorID,
		Text:     oxml.NewCommentText(text),
	})

	s.markCommentsDirty()

	return &Comment{
		sheet:    s,
		threaded: true,
		ref:      ref,
		id:       id,
		rootID:   id,
		author:   author,
		text:     text,
		date:     time.Now().UTC(),
	}
}

// AddNote adds a legacy note (an unthreaded comment) authored by author to the
// cell at ref, returning it. Unlike AddComment it creates no threaded comment
// or person entry; use it when only the classic note mechanism is wanted.
func (s *Sheet) AddNote(ref, author, text string) *Comment {
	canon, ok := canonicalCellRef(ref)
	if !ok {
		return nil
	}
	ref = canon
	s.loadComments()
	if s.comments.legacy == nil {
		s.comments.legacy = &oxml.CT_Comments{}
	}
	s.removeCommentsAt(ref)
	authorID := s.comments.legacy.AuthorIndex(author)
	s.comments.legacy.Comments = append(s.comments.legacy.Comments, oxml.CT_Comment{
		Ref:      ref,
		AuthorID: authorID,
		Text:     oxml.NewCommentText(text),
	})
	s.markCommentsDirty()
	return &Comment{sheet: s, threaded: false, ref: ref, author: author, text: text}
}

// AddNoteRichText adds a legacy note whose body carries per-run formatting (a
// bold label, colored text) to the cell at ref, returning it. Each TextRun may
// set its own font; a nil run font leaves the run in the note's default font.
// Text on the returned comment reads back the flattened plain text.
func (s *Sheet) AddNoteRichText(ref, author string, runs []TextRun) *Comment {
	canon, ok := canonicalCellRef(ref)
	if !ok {
		return nil
	}
	ref = canon
	s.loadComments()
	if s.comments.legacy == nil {
		s.comments.legacy = &oxml.CT_Comments{}
	}
	s.removeCommentsAt(ref)
	authorID := s.comments.legacy.AuthorIndex(author)
	s.comments.legacy.Comments = append(s.comments.legacy.Comments, oxml.CT_Comment{
		Ref:      ref,
		AuthorID: authorID,
		Text:     textRunsToRst(runs),
	})
	s.markCommentsDirty()
	return &Comment{
		sheet:    s,
		threaded: false,
		ref:      ref,
		author:   author,
		text:     plainTextOfRuns(runs),
		rich:     append([]TextRun(nil), runs...),
	}
}

// SetRichText replaces the comment's body with formatted runs, writing through
// to the workbook so a subsequent save persists it. For a legacy note the runs
// are stored with their formatting; for a threaded comment (whose stored body
// is plain text) the runs are flattened for the thread entry while the note's
// back-compat fallback keeps the formatting. It is a no-op on a comment not
// backed by any note or thread entry on its sheet.
func (c *Comment) SetRichText(runs []TextRun) {
	if c.sheet == nil {
		return
	}
	s := c.sheet
	s.loadComments()
	rst := textRunsToRst(runs)
	plain := plainTextOfRuns(runs)

	if s.comments.legacy != nil {
		for i := range s.comments.legacy.Comments {
			if strings.EqualFold(s.comments.legacy.Comments[i].Ref, c.ref) {
				s.comments.legacy.Comments[i].Text = rst
			}
		}
	}
	if c.threaded && s.comments.threaded != nil {
		for i := range s.comments.threaded.Comments {
			if s.comments.threaded.Comments[i].ID == c.id {
				s.comments.threaded.Comments[i].Text = plain
			}
		}
	}

	c.text = plain
	c.rich = append([]TextRun(nil), runs...)
	s.markCommentsDirty()
}

// Reply adds a reply authored by author to the comment's thread, returning the
// new reply. It is a no-op returning nil for a legacy note, which has no
// thread. Replies are stored flat with the thread root as parent, matching
// Excel.
func (c *Comment) Reply(author, text string) *Comment {
	if !c.threaded || c.sheet == nil {
		return nil
	}
	s := c.sheet
	s.loadComments()
	if s.comments.threaded == nil {
		s.comments.threaded = &oxml.CT_ThreadedComments{}
	}
	person := s.workbook.ensurePerson(author)
	id := newGUID()
	rootID := c.rootID
	if rootID == "" {
		rootID = c.id
	}
	s.comments.threaded.Comments = append(s.comments.threaded.Comments, oxml.CT_ThreadedComment{
		Ref:      c.ref,
		DT:       time.Now().UTC().Format(commentDateLayout),
		PersonID: person.ID,
		ID:       id,
		ParentID: rootID,
		Text:     text,
	})
	s.markCommentsDirty()
	reply := &Comment{
		sheet:    s,
		threaded: true,
		ref:      c.ref,
		id:       id,
		rootID:   rootID,
		author:   author,
		text:     text,
		date:     time.Now().UTC(),
		parent:   c,
	}
	c.replies = append(c.replies, reply)
	return reply
}

// Resolve marks the comment's thread resolved (done). Equivalent to
// SetResolved(true).
func (c *Comment) Resolve() { c.SetResolved(true) }

// SetResolved sets the resolved (done) state of the comment's thread. It is a
// no-op for a legacy note.
func (c *Comment) SetResolved(resolved bool) {
	if !c.threaded || c.sheet == nil {
		return
	}
	s := c.sheet
	s.loadComments()
	rootID := c.rootID
	if rootID == "" {
		rootID = c.id
	}
	if s.comments.threaded != nil {
		for i := range s.comments.threaded.Comments {
			tc := &s.comments.threaded.Comments[i]
			if tc.ID == rootID && tc.ParentID == "" {
				tc.Done = resolved
				break
			}
		}
	}
	c.resolved = resolved
	s.markCommentsDirty()
}

// markCommentsDirty flags the sheet's comment model as modified so save
// regenerates the comment parts, and marks the sheet dirty so its worksheet is
// re-marshaled with the legacy-drawing reference.
func (s *Sheet) markCommentsDirty() {
	if s.comments != nil {
		s.comments.dirtyMark()
	}
	s.dirty = true
}

func (sc *sheetComments) dirtyMark() { sc.mutated = true }

// ensurePerson returns the person with the given display name, creating one
// (with a fresh GUID) if absent and marking the person list dirty.
func (w *Workbook) ensurePerson(displayName string) *oxml.CT_Person {
	w.loadPersons()
	if p := w.persons.Find(displayName); p != nil {
		return p
	}
	w.persons.Persons = append(w.persons.Persons, oxml.CT_Person{
		DisplayName: displayName,
		ID:          newGUID(),
		ProviderID:  "None",
	})
	w.personsDirty = true
	return &w.persons.Persons[len(w.persons.Persons)-1]
}

// rstToTextRuns converts a comment's stored rich text (CT_Rst) into public
// TextRuns, or nil when the text is empty. A note with formatting runs yields
// one TextRun per run; a plain note yields a single unformatted run.
func rstToTextRuns(rst *oxml.CT_Rst) []TextRun {
	if rst == nil {
		return nil
	}
	if len(rst.R) > 0 {
		return reltRunsToTextRuns(rst.R)
	}
	if rst.T != nil && *rst.T != "" {
		return []TextRun{{Text: *rst.T}}
	}
	return nil
}

// textRunsToRst converts public TextRuns into a comment's rich-text body. A
// single unformatted run collapses to a plain <t>, matching NewCommentText, so
// simple notes stay byte-identical to the plain-text path.
func textRunsToRst(runs []TextRun) oxml.CT_Rst {
	if len(runs) == 1 && runs[0].Font == nil {
		return oxml.NewCommentText(runs[0].Text)
	}
	rst := oxml.CT_Rst{R: make([]oxml.CT_RElt, 0, len(runs))}
	for _, run := range runs {
		rst.R = append(rst.R, oxml.CT_RElt{
			RPr: fontStyleToRPrElt(run.Font),
			T:   run.Text,
		})
	}
	return rst
}

// plainTextOfRuns flattens formatting runs to their concatenated text.
func plainTextOfRuns(runs []TextRun) string {
	var sb strings.Builder
	for i := range runs {
		sb.WriteString(runs[i].Text)
	}
	return sb.String()
}

// newGUID returns a random RFC-4122 v4 GUID in Excel's brace-wrapped uppercase
// form, e.g. "{1F2E3D4C-...}".
func newGUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("{%08X-%04X-%04X-%04X-%012X}",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
