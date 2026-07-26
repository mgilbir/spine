package docx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// Document represents a Word document.
type Document struct {
	// Properties contains the document properties.
	Properties opc.CoreProperties

	reader *opc.ReadCloser
	// docModel is the parsed main-document body. For an opened document it is
	// parsed lazily on first access (see doc): a document that is round-tripped
	// without inspecting or editing its body is never parsed, so the save writes
	// the original main-part bytes verbatim rather than building (and holding) a
	// full body model. Reach it through doc(); the save path and Create set/read
	// docModel + docParsed directly so they do not trigger a parse.
	docModel *oxml.CT_Document
	// docParsed records that the body was materialized (parsed on access, or
	// built by Create). When false at save time the body was never touched, so
	// the original bytes pass through unchanged.
	docParsed bool
	// docParseErr caches a failure of the lazy body parse in doc(). Open
	// validates the same bytes up front, so a non-nil value here signals
	// in-memory corruption rather than a malformed file; doc() surfaces it as a
	// clear panic instead of returning a nil model that callers would nil-deref.
	docParseErr error
	// mainPartName is the resolved name of the main document part from the
	// package root relationships (usually /word/document.xml, but e.g.
	// /word/document2.xml occurs in the wild). The save paths regenerate THIS
	// part; hardcoding /word/document.xml would preserve the stale original
	// and silently drop every edit.
	mainPartName string
	// flavor is the main part's content type as recorded at open: one of the
	// WordprocessingML main-part flavors (document, template, or their
	// macro-enabled variants). Empty for a created document, which saves as a
	// regular document. The save paths re-emit this content type so e.g. a
	// macro-enabled document (.docm) is not silently retyped to a regular
	// document while still carrying its vbaProject part.
	flavor           string
	styles           *oxml.CT_Styles
	numbering        *oxml.CT_Numbering
	settings         *oxml.CT_Settings
	footnotes        *oxml.CT_Footnotes
	endnotes         *oxml.CT_Endnotes
	comments         *oxml.CT_Comments
	commentsExtended *oxml.CT_CommentsEx
	people           *oxml.CT_People
	headers          map[string]*headerPart
	footers          map[string]*footerPart
	otherParts       map[string]*coxml.RawPart
	relationships    map[string][]*opc.Relationship
	hasCoreProps     bool
	propsSnapshot    *opc.CoreProperties       // Properties as loaded at open; detects edits at save
	customProps      *opc.CustomProperties     // user-defined properties (docProps/custom.xml), nil when none
	customSnapshot   *opc.CustomProperties     // custom props as loaded at open; detects edits at save
	hasCustomPart    bool                      // whether the opened package carried docProps/custom.xml
	preservedParts   map[string]*coxml.RawPart // all original parts for round-trip
	contentTypesData []byte                    // raw [Content_Types].xml
	imageParts       []*imagePart              // images to be written
	chartParts       []*chartPart              // charts (with embedded workbooks) to be written
	importedParts    []*importedPart           // parts carried over verbatim by Append (charts, OLE objects, ...)
	nextRelIDVal     int                       // counter for main-part relationship IDs
	// nextRelIDByPart holds the next free numeric rId per owner part, seeded
	// past the highest existing rId in that part's own .rels. Relationship ids
	// are scoped per part (each part has its own .rels), so a header/footer part
	// whose rels already carry ids above the main part's max must not reuse the
	// main-part counter or it would emit a duplicate Id (C297). See
	// nextRelIDForPart.
	nextRelIDByPart map[string]int
	shapeIDSeq       int                       // counter for text box / shape docPr ids (see nextShapeID)
	// docPrIDSeq is the document-wide counter for the wp:docPr ids of images and
	// charts, so an AddImage* and an AddChart in the same document never emit the
	// same id (ECMA-376 requires document-unique wp:docPr ids). It is seeded once
	// past the highest docPr id already present in an opened document's drawings.
	// See nextDocPrID.
	docPrIDSeq  int
	docPrIDInit bool
	// revisionIDVal is the highest tracked-change id (w:id) handed out so far;
	// revisionIDInit records that the initial scan of existing ids has run. See
	// nextRevisionID in revisions.go.
	revisionIDVal  int
	revisionIDInit bool
	newHeaderParts []*hdrFtrPart // new headers to be written
	newFooterParts []*hdrFtrPart // new footers to be written
	// modifiedHdrFtrParts holds the names of header/footer parts that existed in
	// the opened package (so they live in preservedParts as raw bytes) but were
	// edited in this session — e.g. a watermark shape added to an existing
	// header. The round-trip save skips the preserved bytes for these parts and
	// regenerates the part (and its .rels) from the parsed model instead. New
	// headers/footers are written via newHeaderParts/newFooterParts and never
	// appear here. Empty on a zero-modification save, so untouched documents
	// round-trip byte-for-byte.
	modifiedHdrFtrParts map[string]bool
	// watermarkSeq hands out unique VML shape ids / spids for watermark shapes
	// so multiple watermarked headers (default/first/even) never collide.
	watermarkSeq int
	// numberingModified / settingsModified / stylesModified record that the
	// session changed the numbering, settings, or styles model, so the
	// round-trip save regenerates that part (with its relationship and
	// content-type override) instead of writing the preserved original bytes.
	numberingModified bool
	settingsModified  bool
	stylesModified    bool
	// comment-part modification flags: set when the comments API adds, replies,
	// resolves, or edits, so the round-trip save regenerates the affected part
	// (with its relationship and content-type override) instead of writing the
	// preserved original bytes. A zero-modification save leaves all flags false
	// and every comment part round-trips byte-for-byte.
	commentsModified    bool
	commentsExtModified bool
	peopleModified      bool
	// footnotesModified / endnotesModified record that the session added a
	// footnote or endnote, so the round-trip save regenerates that part (with
	// its relationship and content-type override) instead of writing the
	// preserved original bytes. A zero-modification save leaves them false and
	// the note parts round-trip byte-for-byte.
	footnotesModified bool
	endnotesModified  bool
	// dirEntries preserves the source archive's zip directory entries
	// (Reader.DirectoryEntries) so a round-trip save re-emits them.
	dirEntries []string
	// theme is the lazily resolved read/write theme handle; themeResolved
	// records that the lookup ran (the handle stays nil when the document has
	// no theme part). See theme.go.
	theme         *dml.ThemeEditor
	themeResolved bool
	themePartName string // theme part name, set when theme resolves to a part
	// vbaModified records that SetVBAProject or RemoveVBAProject ran this
	// session; vbaRemove distinguishes removal from injection. When set, the
	// round-trip save regenerates [Content_Types].xml (see hasAddedParts),
	// writes/drops the VBA part, and re-emits the flipped main-part flavor. A
	// zero-modification save leaves them false and any existing vbaProject.bin
	// round-trips byte-for-byte among the preserved parts. See vba.go.
	vbaModified bool
	vbaRemove   bool
	vbaData     []byte // injected/replacement VBA project bytes (nil when removing)
	vbaPartName string // resolved vbaProject.bin part name
	// pendingCustomXML holds custom-XML data parts added this session
	// (customXml/itemN.xml plus their itemProps and relationships), written on
	// save. Empty on a zero-modification save, so untouched documents round-trip
	// byte-for-byte. See customxml.go.
	pendingCustomXML []*pendingCustomXMLPart
	// sources holds the bibliography sources part (word/bibliography/sources.xml)
	// parsed at open or built by AddSource; sourcesModified records that the
	// session added, edited, or removed a source, so the round-trip save
	// regenerates the part (with its relationship and content-type entry)
	// instead of writing the preserved original bytes. See bibliography.go.
	sources         *oxml.CT_Sources
	sourcesModified bool
	// pendingBuildingBlocks holds building blocks queued by AddBuildingBlock;
	// glossaryModified records that the session authored one, so the round-trip
	// save regenerates (or newly creates) the glossary part (with its
	// relationship and content-type override) instead of writing the preserved
	// original bytes. A zero-modification save leaves them empty and any existing
	// glossary part round-trips byte-for-byte. See buildingblocks.go.
	pendingBuildingBlocks []BuildingBlockDef
	glossaryModified      bool
	// pendingFrameset holds the frameset tree queued by SetFrameset;
	// framesetModified records that the session authored it, so the round-trip
	// save regenerates (or newly creates) the web-settings part (with its
	// relationship, frame relationships, and content-type override) instead of
	// writing the preserved original bytes. A zero-modification save leaves them
	// unset and any existing web-settings part round-trips byte-for-byte. See
	// frameset.go.
	pendingFrameset  *FramesetDef
	framesetModified bool
}

