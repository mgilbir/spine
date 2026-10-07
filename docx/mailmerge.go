package docx

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
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
//
// Removing the configuration also removes the settings-part relationships it
// referenced (the data source, header source, ODSO source and recipient data)
// unless something else in the settings part still references them, and drops
// a part such a relationship targeted — the recipient-data part — once no
// relationship in the package targets it any more. Otherwise Word would still
// find the link to the external data source in the saved file. A later
// SetMailMerge whose configuration references one of those relationships by
// id (DataSourceRef, HeaderSourceRef, DataSource.SourceRef) puts it back.
func (d *Document) SetMailMerge(mm *MailMerge) {
	if mm == nil {
		if d.settings == nil {
			return
		}
		var ids []string
		if el := d.settings.Child("mailMerge"); el != nil {
			ids = rawRelIDs(el.RawContent)
		}
		d.settings.SetMailMerge(nil)
		d.markSettingsModified()
		d.releaseSettingsRels(ids)
		return
	}
	s := d.ensureSettings()
	s.SetMailMerge(toCTMailMerge(mm))
	d.markSettingsModified()
	ids := []string{mm.DataSourceRef, mm.HeaderSourceRef}
	if mm.DataSource != nil {
		ids = append(ids, mm.DataSource.SourceRef)
	}
	d.restoreSettingsRels(ids)
}

// relIDAttrRe matches a relationship-id attribute (r:id under any prefix) and
// captures its value.
var relIDAttrRe = regexp.MustCompile(`\b[A-Za-z_][\w.-]*:id="([^"]*)"`)

// rawRelIDs returns the values of the relationship-id attributes in raw
// w:mailMerge content, in document order. The schema puts r:id on
// w:dataSource, w:headerSource, w:odso/w:src and w:odso/w:recipientData and
// gives no other descendant an attribute named id, so every match is a
// relationship id, whatever prefix the producer bound to the namespace.
func rawRelIDs(raw []byte) []string {
	var ids []string
	for _, m := range relIDAttrRe.FindAllSubmatch(raw, -1) {
		ids = append(ids, string(m[1]))
	}
	return ids
}

// releaseSettingsRels removes the settings-part relationships with the given
// ids that the settings part no longer references, and drops a part one of
// them targeted once no relationship in the package targets it. The settings
// part is marshaled to check for remaining references; an id still found as an
// attribute value anywhere in it is kept, which errs on the side of keeping.
func (d *Document) releaseSettingsRels(ids []string) {
	if len(ids) == 0 || d.settings == nil {
		return
	}
	data, err := marshalSettingsXML(d.settings)
	if err != nil {
		return
	}
	drop := map[string]bool{}
	for _, id := range ids {
		if id != "" && !bytes.Contains(data, []byte(`"`+id+`"`)) {
			drop[id] = true
		}
	}
	if len(drop) == 0 {
		return
	}
	part := d.settingsPartName()
	var released []*releasedRel
	kept := make([]*opc.Relationship, 0, len(d.relationships[part]))
	for _, rel := range d.relationships[part] {
		if rel == nil || !drop[rel.ID] {
			kept = append(kept, rel)
			continue
		}
		released = append(released, &releasedRel{rel: rel})
	}
	if len(released) == 0 {
		return
	}
	d.relationships[part] = kept
	if d.rewrittenRels == nil {
		d.rewrittenRels = make(map[string]bool)
	}
	d.rewrittenRels[part] = true
	if d.releasedSettingsRels == nil {
		d.releasedSettingsRels = make(map[string]*releasedRel)
	}
	for _, r := range released {
		if r.rel.TargetMode != opc.TargetModeExternal {
			r.target = opc.ResolvePartName(part, r.rel.Target)
			r.targetRels, r.dropped = d.dropUntargetedPreservedPart(r.target)
		}
		d.releasedSettingsRels[r.rel.ID] = r
	}
}

