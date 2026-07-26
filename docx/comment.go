package docx

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// commentDateLayout is the timestamp format Word writes for w:date on a comment
// (ISO 8601, UTC, no fractional seconds).
const commentDateLayout = "2006-01-02T15:04:05Z"

// Comment is a comment attached to a document. The read accessors work on any
// comment-bearing document; the threading accessors (Parent, Replies, Resolved)
// and AnchorText additionally rely on the Microsoft commentsExtended part and
// the document range markers that modern Word writes.
//
// The core method set — ID, Author, Text, Date, Resolved, Replies, Parent, and
// the AddComment/Reply/Resolve writers — is shared verbatim with the xlsx and
// pptx comment APIs so the three formats are symmetric. Initials, Paragraphs,
// AnchorText, and range-precise anchoring are docx-specific additions.
type Comment struct {
	document *Document
	c        *oxml.CT_Comment
}

// Comments returns the document's top-level comments (thread roots), in the
// order they appear in the comments part. Replies are reached through
// Comment.Replies() rather than appearing in this list, matching the xlsx and
// pptx comment APIs.
func (d *Document) Comments() []*Comment {
	if d.comments == nil {
		return nil
	}
	out := make([]*Comment, 0, len(d.comments.Comment))
	for _, c := range d.comments.Comment {
		// Skip replies: a comment with a commentsExtended paraIdParent is
		// nested under its parent and surfaced via Replies().
		if ce := d.commentExFor(c); ce != nil && ce.ParaIdParent != "" {
			continue
		}
		out = append(out, &Comment{document: d, c: c})
	}
	return out
}

// ID returns the comment's w:id, the value the document range markers reference.
func (c *Comment) ID() string { return c.c.Id }

// Author returns the comment author's display name.
func (c *Comment) Author() string { return c.c.Author }

// Initials returns the author's initials (docx-specific; empty if absent).
func (c *Comment) Initials() string { return c.c.Initials }

// Text returns the comment body text, with paragraphs joined by newlines.
func (c *Comment) Text() string { return c.c.Text() }

// Date returns the comment timestamp, or the zero time if it is absent or
// unparseable.
func (c *Comment) Date() time.Time {
	if c.c.Date == "" {
		return time.Time{}
	}
	if t, err := time.Parse(commentDateLayout, c.c.Date); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, c.c.Date); err == nil {
		return t
	}
	return time.Time{}
}

// Paragraphs returns the comment body paragraphs (docx-specific).
func (c *Comment) Paragraphs() []*Paragraph {
	out := make([]*Paragraph, 0, len(c.c.P))
	for _, p := range c.c.P {
		out = append(out, &Paragraph{document: c.document, p: p})
	}
	return out
}

// Resolved reports whether the comment's thread is marked done in
// commentsExtended. Comments in a document without that part are never resolved.
func (c *Comment) Resolved() bool {
	ce := c.document.commentExFor(c.c)
	return ce != nil && ce.IsDone()
}

// Parent returns the comment this one replies to, or nil for a top-level
// comment (or when the document carries no threading information).
func (c *Comment) Parent() *Comment {
	ce := c.document.commentExFor(c.c)
	if ce == nil || ce.ParaIdParent == "" {
		return nil
	}
	if parent := c.document.commentByParaID(ce.ParaIdParent); parent != nil {
		return &Comment{document: c.document, c: parent}
	}
	return nil
}

// Replies returns the direct replies to this comment, in document order.
func (c *Comment) Replies() []*Comment {
	d := c.document
	if d.commentsExtended == nil {
		return nil
	}
	paraID := c.c.LastParaID()
	if paraID == "" {
		return nil
	}
	var out []*Comment
	for _, cm := range d.comments.Comment {
		ce := d.commentExFor(cm)
		if ce != nil && ce.ParaIdParent == paraID {
			out = append(out, &Comment{document: d, c: cm})
		}
	}
	return out
}

