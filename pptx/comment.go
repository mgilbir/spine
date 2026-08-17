package pptx

import (
	"encoding/xml"
	"fmt"
	xmlb "github.com/mgilbir/spine/common/xml"
	"strconv"
	"strings"
	"time"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// commentKind distinguishes the two comment mechanisms PowerPoint uses.
type commentKind int

const (
	commentLegacy commentKind = iota // pre-2018 p:cmLst / commentAuthors.xml
	commentModern                    // 2018 threaded p188:cmLst / authors.xml
)

// Comment is a comment attached to a slide. The read accessors work on both
// PowerPoint comment mechanisms: legacy per-slide comments (p:cm, anchored to a
// slide position) and modern threaded comments (p188, with replies and a
// resolved status).
//
// The core method set — ID, Author, Text, Date, Resolved, Replies, Parent, and
// the AddComment/Reply/Resolve writers — is shared verbatim with the docx and
// xlsx comment APIs so the three formats are symmetric. Slide, Position, and
// AnchorShapeID are pptx-specific additions (the anchor is a real format
// difference: pptx comments attach to a slide and a point/shape).
type Comment struct {
	slide *Slide
	kind  commentKind

	id       string
	author   string
	text     string
	date     time.Time
	resolved bool

	x, y int64

	parent  *Comment
	replies []*Comment

	// Modern write backing. thread is the top-level p188:cm the comment belongs
	// to; partName is the modernComment part that holds it. reply is set when
	// this Comment is a threaded reply rather than the top-level comment.
	thread   *oxml.ModernComment
	partName string
	reply    *oxml.ModernReply
}

// commentDateLayouts are the timestamp formats seen on PowerPoint comments,
// tried in order. Modern created / legacy dt use fractional seconds without a
// zone; some producers add a zone or omit the fraction.
var commentDateLayouts = []string{
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	time.RFC3339Nano,
	time.RFC3339,
}

func parseCommentDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range commentDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ID returns the comment identifier: the modern comment's GUID or the legacy
// comment's index.
func (c *Comment) ID() string { return c.id }

// Author returns the comment author's display name (resolved through the
// authors part), or "" when the author cannot be resolved.
func (c *Comment) Author() string { return c.author }

// Text returns the comment body text.
func (c *Comment) Text() string { return c.text }

// Date returns the comment timestamp, or the zero time when it is absent or
// unparseable.
func (c *Comment) Date() time.Time { return c.date }

// Resolved reports whether the comment's thread is marked resolved. Legacy
// comments have no resolved concept and are never resolved.
func (c *Comment) Resolved() bool { return c.resolved }

// Replies returns the direct replies to this comment, in thread order. Legacy
// comments never have replies.
func (c *Comment) Replies() []*Comment { return c.replies }

// Parent returns the comment this one replies to, or nil for a top-level
// comment.
func (c *Comment) Parent() *Comment { return c.parent }

// Slide returns the slide the comment is attached to (pptx-specific).
func (c *Comment) Slide() *Slide { return c.slide }

// Position returns the comment anchor position in EMUs (pptx-specific). A
// comment with no explicit position returns (0, 0), which both comment
// mechanisms treat as the slide origin.
func (c *Comment) Position() (x, y int64) { return c.x, c.y }

// AnchorShapeID returns the id of the shape a modern comment is anchored to and
// true, or (0, false) when the comment is anchored to the slide rather than a
// shape (pptx-specific, modern comments only).
func (c *Comment) AnchorShapeID() (uint32, bool) {
	if c.kind != commentModern || c.thread == nil {
		return 0, false
	}
	target := c.thread
	for _, raw := range target.PreChildren {
		if id, ok := scanShapeMarkID(raw); ok {
			return id, true
		}
	}
	return 0, false
}

// scanShapeMarkID finds an spMk id attribute inside a raw anchor marker list.
func scanShapeMarkID(raw []byte) (uint32, bool) {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return 0, false
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "spMk" {
			for _, at := range se.Attr {
				if at.Name.Local == "id" {
					var v uint32
					if _, err := fmt.Sscanf(at.Value, "%d", &v); err == nil {
						return v, true
					}
				}
			}
		}
	}
}

// Comments returns every comment on the slide: legacy comments first (in index
// order), then modern threaded comments (each top-level comment followed by its
// replies via Replies). A slide with no comments returns nil.
func (s *Slide) Comments() []*Comment {
	p := s.presentation
	legacyAuthors := p.legacyAuthorNames()
	modernAuthors := p.modernAuthorNames()

	var out []*Comment
	for _, rel := range p.relationships[s.partName] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		switch rel.Type {
		case opc.RelTypeComments:
			target := opc.ResolvePartName(s.partName, rel.Target)
			out = append(out, s.readLegacyComments(target, legacyAuthors)...)
		case opc.RelTypeModernComments:
			target := opc.ResolvePartName(s.partName, rel.Target)
			if c := s.readModernThread(target, modernAuthors); c != nil {
				out = append(out, c)
			}
		}
	}
	return out
}

