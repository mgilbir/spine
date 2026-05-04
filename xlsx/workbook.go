package xlsx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Workbook represents an Excel workbook.
type Workbook struct {
	// Properties contains the document properties.
	Properties opc.CoreProperties

	reader           *opc.Reader
	closer           io.Closer
	workbook         *oxml.CT_Workbook
	sharedStrings    *oxml.CT_Sst
	stylesheet       *oxml.CT_Stylesheet
	sheets           []*Sheet
	preservedParts   map[string]*coxml.RawPart
	contentTypesData []byte
	relationships    map[string][]*opc.Relationship
	hasCoreProps     bool
	stringTable      []string // plain text values extracted from shared strings
}

// Open opens an Excel workbook from a file path.
func Open(path string) (*Workbook, error) {
	reader, err := opc.OpenReader(path)
	if err != nil {
		return nil, err
	}

	return openFromReader(&reader.Reader, reader)
}

// OpenReader opens an Excel workbook from an in-memory reader.
func OpenReader(r io.ReaderAt, size int64) (*Workbook, error) {
	reader, err := opc.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	return openFromReader(reader, nil)
}

// openFromReader creates a Workbook from an OPC reader.
func openFromReader(reader *opc.Reader, closer io.Closer) (*Workbook, error) {
	rels := reader.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 {
		closeCloser(closer)
		return nil, ErrNotXLSX
	}

	mainPartName := opc.ResolvePartName("/", rels[0].Target)
	mainPart := reader.GetFile(mainPartName)
	if mainPart == nil {
		closeCloser(closer)
		return nil, ErrNotXLSX
	}

	if mainPart.ContentType != opc.ContentTypeWorkbook {
		closeCloser(closer)
		return nil, ErrNotXLSX
	}

	data, err := mainPart.ReadAll()
	if err != nil {
		closeCloser(closer)
		return nil, err
	}

	var wb oxml.CT_Workbook
	if err := xml.Unmarshal(data, &wb); err != nil {
		closeCloser(closer)
		return nil, err
	}

	// Extract formatting details from the raw XML for byte-identical round-trip.
	wb.OriginalXMLSep = extractXMLSeparator(data)
	wb.SelfClosingSpace = detectSelfClosingSpace(data)

	w := &Workbook{
		reader:         reader,
		closer:         closer,
		workbook:       &wb,
		preservedParts: make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
	}

	if reader.Properties != nil {
		w.Properties = *reader.Properties
		w.hasCoreProps = true
	}

	if err := w.loadAllParts(mainPartName); err != nil {
		closeCloser(closer)
		return nil, err
	}

	return w, nil
}

// loadAllParts loads all parts from the package.
func (w *Workbook) loadAllParts(mainPartName string) error {
	if w.reader == nil {
		return nil
	}

	w.loadAllRelationships()

	// Preserve [Content_Types].xml
	if ctData, err := w.reader.GetRawZipFile("[Content_Types].xml"); err == nil {
		w.contentTypesData = ctData
	}

	for _, file := range w.reader.Files {
		name := file.Name
		data, err := file.ReadAll()
		if err != nil {
			continue
		}

		// Preserve all parts as raw bytes for round-trip
		w.preservedParts[name] = &coxml.RawPart{
			ContentType: file.ContentType,
			Data:        data,
		}

		switch {
		case strings.HasSuffix(name, ".rels"):
			continue
		case name == mainPartName:
			continue
		case name == "/docProps/core.xml":
			continue
		case name == "/docProps/app.xml":
			// preserved in preservedParts
		case name == "/xl/sharedStrings.xml":
			w.sharedStrings = &oxml.CT_Sst{}
			if err := xml.Unmarshal(data, w.sharedStrings); err != nil {
				return err
			}
		case name == "/xl/styles.xml":
			w.stylesheet = &oxml.CT_Stylesheet{}
			if err := xml.Unmarshal(data, w.stylesheet); err != nil {
				return err
			}
		default:
			// preserved in preservedParts
		}
	}

	// Build string table from shared strings
	w.buildStringTable()

	// Load worksheets using the sheet order from workbook.xml
	w.loadSheets(mainPartName)

	return nil
}