// AnchorText returns the document text bracketed by this comment's range
// markers (docx-specific). It is "" for a point anchor with no spanned text and
// for a comment whose range cannot be resolved in the document.
func (c *Comment) AnchorText() string {
	text, _ := oxml.AnchorText(c.document.allBodyParagraphs(), c.c.Id)
	return text
}

// SetInitials overrides the author initials on the comment (docx-specific).
func (c *Comment) SetInitials(initials string) {
	c.c.Initials = initials
	c.document.commentsModified = true
}

// --- write API ---

// AddComment attaches a comment authored by author with the given body text,
// anchored over the whole paragraph's content. The returned handle can be used
// to reply, resolve, or set initials.
func (p *Paragraph) AddComment(author, text string) *Comment {
	c := p.document.addCommentModel(author, text, "")
	p.p.AddCommentAroundParagraph(c.Id)
	return &Comment{document: p.document, c: c}
}

// AddComment attaches a comment anchored over this single run (docx-specific
// range-precise form).
func (r *Run) AddComment(author, text string) *Comment {
	doc := r.paragraph.document
	c := doc.addCommentModel(author, text, "")
	r.paragraph.p.AddCommentAroundRun(r.r, c.Id)
	return &Comment{document: doc, c: c}
}

// AddCommentOnRange attaches a comment spanning from the start run to the end
// run (inclusive). The runs may live in the same paragraph or in different
// paragraphs; the range markers are placed around them and the reference mark
// after the end run. It returns nil if either run is not a direct child run of
// its paragraph (e.g. a run nested inside a hyperlink), adding no comment so
// comments.xml gains no orphan entry with no anchor.
func (d *Document) AddCommentOnRange(start, end *Run, author, text string) *Comment {
	if start == nil || end == nil || start.paragraph == nil || end.paragraph == nil {
		return nil
	}
	// Verify both range markers can be anchored before creating the comment
	// model: a nested (non-direct-child) endpoint must not leave an orphan
	// comment in comments.xml with no document anchor (C296).
	if !start.paragraph.p.HasDirectChildRun(start.r) || !end.paragraph.p.HasDirectChildRun(end.r) {
		return nil
	}
	c := d.addCommentModel(author, text, "")
	start.paragraph.p.InsertCommentStartBeforeRun(start.r, c.Id)
	end.paragraph.p.InsertCommentEndAndRefAfterRun(end.r, c.Id)
	return &Comment{document: d, c: c}
}

// Reply adds a threaded reply to this comment, anchored at the same range and
// linked to it through commentsExtended.
func (c *Comment) Reply(author, text string) *Comment {
	d := c.document
	d.ensureParaID(c.c)
	parentParaID := c.c.LastParaID()
	nc := d.addCommentModel(author, text, parentParaID)
	d.nestReplyAnchor(c.c.Id, nc.Id)
	return &Comment{document: d, c: nc}
}

// Resolve marks the comment's thread as done.
func (c *Comment) Resolve() { c.SetResolved(true) }

// SetResolved sets whether the comment's thread is marked done. Word resolves a
// thread as a whole, so the state is applied to every comment in the thread
// (the root and all of its replies), regardless of which one this handle points
// at.
func (c *Comment) SetResolved(resolved bool) {
	d := c.document
	done := "0"
	if resolved {
		done = "1"
	}
	for _, cm := range d.threadFrom(d.threadRoot(c.c)) {
		d.ensureParaID(cm)
		paraID := cm.LastParaID()
		ce := d.ensureCommentEx(paraID)
		ce.Done = done
	}
	d.commentsExtModified = true
}

// --- internal helpers ---