// mainDocumentPart is the default name of the main document part. Image
// relationships default to the main-part scope unless the image is placed in
// a header/footer part.
const mainDocumentPart = "/word/document.xml"

// mainPart returns the name of the main document part: the one resolved from
// the root relationships at open, or the default for created documents.
func (d *Document) mainPart() string {
	if d.mainPartName != "" {
		return d.mainPartName
	}
	return mainDocumentPart
}

// corePropertiesPartName returns the package part the core-properties
// relationship resolves to (usually /docProps/core.xml, but System.IO.Packaging
// producers point it at a *.psmdcp part). Returns "" when the package declares
// no core-properties relationship.
func (d *Document) corePropertiesPartName() string {
	for _, rel := range d.relationships["/"] {
		if rel != nil && rel.Type == opc.RelTypeCore {
			return opc.ResolvePartName("/", rel.Target)
		}
	}
	return ""
}

// doc returns the parsed main-document body, parsing it lazily from the
// original main-part bytes on first access. Marking docParsed means the save
// will regenerate the part from the model (still byte-identical for an
// unmodified body); a document whose body is never accessed keeps docParsed
// false and round-trips its original bytes verbatim. Open validates the body up
// front, so a lazy parse here does not re-surface the malformed-document error
// Open already reports. Returns nil only when the original bytes are
// unavailable (e.g. a hand-constructed Document). If the bytes are present but
// the lazy parse fails — near-unreachable, since Open already parsed the same
// bytes — it panics with a diagnostic message rather than returning a nil model
// that mutation callers (AddParagraph, AddTable, DefaultSection, …) would
// silently nil-deref.
func (d *Document) doc() *oxml.CT_Document {
	if d.docModel == nil && !d.docParsed {
		d.docParsed = true
		if raw := d.rawPartData(d.mainPart()); raw != nil {
			m := &oxml.CT_Document{}
			m.Prolog = xmlb.CaptureProlog(raw)
			m.SelfClosingSpace = xmlb.DetectSelfClosingSpace(raw)
			m.CollapseEmpty = xmlb.DetectCollapsedEmptyElements(raw)
			if err := xmlb.UnmarshalWithSource(raw, m); err != nil {
				d.docParseErr = err
			} else {
				d.docModel = m
			}
		}
	}
	if d.docParseErr != nil {
		panic(fmt.Sprintf("docx: lazy parse of main document part %s failed: %v "+
			"(Open validated the same bytes, so this indicates in-memory corruption)",
			d.mainPart(), d.docParseErr))
	}
	return d.docModel
}

// rawPartData returns a part's raw source bytes for the read-only accessors
// that scan untyped body XML (OLE ProgID resolution). Preserved parts are
// served from the in-memory copy; the main document part is not retained (it is
// parsed into the model and regenerated on save, so keeping its raw bytes would
// double the memory cost of a large document.xml), so it is re-read on demand
// from the still-open source reader. Returns nil when the bytes are unavailable.
func (d *Document) rawPartData(name string) []byte {
	if p, ok := d.preservedParts[name]; ok {
		return p.Data
	}
	if name == d.mainPart() && d.reader != nil {
		if f := d.reader.GetFile(name); f != nil {
			if data, err := f.ReadAll(); err == nil {
				return data
			}
		}
	}
	return nil
}

// documentFlavors is the set of WordprocessingML main-part content types
// (ECMA-376 and [MS-OFFDI]) that Open accepts: regular document (.docx),
// template (.dotx), and their macro-enabled variants (.docm/.dotm).
var documentFlavors = map[string]bool{
	opc.ContentTypeDocument:                  true,
	opc.ContentTypeDocumentTemplateMain:      true,
	opc.ContentTypeDocumentMacroMain:         true,
	opc.ContentTypeDocumentTemplateMacroMain: true,
}

// Flavor returns the main part's content type: one of the WordprocessingML
// flavors (opc.ContentTypeDocument, opc.ContentTypeDocumentTemplateMain, or a
// macro-enabled variant). An opened file reports the flavor it was opened
// with — a template (.dotx) stays a template across a save — and a created
// document reports opc.ContentTypeDocument. There is no conversion API:
// retyping a file to another flavor is out of scope.
func (d *Document) Flavor() string {
	if d.flavor != "" {
		return d.flavor
	}
	return opc.ContentTypeDocument
}

// headerPart stores a parsed header.
type headerPart struct {
	hdr         *oxml.CT_HdrFtr
	contentType string
}

// footerPart stores a parsed footer.
type footerPart struct {
	ftr         *oxml.CT_HdrFtr
	contentType string
}

// Open opens a Word document from a file path. The whole package is read into
// memory, so the returned Document retains no OS file handle.
//
// It returns ErrNotDOCX when the package is not WordprocessingML,
// opc.ErrStrictOOXML for an ISO-Strict package, and opc.ErrEncrypted when the
// input is password-encrypted (open those with docx.OpenEncrypted, or
// opc.OpenEncrypted, and a password). Each is matchable with errors.Is.
func Open(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return OpenReader(bytes.NewReader(data), int64(len(data)))
}

// OpenReader opens a Word document from an in-memory reader. The package is read
// up front, so r need not remain valid after Open returns. It returns the same
// sentinels as Open (ErrNotDOCX, opc.ErrStrictOOXML, opc.ErrEncrypted),
// matchable with errors.Is.
func OpenReader(r io.ReaderAt, size int64) (*Document, error) {
	reader, err := opc.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	return openFromReader(&opc.ReadCloser{Reader: *reader})
}

