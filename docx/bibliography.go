package docx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// bibliographyPartName is the conventional location of the bibliography sources
// part within a WordprocessingML package.
const bibliographyPartName = "/word/bibliography/sources.xml"

// nsB is the shared bibliography namespace (b:).
const nsB = xmlb.NSBibliography

// Common bibliography source types (b:SourceType values). Any other string is
// accepted and passed through verbatim, so callers can emit further types.
const (
	SourceBook                = "Book"
	SourceJournalArticle      = "JournalArticle"
	SourceArticleInPeriodical = "ArticleInAPeriodical"
	SourceReport              = "Report"
	SourceWebSite             = "InternetSite"
	SourceDocumentFromWebSite = "DocumentFromInternetSite"
)

// Source describes a bibliography source (a b:Source entry). It is the value
// type accepted by Document.AddSource and returned by Document.Sources.
//
// Tag is the citation key CITATION fields reference (Paragraph.AddCitation).
// Type is a b:SourceType value (see the Source* constants); it defaults to
// "Book" when empty. Author is a display author: a single "Last, First" name,
// several such names separated by ";", or a corporate/organization name.
type Source struct {
	Tag       string
	Type      string
	Author    string
	Title     string
	Year      string
	City      string
	Publisher string
}

// Sources returns the bibliography sources stored in the document
// (word/bibliography/sources.xml), in document order. It returns nil when the
// document has no bibliography part.
func (d *Document) Sources() []Source {
	if d.sources == nil {
		return nil
	}
	out := make([]Source, 0, len(d.sources.Source))
	for _, src := range d.sources.Source {
		out = append(out, sourceFromElement(src))
	}
	return out
}

// AddSource adds a bibliography source to the document, creating the
// bibliography part (word/bibliography/sources.xml) on first use. A source with
// no Tag is rejected: CITATION fields reference a source by its tag, so an
// untagged source could never be cited. Adding a source whose tag already
// exists replaces the existing entry, so re-adding an edited source updates it
// in place. The source is assigned a fresh b:Guid.
func (d *Document) AddSource(s Source) error {
	if strings.TrimSpace(s.Tag) == "" {
		return fmt.Errorf("docx: AddSource: source tag must not be empty")
	}
	if d.sources == nil {
		d.sources = &oxml.CT_Sources{SelectedStyle: `\APASixthEditionOfficeOnline.xsl`, StyleName: "APA"}
	}
	elem := sourceToElement(s, newBibGUID())
	// Replace an existing source with the same tag rather than duplicating it.
	for i, existing := range d.sources.Source {
		if existing.Leaf("Tag") == s.Tag {
			// Preserve the original Guid so cross-references stay stable.
			if g := existing.Leaf("Guid"); g != "" {
				setBibLeaf(elem, "Guid", g)
			}
			d.sources.Source[i] = elem
			d.sourcesModified = true
			return nil
		}
	}
	d.sources.Source = append(d.sources.Source, elem)
	d.sourcesModified = true
	return nil
}

// RemoveSource removes the bibliography source with the given tag, reporting
// whether one was found and removed.
func (d *Document) RemoveSource(tag string) bool {
	if d.sources == nil {
		return false
	}
	for i, src := range d.sources.Source {
		if src.Leaf("Tag") == tag {
			d.sources.Source = append(d.sources.Source[:i], d.sources.Source[i+1:]...)
			d.sourcesModified = true
			return true
		}
	}
	return false
}

// sourceOf returns the stored source with the given tag, or nil.
func (d *Document) sourceOf(tag string) *oxml.BibElement {
	if d.sources == nil {
		return nil
	}
	for _, src := range d.sources.Source {
		if src.Leaf("Tag") == tag {
			return src
		}
	}
	return nil
}

// AddCitation appends an in-text citation to the paragraph: a CITATION field
// (w:fldSimple) referencing the bibliography source with the given tag,
// together with a cached-result run Word replaces when it formats the citation
// from the active bibliography style. The returned Run holds the placeholder
// result; format it to style the rendered citation.
//
// The source need not exist yet — a citation may precede its AddSource call —
// but when it does exist the placeholder is built from its author and year
// (e.g. "(Smith, 2020)"); otherwise the tag is shown.
//
//	doc.AddSource(docx.Source{Tag: "Smi20", Author: "Smith, John", Title: "A Book", Year: "2020"})
//	p := doc.AddParagraph()
//	p.AddText("As shown ")
//	p.AddCitation("Smi20")
func (p *Paragraph) AddCitation(sourceTag string) *Run {
	instr := " CITATION " + citationArg(sourceTag) + " "
	fld := &oxml.CT_SimpleField{Instr: instr}
	result := &oxml.CT_R{}
	result.SetTexts([]*oxml.CT_Text{{Space: "preserve", Text: p.citationPlaceholder(sourceTag)}})
	fld.R = append(fld.R, result)
	p.mut().AppendFldSimple(fld)
	return &Run{paragraph: p, r: result}
}

