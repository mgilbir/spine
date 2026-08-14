package pptx

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
func (s *Slide) AddComment(author, text string) (*Comment, error) {
	return s.addModernComment(author, text, 0, 0, false)
}

// AddCommentAt attaches a modern threaded comment anchored at the given slide
// position (x, y in EMUs). See AddComment.
func (s *Slide) AddCommentAt(x, y int64, author, text string) (*Comment, error) {
	return s.addModernComment(author, text, x, y, true)
}

func (s *Slide) addModernComment(author, text string, x, y int64, hasPos bool) (*Comment, error) {
	p := s.presentation
	// Comment parts are preserved raw parts written here and re-emitted
	// verbatim, so the write needs no flag to persist and nothing else records
	// that the deck changed.
	p.markModelEdited()
	authorID, err := p.authorIDForName(author)
	if err != nil {
		return nil, err
	}

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
	data, err := part.Marshal()
	if err != nil {
		return nil, err
	}
	p.otherParts[partName] = &coxml.RawPart{
		ContentType: opc.ContentTypeModernComments,
		Data:        data,
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
		thread:   cm,
		partName: partName,
	}
	return c, nil
}

// Reply adds a threaded reply authored by author to this comment's thread and
// returns the reply. Replies require the modern threaded mechanism; calling
// Reply on a legacy comment is a no-op that returns nil (legacy PowerPoint
// comments have no threading — add new comments to get modern threads).
func (c *Comment) Reply(author, text string) (*Comment, error) {
	if c.kind != commentModern || c.thread == nil {
		return nil, nil
	}
	// Attach to the top-level comment of the thread.
	top := c
	if c.parent != nil {
		top = c.parent
	}
	p := c.slide.presentation
	authorID, err := p.authorIDForName(author)
	if err != nil {
		return nil, err
	}

	r := &oxml.ModernReply{
		ID:       newGUID(),
		AuthorID: authorID,
		Created:  time.Now().UTC().Format(modernCreatedLayout),
		BodyText: text,
	}
	c.thread.Replies = append(c.thread.Replies, r)
	if err := p.rewriteModernThread(c.partName, c.thread); err != nil {
		return nil, err
	}

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
	return child, nil
}

// Resolve marks the comment's thread as resolved. See SetResolved.
func (c *Comment) Resolve() error { return c.SetResolved(true) }

// SetResolved sets whether the comment's thread is resolved, propagating the
// state across the top-level comment and all of its replies (as PowerPoint
// does). Legacy comments have no resolved concept; SetResolved is a documented
// no-op on them.
func (c *Comment) SetResolved(resolved bool) error {
	if c.kind != commentModern || c.thread == nil {
		return nil
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
	return c.slide.presentation.rewriteModernThread(c.partName, c.thread)
}

// rewriteModernThread re-marshals a modern comment thread part in place.
func (p *Presentation) rewriteModernThread(partName string, cm *oxml.ModernComment) error {
	// Every thread edit (Reply, Resolve, SetResolved) lands here; see
	// addModernComment for why the write itself needs no flag.
	p.markModelEdited()
	part := &oxml.ModernCommentPart{Comment: cm}
	data, err := part.Marshal()
	if err != nil {
		return err
	}
	p.otherParts[partName] = &coxml.RawPart{
		ContentType: opc.ContentTypeModernComments,
		Data:        data,
	}
	return nil
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
func (p *Presentation) authorIDForName(name string) (string, error) {
	lst := p.loadModernAuthors()
	if lst == nil {
		lst = &oxml.ModernAuthorList{}
		p.modernAuthors = lst
		p.modernAuthorsLoaded = true
	}
	for _, a := range lst.Authors {
		if a != nil && a.Name == name {
			return a.ID, nil
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
	data, err := lst.Marshal()
	if err != nil {
		return "", err
	}
	p.otherParts[modernAuthorsPart] = &coxml.RawPart{
		ContentType: opc.ContentTypeAuthors,
		Data:        data,
	}
	if firstAuthor {
		p.ensureAuthorsRelationship()
	}
	return a.ID, nil
}

// importModernCommentAuthors merges the source deck's modern author list
// (ppt/authors.xml) into this deck's, deduplicating by GUID so an imported
// threaded comment's AuthorID keeps resolving to a real name. The authors part
// is (re)written and the presentation -> authors relationship ensured. Called
// when a merged slide carries a modern (threaded) comments relationship.
func (p *Presentation) importModernCommentAuthors(srcPres *Presentation) error {
	data := srcPres.rawPartData(modernAuthorsPart)
	if data == nil {
		return nil
	}
	srcList, err := oxml.ParseModernAuthorList(data)
	if err != nil || srcList == nil {
		// A source author list this library cannot read is not a reason to fail
		// the merge: the slides still import, their comments just resolve to
		// "Unknown" until the deck is repaired.
		return nil
	}
	dst := p.loadModernAuthors()
	if dst == nil {
		dst = &oxml.ModernAuthorList{}
		p.modernAuthors = dst
		p.modernAuthorsLoaded = true
	}
	have := make(map[string]bool, len(dst.Authors))
	for _, a := range dst.Authors {
		if a != nil {
			have[a.ID] = true
		}
	}
	for _, a := range srcList.Authors {
		if a == nil || have[a.ID] {
			continue
		}
		dst.Authors = append(dst.Authors, a)
		have[a.ID] = true
	}
	data, err = dst.Marshal()
	if err != nil {
		return err
	}
	p.otherParts[modernAuthorsPart] = &coxml.RawPart{
		ContentType: opc.ContentTypeAuthors,
		Data:        data,
	}
	p.ensureAuthorsRelationship()
	return nil
}

// ensureAuthorsRelationship adds the presentation -> authors.xml relationship
// when it is not already present.
func (p *Presentation) ensureAuthorsRelationship() {
	for _, rel := range p.relationships[presentationPartName] {
		if rel != nil && rel.Type == opc.RelTypeAuthors {
			return
		}
	}
	p.addPresentationRel(opc.RelTypeAuthors, relativeTarget(presentationPartName, modernAuthorsPart))
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

// initialsOf derives up to two uppercase initials from a display name. Initials
// are taken by decoding the first rune of each whitespace-separated field, so a
// non-ASCII name ("Émile Zola") yields "ÉZ" rather than the U+FFFD produced by
// slicing the first byte of a multibyte rune.
func initialsOf(name string) string {
	initials := make([]rune, 0, 2)
	for _, f := range strings.Fields(name) {
		r, _ := utf8.DecodeRuneInString(f)
		if r == utf8.RuneError {
			continue
		}
		initials = append(initials, unicode.ToUpper(r))
		if len(initials) >= 2 {
			break
		}
	}
	return string(initials)
}