// openFromReader creates a Document from an OPC reader.
func openFromReader(reader *opc.ReadCloser) (*Document, error) {
	rels := reader.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 {
		// An ISO-Strict Word document carries the officeDocument relationship
		// under the purl.oclc.org namespace; report it distinctly rather than
		// as a generic "not a Word document".
		if len(reader.GetRelationshipsByType(opc.RelTypeOfficeDocumentStrict)) > 0 {
			_ = reader.Close()
			return nil, opc.ErrStrictOOXML
		}
		_ = reader.Close()
		return nil, ErrNotDOCX
	}

	mainPartName := opc.ResolvePartName("/", rels[0].Target)
	mainPart := reader.GetFile(mainPartName)
	if mainPart == nil {
		_ = reader.Close()
		return nil, ErrNotDOCX
	}

	// Any WordprocessingML main-part flavor is accepted (regular document,
	// template, and their macro-enabled variants); the flavor is recorded so
	// the save re-emits it.
	if !documentFlavors[mainPart.ContentType] {
		_ = reader.Close()
		return nil, ErrNotDOCX
	}

	data, err := mainPart.ReadAll()
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	// Validate the body up front (parse-then-discard), then let it be parsed
	// lazily on first access (see doc). A document that is round-tripped without
	// touching its body then never builds — or holds — a full body model.
	// Validation is deliberately strict: some wild files carry XML that is not
	// well-formed (unescaped '&', control characters like U+001F). Word silently
	// repairs those; accepting and re-emitting them here would launder invalid
	// XML through the library, so they are rejected with the part name for
	// context.
	if err := xmlb.UnmarshalWithSource(data, &oxml.CT_Document{}); err != nil {
		_ = reader.Close()
		return nil, fmt.Errorf("docx: parsing %s: %w", mainPartName, err)
	}

	d := &Document{
		reader:         reader,
		mainPartName:   mainPartName,
		flavor:         mainPart.ContentType,
		dirEntries:     reader.DirectoryEntries,
		headers:        make(map[string]*headerPart),
		footers:        make(map[string]*footerPart),
		otherParts:     make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
		preservedParts: make(map[string]*coxml.RawPart),
	}

	if reader.Properties != nil {
		d.Properties = *reader.Properties
		d.hasCoreProps = true
		d.propsSnapshot = reader.Properties.Clone()
	}

	if reader.CustomProperties != nil {
		d.customProps = reader.CustomProperties
		d.customSnapshot = reader.CustomProperties.Clone()
		d.hasCustomPart = true
	}

	if err := d.loadAllParts(mainPartName); err != nil {
		_ = reader.Close()
		return nil, err
	}

	return d, nil
}

// loadAllParts loads all parts from the package.
func (d *Document) loadAllParts(mainPartName string) error {
	if d.reader == nil {
		return nil
	}

	d.loadAllRelationships()

	// Preserve [Content_Types].xml
	if ctData, err := d.reader.GetRawZipFile("[Content_Types].xml"); err == nil {
		d.contentTypesData = ctData
	}

	for _, file := range d.reader.Files {
		name := file.Name
		// The main document part is already parsed into the model (in
		// openFromReader) and is unconditionally regenerated from it on save —
		// the raw copy here is skipped on write (see saveRoundTrip) and read
		// back by nothing. Storing it just doubles the in-memory cost of a
		// large document.xml, so skip it entirely.
		if name == mainPartName {
			continue
		}
		data, err := file.ReadAll()
		if err != nil {
			// A part the model parses must not silently vanish (C60): it
			// would be dropped from the saved package (or regenerated from an
			// empty model), losing content. Other unreadable parts are
			// tolerated: they only fall out of the preserved set, and
			// referenced-part absences are checked after the sweep.
			if isModelParsedDocxPart(name) {
				return fmt.Errorf("docx: reading part %s: %w", name, err)
			}
			continue
		}

		// Preserve all parts as raw bytes for round-trip
		d.preservedParts[name] = &coxml.RawPart{
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
		case name == "/word/styles.xml":
			d.styles = &oxml.CT_Styles{}
			if err := xmlb.Unmarshal(data, d.styles); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == "/word/numbering.xml":
			d.numbering = &oxml.CT_Numbering{}
			if err := xmlb.Unmarshal(data, d.numbering); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == "/word/settings.xml":
			d.settings = &oxml.CT_Settings{}
			if err := xmlb.Unmarshal(data, d.settings); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == "/word/footnotes.xml":
			d.footnotes = &oxml.CT_Footnotes{}
			if err := xmlb.Unmarshal(data, d.footnotes); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == "/word/endnotes.xml":
			d.endnotes = &oxml.CT_Endnotes{}
			if err := xmlb.Unmarshal(data, d.endnotes); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == "/word/comments.xml":
			d.comments = &oxml.CT_Comments{}
			if err := xmlb.Unmarshal(data, d.comments); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == "/word/commentsExtended.xml":
			d.commentsExtended = &oxml.CT_CommentsEx{}
			if err := xmlb.Unmarshal(data, d.commentsExtended); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == "/word/people.xml":
			d.people = &oxml.CT_People{}
			if err := xmlb.Unmarshal(data, d.people); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == bibliographyPartName:
			d.sources = &oxml.CT_Sources{}
			if err := xmlb.Unmarshal(data, d.sources); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
		case name == "/word/fontTable.xml":
			// preserved in preservedParts
		case name == "/word/webSettings.xml":
			// preserved in preservedParts
		case isDocxHeaderPartName(name):
			hdr := &oxml.CT_HdrFtr{}
			if err := xmlb.Unmarshal(data, hdr); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
			d.headers[name] = &headerPart{hdr: hdr, contentType: file.ContentType}
		case isDocxFooterPartName(name):
			ftr := &oxml.CT_HdrFtr{}
			if err := xmlb.Unmarshal(data, ftr); err != nil {
				return fmt.Errorf("docx: parsing %s: %w", name, err)
			}
			d.footers[name] = &footerPart{ftr: ftr, contentType: file.ContentType}
		default:
			d.otherParts[name] = &coxml.RawPart{
				ContentType: file.ContentType,
				Data:        data,
			}
		}
	}

	// A referenced header or footer part must exist (C60): the reference means
	// the document displays that page furniture, so a dangling target is a
	// broken document rather than an optional absence. A referenced-but-absent
	// numbering part is tolerated instead (see checkReferencedParts): numbering
	// is a supplementary definition part, and Word opens such files — list
	// paragraphs simply render without their numbering definition. Genuinely
	// optional parts (unreferenced headers, a numbering.xml no relationship
	// points to) stay tolerated.
	return d.checkReferencedParts(mainPartName)
}

// isModelParsedDocxPart reports whether the named part is parsed into the
// document model at open (as opposed to preserved raw), so a read failure
// must fail the open instead of silently dropping the part.
func isModelParsedDocxPart(name string) bool {
	switch name {
	case "/word/styles.xml", "/word/numbering.xml", "/word/settings.xml",
		"/word/footnotes.xml", "/word/endnotes.xml", "/word/comments.xml",
		"/word/commentsExtended.xml", "/word/people.xml", bibliographyPartName:
		return true
	}
	return isDocxHeaderPartName(name) || isDocxFooterPartName(name)
}

// isDocxHeaderPartName matches the conventional /word/headerN.xml names.
func isDocxHeaderPartName(name string) bool {
	return strings.HasPrefix(name, "/word/header") && strings.HasSuffix(name, ".xml")
}

// isDocxFooterPartName matches the conventional /word/footerN.xml names.
func isDocxFooterPartName(name string) bool {
	return strings.HasPrefix(name, "/word/footer") && strings.HasSuffix(name, ".xml")
}

// checkReferencedParts verifies that every referenced ESSENTIAL part present in
// the main document part's relationships is actually in the package (C60). The
// error names the missing part and the relationship that references it.
//
// Essential-vs-optional boundary: the main document.xml part is essential and
// its absence fails the open earlier (main-part resolution). Among parts
// referenced from it, headers and footers render visible page furniture and are
// treated as essential here — a dangling target is a broken document. Numbering
// is OPTIONAL: it is a supplementary definition part, and Word opens files whose
// numbering rel dangles (real Common Crawl files do exactly this) by rendering
// list paragraphs without their numbering definition. A dangling numbering rel
// is therefore tolerated at open, surfaced instead as a Validate warning, and
// preserved raw so a zero-modification save re-emits the dead rel byte-for-byte.
// (Other supplementary parts — styles, fontTable, settings, webSettings,
// footnotes/endnotes — are not checked here at all, so they are already
// tolerant when referenced but absent.)
func (d *Document) checkReferencedParts(mainPartName string) error {
	kind := map[string]string{
		opc.RelTypeHeader: "header",
		opc.RelTypeFooter: "footer",
	}
	for _, rel := range d.relationships[mainPartName] {
		what, ok := kind[rel.Type]
		if !ok || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		target := opc.ResolvePartName(mainPartName, rel.Target)
		if _, ok := d.preservedParts[target]; ok {
			continue
		}
		// Distinguish an unreadable part from a missing one so the error
		// names the actual failure.
		if f := d.reader.GetFile(target); f != nil {
			if _, err := f.ReadAll(); err != nil {
				return fmt.Errorf("docx: reading %s part %s (relationship %s): %w", what, target, rel.ID, err)
			}
			continue
		}
		return fmt.Errorf("docx: document references missing %s part %s (relationship %s)", what, target, rel.ID)
	}
	return nil
}