// loadAllRelationships loads all relationship files into the model.
func (w *Workbook) loadAllRelationships() {
	if w.reader == nil {
		return
	}

	for _, file := range w.reader.Files {
		if !strings.HasSuffix(file.Name, ".rels") {
			continue
		}

		data, err := file.ReadAll()
		if err != nil {
			continue
		}

		rels, err := opc.UnmarshalRelationships(data)
		if err != nil {
			continue
		}

		sourcePart := coxml.RelsPathToSourcePart(file.Name)
		w.relationships[sourcePart] = rels
	}
}

// loadSheets loads worksheets in the order defined by workbook.xml sheets element.
func (w *Workbook) loadSheets(mainPartName string) {
	wbRels := w.relationships[mainPartName]

	for i, sheetDef := range w.workbook.Sheets.Sheet {
		// Resolve the sheet's part name from its r:id
		partName := ""
		for _, rel := range wbRels {
			if rel.ID == sheetDef.RID {
				partName = opc.ResolvePartName(mainPartName, rel.Target)
				break
			}
		}

		sheet := &Sheet{
			workbook: w,
			name:     sheetDef.Name,
			index:    i,
			partName: partName,
			relID:    sheetDef.RID,
		}

		if partName != "" {
			if part, ok := w.preservedParts[partName]; ok {
				ws := &oxml.CT_Worksheet{}
				if err := xml.Unmarshal(part.Data, ws); err == nil {
					sheet.worksheet = ws
				}
			}
		}

		w.sheets = append(w.sheets, sheet)
	}
}

// buildStringTable extracts plain text from the shared string table.
func (w *Workbook) buildStringTable() {
	if w.sharedStrings == nil {
		return
	}

	w.stringTable = make([]string, len(w.sharedStrings.Si))
	for i, si := range w.sharedStrings.Si {
		if si.T != nil {
			w.stringTable[i] = *si.T
		} else if len(si.R) > 0 {
			// Concatenate rich text runs
			var sb strings.Builder
			for _, run := range si.R {
				sb.WriteString(run.T)
			}
			w.stringTable[i] = sb.String()
		}
	}
}

// resolveSharedString returns the string at the given index in the shared string table.
func (w *Workbook) resolveSharedString(index int) string {
	if index < 0 || index >= len(w.stringTable) {
		return ""
	}
	return w.stringTable[index]
}

// Create creates a new, empty workbook.
func Create() *Workbook {
	wb := &oxml.CT_Workbook{
		Sheets: oxml.CT_Sheets{},
	}

	return &Workbook{
		workbook:       wb,
		preservedParts: make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
	}
}

