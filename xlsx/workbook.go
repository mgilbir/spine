package xlsx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Workbook represents an Excel workbook.
type Workbook struct {
	// Properties contains the document properties.
	Properties opc.CoreProperties

	reader *opc.ReadCloser
	opened bool // true if this workbook was loaded from an existing package
	// mainPartName is the resolved name of the main workbook part from the
	// package root relationships (usually /xl/workbook.xml, but non-standard
	// names such as /xl/book.xml occur in the wild). The save paths regenerate
	// THIS part; hardcoding /xl/workbook.xml would preserve the stale original
	// and silently drop every edit (C231).
	mainPartName string
	// flavor is the main part's content type as recorded at open: one of the
	// SpreadsheetML main-part flavors (workbook, template, or a macro-enabled
	// workbook/template/add-in). Empty for a created workbook, which saves as
	// a regular workbook. The save paths re-emit this content type so e.g. a
	// macro-enabled workbook (.xlsm) is not silently retyped to a regular
	// workbook while still carrying its vbaProject part — a combination that
	// makes Excel flag the file.
	flavor         string
	contentTypes   *opc.ContentTypes
	workbook       *oxml.CT_Workbook
	sharedStrings  *oxml.CT_Sst
	stylesheet     *oxml.CT_Stylesheet
	sheets         []*Sheet
	preservedParts map[string]*coxml.RawPart
	relationships  map[string][]*opc.Relationship
	hasCoreProps   bool
	propsSnapshot  *opc.CoreProperties   // Properties as loaded at open; detects edits at save
	customProps    *opc.CustomProperties // user-defined properties (docProps/custom.xml), nil when none
	customSnapshot *opc.CustomProperties // custom props as loaded at open; detects edits at save
	hasCustomPart  bool                  // whether the opened package carried docProps/custom.xml
	stylesDirty    bool
	sheetsDirty    bool
	// deletedParts names every part removed this session (DeleteSheet and its
	// cascade). The save path must never emit a relationship resolving to one
	// of them, and Validate resolves part existence against this set as well as
	// the source reader — otherwise a deletion-induced dangling reference is
	// invisible to the pre-save gate by construction (C366).
	deletedParts map[string]bool
	// persons is the workbook-shared threaded-comment author list (loaded
	// lazily from xl/persons/personN.xml). personsPartName is the existing
	// part name to reuse when regenerating, or "" if none existed.
	persons         *oxml.CT_PersonList
	personsPartName string
	personsLoaded   bool
	personsDirty    bool
	stringTable     []string // plain text values extracted from shared strings
	// dirEntries preserves the source archive's zip directory entries
	// (Reader.DirectoryEntries) so a round-trip save re-emits them.
	dirEntries []string
	// theme is the lazily resolved read/write theme handle; themeResolved
	// records that the lookup ran (the handle stays nil when the workbook has
	// no theme part). See theme.go.
	theme         *dml.ThemeEditor
	themeResolved bool
	themePartName string // theme part name, set when theme resolves to a part
	// pendingPivotCaches accumulates, during a save pass, the workbook-level
	// pivot cache definition parts written for session-added pivot tables. It is
	// rebuilt from scratch on each save (reset in saveOpenedSheetAttachments) and
	// consumed when the workbook relationships and <pivotCaches> element are
	// finalized.
	pendingPivotCaches []pendingPivotCache

	// vbaModified records that SetVBAProject or RemoveVBAProject ran this
	// session; vbaRemove distinguishes removal from injection. When set, the
	// round-trip save forces the workbook .rels rebuild, writes/drops the VBA
	// part, and re-emits the flipped main-part flavor. A zero-modification save
	// leaves them false and any existing vbaProject.bin round-trips
	// byte-for-byte among the preserved parts. See vba.go.
	vbaModified bool
	vbaRemove   bool
	vbaData     []byte // injected/replacement VBA project bytes (nil when removing)
	vbaPartName string // resolved vbaProject.bin part name

	// contentEdits counts the content changes made through this workbook's API;
	// savedContentEdits is the count the last completed save persisted. A save
	// stamps Properties.Modified exactly when the two differ, so an untouched
	// round-trip stays byte-identical while a real edit records an accurate
	// write time. They are counters rather than a flag because none of the
	// regeneration flags above is ever cleared, which makes "one edit saved
	// twice" and "two edits either side of a save" indistinguishable. See
	// markContentEdited and contentChanged in modified.go.
	contentEdits      int
	savedContentEdits int
	// stampedModified is the Properties.Modified value stampModified wrote on a
	// previous save of this workbook, so a caller's own assignment can still be
	// told apart from ours after Modified has stopped matching propsSnapshot.
	// Zero when this workbook has never stamped.
	stampedModified time.Time
}

// pendingPivotCache records one pivot cache definition part written this save
// so the workbook relationship and <pivotCaches> entry can be wired with a
// matching relationship id.
type pendingPivotCache struct {
	cacheID uint32
	target  string // workbook-relative target, e.g. "pivotCache/pivotCacheDefinition1.xml"
}

// Open opens an Excel workbook from a file path. The whole package is read into
// memory, so the returned Workbook retains no OS file handle and Close is
// effectively a no-op — the same resource model as docx.Open and pptx.Open.
// (Before C570 this alone among the three kept the source file open until
// Close, an xlsx-only descriptor leak for callers trained by the other two;
// nothing needed the handle, because every part is read up front regardless.)
// Sheet contents are held as preserved bytes and parsed lazily on first access.
//
// Options configure the underlying package reader: opc.WithPassword opens a
// password-encrypted workbook, and the opc.WithMax* options adjust the bounds
// that guard against decompression bombs.
//
// It returns ErrNotXLSX when the package is not SpreadsheetML,
// opc.ErrStrictOOXML for an ISO-Strict package, and opc.ErrEncrypted when the
// input is password-encrypted and no opc.WithPassword was given. Each is
// matchable with errors.Is.
func Open(path string, opts ...opc.ReaderOption) (*Workbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return OpenReader(bytes.NewReader(data), int64(len(data)), opts...)
}

// OpenReader opens an Excel workbook from an in-memory reader. Every part is
// read into memory during the call (worksheet models are then parsed lazily on
// first access), so r need not remain valid after Open returns, and no OS file
// handle is retained. It takes the same options and returns the same sentinels
// as Open (ErrNotXLSX, opc.ErrStrictOOXML, opc.ErrEncrypted), matchable with
// errors.Is. Open is implemented on top of it.
func OpenReader(r io.ReaderAt, size int64, opts ...opc.ReaderOption) (*Workbook, error) {
	reader, err := opc.NewReader(r, size, opts...)
	if err != nil {
		return nil, err
	}

	return openFromReader(&opc.ReadCloser{Reader: *reader})
}