// loadAllRelationships loads all relationship files into the model.
func (d *Document) loadAllRelationships() {
	if d.reader == nil {
		return
	}

	for _, file := range d.reader.Files {
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
		d.relationships[sourcePart] = rels
	}
}

// Create creates a new, empty document.
func Create() *Document {
	doc := &oxml.CT_Document{
		Body: &oxml.CT_Body{},
	}

	return &Document{
		docModel:       doc,
		docParsed:      true,
		mainPartName:   mainDocumentPart,
		headers:        make(map[string]*headerPart),
		footers:        make(map[string]*footerPart),
		otherParts:     make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
		preservedParts: make(map[string]*coxml.RawPart),
	}
}

// Save writes the document to a file. Like SaveTo, it enforces the pre-save
// validation gate and the round-trip contract documented there.
func (d *Document) Save(path string) error {
	data, err := d.SaveBytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SaveBytes writes the document to an in-memory buffer through SaveTo (same
// validation gate and round-trip contract).
func (d *Document) SaveBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.SaveTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SaveTo saves the document to an arbitrary writer.
//
// It first runs Validate and refuses to write — returning the Report as an
// error — when any error-severity finding is present, so a structurally corrupt
// package is never produced. SaveToUnvalidated bypasses this gate.
//
// Round-trip contract: for a document opened with Open/OpenReader, parts the
// session never touched are written back byte-for-byte — including a body that
// was never accessed, which is never even parsed — while touched parts are
// regenerated from the model. A document built with Create is generated entirely
// from the model.
func (d *Document) SaveTo(dst io.Writer) error {
	if report := d.Validate(); report.HasErrors() {
		return report
	}
	return d.SaveToUnvalidated(dst)
}

// SaveToUnvalidated saves the document without running the pre-save validation
// pass. Prefer SaveTo; use this only when a finding is known to be advisory for
// the caller's use case.
func (d *Document) SaveToUnvalidated(dst io.Writer) error {
	writer := opc.NewWriter(dst)
	var err error
	if d.reader != nil {
		err = d.saveRoundTrip(writer)
	} else {
		err = d.saveNew(writer)
	}
	if err != nil {
		// Abort, not Close: Close would finalize the half-written package as
		// if it were good; the output must be discarded either way.
		_ = writer.Abort()
		return err
	}
	return writer.Close()
}

// Close releases resources held by a document opened from a file. Open and
// OpenReader read the whole package into memory up front and retain no OS file
// handle, so Close is effectively a no-op. Calling Save (or any Save* method)
// after Close is valid: the in-memory model and preserved parts remain intact.
func (d *Document) Close() error {
	if d.reader != nil {
		return d.reader.Close()
	}
	return nil
}

// saveRoundTrip saves a document opened from a file, preserving all parts.
// Only document.xml is regenerated; all other parts are preserved as original bytes.
func (d *Document) saveRoundTrip(writer *opc.Writer) error {
	// Hand the writer core properties only when the source stored them at
	// /docProps/core.xml (where the preserved raw part plus the writer's
	// skip-if-written rule keeps fidelity) or when the session edited them.
	// Some producers (System.IO.Packaging) keep core properties in a
	// *.psmdcp part instead; that part round-trips verbatim among the
	// preserved parts, and setting writer.Properties would synthesize a
	// docProps/core.xml the source never had.
	//
	// When the core properties live in such a non-standard part AND the session
	// edited them, the edit is written back into that part below (retargeting
	// writer.Properties to /docProps/core.xml would orphan it: the preserved
	// root .rels still points at the original part, so the reader would read the
	// stale copy and the package would carry two core-property parts, C294).
	corePropsEdited := d.hasCoreProps && !d.Properties.Equal(d.propsSnapshot)
	corePartName := d.corePropertiesPartName()
	_, hasPsmdcpCorePart := d.preservedParts[corePartName]
	psmdcpCore := corePartName != "" && corePartName != "/docProps/core.xml" && hasPsmdcpCorePart
	_, hasCorePart := d.preservedParts["/docProps/core.xml"]
	if d.hasCoreProps && (hasCorePart || corePropsEdited) && !psmdcpCore {
		writer.Properties = &d.Properties
	}

	// Regenerated core-property bytes for a non-standard (e.g. *.psmdcp) core
	// part whose properties the session edited. Written in place of the
	// preserved bytes below, keeping the root .rels target and the content-type
	// override unchanged.
	regenCoreName := ""
	var regenCoreData []byte
	if psmdcpCore && corePropsEdited {
		data, err := d.Properties.Marshal()
		if err != nil {
			return err
		}
		regenCoreName = corePartName
		regenCoreData = data
	}

	// Custom properties: an unmodified docProps/custom.xml round-trips as its
	// preserved raw bytes. When the session edited or added properties the
	// writer regenerates the part; the preserved copy is skipped below, and a
	// part created this session has its package relationship injected into the
	// preserved _rels/.rels (its content-type override is merged in by the
	// writer). See customPropertiesModified.
	customModified := d.customPropertiesModified()
	if customModified {
		writer.CustomProperties = d.customProps
	}

	// Re-emit the source archive's directory entries (some producers write
	// them; OPC ignores them but a faithful save keeps the entry listing).
	if err := writer.WriteDirectoryEntries(d.dirEntries); err != nil {
		return err
	}

	// Preserve original content types. Hand the writer a clone: Close mutates
	// its ContentTypes (SetOverride for regenerated metadata parts), so
	// sharing the reader's instance would let repeated saves observe each
	// other's side effects and make concurrent saves race.
	if d.reader != nil && d.reader.ContentTypes != nil {
		writer.ContentTypes = d.reader.ContentTypes.Clone()
	}

	// Write [Content_Types].xml as raw file if preserved. When parts were added
	// after open (images/headers/footers), skip the raw copy so the writer
	// regenerates [Content_Types].xml from the (preserved plus newly registered)
	// content types — otherwise the new parts' content types would be missing.
	if len(d.contentTypesData) > 0 && !d.hasAddedParts() {
		if err := writer.WriteRawFile("[Content_Types].xml", d.contentTypesData); err != nil {
			return err
		}
	}

	// Write core.xml as preserved raw bytes when the in-memory properties
	// still match the snapshot taken at open — or when the part was never
	// parsed at all (some producers write a malformed core-properties
	// relationship type, so the reader finds no properties; the part must
	// still round-trip verbatim). Writing the raw part first wins under the
	// opc writer's skip-if-written rule; when the properties changed, skip
	// the raw copy so Close regenerates core.xml from d.Properties.
	if !d.hasCoreProps || d.Properties.Equal(d.propsSnapshot) {
		if part, ok := d.preservedParts["/docProps/core.xml"]; ok {
			if err := writer.WritePreservedPart("/docProps/core.xml", part.ContentType, part.Data); err != nil {
				return err
			}
		}
	}

	// Write all preserved parts except the main document part (which is
	// regenerated), core.xml (handled above), and .rels files (handled
	// separately). Names are sorted so the zip entry order is deterministic
	// across saves and processes instead of following map iteration order.
	mainPartName := d.mainPart()
	mainRelsName := opc.GetRelationshipsPartName(mainPartName)
	// A theme edited through Document.Theme re-serializes; its preserved source
	// bytes are skipped below and the regenerated bytes written in their place.
	themeName, themeData := d.regeneratedThemePart()
	// Resolved once: the glossary and web-settings parts are skipped below when
	// the session authored a building block or frameset (they are regenerated).
	glossaryName := d.glossaryPartName()
	webSettingsName := d.webSettingsPartName()
	preservedNames := make([]string, 0, len(d.preservedParts))
	for name := range d.preservedParts {
		preservedNames = append(preservedNames, name)
	}
	sort.Strings(preservedNames)
	for _, name := range preservedNames {
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
		// Regenerated below from the (possibly extended) parsed model.
		if name == "/word/styles.xml" && d.stylesModified {
			continue
		}
		if name == "/word/numbering.xml" && d.numberingModified {
			continue
		}
		if name == "/word/settings.xml" && d.settingsModified {
			continue
		}
		if name == "/word/comments.xml" && d.commentsModified {
			continue
		}
		if name == "/word/commentsExtended.xml" && d.commentsExtModified {
			continue
		}
		if name == "/word/people.xml" && d.peopleModified {
			continue
		}
		if name == "/word/footnotes.xml" && d.footnotesModified {
			continue
		}
		if name == "/word/endnotes.xml" && d.endnotesModified {
			continue
		}
		if name == bibliographyPartName && d.sourcesModified {
			continue
		}
		// Regenerated below when the session authored a building block (the
		// existing docParts are spliced through verbatim) or a frameset.
		if name == glossaryName && d.glossaryModified {
			continue
		}
		if name == webSettingsName && d.framesetModified {
			continue
		}
		if strings.HasSuffix(name, ".rels") {
			continue
		}
		// The VBA project part was replaced or removed this session; its fresh
		// bytes (or absence) are handled by writeVBAProject below, so skip the
		// preserved original here.
		if d.vbaModified && name == d.vbaPartName {
			continue
		}
		// Regenerated below from the parsed model (a header/footer edited in this
		// session, e.g. a watermark added to an existing header).
		if d.modifiedHdrFtrParts[name] {
			continue
		}
		// Regenerated below from the edited theme model.
		if name == themeName {
			continue
		}
		// Regenerated below from the edited core properties (a non-standard,
		// e.g. *.psmdcp, core part whose bytes carry the edited props).
		if name == regenCoreName {
			continue
		}
		if err := writer.WritePreservedPart(name, d.preservedParts[name].ContentType, d.preservedParts[name].Data); err != nil {
			return err
		}
	}

	// Write the regenerated theme part (only when the session edited it).
	if themeData != nil {
		if err := writer.WritePreservedPart(themeName, d.preservedParts[themeName].ContentType, themeData); err != nil {
			return err
		}
	}

	// Write the edited core properties back into their non-standard source part
	// (keeping the root .rels target and content-type override), so the reader
	// reads the edit and the package keeps exactly one core-property part.
	if regenCoreData != nil {
		if err := writer.WritePreservedPart(regenCoreName, d.preservedParts[regenCoreName].ContentType, regenCoreData); err != nil {
			return err
		}
	}

	// Write all .rels files from preserved parts (except the main part's rels,
	// which are regenerated).
	for _, name := range preservedNames {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		if name == mainRelsName {
			continue
		}
		// The .rels of a header/footer regenerated this session is rewritten from
		// the parsed relationship set (it may now reference a watermark image),
		// so skip the preserved copy here.
		if d.modifiedHdrFtrRelsPart(name) {
			continue
		}
		// The web-settings .rels is rewritten from the authored frame
		// relationships when the session set a frameset, so skip its preserved
		// copy here.
		if d.framesetModified && name == opc.GetRelationshipsPartName(webSettingsName) {
			continue
		}
		data := d.preservedParts[name].Data
		// A custom-properties part created this session needs its package
		// relationship added to the root _rels/.rels; inject it into the
		// preserved bytes so unrelated relationships keep their exact form.
		if name == "/_rels/.rels" && customModified && !d.hasCustomPart {
			if aug, _, ok := opc.EnsureRelationshipInRels(data, opc.RelTypeCustom, "docProps/custom.xml"); ok {
				data = aug
			}
		}
		if err := writer.WritePreservedPart(name, d.preservedParts[name].ContentType, data); err != nil {
			return err
		}
	}

	// Write parts added through the mutation API (images, headers, footers).
	if err := d.writeAddedParts(writer); err != nil {
		return err
	}

	// Write (or drop) the VBA project part when the session injected, replaced,
	// or removed it. Runs after the content-types clone is in place so the
	// override lands in this save's [Content_Types].xml.
	if err := d.writeVBAProject(writer); err != nil {
		return err
	}

	// Regenerate the header/footer parts edited in this session (existing parts
	// whose preserved bytes were skipped above), along with their .rels.
	if err := d.writeModifiedHdrFtrParts(writer); err != nil {
		return err
	}

	// Write the numbering/settings parts when the session modified them, and
	// make sure document.xml carries a relationship to each. A document opened
	// without the part gets a fresh part, relationship, and content-type
	// override; one that already had it gets the parsed-and-extended content
	// in place of the preserved bytes.
	if err := d.writeModifiedMetadataParts(writer); err != nil {
		return err
	}

	// Write the comment parts (comments/commentsExtended/people) when the
	// session modified them, creating the part, relationship, and content-type
	// override if the opened package did not already carry them.
	if err := d.writeCommentParts(writer, false); err != nil {
		return err
	}

	// Write the footnote/endnote parts when the session added a note, creating
	// the part, relationship, and content-type override if the opened package
	// did not already carry them.
	if err := d.writeNoteParts(writer, false); err != nil {
		return err
	}

	// Write the bibliography sources part when the session modified sources,
	// creating the part, relationship, and content-type entry if the opened
	// package did not already carry them.
	if err := d.writeBibliographyPart(writer, false); err != nil {
		return err
	}

	// Write the glossary (building blocks) part when the session authored a
	// building block, creating the part, relationship, and content-type override
	// if the opened package did not already carry them.
	if err := d.writeGlossaryPart(writer); err != nil {
		return err
	}

	// Write the web-settings (frameset) part when the session authored a
	// frameset, creating the part, relationships, and content-type override if
	// the opened package did not already carry them.
	if err := d.writeFramesetPart(writer); err != nil {
		return err
	}

	// Write the main document part under its resolved name: writing to a
	// hardcoded /word/document.xml while the package's root relationship points
	// elsewhere would orphan the edits. When the body was never materialized the
	// original bytes pass through verbatim; otherwise it is regenerated from the
	// (possibly edited) model.
	var docData []byte
	if d.docModel != nil {
		var err error
		docData, err = marshalDocumentXML(d.docModel)
		if err != nil {
			return err
		}
	} else if docData = d.rawPartData(mainPartName); docData == nil {
		// Bytes unavailable (should not happen for an opened document): fall
		// back to materializing and regenerating.
		var err error
		docData, err = marshalDocumentXML(d.doc())
		if err != nil {
			return err
		}
	}
	if err := writer.WritePart(mainPartName, d.Flavor(), docData); err != nil {
		return err
	}

	// Write document relationships
	if err := d.writeDocumentRelationships(writer); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, strings.TrimPrefix(mainPartName, "/"), opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// writeAddedParts writes the media, header, and footer parts added through the
// mutation API. Both save paths call it, so parts added to a document opened
// from a file are written into the package — previously only the new-document
// path wrote them, leaving the relationships and references dangling.
func (d *Document) writeAddedParts(writer *opc.Writer) error {
	for _, img := range d.imageParts {
		if err := writer.WritePart(img.partName, img.contentType, img.data); err != nil {
			return err
		}
	}
	if err := d.writeChartParts(writer); err != nil {
		return err
	}
	if err := d.writeImportedParts(writer); err != nil {
		return err
	}
	if err := d.writePendingCustomXMLParts(writer); err != nil {
		return err
	}
	for _, hp := range d.newHeaderParts {
		hdrPart, ok := d.headers[hp.partName]
		if !ok {
			continue
		}
		data, err := marshalHdrFtrXML(hdrPart.hdr, "hdr")
		if err != nil {
			return err
		}
		if err := writer.WritePart(hp.partName, opc.ContentTypeDocHeader, data); err != nil {
			return err
		}
		if err := d.writeHdrFtrRelationships(writer, hp.partName); err != nil {
			return err
		}
	}
	for _, fp := range d.newFooterParts {
		ftrPart, ok := d.footers[fp.partName]
		if !ok {
			continue
		}
		data, err := marshalHdrFtrXML(ftrPart.ftr, "ftr")
		if err != nil {
			return err
		}
		if err := writer.WritePart(fp.partName, opc.ContentTypeDocFooter, data); err != nil {
			return err
		}
		if err := d.writeHdrFtrRelationships(writer, fp.partName); err != nil {
			return err
		}
	}
	return nil
}

// modifiedHdrFtrRelsPart reports whether relsName is the .rels part of a
// header/footer regenerated in this session, so the round-trip save skips its
// preserved bytes and rewrites it from the parsed relationship set.
func (d *Document) modifiedHdrFtrRelsPart(relsName string) bool {
	for partName := range d.modifiedHdrFtrParts {
		if opc.GetRelationshipsPartName(partName) == relsName {
			return true
		}
	}
	return false
}

// writeModifiedHdrFtrParts regenerates the header/footer parts edited in this
// session that already existed in the opened package (a watermark added to an
// existing header). The preserved bytes were skipped in saveRoundTrip; here the
// part is re-marshaled from the parsed model and its .rels rewritten so a
// watermark image relationship added to the part resolves.
func (d *Document) writeModifiedHdrFtrParts(writer *opc.Writer) error {
	names := make([]string, 0, len(d.modifiedHdrFtrParts))
	for name := range d.modifiedHdrFtrParts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if hp, ok := d.headers[name]; ok {
			data, err := marshalHdrFtrXML(hp.hdr, "hdr")
			if err != nil {
				return err
			}
			ct := hp.contentType
			if ct == "" {
				ct = opc.ContentTypeDocHeader
			}
			if err := writer.WritePart(name, ct, data); err != nil {
				return err
			}
		} else if fp, ok := d.footers[name]; ok {
			data, err := marshalHdrFtrXML(fp.ftr, "ftr")
			if err != nil {
				return err
			}
			ct := fp.contentType
			if ct == "" {
				ct = opc.ContentTypeDocFooter
			}
			if err := writer.WritePart(name, ct, data); err != nil {
				return err
			}
		} else {
			continue
		}
		if err := d.writeHdrFtrRelationships(writer, name); err != nil {
			return err
		}
	}
	return nil
}

// writeHdrFtrRelationships writes the .rels part for a header/footer added in
// this session, so relationships registered against that part (e.g. images in
// the header) resolve. Parts without registered relationships get no rels
// part, matching Word's output for plain headers.
func (d *Document) writeHdrFtrRelationships(writer *opc.Writer, partName string) error {
	rels := d.relationships[partName]
	if len(rels) == 0 {
		return nil
	}
	return writer.WritePartRelationships(partName, rels)
}

// writeModifiedMetadataParts regenerates the numbering and settings parts on
// the round-trip path when the session modified them, registering the
// document.xml relationship if the opened package did not already carry one.
func (d *Document) writeModifiedMetadataParts(writer *opc.Writer) error {
	if d.stylesModified && d.styles != nil {
		data, err := marshalStylesXML(d.styles)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/styles.xml", opc.ContentTypeDocStyles, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeStyles, "styles.xml")
	}
	if d.numberingModified && d.numbering != nil {
		data, err := marshalNumberingXML(d.numbering)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/numbering.xml", opc.ContentTypeNumbering, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeNumbering, "numbering.xml")
	}
	if d.settingsModified && d.settings != nil {
		data, err := marshalSettingsXML(d.settings)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/settings.xml", opc.ContentTypeDocSettings, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeSettings, "settings.xml")
	}
	return nil
}

// writeCommentParts writes the comments, commentsExtended, and people parts.
// On the round-trip path (created=false) each part is written only when the
// session modified it; on the new-document path (created=true) each is written
// when its model carries content. In both cases the document.xml relationship
// and the content-type override are registered if absent.
func (d *Document) writeCommentParts(writer *opc.Writer, created bool) error {
	if d.comments != nil && len(d.comments.Comment) > 0 && (created || d.commentsModified) {
		data, err := marshalCommentsXML(d.comments)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/comments.xml", opc.ContentTypeDocComments, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeComments, "comments.xml")
	}
	if d.commentsExtended != nil && len(d.commentsExtended.CommentEx) > 0 && (created || d.commentsExtModified) {
		data, err := marshalCommentsExtendedXML(d.commentsExtended)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/commentsExtended.xml", opc.ContentTypeDocCommentsExtended, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeCommentsExtended, "commentsExtended.xml")
	}
	if d.people != nil && len(d.people.Person) > 0 && (created || d.peopleModified) {
		data, err := marshalPeopleXML(d.people)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/people.xml", opc.ContentTypeDocPeople, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypePeople, "people.xml")
	}
	return nil
}

// writeNoteParts writes the footnotes and endnotes parts. On the round-trip
// path (created=false) each part is written only when the session added a note;
// on the new-document path (created=true) each is written when its model
// carries a note. In both cases the document.xml relationship and the
// content-type override are registered if absent.
func (d *Document) writeNoteParts(writer *opc.Writer, created bool) error {
	if d.footnotes != nil && len(d.footnotes.Footnote) > 0 && (created || d.footnotesModified) {
		data, err := marshalFootnotesXML(d.footnotes)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/footnotes.xml", opc.ContentTypeDocFootnotes, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeFootnotes, "footnotes.xml")
	}
	if d.endnotes != nil && len(d.endnotes.Endnote) > 0 && (created || d.endnotesModified) {
		data, err := marshalEndnotesXML(d.endnotes)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/endnotes.xml", opc.ContentTypeDocEndnotes, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeEndnotes, "endnotes.xml")
	}
	return nil
}

// ensureDocRelationship adds a document.xml relationship of the given type
// unless one already exists.
func (d *Document) ensureDocRelationship(relType, target string) {
	for _, rel := range d.relationships[d.mainPart()] {
		if rel.Type == relType {
			return
		}
	}
	d.addDocRelationship(&opc.Relationship{
		ID:     fmt.Sprintf("rId%d", d.nextRelID()),
		Type:   relType,
		Target: target,
	})
}

// hasAddedParts reports whether the mutation API added parts or modified a
// metadata part, requiring [Content_Types].xml to be regenerated so the new
// parts' content types are declared.
func (d *Document) hasAddedParts() bool {
	return len(d.imageParts) > 0 || len(d.chartParts) > 0 || len(d.newHeaderParts) > 0 || len(d.newFooterParts) > 0 ||
		len(d.pendingCustomXML) > 0 ||
		d.numberingModified || d.settingsModified || d.stylesModified ||
		d.commentsModified || d.commentsExtModified || d.peopleModified ||
		d.footnotesModified || d.endnotesModified || d.vbaModified || d.sourcesModified ||
		d.glossaryModified || d.framesetModified
}

// saveNew saves a newly created document.
func (d *Document) saveNew(writer *opc.Writer) error {
	writer.Properties = &d.Properties
	if d.customProps != nil && d.customProps.Len() > 0 {
		writer.CustomProperties = d.customProps
	}

	// A created document always gets a styles part: AddHeading references
	// "Heading1".."Heading9", which would otherwise be undefined and render as
	// plain text.
	if d.styles == nil {
		d.styles = defaultStyles()
	}

	// Write document.xml
	docData, err := marshalDocumentXML(d.doc())
	if err != nil {
		return err
	}
	if err := writer.WritePart("/word/document.xml", d.Flavor(), docData); err != nil {
		return err
	}

	// Collect document relationships - all allocated via nextRelID()
	var docRels []*opc.Relationship

	// Write default styles
	if d.styles != nil {
		data, err := marshalStylesXML(d.styles)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/styles.xml", opc.ContentTypeDocStyles, data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", d.nextRelID()),
			Type:   opc.RelTypeStyles,
			Target: "styles.xml",
		})
	}

	// Write numbering definitions
	if d.numbering != nil && (len(d.numbering.AbstractNum) > 0 || len(d.numbering.Num) > 0) {
		data, err := marshalNumberingXML(d.numbering)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/numbering.xml", opc.ContentTypeNumbering, data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", d.nextRelID()),
			Type:   opc.RelTypeNumbering,
			Target: "numbering.xml",
		})
	}

	// Write settings (created when the API needs a document-level flag, e.g.
	// evenAndOddHeaders for even headers/footers).
	if d.settings != nil {
		data, err := marshalSettingsXML(d.settings)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/settings.xml", opc.ContentTypeDocSettings, data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", d.nextRelID()),
			Type:   opc.RelTypeSettings,
			Target: "settings.xml",
		})
	}

	// Write the media/header/footer parts, then record their relationships.
	if err := d.writeAddedParts(writer); err != nil {
		return err
	}

	// Write the VBA project part when injected into a created document (its
	// document.xml relationship was recorded by SetVBAProject and is appended
	// below).
	if err := d.writeVBAProject(writer); err != nil {
		return err
	}

	// Write the comment parts for a created document (comments added before the
	// first save), registering their document.xml relationships so the append
	// below picks them up.
	if err := d.writeCommentParts(writer, true); err != nil {
		return err
	}
	// Write the footnote/endnote parts for a created document (notes added
	// before the first save), registering their document.xml relationships so
	// the append below picks them up.
	if err := d.writeNoteParts(writer, true); err != nil {
		return err
	}
	// Write the bibliography sources part for a created document (sources added
	// before the first save), registering its document.xml relationship so the
	// append below picks it up.
	if err := d.writeBibliographyPart(writer, true); err != nil {
		return err
	}
	// Write the glossary (building blocks) part for a created document (blocks
	// added before the first save), registering its document.xml relationship so
	// the append below picks it up.
	if err := d.writeGlossaryPart(writer); err != nil {
		return err
	}
	// Write the web-settings (frameset) part for a created document (a frameset
	// set before the first save), registering its document.xml relationship so
	// the append below picks it up.
	if err := d.writeFramesetPart(writer); err != nil {
		return err
	}
	// Emit every relationship registered against the main part by the
	// mutation API (images, headers, footers). Rebuilding this list from
	// imageParts instead would drop the relationship of a deduplicated image
	// placement: adding the same image bytes twice stores one part but two
	// relationships, and only the first lives in imageParts — the second
	// placement's r:embed would dangle.
	docRels = append(docRels, d.relationships[d.mainPart()]...)

	if err := writer.WritePartRelationships("/word/document.xml", docRels); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, "word/document.xml", opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// writeDocumentRelationships writes the main document part's .rels file. When