// addCommentModel builds a new comment (appending it to the comments model),
// records its commentsExtended entry (threaded under parentParaID when set),
// registers the author in people, and marks the affected parts modified. It
// does not place any document range markers; the caller anchors the comment.
func (d *Document) addCommentModel(author, text, parentParaID string) *oxml.CT_Comment {
	d.ensureCommentModels()

	id := strconv.Itoa(d.comments.MaxID() + 1)
	paraID := d.nextParaID()

	c := &oxml.CT_Comment{
		Id:       id,
		Author:   author,
		Initials: deriveInitials(author),
		Date:     time.Now().UTC().Format(commentDateLayout),
	}
	para := &oxml.CT_P{
		ParaId: paraID,
		PPr:    &oxml.CT_PPr{PStyle: &oxml.CT_String{Val: "CommentText"}},
	}
	para.AppendR(&oxml.CT_R{
		RPr: &oxml.CT_RPr{RStyle: &oxml.CT_String{Val: "CommentReference"}},
		T:   []*oxml.CT_Text{{Space: "preserve", Text: text}},
	})
	c.AppendP(para)
	d.comments.Comment = append(d.comments.Comment, c)

	d.commentsExtended.CommentEx = append(d.commentsExtended.CommentEx, &oxml.CT_CommentEx{
		ParaId:       paraID,
		ParaIdParent: parentParaID,
	})

	if author != "" && !d.people.Has(author) {
		d.people.Person = append(d.people.Person, &oxml.CT_Person{
			Author:       author,
			PresenceInfo: &oxml.CT_PresenceInfo{ProviderId: "None", UserId: author},
		})
	}

	d.commentsModified = true
	d.commentsExtModified = true
	d.peopleModified = true
	return c
}

// ensureCommentModels lazily creates the comment part models so the write API
// works on a document that had no comments (created or opened).
func (d *Document) ensureCommentModels() {
	if d.comments == nil {
		d.comments = &oxml.CT_Comments{}
	}
	if d.commentsExtended == nil {
		d.commentsExtended = &oxml.CT_CommentsEx{}
	}
	if d.people == nil {
		d.people = &oxml.CT_People{}
	}
}

// commentExFor returns the commentsExtended entry keyed by a comment's
// last-paragraph paraId, or nil.
func (d *Document) commentExFor(c *oxml.CT_Comment) *oxml.CT_CommentEx {
	if d.commentsExtended == nil {
		return nil
	}
	paraID := c.LastParaID()
	if paraID == "" {
		return nil
	}
	return d.commentsExtended.Find(paraID)
}

// ensureCommentEx returns the commentsExtended entry for paraID, creating an
// empty one if absent.
func (d *Document) ensureCommentEx(paraID string) *oxml.CT_CommentEx {
	d.ensureCommentModels()
	if ce := d.commentsExtended.Find(paraID); ce != nil {
		return ce
	}
	ce := &oxml.CT_CommentEx{ParaId: paraID}
	d.commentsExtended.CommentEx = append(d.commentsExtended.CommentEx, ce)
	return ce
}

// commentByParaID returns the comment whose last body paragraph has the given
// paraId, or nil.
func (d *Document) commentByParaID(paraID string) *oxml.CT_Comment {
	if d.comments == nil || paraID == "" {
		return nil
	}
	for _, cm := range d.comments.Comment {
		if cm.LastParaID() == paraID {
			return cm
		}
	}
	return nil
}

// threadRoot walks up the parent links to the top-level comment of the thread.
// A missing link, a broken parent reference, or a cycle stops the walk.
func (d *Document) threadRoot(c *oxml.CT_Comment) *oxml.CT_Comment {
	cur := c
	seen := map[*oxml.CT_Comment]bool{cur: true}
	for {
		ce := d.commentExFor(cur)
		if ce == nil || ce.ParaIdParent == "" {
			return cur
		}
		parent := d.commentByParaID(ce.ParaIdParent)
		if parent == nil || seen[parent] {
			return cur
		}
		seen[parent] = true
		cur = parent
	}
}

