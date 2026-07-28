// Package pptx provides functionality for reading and writing PowerPoint presentations.
//
// A Presentation is not safe for concurrent use. A single Presentation, and the
// slides and shapes reached through it, must be confined to one goroutine, or
// all access must be guarded by external synchronization. In particular Save,
// SaveBytes, and SaveTo mutate shared state while serializing, so they must not
// run concurrently with each other or with any mutation of the same
// Presentation. Distinct Presentation values may be used from different
// goroutines.
package pptx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Common errors
var (
	ErrNotPPTX      = errors.New("pptx: not a valid PowerPoint file")
	ErrNoSlides     = errors.New("pptx: presentation has no slides")
	ErrSlideIndex   = errors.New("pptx: slide index out of range")
	ErrInvalidSlide = errors.New("pptx: invalid slide")

	// ErrLayoutNotFound indicates no slide layout matched a LayoutByName /
	// LayoutByType lookup. It is pptx's counterpart of xlsx.ErrSheetNotFound
	// (C565).
	ErrLayoutNotFound = errors.New("pptx: slide layout not found")

	// ErrPlaceholderNotFound indicates the slide, layout or master carries no
	// placeholder of the requested type.
	ErrPlaceholderNotFound = errors.New("pptx: placeholder not found")
)

// presentationFlavors is the set of PresentationML main-part content types
// (ECMA-376 and [MS-OFFDI]) that Open accepts: regular presentation (.pptx),
// slideshow (.ppsx), template (.potx), and their macro-enabled variants
// (.pptm/.ppsm/.potm).
var presentationFlavors = map[string]bool{
	opc.ContentTypePresentationMain:              true,
	opc.ContentTypeSlideshowMain:                 true,
	opc.ContentTypePresentationTemplateMain:      true,
	opc.ContentTypePresentationMacroMain:         true,
	opc.ContentTypeSlideshowMacroMain:            true,
	opc.ContentTypePresentationTemplateMacroMain: true,
}

// Presentation represents a PowerPoint presentation.
type Presentation struct {
	// Properties contains the document properties.
	Properties opc.CoreProperties

	// hasCorePart records that the opened package stored core properties at
	// /docProps/core.xml; propsSnapshot is Properties as loaded at open, so
	// the save can tell user edits apart. Together they keep a round-trip
	// save from synthesizing a core.xml the source never had (e.g. sources
	// with *.psmdcp core properties or none at all).
	hasCorePart   bool
	propsSnapshot *opc.CoreProperties
	// corePartRaw preserves the source /docProps/core.xml bytes (and content
	// type) so an unmodified save reproduces the part verbatim instead of
	// regenerating it (declaration form, element order, and formatting all
	// vary by producer).
	corePartRaw *coxml.RawPart

	// customProps holds the user-defined properties (docProps/custom.xml), nil
	// when the package has none. customSnapshot is the set as loaded at open so
	// the save can tell edits apart; hasCustomPart records that the opened
	// package carried the part (so a created part gets its package relationship).
	customProps    *opc.CustomProperties
	customSnapshot *opc.CustomProperties
	hasCustomPart  bool

	slides       []*Slide
	slideMasters []*SlideMaster
	slideLayouts []*SlideLayout

	reader       *opc.ReadCloser
	presentation *oxml.Presentation
	nextSlideID  uint32
	nextRelID    int
	templatePath string // Path to template file if using one

	// flavor is the main part's content type as recorded at open: one of the
	// PresentationML main-part flavors (presentation, slideshow, template, or
	// their macro-enabled variants). Empty for a created presentation, which
	// saves as a regular presentation. The save paths re-emit this content
	// type so e.g. a slideshow (.ppsx) is not silently retyped to a regular
	// presentation on save.
	flavor string

	// Raw data for parts we serialize but don't fully parse
	presPropsData   []byte                         // /ppt/presProps.xml
	viewPropsData   []byte                         // /ppt/viewProps.xml
	tableStylesData []byte                         // /ppt/tableStyles.xml
	themeData       map[string][]byte              // /ppt/theme/*.xml (keyed by part name)
	// themeEditors caches one dml.ThemeEditor per theme part name, created on
	// the first Theme() call. A nil value means "this part does not parse",
	// cached so the failure is not retried on every access. applyThemeEdits
	// folds the modified ones back into themeData at save (C571).
	themeEditors map[string]*dml.ThemeEditor
	thumbnailData   []byte                         // /docProps/thumbnail.jpeg
	appPropsData    []byte                         // /docProps/app.xml
	printerSettings map[string][]byte              // /ppt/printerSettings/*.bin
	otherParts      map[string]*coxml.RawPart      // Any other parts (media, custom XML, etc.)
	relationships   map[string][]*opc.Relationship // Relationships for each part
	// rawRels keeps the source bytes of each parsed .rels part (keyed by
	// source part name) so an unmodified relationship set is written back
	// verbatim, preserving producer formatting.
	rawRels map[string][]byte

	// unreferencedSlides records slide parts present in the opened package but
	// absent from presentation.xml's sldIdLst. They are preserved verbatim
	// (written back from otherParts, C118) and are not part of p.slides.
	unreferencedSlides map[string]bool
	// removedParts records part names deleted by this session's mutations
	// (RemoveSlide and the parts it owns), so the save drops their lingering
	// content-type overrides.
	removedParts map[string]bool
	// mediaGCNeeded marks that relationships were dropped this session
	// (RemoveSlide, RemoveShape sync, poster swaps), allowing the save to
	// garbage-collect /ppt/media/ parts no relationship references anymore
	// (C221). It stays false on zero-modification saves so those remain
	// byte-identical.
	mediaGCNeeded bool

	// modernAuthors caches the parsed ppt/authors.xml (the shared threaded
	// comment author list). It is loaded lazily and, once a comment write adds
	// an author, re-marshaled back into otherParts.
	modernAuthors       *oxml.ModernAuthorList
	modernAuthorsLoaded bool

	// dirEntries preserves the source archive's zip directory entries
	// (Reader.DirectoryEntries) so a round-trip save re-emits them.
	dirEntries []string
}

// Open opens a PowerPoint presentation from a file path. The whole package is
// read into memory, so the returned Presentation retains no OS file handle.
//
// It returns ErrNotPPTX when the package is not PresentationML,
// opc.ErrStrictOOXML for an ISO-Strict package, and opc.ErrEncrypted when the
// input is password-encrypted (open those with OpenEncrypted and a password).
// Each is matchable with errors.Is.
func Open(path string) (*Presentation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return OpenReader(bytes.NewReader(data), int64(len(data)))
}

// OpenReader opens a PowerPoint presentation from an in-memory reader. The
// package is read up front, so r need not remain valid after Open returns. It
// returns the same sentinels as Open (ErrNotPPTX, opc.ErrStrictOOXML,
// opc.ErrEncrypted), matchable with errors.Is.
func OpenReader(r io.ReaderAt, size int64) (*Presentation, error) {
	reader, err := opc.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	return openFromReader(&opc.ReadCloser{Reader: *reader})
}

// openFromReader creates a Presentation from an OPC reader.
func openFromReader(reader *opc.ReadCloser) (*Presentation, error) {
	// Find the main presentation part
	rels := reader.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 {
		// An ISO-Strict presentation carries the officeDocument relationship
		// under the purl.oclc.org namespace; report it distinctly rather than
		// as a generic "not a PowerPoint file".
		if len(reader.GetRelationshipsByType(opc.RelTypeOfficeDocumentStrict)) > 0 {
			_ = reader.Close()
			return nil, opc.ErrStrictOOXML
		}
		_ = reader.Close()
		return nil, ErrNotPPTX
	}

	mainPartName := opc.ResolvePartName("/", rels[0].Target)
	mainPart := reader.GetFile(mainPartName)
	if mainPart == nil {
		_ = reader.Close()
		return nil, ErrNotPPTX
	}

	// Verify content type: any PresentationML main-part flavor is accepted
	// (regular presentation, slideshow, template, and their macro-enabled
	// variants); the flavor is recorded so the save re-emits it.
	if !presentationFlavors[mainPart.ContentType] {
		_ = reader.Close()
		return nil, ErrNotPPTX
	}

	// Parse presentation XML
	data, err := mainPart.ReadAll()
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	var pres oxml.Presentation
	if err := xmlb.UnmarshalWithSource(data, &pres); err != nil {
		_ = reader.Close()
		return nil, err
	}
	pres.Prolog = xmlb.CaptureProlog(data)
	pres.SelfClosingSpace = xmlb.DetectSelfClosingSpace(data)
	pres.CollapseEmpty = xmlb.DetectCollapsedEmptyElements(data)
	pres.SourceXML = data

	p := &Presentation{
		reader:          reader,
		flavor:          mainPart.ContentType,
		presentation:    &pres,
		nextSlideID:     256,
		nextRelID:       1,
		slideMasters:    make([]*SlideMaster, 0),
		slideLayouts:    make([]*SlideLayout, 0),
		themeData:       make(map[string][]byte),
		themeEditors:    make(map[string]*dml.ThemeEditor),
		printerSettings: make(map[string][]byte),
		otherParts:      make(map[string]*coxml.RawPart),
		relationships:   make(map[string][]*opc.Relationship),
		rawRels:         make(map[string][]byte),
		dirEntries:      reader.DirectoryEntries,
	}

	// Copy properties and snapshot them so the save can detect edits.
	if reader.Properties != nil {
		p.Properties = *reader.Properties
	}
	p.propsSnapshot = p.Properties.Clone()

	if reader.CustomProperties != nil {
		p.customProps = reader.CustomProperties
		p.customSnapshot = reader.CustomProperties.Clone()
		p.hasCustomPart = true
	}

	// Determine next slide ID and relationship ID
	if pres.SlideIDs != nil {
		for _, sid := range pres.SlideIDs.SlideID {
			if sid.ID >= p.nextSlideID {
				p.nextSlideID = sid.ID + 1
			}
			p.updateNextRelID(sid.RID)
		}
	}

	// Get relationships for the main part
	presRels, err := reader.GetPartRelationships(mainPartName)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	// Build relationship map
	relMap := make(map[string]*opc.Relationship)
	for _, rel := range presRels {
		relMap[rel.ID] = rel
		p.updateNextRelID(rel.ID)
	}

	// Load slide masters
	if err := p.loadSlideMasters(mainPartName, relMap); err != nil {
		_ = reader.Close()
		return nil, err
	}

	// Relationship files are needed to resolve slide -> layout links on loaded slides.
	p.loadAllRelationships()

	// Load slides
	if err := p.loadSlides(mainPartName); err != nil {
		_ = reader.Close()
		return nil, err
	}

	// Load all remaining parts into the model
	if err := p.loadAllParts(mainPartName); err != nil {
		_ = reader.Close()
		return nil, err
	}

	// Bind each master to its theme part (needs the theme data captured by
	// loadAllParts and the rels from loadAllRelationships). The part is only
	// parsed if Theme() is actually called; see theme.go.
	p.resolveThemes()

	return p, nil
}

// updateNextRelID updates nextRelID based on an existing relationship ID.
func (p *Presentation) updateNextRelID(relID string) {
	if len(relID) > 3 && relID[:3] == "rId" {
		var id int
		if _, err := fmt.Sscanf(relID, "rId%d", &id); err == nil {
			if id >= p.nextRelID {
				p.nextRelID = id + 1
			}
		}
	}
}