// the relationship set is unchanged from the opened package, the source .rels
// bytes are preserved verbatim so producer formatting (declaration style, line
// endings, trailing newline) survives the round trip.
func (d *Document) writeDocumentRelationships(writer *opc.Writer) error {
	rels, ok := d.relationships[d.mainPart()]
	if !ok || len(rels) == 0 {
		return nil
	}
	relsName := opc.GetRelationshipsPartName(d.mainPart())
	if part, ok := d.preservedParts[relsName]; ok {
		orig, err := opc.UnmarshalRelationships(part.Data)
		// Exact order match, or the same set in a different order — OPC
		// assigns no meaning to .rels element order, so either way the source
		// bytes are a faithful serialization of the current set.
		if err == nil && (opc.RelationshipsEqual(orig, rels) || opc.RelationshipsEquivalent(orig, rels)) {
			return writer.WritePreservedPart(relsName, part.ContentType, part.Data)
		}
	}
	return writer.WritePartRelationships(d.mainPart(), rels)
}

// Paragraphs returns all paragraphs in the document body in document order,
// including paragraphs wrapped in body-level structured document tags.
func (d *Document) Paragraphs() []*Paragraph {
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	paras := d.doc().Body.Paragraphs()
	result := make([]*Paragraph, len(paras))
	for i, p := range paras {
		result[i] = &Paragraph{document: d, p: p}
	}
	return result
}