// openFromReader creates a Workbook from an OPC reader.
func openFromReader(reader *opc.ReadCloser) (*Workbook, error) {
	rels := reader.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 {
		// An ISO-Strict workbook carries the officeDocument relationship under
		// the purl.oclc.org namespace; report it distinctly rather than as a
		// generic "not an Excel file".
		if len(reader.GetRelationshipsByType(opc.RelTypeOfficeDocumentStrict)) > 0 {
			_ = reader.Close()
			return nil, opc.ErrStrictOOXML
		}
		_ = reader.Close()
		return nil, ErrNotXLSX
	}

	mainPartName := opc.ResolvePartName("/", rels[0].Target)
	mainPart := reader.GetFile(mainPartName)
	if mainPart == nil {
		_ = reader.Close()
		return nil, ErrNotXLSX
	}

	// Any SpreadsheetML main-part flavor is accepted (regular workbook,
	// template, and the macro-enabled workbook/template/add-in variants); the
	// flavor is recorded so the save re-emits it.
	if !workbookFlavors[mainPart.ContentType] {
		_ = reader.Close()
		return nil, ErrNotXLSX
	}

	data, err := mainPart.ReadAll()
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	var wb oxml.CT_Workbook
	// Give the unmarshal access to the raw part bytes so unknown children
	// (e.g. xr:revisionPtr) are captured verbatim instead of re-encoded.
	//
	// Only when the decoder's input offsets still index these bytes: a part
	// declaring a single-byte code page is transcoded to UTF-8 on read, so the
	// offsets index the transcoded stream and slicing data at them would splice
	// misaligned fragments into the always-regenerated workbook.xml (C369).
	// Without a source the capture paths fall back to canonical regeneration.
	if xmlb.OffsetCaptureSafe(data) {
		wb.RawSource = data
	}
	if err := xmlb.UnmarshalWithSource(data, &wb); err != nil {
		_ = reader.Close()
		return nil, err
	}

	// Extract formatting details from the raw XML for byte-identical round-trip.
	wb.Prolog = xmlb.CaptureProlog(data)
	// workbook.xml is always regenerated, and regeneration emits UTF-8. Replaying
	// a transcoded part's declaration verbatim would label UTF-8 bytes with the
	// source code page, so rewrite the encoding pseudo-attribute (C369).
	if !xmlb.OffsetCaptureSafe(data) {
		wb.Prolog.Decl = declEncodingUTF8(wb.Prolog.Decl)
	}
	wb.SelfClosingSpace = xmlb.DetectSelfClosingSpace(data)

	w := &Workbook{
		reader:         reader,
		opened:         true,
		mainPartName:   mainPartName,
		flavor:         mainPart.ContentType,
		contentTypes:   reader.ContentTypes,
		workbook:       &wb,
		preservedParts: make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
		dirEntries:     reader.DirectoryEntries,
	}

	if reader.Properties != nil {
		w.Properties = *reader.Properties
		w.hasCoreProps = true
		w.propsSnapshot = reader.Properties.Clone()
	}

	if reader.CustomProperties != nil {
		w.customProps = reader.CustomProperties
		w.customSnapshot = reader.CustomProperties.Clone()
		w.hasCustomPart = true
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
			// A part the model parses must not silently vanish (C60): its
			// absence would fabricate content on the next save (an empty
			// string table, default styles). Other unreadable parts are
			// tolerated here — worksheets referenced from workbook.xml are
			// checked in loadSheets, and genuinely unreferenced damaged
			// entries only drop out of the preserved set.
			if name == "/xl/sharedStrings.xml" || name == "/xl/styles.xml" || name == mainPartName {
				return fmt.Errorf("xlsx: reading part %s: %w", name, err)
			}
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
			if err := xmlb.Unmarshal(data, w.sharedStrings); err != nil {
				return fmt.Errorf("xlsx: parsing %s: %w", name, err)
			}
		case name == "/xl/styles.xml":
			w.stylesheet = &oxml.CT_Stylesheet{}
			// An empty (0-byte or whitespace-only) styles part is tolerated as
			// an empty stylesheet, matching Excel: it opens such files treating
			// styles as defaults. Real Common Crawl files ship a 0-byte
			// xl/styles.xml. Only the empty case is swallowed — a non-empty but
			// malformed styles.xml is genuine corruption and still errors. The
			// raw part is preserved above, so a zero-modification save re-emits
			// the original 0-byte part byte-for-byte (stylesDirty stays false
			// because the empty model is non-nil and is never mutated on read).
			if len(bytes.TrimSpace(data)) == 0 {
				break
			}
			if err := xmlb.Unmarshal(data, w.stylesheet); err != nil {
				return fmt.Errorf("xlsx: parsing %s: %w", name, err)
			}
		default:
			// preserved in preservedParts
		}
	}

	// Build string table from shared strings
	w.buildStringTable()

	// Load worksheets using the sheet order from workbook.xml
	return w.loadSheets(mainPartName)
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

// loadSheets loads worksheets in the order defined by workbook.xml sheets
// element. A referenced sheet part that fails to resolve, read, or parse is an
// Open error (C60/C78): a silently-empty sheet model would replace the
// original part with a fabricated near-empty sheet on the first save after
// any mutation, destroying the original data.
func (w *Workbook) loadSheets(mainPartName string) error {
	wbRels := w.relationships[mainPartName]

	for i, sheetDef := range w.workbook.Sheets.Sheet {
		// Resolve the sheet's part name from its r:id, keeping the relationship
		// type so non-worksheet sheets (chartsheet/dialogsheet) can be detected.
		partName := ""
		relType := ""
		for _, rel := range wbRels {
			if rel.ID == sheetDef.RID {
				partName = opc.ResolvePartName(mainPartName, rel.Target)
				relType = rel.Type
				break
			}
		}
		if partName == "" {
			return fmt.Errorf("xlsx: sheet %q references relationship %q, which does not exist in the workbook relationships", sheetDef.Name, sheetDef.RID)
		}

		part, ok := w.preservedParts[partName]
		if !ok {
			// Distinguish an unreadable part from a missing one so the error
			// names the actual failure.
			if f := w.reader.GetFile(partName); f != nil {
				if _, err := f.ReadAll(); err != nil {
					return fmt.Errorf("xlsx: reading sheet part %s: %w", partName, err)
				}
			}
			return fmt.Errorf("xlsx: sheet %q references missing worksheet part %s", sheetDef.Name, partName)
		}

		// A <sheet> may resolve to a chartsheet/dialogsheet/macrosheet part, whose
		// root is not <worksheet>. Such a sheet is preserved opaquely: its part
		// bytes, its (correctly typed) relationship and its content-type override
		// round-trip verbatim, it is excluded from the worksheet relationship
		// rebuild, and the worksheet mutation API refuses it. Parsing it as a
		// worksheet, or emitting a worksheet-typed relationship for it, corrupts
		// the file on the first save after any edit (C241).
		opaque := isNonWorksheetSheet(part.ContentType, relType)

		sheet := &Sheet{
			workbook: w,
			name:     sheetDef.Name,
			index:    i,
			partName: partName,
			relID:    sheetDef.RID,
			state:    sheetDef.State,
			opaque:   opaque,
		}

		// Validate a worksheet up front (C60/C78: a malformed sheet must fail
		// Open, not silently become an empty model that fabricates content on a
		// later save), but discard the model — it is re-parsed lazily on first
		// access (see Sheet.ws). A workbook that is round-tripped unmodified
		// then holds only the raw sheet bytes, not a full model per sheet. An
		// opaque (non-worksheet) sheet is never parsed as a worksheet.
		if !opaque {
			if err := xmlb.Unmarshal(part.Data, &oxml.CT_Worksheet{}); err != nil {
				return fmt.Errorf("xlsx: parsing sheet part %s: %w", partName, err)
			}
		}

		w.sheets = append(w.sheets, sheet)
	}
	return nil
}

// isNonWorksheetSheet reports whether a workbook <sheet> entry resolves to a part
// that is not an ordinary worksheet — a chartsheet, dialogsheet or (Excel 4.0)
// macrosheet. The content type recorded for the part is authoritative; the
// relationship type is a fallback for packages that omit the override.
func isNonWorksheetSheet(contentType, relType string) bool {
	switch contentType {
	case opc.ContentTypeChartsheet, opc.ContentTypeDialogsheet,
		opc.ContentTypeMacrosheet, opc.ContentTypeIntlMacrosheet:
		return true
	}
	switch relType {
	case opc.RelTypeChartsheet, opc.RelTypeDialogsheet:
		return true
	}
	return false
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

	w := &Workbook{
		workbook:       wb,
		mainPartName:   defaultMainPartName,
		preservedParts: make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
	}
	// Snapshot the (empty) properties so a caller's own Properties.Modified
	// assignment can be told from the value a save of this workbook stamped
	// (see stampModified); without it a created workbook's explicit timestamp
	// would be overwritten by the first save that records content.
	w.propsSnapshot = w.Properties.Clone()
	return w
}

// defaultMainPartName is the conventional name of the main workbook part,
// used for created workbooks.
const defaultMainPartName = "/xl/workbook.xml"

// mainPart returns the name of the main workbook part: the one resolved from
// the root relationships at open, or the default for created workbooks.
func (w *Workbook) mainPart() string {
	if w.mainPartName != "" {
		return w.mainPartName
	}
	return defaultMainPartName
}

// workbookFlavors is the set of SpreadsheetML main-part content types
// (ECMA-376 and [MS-OFFDI]) that Open accepts: regular workbook (.xlsx),
// template (.xltx), and the macro-enabled workbook (.xlsm), template (.xltm),
// and add-in (.xlam) variants.
var workbookFlavors = map[string]bool{
	opc.ContentTypeWorkbook:                  true,
	opc.ContentTypeWorkbookTemplateMain:      true,
	opc.ContentTypeWorkbookMacroMain:         true,
	opc.ContentTypeWorkbookTemplateMacroMain: true,
	opc.ContentTypeWorkbookAddinMacroMain:    true,
}