// loadAllParts loads all remaining parts into the model for serialization.
func (p *Presentation) loadAllParts(mainPartName string) error {
	if p.reader == nil {
		return nil
	}

	// Load specific known parts
	for _, file := range p.reader.Files {
		name := file.Name
		data, err := file.ReadAll()
		if err != nil {
			continue
		}

		// Categorize the part
		switch {
		case strings.HasSuffix(name, ".rels"):
			// Relationships handled separately (must be checked before
			// prefix-based matches like /ppt/theme/ to avoid capturing
			// .rels files under those directories)
			continue
		case name == "/ppt/presProps.xml":
			p.presPropsData = data
		case name == "/ppt/viewProps.xml":
			p.viewPropsData = data
		case name == "/ppt/tableStyles.xml":
			p.tableStylesData = data
		case strings.HasPrefix(name, "/ppt/theme/"):
			p.themeData[name] = data
		case name == "/docProps/thumbnail.jpeg":
			p.thumbnailData = data
		case strings.HasPrefix(name, "/ppt/printerSettings/"):
			p.printerSettings[name] = data
		case name == "/ppt/presentation.xml":
			// Already loaded into p.presentation
			continue
		case strings.HasPrefix(name, "/ppt/slides/") && !strings.Contains(name, "_rels"):
			if p.slidePartLoaded(name) {
				// Already loaded into p.slides
				continue
			}
			// A slide part not referenced by presentation.xml's sldIdLst
			// (valid packages can carry them). Preserve it verbatim as a raw
			// part instead of dropping it (C118); it is not added to p.slides.
			p.otherParts[name] = &coxml.RawPart{
				ContentType: file.ContentType,
				Data:        data,
			}
			if p.unreferencedSlides == nil {
				p.unreferencedSlides = make(map[string]bool)
			}
			p.unreferencedSlides[name] = true
		case strings.HasPrefix(name, "/ppt/slideMasters/") && !strings.Contains(name, "_rels"):
			// Already loaded into p.slideMasters
			continue
		case strings.HasPrefix(name, "/ppt/slideLayouts/") && !strings.Contains(name, "_rels"):
			// Already loaded into p.slideLayouts
			continue
		case name == "/docProps/core.xml":
			// Already loaded into p.Properties. Keep the raw bytes too: an
			// unmodified save writes them verbatim; only edited properties
			// regenerate the part.
			p.hasCorePart = true
			p.corePartRaw = &coxml.RawPart{ContentType: file.ContentType, Data: data}
			continue
		case name == "/docProps/app.xml":
			p.appPropsData = data
		default:
			// Store as other part
			p.otherParts[name] = &coxml.RawPart{
				ContentType: file.ContentType,
				Data:        data,
			}
		}
	}

	return nil
}

// slidePartLoaded reports whether the named part is one of the slides loaded
// from presentation.xml's sldIdLst.
func (p *Presentation) slidePartLoaded(name string) bool {
	for _, slide := range p.slides {
		if slide.partName == name {
			return true
		}
	}
	return false
}

// loadAllRelationships loads all relationship files into the model.
func (p *Presentation) loadAllRelationships() {
	if p.reader == nil {
		return
	}

	for _, file := range p.reader.Files {
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

		// Store relationships keyed by the source part
		// e.g., "/ppt/_rels/presentation.xml.rels" -> relationships for /ppt/presentation.xml
		sourcePart := coxml.RelsPathToSourcePart(file.Name)
		p.relationships[sourcePart] = rels
		p.rawRels[sourcePart] = data
	}
}

// loadSlides loads all slides from the presentation.
func (p *Presentation) loadSlides(mainPartName string) error {
	if p.presentation.SlideIDs == nil {
		return nil
	}

	// Get relationships for the main part
	rels, err := p.reader.GetPartRelationships(mainPartName)
	if err != nil {
		return err
	}

	// Build relationship map
	relMap := make(map[string]*opc.Relationship)
	for _, rel := range rels {
		relMap[rel.ID] = rel
	}

	// Load each slide
	for _, slideRef := range p.presentation.SlideIDs.SlideID {
		rel, ok := relMap[slideRef.RID]
		if !ok {
			// Dangling sldId (rel/part missing): skip it. index is assigned as
			// len(p.slides) at append below rather than from the sldIdLst
			// position, so a skipped entry does not leave a gap that would
			// misalign Slide.index from its p.slides slot (C301).
			continue
		}

		slideName := opc.ResolvePartName(mainPartName, rel.Target)
		slideFile := p.reader.GetFile(slideName)
		if slideFile == nil {
			continue
		}

		data, err := slideFile.ReadAll()
		if err != nil {
			return fmt.Errorf("pptx: reading slide part %s: %w", slideName, err)
		}

		// Validate the slide up front (parse-then-discard), then leave it to be
		// parsed lazily on first access (see Slide.sx). A slide that is
		// round-tripped without being inspected or edited then never builds — or
		// holds — a full model, and its original bytes pass through verbatim on
		// save. Validation is deliberately strict: some wild files carry XML that
		// is not well-formed (unescaped '&', control characters); PowerPoint
		// silently repairs those, but accepting and re-emitting them here would
		// launder invalid XML through the library, so they are rejected with the
		// part name for context.
		if err := xmlb.UnmarshalWithSource(data, &oxml.Slide{}); err != nil {
			return fmt.Errorf("pptx: parsing slide part %s: %w", slideName, err)
		}

		slide := &Slide{
			presentation: p,
			partName:     slideName,
			index:        len(p.slides),
			id:           slideRef.ID,
			relID:        slideRef.RID,
			idExtLst:     slideRef.ExtLst,
		}

		p.slides = append(p.slides, slide)
	}

	for _, slide := range p.slides {
		// Resolve each slide's layout pointer from its relationships (independent
		// of the slide model, so it runs even though the slides are not parsed
		// yet). Hyperlink targets — including cross-slide jumps that need the full
		// slide list — are resolved when a slide is materialized (see
		// materializeShapes), by which point every slide is loaded.
		p.resolveSlideLayout(slide)
	}

	return nil
}

// loadSlideMasters loads all slide masters and their layouts from the presentation.
func (p *Presentation) loadSlideMasters(mainPartName string, relMap map[string]*opc.Relationship) error {
	if p.presentation.SlideMasterIDs == nil {
		return nil
	}

	for _, masterRef := range p.presentation.SlideMasterIDs.SlideMasterID {
		rel, ok := relMap[masterRef.RID]
		if !ok {
			continue
		}

		masterName := opc.ResolvePartName(mainPartName, rel.Target)
		masterFile := p.reader.GetFile(masterName)
		if masterFile == nil {
			continue
		}

		data, err := masterFile.ReadAll()
		if err != nil {
			return fmt.Errorf("pptx: reading slide master part %s: %w", masterName, err)
		}

		var masterXML oxml.SlideMaster
		if err := xmlb.UnmarshalWithSource(data, &masterXML); err != nil {
			return fmt.Errorf("pptx: parsing slide master part %s: %w", masterName, err)
		}
		masterXML.Prolog = xmlb.CaptureProlog(data)
		masterXML.SelfClosingSpace = xmlb.DetectSelfClosingSpace(data)
		masterXML.CollapseEmpty = xmlb.DetectCollapsedEmptyElements(data)
		masterXML.SourceXML = data

		master := &SlideMaster{
			presentation: p,
			partName:     masterName,
			masterXML:    &masterXML,
			relID:        masterRef.RID,
			numericID:    masterRef.ID,
			idOmitted:    masterRef.IDOmitted,
			idExtLst:     masterRef.ExtLst,
			layouts:      make([]*SlideLayout, 0),
		}

		// Load layouts for this master
		if err := p.loadSlideLayouts(master, masterName); err != nil {
			return err
		}

		p.slideMasters = append(p.slideMasters, master)
	}

	return nil
}

// loadSlideLayouts loads all slide layouts for a given master.
func (p *Presentation) loadSlideLayouts(master *SlideMaster, masterPartName string) error {
	// Get relationships for the master
	masterRels, err := p.reader.GetPartRelationships(masterPartName)
	if err != nil {
		return err
	}

	// Build relationship map
	relMap := make(map[string]*opc.Relationship)
	for _, rel := range masterRels {
		relMap[rel.ID] = rel
	}

	// Load layouts referenced from master
	if master.masterXML.SlideLayoutIDs != nil {
		for _, layoutRef := range master.masterXML.SlideLayoutIDs.SlideLayoutID {
			rel, ok := relMap[layoutRef.RID]
			if !ok {
				continue
			}

			layoutName := opc.ResolvePartName(masterPartName, rel.Target)
			layoutFile := p.reader.GetFile(layoutName)
			if layoutFile == nil {
				continue
			}

			data, err := layoutFile.ReadAll()
			if err != nil {
				return fmt.Errorf("pptx: reading slide layout part %s: %w", layoutName, err)
			}

			var layoutXML oxml.SlideLayout
			if err := xmlb.UnmarshalWithSource(data, &layoutXML); err != nil {
				return fmt.Errorf("pptx: parsing slide layout part %s: %w", layoutName, err)
			}
			layoutXML.Prolog = xmlb.CaptureProlog(data)
			layoutXML.SelfClosingSpace = xmlb.DetectSelfClosingSpace(data)
			layoutXML.CollapseEmpty = xmlb.DetectCollapsedEmptyElements(data)
			layoutXML.SourceXML = data

			layout := &SlideLayout{
				presentation: p,
				master:       master,
				partName:     layoutName,
				layoutXML:    &layoutXML,
				layoutType:   LayoutTypeFromString(layoutXML.Type),
				name:         layoutXML.MatchingName,
				relID:        layoutRef.RID,
			}

			master.layouts = append(master.layouts, layout)
			p.slideLayouts = append(p.slideLayouts, layout)
		}
	}

	return nil
}

// Create creates a new, empty presentation.
func Create() *Presentation {
	return CreateWithOptions(DefaultCreateOptions())
}

// CreateWithOptions creates a new presentation with the specified options.
// The slide size is taken from opts.SlideSize (previously the option was
// ignored and every deck came out 4:3).
func CreateWithOptions(opts CreateOptions) *Presentation {
	now := time.Now()
	p := &Presentation{
		Properties: opc.CoreProperties{
			Created:  now,
			Modified: now,
		},
		slides:          make([]*Slide, 0),
		slideMasters:    make([]*SlideMaster, 0),
		slideLayouts:    make([]*SlideLayout, 0),
		themeData:       make(map[string][]byte),
		themeEditors:    make(map[string]*dml.ThemeEditor),
		printerSettings: make(map[string][]byte),
		otherParts:      make(map[string]*coxml.RawPart),
		relationships:   make(map[string][]*opc.Relationship),
		nextSlideID:     256,
		nextRelID:       1,
		presentation: &oxml.Presentation{
			SlideSize: createSlideSize(opts),
			NotesSize: &oxml.SlideSize{
				Cx: 6858000,
				Cy: 9144000,
			},
		},
	}

	// Add default master and layouts if requested
	if opts.IncludeDefaultLayouts {
		p.initializeDefaultMasterAndLayouts()
	}

	return p
}

// createSlideSize resolves the p:sldSz for a new presentation, honoring
// explicit Width/Height when SlideSize is SlideSizeCustom. A custom size with
// either dimension unset (0) falls back to the predefined mapping (4:3).
func createSlideSize(opts CreateOptions) *oxml.SlideSize {
	if opts.SlideSize == SlideSizeCustom && opts.Width > 0 && opts.Height > 0 {
		return &oxml.SlideSize{Cx: int64(opts.Width), Cy: int64(opts.Height)}
	}
	return slideSizeToOxml(opts.SlideSize)
}

// slideSizeToOxml maps an Options slide size to the p:sldSz element. The two
// screen formats carry their canonical type attribute; paper sizes emit plain
// dimensions.
func slideSizeToOxml(ss SlideSize) *oxml.SlideSize {
	switch ss {
	case SlideSizeStandard:
		return oxml.DefaultSlideSize()
	case SlideSizeWidescreen:
		return oxml.WidescreenSlideSize()
	default:
		w, h := ss.Dimensions()
		return &oxml.SlideSize{Cx: int64(w), Cy: int64(h)}
	}
}

// slideDimensions returns the presentation's slide width and height in EMU,
// falling back to the 4:3 default when no size is set.
func (p *Presentation) slideDimensions() (w, h dml.EMU) {
	if p.presentation != nil && p.presentation.SlideSize != nil {
		return dml.EMU(p.presentation.SlideSize.Cx), dml.EMU(p.presentation.SlideSize.Cy)
	}
	return dml.EMU(9144000), dml.EMU(6858000)
}