// AddParagraph adds a new paragraph to the document body.
func (d *Document) AddParagraph() *Paragraph {
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}
	p := &oxml.CT_P{}
	d.doc().Body.AppendP(p)
	return &Paragraph{document: d, p: p}
}

// AddParagraphWithText adds a new paragraph with the specified text.
func (d *Document) AddParagraphWithText(text string) *Paragraph {
	p := d.AddParagraph()
	p.AddRun().SetText(text)
	return p
}

// AddHeading adds a heading paragraph with the specified level. The level is
// clamped to the valid 1-9 range so out-of-range values cannot produce a
// nonsensical style name (e.g. level 10 previously yielded "Heading:").
func (d *Document) AddHeading(text string, level int) *Paragraph {
	if level < 1 {
		level = 1
	} else if level > 9 {
		level = 9
	}
	p := d.AddParagraph()
	p.SetStyle("Heading" + strconv.Itoa(level))
	p.AddRun().SetText(text)
	return p
}

// Body returns the document body text.
func (d *Document) Body() string {
	paras := d.Paragraphs()
	text := ""
	for _, p := range paras {
		if text != "" {
			text += "\n"
		}
		text += p.Text()
	}
	return text
}

// Tables returns all tables in the document body.
func (d *Document) Tables() []*Table {
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	result := make([]*Table, len(d.doc().Body.Tbl))
	for i, tbl := range d.doc().Body.Tbl {
		result[i] = &Table{document: d, tbl: tbl}
	}
	return result
}

