// Package pptx provides functionality for reading and writing PowerPoint presentations.
package pptx

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Common errors
var (
	ErrNotPPTX     = errors.New("pptx: not a valid PowerPoint file")
	ErrNoSlides    = errors.New("pptx: presentation has no slides")
	ErrSlideIndex  = errors.New("pptx: slide index out of range")
	ErrInvalidSlide = errors.New("pptx: invalid slide")
)

// Presentation represents a PowerPoint presentation.
type Presentation struct {
	// Properties contains the document properties.
	Properties opc.CoreProperties

	slides       []*Slide
	slideMasters []*SlideMaster
	slideLayouts []*SlideLayout
	theme        *Theme

	reader       *opc.ReadCloser
	presentation *oxml.Presentation
	nextSlideID  uint32
	nextRelID    int
	templatePath string // Path to template file if using one

	// Raw data for parts we serialize but don't fully parse
	presPropsData   []byte                   // /ppt/presProps.xml
	viewPropsData   []byte                   // /ppt/viewProps.xml
	tableStylesData []byte                   // /ppt/tableStyles.xml
	themeData       map[string][]byte        // /ppt/theme/*.xml (keyed by part name)
	thumbnailData   []byte                   // /docProps/thumbnail.jpeg
	appPropsData    []byte                   // /docProps/app.xml
	printerSettings map[string][]byte        // /ppt/printerSettings/*.bin
	otherParts      map[string]*rawPart      // Any other parts (media, custom XML, etc.)
	relationships   map[string][]*opc.Relationship // Relationships for each part
}

// Open opens a PowerPoint presentation from a file path.
func Open(path string) (*Presentation, error) {
	reader, err := opc.OpenReader(path)
	if err != nil {
		return nil, err
	}

	return openFromReader(reader)
}

// openFromReader creates a Presentation from an OPC reader.
func openFromReader(reader *opc.ReadCloser) (*Presentation, error) {
	// Find the main presentation part
	rels := reader.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 {
		reader.Close()
		return nil, ErrNotPPTX
	}

	mainPartName := opc.ResolvePartName("/", rels[0].Target)
	mainPart := reader.GetFile(mainPartName)
	if mainPart == nil {
		reader.Close()
		return nil, ErrNotPPTX
	}

	// Verify content type
	if mainPart.ContentType != opc.ContentTypePresentationMain {
		reader.Close()
		return nil, ErrNotPPTX
	}

	// Parse presentation XML
	data, err := mainPart.ReadAll()
	if err != nil {
		reader.Close()
		return nil, err
	}

	var pres oxml.Presentation
	if err := xml.Unmarshal(data, &pres); err != nil {
		reader.Close()
		return nil, err
	}

	p := &Presentation{
		reader:          reader,
		presentation:    &pres,
		nextSlideID:     256,
		nextRelID:       1,
		slideMasters:    make([]*SlideMaster, 0),
		slideLayouts:    make([]*SlideLayout, 0),
		themeData:       make(map[string][]byte),
		printerSettings: make(map[string][]byte),
		otherParts:      make(map[string]*rawPart),
		relationships:   make(map[string][]*opc.Relationship),
	}

	// Copy properties
	if reader.Properties != nil {
		p.Properties = *reader.Properties
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
		reader.Close()
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
		reader.Close()
		return nil, err
	}

	// Load slides
	if err := p.loadSlides(mainPartName); err != nil {
		reader.Close()
		return nil, err
	}

	// Load all remaining parts into the model
	if err := p.loadAllParts(mainPartName); err != nil {
		reader.Close()
		return nil, err
	}

	return p, nil
}

// rawPart stores a part that we don't fully parse but need to preserve.
type rawPart struct {
	contentType string
	data        []byte
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

	// Load all relationships
	p.loadAllRelationships()

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
			// Already loaded into p.slides
			continue
		case strings.HasPrefix(name, "/ppt/slideMasters/") && !strings.Contains(name, "_rels"):
			// Already loaded into p.slideMasters
			continue
		case strings.HasPrefix(name, "/ppt/slideLayouts/") && !strings.Contains(name, "_rels"):
			// Already loaded into p.slideLayouts
			continue
		case name == "/docProps/core.xml":
			// Already loaded into p.Properties
			continue
		case name == "/docProps/app.xml":
			p.appPropsData = data
		default:
			// Store as other part
			p.otherParts[name] = &rawPart{
				contentType: file.ContentType,
				data:        data,
			}
		}
	}

	return nil
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
		sourcePart := relsPathToSourcePart(file.Name)
		p.relationships[sourcePart] = rels
	}
}

