package pptx

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// modernCreatedLayout is the timestamp format written for a new modern comment
// (ISO 8601, milliseconds, no zone — the form PowerPoint writes).
const modernCreatedLayout = "2006-01-02T15:04:05.000"

// AddComment attaches a modern threaded comment authored by author with the
// given body text to the slide, anchored at the slide level. The author is
// registered in the presentation's author list (deduplicated by name) and a new
// threaded comment part is written for the returned comment. The handle can be
// used to reply or resolve.
//
// Newly added comments always use the modern (2018 threaded) mechanism, which
// is what current PowerPoint writes and the only one that supports replies and
// resolution — even on a deck whose existing comments are legacy (both
// mechanisms may coexist in one file).
func (s *Slide) AddComment(author, text string) *Comment {
	return s.addModernComment(author, text, 0, 0, false)
}

// AddCommentAt attaches a modern threaded comment anchored at the given slide
// position (x, y in EMUs). See AddComment.
func (s *Slide) AddCommentAt(x, y int64, author, text string) *Comment {
	return s.addModernComment(author, text, x, y, true)
}

func (s *Slide) addModernComment(author, text string, x, y int64, hasPos bool) *Comment {
	p := s.presentation
	authorID := p.authorIDForName(author)

	cm := &oxml.ModernComment{
		ID:       newGUID(),
		AuthorID: authorID,
		Created:  time.Now().UTC().Format(modernCreatedLayout),
		BodyText: text,
	}
	cm.PreChildren = append(cm.PreChildren, slideAnchorMarker(s.id))
	if hasPos {
		cm.PreChildren = append(cm.PreChildren, positionMarker(x, y))
	}

	partName := p.nextAvailableModernCommentName()
	part := &oxml.ModernCommentPart{Comment: cm}
	p.otherParts[partName] = &coxml.RawPart{
		ContentType: opc.ContentTypeModernComments,
		Data:        part.Marshal(),
	}

	relID := s.nextRelID()
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         relID,
		Type:       opc.RelTypeModernComments,
		Target:     relativeTarget(s.partName, partName),
		TargetMode: opc.TargetModeInternal,
	})

	c := &Comment{
		slide:    s,
		kind:     commentModern,
		id:       cm.ID,
		author:   author,
		text:     text,
		date:     parseCommentDate(cm.Created),
		x:        x,
		y:        y,
		hasPos:   hasPos,
		thread:   cm,
		partName: partName,
	}
	return c
}

// Reply adds a threaded reply authored by author to this comment's thread and
// returns the reply. Replies require the modern threaded mechanism; calling
// Reply on a legacy comment is a no-op that returns nil (legacy PowerPoint
// comments have no threading — add new comments to get modern threads).
func (c *Comment) Reply(author, text string) *Comment {
	if c.kind != commentModern || c.thread == nil {
		return nil
	}
	// Attach to the top-level comment of the thread.
	top := c
	if c.parent != nil {
		top = c.parent
	}
	p := c.slide.presentation
	authorID := p.authorIDForName(author)

	r := &oxml.ModernReply{
		ID:       newGUID(),
		AuthorID: authorID,
		Created:  time.Now().UTC().Format(modernCreatedLayout),
		BodyText: text,
	}
	c.thread.Replies = append(c.thread.Replies, r)
	p.rewriteModernThread(c.partName, c.thread)

	child := &Comment{
		slide:    c.slide,
		kind:     commentModern,
		id:       r.ID,
		author:   author,
		text:     text,
		date:     parseCommentDate(r.Created),
		resolved: top.resolved,
		parent:   top,
		thread:   c.thread,
		partName: c.partName,
		reply:    r,
	}
	top.replies = append(top.replies, child)
	return child
}

// Resolve marks the comment's thread as resolved. See SetResolved.
func (c *Comment) Resolve() { c.SetResolved(true) }

// SetResolved sets whether the comment's thread is resolved, propagating the
// state across the top-level comment and all of its replies (as PowerPoint
// does). Legacy comments have no resolved concept; SetResolved is a documented
// no-op on them.
func (c *Comment) SetResolved(resolved bool) {
	if c.kind != commentModern || c.thread == nil {
		return
	}
	top := c
	if c.parent != nil {
		top = c.parent
	}
	if resolved {
		c.thread.Status = "resolved"
	} else {
		c.thread.Status = ""
	}
	top.resolved = resolved
	for _, r := range top.replies {
		r.resolved = resolved
	}
	c.slide.presentation.rewriteModernThread(c.partName, c.thread)
}

// rewriteModernThread re-marshals a modern comment thread part in place.
func (p *Presentation) rewriteModernThread(partName string, cm *oxml.ModernComment) {
	part := &oxml.ModernCommentPart{Comment: cm}
	p.otherParts[partName] = &coxml.RawPart{
		ContentType: opc.ContentTypeModernComments,
		Data:        part.Marshal(),
	}
}