// initializeDefaultMasterAndLayouts creates the default slide master and layouts.
func (p *Presentation) initializeDefaultMasterAndLayouts() {
	w, h := p.slideDimensions()

	// Create default master, sized to the slide (C139).
	master := createDefaultMaster(w, h)
	master.presentation = p
	master.partName = "/ppt/slideMasters/slideMaster1.xml"
	master.relID = fmt.Sprintf("rId%d", p.nextPresentationRelID())

	// Add default layouts
	defaultLayouts := []SlideLayoutType{
		LayoutTitle,
		LayoutTitleAndContent,
		LayoutSectionHeader,
		LayoutTwoContent,
		LayoutTitleOnly,
		LayoutBlank,
	}

	for i, lt := range defaultLayouts {
		layout := createDefaultLayout(lt, master, w, h)
		layout.presentation = p
		layout.partName = fmt.Sprintf("/ppt/slideLayouts/slideLayout%d.xml", i+1)
		layout.relID = fmt.Sprintf("rId%d", i+1) // Layout relationships are relative to master
		master.layouts = append(master.layouts, layout)
		p.slideLayouts = append(p.slideLayouts, layout)
	}

	p.slideMasters = append(p.slideMasters, master)
}

// CreateWidescreen creates a new presentation with widescreen (16:9) dimensions.
func CreateWidescreen() *Presentation {
	opts := DefaultCreateOptions()
	opts.SlideSize = SlideSizeWidescreen
	return CreateWithOptions(opts)
}

