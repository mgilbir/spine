package xlsx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// sheetsHaveComments reports whether any sheet has pending comment mutations.
func (w *Workbook) sheetsHaveComments() bool {
	for _, sheet := range w.sheets {
		if sheet.comments != nil && sheet.comments.mutated {
			return true
		}
	}
	return false
}

// writeSheetComments regenerates the comment parts (legacy comments, VML
// drawing, threaded comments) for one sheet whose comment model was mutated,
// wiring the new relationships into sheetRels and pointing the worksheet's
// <legacyDrawing> at the VML. Existing part names and relationship ids are
// reused so a comment-bearing sheet keeps a stable relationship structure; only
// missing ones are allocated. The (possibly extended) sheetRels is returned.
//
// It also deletes any superseded original comment parts from the preserved set
// so the verbatim streaming loop does not re-emit stale bytes on top of the
// regenerated parts.
func (w *Workbook) writeSheetComments(
	writer *opc.Writer,
	sheet *Sheet,
	sheetRels []*opc.Relationship,
	relUsed map[string]struct{},
	used map[string]struct{},
	seq *commentSeq,
) ([]*opc.Relationship, error) {
	sc := sheet.comments

	nextRID := func() string {
		id := fmt.Sprintf("rId%d", nextRelationshipID(relUsed))
		relUsed[id] = struct{}{}
		return id
	}

	// Capture the sheet's existing legacy VML (which may host form-control or OLE
	// shapes this sheet's comments do not own, and the user's own note-box
	// geometry) before dropping the superseded originals, so composeLegacyVML
	// can re-emit those shapes verbatim.
	//
	// The capture is cached on the sheet because the drop below is destructive:
	// without it a second save found no preserved original, regenerated the
	// drawing from scratch and threw away everything the first save preserved.
	if sc.originalVML == nil && sc.vmlPart != "" {
		if part, ok := w.preservedParts[sc.vmlPart]; ok && part != nil {
			sc.originalVML = part.Data
		}
	}
	originalVML := sc.originalVML

	// Drop superseded originals so they are not streamed verbatim.
	for _, name := range []string{sc.commentsPart, sc.vmlPart, sc.threadedPart} {
		if name != "" {
			w.dropPreservedPart(name)
		}
	}

	hasLegacy := sc.legacy != nil && len(sc.legacy.Comments) > 0
	hasThreaded := sc.threaded != nil && len(sc.threaded.Comments) > 0

	// When the sheet also has pending OLE embeds, comments and OLE objects share
	// one legacy VML part: writeSheetOLE (which runs after this) owns it and
	// composes the comment note shapes into it. Here we only assign note shape
	// ids above the OLE shape range (1025..1025+n-1) and write comments.xml; the
	// VML part and worksheet <legacyDrawing> are left to the OLE path so a single
	// <legacyDrawing> points at one part holding both kinds of shape (C283).
	hasOLE := len(sheet.oleEmbeds) > 0

	// Legacy comments + VML drawing (the note boxes). Both are needed together:
	// the legacy comment is the text, the VML shape is its rendering.
	if hasLegacy {
		commentsPart := sc.commentsPart
		if commentsPart == "" {
			commentsPart = allocName(used, &seq.comments, "/xl/comments%d.xml")
		}
		commentsRID := sc.commentsRID
		if commentsRID == "" {
			commentsRID = nextRID()
		}

		if hasOLE {
			// Reserve note shape ids above the OLE shapes; writeSheetOLE renders
			// the notes into the combined VML using these ids.
			assignShapeIDsFrom(sc.legacy, 1025+len(sheet.oleEmbeds))
		} else {
			// Compose the legacy VML, preserving any non-comment shapes (form
			// controls) already present and allocating note shape ids above the
			// max shape id in use. composeLegacyVML also assigns each comment's
			// ShapeID, so it must run before comments.xml is marshaled.
			vmlBytes := composeLegacyVML(originalVML, sc.legacy)
			vmlPart := sc.vmlPart
			if vmlPart == "" {
				vmlPart = allocName(used, &seq.vml, "/xl/drawings/vmlDrawing%d.vml")
			}
			vmlRID := sc.vmlRID
			if vmlRID == "" {
				vmlRID = nextRID()
			}
			if err := writer.WritePart(vmlPart, opc.ContentTypeVMLDrawing, vmlBytes); err != nil {
				return nil, err
			}
			sheetRels = ensureRel(sheetRels, vmlRID, opc.RelTypeVMLDrawing, relTargetFromSheet(vmlPart))

			// Point the worksheet at the VML drawing (the element that renders the
			// note boxes). EnsureChildOrder splices it into the captured child
			// order if the sheet had none.
			sheet.ws().LegacyDrawing = &oxml.CT_LegacyDrawing{RID: vmlRID}
			sheet.ws().EnsureChildOrder("legacyDrawing")
		}

		if err := writer.WritePart(commentsPart, opc.ContentTypeSheetComments, oxml.MarshalComments(sc.legacy)); err != nil {
			return nil, err
		}
		sheetRels = ensureRel(sheetRels, commentsRID, opc.RelTypeComments, relTargetFromSheet(commentsPart))
	}

	// Threaded comments. Linked to the worksheet by relationship only (no body
	// element); authors resolve through the workbook person list.
	if hasThreaded {
		threadedPart := sc.threadedPart
		if threadedPart == "" {
			threadedPart = allocName(used, &seq.threaded, "/xl/threadedComments/threadedComment%d.xml")
		}
		threadedRID := sc.threadedRID
		if threadedRID == "" {
			threadedRID = nextRID()
		}
		if err := writer.WritePart(threadedPart, opc.ContentTypeThreadedComments, oxml.MarshalThreadedComments(sc.threaded)); err != nil {
			return nil, err
		}
		sheetRels = ensureRel(sheetRels, threadedRID, opc.RelTypeThreadedComment, relTargetFromSheet(threadedPart))
	}

	return sheetRels, nil
}