// --- author list management ---

// loadModernAuthors returns the parsed authors.xml, loading it from otherParts
// on first use. It returns nil when the deck has no authors part and none has
// been created yet.
func (p *Presentation) loadModernAuthors() *oxml.ModernAuthorList {
	if p.modernAuthorsLoaded {
		return p.modernAuthors
	}
	p.modernAuthorsLoaded = true
	if data := p.rawPartData(modernAuthorsPart); data != nil {
		if lst, err := oxml.ParseModernAuthorList(data); err == nil {
			p.modernAuthors = lst
		}
	}
	return p.modernAuthors
}

// authorIDForName returns the GUID of the author with the given display name,
// registering a new author (and creating/rewriting authors.xml plus the
// presentation relationship) when none matches.
func (p *Presentation) authorIDForName(name string) string {
	lst := p.loadModernAuthors()
	if lst == nil {
		lst = &oxml.ModernAuthorList{}
		p.modernAuthors = lst
		p.modernAuthorsLoaded = true
	}
	for _, a := range lst.Authors {
		if a != nil && a.Name == name {
			return a.ID
		}
	}
	a := &oxml.ModernAuthor{
		ID:         newGUID(),
		Name:       name,
		Initials:   initialsOf(name),
		ProviderID: "None",
	}
	lst.Authors = append(lst.Authors, a)

	firstAuthor := p.rawPartData(modernAuthorsPart) == nil
	p.otherParts[modernAuthorsPart] = &coxml.RawPart{
		ContentType: opc.ContentTypeAuthors,
		Data:        lst.Marshal(),
	}
	if firstAuthor {
		p.ensureAuthorsRelationship()
	}
	return a.ID
}

// ensureAuthorsRelationship adds the presentation -> authors.xml relationship
// when it is not already present.
func (p *Presentation) ensureAuthorsRelationship() {
	const presPart = "/ppt/presentation.xml"
	for _, rel := range p.relationships[presPart] {
		if rel != nil && rel.Type == opc.RelTypeAuthors {
			return
		}
	}
	p.relationships[presPart] = append(p.relationships[presPart], &opc.Relationship{
		ID:         p.nextPresRelID(),
		Type:       opc.RelTypeAuthors,
		Target:     relativeTarget(presPart, modernAuthorsPart),
		TargetMode: opc.TargetModeInternal,
	})
}

// nextPresRelID allocates the next free rId for the presentation part.
func (p *Presentation) nextPresRelID() string {
	maxID := 0
	for _, r := range p.relationships["/ppt/presentation.xml"] {
		var id int
		if _, err := fmt.Sscanf(r.ID, "rId%d", &id); err == nil && id > maxID {
			maxID = id
		}
	}
	if maxID < p.nextRelID {
		maxID = p.nextRelID - 1
	}
	return fmt.Sprintf("rId%d", maxID+1)
}

// nextAvailableModernCommentName returns a free modernComment part name.
func (p *Presentation) nextAvailableModernCommentName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/comments/modernComment%d.xml", i)
		if _, exists := p.otherParts[name]; !exists {
			return name
		}
	}
}

// --- small helpers ---

// slideAnchorMarker builds the raw slide-creation marker that anchors a modern
// comment to a slide (self-declaring its pc: namespace so it re-emits intact).
func slideAnchorMarker(sldID uint32) []byte {
	return []byte(fmt.Sprintf(
		`<pc:sldMkLst xmlns:pc="http://schemas.microsoft.com/office/powerpoint/2013/main/command"><pc:docMk/><pc:sldMk cId="%d" sldId="%d"/></pc:sldMkLst>`,
		randUint32(), sldID))
}

// positionMarker builds the raw p188:pos child for an anchored comment.
func positionMarker(x, y int64) []byte {
	return []byte(fmt.Sprintf(`<p188:pos x="%d" y="%d"/>`, x, y))
}

// newGUID returns a random RFC 4122 v4 GUID in PowerPoint's brace-wrapped
// uppercase form, e.g. "{4CCACD5F-CEAB-1D4B-9994-C599F46901C0}".
func newGUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should not fail; fall back to a time-seeded value.
		for i := range b {
			b[i] = byte(time.Now().UnixNano() >> (i % 8 * 8))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("{%08X-%04X-%04X-%04X-%012X}",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func randUint32() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return uint32(time.Now().UnixNano())
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// initialsOf derives up to two uppercase initials from a display name.
func initialsOf(name string) string {
	fields := strings.Fields(name)
	var sb strings.Builder
	for _, f := range fields {
		sb.WriteString(strings.ToUpper(f[:1]))
		if sb.Len() >= 2 {
			break
		}
	}
	return sb.String()
}