// Save saves the workbook to a file.
func (w *Workbook) Save(path string) error {
	writer, err := opc.Create(path)
	if err != nil {
		return err
	}

	if err := w.saveTo(writer); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

// SaveTo saves the workbook to an arbitrary writer.
func (w *Workbook) SaveTo(dst io.Writer) error {
	writer := opc.NewWriter(dst)
	if err := w.saveTo(writer); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

// WriteToBuffer saves the workbook to an in-memory buffer.
func (w *Workbook) WriteToBuffer() (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if err := w.SaveTo(&buf); err != nil {
		return nil, err
	}
	return &buf, nil
}

// Close closes the workbook and releases resources.
func (w *Workbook) Close() error {
	if w.closer != nil {
		closer := w.closer
		w.closer = nil
		return closer.Close()
	}
	return nil
}

// saveTo writes the workbook to an OPC writer.
func (w *Workbook) saveTo(writer *opc.Writer) error {
	if w.reader != nil {
		return w.saveRoundTrip(writer)
	}
	return w.saveNew(writer)
}

func closeCloser(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}

// saveRoundTrip saves a workbook opened from a file, preserving all parts.
func (w *Workbook) saveRoundTrip(writer *opc.Writer) error {
	if w.hasCoreProps {
		writer.Properties = &w.Properties
	}

	// Preserve original content types
	if w.reader != nil && w.reader.ContentTypes != nil {
		writer.ContentTypes = w.reader.ContentTypes
	}

	// Write [Content_Types].xml as raw file if preserved
	if len(w.contentTypesData) > 0 {
		if err := writer.WriteRawFile("[Content_Types].xml", w.contentTypesData); err != nil {
			return err
		}
	}

	// Write core.xml as preserved raw bytes if original had it
	if w.hasCoreProps {
		if part, ok := w.preservedParts["/docProps/core.xml"]; ok {
			if err := writer.WritePart("/docProps/core.xml", part.ContentType, part.Data); err != nil {
				return err
			}
		}
	}

	// Write all preserved parts except workbook.xml (which is regenerated),
	// core.xml (handled above), and .rels files (handled separately)
	mainPartName := "/xl/workbook.xml"
	for name, part := range w.preservedParts {
		if name == mainPartName {
			continue
		}
		if name == "/docProps/core.xml" {
			continue
		}
		if strings.HasSuffix(name, ".rels") {
			continue
		}
		if err := writer.WritePart(name, part.ContentType, part.Data); err != nil {
			return err
		}
	}

	// Write all .rels files from preserved parts
	for name, part := range w.preservedParts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		if err := writer.WritePart(name, part.ContentType, part.Data); err != nil {
			return err
		}
	}

	// Write workbook.xml (always regenerated from parsed model)
	wbData := marshalWorkbookXML(w.workbook)
	if err := writer.WritePart(mainPartName, opc.ContentTypeWorkbook, wbData); err != nil {
		return err
	}

	// Add main relationship
	writer.AddRelationship(opc.RelTypeOfficeDocument, "xl/workbook.xml", opc.TargetModeInternal)

	return nil
}

// saveNew saves a newly created workbook.
func (w *Workbook) saveNew(writer *opc.Writer) error {
	writer.Properties = &w.Properties

	mainPartName := "/xl/workbook.xml"
	var wbRels []*opc.Relationship
	relID := 1

	// Write each worksheet
	for i, sheet := range w.sheets {
		if sheet.worksheet == nil {
			sheet.worksheet = &oxml.CT_Worksheet{
				SheetData: oxml.CT_SheetData{},
			}
		}

		sheetPartName := fmt.Sprintf("/xl/worksheets/sheet%d.xml", i+1)
		if sheet.streamWriter != nil {
			if err := sheet.streamWriter.Flush(); err != nil {
				return err
			}
			updateStreamedSheetDimension(sheet)

			partWriter, err := writer.CreatePart(sheetPartName, opc.ContentTypeWorksheet, opc.CompressionDeflate)
			if err != nil {
				return err
			}
			if err := writeWorksheetPrefix(partWriter, sheet.worksheet); err != nil {
				return err
			}
			if _, err := partWriter.Write(sheet.streamWriter.buf.Bytes()); err != nil {
				return err
			}
			if err := writeWorksheetSuffix(partWriter, sheet.worksheet); err != nil {
				return err
			}
		} else {
			wsData := marshalWorksheetXML(sheet.worksheet)
			if err := writer.WritePart(sheetPartName, opc.ContentTypeWorksheet, wsData); err != nil {
				return err
			}
		}

		rid := fmt.Sprintf("rId%d", relID)
		wbRels = append(wbRels, &opc.Relationship{
			ID:     rid,
			Type:   opc.RelTypeWorksheet,
			Target: fmt.Sprintf("worksheets/sheet%d.xml", i+1),
		})

		// Update the workbook model
		w.workbook.Sheets.Sheet = append(w.workbook.Sheets.Sheet[:0:0], w.workbook.Sheets.Sheet...)
		if i < len(w.workbook.Sheets.Sheet) {
			w.workbook.Sheets.Sheet[i].RID = rid
		}
		relID++
	}

	// Rebuild the sheets element in the workbook model
	w.workbook.Sheets.Sheet = make([]oxml.CT_Sheet, len(w.sheets))
	sheetRelID := 1
	for i, sheet := range w.sheets {
		w.workbook.Sheets.Sheet[i] = oxml.CT_Sheet{
			Name:    sheet.name,
			SheetId: uint32(i + 1),
			RID:     fmt.Sprintf("rId%d", sheetRelID),
		}
		sheetRelID++
	}

	// Write styles.xml if a stylesheet exists
	if w.stylesheet != nil {
		stylesPartName := "/xl/styles.xml"
		stylesData := marshalStylesheetXML(w.stylesheet)
		if err := writer.WritePart(stylesPartName, opc.ContentTypeStyles, stylesData); err != nil {
			return err
		}

		rid := fmt.Sprintf("rId%d", relID)
		wbRels = append(wbRels, &opc.Relationship{
			ID:     rid,
			Type:   opc.RelTypeStyles,
			Target: "styles.xml",
		})
	}

	// Write workbook.xml
	wbData := marshalWorkbookXML(w.workbook)
	if err := writer.WritePart(mainPartName, opc.ContentTypeWorkbook, wbData); err != nil {
		return err
	}

	// Write workbook relationships
	if err := writer.WritePartRelationships(mainPartName, wbRels); err != nil {
		return err
	}

	// Add main relationship
	writer.AddRelationship(opc.RelTypeOfficeDocument, "xl/workbook.xml", opc.TargetModeInternal)

	return nil
}

func updateStreamedSheetDimension(sheet *Sheet) {
	if sheet.streamWriter == nil || sheet.streamWriter.maxCol == 0 || sheet.streamWriter.maxRow() == 0 {
		return
	}
	endRef := FormatCellRef(sheet.streamWriter.maxRow(), sheet.streamWriter.maxCol)
	sheet.worksheet.Dimension = &oxml.CT_SheetDimension{Ref: "A1:" + endRef}
}

// Styles returns the StyleManager for this workbook. If no stylesheet exists
// yet (e.g. for a newly created workbook), a default one is created.
func (w *Workbook) Styles() *StyleManager {
	if w.stylesheet == nil {
		w.stylesheet = defaultStylesheet()
	}
	return newStyleManager(w.stylesheet)
}

// Sheets returns all sheets in the workbook.
func (w *Workbook) Sheets() []*Sheet {
	return w.sheets
}

// SheetCount returns the number of sheets.
func (w *Workbook) SheetCount() int {
	return len(w.sheets)
}

// Sheet returns the sheet at the specified index (0-based).
func (w *Workbook) Sheet(index int) (*Sheet, error) {
	if index < 0 || index >= len(w.sheets) {
		return nil, ErrSheetIndex
	}
	return w.sheets[index], nil
}

// SheetByName returns the sheet with the specified name.
func (w *Workbook) SheetByName(name string) (*Sheet, error) {
	for _, sheet := range w.sheets {
		if sheet.name == name {
			return sheet, nil
		}
	}
	return nil, ErrSheetNotFound
}

// AddSheet adds a new sheet to the workbook.
func (w *Workbook) AddSheet(name string) *Sheet {
	ws := &oxml.CT_Worksheet{
		SheetData: oxml.CT_SheetData{},
	}

	sheet := &Sheet{
		workbook:  w,
		name:      name,
		index:     len(w.sheets),
		worksheet: ws,
	}
	w.sheets = append(w.sheets, sheet)

	// Update the workbook model
	w.workbook.Sheets.Sheet = append(w.workbook.Sheets.Sheet, oxml.CT_Sheet{
		Name:    name,
		SheetId: uint32(len(w.sheets)),
		RID:     fmt.Sprintf("rId%d", len(w.sheets)),
	})

	return sheet
}

// DeleteSheet removes the sheet at the specified index.
func (w *Workbook) DeleteSheet(index int) error {
	if index < 0 || index >= len(w.sheets) {
		return ErrSheetIndex
	}
	w.sheets = append(w.sheets[:index], w.sheets[index+1:]...)
	for i := index; i < len(w.sheets); i++ {
		w.sheets[i].index = i
	}

	// Update the workbook model
	w.workbook.Sheets.Sheet = append(w.workbook.Sheets.Sheet[:index], w.workbook.Sheets.Sheet[index+1:]...)

	return nil
}

// ActiveSheet returns the currently active sheet.
func (w *Workbook) ActiveSheet() *Sheet {
	if len(w.sheets) == 0 {
		return nil
	}

	// Check bookViews for active tab
	if w.workbook.BookViews != nil {
		for _, bv := range w.workbook.BookViews.WorkbookView {
			if bv.ActiveTab != nil {
				idx := int(*bv.ActiveTab)
				if idx >= 0 && idx < len(w.sheets) {
					return w.sheets[idx]
				}
			}
		}
	}

	return w.sheets[0]
}

// SetActiveSheet sets the active sheet by index.
func (w *Workbook) SetActiveSheet(index int) error {
	if index < 0 || index >= len(w.sheets) {
		return ErrSheetIndex
	}

	if w.workbook.BookViews == nil {
		w.workbook.BookViews = &oxml.CT_BookViews{}
	}
	if len(w.workbook.BookViews.WorkbookView) == 0 {
		w.workbook.BookViews.WorkbookView = append(w.workbook.BookViews.WorkbookView, oxml.CT_BookView{})
	}

	idx := uint32(index)
	w.workbook.BookViews.WorkbookView[0].ActiveTab = &idx

	return nil
}

// ParseCellRef parses a cell reference like "A1" into 1-based row and column numbers.
func ParseCellRef(ref string) (row, col int, err error) {
	if ref == "" {
		return 0, 0, ErrInvalidCell
	}

	// Split into column letters and row number
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 {
		// Try lowercase
		for i < len(ref) && ref[i] >= 'a' && ref[i] <= 'z' {
			i++
		}
	}
	if i == 0 || i == len(ref) {
		return 0, 0, ErrInvalidCell
	}

	colStr := strings.ToUpper(ref[:i])
	rowStr := ref[i:]

	// Parse column letters to number
	col = 0
	for _, c := range colStr {
		col = col*26 + int(c-'A'+1)
	}

	// Parse row number
	row, err = strconv.Atoi(rowStr)
	if err != nil || row < 1 {
		return 0, 0, ErrInvalidCell
	}

	return row, col, nil
}

// DefinedName represents a named range or formula in the workbook.
type DefinedName struct {
	Name       string
	Value      string
	SheetIndex int // -1 for workbook scope
}

// AddDefinedName adds a workbook-scoped defined name.
func (w *Workbook) AddDefinedName(name, ref string) error {
	return w.addDefinedName(name, ref, -1)
}

// AddDefinedNameScoped adds a sheet-scoped defined name.
func (w *Workbook) AddDefinedNameScoped(name, ref string, sheetIndex int) error {
	if sheetIndex < 0 || sheetIndex >= len(w.sheets) {
		return ErrSheetIndex
	}
	return w.addDefinedName(name, ref, sheetIndex)
}

func (w *Workbook) addDefinedName(name, ref string, sheetIndex int) error {
	if w.workbook.DefinedNames == nil {
		w.workbook.DefinedNames = &oxml.CT_DefinedNames{}
	}

	dn := oxml.CT_DefinedName{
		Name:  name,
		Value: ref,
	}
	if sheetIndex >= 0 {
		idx := uint32(sheetIndex)
		dn.LocalSheetId = &idx
	}

	w.workbook.DefinedNames.DefinedName = append(w.workbook.DefinedNames.DefinedName, dn)
	return nil
}

// DefinedNames returns all defined names in the workbook.
func (w *Workbook) DefinedNames() []DefinedName {
	if w.workbook.DefinedNames == nil {
		return nil
	}

	result := make([]DefinedName, len(w.workbook.DefinedNames.DefinedName))
	for i, dn := range w.workbook.DefinedNames.DefinedName {
		result[i] = DefinedName{
			Name:       dn.Name,
			Value:      dn.Value,
			SheetIndex: -1,
		}
		if dn.LocalSheetId != nil {
			result[i].SheetIndex = int(*dn.LocalSheetId)
		}
	}
	return result
}

// detectSelfClosingSpace detects whether the XML uses " />" (space before close)
// for self-closing elements, vs "/>" (no space).
func detectSelfClosingSpace(data []byte) bool {
	// Look for the first self-closing element after the root element's opening tag.
	// Find end of XML declaration, then find end of root opening tag.
	start := bytes.Index(data, []byte("?>"))
	if start < 0 {
		start = 0
	}
	// Find root element's closing >
	rootOpen := bytes.Index(data[start:], []byte(">"))
	if rootOpen < 0 {
		return false
	}
	searchFrom := start + rootOpen + 1
	// Find first /> in the body
	idx := bytes.Index(data[searchFrom:], []byte("/>"))
	if idx < 0 {
		return false
	}
	absIdx := searchFrom + idx
	return absIdx > 0 && data[absIdx-1] == ' '
}

// extractXMLSeparator extracts the bytes between the XML declaration "?>" and
// the root element "<" for preserving the exact whitespace during round-trip.
// Returns empty string if the standard "\r\n" separator is used (our default).
func extractXMLSeparator(data []byte) string {
	declEnd := bytes.Index(data, []byte("?>"))
	if declEnd < 0 {
		return ""
	}
	declEnd += 2 // past "?>"

	rootStart := bytes.IndexByte(data[declEnd:], '<')
	if rootStart < 0 {
		return ""
	}

	sep := string(data[declEnd : declEnd+rootStart])
	// Our builder's WriteHeader already writes "\r\n", so if the separator
	// matches we don't need to store it.
	if sep == "\r\n" {
		return ""
	}
	return sep
}
