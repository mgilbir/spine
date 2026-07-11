package xlsx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Workbook represents an Excel workbook.
type Workbook struct {
	// Properties contains the document properties.
	Properties opc.CoreProperties

	reader         *opc.ReadCloser
	opened         bool // true if this workbook was loaded from an existing package
	contentTypes   *opc.ContentTypes
	workbook       *oxml.CT_Workbook
	sharedStrings  *oxml.CT_Sst
	stylesheet     *oxml.CT_Stylesheet
	sheets         []*Sheet
	preservedParts map[string]*coxml.RawPart
	relationships  map[string][]*opc.Relationship
	hasCoreProps   bool
	stylesDirty    bool
	sheetsDirty    bool
	stringTable    []string // plain text values extracted from shared strings
}

// Open opens an Excel workbook from a file path.
func Open(path string) (*Workbook, error) {
	reader, err := opc.OpenReader(path)
	if err != nil {
		return nil, err
	}

	return openFromReader(reader)
}

// OpenReader opens an Excel workbook from an in-memory reader.
func OpenReader(r io.ReaderAt, size int64) (*Workbook, error) {
	reader, err := opc.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	return openFromReader(&opc.ReadCloser{Reader: *reader})
}

// openFromReader creates a Workbook from an OPC reader.
func openFromReader(reader *opc.ReadCloser) (*Workbook, error) {
	rels := reader.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 {
		_ = reader.Close()
		return nil, ErrNotXLSX
	}

	mainPartName := opc.ResolvePartName("/", rels[0].Target)
	mainPart := reader.GetFile(mainPartName)
	if mainPart == nil {
		_ = reader.Close()
		return nil, ErrNotXLSX
	}

	if mainPart.ContentType != opc.ContentTypeWorkbook {
		_ = reader.Close()
		return nil, ErrNotXLSX
	}

	data, err := mainPart.ReadAll()
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	var wb oxml.CT_Workbook
	if err := xml.Unmarshal(data, &wb); err != nil {
		_ = reader.Close()
		return nil, err
	}

	// Extract formatting details from the raw XML for byte-identical round-trip.
	wb.OriginalXMLSep = extractXMLSeparator(data)
	wb.SelfClosingSpace = detectSelfClosingSpace(data)

	w := &Workbook{
		reader:         reader,
		opened:         true,
		contentTypes:   reader.ContentTypes,
		workbook:       &wb,
		preservedParts: make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
	}

	if reader.Properties != nil {
		w.Properties = *reader.Properties
		w.hasCoreProps = true
	}

	if err := w.loadAllParts(mainPartName); err != nil {
		_ = reader.Close()
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
	data, err := w.SaveBytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SaveBytes saves the workbook to an in-memory buffer.
func (w *Workbook) SaveBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := w.SaveTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SaveTo saves the workbook to an arbitrary writer.
func (w *Workbook) SaveTo(dst io.Writer) error {
	writer := opc.NewWriter(dst)
	var err error
	// Use the durable opened flag, not w.reader: Close() releases the reader
	// but the preserved parts and models remain in memory, so a round-trip save
	// must still take the preserving path (otherwise every preserved part —
	// themes, sharedStrings, media — would be silently dropped).
	if w.opened {
		err = w.saveRoundTrip(writer)
	} else {
		err = w.saveNew(writer)
	}
	if err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

// WriteToBuffer saves the workbook to an in-memory buffer.
func (w *Workbook) WriteToBuffer() (*bytes.Buffer, error) {
	data, err := w.SaveBytes()
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(data), nil
}

// Close closes the workbook and releases resources.
func (w *Workbook) Close() error {
	if w.reader != nil {
		reader := w.reader
		w.reader = nil
		if err := reader.Close(); err != nil {
			return err
		}
	}
	return nil
}

// saveRoundTrip saves a workbook opened from a file, preserving all parts.
func (w *Workbook) saveRoundTrip(writer *opc.Writer) error {
	if w.hasCoreProps {
		writer.Properties = &w.Properties
	}

	// Preserve original content types (captured at open so this still works
	// after Close has released the reader).
	if w.contentTypes != nil {
		writer.ContentTypes = w.contentTypes
	}

	// Write core.xml as preserved raw bytes if original had it
	if w.hasCoreProps {
		if part, ok := w.preservedParts["/docProps/core.xml"]; ok {
			if err := writer.WritePart("/docProps/core.xml", part.ContentType, part.Data); err != nil {
				return err
			}
		}
	}

	worksheetParts := make(map[string]struct{}, len(w.sheets))
	anySheetDirty := w.sheetsDirty
	for _, sheet := range w.sheets {
		if sheet.dirty {
			anySheetDirty = true
		}
		if sheet.partName != "" && sheet.worksheet != nil && sheet.dirty {
			worksheetParts[sheet.partName] = struct{}{}
		}
	}
	stylesDirty := w.stylesDirty

	// A dirty sheet can add, move or remove formulas, so a preserved
	// calcChain.xml may reference cells that no longer hold one — a known
	// Excel "we found a problem" repair class (C198). Drop the part together
	// with its content-type override and workbook relationship; Excel rebuilds
	// the calculation chain transparently on next open. Untouched workbooks
	// keep their calcChain byte-identical.
	dropCalcChain := anySheetDirty
	if dropCalcChain {
		w.dropCalcChainParts("/xl/workbook.xml")
	}

	// Determine if the workbook .rels need rebuilding. We need to rebuild if
	// any sheet was modified/added/deleted or if styles were changed.
	needRelsRebuild := stylesDirty || w.sheetsDirty
	if !needRelsRebuild {
		for _, sheet := range w.sheets {
			if sheet.partName == "" || sheet.dirty {
				needRelsRebuild = true
				break
			}
		}
	}

	// Write all preserved parts except workbook.xml (which is regenerated),
	// core.xml (handled above), rewritten worksheet/style parts, and workbook/.rels
	// files (handled separately when rebuilt)
	mainPartName := "/xl/workbook.xml"
	workbookRelsName := "/xl/_rels/workbook.xml.rels"
	for name, part := range w.preservedParts {
		if name == mainPartName {
			continue
		}
		if name == "/docProps/core.xml" {
			continue
		}
		if name == workbookRelsName && needRelsRebuild {
			continue
		}
		if strings.HasSuffix(name, ".rels") && name != workbookRelsName {
			continue
		}
		if _, ok := worksheetParts[name]; ok {
			continue
		}
		if name == "/xl/styles.xml" && stylesDirty {
			continue
		}
		if err := writer.WritePart(name, part.ContentType, part.Data); err != nil {
			return err
		}
	}

	// Write non-workbook .rels files from preserved parts.
	for name, part := range w.preservedParts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		if name == workbookRelsName {
			continue
		}
		if err := writer.WritePart(name, part.ContentType, part.Data); err != nil {
			return err
		}
	}

	if needRelsRebuild {
		var wbRels []*opc.Relationship
		if existing := w.relationships[mainPartName]; len(existing) > 0 {
			wbRels = cloneRelationships(existing)
		}
		if dropCalcChain {
			wbRels = dropRelationshipsOfType(wbRels, opc.RelTypeCalcChain)
		}
		worksheetTargets := make(map[string]struct{}, len(w.sheets))
		for i, sheet := range w.sheets {
			partName, target := w.roundTripSheetPartName(sheet, i+1)
			if sheet.partName == "" {
				sheet.partName = partName
			}
			worksheetTargets[target] = struct{}{}
			if sheet.worksheet == nil || !sheet.dirty {
				continue
			}
			if err := writeSheetPart(writer, partName, sheet); err != nil {
				return err
			}
		}
		wbRels = rebuildWorksheetRelationships(wbRels, w.sheets, worksheetTargets)
		syncWorkbookSheetRefs(w.workbook, w.sheets)

		if stylesDirty {
			stylesData := marshalStylesheetXML(w.stylesheet)
			if err := writer.WritePart("/xl/styles.xml", opc.ContentTypeStyles, stylesData); err != nil {
				return err
			}
			wbRels = ensureRelationship(wbRels, opc.RelTypeStyles, "styles.xml")
		}

		if err := writer.WritePartRelationships(mainPartName, wbRels); err != nil {
			return err
		}
	}

	// Write workbook.xml (always regenerated from parsed model)
	wbData := marshalWorkbookXML(w.workbook)
	if err := writer.WritePart(mainPartName, opc.ContentTypeWorkbook, wbData); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, "xl/workbook.xml", opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// dropCalcChainParts removes the calculation-chain part(s) from the preserved
// part set along with their content-type overrides, so a save that rewrote
// sheet data does not re-emit a stale calcChain.xml (C198). Part names are
// resolved from the workbook's calcChain relationships, with the conventional
// /xl/calcChain.xml as a fallback.
func (w *Workbook) dropCalcChainParts(mainPartName string) {
	names := map[string]struct{}{"/xl/calcChain.xml": {}}
	for _, rel := range w.relationships[mainPartName] {
		if rel != nil && rel.Type == opc.RelTypeCalcChain {
			names[opc.ResolvePartName(mainPartName, rel.Target)] = struct{}{}
		}
	}
	for name := range names {
		if _, ok := w.preservedParts[name]; !ok {
			continue
		}
		delete(w.preservedParts, name)
		if w.contentTypes != nil {
			w.contentTypes.RemoveOverride(name)
		}
	}
}

// dropRelationshipsOfType removes every relationship of the given type.
func dropRelationshipsOfType(rels []*opc.Relationship, relType string) []*opc.Relationship {
	filtered := rels[:0]
	for _, rel := range rels {
		if rel != nil && rel.Type == relType {
			continue
		}
		filtered = append(filtered, rel)
	}
	return filtered
}

func (w *Workbook) roundTripSheetPartName(sheet *Sheet, fallbackIndex int) (string, string) {
	if sheet.partName != "" {
		return sheet.partName, strings.TrimPrefix(sheet.partName, "/xl/")
	}

	partName, target := nextWorksheetPartName(w.preservedParts, w.sheets, fallbackIndex)
	return partName, target
}

// saveNew saves a newly created workbook.
func (w *Workbook) saveNew(writer *opc.Writer) error {
	writer.Properties = &w.Properties

	mainPartName := "/xl/workbook.xml"
	var wbRels []*opc.Relationship
	relID := 1
	drawingCount := 0
	mediaCount := 0

	// Write each worksheet
	for i, sheet := range w.sheets {
		sheetPartName := fmt.Sprintf("/xl/worksheets/sheet%d.xml", i+1)

		// Attach a drawing part to the worksheet model before it is marshalled
		// so the <drawing> element is emitted.
		if len(sheet.images) > 0 {
			drawingCount++
			if sheet.worksheet == nil {
				sheet.worksheet = &oxml.CT_Worksheet{SheetData: oxml.CT_SheetData{}}
			}
			sheet.worksheet.Drawing = &oxml.CT_Drawing{RID: "rId1"}
		}

		if err := writeSheetPart(writer, sheetPartName, sheet); err != nil {
			return err
		}

		if len(sheet.images) > 0 {
			if err := writeSheetDrawing(writer, sheetPartName, sheet, drawingCount, &mediaCount); err != nil {
				return err
			}
		}

		rid := fmt.Sprintf("rId%d", relID)
		wbRels = append(wbRels, &opc.Relationship{
			ID:     rid,
			Type:   opc.RelTypeWorksheet,
			Target: fmt.Sprintf("worksheets/sheet%d.xml", i+1),
		})
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
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, "xl/workbook.xml", opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// writeSheetPart writes a worksheet part from the worksheet model.
func writeSheetPart(writer *opc.Writer, partName string, sheet *Sheet) error {
	if sheet.worksheet == nil {
		sheet.worksheet = &oxml.CT_Worksheet{
			SheetData: oxml.CT_SheetData{},
		}
	}

	wsData := marshalWorksheetXML(sheet.worksheet)
	return writer.WritePart(partName, opc.ContentTypeWorksheet, wsData)
}

// writeSheetDrawing emits the drawing part, the media parts and the
// relationships that anchor a sheet's images. It writes:
//   - the worksheet -> drawing relationship (rId1 in the sheet's rels),
//   - one /xl/media/imageN.<ext> part per image,
//   - the /xl/drawings/drawing<idx>.xml part, and
//   - the drawing -> image relationships.
//
// mediaCount is a workbook-wide counter so media part names stay unique across
// sheets.
func writeSheetDrawing(writer *opc.Writer, sheetPartName string, sheet *Sheet, drawingIndex int, mediaCount *int) error {
	drawingPartName := fmt.Sprintf("/xl/drawings/drawing%d.xml", drawingIndex)

	// Worksheet -> drawing relationship (matches CT_Drawing.RID = "rId1").
	if err := writer.WritePartRelationships(sheetPartName, []*opc.Relationship{{
		ID:     "rId1",
		Type:   opc.RelTypeDrawing,
		Target: fmt.Sprintf("../drawings/drawing%d.xml", drawingIndex),
	}}); err != nil {
		return err
	}

	// Media parts + drawing -> image relationships.
	drawingRels := make([]*opc.Relationship, 0, len(sheet.images))
	for i := range sheet.images {
		*mediaCount++
		img := sheet.images[i]
		mediaPartName := fmt.Sprintf("/xl/media/image%d.%s", *mediaCount, img.ext)
		if err := writer.WritePart(mediaPartName, img.contentType, img.data); err != nil {
			return err
		}
		drawingRels = append(drawingRels, &opc.Relationship{
			ID:     relIDForImage(i),
			Type:   opc.RelTypeImage,
			Target: fmt.Sprintf("../media/image%d.%s", *mediaCount, img.ext),
		})
	}

	// Drawing part + its relationships.
	if err := writer.WritePart(drawingPartName, opc.ContentTypeDrawing, marshalDrawingXML(sheet.images)); err != nil {
		return err
	}
	return writer.WritePartRelationships(drawingPartName, drawingRels)
}

func cloneRelationships(rels []*opc.Relationship) []*opc.Relationship {
	if len(rels) == 0 {
		return nil
	}
	cloned := make([]*opc.Relationship, 0, len(rels))
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		copyRel := *rel
		cloned = append(cloned, &copyRel)
	}
	return cloned
}

func rebuildWorksheetRelationships(existing []*opc.Relationship, sheets []*Sheet, worksheetTargets map[string]struct{}) []*opc.Relationship {
	filtered := make([]*opc.Relationship, 0, len(existing)+len(sheets))
	for _, rel := range existing {
		if rel == nil {
			continue
		}
		if rel.Type == opc.RelTypeWorksheet {
			continue
		}
		filtered = append(filtered, rel)
	}

	usedIDs := make(map[string]struct{}, len(filtered))
	for _, rel := range filtered {
		usedIDs[rel.ID] = struct{}{}
	}

	for _, sheet := range sheets {
		partName := sheet.partName
		if partName == "" {
			continue
		}
		target := strings.TrimPrefix(partName, "/xl/")
		if _, ok := worksheetTargets[target]; !ok {
			continue
		}
		id := sheet.relID
		if id == "" || relationshipIDInUse(usedIDs, id) {
			id = fmt.Sprintf("rId%d", nextRelationshipID(usedIDs))
		}
		usedIDs[id] = struct{}{}
		sheet.relID = id
		filtered = append(filtered, &opc.Relationship{
			ID:     id,
			Type:   opc.RelTypeWorksheet,
			Target: target,
		})
	}

	return filtered
}

func ensureRelationship(rels []*opc.Relationship, relType, target string) []*opc.Relationship {
	for _, rel := range rels {
		if rel != nil && rel.Type == relType && rel.Target == target {
			return rels
		}
	}
	usedIDs := make(map[string]struct{}, len(rels))
	for _, rel := range rels {
		if rel != nil {
			usedIDs[rel.ID] = struct{}{}
		}
	}
	return append(rels, &opc.Relationship{
		ID:     fmt.Sprintf("rId%d", nextRelationshipID(usedIDs)),
		Type:   relType,
		Target: target,
	})
}

func syncWorkbookSheetRefs(wb *oxml.CT_Workbook, sheets []*Sheet) {
	if wb == nil {
		return
	}
	for i := range sheets {
		if i >= len(wb.Sheets.Sheet) {
			break
		}
		wb.Sheets.Sheet[i].Name = sheets[i].name
		wb.Sheets.Sheet[i].RID = sheets[i].relID
	}
}

func nextRelationshipID(used map[string]struct{}) int {
	nextID := 1
	for {
		candidate := fmt.Sprintf("rId%d", nextID)
		if _, ok := used[candidate]; !ok {
			return nextID
		}
		nextID++
	}
}

func relationshipIDInUse(used map[string]struct{}, id string) bool {
	if id == "" {
		return false
	}
	_, ok := used[id]
	return ok
}

func nextWorksheetPartName(preserved map[string]*coxml.RawPart, sheets []*Sheet, fallbackIndex int) (string, string) {
	used := make(map[string]struct{}, len(preserved)+len(sheets))
	for name := range preserved {
		used[name] = struct{}{}
	}
	for _, sheet := range sheets {
		if sheet.partName != "" {
			used[sheet.partName] = struct{}{}
		}
	}

	for idx := fallbackIndex; ; idx++ {
		partName := fmt.Sprintf("/xl/worksheets/sheet%d.xml", idx)
		if _, ok := used[partName]; ok {
			continue
		}
		return partName, fmt.Sprintf("worksheets/sheet%d.xml", idx)
	}
}

// Styles returns the StyleManager for this workbook. If no stylesheet exists
// yet (e.g. for a newly created workbook), a default one is created and styles
// are marked dirty. Merely reading styles from an existing workbook does not
// mark them dirty (which would force styles.xml to be regenerated and break
// byte-identical round-trip); the returned manager marks styles dirty only when
// a mutating method is called.
func (w *Workbook) Styles() *StyleManager {
	if w.stylesheet == nil {
		w.stylesheet = defaultStylesheet()
		w.stylesDirty = true
	}
	return newStyleManager(w.stylesheet, func() { w.stylesDirty = true })
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

// AddSheet adds a new sheet to the workbook. The name is coerced to a valid,
// unique Excel sheet name (see ValidateSheetName); use the returned sheet's
// Name to observe the final name.
func (w *Workbook) AddSheet(name string) *Sheet {
	name = w.sanitizeSheetName(name)
	ws := &oxml.CT_Worksheet{
		SheetData: oxml.CT_SheetData{},
	}

	sheet := &Sheet{
		workbook:  w,
		name:      name,
		index:     len(w.sheets),
		worksheet: ws,
		dirty:     true,
	}
	w.sheets = append(w.sheets, sheet)
	w.sheetsDirty = true

	// Update the workbook model. SheetId must be unique across the workbook's
	// lifetime — allocating len(sheets) collides after a delete+add, so use one
	// past the current maximum.
	sheetID := w.nextSheetID()
	w.workbook.Sheets.Sheet = append(w.workbook.Sheets.Sheet, oxml.CT_Sheet{
		Name:    name,
		SheetId: sheetID,
		RID:     fmt.Sprintf("rId%d", sheetID),
	})

	return sheet
}

// nextSheetID returns an unused sheet id (one past the current maximum).
func (w *Workbook) nextSheetID() uint32 {
	var max uint32
	for _, s := range w.workbook.Sheets.Sheet {
		if s.SheetId > max {
			max = s.SheetId
		}
	}
	return max + 1
}

// forbiddenSheetNameChars are the characters Excel disallows in a sheet name.
const forbiddenSheetNameChars = `\/?*[]:`

// ValidateSheetName reports whether name is a legal Excel sheet name: non-empty,
// at most 31 characters, containing none of \ / ? * [ ] :, and not beginning or
// ending with an apostrophe. It does not check uniqueness within a workbook.
func ValidateSheetName(name string) error {
	if name == "" {
		return fmt.Errorf("xlsx: sheet name must not be empty")
	}
	if utf8.RuneCountInString(name) > 31 {
		return fmt.Errorf("xlsx: sheet name %q exceeds 31 characters", name)
	}
	if strings.ContainsAny(name, forbiddenSheetNameChars) {
		return fmt.Errorf(`xlsx: sheet name %q contains a forbidden character (\ / ? * [ ] :)`, name)
	}
	if strings.HasPrefix(name, "'") || strings.HasSuffix(name, "'") {
		return fmt.Errorf("xlsx: sheet name %q must not start or end with an apostrophe", name)
	}
	return nil
}

// sanitizeSheetName coerces name into a valid, unique sheet name, mirroring how
// Excel repairs invalid names: forbidden characters are stripped, the result is
// trimmed to 31 characters and made non-empty, and a numeric suffix is appended
// if the name collides (case-insensitively) with an existing sheet.
func (w *Workbook) sanitizeSheetName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if !strings.ContainsRune(forbiddenSheetNameChars, r) {
			b.WriteRune(r)
		}
	}
	name = strings.Trim(strings.TrimSpace(b.String()), "'")
	if name == "" {
		name = "Sheet"
	}
	name = truncateRunes(name, 31)
	if !w.sheetNameExists(name) {
		return name
	}
	base := name
	for i := 2; ; i++ {
		suffix := fmt.Sprintf(" (%d)", i)
		cand := truncateRunes(base, 31-utf8.RuneCountInString(suffix)) + suffix
		if !w.sheetNameExists(cand) {
			return cand
		}
	}
}

// sheetNameExists reports whether a sheet with the given name (case-insensitive)
// already exists in the workbook.
func (w *Workbook) sheetNameExists(name string) bool {
	for _, s := range w.sheets {
		if strings.EqualFold(s.name, name) {
			return true
		}
	}
	return false
}

// truncateRunes returns s limited to at most n runes.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// DeleteSheet removes the sheet at the specified index.
func (w *Workbook) DeleteSheet(index int) error {
	if index < 0 || index >= len(w.sheets) {
		return ErrSheetIndex
	}
	if sheet := w.sheets[index]; sheet != nil && sheet.partName != "" {
		delete(w.preservedParts, sheet.partName)
	}
	w.sheets = append(w.sheets[:index], w.sheets[index+1:]...)
	for i := index; i < len(w.sheets); i++ {
		w.sheets[i].index = i
	}
	w.sheetsDirty = true

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

// Worksheet grid limits (Excel 2007+): 1,048,576 rows by 16,384 columns (XFD).
const (
	MaxRow = 1048576
	MaxCol = 16384
)

// ParseCellRef parses a cell reference like "A1" into 1-based row and column
// numbers. It rejects references outside the worksheet grid and guards against
// integer overflow from pathologically long column strings.
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

	// Parse column letters to number, rejecting anything past the last column
	// as soon as it overflows the grid (which also prevents int overflow).
	col = 0
	for _, c := range colStr {
		col = col*26 + int(c-'A'+1)
		if col > MaxCol {
			return 0, 0, ErrInvalidCell
		}
	}

	// Parse row number
	row, err = strconv.Atoi(rowStr)
	if err != nil || row < 1 || row > MaxRow {
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