// Flavor returns the main part's content type: one of the SpreadsheetML
// flavors (opc.ContentTypeWorkbook, opc.ContentTypeWorkbookTemplateMain, or a
// macro-enabled variant). An opened file reports the flavor it was opened
// with — a macro-enabled workbook (.xlsm) stays macro-enabled across a save —
// and a created workbook reports opc.ContentTypeWorkbook. There is no
// conversion API: retyping a file to another flavor is out of scope.
func (w *Workbook) Flavor() string {
	if w.flavor != "" {
		return w.flavor
	}
	return opc.ContentTypeWorkbook
}

// Save writes the workbook to a file. Like SaveTo, it enforces the pre-save
// validation gate and the round-trip contract documented there.
func (w *Workbook) Save(path string) error {
	data, err := w.SaveBytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SaveBytes writes the workbook to an in-memory buffer through SaveTo (same
// validation gate and round-trip contract).
func (w *Workbook) SaveBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := w.SaveTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SaveTo saves the workbook to an arbitrary writer.
//
// A workbook must contain at least one sheet (Excel refuses zero-sheet files),
// so saving an empty workbook returns ErrNoSheets (C130). It then runs Validate
// and refuses to write — returning the Report as an error — when any
// error-severity finding is present, so a structurally corrupt package is never
// produced. SaveToUnvalidated bypasses the validation gate (but still enforces
// the non-empty-sheet invariant).
//
// Round-trip contract: for a workbook opened with Open/OpenReader, parts the
// session never touched are written back byte-for-byte — including sheets that
// were never accessed, which are never even parsed — while touched parts are
// regenerated from the model. A workbook built with Create is generated entirely
// from the model. This holds after Close, too (see Close).
//
// Modification time: when the session changed the workbook's content, the save
// records its own time in Properties.Modified (docProps/core.xml,
// dcterms:modified). When it did not — including a session that only read the
// workbook, and a repeat save of one already written — the timestamp is left
// exactly as it was, so an unchanged save still reproduces the package
// byte-for-byte. Assigning Properties.Modified yourself takes precedence over
// the automatic value. See modified.go.
func (w *Workbook) SaveTo(dst io.Writer) error {
	if len(w.sheets) == 0 {
		return ErrNoSheets
	}
	if report := w.Validate(); report.HasErrors() {
		return report
	}
	return w.SaveToUnvalidated(dst)
}

// SaveToUnvalidated saves the workbook without running the pre-save validation
// pass (it still enforces the non-empty-sheet invariant). Prefer SaveTo; use
// this only when a finding is known to be advisory for the caller's use case.
func (w *Workbook) SaveToUnvalidated(dst io.Writer) error {
	if len(w.sheets) == 0 {
		return ErrNoSheets
	}
	// Fold any theme edit back into the preserved theme bytes before change
	// detection reads them: that is the single point where a Theme() edit
	// becomes a recorded content change, and an untouched theme is left
	// byte-identical.
	w.applyThemeEdits()
	// Record the write time in dcterms:modified, but only when this session
	// actually changed something. Stamping unconditionally would make SaveBytes
	// non-idempotent; never stamping loses a real edit's write time. Must run
	// before either save path, because whether the stamp happened is also what
	// decides between regenerating docProps/core.xml and preserving its bytes.
	w.stampModified()
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
		// Abort, not Close: Close would finalize the half-written package as
		// if it were good; the output must be discarded either way.
		_ = writer.Abort()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	// The package now on the wire is the new baseline for change detection, so
	// saving the same workbook again reproduces these bytes rather than
	// reporting the edits it has just written out as still outstanding.
	w.noteSaved()
	return nil
}

// WriteToBuffer saves the workbook to an in-memory buffer.
//
// Deprecated: use SaveBytes, which returns the same bytes without the buffer
// wrapper, or SaveTo for an arbitrary io.Writer. This is SaveBytes wrapped in a
// *bytes.Buffer and has no counterpart in docx or pptx (C567).
func (w *Workbook) WriteToBuffer() (*bytes.Buffer, error) {
	data, err := w.SaveBytes()
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(data), nil
}

// Close releases the workbook's underlying resources. Since C570 no open path
// retains an OS file handle — Open reads the file into memory exactly as
// docx.Open and pptx.Open do — so this is effectively a no-op, kept because it
// is part of the API and because it drops the workbook's reference to the
// source package reader. It remains safe (and unnecessary) to call.
//
// Calling Save (or any Save* method) after Close is valid: the preserved parts
// and parsed models stay in memory and a durable internal flag — not the
// reader — keeps the round-trip save path, so Close does not turn a saved
// workbook into a from-scratch regeneration.
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
	// Hand the writer core properties only when the source stored them at
	// /docProps/core.xml (where the preserved raw part plus the writer's
	// skip-if-written rule keeps fidelity) or when the session edited them.
	// Some producers (System.IO.Packaging) keep core properties in a
	// *.psmdcp part instead; that part round-trips verbatim among the
	// preserved parts, and setting writer.Properties would synthesize a
	// docProps/core.xml the source never had.
	_, hasCorePart := w.preservedParts["/docProps/core.xml"]
	if w.hasCoreProps && (hasCorePart || !w.Properties.Equal(w.propsSnapshot)) {
		writer.Properties = &w.Properties
	}

	// Custom properties: an unmodified docProps/custom.xml round-trips as its
	// preserved raw bytes. When the session edited or added properties the
	// writer regenerates the part; the preserved copy is skipped below, and a
	// part created this session has its package relationship injected into the
	// preserved _rels/.rels (its content-type override is registered on the
	// cloned content types). See customPropertiesModified.
	customModified := w.customPropertiesModified()
	if customModified {
		writer.CustomProperties = w.customProps
	}

	// Re-emit the source archive's directory entries (some producers write
	// them; OPC ignores them but a faithful save keeps the entry listing).
	if err := writer.WriteDirectoryEntries(w.dirEntries); err != nil {
		return err
	}

	// A dirty sheet can add, move or remove formulas, so a preserved
	// calcChain.xml may reference cells that no longer hold one — a known
	// Excel "we found a problem" repair class (C198). Drop the part together
	// with its content-type override and workbook relationship; Excel rebuilds
	// the calculation chain transparently on next open. Untouched workbooks
	// keep their calcChain byte-identical. Sheets that receive images in
	// saveOpenedSheetImages below count as dirty: attaching the drawing
	// reference re-marshals them. This mutates the durable model (preserved
	// parts and w.contentTypes), so it must run before the writer clones the
	// content types below.
	dropCalcChain := w.sheetsDirty || w.sheetsHaveImages() || w.sheetsHaveCharts() || w.sheetsHaveComments() || w.sheetsHaveTables() || w.sheetsHavePivots() || w.sheetsHaveOLE()
	if !dropCalcChain {
		for _, sheet := range w.sheets {
			if sheet.dirty {
				dropCalcChain = true
				break
			}
		}
	}
	if dropCalcChain {
		w.dropCalcChainParts(w.mainPart())
	}

	// Preserve original content types (captured at open so this still works
	// after Close has released the reader). Hand the writer a clone: Close
	// mutates its ContentTypes (SetOverride for regenerated metadata parts),
	// so sharing the captured instance would let repeated saves observe each
	// other's side effects and make concurrent saves race. The clone must be
	// in place before any part is written, so overrides recorded by WritePart
	// (e.g. for the image drawing/media parts below) land in this save's
	// content types instead of being discarded.
	if w.contentTypes != nil {
		writer.ContentTypes = w.contentTypes.Clone()
	}

	// Images and comments added to the opened workbook: write their parts first
	// so the sheets they belong to are dirtied before worksheetParts and
	// needRelsRebuild are computed below. The returned set names the sheet
	// .rels parts rebuilt here, so the verbatim stream skips their stale
	// originals; personRelTarget names a regenerated person list to wire into
	// the workbook .rels.
	var rebuiltRels, rebuiltParts map[string]bool
	var personRelTarget string
	if w.sheetsHaveImages() || w.sheetsHaveCharts() || w.sheetsHaveComments() || w.sheetsHavePendingHyperlinkRels() || w.sheetsHaveTables() || w.sheetsHavePivots() || w.sheetsHaveOLE() {
		var err error
		rebuiltRels, rebuiltParts, personRelTarget, err = w.saveOpenedSheetAttachments(writer)
		if err != nil {
			return err
		}
	}

	// Write core.xml as preserved raw bytes when the in-memory properties still
	// match the snapshot taken at open (producer formatting varies, so
	// regenerating drifts), or when they were never parsed at all (malformed
	// core-properties relationship type: the part must round-trip verbatim).
	// Writing the raw part would otherwise win under the opc writer's
	// skip-if-written rule and silently drop the edits.
	//
	// Edited properties — including the dcterms:modified stamp a content change
	// applies — regenerate the part, but it is written HERE rather than left to
	// Close, which appends it after everything else. The source stored core.xml
	// at this position, and moving it means saving an edited workbook and then
	// re-saving the reopened result — which preserves the part where it was
	// found — produces a different archive for the same content.
	if part, ok := w.preservedParts["/docProps/core.xml"]; ok {
		if !w.hasCoreProps || w.Properties.Equal(w.propsSnapshot) {
			if err := writer.WritePreservedPart("/docProps/core.xml", part.ContentType, part.Data); err != nil {
				return err
			}
		} else {
			data, err := w.Properties.Marshal()
			if err != nil {
				return err
			}
			if err := writer.WritePart("/docProps/core.xml", part.ContentType, data); err != nil {
				return err
			}
		}
	}

	worksheetParts := make(map[string]struct{}, len(w.sheets))
	for _, sheet := range w.sheets {
		// Opaque (chartsheet/dialogsheet/macrosheet) parts are never regenerated
		// from a worksheet model; keeping them out of worksheetParts leaves their
		// preserved bytes to round-trip verbatim (C241).
		if sheet.opaque {
			continue
		}
		if sheet.partName != "" && sheet.wsModel != nil && sheet.dirty {
			worksheetParts[sheet.partName] = struct{}{}
		}
	}
	stylesDirty := w.stylesDirty

	// Determine if the workbook .rels need rebuilding. We need to rebuild if
	// any sheet was modified/added/deleted, styles were changed, or a pivot
	// cache was added (its workbook relationship and <pivotCaches> entry are
	// wired during the rebuild).
	needRelsRebuild := stylesDirty || w.sheetsDirty || w.sheetsHavePivots() || w.vbaModified
	if !needRelsRebuild {
		for _, sheet := range w.sheets {
			if sheet.partName == "" || sheet.dirty {
				needRelsRebuild = true
				break
			}
		}
	}

	// Write all preserved parts except the main workbook part (which is
	// regenerated), core.xml (handled above), rewritten worksheet/style parts,
	// and workbook/.rels files (handled separately when rebuilt). Names are
	// sorted so the zip entry order is deterministic across saves and
	// processes instead of following map iteration order.
	mainPartName := w.mainPart()
	workbookRelsName := opc.GetRelationshipsPartName(mainPartName)
	// A theme edited through Workbook.Theme re-serializes; its preserved source
	// bytes are skipped below and the regenerated bytes written in their place.
	themeName, themeData := w.regeneratedThemePart()
	preservedNames := make([]string, 0, len(w.preservedParts))
	for name := range w.preservedParts {
		preservedNames = append(preservedNames, name)
	}
	sort.Strings(preservedNames)
	for _, name := range preservedNames {
		part := w.preservedParts[name]
		if name == mainPartName {
			continue
		}
		if name == "/docProps/core.xml" {
			continue
		}
		// Regenerated by the writer from the edited custom-properties model.
		if name == "/docProps/custom.xml" && customModified {
			continue
		}
		if name == workbookRelsName && needRelsRebuild {
			continue
		}
		if strings.HasSuffix(name, ".rels") && name != workbookRelsName {
			continue
		}
		// A drawing merged with session-added anchors was written fresh above;
		// skip its stale preserved bytes (C249).
		if rebuiltParts[name] {
			continue
		}
		if _, ok := worksheetParts[name]; ok {
			continue
		}
		if name == "/xl/styles.xml" && stylesDirty {
			continue
		}
		// The VBA project part was replaced or removed this session; its fresh
		// bytes (or absence) are handled by writeVBAProject below.
		if w.vbaModified && name == w.vbaPartName {
			continue
		}
		// Regenerated below from the edited theme model.
		if name == themeName {
			continue
		}
		if err := writer.WritePreservedPart(name, part.ContentType, part.Data); err != nil {
			return err
		}
	}

	// Write the regenerated theme part (only when the session edited it).
	if themeData != nil {
		if err := writer.WritePreservedPart(themeName, w.preservedParts[themeName].ContentType, themeData); err != nil {
			return err
		}
	}

	// Write non-workbook .rels files from preserved parts, except any sheet
	// .rels rebuilt above to carry a new drawing relationship.
	for _, name := range preservedNames {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		if name == workbookRelsName {
			continue
		}
		if rebuiltRels[name] {
			continue
		}
		part := w.preservedParts[name]
		data := part.Data
		// A custom-properties part created this session needs its package
		// relationship added to the root _rels/.rels; inject it into the
		// preserved bytes so unrelated relationships keep their exact form.
		if name == "/_rels/.rels" && customModified && !w.hasCustomPart {
			if aug, _, ok := opc.EnsureRelationshipInRels(data, opc.RelTypeCustom, "docProps/custom.xml"); ok {
				data = aug
			}
		}
		if err := writer.WritePreservedPart(name, part.ContentType, data); err != nil {
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
		// Synthesize xl/metadata.xml for dynamic-array (spill) master cells
		// written this session, tagging them before their worksheet parts are
		// re-marshaled below so the cm attribute is emitted.
		metaTarget, err := w.prepareDynamicArrayMetadata(writer, true)
		if err != nil {
			return err
		}
		if metaTarget != "" {
			wbRels = ensureRelationship(wbRels, relTypeSheetMetadata, metaTarget)
		}
		worksheetTargets := make(map[string]struct{}, len(w.sheets))
		for i, sheet := range w.sheets {
			// Opaque sheets keep their own relationship (chartsheet/dialogsheet
			// type) and preserved bytes: exclude them from the worksheet target
			// set so rebuildWorksheetRelationships does not synthesize a second,
			// worksheet-typed relationship for the same part (C241).
			if sheet.opaque {
				continue
			}
			partName, target := w.roundTripSheetPartName(sheet, i+1)
			if sheet.partName == "" {
				sheet.partName = partName
			}
			worksheetTargets[target] = struct{}{}
			if sheet.wsModel == nil || !sheet.dirty {
				continue
			}
			if err := writeSheetPart(writer, partName, sheet); err != nil {
				return err
			}
		}
		wbRels = rebuildWorksheetRelationships(wbRels, w.sheets, worksheetTargets)
		syncWorkbookSheetRefs(w.workbook, w.sheets)

		// A regenerated person list (threaded comment authors) needs a workbook
		// relationship. ensureRelationship keeps the existing one when the part
		// name is unchanged.
		if personRelTarget != "" {
			wbRels = ensureRelationship(wbRels, opc.RelTypePerson, personRelTarget)
		}

		if stylesDirty {
			stylesData, err := marshalStylesheetXML(w.stylesheet)
			if err != nil {
				return err
			}
			if err := writer.WritePart("/xl/styles.xml", opc.ContentTypeStyles, stylesData); err != nil {
				return err
			}
			wbRels = ensureRelationship(wbRels, opc.RelTypeStyles, "styles.xml")
		}

		// Wire the workbook -> pivotCacheDefinition relationships and the
		// <pivotCaches> element for pivot tables added this session.
		wbRels = w.finalizeWorkbookPivotCaches(wbRels)

		if err := w.writeWorkbookRelationships(writer, mainPartName, workbookRelsName, wbRels); err != nil {
			return err
		}
	}

	// Write (or drop) the VBA project part when the session injected, replaced,
	// or removed it.
	if err := w.writeVBAProject(writer); err != nil {
		return err
	}

	// Write the main workbook part (always regenerated from the parsed model)
	// under its resolved name: writing to a hardcoded /xl/workbook.xml while
	// the package's root relationship points elsewhere would orphan the edits.
	wbData, err := marshalWorkbookXML(w.workbook)
	if err != nil {
		return err
	}
	if err := writer.WritePart(mainPartName, w.Flavor(), wbData); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, strings.TrimPrefix(mainPartName, "/"), opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// writeWorkbookRelationships writes the workbook part's .rels. When the
// rebuilt relationship set still matches what was parsed from the source —
// exactly, or as the same set in a different order (OPC assigns no meaning to
// .rels element order) — the source bytes are preserved verbatim, keeping
// producer formatting (prolog form, BOM, attribute order) intact.
func (w *Workbook) writeWorkbookRelationships(writer *opc.Writer, mainPartName, workbookRelsName string, wbRels []*opc.Relationship) error {
	if part, ok := w.preservedParts[workbookRelsName]; ok {
		orig, err := opc.UnmarshalRelationships(part.Data)
		if err == nil && (opc.RelationshipsEqual(orig, wbRels) || opc.RelationshipsEquivalent(orig, wbRels)) {
			return writer.WritePreservedPart(workbookRelsName, part.ContentType, part.Data)
		}
	}
	return writer.WritePartRelationships(mainPartName, wbRels)
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
	if w.customProps != nil && w.customProps.Len() > 0 {
		writer.CustomProperties = w.customProps
	}

	mainPartName := w.mainPart()

	// Give every sheet a stable part name and workbook relationship id up front
	// so the shared attachment writer can wire each sheet's .rels exactly once,
	// just like the opened save path does.
	for i, sheet := range w.sheets {
		sheet.partName = fmt.Sprintf("/xl/worksheets/sheet%d.xml", i+1)
		sheet.relID = fmt.Sprintf("rId%d", i+1)
	}

	// Write the attachment parts — image drawings/media and comment (legacy
	// comments, VML, threaded comments) parts plus the workbook-shared person
	// list — and the per-sheet .rels that combine them. Both save paths call the
	// same code so they cannot drift: saveOpenedSheetAttachments sets each
	// touched sheet's <drawing>/<legacyDrawing> element and marks the sheet
	// dirty, so the worksheet parts written below carry those references. It
	// returns the person list's workbook-relative target ("" if none) to wire
	// the workbook relationship.
	var personTarget string
	if w.sheetsHaveImages() || w.sheetsHaveCharts() || w.sheetsHaveComments() || w.sheetsHavePendingHyperlinkRels() || w.sheetsHaveTables() || w.sheetsHavePivots() || w.sheetsHaveOLE() {
		var err error
		_, _, personTarget, err = w.saveOpenedSheetAttachments(writer)
		if err != nil {
			return err
		}
	}

	// Synthesize xl/metadata.xml for any dynamic-array (spill) master cells,
	// tagging them before the worksheet parts below are marshaled so the cm
	// attribute is emitted.
	metaTarget, err := w.prepareDynamicArrayMetadata(writer, false)
	if err != nil {
		return err
	}

	// Write each worksheet part (after attachments so any drawing/legacyDrawing
	// reference is present) and its workbook relationship.
	var wbRels []*opc.Relationship
	for i, sheet := range w.sheets {
		if err := writeSheetPart(writer, sheet.partName, sheet); err != nil {
			return err
		}
		wbRels = append(wbRels, &opc.Relationship{
			ID:     sheet.relID,
			Type:   opc.RelTypeWorksheet,
			Target: fmt.Sprintf("worksheets/sheet%d.xml", i+1),
		})
	}

	// Rebuild the sheets element in the workbook model, keeping the relationship
	// ids assigned above.
	w.workbook.Sheets.Sheet = make([]oxml.CT_Sheet, len(w.sheets))
	for i, sheet := range w.sheets {
		w.workbook.Sheets.Sheet[i] = oxml.CT_Sheet{
			Name:    sheet.name,
			SheetId: uint32(i + 1),
			RID:     sheet.relID,
			State:   sheet.state,
		}
	}

	// Write styles.xml if a stylesheet exists. Every workbook relationship added
	// past the worksheets allocates its id by scanning the ids already in use
	// (nextRelationshipID) rather than a hand-maintained counter: the pivot-cache
	// and metadata relationships below allocate that way too, so a hand-counted id
	// could duplicate one of theirs (C258 — e.g. a pivot cache and the VBA project
	// both taking rId3).
	if w.stylesheet != nil {
		stylesPartName := "/xl/styles.xml"
		stylesData, err := marshalStylesheetXML(w.stylesheet)
		if err != nil {
			return err
		}
		if err := writer.WritePart(stylesPartName, opc.ContentTypeStyles, stylesData); err != nil {
			return err
		}

		wbRels = append(wbRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", nextRelationshipID(relIDSet(wbRels))),
			Type:   opc.RelTypeStyles,
			Target: "styles.xml",
		})
	}

	// Wire the workbook-shared person list (threaded-comment authors) when the
	// attachment pass regenerated one.
	if personTarget != "" {
		wbRels = append(wbRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", nextRelationshipID(relIDSet(wbRels))),
			Type:   opc.RelTypePerson,
			Target: personTarget,
		})
	}

	// Wire the workbook -> pivotCacheDefinition relationships and the
	// <pivotCaches> element for pivot tables added this session. Must run
	// before workbook.xml is marshaled so the element is emitted.
	wbRels = w.finalizeWorkbookPivotCaches(wbRels)

	// Wire the workbook -> metadata relationship for a synthesized
	// xl/metadata.xml (dynamic-array spill records).
	if metaTarget != "" {
		wbRels = ensureRelationship(wbRels, relTypeSheetMetadata, metaTarget)
	}

	// Wire and write the VBA project part when injected into a created workbook.
	// Its relationship id is scan-allocated so it never collides with the
	// styles/person/pivot-cache/metadata relationships assigned above (C258).
	if w.vbaModified && !w.vbaRemove {
		wbRels = append(wbRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", nextRelationshipID(relIDSet(wbRels))),
			Type:   opc.RelTypeVBAProject,
			Target: strings.TrimPrefix(w.vbaPartName, "/xl/"),
		})
	}
	if err := w.writeVBAProject(writer); err != nil {
		return err
	}

	// Write workbook.xml
	wbData, err := marshalWorkbookXML(w.workbook)
	if err != nil {
		return err
	}
	if err := writer.WritePart(mainPartName, w.Flavor(), wbData); err != nil {
		return err
	}

	// Write workbook relationships
	if err := writer.WritePartRelationships(mainPartName, wbRels); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, strings.TrimPrefix(mainPartName, "/"), opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// writeSheetPart writes a worksheet part from the worksheet model. It is only
// called for sheets that are regenerated (dirty or new), whose model is already
// materialized; ensureWS is defensive for a new sheet with no content.
func writeSheetPart(writer *opc.Writer, partName string, sheet *Sheet) error {
	ws := sheet.ensureWS()

	// Serialize a view without the contentless <c>/<row> elements a read-only
	// Cell() probe left in the model: they must neither reach the file nor
	// inflate the recorded used range (C425). The durable model is untouched,
	// so a *Cell handle the caller holds stays valid; the pruned view is
	// swapped in only for the dimension pass and the marshal.
	origRows := ws.SheetData.Row
	ws.SheetData.Row = prunedRows(origRows)
	defer func() { ws.SheetData.Row = origRows }()

	// Regenerated sheets are exactly the dirty ones (plus new sheets), so the
	// recorded used range must reflect any cells written since open (C117).
	updateSheetDimension(ws)

	wsData, err := marshalWorksheetXML(ws)
	if err != nil {
		return err
	}
	return writer.WritePart(partName, opc.ContentTypeWorksheet, wsData)
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
		// Opaque sheets (chartsheet/dialogsheet/macrosheet) keep their own
		// relationship of the correct type; never emit a worksheet-typed one for
		// them (C241).
		if sheet.opaque {
			continue
		}
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
			ID:         id,
			Type:       opc.RelTypeWorksheet,
			Target:     target,
			TargetMode: opc.TargetModeInternal,
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
		ID:         fmt.Sprintf("rId%d", nextRelationshipID(usedIDs)),
		Type:       relType,
		Target:     target,
		TargetMode: opc.TargetModeInternal,
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
		wb.Sheets.Sheet[i].State = sheets[i].state
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
// yet (e.g. for a newly created workbook), a default one is materialized in
// memory. Merely reading styles does not mark them dirty (which would force
// styles.xml to be regenerated and break byte-identical round-trip); the
// returned manager marks styles dirty only when a mutating method is called.
func (w *Workbook) Styles() *StyleManager {
	if w.stylesheet == nil {
		// Materializing the default stylesheet for a package that carries no
		// styles part is a READ, so it must not set stylesDirty: an opened
		// workbook with no xl/styles.xml otherwise grew one — plus a
		// content-type override and a workbook relationship — from nothing but
		// wb.Styles().NamedStyles() (C425 shape; the mutating-accessor class).
		// Created workbooks are unaffected: saveNew writes styles.xml whenever
		// w.stylesheet is non-nil, without consulting stylesDirty.
		w.stylesheet = defaultStylesheet()
	}
	// The hook fires only from a mutating StyleManager method, which is exactly
	// where the content edit is recorded too — resolving the styles model above
	// is a read and must reach neither.
	return newStyleManager(w.stylesheet, func() {
		w.stylesDirty = true
		w.markContentEdited()
	})
}

// Sheets returns all sheets in the workbook. The returned slice is a copy of
// the workbook's backing slice, so sorting, truncating or otherwise mutating
// it does not desynchronize the workbook's internal sheet order (and the cached
// per-sheet index). The Sheet pointers themselves are shared.
func (w *Workbook) Sheets() []*Sheet {
	out := make([]*Sheet, len(w.sheets))
	copy(out, w.sheets)
	return out
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

// AddSheet appends a new worksheet named name and returns it.
//
// The name must be a legal Excel sheet name (see ValidateSheetName) and must
// not already be taken, compared case-insensitively as Excel compares them;
// otherwise the sheet is not added and the error says why —
// ErrDuplicateSheetName for a collision, a descriptive error for an illegal
// name.
//
// It used to coerce instead: AddSheet("Bad[Name]…") silently returned a sheet
// actually called "BadName…" truncated to 31 runes, and a second AddSheet("Data")
// silently produced "Data (2)", after which SheetByName(<the name you passed>)
// failed with ErrSheetNotFound. The package already exported ValidateSheetName
// and Sheet.SetName already rejected exactly these names, so the library knew
// the name was illegal and rewrote the caller's identity anyway — in the API
// where names are passed most often, and in a way no docx or pptx creation API
// does (C440). When a suffixed fallback IS what you want, ask for it explicitly:
//
//	sheet, err := wb.AddSheet(wb.UniqueSheetName("Data"))
func (w *Workbook) AddSheet(name string) (*Sheet, error) {
	if err := ValidateSheetName(name); err != nil {
		return nil, err
	}
	if w.sheetNameExists(name) {
		return nil, fmt.Errorf("%w: %q", ErrDuplicateSheetName, name)
	}
	return w.addSheet(name), nil
}

// addSheet appends a sheet under a name the caller has already established is
// legal and free. AddSheet validates; the internal callers that derive their
// own name (CopySheetFrom via UniqueSheetName, the chart data-sheet allocator)
// come straight here.
func (w *Workbook) addSheet(name string) *Sheet {
	ws := &oxml.CT_Worksheet{
		SheetData: oxml.CT_SheetData{},
	}

	sheet := &Sheet{
		workbook: w,
		name:     name,
		index:    len(w.sheets),
		wsModel:  ws,
		wsParsed: true,
		dirty:    true,
	}
	w.sheets = append(w.sheets, sheet)
	w.sheetsDirty = true
	w.markContentEdited()

	// Update the workbook model. SheetId must be unique across the workbook's
	// lifetime — allocating len(sheets) collides after a delete+add, so use one
	// past the current maximum.
	w.workbook.EnsureChildOrder("sheets")
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

// UniqueSheetName coerces name into a legal sheet name that is free in this
// workbook, mirroring how Excel repairs an invalid name: forbidden characters
// are stripped, the result is trimmed to 31 runes and made non-empty, and a
// " (2)"-style suffix is appended while the name collides (case-insensitively)
// with an existing sheet.
//
// It is the explicit form of what AddSheet used to do silently (C440). Pair it
// with AddSheet when a derived name is genuinely what you want:
//
//	sheet, err := wb.AddSheet(wb.UniqueSheetName(userSuppliedName))
//
// The result is only guaranteed free until the next sheet is added.
func (w *Workbook) UniqueSheetName(name string) string {
	return w.sanitizeSheetName(name)
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

// RemoveSheet removes the sheet at the specified index. It is the primary
// spelling; see DeleteSheet for the full description of what removal drags
// along with it.
//
// The name matches every other removal in the library — pptx's RemoveSlide,
// and RemoveCustomProperty / RemoveVBAProject / RemoveDefinedName in this very
// package — where DeleteSheet was the lone Delete* (C565).
func (w *Workbook) RemoveSheet(index int) error {
	return w.DeleteSheet(index)
}

// DeleteSheet removes the sheet at the specified index, together with its
// preserved part, its content-type override, and its own .rels part (C75).
// Workbook state that indexes sheets by position is adjusted: the active tab
// is shifted/clamped, sheet-scoped defined names are re-pointed (names scoped
// to the deleted sheet are dropped), every relationship resolving to a removed
// part is dropped (C366), and references naming the deleted sheet in
// defined-name values and in surviving sheets' formulas are rewritten to
// #REF!, matching what Excel does (C424).
//
// Not rewritten: references held in chart definition parts and in pivot-cache
// definitions, which round-trip as preserved bytes; and the pivot caches of the
// deleted sheet's pivot tables, which are shared workbook-level parts left to a
// dedicated pass.
//
// Every *Sheet handle for a sheet at or after index is invalidated: the
// handles left in the workbook are re-indexed, but a handle the caller already
// holds for the removed sheet is detached and must not be used again.
//
// Deprecated: use RemoveSheet, the name this library uses for every other
// removal (C565). The two are the same call; DeleteSheet is kept working.
func (w *Workbook) DeleteSheet(index int) error {
	if index < 0 || index >= len(w.sheets) {
		return ErrSheetIndex
	}
	deleted := w.sheets[index]
	deletedName := ""
	if deleted != nil {
		deletedName = deleted.name
	}
	if deleted != nil && deleted.partName != "" {
		partName := deleted.partName
		// Cascade-delete the parts reachable only from this sheet — drawings,
		// tables, comments and the media/chart parts they own — before removing
		// the sheet's own relationships (which the cascade reads). A part still
		// referenced by another sheet or part is preserved.
		for part := range w.cascadeDeletableParts(partName) {
			w.deletePartAndRels(part)
		}
		w.deletePartAndRels(partName)
	}
	w.sheets = append(w.sheets[:index], w.sheets[index+1:]...)
	for i := index; i < len(w.sheets); i++ {
		w.sheets[i].index = i
	}
	if deleted != nil {
		// Detach the removed handle so a stale reference cannot rename or dirty
		// a sheet that now occupies its old index.
		deleted.workbook = nil
		deleted.index = -1
	}
	w.sheetsDirty = true
	w.markContentEdited()

	// Update the workbook model
	w.workbook.Sheets.Sheet = append(w.workbook.Sheets.Sheet[:index], w.workbook.Sheets.Sheet[index+1:]...)

	w.adjustActiveTabAfterDelete(index)
	w.adjustDefinedNamesAfterDelete(index, deletedName)
	w.dropRelationshipsToDeletedParts()
	w.rewriteFormulasAfterDelete(deletedName)

	return nil
}

// dropRelationshipsToDeletedParts removes, from every surviving source part's
// relationship list, each internal relationship whose resolved target was
// deleted this session. Without it a chartsheet-typed relationship survives the
// worksheet-only rebuild in rebuildWorksheetRelationships, the resulting set is
// still set-equivalent to the source, and writeWorkbookRelationships re-emits
// the original .rels bytes verbatim — still targeting an absent part (C366).
func (w *Workbook) dropRelationshipsToDeletedParts() {
	if len(w.deletedParts) == 0 {
		return
	}
	for src, rels := range w.relationships {
		if w.deletedParts[src] {
			continue
		}
		kept := rels[:0:0]
		changed := false
		for _, rel := range rels {
			if rel != nil && rel.TargetMode != opc.TargetModeExternal &&
				w.deletedParts[opc.ResolvePartName(src, rel.Target)] {
				changed = true
				continue
			}
			kept = append(kept, rel)
		}
		if changed {
			w.relationships[src] = kept
		}
	}
}

// rewriteFormulasAfterDelete rewrites, on every surviving sheet, formulas that
// name the deleted sheet so they read #REF! the way Excel writes them (C424).
// A sheet whose preserved bytes cannot contain the name is never parsed and
// never dirtied, so an unrelated deletion keeps every other sheet's bytes.
func (w *Workbook) rewriteFormulasAfterDelete(deletedName string) {
	if deletedName == "" {
		return
	}
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.opaque {
			continue
		}
		if !sheet.mayReferenceSheetName(deletedName) {
			continue
		}
		ws := sheet.ws()
		if ws == nil {
			continue
		}
		changed := false
		for i := range ws.SheetData.Row {
			for _, cell := range ws.SheetData.Row[i].C {
				if cell == nil || cell.F == nil || cell.F.Value == "" {
					continue
				}
				if v := rewriteDeletedSheetRefs(cell.F.Value, deletedName); v != cell.F.Value {
					cell.F.Value = v
					changed = true
				}
			}
		}
		if changed {
			sheet.markDirty()
		}
	}
}

// mayReferenceSheetName reports whether the sheet could carry a reference to
// the given sheet name, by scanning its preserved bytes for the name (raw and
// XML-escaped). A materialized sheet always says yes: its model may hold
// formulas the source bytes never did.
func (s *Sheet) mayReferenceSheetName(name string) bool {
	if s.wsModel != nil {
		return true
	}
	if s.workbook == nil || s.partName == "" {
		return false
	}
	part, ok := s.workbook.preservedParts[s.partName]
	if !ok || part == nil {
		return false
	}
	if bytes.Contains(part.Data, []byte(name)) {
		return true
	}
	var esc bytes.Buffer
	if err := xml.EscapeText(&esc, []byte(name)); err != nil {
		return true
	}
	return bytes.Contains(part.Data, esc.Bytes())
}

// rewriteDeletedSheetRefs replaces every sheet-qualified reference to
// sheetName in a formula or defined-name value with "#REF!", the marker Excel
// writes when the referenced sheet is gone. Both the bare form (Data!A1) and
// the quoted form ('My Sheet'!A1) are handled; string literals are skipped so
// a quoted "Data!A1" inside a formula is left alone, and a longer name that
// merely starts with sheetName (MyData!, DataX!) is not matched.
func rewriteDeletedSheetRefs(expr, sheetName string) string {
	if expr == "" || sheetName == "" || !strings.Contains(expr, "!") {
		return expr
	}
	var b strings.Builder
	b.Grow(len(expr))
	i := 0
	for i < len(expr) {
		switch expr[i] {
		case '"':
			// Copy a string literal verbatim ("" escapes an embedded quote).
			j := i + 1
			for j < len(expr) {
				if expr[j] == '"' {
					if j+1 < len(expr) && expr[j+1] == '"' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			b.WriteString(expr[i:j])
			i = j
		case '\'':
			name, end, ok := scanQuotedSheetName(expr, i)
			if ok && end < len(expr) && expr[end] == '!' && strings.EqualFold(name, sheetName) {
				b.WriteString("#REF!")
				i = end + 1
				continue
			}
			if ok {
				b.WriteString(expr[i:end])
				i = end
				continue
			}
			b.WriteByte(expr[i])
			i++
		default:
			if isSheetNameStart(expr[i]) && (i == 0 || !isSheetNameChar(expr[i-1])) {
				j := i
				for j < len(expr) && isSheetNameChar(expr[j]) {
					j++
				}
				if j < len(expr) && expr[j] == '!' && strings.EqualFold(expr[i:j], sheetName) {
					b.WriteString("#REF!")
					i = j + 1
					continue
				}
				b.WriteString(expr[i:j])
				i = j
				continue
			}
			b.WriteByte(expr[i])
			i++
		}
	}
	return b.String()
}

// scanQuotedSheetName reads the single-quoted sheet name starting at expr[i]
// (which must be a quote), returning the unescaped name and the index just
// past the closing quote. ok is false when the quote is unterminated.
func scanQuotedSheetName(expr string, i int) (name string, end int, ok bool) {
	var b strings.Builder
	j := i + 1
	for j < len(expr) {
		if expr[j] == '\'' {
			if j+1 < len(expr) && expr[j+1] == '\'' {
				b.WriteByte('\'')
				j += 2
				continue
			}
			return b.String(), j + 1, true
		}
		b.WriteByte(expr[j])
		j++
	}
	return "", i, false
}

// isSheetNameStart / isSheetNameChar delimit an unquoted sheet-name token in a
// formula. Excel requires quoting for anything outside this set.
func isSheetNameStart(c byte) bool {
	return c == '_' || c == '\\' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c >= 0x80
}

func isSheetNameChar(c byte) bool {
	return isSheetNameStart(c) || (c >= '0' && c <= '9') || c == '.'
}

// sheetPrivateRelTypes are the relationship kinds whose target parts belong to
// a single worksheet. DeleteSheet cascade-deletes these (and the parts they
// transitively own); the closure guard preserves anything still referenced
// elsewhere. Pivot-table parts are deliberately excluded: their caches are
// shared with the workbook and other pivots and are left to a dedicated pass.
//
// Embedded OLE objects, control-properties parts and slicer/timeline parts are
// sheet-private too. Omitting them left an /xl/embeddings/oleObjectN.bin —
// often megabytes — in preservedParts with a content-type override but no
// incoming relationship after DeleteSheet: permanent bloat plus an orphan
// override (C422). Their shared backing parts (slicerCache, timelineCache) are
// referenced from the workbook, so the closure guard spares them.
var sheetPrivateRelTypes = map[string]bool{
	opc.RelTypeDrawing:         true,
	opc.RelTypeTable:           true,
	opc.RelTypeVMLDrawing:      true,
	opc.RelTypeComments:        true,
	opc.RelTypeThreadedComment: true,
	opc.RelTypeOLEObject:       true,
	opc.RelTypeSlicer:          true,
	opc.RelTypeTimeline:        true,
	relTypeCtrlProp:            true,
}

// cascadeDeletableParts returns the set of parts safe to remove together with
// the sheet whose part is sheetPart. It seeds from the sheet-private parts the
// sheet's rels point at (drawings, tables, comments, ...), expands transitively
// through those parts' own rels (a drawing owns its media and chart parts),
// then drops any part still referenced by a relationship originating outside
// the candidate set (excluding the sheet being deleted). The final removal is
// iterated to a fixed point so that sparing a shared part also spares the parts
// only it referenced.
func (w *Workbook) cascadeDeletableParts(sheetPart string) map[string]bool {
	del := make(map[string]bool)
	var queue []string

	// Seed: sheet-private targets of the deleted sheet's rels.
	for _, rel := range w.relationships[sheetPart] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal || !sheetPrivateRelTypes[rel.Type] {
			continue
		}
		if target := opc.ResolvePartName(sheetPart, rel.Target); target != "" && !del[target] {
			del[target] = true
			queue = append(queue, target)
		}
	}

	// Expand transitively through each candidate's own rels (media, charts, ...).
	for len(queue) > 0 {
		part := queue[0]
		queue = queue[1:]
		for _, rel := range w.relationships[part] {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			if target := opc.ResolvePartName(part, rel.Target); target != "" && !del[target] {
				del[target] = true
				queue = append(queue, target)
			}
		}
	}

	// Closure guard: never delete a part referenced from outside the delete set.
	for {
		changed := false
		for part := range del {
			if w.partReferencedFromOutside(part, del, sheetPart) {
				delete(del, part)
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return del
}

// partReferencedFromOutside reports whether any relationship targeting part
// originates from a source part that is neither the sheet being deleted nor a
// member of the delete set.
func (w *Workbook) partReferencedFromOutside(part string, del map[string]bool, sheetPart string) bool {
	for src, rels := range w.relationships {
		if src == sheetPart || del[src] {
			continue
		}
		for _, rel := range rels {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			if opc.ResolvePartName(src, rel.Target) == part {
				return true
			}
		}
	}
	return false
}

// deletePartAndRels removes a part, its relationships part, and their content
// type registrations from the workbook.
func (w *Workbook) deletePartAndRels(part string) {
	relsName := opc.GetRelationshipsPartName(part)
	delete(w.preservedParts, part)
	delete(w.preservedParts, relsName)
	delete(w.relationships, part)
	if w.contentTypes != nil {
		w.contentTypes.RemoveOverride(part)
		w.contentTypes.RemoveOverride(relsName)
	}
	// Record the removal so the save path and Validate both resolve against
	// the output set rather than the source reader, which still holds the part
	// (C366).
	if w.deletedParts == nil {
		w.deletedParts = make(map[string]bool)
	}
	w.deletedParts[part] = true
	w.deletedParts[relsName] = true
}

// adjustActiveTabAfterDelete shifts workbookView activeTab indices down when
// they pointed past the deleted sheet and clamps them into the remaining
// range; a stale index would select the wrong sheet or fall off the end.
func (w *Workbook) adjustActiveTabAfterDelete(index int) {
	if w.workbook.BookViews == nil {
		return
	}
	for i := range w.workbook.BookViews.WorkbookView {
		bv := &w.workbook.BookViews.WorkbookView[i]
		if bv.ActiveTab == nil {
			continue
		}
		tab := int(*bv.ActiveTab)
		if tab > index {
			tab--
		}
		if tab >= len(w.sheets) {
			tab = len(w.sheets) - 1
		}
		if tab < 0 {
			bv.ActiveTab = nil
			continue
		}
		v := uint32(tab)
		bv.ActiveTab = &v
	}
}

// adjustDefinedNamesAfterDelete drops defined names scoped to the deleted
// sheet, shifts the localSheetId of names scoped to later sheets (which would
// otherwise point at the wrong sheet, or beyond the sheet list), and rewrites
// values that reference the deleted sheet by name to #REF! — fixing only the
// scope left a workbook-scoped name such as "Data!$A$1" dangling after Data
// was removed (C424).
func (w *Workbook) adjustDefinedNamesAfterDelete(index int, deletedName string) {
	if w.workbook.DefinedNames == nil {
		return
	}
	names := w.workbook.DefinedNames.DefinedName
	kept := names[:0]
	for _, dn := range names {
		if dn.LocalSheetId != nil {
			id := int(*dn.LocalSheetId)
			if id == index {
				continue // scoped to the deleted sheet
			}
			if id > index {
				v := uint32(id - 1)
				dn.LocalSheetId = &v
			}
		}
		dn.Value = rewriteDeletedSheetRefs(dn.Value, deletedName)
		kept = append(kept, dn)
	}
	w.workbook.DefinedNames.DefinedName = kept
	if len(kept) == 0 {
		w.workbook.DefinedNames = nil
	}
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
	// Workbook marshaling is ChildOrder-gated for opened files: a bookViews
	// element the original lacked must be inserted at its schema position or
	// it would be silently dropped on save (C12).
	w.workbook.EnsureChildOrder("bookViews")
	if len(w.workbook.BookViews.WorkbookView) == 0 {
		w.workbook.BookViews.WorkbookView = append(w.workbook.BookViews.WorkbookView, oxml.CT_BookView{})
	}

	idx := uint32(index)
	w.workbook.BookViews.WorkbookView[0].ActiveTab = &idx
	// workbook.xml is always regenerated, so no flag is needed for this to
	// persist; the content edit is recorded so the save stamps.
	w.markContentEdited()

	return nil
}

// Date1904 reports whether the workbook uses the 1904 serial-date system
// (workbookPr/@date1904, the historical Mac Excel default): serial 0 is
// 1904-01-01 and there is no fictitious 1900-02-29 leap day. The default is
// the 1900 system. Cell.Time and Cell.SetTime follow this setting (C367).
func (w *Workbook) Date1904() bool {
	if w == nil || w.workbook == nil || w.workbook.WorkbookPr == nil {
		return false
	}
	d := w.workbook.WorkbookPr.Date1904
	return d != nil && d.Val
}

// ForceFullCalc reports whether the workbook is marked to recalculate every
// formula the next time it is opened (calcPr fullCalcOnLoad).
func (w *Workbook) ForceFullCalc() bool {
	if w.workbook == nil || w.workbook.CalcPr == nil || w.workbook.CalcPr.FullCalcOnLoad == nil {
		return false
	}
	return w.workbook.CalcPr.FullCalcOnLoad.Val
}

// SetForceFullCalc controls whether Excel recalculates every formula when the
// workbook is next opened, by setting the workbook calcPr fullCalcOnLoad flag.
// Enable it after editing formulas so their cached results are refreshed on
// open. Disabling clears the flag; when the workbook has no other calcPr
// settings the (now default-only) element is still emitted, which Excel
// accepts.
func (w *Workbook) SetForceFullCalc(force bool) {
	if w.workbook == nil {
		return
	}
	if !force {
		if w.workbook.CalcPr != nil {
			// Clearing a flag that was not set changes nothing, so it is not
			// recorded as an edit (and does not move dcterms:modified).
			had := w.workbook.CalcPr.FullCalcOnLoad != nil
			w.workbook.CalcPr.SetFullCalcOnLoad(nil)
			if had {
				w.markContentEdited()
			}
		}
		return
	}
	if w.workbook.CalcPr == nil {
		w.workbook.CalcPr = &oxml.CT_CalcPr{}
	}
	// Marshaling of an opened workbook is ChildOrder-gated: a calcPr element the
	// source lacked must be inserted at its schema position or it is dropped on
	// save.
	w.workbook.EnsureChildOrder("calcPr")
	w.workbook.CalcPr.SetFullCalcOnLoad(oxml.NewBoolLex(true))
	// calcPr lives in workbook.xml, which is always regenerated: no flag needed
	// for the setting to persist, but the content edit still has to be recorded.
	w.markContentEdited()
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

	// Split into column letters and row number. Accept any mix of upper- and
	// lower-case letters ("Aa1", "aB3") the way Excel does, rather than
	// requiring the prefix to be uniformly one case; the prefix is upper-cased
	// below before it is decoded into a column number.
	i := 0
	for i < len(ref) && ((ref[i] >= 'A' && ref[i] <= 'Z') || (ref[i] >= 'a' && ref[i] <= 'z')) {
		i++
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

	// Parse row number. strconv.Atoi accepts a leading sign, so "A+5" would
	// otherwise silently address A5 and the caller would write to a cell it
	// never named (C547); require the row to be digits only.
	for i := 0; i < len(rowStr); i++ {
		if rowStr[i] < '0' || rowStr[i] > '9' {
			return 0, 0, ErrInvalidCell
		}
	}
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
	// Hidden hides the name from Excel's Name Manager.
	Hidden bool
	// Comment is the name's optional comment.
	Comment string
	// Description is the name's optional description.
	Description string
}

// AddDefinedName adds a workbook-scoped defined name. The name must be legal
// for Excel (see ValidateDefinedName) and must not already exist at workbook
// scope; otherwise an error is returned and the workbook is left unchanged
// (C426).
func (w *Workbook) AddDefinedName(name, ref string) error {
	return w.addDefinedName(name, ref, -1)
}

// AddDefinedNameScoped adds a sheet-scoped defined name. The name must be
// legal for Excel (see ValidateDefinedName) and must not already exist on that
// sheet.
func (w *Workbook) AddDefinedNameScoped(name, ref string, sheetIndex int) error {
	if sheetIndex < 0 || sheetIndex >= len(w.sheets) {
		return ErrSheetIndex
	}
	return w.addDefinedName(name, ref, sheetIndex)
}

func (w *Workbook) addDefinedName(name, ref string, sheetIndex int) error {
	if err := ValidateDefinedName(name); err != nil {
		return err
	}
	if err := w.checkDefinedNameCollision(name, sheetIndex); err != nil {
		return err
	}
	if w.workbook.DefinedNames == nil {
		w.workbook.DefinedNames = &oxml.CT_DefinedNames{}
	}
	// See SetActiveSheet: insert the child kind into the preserved order (C12).
	w.workbook.EnsureChildOrder("definedNames")

	dn := oxml.CT_DefinedName{
		Name:  name,
		Value: ref,
	}
	if sheetIndex >= 0 {
		idx := uint32(sheetIndex)
		dn.LocalSheetId = &idx
	}

	w.workbook.DefinedNames.DefinedName = append(w.workbook.DefinedNames.DefinedName, dn)
	// See AddDefinedNameFull: workbook.xml is always regenerated, so this needs
	// no flag, but the content edit still has to be recorded.
	w.markContentEdited()
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
			Name:        dn.Name,
			Value:       dn.Value,
			SheetIndex:  -1,
			Hidden:      dn.Hidden != nil && dn.Hidden.Val,
			Comment:     dn.Comment,
			Description: dn.Description,
		}
		if dn.LocalSheetId != nil {
			result[i].SheetIndex = int(*dn.LocalSheetId)
		}
	}
	return result
}