// citationArg quotes a citation tag that contains whitespace so the field
// instruction parses it as a single argument, matching what Word writes.
func citationArg(tag string) string {
	if strings.ContainsAny(tag, " \t\r\n") {
		return `"` + tag + `"`
	}
	return tag
}

// citationPlaceholder builds the cached-result text shown until Word formats
// the citation: "(Author, Year)" when the source is known, else "(Tag)".
func (p *Paragraph) citationPlaceholder(tag string) string {
	if p.document != nil {
		if src := p.document.sourceOf(tag); src != nil {
			author := firstAuthorSurname(src.Child("Author"))
			year := src.Leaf("Year")
			switch {
			case author != "" && year != "":
				return "(" + author + ", " + year + ")"
			case author != "":
				return "(" + author + ")"
			case year != "":
				return "(" + year + ")"
			}
		}
	}
	return "(" + tag + ")"
}

// firstAuthorSurname returns the surname of the first person in a b:Author
// subtree, or the corporate name, for building a citation placeholder.
func firstAuthorSurname(a *oxml.BibElement) string {
	if a == nil {
		return ""
	}
	inner := a.Child("Author")
	if inner == nil {
		return ""
	}
	if corp := inner.Leaf("Corporate"); corp != "" {
		return corp
	}
	if nl := inner.Child("NameList"); nl != nil {
		for _, person := range nl.Children {
			if person.Local != "Person" {
				continue
			}
			if last := person.Leaf("Last"); last != "" {
				return last
			}
			if first := person.Leaf("First"); first != "" {
				return first
			}
		}
	}
	return ""
}

// sourceToElement builds a b:Source element tree from a Source value in the
// child order Word writes (tag, type, guid, author, title, year, ...).
func sourceToElement(s Source, guid string) *oxml.BibElement {
	typ := s.Type
	if typ == "" {
		typ = SourceBook
	}
	src := &oxml.BibElement{Local: "Source"}
	add := func(local, text string) {
		if text != "" {
			src.Children = append(src.Children, &oxml.BibElement{Local: local, Text: text})
		}
	}
	add("Tag", s.Tag)
	add("SourceType", typ)
	add("Guid", guid)
	if a := authorElement(s.Author); a != nil {
		src.Children = append(src.Children, a)
	}
	add("Title", s.Title)
	add("Year", s.Year)
	add("City", s.City)
	add("Publisher", s.Publisher)
	return src
}

// authorElement builds the nested b:Author subtree for a display author string,
// or nil when the string is empty. A name containing a comma is split into
// Last/First; several names may be separated by ";". A name with no comma is
// treated as a corporate (organization) author.
func authorElement(author string) *oxml.BibElement {
	author = strings.TrimSpace(author)
	if author == "" {
		return nil
	}
	// b:Author > b:Author > (b:NameList > b:Person...) | b:Corporate
	inner := &oxml.BibElement{Local: "Author"}
	if strings.Contains(author, ",") {
		nameList := &oxml.BibElement{Local: "NameList"}
		for _, name := range strings.Split(author, ";") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			person := &oxml.BibElement{Local: "Person"}
			last, first, _ := strings.Cut(name, ",")
			if l := strings.TrimSpace(last); l != "" {
				person.Children = append(person.Children, &oxml.BibElement{Local: "Last", Text: l})
			}
			if f := strings.TrimSpace(first); f != "" {
				person.Children = append(person.Children, &oxml.BibElement{Local: "First", Text: f})
			}
			nameList.Children = append(nameList.Children, person)
		}
		inner.Children = append(inner.Children, nameList)
	} else {
		inner.Children = append(inner.Children, &oxml.BibElement{Local: "Corporate", Text: author})
	}
	return &oxml.BibElement{Local: "Author", Children: []*oxml.BibElement{inner}}
}