func (s *Slide) readLegacyComments(partName string, authors map[uint32]string) []*Comment {
	data := s.presentation.rawPartData(partName)
	if data == nil {
		return nil
	}
	var lst oxml.CommentList
	if err := xmlb.Unmarshal(data, &lst); err != nil {
		return nil
	}
	var out []*Comment
	for _, cm := range lst.Cm {
		if cm == nil {
			continue
		}
		c := &Comment{
			slide:  s,
			kind:   commentLegacy,
			id:     fmt.Sprintf("%d", cm.Idx),
			author: authors[cm.AuthorId],
			text:   cm.Text,
			date:   parseCommentDate(cm.Dt),
		}
		if cm.Pos != nil {
			c.x, c.y = cm.Pos.X, cm.Pos.Y
		}
		out = append(out, c)
	}
	return out
}

// readModernThread builds the Comment handles for one modern comment part.
//
// It reads the part's model rather than re-parsing its bytes, so a thread that
// has been replied to or resolved this session reads back as edited. Re-parsing
// here is how a comment added in-session came back as no comment at all.
func (s *Slide) readModernThread(partName string, authors map[string]string) *Comment {
	part := s.presentation.commentModel(partName)
	if part == nil || part.Comment == nil {
		return nil
	}
	cm := part.Comment
	top := &Comment{
		slide:    s,
		kind:     commentModern,
		id:       cm.ID,
		author:   authors[cm.AuthorID],
		text:     cm.Text(),
		date:     parseCommentDate(cm.Created),
		resolved: cm.Status == "resolved",
		thread:   cm,
		partName: partName,
	}
	if x, y, ok := modernPos(cm.PreChildren); ok {
		top.x, top.y = x, y
	}
	for _, r := range cm.Replies {
		child := &Comment{
			slide:    s,
			kind:     commentModern,
			id:       r.ID,
			author:   authors[r.AuthorID],
			text:     r.Text(),
			date:     parseCommentDate(r.Created),
			resolved: top.resolved,
			parent:   top,
			thread:   cm,
			partName: partName,
			reply:    r,
		}
		top.replies = append(top.replies, child)
	}
	return top
}

// modernPos extracts an x/y position from a raw p188:pos child, if present.
func modernPos(children [][]byte) (x, y int64, ok bool) {
	for _, raw := range children {
		dec := xml.NewDecoder(strings.NewReader(string(raw)))
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			if se, ok2 := tok.(xml.StartElement); ok2 && se.Name.Local == "pos" {
				var gotX, gotY bool
				for _, at := range se.Attr {
					switch at.Name.Local {
					case "x":
						if v, err := strconv.ParseInt(at.Value, 10, 64); err == nil {
							x, gotX = v, true
						}
					case "y":
						if v, err := strconv.ParseInt(at.Value, 10, 64); err == nil {
							y, gotY = v, true
						}
					}
				}
				if gotX || gotY {
					return x, y, true
				}
			}
		}
	}
	return 0, 0, false
}

// Comments returns every comment across all slides, slide by slide in slide
// order (see Slide.Comments for per-slide ordering).
func (p *Presentation) Comments() []*Comment {
	var out []*Comment
	for _, s := range p.slides {
		out = append(out, s.Comments()...)
	}
	return out
}

// --- author part reading ---

func (p *Presentation) rawPartData(partName string) []byte {
	if part, ok := p.otherParts[partName]; ok && part != nil {
		return part.Data
	}
	return nil
}

// legacyAuthorNames maps legacy comment author ids to display names.
func (p *Presentation) legacyAuthorNames() map[uint32]string {
	out := map[uint32]string{}
	data := p.rawPartData(legacyAuthorsPart)
	if data == nil {
		return out
	}
	var lst oxml.CommentAuthorList
	if err := xmlb.Unmarshal(data, &lst); err != nil {
		return out
	}
	for _, a := range lst.CmAuthor {
		if a != nil {
			out[a.Id] = a.Name
		}
	}
	return out
}

// modernAuthorNames maps modern author GUIDs to display names.
func (p *Presentation) modernAuthorNames() map[string]string {
	out := map[string]string{}
	lst := p.loadModernAuthors()
	if lst == nil {
		return out
	}
	for _, a := range lst.Authors {
		if a != nil {
			out[a.ID] = a.Name
		}
	}
	return out
}

const (
	legacyAuthorsPart = "/ppt/commentAuthors.xml"
	modernAuthorsPart = "/ppt/authors.xml"
)