// releasedRel is a settings-part relationship releaseSettingsRels removed, and
// the part its removal dropped, if any, with that part's relationships.
type releasedRel struct {
	rel        *opc.Relationship
	target     string
	dropped    bool
	targetRels []*opc.Relationship
}

// restoreSettingsRels puts back the released settings-part relationships with
// the given ids, and the parts their removal dropped. An id the settings part
// has a relationship for again is left alone.
func (d *Document) restoreSettingsRels(ids []string) {
	if len(d.releasedSettingsRels) == 0 {
		return
	}
	part := d.settingsPartName()
	for _, id := range ids {
		r := d.releasedSettingsRels[id]
		if r == nil {
			continue
		}
		delete(d.releasedSettingsRels, id)
		exists := false
		for _, rel := range d.relationships[part] {
			if rel != nil && rel.ID == id {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		d.relationships[part] = append(d.relationships[part], r.rel)
		if r.dropped {
			delete(d.droppedParts, r.target)
			delete(d.droppedParts, opc.GetRelationshipsPartName(r.target))
			if len(r.targetRels) > 0 {
				d.relationships[r.target] = r.targetRels
			}
		}
	}
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

// FillMergeFields completes a mail merge for one record: every MERGEFIELD whose
// name is a key of values is replaced by the value as plain text, the way Word
// writes a merged letter. Fields are matched by the name exactly as it appears
// in the field, which is what MergeFields returns. The fields covered are the
// ones MergeFields reads: in the body, including tables and content controls,
// and in every header and footer.
//
// The text takes the formatting of the field's result (for a field Word has
// shown, the «name» placeholder). Line breaks in a value become line breaks
// and tabs become tabs. The field switches that change the text are applied:
// \* Upper, \* Lower, \* FirstCap and \* Caps to the value, using Unicode
// simple case mapping, then \b and \f, whose text goes before and after a
// non-empty value. \* MERGEFORMAT and \* CHARFORMAT need nothing. Number and
// date pictures (\# and \@) and other \* formats are not applied: pass the
// value already formatted. An empty value removes the field.
//
// A MERGEFIELD in the condition of an enclosing field, such as an IF, becomes
// instruction text, so the enclosing field still compares the value when Word
// updates it; the enclosing field itself is left as it is. The value is
// written as one argument, as Word reads a nested field's result: quoted, with
// quotes and backslashes escaped, or only escaped when the MERGEFIELD already
// sits inside a quoted argument.
//
// FillMergeFields returns the names of the MERGEFIELDs left in the document,
// in first-appearance order and without repeats: those with no entry in
// values, and those it does not rewrite because they are locked, inside a
// tracked deletion, or laid out in a way it cannot replace without losing
// content (see the fields it skips below). Other fields are not touched, and a
// document in which nothing was replaced saves unchanged.
//
// A MERGEFIELD is skipped when its begin and end are not in the same paragraph
// and the same hyperlink, tracked insertion or content control, when it
// contains another field, a hyperlink, a tracked change, a content control or
// math, when a field character shares its run with other content, or when its
// result holds anything other than text, tabs and breaks.
func (d *Document) FillMergeFields(values map[string]string) []string {
	order := map[string]int{}
	for i, name := range d.MergeFields() {
		order[name] = i
	}
	var unfilled []string
	seen := map[string]bool{}
	note := func(name string) {
		if !seen[name] {
			seen[name] = true
			unfilled = append(unfilled, name)
		}
	}
	filler := oxml.FieldFiller{
		Fill: func(instr string, _ bool) (string, bool) {
			name, ok := mergeFieldName(instr)
			if !ok {
				return "", false
			}
			value, ok := values[name]
			if !ok {
				note(name)
				return "", false
			}
			return mergeFieldText(value, instr), true
		},
		Skipped: func(instr string) {
			if name, ok := mergeFieldName(instr); ok {
				note(name)
			}
		},
	}

	if d.doc() != nil && d.doc().Body != nil {
		if oxml.FillFields(d.doc().Body.AllParagraphs(), filler) {
			d.markEdited()
		}
	}
	for _, name := range d.sortedHeaderNames() {
		if hp := d.headers[name]; hp != nil && hp.hdr != nil {
			if oxml.FillFields(hp.hdr.AllParagraphs(), filler) {
				d.markHdrFtrModified(name)
			}
		}
	}
	for _, name := range d.sortedFooterNames() {
		if fp := d.footers[name]; fp != nil && fp.ftr != nil {
			if oxml.FillFields(fp.ftr.AllParagraphs(), filler) {
				d.markHdrFtrModified(name)
			}
		}
	}
	// The walk notes a field when it reaches it, and a skipped field only
	// after its part; report them in the order the document has them.
	sort.SliceStable(unfilled, func(i, j int) bool {
		oi, iok := order[unfilled[i]]
		oj, jok := order[unfilled[j]]
		if iok != jok {
			return iok
		}
		return iok && oi < oj
	})
	return unfilled
}

// mergeFieldText applies a MERGEFIELD instruction's text switches to value:
// the \* case formats first, then the \b and \f text around a non-empty
// result. Switch names match case-insensitively, as in Word; an argument may
// follow its switch as the next token or be written against it (\b"Dear ").
func mergeFieldText(value, instr string) string {
	toks := fieldSwitchTokens(instr)
	var before, after string
	for i := 2; i < len(toks); i++ {
		tok := toks[i]
		if len(tok) < 2 || tok[0] != '\\' {
			continue
		}
		sw := strings.ToLower(tok[1:2])
		arg := tok[2:]
		if arg == "" && i+1 < len(toks) && (sw == "b" || sw == "f" || sw == "*") {
			i++
			arg = toks[i]
		}
		switch sw {
		case "b":
			before = arg
		case "f":
			after = arg
		case "*":
			value = applyCaseFormat(value, arg)
		}
	}
	if value == "" {
		return ""
	}
	return before + value + after
}

// fieldSwitchTokens splits a field instruction into tokens like
// tokenizeFieldInstr, except that a quoted span is always a token of its own:
// a quote ends the token before it, and the closing quote ends the span. So
// \b"Dear "\f"!" yields \b, Dear , \f and !, as Word reads it.
func fieldSwitchTokens(instr string) []string {
	var toks []string
	var cur strings.Builder
	inQuote := false
	flush := func(force bool) {
		if cur.Len() > 0 || force {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for _, r := range instr {
		switch {
		case r == '"' && !inQuote:
			flush(false)
			inQuote = true
		case r == '"':
			flush(true) // "" is an empty argument
			inQuote = false
		case !inQuote && (r == ' ' || r == '\t' || r == '\r' || r == '\n'):
			flush(false)
		default:
			cur.WriteRune(r)
		}
	}
	flush(false)
	return toks
}

// applyCaseFormat applies one \* general format switch to s. Formats other
// than the four case formats leave s unchanged.
func applyCaseFormat(s, format string) string {
	switch strings.ToLower(format) {
	case "upper":
		return strings.ToUpper(s)
	case "lower":
		return strings.ToLower(s)
	case "firstcap":
		return capitalizeWords(s, true)
	case "caps":
		return capitalizeWords(s, false)
	}
	return s
}

// capitalizeWords upper-cases the first letter of each whitespace-separated
// word of s, or only of the first word when firstOnly is set, leaving the
// other letters as they are.
func capitalizeWords(s string, firstOnly bool) string {
	var b strings.Builder
	atWordStart := true
	done := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			atWordStart = true
			b.WriteRune(r)
			continue
		}
		if atWordStart && !done {
			r = unicode.ToUpper(r)
			if firstOnly {
				done = true
			}
		}
		atWordStart = false
		b.WriteRune(r)
	}
	return b.String()
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