// Save writes the presentation to a file. Like SaveTo, it enforces the pre-save
// validation gate and the round-trip contract documented there.
func (p *Presentation) Save(path string) error {
	data, err := p.SaveBytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SaveBytes writes the presentation to an in-memory buffer through SaveTo (same
// validation gate and round-trip contract).
func (p *Presentation) SaveBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := p.SaveTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SaveTo saves the presentation to an arbitrary writer.
//
// It first runs Validate and refuses to write — returning the Report as an
// error — when any error-severity finding is present, so a structurally corrupt
// package is never produced. SaveToUnvalidated bypasses this gate.
//
// Round-trip contract: for a presentation opened with Open/OpenReader, parts the
// session never touched are written back byte-for-byte — including slides that
// were never accessed, which are never even parsed — while touched parts are
// regenerated from the model. A presentation built with Create (or a Create*
// helper) is generated entirely from the model.
func (p *Presentation) SaveTo(dst io.Writer) error {
	if report := p.Validate(); report.HasErrors() {
		return report
	}
	return p.SaveToUnvalidated(dst)
}

// SaveToUnvalidated saves the presentation without running the pre-save
// validation pass. Prefer SaveTo; use this only when a finding is known to be
// advisory for the caller's use case.
func (p *Presentation) SaveToUnvalidated(dst io.Writer) error {
	// Fold any theme edit back into the preserved theme bytes before either
	// save path reads themeData. Both paths write theme parts straight from
	// that map, so this is the single point where a Theme() edit becomes
	// output (C571); an untouched theme is left byte-identical.
	p.applyThemeEdits()
	writer := opc.NewWriter(dst)
	var err error
	if p.reader != nil {
		err = p.saveRoundTrip(writer)
	} else {
		err = p.saveNew(writer)
	}
	if err != nil {
		// Abort, not Close: Close would finalize the half-written package as
		// if it were good; the output must be discarded either way.
		_ = writer.Abort()
		return err
	}
	return writer.Close()
}

// SaveAs writes the presentation to a new file path — the "open a template,
// modify, save as a new file" workflow. It is Save to a different path and
// enforces the same validation gate and round-trip contract as SaveTo.
//
// Deprecated: use Save. Save already takes the destination path, so this is an
// exact alias, and neither docx nor xlsx has one (C567).
func (p *Presentation) SaveAs(path string) error {
	return p.Save(path)
}

// CreateFromTemplate creates a new presentation based on an existing template file.
// The template's masters, layouts, themes, and styles are preserved.
// All slides from the template are removed, leaving a blank presentation with the template's styling.
func CreateFromTemplate(templatePath string) (*Presentation, error) {
	// Open the template
	p, err := Open(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}

	// Store template path for reference
	p.templatePath = templatePath

	// Clear all slides but keep masters, layouts, and themes. Route each removal
	// through the same cleanup RemoveSlide performs — delete the slide's
	// relationships, remove the parts it exclusively owns (notes slide, comments),
	// record removed parts so their content-type overrides are dropped, and flag
	// media GC. A plain `p.slides = nil` left the template slides' relationships
	// behind, so a new AddSlide reused slide1.xml and inherited the template
	// slide's surviving rels — including its notesSlide (a data leak, C243) —
	// while the removed slides' <Override> entries dangled in [Content_Types].xml
	// (C303).
	for len(p.slides) > 0 {
		if err := p.RemoveSlide(0); err != nil {
			return nil, fmt.Errorf("failed to clear template slides: %w", err)
		}
	}
	p.presentation.SlideIDs = nil

	// Reset slide ID counter but keep relationship IDs
	p.nextSlideID = 256

	// A template opens with the template main-part content type (e.g. .potx's
	// ContentTypePresentationTemplateMain). The output is a new presentation, so
	// re-emitting the template flavor would make PowerPoint open it as a template.
	// Reset to the plain presentation flavor, preserving macro-enablement (a .potm
	// template maps to a macro-enabled presentation, not a plain one) — C306.
	// PlainFlavor returns a different value only for a macro-enabled input, so it
	// distinguishes the two cases without enumerating every template flavor.
	if opc.PlainFlavor(p.flavor) != p.flavor {
		p.flavor = opc.ContentTypePresentationMacroMain
	} else {
		p.flavor = opc.ContentTypePresentationMain
	}

	// Update properties for new presentation
	now := time.Now()
	p.Properties.Created = now
	p.Properties.Modified = now
	p.Properties.Title = ""
	p.Properties.Subject = ""
	p.Properties.Description = ""

	return p, nil
}

// CreateFromTemplateWithSlides creates a new presentation based on a template,
// keeping the template's slides as well as its styling.
func CreateFromTemplateWithSlides(templatePath string) (*Presentation, error) {
	// Open the template
	p, err := Open(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}

	// Store template path for reference
	p.templatePath = templatePath

	// Update properties for new presentation
	now := time.Now()
	p.Properties.Created = now
	p.Properties.Modified = now

	return p, nil
}

// TemplatePath returns the path to the template file if this presentation
// was created from a template, or empty string otherwise.
func (p *Presentation) TemplatePath() string {
	return p.templatePath
}

// Flavor returns the main part's content type: one of the PresentationML
// flavors (opc.ContentTypePresentationMain, opc.ContentTypeSlideshowMain,
// opc.ContentTypePresentationTemplateMain, or a macro-enabled variant). An
// opened file reports the flavor it was opened with — a slideshow (.ppsx)
// stays a slideshow across a save — and a created presentation reports
// opc.ContentTypePresentationMain. There is no conversion API: retyping a
// file to another flavor is out of scope.
func (p *Presentation) Flavor() string {
	if p.flavor != "" {
		return p.flavor
	}
	return opc.ContentTypePresentationMain
}

// LayoutByType returns the first slide layout matching the specified type, or
// ErrLayoutNotFound when the deck has none.
//
// It is the counterpart of xlsx's Workbook.SheetByName: a By<Field> lookup that
// reports a miss as an error rather than as a nil pointer the caller must
// remember to check. GetLayoutByType, which returned a bare nil, is the
// deprecated spelling (C565).
func (p *Presentation) LayoutByType(layoutType SlideLayoutType) (*SlideLayout, error) {
	if l := p.GetLayoutByType(layoutType); l != nil {
		return l, nil
	}
	return nil, fmt.Errorf("%w: no layout of type %v", ErrLayoutNotFound, layoutType)
}

// LayoutByName returns the slide layout with the specified name, or
// ErrLayoutNotFound when the deck has none by that name. See LayoutByType for
// why the error result exists.
func (p *Presentation) LayoutByName(name string) (*SlideLayout, error) {
	if l := p.GetLayoutByName(name); l != nil {
		return l, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrLayoutNotFound, name)
}

// GetLayoutByType returns the first slide layout matching the specified type.
//
// Deprecated: use LayoutByType, which reports a miss as an error instead of a
// nil pointer (C565).
func (p *Presentation) GetLayoutByType(layoutType SlideLayoutType) *SlideLayout {
	for _, layout := range p.slideLayouts {
		if layout.Type() == layoutType {
			return layout
		}
	}
	return nil
}

// GetLayoutByName returns the slide layout with the specified name.
//
// Deprecated: use LayoutByName, which reports a miss as an error instead of a
// nil pointer (C565).
func (p *Presentation) GetLayoutByName(name string) *SlideLayout {
	for _, layout := range p.slideLayouts {
		if layout.Name() == name {
			return layout
		}
	}
	return nil
}

// AddSlideFromLayout adds a new slide using the specified layout.
func (p *Presentation) AddSlideFromLayout(layout *SlideLayout) *Slide {
	slide := p.AddSlide()
	slide.layout = layout
	return slide
}

// saveRoundTrip saves a presentation by serializing all parts from the model.
func (p *Presentation) saveRoundTrip(writer *opc.Writer) error {
	// Core properties: an unmodified /docProps/core.xml round-trips as the
	// preserved raw bytes (producer formatting varies; regenerating drifts).
	// Edited properties regenerate the part via the writer; a source without
	// the part never gains one on a zero-modification save (some producers
	// keep core properties in a *.psmdcp part or have none).
	propsEdited := p.propsSnapshot != nil && !p.Properties.Equal(p.propsSnapshot)
	if p.hasCorePart && p.corePartRaw != nil && !propsEdited {
		if err := writer.WritePreservedPart("/docProps/core.xml", p.corePartRaw.ContentType, p.corePartRaw.Data); err != nil {
			return err
		}
	} else if p.hasCorePart || propsEdited {
		writer.Properties = &p.Properties
	}

	// Custom properties: an unmodified docProps/custom.xml round-trips as its
	// preserved raw bytes (written from otherParts below). When the session
	// edited or added properties the writer regenerates the part; the preserved
	// copy is skipped below, and a part created this session has its package
	// relationship injected into the preserved root _rels/.rels. See
	// customPropertiesModified.
	customModified := p.customPropertiesModified()
	if customModified {
		writer.CustomProperties = p.customProps
		if !p.hasCustomPart {
			p.ensureCustomPropsPackageRelationship()
		}
	}

	// Re-emit the source archive's directory entries (some producers write
	// them; OPC ignores them but a faithful save keeps the entry listing).
	if err := writer.WriteDirectoryEntries(p.dirEntries); err != nil {
		return err
	}

	currentSlideParts := make(map[string]bool, len(p.slides))
	for _, slide := range p.slides {
		oldSlideName := slide.partName
		slideName := oldSlideName
		if slideName == "" || currentSlideParts[slideName] {
			// Allocate a name not already taken by any slide or other part.
			// An index-based name (slide{i+1}.xml) collides with a surviving
			// slide after a RemoveSlide+AddSlide, failing Save with a duplicate
			// part.
			slideName = p.nextAvailableSlidePartName()
			slide.partName = slideName
			if oldSlideName != "" && oldSlideName != slideName {
				if rels, ok := p.relationships[oldSlideName]; ok {
					p.relationships[slideName] = rels
					delete(p.relationships, oldSlideName)
				}
			}
		}
		currentSlideParts[slideName] = true
	}

	// Use original content types to preserve ordering and avoid extra entries.
	// Hand the writer a clone: Close mutates its ContentTypes (SetOverride for
	// regenerated metadata parts), so sharing the reader's instance would let
	// repeated saves observe each other's side effects and make concurrent
	// saves race.
	if p.reader != nil && p.reader.ContentTypes != nil {
		writer.ContentTypes = p.reader.ContentTypes.Clone()
		// Drop overrides for parts removed this session (C88): the parts are
		// not written, so their overrides would dangle. A later part reusing
		// the name gets its override re-registered by WritePart.
		for name := range p.removedParts {
			writer.ContentTypes.RemoveOverride(name)
		}
	}

	// Track which parts have had their rels written explicitly
	writtenRels := make(map[string]bool)

	// Write slide masters using original part names
	for i, master := range p.slideMasters {
		masterData, err := master.marshal()
		if err != nil {
			return fmt.Errorf("marshal slide master %d: %w", i, err)
		}
		masterName := master.partName
		if masterName == "" {
			masterName = fmt.Sprintf("/ppt/slideMasters/slideMaster%d.xml", i+1)
		}
		if err := writer.WritePart(masterName, opc.ContentTypeSlideMaster, masterData); err != nil {
			return err
		}

		// Write master relationships
		if err := p.writePartRelationships(writer, masterName); err != nil {
			return err
		}
		writtenRels[masterName] = true
	}

	// Write slide layouts using original part names
	for i, layout := range p.slideLayouts {
		layoutData, err := layout.marshal()
		if err != nil {
			return fmt.Errorf("marshal slide layout %d: %w", i, err)
		}
		layoutName := layout.partName
		if layoutName == "" {
			layoutName = fmt.Sprintf("/ppt/slideLayouts/slideLayout%d.xml", i+1)
		}
		if err := writer.WritePart(layoutName, opc.ContentTypeSlideLayout, layoutData); err != nil {
			return err
		}

		// Write layout relationships
		if err := p.writePartRelationships(writer, layoutName); err != nil {
			return err
		}
		writtenRels[layoutName] = true
	}

	// Write slides using original part names. The allocation loop above has
	// already given every slide a non-empty name, so no index-derived fallback
	// is needed here — and the one this replaced would have produced exactly the
	// collision-prone slide{i+1}.xml form that loop exists to avoid (C515).
	for i, slide := range p.slides {
		slideName := slide.partName
		p.ensureSlideLayoutRelationship(slide, slideName)

		// When the slide was never materialized it is unmodified, so its original
		// bytes pass through verbatim (regeneration occasionally drifts from a
		// whitespace-preserving producer's exact bytes); otherwise it is
		// regenerated from the (possibly edited) model.
		var slideData []byte
		if slide.sxModel != nil {
			var err error
			slideData, err = slide.marshal()
			if err != nil {
				return fmt.Errorf("marshal slide %d: %w", i, err)
			}
		} else if slideData = slide.rawBytes(); slideData == nil {
			// Bytes unavailable (should not happen for an opened slide): fall
			// back to materializing and regenerating.
			var err error
			slideData, err = slide.marshal()
			if err != nil {
				return fmt.Errorf("marshal slide %d: %w", i, err)
			}
		}
		if err := writer.WritePart(slideName, opc.ContentTypeSlide, slideData); err != nil {
			return err
		}

		// Write slide relationships
		if err := p.writePartRelationships(writer, slideName); err != nil {
			return err
		}
		writtenRels[slideName] = true
	}

	// Write all themes (sorted for deterministic package output). Theme
	// override parts share the /ppt/theme/ directory but have their own
	// content type; registering them as theme+xml corrupts
	// [Content_Types].xml.
	for _, themeName := range sortedKeys(p.themeData) {
		contentType := opc.ContentTypeTheme
		if strings.HasPrefix(path.Base(themeName), "themeOverride") {
			contentType = opc.ContentTypeThemeOverride
		}
		if err := writer.WritePart(themeName, contentType, p.themeData[themeName]); err != nil {
			return err
		}
	}

	// Write presentation properties
	if len(p.presPropsData) > 0 {
		if err := writer.WritePart("/ppt/presProps.xml", opc.ContentTypePresentationProps, p.presPropsData); err != nil {
			return err
		}
	}

	// Write view properties
	if len(p.viewPropsData) > 0 {
		if err := writer.WritePart("/ppt/viewProps.xml", opc.ContentTypeViewProps, p.viewPropsData); err != nil {
			return err
		}
	}

	// Write table styles
	if len(p.tableStylesData) > 0 {
		if err := writer.WritePart("/ppt/tableStyles.xml", opc.ContentTypeTableStyles, p.tableStylesData); err != nil {
			return err
		}
	}

	// Write thumbnail
	if len(p.thumbnailData) > 0 {
		if err := writer.WritePart("/docProps/thumbnail.jpeg", "image/jpeg", p.thumbnailData); err != nil {
			return err
		}
	}

	// Write docProps/app.xml if preserved
	if len(p.appPropsData) > 0 {
		if err := writer.WritePart("/docProps/app.xml", opc.ContentTypeExtendedProps, p.appPropsData); err != nil {
			return err
		}
	}

	// Write printer settings (sorted for deterministic package output)
	for _, name := range sortedKeys(p.printerSettings) {
		if err := writer.WritePart(name, "application/vnd.openxmlformats-officedocument.presentationml.printerSettings", p.printerSettings[name]); err != nil {
			return err
		}
	}

	// Write other parts (sorted for deterministic package output), skipping
	// media parts no relationship references anymore (C221). The slides were
	// marshaled above, so every relationship drop from this session's shape
	// removals has already been applied. These are preserved source parts:
	// some (e.g. /[trash]/0000.dat junk emitted by certain producers) have no
	// content-type entry in the source, so they bypass the new-part
	// content-type requirement.
	deadMedia := p.unreferencedMediaParts()
	for _, name := range sortedKeys(p.otherParts) {
		if deadMedia[name] {
			continue
		}
		// Regenerated by the writer from the edited custom-properties model.
		if name == "/docProps/custom.xml" && customModified {
			continue
		}
		part := p.otherParts[name]
		if err := writer.WritePreservedPart(name, part.ContentType, part.Data); err != nil {
			return err
		}
	}

	// Write relationships for all parts that have rels but haven't been written
	// explicitly (sorted for deterministic package output)
	writtenRels["/ppt/presentation.xml"] = true // will be written below
	for _, partName := range sortedKeys(p.relationships) {
		rels := p.relationships[partName]
		if writtenRels[partName] || len(rels) == 0 {
			continue
		}
		// Skip rels for slide part names no longer written — unless the part
		// is a preserved unreferenced slide (C118), whose rels must survive.
		if strings.HasPrefix(partName, "/ppt/slides/") && !currentSlideParts[partName] && !p.unreferencedSlides[partName] {
			continue
		}
		if err := p.writePartRelationships(writer, partName); err != nil {
			return err
		}
	}

	// Write presentation.xml (regenerated to reflect current slide list),
	// keeping the flavor recorded at open (slideshow/template/macro-enabled
	// sources must not be retyped to a regular presentation).
	presData, err := p.marshalPresentation()
	if err != nil {
		return err
	}
	if err := writer.WritePart("/ppt/presentation.xml", p.Flavor(), presData); err != nil {
		return err
	}

	// Write presentation relationships
	if err := p.writePresentationRelationships(writer); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, "ppt/presentation.xml", opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// unreferencedMediaParts returns the /ppt/media/ parts that no relationship
// in the package references (C221). It returns nil unless relationships were
// dropped this session (mediaGCNeeded), so a zero-modification save never
// garbage-collects anything and stays byte-identical. The scan is
// package-global and exact: p.relationships holds every parsed .rels file, so
// a part referenced from any slide, layout, master, notes slide, or the
// presentation itself is kept.
func (p *Presentation) unreferencedMediaParts() map[string]bool {
	if !p.mediaGCNeeded {
		return nil
	}
	referenced := make(map[string]bool)
	for src, rels := range p.relationships {
		for _, rel := range rels {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			referenced[opc.ResolvePartName(src, rel.Target)] = true
		}
	}
	var dead map[string]bool
	for name := range p.otherParts {
		if strings.HasPrefix(name, "/ppt/media/") && !referenced[name] {
			if dead == nil {
				dead = make(map[string]bool)
			}
			dead[name] = true
		}
	}
	return dead
}

// writePartRelationships writes the relationships file for a given part. When
// the relationship set is unchanged from the opened package, the source .rels
// bytes are preserved verbatim so producer formatting (declaration style, line
// endings, trailing newline) survives the round trip.
func (p *Presentation) writePartRelationships(writer *opc.Writer, partName string) error {
	rels, ok := p.relationships[partName]
	if !ok || len(rels) == 0 {
		return nil
	}

	relsPath := partNameToRelsPath(partName)
	if data := p.unchangedRawRels(partName, rels); data != nil {
		return writer.WritePreservedPart(relsPath, opc.ContentTypeRelationships, data)
	}

	data, err := opc.MarshalRelationships(rels)
	if err != nil {
		return err
	}

	return writer.WritePart(relsPath, opc.ContentTypeRelationships, data)
}

// unchangedRawRels returns the source .rels bytes for partName when the
// current relationship set still matches what was parsed from them, or nil.
func (p *Presentation) unchangedRawRels(partName string, rels []*opc.Relationship) []byte {
	raw, ok := p.rawRels[partName]
	if !ok {
		return nil
	}
	orig, err := opc.UnmarshalRelationships(raw)
	if err != nil {
		return nil
	}
	// Exact order match, or the same set in a different order — OPC assigns
	// no meaning to .rels element order, so either way the source bytes are a
	// faithful serialization of the current set.
	if !opc.RelationshipsEqual(orig, rels) && !opc.RelationshipsEquivalent(orig, rels) {
		return nil
	}
	return raw
}

// resolveSlideLayout maps a loaded slide's slideLayout relationship back to its
// *SlideLayout pointer. Without this, Slide.Layout() returns nil for slides loaded
// from disk, which breaks AddSlideFromLayout in round-trip (Open -> mutate -> SaveAs)
// scenarios because the caller cannot obtain a valid layout reference.
func (p *Presentation) resolveSlideLayout(slide *Slide) {
	rels := p.relationships[slide.partName]
	for _, rel := range rels {
		if rel.Type != opc.RelTypeSlideLayout {
			continue
		}

		layoutPartName := opc.ResolvePartName(slide.partName, rel.Target)
		for _, layout := range p.slideLayouts {
			if layout.partName == layoutPartName {
				slide.layout = layout
				return
			}
		}
	}
}

// ensureSlideLayoutRelationship guarantees that a slide with a layout pointer also
// has a corresponding slideLayout relationship in p.relationships. This is needed
// during round-trip save because the "save new" path creates layout rels automatically,
// but the round-trip path only writes pre-existing rels — so new slides added via
// AddSlideFromLayout would otherwise lose their layout link on save.
func (p *Presentation) ensureSlideLayoutRelationship(slide *Slide, slidePartName string) {
	// Use the slide's layout, or fall back to the first available layout so a
	// slide added to an opened deck (which leaves slide.layout nil) still gets a
	// slideLayout relationship. A slide with no layout rel makes PowerPoint
	// prompt to repair — mirroring the "save new" path's fallback.
	target := slide.layout
	if target == nil {
		if len(p.slideLayouts) == 0 {
			return
		}
		target = p.slideLayouts[0]
	}

	slideRels := append([]*opc.Relationship(nil), p.relationships[slidePartName]...)
	for _, rel := range slideRels {
		if rel.Type == opc.RelTypeSlideLayout {
			return
		}
	}

	for _, layout := range p.slideLayouts {
		if layout != target {
			continue
		}

		slideRels = append(slideRels, &opc.Relationship{
			ID:         fmt.Sprintf("rId%d", nextRelationshipID(slideRels)),
			Type:       opc.RelTypeSlideLayout,
			Target:     partNameToRelTarget(layout.partName, path.Dir(slidePartName)+"/"),
			TargetMode: opc.TargetModeInternal,
		})
		p.relationships[slidePartName] = slideRels
		return
	}
}

// hasRelOfType reports whether rels already contains a relationship of the given
// type.
func hasRelOfType(rels []*opc.Relationship, relType string) bool {
	for _, rel := range rels {
		if rel != nil && rel.Type == relType {
			return true
		}
	}
	return false
}

// hasRelForTarget reports whether rels already contains a relationship with the
// given type and target.
func hasRelForTarget(rels []*opc.Relationship, relType, target string) bool {
	for _, rel := range rels {
		if rel != nil && rel.Type == relType && rel.Target == target {
			return true
		}
	}
	return false
}

// nextRelationshipID returns the lowest rIdN number unused by rels.
//
// It is scope-blind by construction: it sees only the slice it is handed, so it
// is correct only when that slice is the complete relationship set for exactly
// one part. It must NEVER be used for the presentation part, whose id space has
// a second, invisible claimant — p.nextRelID, holding ids handed to slides whose
// relationships only enter p.relationships at save time (C363). Presentation
// allocation goes through nextPresentationRelID / addPresentationRel; the
// TestNoBlindPresentationRelIDAllocation guard enforces that statically.
func nextRelationshipID(rels []*opc.Relationship) int {
	maxID := 0
	for _, rel := range rels {
		var id int
		if _, err := fmt.Sscanf(rel.ID, "rId%d", &id); err == nil && id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}

// nextRelIDNum returns a relationship id number free for partName's own
// relationship scope, routing the presentation part to the allocator that also
// consults p.nextRelID. Prefer it over calling nextRelationshipID with a map
// lookup: it cannot be handed the wrong scope.
func (p *Presentation) nextRelIDNum(partName string) int {
	if partName == presentationPartName {
		return p.nextPresentationRelID()
	}
	return nextRelationshipID(p.relationships[partName])
}

// nextPresentationRelID returns a relationship id number free for the
// presentation part and reserves it, so two consecutive calls never collide.
//
// This is the single allocator for presentation-level relationship ids. The
// presentation part is the one scope with two claimants that cannot see each
// other: relationships already registered in p.relationships, and p.nextRelID,
// which holds ids handed out to slides by AddSlide whose relationships are only
// materialized by writePresentationRelationships at save time. An allocator
// reading just one of the two hands out an id the other already owns — the save
// path then keeps the first relationship carrying that id and drops the second
// as a duplicate, leaving p:sldId entries resolved to slideMaster/notesMaster
// relationships (C363). Consult both, and bump p.nextRelID past the result.
func (p *Presentation) nextPresentationRelID() int {
	maxID := p.nextRelID - 1
	for _, rel := range p.relationships[presentationPartName] {
		if rel == nil {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(rel.ID, "rId%d", &id); err == nil && id > maxID {
			maxID = id
		}
	}
	p.nextRelID = maxID + 2
	return maxID + 1
}

// addPresentationRel appends a relationship on the presentation part with a
// freshly allocated, collision-free id and returns it. Every presentation-level
// relationship added outside the save path's own rebuild should be created here
// rather than by formatting an id and appending by hand.
func (p *Presentation) addPresentationRel(relType, target string) *opc.Relationship {
	rel := &opc.Relationship{
		ID:         fmt.Sprintf("rId%d", p.nextPresentationRelID()),
		Type:       relType,
		Target:     target,
		TargetMode: opc.TargetModeInternal,
	}
	p.relationships[presentationPartName] = append(p.relationships[presentationPartName], rel)
	return rel
}

// partNameToRelsPath converts a part name to its .rels file path.
// e.g., "/ppt/slides/slide1.xml" -> "/ppt/slides/_rels/slide1.xml.rels"
func partNameToRelsPath(partName string) string {
	dir := partName[:strings.LastIndex(partName, "/")+1]
	file := partName[strings.LastIndex(partName, "/")+1:]
	return dir + "_rels/" + file + ".rels"
}

// writePresentationRelationships writes the presentation.xml.rels file.
func (p *Presentation) writePresentationRelationships(writer *opc.Writer) error {
	rels := p.presentationRelationships()

	if data := p.unchangedRawRels("/ppt/presentation.xml", rels); data != nil {
		return writer.WritePreservedPart("/ppt/_rels/presentation.xml.rels", opc.ContentTypeRelationships, data)
	}

	data, err := opc.MarshalRelationships(rels)
	if err != nil {
		return err
	}

	return writer.WritePart("/ppt/_rels/presentation.xml.rels", opc.ContentTypeRelationships, data)
}

// presentationRelationships computes the relationship set the save path will
// emit for presentation.xml. It is pure — it reads the model and writes nothing
// — so Validate can gate on the relationships that will actually be written
// rather than on the ones currently registered, which differ (pending slide rels
// are not in the map yet, and rels for departed slides still are).
func (p *Presentation) presentationRelationships() []*opc.Relationship {
	var rels []*opc.Relationship
	currentSlideTargets := make(map[string]string, len(p.slides))
	for i, slide := range p.slides {
		slideName := slide.partName
		if slideName == "" {
			slideName = fmt.Sprintf("/ppt/slides/slide%d.xml", i+1)
		}
		currentSlideTargets[slide.relID] = partNameToRelTarget(slideName, "/ppt/")
	}

	if existingRels, ok := p.relationships["/ppt/presentation.xml"]; ok && len(existingRels) > 0 {
		// Start with existing relationships (preserves original order and IDs)
		rels = make([]*opc.Relationship, 0, len(existingRels))

		// Build set of existing relationship IDs
		existingIDs := make(map[string]bool)
		for _, rel := range existingRels {
			if rel.Type == opc.RelTypeSlide {
				target, ok := currentSlideTargets[rel.ID]
				if !ok {
					// Not one of the slides in sldIdLst. A slide part the
					// package carries without listing it there is preserved
					// verbatim in otherParts (C118); dropping its relationship
					// would leave the part rel-orphaned and — because the rel
					// set then differs from the parsed original — would also
					// force presentation.xml.rels to be regenerated, losing
					// byte-identity on an otherwise untouched deck (C512).
					// Anything else here is a rel whose slide really is gone.
					if p.unreferencedSlides[opc.ResolvePartName("/ppt/presentation.xml", rel.Target)] {
						copied := *rel
						rels = append(rels, &copied)
						existingIDs[rel.ID] = true
					}
					continue
				}
				copied := *rel
				// Keep the source's target spelling when it still resolves to
				// the same slide part: some producers write absolute targets
				// ("/ppt/slides/slide1.xml"), and rewriting them to the
				// relative form would make an otherwise unmodified set differ
				// from the parsed original, forcing the .rels part to be
				// regenerated (with a canonical prolog) instead of preserved
				// verbatim.
				if opc.ResolvePartName("/ppt/presentation.xml", rel.Target) != opc.ResolvePartName("/ppt/presentation.xml", target) {
					copied.Target = target
				}
				rels = append(rels, &copied)
				existingIDs[rel.ID] = true
				continue
			}

			copied := *rel
			rels = append(rels, &copied)
			existingIDs[rel.ID] = true
		}

		// Add relationships for any new slides not already in the existing rels
		for i, slide := range p.slides {
			if !existingIDs[slide.relID] {
				slideName := slide.partName
				if slideName == "" {
					slideName = fmt.Sprintf("/ppt/slides/slide%d.xml", i+1)
				}
				target := partNameToRelTarget(slideName, "/ppt/")
				rels = append(rels, &opc.Relationship{
					ID:     slide.relID,
					Type:   opc.RelTypeSlide,
					Target: target,
				})
			}
		}
	} else {
		// No existing relationships - build from scratch for new presentations.
		for i, master := range p.slideMasters {
			masterName := master.partName
			if masterName == "" {
				masterName = fmt.Sprintf("/ppt/slideMasters/slideMaster%d.xml", i+1)
			}
			target := partNameToRelTarget(masterName, "/ppt/")
			rels = append(rels, &opc.Relationship{
				ID:     master.relID,
				Type:   opc.RelTypeSlideMaster,
				Target: target,
			})
		}

		for i, slide := range p.slides {
			slideName := slide.partName
			if slideName == "" {
				slideName = fmt.Sprintf("/ppt/slides/slide%d.xml", i+1)
			}
			target := partNameToRelTarget(slideName, "/ppt/")
			rels = append(rels, &opc.Relationship{
				ID:     slide.relID,
				Type:   opc.RelTypeSlide,
				Target: target,
			})
		}
	}

	return rels
}

// outputRelationships computes the whole package's relationship map as the save
// path will emit it: the presentation part's set comes from
// presentationRelationships (pending slide rels included, departed ones gone),
// and the .rels of parts the save will not write are omitted, mirroring the skip
// in save's relationship loop.
//
// It is the source-side half of answering relationship questions against the
// output rather than the input; partExists is the target-side half. Together
// they are what lets Validate see a deletion-induced dangling reference at all
// (audit tension T-A). Pure: it copies, and mutates nothing.
func (p *Presentation) outputRelationships() map[string][]*opc.Relationship {
	currentSlideParts := make(map[string]bool, len(p.slides))
	for _, s := range p.slides {
		if s != nil && s.partName != "" {
			currentSlideParts[s.partName] = true
		}
	}
	out := make(map[string][]*opc.Relationship, len(p.relationships))
	for partName, rels := range p.relationships {
		if partName == presentationPartName || len(rels) == 0 {
			continue
		}
		// A part deleted this session takes its .rels with it, and so does a
		// slide part no longer in sldIdLst — except a preserved unreferenced
		// slide, whose rels the save keeps (C118).
		if p.removedParts[partName] {
			continue
		}
		if strings.HasPrefix(partName, "/ppt/slides/") && !currentSlideParts[partName] && !p.unreferencedSlides[partName] {
			continue
		}
		out[partName] = rels
	}
	if presRels := p.presentationRelationships(); len(presRels) > 0 {
		out[presentationPartName] = presRels
	}
	return out
}

// remapCustomShowRefs rewrites custom-show slide references through an old->new
// relationship-id map, so a p:custShow recorded against a slide's earlier id
// still points at that same slide after saveNew reassigns relationship ids by
// order. It is a no-op when the map is empty (nothing was reordered).
func (p *Presentation) remapCustomShowRefs(remap map[string]string) {
	if len(remap) == 0 || p.presentation == nil || p.presentation.CustShowLst == nil {
		return
	}
	for i := range p.presentation.CustShowLst.CustShow {
		lst := p.presentation.CustShowLst.CustShow[i].SldLst
		if lst == nil {
			continue
		}
		for j := range lst.Sld {
			if newID, ok := remap[lst.Sld[j].Id]; ok {
				lst.Sld[j].Id = newID
			}
		}
	}
}

// saveNew saves a newly created presentation.
func (p *Presentation) saveNew(writer *opc.Writer) error {
	// Set properties
	p.Properties.Modified = time.Now()
	writer.Properties = &p.Properties
	if p.customProps != nil && p.customProps.Len() > 0 {
		writer.CustomProperties = p.customProps
	}

	// Set extended properties. The format string reflects the actual slide
	// size (previously hardcoded to "Widescreen" even for 4:3 decks).
	format := "On-screen Show (4:3)"
	if p.presentation.SlideSize != nil && p.presentation.SlideSize.Cx == 12192000 {
		format = "Widescreen"
	}
	writer.ExtendedProperties = &opc.ExtendedProperties{
		Slides:             len(p.slides),
		PresentationFormat: format,
	}

	// Assign fresh relationship IDs for all parts
	// PowerPoint expects: slideMaster first, then slides, then other parts (theme, presProps, etc.)
	presRelID := 1

	// Reserve relationship ids already taken by presentation-level rels created
	// before save (embedded-font rels from EmbedFont, an injected VBA project),
	// so the master/slide/fixed-part ids assigned below do not collide with them
	// (and the p:embeddedFont r:id references stay valid).
	for _, rel := range p.relationships[presentationPartName] {
		if rel == nil {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(rel.ID, "rId%d", &id); err == nil && id >= presRelID {
			presRelID = id + 1
		}
	}

	// Assign relationship IDs to masters FIRST
	for _, master := range p.slideMasters {
		master.relID = fmt.Sprintf("rId%d", presRelID)
		presRelID++
	}

	// Assign relationship IDs to slides SECOND, recording the old->new id map so
	// stored consumers of a slide's relId survive the reassignment. Reassigning
	// by slice order otherwise made a MoveSlide/EmbedFont between AddCustomShow
	// and Save silently repoint the custom show at whatever slide landed on the
	// referenced id (C255).
	slideRelIDRemap := make(map[string]string, len(p.slides))
	for _, slide := range p.slides {
		newID := fmt.Sprintf("rId%d", presRelID)
		if slide.relID != "" && slide.relID != newID {
			slideRelIDRemap[slide.relID] = newID
		}
		slide.relID = newID
		presRelID++
	}
	p.remapCustomShowRefs(slideRelIDRemap)

	// Fixed parts (presProps, viewProps, theme, tableStyles) get IDs after masters and slides
	presPropsRelID := fmt.Sprintf("rId%d", presRelID)
	presRelID++
	viewPropsRelID := fmt.Sprintf("rId%d", presRelID)
	presRelID++
	themeRelID := fmt.Sprintf("rId%d", presRelID)
	presRelID++
	tableStylesRelID := fmt.Sprintf("rId%d", presRelID)

	// Create presentation part (now with correct relationship IDs)
	presData, err := p.marshalPresentation()
	if err != nil {
		return err
	}

	if err := writer.WritePart("/ppt/presentation.xml", p.Flavor(), presData); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, "ppt/presentation.xml", opc.TargetModeInternal); err != nil {
		return err
	}

	// Collect presentation relationships
	presRels := make([]*opc.Relationship, 0)

	// writtenThemes tracks imported per-master theme parts already emitted, so a
	// theme shared by two imported masters is written once.
	writtenThemes := make(map[string]bool)

	// Write slide masters and their layouts FIRST
	for i, master := range p.slideMasters {
		// Use original part name if loaded from file, otherwise generate
		masterPartName := master.partName
		if masterPartName == "" {
			masterPartName = fmt.Sprintf("/ppt/slideMasters/slideMaster%d.xml", i+1)
		}
		masterRelTarget := partNameToRelTarget(masterPartName, "/ppt/")

		masterData, err := master.marshal()
		if err != nil {
			return err
		}

		if err := writer.WritePart(masterPartName, opc.ContentTypeSlideMaster, masterData); err != nil {
			return err
		}

		// Add relationship from presentation to master
		presRels = append(presRels, &opc.Relationship{
			ID:         master.relID,
			Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster",
			Target:     masterRelTarget,
			TargetMode: opc.TargetModeInternal,
		})

		// Write layouts for this master. Start the master's relationship set
		// from any master-level rels the model already carries (e.g. an image
		// background embedded before save), then add the layout and theme rels.
		masterRels := append([]*opc.Relationship(nil), p.relationships[masterPartName]...)
		for j, layout := range master.layouts {
			// Use original part name if loaded from file, otherwise generate
			layoutPartName := layout.partName
			if layoutPartName == "" {
				layoutPartName = fmt.Sprintf("/ppt/slideLayouts/slideLayout%d.xml", j+1)
			}
			layoutRelTarget := partNameToRelTarget(layoutPartName, "/ppt/slideMasters/")

			layoutData, err := layout.marshal()
			if err != nil {
				return err
			}

			if err := writer.WritePart(layoutPartName, opc.ContentTypeSlideLayout, layoutData); err != nil {
				return err
			}

			// Relationship from master to layout. An imported master already carries
			// this rel in p.relationships (registered when the layout was cloned);
			// only synthesize it when absent so the .rels does not list the id
			// twice (C236).
			if !hasRelForTarget(masterRels, opc.RelTypeSlideLayout, layoutRelTarget) {
				masterRels = append(masterRels, &opc.Relationship{
					ID:         layout.relID,
					Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout",
					Target:     layoutRelTarget,
					TargetMode: opc.TargetModeInternal,
				})
			}

			// Write layout relationships (back to master), starting from any
			// layout-level rels the model already carries (e.g. an image
			// background embedded before save) and giving the required
			// slideMaster rel a non-colliding id.
			layoutRels := append([]*opc.Relationship(nil), p.relationships[layoutPartName]...)
			hasMasterRel := false
			for _, rel := range layoutRels {
				if rel != nil && rel.Type == opc.RelTypeSlideMaster {
					hasMasterRel = true
					break
				}
			}
			if !hasMasterRel {
				layoutRels = append(layoutRels, &opc.Relationship{
					ID:         fmt.Sprintf("rId%d", nextRelationshipID(layoutRels)),
					Type:       opc.RelTypeSlideMaster,
					Target:     partNameToRelTarget(masterPartName, "/ppt/slideLayouts/"),
					TargetMode: opc.TargetModeInternal,
				})
			}
			if err := writer.WritePartRelationships(layoutPartName, layoutRels); err != nil {
				return err
			}
		}

		// Write master relationships (always include theme). A master carrying an
		// imported theme (from a merge) points at that theme part, which is
		// written here once; every other master uses the default theme1.xml.
		// Allocate the next free rel id rather than assuming the layout ids are
		// contiguous from 1: a gap or an out-of-sequence layout relID would make
		// len+1 collide.
		themeTarget := "../theme/theme1.xml"
		if master.themePartName != "" {
			themeTarget = partNameToRelTarget(master.themePartName, "/ppt/slideMasters/")
			if data, ok := p.themeData[master.themePartName]; ok && !writtenThemes[master.themePartName] {
				if err := writer.WritePart(master.themePartName, opc.ContentTypeTheme, data); err != nil {
					return err
				}
				writtenThemes[master.themePartName] = true
			}
		}
		// An imported master already carries its theme rel in p.relationships
		// (registered by importMaster); only synthesize it when absent so the
		// .rels does not list two theme rels (C236). The theme part itself is
		// still written above.
		if !hasRelOfType(masterRels, opc.RelTypeTheme) {
			masterRels = append(masterRels, &opc.Relationship{
				ID:         fmt.Sprintf("rId%d", nextRelationshipID(masterRels)),
				Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme",
				Target:     themeTarget,
				TargetMode: opc.TargetModeInternal,
			})
		}

		if err := writer.WritePartRelationships(masterPartName, masterRels); err != nil {
			return err
		}
	}

	// Create slide parts and relationships SECOND. Slides keep the part name
	// assigned when they were added: slide-level relationships (media, images)
	// are keyed by part name, so re-deriving names from the current index here
	// would detach those rels from their slide after a MoveSlide/RemoveSlide.
	currentSlideParts := make(map[string]bool, len(p.slides))
	for _, slide := range p.slides {
		if slide.partName == "" || currentSlideParts[slide.partName] {
			slide.partName = p.nextAvailableSlidePartName()
		}
		currentSlideParts[slide.partName] = true
	}
	for _, slide := range p.slides {
		slidePartName := slide.partName

		slideRels := append([]*opc.Relationship(nil), p.relationships[slidePartName]...)
		hasLayoutRel := false
		for _, rel := range slideRels {
			if rel.Type == opc.RelTypeSlideLayout {
				hasLayoutRel = true
				break
			}
		}

		if !hasLayoutRel {
			// Allocate the next free rel ID: the slide may already carry media
			// or image relationships created before the first save.
			layoutRelID := fmt.Sprintf("rId%d", nextRelationshipID(slideRels))
			if slide.layout != nil {
				// Find layout index
				for j, layout := range p.slideLayouts {
					if layout == slide.layout {
						slideRels = append(slideRels, &opc.Relationship{
							ID:         layoutRelID,
							Type:       opc.RelTypeSlideLayout,
							Target:     fmt.Sprintf("../slideLayouts/slideLayout%d.xml", j+1),
							TargetMode: opc.TargetModeInternal,
						})
						break
					}
				}
			} else if len(p.slideLayouts) > 0 {
				slideRels = append(slideRels, &opc.Relationship{
					ID:         layoutRelID,
					Type:       opc.RelTypeSlideLayout,
					Target:     "../slideLayouts/slideLayout1.xml",
					TargetMode: opc.TargetModeInternal,
				})
			}
		}
		if len(slideRels) > 0 {
			p.relationships[slidePartName] = slideRels
		}

		// Every slide is regenerated from its model here. In this "save new" path
		// there is no reader, so there are no original bytes to pass through:
		// rawBytes returns nil unconditionally, and every slide reachable here
		// has a model anyway. The passthrough branch this replaced was therefore
		// dead twice over, and it matters only in the round-trip save (C515).
		slideData, err := slide.marshal()
		if err != nil {
			return err
		}

		if err := writer.WritePart(slidePartName, opc.ContentTypeSlide, slideData); err != nil {
			return err
		}

		presRels = append(presRels, &opc.Relationship{
			ID:         slide.relID,
			Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide",
			Target:     partNameToRelTarget(slidePartName, "/ppt/"),
			TargetMode: opc.TargetModeInternal,
		})

		slideRels = p.relationships[slidePartName]

		if len(slideRels) > 0 {
			if err := writer.WritePartRelationships(slidePartName, slideRels); err != nil {
				return err
			}
		}
	}

	// Write presentation properties (after masters and slides)
	if err := writer.WritePart("/ppt/presProps.xml", opc.ContentTypePresentationProps, defaultPresPropsXML()); err != nil {
		return err
	}
	presRels = append(presRels, &opc.Relationship{
		ID:         presPropsRelID,
		Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps",
		Target:     "presProps.xml",
		TargetMode: opc.TargetModeInternal,
	})

	// Write view properties
	if err := writer.WritePart("/ppt/viewProps.xml", opc.ContentTypeViewProps, defaultViewPropsXML()); err != nil {
		return err
	}
	presRels = append(presRels, &opc.Relationship{
		ID:         viewPropsRelID,
		Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps",
		Target:     "viewProps.xml",
		TargetMode: opc.TargetModeInternal,
	})

	// Write theme file
	themePartName := "/ppt/theme/theme1.xml"
	if err := writer.WritePart(themePartName, opc.ContentTypeTheme, defaultThemeXML()); err != nil {
		return err
	}
	presRels = append(presRels, &opc.Relationship{
		ID:         themeRelID,
		Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme",
		Target:     "theme/theme1.xml",
		TargetMode: opc.TargetModeInternal,
	})

	// Write table styles
	if err := writer.WritePart("/ppt/tableStyles.xml", opc.ContentTypeTableStyles, defaultTableStylesXML()); err != nil {
		return err
	}
	presRels = append(presRels, &opc.Relationship{
		ID:         tableStylesRelID,
		Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles",
		Target:     "tableStyles.xml",
		TargetMode: opc.TargetModeInternal,
	})

	// Write media and other auxiliary parts created while marshaling slides
	// (sorted for deterministic package output), skipping media parts no
	// relationship references anymore (C221).
	deadMedia := p.unreferencedMediaParts()
	for _, name := range sortedKeys(p.otherParts) {
		if deadMedia[name] {
			continue
		}
		part := p.otherParts[name]
		if err := writer.WritePart(name, part.ContentType, part.Data); err != nil {
			return err
		}
		// Write this part's own relationships (e.g. a chart part's link to its
		// embedded workbook). Slide/master/layout/presentation rels are handled
		// above; auxiliary parts are not, so their .rels would otherwise be lost.
		if len(p.relationships[name]) > 0 {
			if err := p.writePartRelationships(writer, name); err != nil {
				return err
			}
		}
	}

	// Write theme parts imported by non-master furniture (e.g. a carried handout
	// master's theme). The master loop above emits per-master themes and the
	// default theme1.xml is written separately; anything else in themeData must be
	// emitted here so the auxiliary part's theme relationship resolves.
	for _, themeName := range sortedKeys(p.themeData) {
		if themeName == themePartName || writtenThemes[themeName] {
			continue
		}
		if err := writer.WritePart(themeName, opc.ContentTypeTheme, p.themeData[themeName]); err != nil {
			return err
		}
		writtenThemes[themeName] = true
	}

	// Include presentation-level relationships added to the model that the
	// inline builder above does not emit (embedded-font rels from EmbedFont, an
	// injected VBA project). Merge by id so nothing is duplicated.
	for _, rel := range p.relationships[presentationPartName] {
		if rel == nil {
			continue
		}
		dup := false
		for _, existing := range presRels {
			// Match by id, and also by type+target: importMaster pre-registers a
			// presentation->master rel (with its own id) that saveNew regenerates
			// above under a reassigned id — without the type+target check the two
			// would both be emitted, duplicating the rel (C236).
			if existing.ID == rel.ID ||
				(existing.Type == rel.Type && existing.Target == rel.Target) {
				dup = true
				break
			}
		}
		if !dup {
			presRels = append(presRels, rel)
		}
	}

	// Write presentation relationships
	if err := writer.WritePartRelationships("/ppt/presentation.xml", presRels); err != nil {
		return err
	}

	return nil
}

// partNameToRelTarget converts an absolute part name to a relative target path.
// baseDir is the directory of the source part (e.g., "/ppt/" or "/ppt/slideMasters/").
func partNameToRelTarget(partName, baseDir string) string {
	// Remove leading slash if present
	if len(partName) > 0 && partName[0] == '/' {
		partName = partName[1:]
	}
	if len(baseDir) > 0 && baseDir[0] == '/' {
		baseDir = baseDir[1:]
	}

	// If part is in the base directory, just return the filename
	if len(partName) > len(baseDir) && partName[:len(baseDir)] == baseDir {
		return partName[len(baseDir):]
	}

	// Otherwise compute relative path with ../
	// Count levels to go up from baseDir
	baseParts := splitPath(baseDir)
	partParts := splitPath(partName)

	// Find common prefix
	commonLen := 0
	for i := 0; i < len(baseParts) && i < len(partParts); i++ {
		if baseParts[i] == partParts[i] {
			commonLen = i + 1
		} else {
			break
		}
	}

	// Build relative path
	result := ""
	for i := commonLen; i < len(baseParts); i++ {
		result += "../"
	}
	for i := commonLen; i < len(partParts); i++ {
		if i > commonLen {
			result += "/"
		}
		result += partParts[i]
	}

	return result
}

// splitPath splits a path into its components.
func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// slideMasterIDBase and slideLayoutIDBase are the low ends of ST_SlideMasterId
// and ST_SlideLayoutId (both are unsignedInt constrained to >= 2147483648;
// sldLayoutId conventionally starts one higher). Both id spaces are
// document-unique, not master-unique.
const (
	slideMasterIDBase uint32 = 2147483648
	slideLayoutIDBase uint32 = 2147483649
)

// nextIDAbove returns the first id at or above base that no item already holds,
// reading each item's id through id (0 meaning "carries none"). It saturates at
// math.MaxUint32 rather than wrapping.
func nextIDAbove[T any](base uint32, items []T, id func(T) uint32) uint32 {
	next := base
	for _, it := range items {
		v := id(it)
		if v == 0 || v < next {
			continue
		}
		if v == math.MaxUint32 {
			return math.MaxUint32
		}
		next = v + 1
	}
	return next
}

// marshalPresentation generates the presentation.xml content.
func (p *Presentation) marshalPresentation() ([]byte, error) {
	// Update slide master IDs
	if len(p.slideMasters) > 0 {
		p.presentation.SlideMasterIDs = &oxml.SlideMasterIDs{
			SlideMasterID: make([]oxml.SlideMasterID, len(p.slideMasters)),
		}
		// ST_SlideMasterId values are document-unique. Masters that carry no id
		// of their own take one above every id the preserved masters already
		// hold rather than the fixed base+index: a destination master preserving
		// e.g. 2147483649 at index 0 collided with the index-1 fallback (C419).
		nextMasterID := nextIDAbove(slideMasterIDBase, p.slideMasters, func(m *SlideMaster) uint32 {
			if m == nil {
				return 0
			}
			return m.numericID
		})
		for i, master := range p.slideMasters {
			id := master.numericID
			if id == 0 && !master.idOmitted {
				id = nextMasterID
				if nextMasterID < math.MaxUint32 {
					nextMasterID++
				}
			}
			p.presentation.SlideMasterIDs.SlideMasterID[i] = oxml.SlideMasterID{
				ID:        id,
				IDOmitted: master.idOmitted && master.numericID == 0,
				RID:       master.relID,
				ExtLst:    master.idExtLst,
			}
		}
	}

	// Update slide IDs
	p.presentation.SlideIDs = &oxml.SlideIDs{
		SlideID: make([]oxml.SlideID, len(p.slides)),
	}
	for i, slide := range p.slides {
		p.presentation.SlideIDs.SlideID[i] = oxml.SlideID{
			ID:     slide.id,
			RID:    slide.relID,
			ExtLst: slide.idExtLst,
		}
	}

	// Use the namespace-aware marshaler for PowerPoint compatibility.
	// Only decks created programmatically (no backing reader) get a
	// fabricated defaultTextStyle; opened decks keep whatever they had.
	return marshalPresentationXML(p.presentation, p.reader == nil)
}

// Close releases resources held by a presentation opened from a file. Open and
// OpenReader read the whole package into memory up front and retain no OS file
// handle, so Close is effectively a no-op. Calling Save (or any Save* method)
// after Close is valid: the in-memory model and preserved parts remain intact.
func (p *Presentation) Close() error {
	if p.reader != nil {
		return p.reader.Close()
	}
	return nil
}

// Slides returns all slides in the presentation.
func (p *Presentation) Slides() []*Slide {
	return p.slides
}

// SlideCount returns the number of slides.
func (p *Presentation) SlideCount() int {
	return len(p.slides)
}

// Slide returns the slide at the specified index (0-based).
func (p *Presentation) Slide(index int) (*Slide, error) {
	if index < 0 || index >= len(p.slides) {
		return nil, ErrSlideIndex
	}
	return p.slides[index], nil
}

// slideIndexByPart returns the 0-based index of the slide stored at partName, or
// -1 when no loaded slide matches.
func (p *Presentation) slideIndexByPart(partName string) int {
	for i, s := range p.slides {
		if s != nil && s.partName == partName {
			return i
		}
	}
	return -1
}

// slidePartByIndex returns the part name of the slide at the 0-based index, or ""
// when the index is out of range.
func (p *Presentation) slidePartByIndex(index int) string {
	if index < 0 || index >= len(p.slides) || p.slides[index] == nil {
		return ""
	}
	return p.slides[index].partName
}

// AddSlide adds a new blank slide to the presentation.
func (p *Presentation) AddSlide() *Slide {
	// The slide's presentation relationship is only materialized at save, so
	// its id must be reserved against both claimants of the presentation id
	// space right now — otherwise a merge-time importer allocating from
	// p.relationships alone reuses it and the save drops one of the two (C363).
	relID := fmt.Sprintf("rId%d", p.nextPresentationRelID())

	slide := &Slide{
		presentation: p,
		index:        len(p.slides),
		id:           p.nextSlideID,
		relID:        relID,
		// A created slide has no bytes to parse lazily, so its model is built up
		// front and marked parsed; it always marshals rather than passing raw
		// bytes through.
		sxModel:  newSlideXML(),
		sxParsed: true,
		// Assign the part name eagerly so the slide has a stable identity from
		// the moment it exists. Slide-level relationships (media, images) are
		// keyed by part name; allocating it lazily at save time meant rels
		// created before the first save were stored under "" and lost, and
		// index-derived names reattached rels to the wrong slide after a
		// MoveSlide/RemoveSlide.
		partName: p.nextAvailableSlidePartName(),
	}
	p.nextSlideID++

	p.slides = append(p.slides, slide)
	return slide
}

func (p *Presentation) nextAvailableSlidePartName() string {
	used := make(map[string]bool, len(p.slides)+len(p.otherParts))
	for _, slide := range p.slides {
		if slide.partName != "" {
			used[slide.partName] = true
		}
	}
	for name := range p.otherParts {
		used[name] = true
	}
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/slides/slide%d.xml", i)
		if !used[name] {
			return name
		}
	}
}

// deepCloneNotesSlide gives the slide at newSlidePart its own copy of the notes
// slide it currently references (shared verbatim from the slide it was
// duplicated from). Without this, both slides point at one notesSlide part and
// editing one slide's notes changes the other.
func (p *Presentation) deepCloneNotesSlide(newSlidePart string) {
	for _, rel := range p.relationships[newSlidePart] {
		if rel == nil || rel.Type != opc.RelTypeNotesSlide {
			continue
		}
		srcNotes := opc.ResolvePartName(newSlidePart, rel.Target)
		part, ok := p.otherParts[srcNotes]
		if !ok {
			return
		}

		newNotes := p.nextAvailableNotesName()
		copied := *part
		p.otherParts[newNotes] = &copied

		// Point the duplicate's notesSlide relationship at the new part.
		rel.Target = relativeTarget(newSlidePart, newNotes)

		// Copy the notes part's own relationships, repointing its slide
		// back-reference to the duplicate rather than the original slide.
		var notesRels []*opc.Relationship
		for _, nr := range p.relationships[srcNotes] {
			if nr == nil {
				continue
			}
			c := *nr
			if c.Type == opc.RelTypeSlide {
				c.Target = relativeTarget(newNotes, newSlidePart)
			}
			notesRels = append(notesRels, &c)
		}
		if len(notesRels) > 0 {
			p.relationships[newNotes] = notesRels
		}
		return
	}
}

// deepCloneCommentParts gives the slide at newSlidePart its own copy of every
// comment part it currently references — legacy p:cmLst and modern threaded
// alike — instead of the copy it inherited verbatim from the slide it was
// duplicated from (C414).
//
// A comments part belongs to exactly one slide in ECMA's model, so sharing one
// between two slides is wrong twice over: an edit through either slide changed
// both, and the modern part's pc:sldMk sldId still named the *source* slide, so
// the duplicate carried a thread anchored somewhere else. The anchor rewrite
// reuses the merge path's helper, which faces the identical problem when a slide
// is imported from another deck.
func (p *Presentation) deepCloneCommentParts(newSlidePart string, srcSlideID, newSlideID uint32) {
	for _, rel := range p.relationships[newSlidePart] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		if rel.Type != opc.RelTypeComments && rel.Type != opc.RelTypeModernComments {
			continue
		}
		srcPart := opc.ResolvePartName(newSlidePart, rel.Target)
		part, ok := p.otherParts[srcPart]
		if !ok || part == nil {
			continue
		}

		newPart := p.freePartNameLike(srcPart)
		copied := *part
		// Clone the bytes: the two parts diverge from here (the anchor rewrite
		// below is the first divergence), so they must not share a backing array.
		copied.Data = bytes.Clone(part.Data)
		p.otherParts[newPart] = &copied
		rel.Target = relativeTarget(newSlidePart, newPart)

		// Carry the part's own relationships, repointing any slide back-reference
		// at the duplicate.
		var partRels []*opc.Relationship
		for _, pr := range p.relationships[srcPart] {
			if pr == nil {
				continue
			}
			c := *pr
			if c.Type == opc.RelTypeSlide && c.TargetMode != opc.TargetModeExternal {
				c.Target = relativeTarget(newPart, newSlidePart)
			}
			partRels = append(partRels, &c)
		}
		if len(partRels) > 0 {
			p.relationships[newPart] = partRels
		}

		if rel.Type == opc.RelTypeModernComments {
			p.rewriteModernCommentSlideID(newPart, srcSlideID, newSlideID)
		}
	}
}

// nextAvailableNotesName returns a notesSlide part name not already in use.
func (p *Presentation) nextAvailableNotesName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/notesSlides/notesSlide%d.xml", i)
		if _, exists := p.otherParts[name]; !exists {
			return name
		}
	}
}

// nextAvailableLayoutPartName returns a slideLayout part name not already used
// by an existing layout or other part.
func (p *Presentation) nextAvailableLayoutPartName() string {
	used := make(map[string]bool, len(p.slideLayouts)+len(p.otherParts))
	for _, l := range p.slideLayouts {
		if l.partName != "" {
			used[l.partName] = true
		}
	}
	for name := range p.otherParts {
		used[name] = true
	}
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/slideLayouts/slideLayout%d.xml", i)
		if !used[name] {
			return name
		}
	}
}