// sourceFromElement reads a Source value from a b:Source element tree.
func sourceFromElement(src *oxml.BibElement) Source {
	return Source{
		Tag:       src.Leaf("Tag"),
		Type:      src.Leaf("SourceType"),
		Author:    authorString(src.Child("Author")),
		Title:     src.Leaf("Title"),
		Year:      src.Leaf("Year"),
		City:      src.Leaf("City"),
		Publisher: src.Leaf("Publisher"),
	}
}

// authorString renders a display author from a b:Author subtree (the inverse of
// authorElement): "Last, First" names joined by "; ", or the corporate name.
func authorString(a *oxml.BibElement) string {
	if a == nil {
		return ""
	}
	inner := a.Child("Author")
	if inner == nil {
		return ""
	}
	if corp := inner.Leaf("Corporate"); corp != "" {
		return corp
	}
	nameList := inner.Child("NameList")
	if nameList == nil {
		return ""
	}
	var names []string
	for _, person := range nameList.Children {
		if person.Local != "Person" {
			continue
		}
		last := person.Leaf("Last")
		first := person.Leaf("First")
		switch {
		case last != "" && first != "":
			names = append(names, last+", "+first)
		case last != "":
			names = append(names, last)
		case first != "":
			names = append(names, first)
		}
	}
	return strings.Join(names, "; ")
}

// setBibLeaf sets (or replaces) the text of a direct leaf child.
func setBibLeaf(e *oxml.BibElement, local, text string) {
	for _, c := range e.Children {
		if c.Local == local && len(c.Children) == 0 {
			c.Text = text
			return
		}
	}
	e.Children = append(e.Children, &oxml.BibElement{Local: local, Text: text})
}

// newBibGUID returns a Word-style bibliography GUID, e.g.
// "{1E2C4F0A-...-...}", uppercase and brace-wrapped.
func newBibGUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is unexpected; a zero GUID still round-trips.
		return "{00000000-0000-0000-0000-000000000000}"
	}
	// Set the RFC 4122 version (4) and variant bits so the value is a valid v4 UUID.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := strings.ToUpper(hex.EncodeToString(b[:]))
	return "{" + h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32] + "}"
}

// marshalBibliographyXML serializes the bibliography sources part.
func marshalBibliographyXML(sources *oxml.CT_Sources) ([]byte, error) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(nsB, xmlb.PrefixBibliography)
	b.SetCollapseEmptyElements(true)
	b.WriteHeader()

	// The b:Sources root attributes are unprefixed (no namespace), matching
	// what Word writes.
	var attrs []xmlb.Attr
	if sources.SelectedStyle != "" {
		attrs = append(attrs, xmlb.Attr{Name: "SelectedStyle", Value: sources.SelectedStyle})
	}
	if sources.StyleName != "" {
		attrs = append(attrs, xmlb.Attr{Name: "StyleName", Value: sources.StyleName})
	}
	if sources.URI != "" {
		attrs = append(attrs, xmlb.Attr{Name: "URI", Value: sources.URI})
	}

	nsDecls := []xmlb.NSDecl{{Prefix: xmlb.PrefixBibliography, URI: nsB}}
	if len(sources.OriginalNSDecls) > 0 {
		nsDecls = sources.OriginalNSDecls
	}
	b.StartElementWithNS(nsB, "Sources", nsDecls, attrs...)
	sources.MarshalContent(b, nsB)
	b.EndElement(nsB, "Sources")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal bibliography sources.xml: %w", err)
	}
	return b.Bytes(), nil
}

// writeBibliographyPart writes word/bibliography/sources.xml when the document
// carries sources and (on the round-trip path) the session modified them. It
// registers the document.xml relationship and the content-type override if
// absent. created=true is the new-document path (write when non-empty).
func (d *Document) writeBibliographyPart(writer *opc.Writer, created bool) error {
	if d.sources == nil || d.sources.Empty() || (!created && !d.sourcesModified) {
		return nil
	}
	data, err := marshalBibliographyXML(d.sources)
	if err != nil {
		return err
	}
	// The resolved part name, not the conventional one: an opened package can
	// point its customXml relationship at any name, and rewriting the store to
	// the conventional location would orphan the original (C502).
	part := d.bibliographyPartName()
	if err := writer.WritePart(part, "application/xml", data); err != nil {
		return err
	}
	d.ensureDocRelationship(opc.RelTypeBibliography, d.metaRelTarget(part))
	return nil
}
