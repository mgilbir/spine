package docx

import (
	"path"
	"strings"

	"github.com/mgilbir/spine/opc"
)

// Metadata-part naming.
//
// The names below are the conventional ones Word writes, and they are what a
// created document gets. An *opened* package is different: OPC binds a part to
// its role through a relationship, not through its name, so a producer may
// perfectly legally point the styles relationship at /word/styles2.xml.
//
// loadAllParts matched those parts by hardcoded name, so such a part landed in
// otherParts with d.styles nil. Styles().AddStyle then seeded Word's defaults
// into a fresh model and the save wrote /word/styles.xml — while
// ensureDocRelationship, which matches on relationship *type*, saw the existing
// styles relationship and added none. The result was an orphan part and a style
// that silently never took effect (C502). Resolving through the relationship
// keeps the original name, so the edit lands in the part the document actually
// points at.
const (
	defaultStylesPartName           = "/word/styles.xml"
	defaultNumberingPartName        = "/word/numbering.xml"
	defaultSettingsPartName         = "/word/settings.xml"
	defaultFootnotesPartName        = "/word/footnotes.xml"
	defaultEndnotesPartName         = "/word/endnotes.xml"
	defaultCommentsPartName         = "/word/comments.xml"
	defaultCommentsExtendedPartName = "/word/commentsExtended.xml"
	defaultPeoplePartName           = "/word/people.xml"
)

// metaPartNames holds the resolved package name of each metadata part the model
// parses. Every field defaults to the conventional name and is overwritten at
// open by the target of the corresponding main-part relationship.
type metaPartNames struct {
	styles           string
	numbering        string
	settings         string
	footnotes        string
	endnotes         string
	comments         string
	commentsExtended string
	people           string
	bibliography     string
}

// defaultMetaPartNames returns the conventional names, the set a created
// document uses and the fallback for an opened package that declares no
// relationship of a given type.
func defaultMetaPartNames() metaPartNames {
	return metaPartNames{
		styles:           defaultStylesPartName,
		numbering:        defaultNumberingPartName,
		settings:         defaultSettingsPartName,
		footnotes:        defaultFootnotesPartName,
		endnotes:         defaultEndnotesPartName,
		comments:         defaultCommentsPartName,
		commentsExtended: defaultCommentsExtendedPartName,
		people:           defaultPeoplePartName,
		bibliography:     bibliographyPartName,
	}
}

// resolveMetaPartNames points each metadata part name at the target of the
// main part's relationship of the matching type, leaving the conventional name
// in place when the package declares no such relationship. It must run after
// loadAllRelationships and before the part sweep that parses them.
func (d *Document) resolveMetaPartNames(mainPartName string) {
	targets := map[string]*string{
		opc.RelTypeStyles:           &d.metaParts.styles,
		opc.RelTypeNumbering:        &d.metaParts.numbering,
		opc.RelTypeSettings:         &d.metaParts.settings,
		opc.RelTypeFootnotes:        &d.metaParts.footnotes,
		opc.RelTypeEndnotes:         &d.metaParts.endnotes,
		opc.RelTypeComments:         &d.metaParts.comments,
		opc.RelTypeCommentsExtended: &d.metaParts.commentsExtended,
		opc.RelTypePeople:           &d.metaParts.people,
		opc.RelTypeCustomXML:        &d.metaParts.bibliography,
	}
	seen := make(map[string]bool, len(targets))
	for _, rel := range d.relationships[mainPartName] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		slot, ok := targets[rel.Type]
		if !ok || seen[rel.Type] {
			continue
		}
		// The bibliography lives under the generic customXml relationship type,
		// which also carries ordinary custom-XML item parts. Only a target that
		// looks like the bibliography store claims that slot; anything else
		// stays an opaque custom-XML part.
		if rel.Type == opc.RelTypeCustomXML &&
			!strings.EqualFold(path.Base(opc.ResolvePartName(mainPartName, rel.Target)), "sources.xml") {
			continue
		}
		resolved := opc.ResolvePartName(mainPartName, rel.Target)
		if resolved == "" {
			continue
		}
		*slot = resolved
		seen[rel.Type] = true
	}
}

// metaRelTarget renders a metadata part name as the relationship target the
// main part would carry for it: relative to the main part's directory when it
// sits under it, otherwise an absolute-style package path.
func (d *Document) metaRelTarget(partName string) string {
	dir := path.Dir(d.mainPart())
	if dir != "/" && strings.HasPrefix(partName, dir+"/") {
		return partName[len(dir)+1:]
	}
	return strings.TrimPrefix(partName, "/")
}

// Resolved metadata part names. Each returns the conventional name for a
// created document and the relationship's target for an opened one.

func (d *Document) stylesPartName() string {
	return orDefault(d.metaParts.styles, defaultStylesPartName)
}

func (d *Document) numberingPartName() string {
	return orDefault(d.metaParts.numbering, defaultNumberingPartName)
}

func (d *Document) settingsPartName() string {
	return orDefault(d.metaParts.settings, defaultSettingsPartName)
}

func (d *Document) footnotesPartName() string {
	return orDefault(d.metaParts.footnotes, defaultFootnotesPartName)
}

func (d *Document) endnotesPartName() string {
	return orDefault(d.metaParts.endnotes, defaultEndnotesPartName)
}

func (d *Document) commentsPartName() string {
	return orDefault(d.metaParts.comments, defaultCommentsPartName)
}

func (d *Document) commentsExtendedPartName() string {
	return orDefault(d.metaParts.commentsExtended, defaultCommentsExtendedPartName)
}

func (d *Document) peoplePartName() string {
	return orDefault(d.metaParts.people, defaultPeoplePartName)
}

func (d *Document) bibliographyPartName() string {
	return orDefault(d.metaParts.bibliography, bibliographyPartName)
}

// orDefault returns v, or def when v is empty (a Document built without going
// through Open or Create carries a zero metaPartNames).
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