func (p *Presentation) clonePartRelationships(sourcePart, targetPart string) {
	if sourcePart == "" || targetPart == "" {
		return
	}
	sourceRels := p.relationships[sourcePart]
	if len(sourceRels) == 0 {
		return
	}
	cloned := make([]*opc.Relationship, 0, len(sourceRels))
	for _, rel := range sourceRels {
		if rel == nil {
			continue
		}
		copied := *rel
		cloned = append(cloned, &copied)
	}
	p.relationships[targetPart] = cloned
}

// AddSlideWithLayout adds a new slide using the specified layout. It is an
// alias for AddSlideFromLayout, kept for API compatibility.
//
// Deprecated: use AddSlideFromLayout. The two are the same call and no sibling
// format carries a second spelling of its slide/sheet/section constructor
// (C567).
func (p *Presentation) AddSlideWithLayout(layout *SlideLayout) *Slide {
	return p.AddSlideFromLayout(layout)
}

// RemoveSlide removes the slide at the specified index. Parts owned
// exclusively by the slide — its notes slide and comments part, along with
// their relationships and content-type overrides — are removed with it
// (previously the orphaned notes part kept a slide back-reference that ended
// up pointing at whatever unrelated slide later reused the freed part name,
// C88). Media and image parts may be shared between slides, so they are
// garbage-collected at save time instead (C221).
//
// References held by everything that stays are swept too: another slide's
// slide-jump hyperlink relationship (C364), the removed slide's membership in
// any section (C307) and in any custom show (C365). Without the sweep the saved
// package carried a relationship to a part it did not contain — OPC-invalid —
// and a p:custShow r:id that resolved to nothing.
func (p *Presentation) RemoveSlide(index int) error {
	if index < 0 || index >= len(p.slides) {
		return ErrSlideIndex
	}

	removed := p.slides[index]

	// Clean up relationships for the removed slide to prevent stale entries from
	// leaking into a new slide that may later reuse the same part name during save.
	if partName := removed.partName; partName != "" {
		p.removeSlideOwnedParts(partName)
		delete(p.relationships, partName)
		p.markPartRemoved(partName)
		p.sweepInboundPartReferences(partName)
	}
	p.mediaGCNeeded = true

	// Strip the removed slide's id from any section it belonged to, so the
	// section list does not emit a p14:sldId that no longer resolves to a slide
	// in the sldIdLst (a dangling reference Validate does not flag) — C307.
	p.removeSlideFromSections(removed.id)

	// Same for custom shows, which name their members by presentation-level
	// relationship id: the rel goes away with the slide, so an entry left behind
	// is a schema-invalid ST_RelationshipId (C365).
	p.removeSlideFromCustomShows(removed.relID)

	// Remove the slide
	p.slides = append(p.slides[:index], p.slides[index+1:]...)

	// Update indices
	for i := index; i < len(p.slides); i++ {
		p.slides[i].index = i
	}

	// Invalidate the removed handle so a stale reference — a second Delete, or a
	// Duplicate on it — is rejected instead of silently acting on whatever slide
	// now occupies the freed index (C302). Live handles keep valid indices
	// (updated by the loop above); only the removed one is marked -1.
	removed.index = -1
	removed.presentation = nil

	return nil
}