// relsPathToSourcePart converts a .rels path to its source part path.
// e.g., "/ppt/_rels/presentation.xml.rels" -> "/ppt/presentation.xml"
func relsPathToSourcePart(relsPath string) string {
	// Remove trailing ".rels"
	if !strings.HasSuffix(relsPath, ".rels") {
		return relsPath
	}
	path := relsPath[:len(relsPath)-5]

	// Remove "/_rels" directory component
	path = strings.Replace(path, "/_rels/", "/", 1)
	path = strings.Replace(path, "_rels/", "", 1)

	return path
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
	for i, slideRef := range p.presentation.SlideIDs.SlideID {
		rel, ok := relMap[slideRef.RID]
		if !ok {
			continue
		}

		slideName := opc.ResolvePartName(mainPartName, rel.Target)
		slideFile := p.reader.GetFile(slideName)
		if slideFile == nil {
			continue
		}

		data, err := slideFile.ReadAll()
		if err != nil {
			continue
		}

		var slideXML oxml.Slide
		if err := xml.Unmarshal(data, &slideXML); err != nil {
			continue
		}

		slide := &Slide{
			presentation: p,
			partName:     slideName,
			slideXML:     &slideXML,
			index:        i,
			id:           slideRef.ID,
			relID:        slideRef.RID,
		}

		p.slides = append(p.slides, slide)
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
			continue
		}

		var masterXML oxml.SlideMaster
		if err := xml.Unmarshal(data, &masterXML); err != nil {
			continue
		}

		master := &SlideMaster{
			presentation: p,
			partName:     masterName,
			masterXML:    &masterXML,
			relID:        masterRef.RID,
			numericID:    masterRef.ID,
			layouts:      make([]*SlideLayout, 0),
		}

		// Load layouts for this master
		if err := p.loadSlideLayouts(master, masterName); err != nil {
			continue
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
				continue
			}

			var layoutXML oxml.SlideLayout
			if err := xml.Unmarshal(data, &layoutXML); err != nil {
				continue
			}

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
func CreateWithOptions(opts CreateOptions) *Presentation {
	now := time.Now()
	p := &Presentation{
		Properties: opc.CoreProperties{
			Created:  now,
			Modified: now,
		},
		slides:       make([]*Slide, 0),
		slideMasters: make([]*SlideMaster, 0),
		slideLayouts: make([]*SlideLayout, 0),
		nextSlideID:  256,
		nextRelID:    1,
		presentation: &oxml.Presentation{
			SlideSize: oxml.DefaultSlideSize(),
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

// initializeDefaultMasterAndLayouts creates the default slide master and layouts.
func (p *Presentation) initializeDefaultMasterAndLayouts() {
	// Create default master
	master := createDefaultMaster()
	master.presentation = p
	master.partName = "/ppt/slideMasters/slideMaster1.xml"
	master.relID = fmt.Sprintf("rId%d", p.nextRelID)
	p.nextRelID++

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
		layout := createDefaultLayout(lt, master)
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
	p := Create()
	p.presentation.SlideSize = oxml.WidescreenSlideSize()
	return p
}

// Save saves the presentation to a file.
func (p *Presentation) Save(path string) error {
	writer, err := opc.Create(path)
	if err != nil {
		return err
	}
	defer writer.Close()

	return p.saveTo(writer)
}

// SaveAs saves the presentation to a new file path.
// This is useful for the "open template, modify, save as new file" workflow.
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

	// Clear all slides but keep masters, layouts, and themes
	p.slides = make([]*Slide, 0)
	p.presentation.SlideIDs = nil

	// Reset slide ID counter but keep relationship IDs
	p.nextSlideID = 256

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

// GetLayoutByType returns the first slide layout matching the specified type.
func (p *Presentation) GetLayoutByType(layoutType SlideLayoutType) *SlideLayout {
	for _, layout := range p.slideLayouts {
		if layout.Type() == layoutType {
			return layout
		}
	}
	return nil
}

// GetLayoutByName returns the slide layout with the specified name.
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

// saveTo writes the presentation to an OPC writer.
func (p *Presentation) saveTo(writer *opc.Writer) error {
	// For round-trip, use preserved data when available
	if p.reader != nil {
		return p.saveRoundTrip(writer)
	}
	return p.saveNew(writer)
}

// saveRoundTrip saves a presentation by serializing all parts from the model.
func (p *Presentation) saveRoundTrip(writer *opc.Writer) error {
	// Set properties
	writer.Properties = &p.Properties

	// Use original content types to preserve ordering and avoid extra entries
	if p.reader != nil && p.reader.ContentTypes != nil {
		writer.ContentTypes = p.reader.ContentTypes
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

	// Write slides using original part names
	for i, slide := range p.slides {
		slideData, err := slide.marshal()
		if err != nil {
			return fmt.Errorf("marshal slide %d: %w", i, err)
		}
		slideName := slide.partName
		if slideName == "" {
			slideName = fmt.Sprintf("/ppt/slides/slide%d.xml", i+1)
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

	// Write all themes
	for themeName, themeBytes := range p.themeData {
		if err := writer.WritePart(themeName, opc.ContentTypeTheme, themeBytes); err != nil {
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

	// Write printer settings
	for name, data := range p.printerSettings {
		if err := writer.WritePart(name, "application/vnd.openxmlformats-officedocument.presentationml.printerSettings", data); err != nil {
			return err
		}
	}

	// Write other parts
	for name, part := range p.otherParts {
		if err := writer.WritePart(name, part.contentType, part.data); err != nil {
			return err
		}
	}

	// Write relationships for all parts that have rels but haven't been written explicitly
	writtenRels["/ppt/presentation.xml"] = true // will be written below
	for partName, rels := range p.relationships {
		if writtenRels[partName] || len(rels) == 0 {
			continue
		}
		if err := p.writePartRelationships(writer, partName); err != nil {
			return err
		}
	}

	// Write presentation.xml (regenerated to reflect current slide list)
	presData, err := p.marshalPresentation()
	if err != nil {
		return err
	}
	if err := writer.WritePart("/ppt/presentation.xml", opc.ContentTypePresentationMain, presData); err != nil {
		return err
	}

	// Write presentation relationships
	if err := p.writePresentationRelationships(writer); err != nil {
		return err
	}

	// Add main relationship
	writer.AddRelationship(opc.RelTypeOfficeDocument, "ppt/presentation.xml", opc.TargetModeInternal)

	return nil
}

// writePartRelationships writes the relationships file for a given part.
func (p *Presentation) writePartRelationships(writer *opc.Writer, partName string) error {
	rels, ok := p.relationships[partName]
	if !ok || len(rels) == 0 {
		return nil
	}

	data, err := opc.MarshalRelationships(rels)
	if err != nil {
		return err
	}

	relsPath := partNameToRelsPath(partName)
	return writer.WritePart(relsPath, opc.ContentTypeRelationships, data)
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
	var rels []*opc.Relationship

	if existingRels, ok := p.relationships["/ppt/presentation.xml"]; ok && len(existingRels) > 0 {
		// Start with existing relationships (preserves original order and IDs)
		rels = append(rels, existingRels...)

		// Build set of existing relationship IDs
		existingIDs := make(map[string]bool)
		for _, rel := range existingRels {
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

	data, err := opc.MarshalRelationships(rels)
	if err != nil {
		return err
	}

	return writer.WritePart("/ppt/_rels/presentation.xml.rels", opc.ContentTypeRelationships, data)
}

// saveNew saves a newly created presentation.
func (p *Presentation) saveNew(writer *opc.Writer) error {
	// Set properties
	p.Properties.Modified = time.Now()
	writer.Properties = &p.Properties

	// Set extended properties
	writer.ExtendedProperties = &opc.ExtendedProperties{
		Slides:             len(p.slides),
		PresentationFormat: "Widescreen",
	}

	// Assign fresh relationship IDs for all parts
	// PowerPoint expects: slideMaster first, then slides, then other parts (theme, presProps, etc.)
	presRelID := 1

	// Assign relationship IDs to masters FIRST
	for _, master := range p.slideMasters {
		master.relID = fmt.Sprintf("rId%d", presRelID)
		presRelID++
	}

	// Assign relationship IDs to slides SECOND
	for _, slide := range p.slides {
		slide.relID = fmt.Sprintf("rId%d", presRelID)
		presRelID++
	}

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

	if err := writer.WritePart("/ppt/presentation.xml", opc.ContentTypePresentationMain, presData); err != nil {
		return err
	}

	// Add main relationship
	writer.AddRelationship(opc.RelTypeOfficeDocument, "ppt/presentation.xml", opc.TargetModeInternal)

	// Collect presentation relationships
	presRels := make([]*opc.Relationship, 0)

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

		// Write layouts for this master
		masterRels := make([]*opc.Relationship, 0)
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

			// Relationship from master to layout
			masterRels = append(masterRels, &opc.Relationship{
				ID:         layout.relID,
				Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout",
				Target:     layoutRelTarget,
				TargetMode: opc.TargetModeInternal,
			})

			// Write layout relationships (back to master)
			layoutRels := []*opc.Relationship{
				{
					ID:         "rId1",
					Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster",
					Target:     partNameToRelTarget(masterPartName, "/ppt/slideLayouts/"),
					TargetMode: opc.TargetModeInternal,
				},
			}
			if err := writer.WritePartRelationships(layoutPartName, layoutRels); err != nil {
				return err
			}
		}

		// Write master relationships (always include theme)
		masterRels = append(masterRels, &opc.Relationship{
			ID:         fmt.Sprintf("rId%d", len(masterRels)+1),
			Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme",
			Target:     "../theme/theme1.xml",
			TargetMode: opc.TargetModeInternal,
		})

		if err := writer.WritePartRelationships(masterPartName, masterRels); err != nil {
			return err
		}
	}

	// Create slide parts and relationships SECOND
	for i, slide := range p.slides {
		slidePartName := fmt.Sprintf("/ppt/slides/slide%d.xml", i+1)
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
			Target:     fmt.Sprintf("slides/slide%d.xml", i+1),
			TargetMode: opc.TargetModeInternal,
		})

		// Write slide relationships (to layout if set)
		slideRels := make([]*opc.Relationship, 0)
		if slide.layout != nil {
			// Find layout index
			for j, layout := range p.slideLayouts {
				if layout == slide.layout {
					slideRels = append(slideRels, &opc.Relationship{
						ID:         "rId1",
						Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout",
						Target:     fmt.Sprintf("../slideLayouts/slideLayout%d.xml", j+1),
						TargetMode: opc.TargetModeInternal,
					})
					break
				}
			}
		} else if len(p.slideLayouts) > 0 {
			// Default to first layout (Title layout)
			slideRels = append(slideRels, &opc.Relationship{
				ID:         "rId1",
				Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout",
				Target:     "../slideLayouts/slideLayout1.xml",
				TargetMode: opc.TargetModeInternal,
			})
		}

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

// marshalPresentation generates the presentation.xml content.
func (p *Presentation) marshalPresentation() ([]byte, error) {
	// Update slide master IDs
	if len(p.slideMasters) > 0 {
		p.presentation.SlideMasterIDs = &oxml.SlideMasterIDs{
			SlideMasterID: make([]oxml.SlideMasterID, len(p.slideMasters)),
		}
		for i, master := range p.slideMasters {
			id := master.numericID
			if id == 0 {
				id = uint32(2147483648 + i) // High IDs as per spec
			}
			p.presentation.SlideMasterIDs.SlideMasterID[i] = oxml.SlideMasterID{
				ID:  id,
				RID: master.relID,
			}
		}
	}

	// Update slide IDs
	p.presentation.SlideIDs = &oxml.SlideIDs{
		SlideID: make([]oxml.SlideID, len(p.slides)),
	}
	for i, slide := range p.slides {
		p.presentation.SlideIDs.SlideID[i] = oxml.SlideID{
			ID:  slide.id,
			RID: slide.relID,
		}
	}

	// Use the namespace-aware marshaler for PowerPoint compatibility
	return marshalPresentationXML(p.presentation), nil
}

// Close closes the presentation and releases resources.
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

// AddSlide adds a new blank slide to the presentation.
func (p *Presentation) AddSlide() *Slide {
	relID := fmt.Sprintf("rId%d", p.nextRelID)
	p.nextRelID++

	slide := &Slide{
		presentation: p,
		index:        len(p.slides),
		id:           p.nextSlideID,
		relID:        relID,
		slideXML:     newSlideXML(),
	}
	p.nextSlideID++

	p.slides = append(p.slides, slide)
	return slide
}

// AddSlideWithLayout adds a new slide using the specified layout.
func (p *Presentation) AddSlideWithLayout(layout *SlideLayout) *Slide {
	slide := p.AddSlide()
	slide.layout = layout
	return slide
}

// RemoveSlide removes the slide at the specified index.
func (p *Presentation) RemoveSlide(index int) error {
	if index < 0 || index >= len(p.slides) {
		return ErrSlideIndex
	}

	// Remove the slide
	p.slides = append(p.slides[:index], p.slides[index+1:]...)

	// Update indices
	for i := index; i < len(p.slides); i++ {
		p.slides[i].index = i
	}

	return nil
}

// MoveSlide moves a slide from one position to another.
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

// Theme returns the presentation theme.
func (p *Presentation) Theme() *Theme {
	return p.theme
}

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