// writeWorkbookPersons regenerates the workbook-shared person list when it was
// mutated, returning the relationship target (relative to the workbook part) so
// the caller can wire the workbook relationship, or "" if nothing was written.
func (w *Workbook) writeWorkbookPersons(writer *opc.Writer, used map[string]struct{}) (string, error) {
	if !w.personsDirty || w.persons == nil || len(w.persons.Persons) == 0 {
		return "", nil
	}
	partName := w.personsPartName
	if partName == "" {
		seq := 1
		partName = allocName(used, &seq, "/xl/persons/person%d.xml")
		w.personsPartName = partName
	} else {
		w.dropPreservedPart(partName)
	}
	if err := writer.WritePart(partName, opc.ContentTypePerson, oxml.MarshalPersonList(w.persons)); err != nil {
		return "", err
	}
	return strings.TrimPrefix(partName, "/xl/"), nil
}

// dropPreservedPart removes a part from the preserved set and its content-type
// override, so a regenerated part is not shadowed by stale original bytes.
func (w *Workbook) dropPreservedPart(name string) {
	delete(w.preservedParts, name)
	if w.contentTypes != nil {
		w.contentTypes.RemoveOverride(name)
	}
}

// commentSeq tracks the next free numeric suffix per comment part family, so
// multiple comment-bearing sheets in one save don't collide.
type commentSeq struct {
	comments int
	vml      int
	threaded int
}

func newCommentSeq() *commentSeq { return &commentSeq{comments: 1, vml: 1, threaded: 1} }

// allocName finds a free part name of the form fmt.Sprintf(pattern, N),
// marking it used.
func allocName(used map[string]struct{}, seq *int, pattern string) string {
	for {
		name := fmt.Sprintf(pattern, *seq)
		*seq++
		if _, ok := used[name]; !ok {
			used[name] = struct{}{}
			return name
		}
	}
}

// ensureRel appends a relationship if one with the id is not already present.
func ensureRel(rels []*opc.Relationship, id, relType, target string) []*opc.Relationship {
	for _, r := range rels {
		if r != nil && r.ID == id {
			return rels
		}
	}
	return append(rels, &opc.Relationship{ID: id, Type: relType, Target: target})
}

// relTargetFromSheet turns an absolute part name into a target relative to a
// worksheet's .rels (which lives in /xl/worksheets/_rels/).
func relTargetFromSheet(partName string) string {
	return "../" + strings.TrimPrefix(partName, "/xl/")
}

// assignShapeIDs assigns sequential VML shape ids to legacy comments, starting
// at Excel's conventional 1025, when they are not already set.
func assignShapeIDs(c *oxml.CT_Comments) {
	assignShapeIDsFrom(c, 1025)
}

// assignShapeIDsFrom assigns sequential VML shape ids to legacy comments that
// have none, starting at start (clamped to Excel's conventional minimum of
// 1025). Comments that already carry a ShapeID keep it but still consume a slot,
// preserving the 1:1 comment-to-shape numbering.
func assignShapeIDsFrom(c *oxml.CT_Comments, start int) {
	if start < 1025 {
		start = 1025
	}
	next := start
	for i := range c.Comments {
		if c.Comments[i].ShapeID == "" {
			c.Comments[i].ShapeID = strconv.Itoa(next)
		}
		next++
	}
}