// removeSlideOwnedParts deletes the parts only the given slide references:
// its notes slide and comments part. Their own relationships (e.g. the notes
// part's notes-master and slide back-reference rels) go with them. A part
// also referenced by any other part's relationships is kept.
func (p *Presentation) removeSlideOwnedParts(slidePart string) {
	for _, rel := range p.relationships[slidePart] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		if rel.Type != opc.RelTypeNotesSlide && rel.Type != opc.RelTypeComments &&
			rel.Type != opc.RelTypeModernComments {
			continue
		}
		target := opc.ResolvePartName(slidePart, rel.Target)
		if p.partReferencedElsewhere(target, slidePart) {
			continue
		}
		delete(p.otherParts, target)
		delete(p.relationships, target)
		p.markPartRemoved(target)
	}
}

// partReferencedElsewhere reports whether any part other than exclude has an
// internal relationship resolving to target.
func (p *Presentation) partReferencedElsewhere(target, exclude string) bool {
	for src, rels := range p.relationships {
		if src == exclude {
			continue
		}
		for _, rel := range rels {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			if opc.ResolvePartName(src, rel.Target) == target {
				return true
			}
		}
	}
	return false
}

// sweepInboundPartReferences drops every relationship — on any part other than
// the removed part itself — that resolves to it, and strips the matching r:id
// out of the referring slide's XML.
//
// Deletion used to clean only the removed node and its own outgoing edges, never
// the inbound ones (C364): a surviving slide with a slide-jump hyperlink kept
// both its RelTypeSlide relationship, now targeting a part absent from the zip,
// and the ppaction://hlinksldjump that used it. A relationship to a part the
// package does not contain is an OPC violation, so the relationship goes
// unconditionally; the reference in the content is cleared where the model can
// express it (slide-jump hlinkClicks, via the same helper the merge path uses
// for jumps whose target was not carried).
//
// Only parts referencing the removed one are touched, so a deck that holds no
// inbound reference is not disturbed and still round-trips byte-identically.
func (p *Presentation) sweepInboundPartReferences(removedPart string) {
	if removedPart == "" {
		return
	}
	// Snapshot the source part names first: materializing a referring slide
	// below may add relationship entries, and mutating the map under range is
	// only safe for keys that already exist.
	sources := make([]string, 0, len(p.relationships))
	for src := range p.relationships {
		if src != removedPart {
			sources = append(sources, src)
		}
	}
	for _, src := range sources {
		var dangling []string
		for _, rel := range p.relationships[src] {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			if opc.ResolvePartName(src, rel.Target) == removedPart {
				dangling = append(dangling, rel.ID)
			}
		}
		if len(dangling) == 0 {
			continue
		}
		p.stripSlideReferences(src, dangling)
		p.dropRelationships(src, dangling)
	}
}

