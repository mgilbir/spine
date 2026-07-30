package docx

import (
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Mail-merge main-document types (w:mailMerge/w:mainDocumentType), the
// ECMA-376 ST_MailMergeDocType values. Any other value is passed through
// verbatim, so these are conveniences rather than an exhaustive set.
const (
	MailMergeFormLetters = "formLetters"
	MailMergeEmail       = "email"
	MailMergeEnvelopes   = "envelopes"
	MailMergeFax         = "fax"
	MailMergeCatalog     = "catalog"
)

// MailMerge describes a document's mail-merge configuration, stored in the
// settings part (w:mailMerge). Obtain the current configuration from
// Document.MailMerge and write one back with Document.SetMailMerge.
type MailMerge struct {
	// MainDocumentType is the kind of merge document
	// (w:mainDocumentType), e.g. MailMergeFormLetters or MailMergeEmail.
	MainDocumentType string
	// DataType is the data-source kind (w:dataType), e.g. "textFile",
	// "database", "native", or "spreadsheet".
	DataType string
	// ConnectString is the data-source connection string (w:connectString).
	ConnectString string
	// Query is the data-source query (w:query).
	Query string
	// LinkToQuery indicates the query is stored in an external ODC file
	// (w:linkToQuery).
	LinkToQuery bool
	// ViewMergedData shows merged data instead of field codes
	// (w:viewMergedData).
	ViewMergedData bool
	// Destination is the merge output target (w:destination), e.g.
	// "newDocument", "printer", "email", or "fax".
	Destination string
	// DataSourceRef is the relationship ID (r:id) of the merge data source
	// (w:dataSource) when it is an external part.
	DataSourceRef string
	// HeaderSourceRef is the relationship ID (r:id) of the header data source
	// (w:headerSource).
	HeaderSourceRef string
	// DataSource holds the Office Data Source Object connection (w:odso): the
	// source path and the mapping of data-source columns to merge field names.
	DataSource *MailMergeDataSource
}

// MailMergeDataSource is the Office Data Source Object (w:odso) inside a
// mail-merge configuration: where the recipient records live and how the
// data-source columns map to standard merge field names.
type MailMergeDataSource struct {
	// SourceRef is the relationship ID (r:id) of the data-source part (w:src).
	SourceRef string
	// Table is the table or sheet within the data source (w:table).
	Table string
	// UDLConnectString is the Universal Data Link connection string (w:udl).
	UDLConnectString string
	// ConnectionType is the connection kind (w:type), e.g. "database",
	// "addressBook", "textFile", or "spreadsheet".
	ConnectionType string
	// FirstRowHeader indicates the first data row holds column headers (w:fHdr).
	FirstRowHeader bool
	// ColumnDelimiter is the delimiter character code for a delimited-text
	// source (w:colDelim). Zero means unset.
	ColumnDelimiter int
	// FieldMappings maps data-source columns to standard merge field names.
	FieldMappings []MailMergeFieldMapping
}

// MailMergeFieldMapping maps a data-source column to a mail-merge field name
// (w:fieldMapData).
type MailMergeFieldMapping struct {
	// Name is the data-source column name (w:name).
	Name string
	// MappedName is the standard field this column maps to (w:mappedName).
	MappedName string
	// Column is the zero-based column index (w:column).
	Column int
	// Type classifies the mapping (w:type), e.g. "dbColumn" or "null".
	Type string
	// LanguageID is the LCID for the mapping (w:lid).
	LanguageID string
}

// MailMerge returns the document's mail-merge configuration (w:mailMerge in the
// settings part), or nil when the document is not a mail-merge main document.
func (d *Document) MailMerge() *MailMerge {
	if d.settings == nil {
		return nil
	}
	mm := d.settings.MailMerge()
	if mm == nil {
		return nil
	}
	return fromCTMailMerge(mm)
}

// SetMailMerge writes the document's mail-merge configuration (w:mailMerge),
// creating the settings part if necessary. A nil configuration removes the
// element, turning the document back into a plain document. Regenerating the
// element is a modification: the settings part is rewritten on save.
func (d *Document) SetMailMerge(mm *MailMerge) {
	if mm == nil {
		if d.settings == nil {
			return
		}
		d.settings.SetMailMerge(nil)
		d.markSettingsModified()
		return
	}
	s := d.ensureSettings()
	s.SetMailMerge(toCTMailMerge(mm))
	d.markSettingsModified()
}

// fromCTMailMerge converts the internal model to the public struct.
func fromCTMailMerge(c *oxml.CT_MailMerge) *MailMerge {
	mm := &MailMerge{
		MainDocumentType: strVal(c.MainDocumentType),
		DataType:         strVal(c.DataType),
		ConnectString:    strVal(c.ConnectString),
		Query:            strVal(c.Query),
		LinkToQuery:      c.LinkToQuery.IsOn(),
		ViewMergedData:   c.ViewMergedData.IsOn(),
		Destination:      strVal(c.Destination),
		DataSourceRef:    relID(c.DataSource),
		HeaderSourceRef:  relID(c.HeaderSource),
	}
	if c.Odso != nil {
		ds := &MailMergeDataSource{
			SourceRef:        relID(c.Odso.Src),
			Table:            strVal(c.Odso.Table),
			UDLConnectString: strVal(c.Odso.UdlConnString),
			ConnectionType:   strVal(c.Odso.Type),
			FirstRowHeader:   c.Odso.FHdr.IsOn(),
			ColumnDelimiter:  decVal(c.Odso.ColDelim),
		}
		for _, f := range c.Odso.FieldMapData {
			if f == nil {
				continue
			}
			ds.FieldMappings = append(ds.FieldMappings, MailMergeFieldMapping{
				Name:       strVal(f.Name),
				MappedName: strVal(f.MappedName),
				Column:     decVal(f.Column),
				Type:       strVal(f.Type),
				LanguageID: strVal(f.Lid),
			})
		}
		mm.DataSource = ds
	}
	return mm
}

// toCTMailMerge converts the public struct to the internal model.
func toCTMailMerge(mm *MailMerge) *oxml.CT_MailMerge {
	c := &oxml.CT_MailMerge{
		MainDocumentType: newStr(mm.MainDocumentType),
		DataType:         newStr(mm.DataType),
		ConnectString:    newStr(mm.ConnectString),
		Query:            newStr(mm.Query),
		LinkToQuery:      newOnOff(mm.LinkToQuery),
		ViewMergedData:   newOnOff(mm.ViewMergedData),
		Destination:      newStr(mm.Destination),
		DataSource:       newRel(mm.DataSourceRef),
		HeaderSource:     newRel(mm.HeaderSourceRef),
	}
	if mm.DataSource != nil {
		ds := mm.DataSource
		odso := &oxml.CT_Odso{
			Src:           newRel(ds.SourceRef),
			Table:         newStr(ds.Table),
			UdlConnString: newStr(ds.UDLConnectString),
			Type:          newStr(ds.ConnectionType),
			FHdr:          newOnOff(ds.FirstRowHeader),
			ColDelim:      newDec(ds.ColumnDelimiter),
		}
		for _, f := range ds.FieldMappings {
			odso.FieldMapData = append(odso.FieldMapData, &oxml.CT_OdsoFieldMapData{
				Name:       newStr(f.Name),
				MappedName: newStr(f.MappedName),
				Column:     newDec(f.Column),
				Type:       newStr(f.Type),
				Lid:        newStr(f.LanguageID),
			})
		}
		c.Odso = odso
	}
	return c
}

// MergeFields returns the distinct MERGEFIELD field names present in the
// document, in first-appearance order. Both simple fields (w:fldSimple) and
// complex fields (w:fldChar/w:instrText run sequences) are scanned, in
// paragraphs anywhere in the body and in every header and footer, including
// paragraphs nested inside tables and content nested inside content controls,
// hyperlinks and tracked changes. This matches FormFields, which also covers
// headers and footers, and which uses the same descent.
//
// The complex-field state machine runs over each part as a whole rather than
// per paragraph, so a field whose begin, instruction and end runs are split
// across paragraphs — legal per ECMA-376 §17.16.18 and common for IF fields —
// is still read as one field. Headers and footers are walked in part-name
// order, so the result is deterministic.
func (d *Document) MergeFields() []string {
	var out []string
	seen := map[string]bool{}
	collect := func(paras []*oxml.CT_P) {
		st := &mergeFieldScan{out: &out, seen: seen}
		for _, p := range paras {
			st.paragraph(p)
		}
	}
	if d.doc() != nil && d.doc().Body != nil {
		collect(d.doc().Body.AllParagraphs())
	}
	for _, hp := range d.sortedHeaderParts() {
		if hp == nil || hp.hdr == nil {
			continue
		}
		collect(hp.hdr.AllParagraphs())
	}
	for _, fp := range d.sortedFooterParts() {
		if fp == nil || fp.ftr == nil {
			continue
		}
		collect(fp.ftr.AllParagraphs())
	}
	return out
}

// mergeFieldScan is the complex-field state machine, carried across paragraphs
// so a field split at a paragraph boundary keeps its capture state.
type mergeFieldScan struct {
	out       *[]string
	seen      map[string]bool
	instr     strings.Builder
	capturing bool
}

// paragraph feeds one paragraph's content through the scan, descending into
// hyperlinks, tracked changes, inline content controls and simple fields.
func (st *mergeFieldScan) paragraph(p *oxml.CT_P) {
	if p == nil {
		return
	}
	oxml.VisitContent(p, oxml.ContentVisitor{
		Run: st.run,
		// A w:fldSimple carries its instruction as an attribute; its content
		// runs are visited by the same walk, so nested complex fields inside it
		// are picked up by st.run.
		FldSimple: func(f *oxml.CT_SimpleField) {
			if f == nil {
				return
			}
			if name, ok := mergeFieldName(f.Instr); ok {
				addMergeField(name, st.out, st.seen)
			}
		},
	})
}

// run advances the state machine over a single run: the instruction text
// between a w:fldChar "begin" and the following "separate" (or "end") names the
// field.
func (st *mergeFieldScan) run(r *oxml.CT_R) {
	if r == nil {
		return
	}
	for _, fc := range r.FldChar {
		switch fc.FldCharType {
		case "begin":
			st.capturing = true
			st.instr.Reset()
		case "separate", "end":
			if st.capturing {
				if name, ok := mergeFieldName(st.instr.String()); ok {
					addMergeField(name, st.out, st.seen)
				}
				st.capturing = false
			}
		}
	}
	if st.capturing {
		for _, t := range r.InstrText {
			st.instr.WriteString(t.Text)
		}
	}
}

// addMergeField records a name once, preserving first-appearance order.
func addMergeField(name string, out *[]string, seen map[string]bool) {
	if name == "" || seen[name] {
		return
	}
	seen[name] = true
	*out = append(*out, name)
}

// mergeFieldName extracts the field name from a field instruction when it is a
// MERGEFIELD, e.g. ` MERGEFIELD "First Name" \* MERGEFORMAT ` yields
// "First Name". It reports false for any other field.
func mergeFieldName(instr string) (string, bool) {
	toks := tokenizeFieldInstr(instr)
	if len(toks) >= 2 && strings.EqualFold(toks[0], "MERGEFIELD") {
		return toks[1], true
	}
	return "", false
}

// tokenizeFieldInstr splits a field instruction on whitespace, honoring
// double-quoted spans (which may contain spaces) as single tokens.
func tokenizeFieldInstr(instr string) []string {
	var toks []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for _, r := range instr {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t' || r == '\r' || r == '\n') && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return toks
}

// strVal returns a CT_String's value, or "" when nil.
func strVal(s *oxml.CT_String) string {
	if s == nil {
		return ""
	}
	return s.Val
}

// decVal returns a CT_DecimalNumber's value, or 0 when nil.
func decVal(n *oxml.CT_DecimalNumber) int {
	if n == nil {
		return 0
	}
	return n.Val
}

// relID returns a CT_Rel's relationship ID, or "" when nil.
func relID(r *oxml.CT_Rel) string {
	if r == nil {
		return ""
	}
	return r.RID
}

// newStr builds a CT_String, or nil for an empty value so the element is
// omitted.
func newStr(v string) *oxml.CT_String {
	if v == "" {
		return nil
	}
	return &oxml.CT_String{Val: v}
}

// newOnOff builds a present <w:x/> toggle for true, or nil for false.
func newOnOff(v bool) *oxml.CT_OnOff {
	if !v {
		return nil
	}
	return &oxml.CT_OnOff{}
}

// newDec builds a CT_DecimalNumber, or nil for zero so the element is omitted.
func newDec(v int) *oxml.CT_DecimalNumber {
	if v == 0 {
		return nil
	}
	return &oxml.CT_DecimalNumber{Val: v}
}

// newRel builds a CT_Rel, or nil for an empty relationship ID.
func newRel(rid string) *oxml.CT_Rel {
	if rid == "" {
		return nil
	}
	return &oxml.CT_Rel{RID: rid}
}