// AddTable creates a new table with the specified number of rows and columns.
func (d *Document) AddTable(rows, cols int) *Table {
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}
	tbl := &oxml.CT_Tbl{
		TblPr:   &oxml.CT_TblPr{},
		TblGrid: &oxml.CT_TblGrid{},
	}
	// Create grid columns
	for i := 0; i < cols; i++ {
		tbl.TblGrid.GridCol = append(tbl.TblGrid.GridCol, oxml.CT_GridCol{})
	}
	// Create rows with cells, seeding through the tracking helpers so the
	// required empty cell paragraph is recorded in the child order and is not
	// dropped by the first tracked append on the cell.
	for i := 0; i < rows; i++ {
		tr := &oxml.CT_Tr{}
		for j := 0; j < cols; j++ {
			tc := &oxml.CT_Tc{}
			tc.AppendP(&oxml.CT_P{})
			tr.AppendCell(tc)
		}
		tbl.AppendRow(tr)
	}
	d.doc().Body.AppendTbl(tbl)
	return &Table{document: d, tbl: tbl}
}

// nextRelID returns the next available relationship ID number for the main
// document part. Relationships registered into a header/footer (or any other)
// part's own .rels must use nextRelIDForPart so ids stay unique within that
// part's scope (C297).
func (d *Document) nextRelID() int {
	if d.nextRelIDVal == 0 {
		// Seed past the highest existing numeric rId, not the count: rIds are
		// often non-contiguous after Word edits (e.g. rId1, rId3), so seeding
		// from the count would collide with an existing id.
		max := 0
		for _, rel := range d.relationships[d.mainPart()] {
			if n := relIDNumber(rel.ID); n > max {
				max = n
			}
		}
		d.nextRelIDVal = max + 1
	}
	id := d.nextRelIDVal
	d.nextRelIDVal++
	return id
}