// stripSlideReferences clears the given relationship ids from the slide stored
// at partName: the slide-jump action in its XML, and the materialized Hyperlink
// handles that carry the id (which would otherwise re-allocate a relationship
// for a stale slide index on the next save).
func (p *Presentation) stripSlideReferences(partName string, relIDs []string) {
	s := p.slideByPart(partName)
	if s == nil {
		return
	}
	sx := s.sx()
	if sx == nil {
		return
	}
	gone := make(map[string]bool, len(relIDs))
	for _, id := range relIDs {
		gone[id] = true
		stripSlideJumpRel(sx, id)
	}
	s.forEachHyperlink(func(h *Hyperlink) {
		if h != nil && gone[h.relID] {
			h.clearTarget()
		}
	})
}

// dropRelationships removes the relationships with the given ids from partName's
// relationship set.
func (p *Presentation) dropRelationships(partName string, relIDs []string) {
	rels := p.relationships[partName]
	if len(rels) == 0 || len(relIDs) == 0 {
		return
	}
	gone := make(map[string]bool, len(relIDs))
	for _, id := range relIDs {
		gone[id] = true
	}
	kept := make([]*opc.Relationship, 0, len(rels))
	for _, rel := range rels {
		if rel != nil && gone[rel.ID] {
			continue
		}
		kept = append(kept, rel)
	}
	p.relationships[partName] = kept
}