// threadFrom returns the comment plus all of its descendant replies.
func (d *Document) threadFrom(root *oxml.CT_Comment) []*oxml.CT_Comment {
	out := []*oxml.CT_Comment{root}
	if d.commentsExtended == nil {
		return out
	}
	for i := 0; i < len(out); i++ {
		paraID := out[i].LastParaID()
		if paraID == "" {
			continue
		}
		for _, cm := range d.comments.Comment {
			ce := d.commentExFor(cm)
			if ce != nil && ce.ParaIdParent == paraID {
				out = append(out, cm)
			}
		}
	}
	return out
}

// ensureParaID assigns a w14:paraId to the comment's last body paragraph if it
// has none, so threading and resolve state can key on it.
func (d *Document) ensureParaID(c *oxml.CT_Comment) {
	if c.LastParaID() != "" {
		return
	}
	if len(c.P) == 0 {
		return
	}
	last := c.P[len(c.P)-1]
	if last == nil {
		return
	}
	last.ParaId = d.nextParaID()
	d.commentsModified = true
}

// nestReplyAnchor places the reply's range markers nested inside the parent's,
// so the reply shares the parent's anchored span.
func (d *Document) nestReplyAnchor(parentID, newID string) {
	paras := d.allBodyParagraphs()
	for _, p := range paras {
		if p.NestCommentStartAfter(parentID, newID) {
			break
		}
	}
	for _, p := range paras {
		if p.NestCommentEndBefore(parentID, newID) {
			break
		}
	}
	for _, p := range paras {
		if p.AppendCommentRefRunAfter(parentID, newID) {
			break
		}
	}
}

// bodyParagraphs returns the body's top-level paragraphs in document order
// (top-level plus block-level SDT content; tables are not descended).
func (d *Document) bodyParagraphs() []*oxml.CT_P {
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	return d.doc().Body.Paragraphs()
}

// allBodyParagraphs returns every paragraph reachable from the body in document
// order, descending into tables (and block-level SDTs). Comment anchoring and
// threading use this so a comment anchored in a table cell is not invisible to
// Reply() and AnchorText() (C267): the range markers live in a table-cell
// paragraph that the top-level-only bodyParagraphs walk never reaches.
func (d *Document) allBodyParagraphs() []*oxml.CT_P {
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	return d.doc().Body.AllParagraphs()
}

// nextParaID returns an 8-hex-digit paraId not already used by any body or
// comment paragraph, or by an existing commentsExtended entry.
func (d *Document) nextParaID() string {
	used := make(map[string]bool)
	for _, p := range d.bodyParagraphs() {
		if p != nil && p.ParaId != "" {
			used[strings.ToUpper(p.ParaId)] = true
		}
	}
	if d.comments != nil {
		for _, cm := range d.comments.Comment {
			for _, p := range cm.P {
				if p != nil && p.ParaId != "" {
					used[strings.ToUpper(p.ParaId)] = true
				}
			}
		}
	}
	if d.commentsExtended != nil {
		for _, ce := range d.commentsExtended.CommentEx {
			if ce.ParaId != "" {
				used[strings.ToUpper(ce.ParaId)] = true
			}
		}
	}
	for {
		var buf [4]byte
		if _, err := rand.Read(buf[:]); err != nil {
			// crypto/rand failure is unexpected; fall back to a time seed so the
			// library never panics building an id.
			n := time.Now().UnixNano()
			buf = [4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
		}
		id := strings.ToUpper(hex.EncodeToString(buf[:]))
		if !used[id] {
			return id
		}
	}
}

// deriveInitials builds up to three uppercase initials from the author's name
// (the leading letter of each whitespace-separated word).
func deriveInitials(author string) string {
	var b strings.Builder
	count := 0
	for _, field := range strings.Fields(author) {
		for _, r := range field {
			b.WriteRune(unicode.ToUpper(r))
			count++
			break
		}
		if count >= 3 {
			break
		}
	}
	return b.String()
}