// nextRelIDForPart returns the next available relationship ID number scoped to
// the owner part named by part. Relationship ids live in each part's own .rels,
// so an id written into a header/footer part must be unique within that part —
// not merely past the main part's max. The counter is seeded once per part past
// the highest existing numeric rId already in that part's rels, so a wild header
// whose .rels carry ids above the main part cannot get a duplicate Id (C297).
// For the main part it matches nextRelID.
func (d *Document) nextRelIDForPart(part string) int {
	if part == "" || part == d.mainPart() {
		return d.nextRelID()
	}
	if d.nextRelIDByPart == nil {
		d.nextRelIDByPart = make(map[string]int)
	}
	next, ok := d.nextRelIDByPart[part]
	if !ok {
		max := 0
		for _, rel := range d.relationships[part] {
			if n := relIDNumber(rel.ID); n > max {
				max = n
			}
		}
		next = max + 1
	}
	d.nextRelIDByPart[part] = next + 1
	return next
}

// relIDNumber parses the numeric suffix of a relationship id like "rId7",
// returning 0 if it does not have that form.
func relIDNumber(id string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "rId"))
	if err != nil {
		return 0
	}
	return n
}

// nextImageNumber returns the smallest positive N for which no
// /word/media/imageN.* part exists. Word shares a single number space across
// image extensions (image1.png and image1.jpeg collide), so numbering is by
// basename number regardless of extension. The scan covers the parts preserved
// from an opened package as well as images added through the mutation API, so
// names stay collision-free across open→add→save cycles.
func (d *Document) nextImageNumber() int {
	used := make(map[int]bool)
	mark := func(name string) {
		const prefix = "/word/media/image"
		// OPC part names are case-insensitive: /word/media/IMAGE1.PNG occupies
		// the same name as image1.png, so match case-insensitively or the
		// generated name would collide at save time.
		name = strings.ToLower(name)
		if !strings.HasPrefix(name, prefix) {
			return
		}
		rest := name[len(prefix):]
		dot := strings.IndexByte(rest, '.')
		if dot <= 0 {
			return
		}
		if n, err := strconv.Atoi(rest[:dot]); err == nil && n > 0 {
			used[n] = true
		}
	}
	for name := range d.preservedParts {
		mark(name)
	}
	for name := range d.otherParts {
		mark(name)
	}
	for _, img := range d.imageParts {
		mark(img.partName)
	}
	for n := 1; ; n++ {
		if !used[n] {
			return n
		}
	}
}

// nextHdrFtrPartName returns the first free /word/<kind>N.xml part name, where
// kind is "header" or "footer". It scans the parsed header/footer maps, the
// parts added earlier in this session, and everything preserved from the
// opened package, so the returned name never collides with an existing part
// and is always a fresh key into d.headers/d.footers (an existing parsed
// header is never clobbered in memory). OPC part names are case-insensitive, so
// a wild /word/Header1.xml occupies the same name as the generated
// /word/header1.xml: the used-name set and the candidate are compared
// lower-cased (as nextImageNumber does) or the chosen name would collide
// case-insensitively at save time.
func (d *Document) nextHdrFtrPartName(kind string) string {
	used := make(map[string]bool,
		len(d.preservedParts)+len(d.headers)+len(d.footers)+len(d.newHeaderParts)+len(d.newFooterParts))
	mark := func(name string) { used[strings.ToLower(name)] = true }
	for name := range d.preservedParts {
		mark(name)
	}
	for name := range d.headers {
		mark(name)
	}
	for name := range d.footers {
		mark(name)
	}
	for _, hp := range d.newHeaderParts {
		mark(hp.partName)
	}
	for _, fp := range d.newFooterParts {
		mark(fp.partName)
	}
	for n := 1; ; n++ {
		name := fmt.Sprintf("/word/%s%d.xml", kind, n)
		if !used[strings.ToLower(name)] {
			return name
		}
	}
}

// addDocRelationship adds a relationship to the main document part's
// relationships.
func (d *Document) addDocRelationship(rel *opc.Relationship) {
	d.addPartRelationship(d.mainPart(), rel)
}

// addPartRelationship adds a relationship to the given source part's
// relationship set (e.g. a header part's own rels for images placed in it).
func (d *Document) addPartRelationship(partName string, rel *opc.Relationship) {
	d.relationships[partName] = append(d.relationships[partName], rel)
}

// removeDocRelationship removes the document.xml relationship with the given
// ID, if present.
func (d *Document) removeDocRelationship(relID string) {
	main := d.mainPart()
	rels := d.relationships[main]
	for i, rel := range rels {
		if rel.ID == relID {
			d.relationships[main] = append(rels[:i], rels[i+1:]...)
			return
		}
	}
}

// dropSessionHeader removes a header added earlier in this session, identified
// by its document relationship ID: the pending part, its part-scoped
// relationships, and its document.xml relationship. A relID that does not
// match a session-added header (e.g. one parsed from the original package) is
// left untouched — preserved parts are never dropped.
func (d *Document) dropSessionHeader(relID string) {
	for i, hp := range d.newHeaderParts {
		if hp.relID != relID {
			continue
		}
		delete(d.headers, hp.partName)
		delete(d.relationships, hp.partName)
		d.newHeaderParts = append(d.newHeaderParts[:i], d.newHeaderParts[i+1:]...)
		d.removeDocRelationship(relID)
		return
	}
}

// dropSessionFooter is the footer counterpart of dropSessionHeader.
func (d *Document) dropSessionFooter(relID string) {
	for i, fp := range d.newFooterParts {
		if fp.relID != relID {
			continue
		}
		delete(d.footers, fp.partName)
		delete(d.relationships, fp.partName)
		d.newFooterParts = append(d.newFooterParts[:i], d.newFooterParts[i+1:]...)
		d.removeDocRelationship(relID)
		return
	}
}

// DefaultSection returns the document's default (last) section.
// If no section properties exist, they are created with default values.
func (d *Document) DefaultSection() *Section {
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}
	if d.doc().Body.SectPr == nil {
		d.doc().Body.SectPr = &oxml.CT_SectPr{}
	}
	return &Section{sectPr: d.doc().Body.SectPr}
}

// AddSectionBreak adds a section break by setting section properties on the
// last block-level paragraph and creating a new default section. When the
// body's last block is not a paragraph (e.g. the document ends with a table),
// a new paragraph is appended after it to carry the section properties —
// attaching them to an earlier paragraph would move the section boundary.
func (d *Document) AddSectionBreak() *Section {
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}

	// Move current body sectPr into the last paragraph
	oldSectPr := d.doc().Body.SectPr
	if oldSectPr == nil {
		oldSectPr = &oxml.CT_SectPr{}
	}

	lastP := d.doc().Body.LastBlockParagraph()
	if lastP == nil {
		lastP = &oxml.CT_P{}
		d.doc().Body.AppendP(lastP)
	}
	if lastP.PPr == nil {
		lastP.PPr = &oxml.CT_PPr{}
	}
	lastP.PPr.SectPr = oldSectPr

	// Create new body-level section
	d.doc().Body.SectPr = &oxml.CT_SectPr{}
	return &Section{sectPr: d.doc().Body.SectPr}
}