// slideByPart returns the slide stored at partName, or nil.
func (p *Presentation) slideByPart(partName string) *Slide {
	for _, s := range p.slides {
		if s != nil && s.partName == partName {
			return s
		}
	}
	return nil
}

// removeSlideFromCustomShows strips relID from every custom show's slide list.
// A show left with no members is dropped: CT_CustomShow requires a p:sldLst with
// at least one p:sld, so emitting the emptied show would be schema-invalid in a
// second way (C365).
func (p *Presentation) removeSlideFromCustomShows(relID string) {
	if relID == "" || p.presentation == nil || p.presentation.CustShowLst == nil {
		return
	}
	lst := p.presentation.CustShowLst
	kept := make([]oxml.CustomShow, 0, len(lst.CustShow))
	for _, cs := range lst.CustShow {
		if cs.SldLst != nil {
			refs := cs.SldLst.Sld[:0]
			for _, ref := range cs.SldLst.Sld {
				if ref.Id != relID {
					refs = append(refs, ref)
				}
			}
			cs.SldLst.Sld = refs
		}
		if cs.SldLst == nil || len(cs.SldLst.Sld) == 0 {
			continue
		}
		kept = append(kept, cs)
	}
	if len(kept) == 0 {
		p.presentation.CustShowLst = nil
		return
	}
	lst.CustShow = kept
}

// markPartRemoved records a part deleted this session so the save can drop
// its content-type override.
func (p *Presentation) markPartRemoved(name string) {
	if p.removedParts == nil {
		p.removedParts = make(map[string]bool)
	}
	p.removedParts[name] = true
}

// MoveSlide moves a slide from one position to another.
//
// It reorders the slide list only. Sections track membership by slide id, so a
// slide moved across a section boundary keeps the section it was in and the
// sections stop being contiguous runs of the slide order — which is a state
// PowerPoint's own UI never produces. Follow with MoveSlideToSection when the
// deck has sections and the move crosses one.
func (p *Presentation) MoveSlide(from, to int) error {
	if from < 0 || from >= len(p.slides) || to < 0 || to >= len(p.slides) {
		return ErrSlideIndex
	}

	if from == to {
		return nil
	}

	slide := p.slides[from]

	// Remove from original position
	p.slides = append(p.slides[:from], p.slides[from+1:]...)

	// Insert at new position
	p.slides = append(p.slides[:to], append([]*Slide{slide}, p.slides[to:]...)...)

	// Update indices
	for i := range p.slides {
		p.slides[i].index = i
	}

	return nil
}

// SlideMasters returns all slide masters in the presentation.
func (p *Presentation) SlideMasters() []*SlideMaster {
	return p.slideMasters
}

// SlideLayouts returns all slide layouts in the presentation.
func (p *Presentation) SlideLayouts() []*SlideLayout {
	return p.slideLayouts
}

// Theme is declared in theme.go, alongside the rest of the theme wiring.

// SlideWidth returns the width of slides in EMUs.
func (p *Presentation) SlideWidth() int64 {
	if p.presentation.SlideSize != nil {
		return p.presentation.SlideSize.Cx
	}
	return oxml.DefaultSlideSize().Cx
}

// SlideHeight returns the height of slides in EMUs.
func (p *Presentation) SlideHeight() int64 {
	if p.presentation.SlideSize != nil {
		return p.presentation.SlideSize.Cy
	}
	return oxml.DefaultSlideSize().Cy
}

// SetSlideSize sets the dimensions of slides.
func (p *Presentation) SetSlideSize(width, height int64) {
	p.presentation.SlideSize = &oxml.SlideSize{
		Cx: width,
		Cy: height,
	}
}

// newSlideXML creates a new empty slide XML structure.
func newSlideXML() *oxml.Slide {
	return &oxml.Slide{
		CSld: &oxml.CommonSlideData{
			SpTree: newShapeTree(),
		},
		ClrMapOvr: &oxml.ColorMapOverride{
			MasterClrMapping: &oxml.MasterColorMapping{},
		},
	}
}

// newShapeTree creates a new shape tree structure.
func newShapeTree() *oxml.ShapeTree {
	return &oxml.ShapeTree{
		NvGrpSpPr: &oxml.NvGrpSpPr{
			CNvPr:      &dml.CNvPr{Id: 1, Name: ""},
			CNvGrpSpPr: &dml.CNvGrpSpPr{},
			NvPr:       &oxml.NvPr{},
		},
		GrpSpPr: &oxml.GrpSpPr{
			Xfrm: &dml.GrpXfrm{
				Off:   &dml.OffXML{X: 0, Y: 0},
				Ext:   &dml.ExtXML{Cx: 0, Cy: 0},
				ChOff: &dml.OffXML{X: 0, Y: 0},
				ChExt: &dml.ExtXML{Cx: 0, Cy: 0},
			},
		},
	}
}

// sortedKeys returns the keys of a string-keyed map in sorted order, so parts
// written by iterating a map land in a deterministic order (Go randomizes map
// iteration per process, which would otherwise vary the package byte-for-byte
// between runs).
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
