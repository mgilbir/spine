// Package pptx provides functionality for reading and writing PowerPoint presentations.
package pptx

import (
	"encoding/xml"
	"errors"
	"fmt"
	"time"

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

	reader         *opc.ReadCloser
	presentation   *oxml.Presentation
	nextSlideID    uint32
	nextRelID      int
	preservedParts map[string]*preservedPart // Parts from original file for round-trip
	templatePath   string                    // Path to template file if using one
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
		reader:         reader,
		presentation:   &pres,
		nextSlideID:    256,
		nextRelID:      1,
		slideMasters:   make([]*SlideMaster, 0),
		slideLayouts:   make([]*SlideLayout, 0),
		preservedParts: make(map[string]*preservedPart),
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

	// Preserve all other parts for round-trip saving
	p.preserveAllParts()

	return p, nil
}

// preservedPart stores a part from the original file for round-trip saving.
type preservedPart struct {
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

// preserveAllParts saves all parts from the original file for round-trip saving.
func (p *Presentation) preserveAllParts() {
	if p.reader == nil {
		return
	}

	for _, file := range p.reader.Files {
		// Skip parts we'll regenerate
		if p.isRegeneratedPart(file.Name) {
			continue
		}

		data, err := file.ReadAll()
		if err != nil {
			continue
		}

		p.preservedParts[file.Name] = &preservedPart{
			contentType: file.ContentType,
			data:        data,
		}
	}

	// Also preserve relationships
	p.preserveRelationships()
}

// isRegeneratedPart returns true if this part will be regenerated during save.
func (p *Presentation) isRegeneratedPart(name string) bool {
	// These parts are regenerated from our in-memory model
	regenerated := []string{
		"/ppt/presentation.xml",
		"[Content_Types].xml",
		"_rels/.rels",
		"/docProps/core.xml",
	}

	for _, r := range regenerated {
		if name == r {
			return true
		}
	}

	// Slides, slide masters, and slide layouts are regenerated
	if len(name) > 12 && name[:12] == "/ppt/slides/" {
		return true
	}
	if len(name) > 18 && name[:18] == "/ppt/slideMasters/" {
		return true
	}
	if len(name) > 18 && name[:18] == "/ppt/slideLayouts/" {
		return true
	}

	return false
}

// preserveRelationships preserves relationship files from the original.
func (p *Presentation) preserveRelationships() {
	if p.reader == nil {
		return
	}

	// Store all relationship data
	for _, file := range p.reader.Files {
		if len(file.Name) > 5 && file.Name[len(file.Name)-5:] == ".rels" {
			data, err := file.ReadAll()
			if err != nil {
				continue
			}
			p.preservedParts[file.Name] = &preservedPart{
				contentType: opc.ContentTypeRelationships,
				data:        data,
			}
		}
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
	// Set properties
	p.Properties.Modified = time.Now()
	writer.Properties = &p.Properties

	// Create presentation part
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

	// Write slide masters and their layouts
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

		// Write master relationships (include theme if present)
		if master.theme != nil && master.theme.partName != "" {
			themeRelTarget := partNameToRelTarget(master.theme.partName, "/ppt/slideMasters/")
			masterRels = append(masterRels, &opc.Relationship{
				ID:         "rId" + fmt.Sprintf("%d", len(masterRels)+1),
				Type:       "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme",
				Target:     themeRelTarget,
				TargetMode: opc.TargetModeInternal,
			})
		}

		if err := writer.WritePartRelationships(masterPartName, masterRels); err != nil {
			return err
		}
	}

	// Create slide parts and relationships
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

	// Write presentation relationships
	if err := writer.WritePartRelationships("/ppt/presentation.xml", presRels); err != nil {
		return err
	}

	// Write preserved parts from original file (for round-trip support)
	if err := p.writePreservedParts(writer); err != nil {
		return err
	}

	return nil
}

// writePreservedParts writes parts from the original file that weren't regenerated.
func (p *Presentation) writePreservedParts(writer *opc.Writer) error {
	for partName, part := range p.preservedParts {
		// Skip relationship files - they're handled separately
		if len(partName) > 5 && partName[len(partName)-5:] == ".rels" {
			continue
		}

		if err := writer.WritePart(partName, part.contentType, part.data); err != nil {
			return err
		}
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
	// Set namespace attributes
	p.presentation.XmlnsA = oxml.NsDrawingML
	p.presentation.XmlnsR = oxml.NsRelationships
	p.presentation.XmlnsP = oxml.NsPresentationML

	// Update slide master IDs
	if len(p.slideMasters) > 0 {
		p.presentation.SlideMasterIDs = &oxml.SlideMasterIDs{
			SlideMasterID: make([]oxml.SlideMasterID, len(p.slideMasters)),
		}
		for i, master := range p.slideMasters {
			p.presentation.SlideMasterIDs.SlideMasterID[i] = oxml.SlideMasterID{
				ID:  uint32(2147483648 + i), // High IDs as per spec
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

	output, err := xml.MarshalIndent(p.presentation, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), output...), nil
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
		NvGrpSpPr: &oxml.NonVisualGroupShapeProperties{
			CNvPr: &oxml.NonVisualDrawingProperties{
				ID:   1,
				Name: "",
			},
			CNvGrpSpPr: &oxml.NonVisualGroupShapeDrawingProperties{},
			NvPr:       &oxml.ApplicationNonVisualDrawingProperties{},
		},
		GrpSpPr: &oxml.GroupShapeProperties{
			Xfrm: &oxml.GroupTransform2D{
				Off:   &oxml.Offset2D{X: 0, Y: 0},
				Ext:   &oxml.Extent2D{Cx: 0, Cy: 0},
				ChOff: &oxml.Offset2D{X: 0, Y: 0},
				ChExt: &oxml.Extent2D{Cx: 0, Cy: 0},
			},
		},
	}
}
