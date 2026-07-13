package pptx

import (
	"fmt"
	"path"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// SlideMaster represents a slide master.
type SlideMaster struct {
	presentation *Presentation
	partName     string
	masterXML    *oxml.SlideMaster
	theme        *Theme
	layouts      []*SlideLayout
	relID        string
	numericID    uint32 // original numeric ID from presentation.xml
	// idExtLst preserves the extLst child of this master's p:sldMasterId
	// entry in presentation.xml, which is regenerated on every save (C225).
	idExtLst        *oxml.ExtensionList
	layoutsModified bool // true if layouts changed via Go API
}

// Name returns the name of the slide master.
func (sm *SlideMaster) Name() string {
	if sm.masterXML != nil && sm.masterXML.CSld != nil {
		return sm.masterXML.CSld.Name
	}
	return ""
}

// SetName sets the name of the slide master.
func (sm *SlideMaster) SetName(name string) {
	if sm.masterXML == nil {
		sm.masterXML = &oxml.SlideMaster{}
	}
	if sm.masterXML.CSld == nil {
		sm.masterXML.CSld = &oxml.CommonSlideData{}
	}
	sm.masterXML.CSld.Name = name
}

// Layouts returns all slide layouts based on this master.
func (sm *SlideMaster) Layouts() []*SlideLayout {
	return sm.layouts
}

// GetLayout returns the layout with the specified type.
func (sm *SlideMaster) GetLayout(layoutType SlideLayoutType) *SlideLayout {
	for _, layout := range sm.layouts {
		if layout.Type() == layoutType {
			return layout
		}
	}
	return nil
}

// GetLayoutByName returns the layout with the specified name.
func (sm *SlideMaster) GetLayoutByName(name string) *SlideLayout {
	for _, layout := range sm.layouts {
		if layout.Name() == name {
			return layout
		}
	}
	return nil
}

// Theme returns the theme associated with this master, parsed from its theme
// part when the presentation was opened. It is a read-only view: the theme
// part is preserved verbatim on save, so edits made through the returned
// value are not written back. It returns nil for masters created
// programmatically or whose theme part could not be parsed.
func (sm *SlideMaster) Theme() *Theme {
	return sm.theme
}

// ColorMap returns the color mapping for this master.
func (sm *SlideMaster) ColorMap() *ColorMap {
	if sm.masterXML == nil || sm.masterXML.ClrMap == nil {
		return nil
	}
	return &ColorMap{
		Background1:       sm.masterXML.ClrMap.Bg1,
		Text1:             sm.masterXML.ClrMap.Tx1,
		Background2:       sm.masterXML.ClrMap.Bg2,
		Text2:             sm.masterXML.ClrMap.Tx2,
		Accent1:           sm.masterXML.ClrMap.Accent1,
		Accent2:           sm.masterXML.ClrMap.Accent2,
		Accent3:           sm.masterXML.ClrMap.Accent3,
		Accent4:           sm.masterXML.ClrMap.Accent4,
		Accent5:           sm.masterXML.ClrMap.Accent5,
		Accent6:           sm.masterXML.ClrMap.Accent6,
		Hyperlink:         sm.masterXML.ClrMap.Hlink,
		FollowedHyperlink: sm.masterXML.ClrMap.FolHlink,
	}
}

// Placeholders returns the placeholder shapes defined in this master's shape
// tree, materialized read-only from the parsed XML: mutating the returned
// shapes does not modify the master part, which is written from its parsed
// tree on save.
func (sm *SlideMaster) Placeholders() []*PlaceholderShape {
	if sm.masterXML == nil || sm.masterXML.CSld == nil {
		return nil
	}
	return placeholdersFromSpTree(sm.masterXML.CSld.SpTree)
}

// GetPlaceholder returns the master placeholder with the specified type, or nil.
func (sm *SlideMaster) GetPlaceholder(phType PlaceholderType) *PlaceholderShape {
	for _, ph := range sm.Placeholders() {
		if ph.PlaceholderType() == phType {
			return ph
		}
	}
	return nil
}

// placeholdersFromSpTree materializes the placeholder shapes (p:sp children
// carrying a p:ph) of a master or layout shape tree.
func placeholdersFromSpTree(spTree *oxml.ShapeTree) []*PlaceholderShape {
	if spTree == nil {
		return nil
	}
	var placeholders []*PlaceholderShape
	for _, sp := range spTree.Sp {
		if sp == nil || sp.NvSpPr == nil || sp.NvSpPr.NvPr == nil || sp.NvSpPr.NvPr.Ph == nil {
			continue
		}
		placeholders = append(placeholders, oxmlShapeToPlaceholder(sp))
	}
	return placeholders
}

// ColorMap represents the mapping of semantic colors to theme colors.
type ColorMap struct {
	Background1       string
	Text1             string
	Background2       string
	Text2             string
	Accent1           string
	Accent2           string
	Accent3           string
	Accent4           string
	Accent5           string
	Accent6           string
	Hyperlink         string
	FollowedHyperlink string
}

// MasterTextStyle represents text styling at different levels.
type MasterTextStyle struct {
	levels [9]*TextLevelStyle
}

// TextLevelStyle represents text styling for a specific indentation level.
type TextLevelStyle struct {
	FontName   string
	FontSize   float64
	Bold       bool
	Italic     bool
	Color      dml.Color
	Alignment  string
	MarginLeft dml.EMU
	Indent     dml.EMU
	BulletType BulletType
	BulletChar string
}

// Level returns the style for the specified level (0-8).
func (ts *MasterTextStyle) Level(level int) *TextLevelStyle {
	if level < 0 || level > 8 {
		return nil
	}
	return ts.levels[level]
}

// SetLevel sets the style for the specified level.
func (ts *MasterTextStyle) SetLevel(level int, style *TextLevelStyle) {
	if level >= 0 && level <= 8 {
		ts.levels[level] = style
	}
}

// marshal converts the slide master to XML bytes.
func (sm *SlideMaster) marshal() ([]byte, error) {
	if sm.masterXML == nil {
		sm.masterXML = newMasterXML()
	}

	// Update layout ID list only if layouts were modified or IDs are missing.
	// Preserve original layout IDs for round-trip fidelity.
	if sm.layoutsModified || sm.masterXML.SlideLayoutIDs == nil {
		if len(sm.layouts) > 0 {
			sm.masterXML.SlideLayoutIDs = &oxml.SlideLayoutIDs{
				SlideLayoutID: make([]oxml.SlideLayoutID, len(sm.layouts)),
			}
			for i, layout := range sm.layouts {
				sm.masterXML.SlideLayoutIDs.SlideLayoutID[i] = oxml.SlideLayoutID{
					ID:  uint32(2147483649 + i), // Starting from high number as per OOXML spec
					RID: layout.relID,
				}
			}
		}
	}

	// Use the namespace-aware marshaler for PowerPoint compatibility
	return marshalSlideMaster(sm.masterXML)
}

// newMasterXML creates a new slide master XML structure.
func newMasterXML() *oxml.SlideMaster {
	return &oxml.SlideMaster{
		Preserve: true,
		CSld: &oxml.CommonSlideData{
			SpTree: newShapeTree(),
		},
		ClrMap: &oxml.ColorMap{
			Bg1:      "lt1",
			Tx1:      "dk1",
			Bg2:      "lt2",
			Tx2:      "dk2",
			Accent1:  "accent1",
			Accent2:  "accent2",
			Accent3:  "accent3",
			Accent4:  "accent4",
			Accent5:  "accent5",
			Accent6:  "accent6",
			Hlink:    "hlink",
			FolHlink: "folHlink",
		},
		TxStyles: &oxml.TxStyles{
			TitleStyle: newDefaultTextListStyle(4400), // 44pt for titles
			BodyStyle:  newDefaultTextListStyle(1800), // 18pt for body
			OtherStyle: newDefaultTextListStyle(1800), // 18pt for other
		},
	}
}

// newDefaultTextListStyle creates a default text list style.
func newDefaultTextListStyle(baseFontSize int32) *dml.LstStyle {
	return &dml.LstStyle{
		Lvl1pPr: newDefaultLevelPPr(baseFontSize, 0),
		Lvl2pPr: newDefaultLevelPPr(baseFontSize-200, 457200),
		Lvl3pPr: newDefaultLevelPPr(baseFontSize-400, 914400),
		Lvl4pPr: newDefaultLevelPPr(baseFontSize-600, 1371600),
		Lvl5pPr: newDefaultLevelPPr(baseFontSize-800, 1828800),
	}
}

// newDefaultLevelPPr creates default paragraph properties for a level.
func newDefaultLevelPPr(fontSize int32, marginLeft int32) *dml.PPr {
	return &dml.PPr{
		MarL: &marginLeft,
		Algn: "l",
		DefRPr: &dml.RPr{
			Sz: fontSize,
		},
	}
}

// createDefaultMaster creates a slide master with default settings.
func createDefaultMaster() *SlideMaster {
	master := &SlideMaster{
		masterXML: newMasterXML(),
		layouts:   make([]*SlideLayout, 0),
	}

	// Add title placeholder to master
	titlePh := NewPlaceholderShape(PlaceholderTitle)
	titlePh.SetPosition(dml.Inches(0.5), dml.Inches(0.3))
	titlePh.SetSize(dml.Inches(12.33), dml.Inches(1.2))
	titlePh.SetIndex(0)

	// Add body placeholder to master
	bodyPh := NewPlaceholderShape(PlaceholderBody)
	bodyPh.SetPosition(dml.Inches(0.5), dml.Inches(1.6))
	bodyPh.SetSize(dml.Inches(12.33), dml.Inches(5.1))
	bodyPh.SetIndex(1)

	// Add placeholders to master shape tree
	if master.masterXML.CSld.SpTree != nil {
		titleSp := placeholderToOxml(titlePh, 2)
		bodySp := placeholderToOxml(bodyPh, 3)
		master.masterXML.CSld.SpTree.Sp = append(master.masterXML.CSld.SpTree.Sp, titleSp, bodySp)
	}

	return master
}

// AddLayout adds a slide layout to this master. It assigns the layout a
// relationship id (unique within the master) and a part name, and registers it
// with the presentation, so the saved package references it correctly. Without
// this the output contains <p:sldLayoutId r:id=""/> and an empty
// <Relationship Id=""/> — a corrupt package.
func (sm *SlideMaster) AddLayout(layoutType SlideLayoutType) *SlideLayout {
	layout := createDefaultLayout(layoutType, sm)
	layout.presentation = sm.presentation
	layout.relID = fmt.Sprintf("rId%d", sm.nextLayoutRelIDNum())

	sm.layouts = append(sm.layouts, layout)
	if sm.presentation != nil {
		layout.partName = sm.presentation.nextAvailableLayoutPartName()
		sm.presentation.slideLayouts = append(sm.presentation.slideLayouts, layout)
		sm.registerLayoutRelationships(layout)
	}
	sm.layoutsModified = true
	return layout
}

// nextLayoutRelIDNum returns the next free relationship id number within the
// master, scanning the sibling layouts' relIDs and the master's loaded
// relationships. The latter matters on opened decks: the master's rels already
// hold a theme rel, so scanning only the layouts handed the new layout the
// theme's rId — the <p:sldLayoutId r:id> resolved to theme1.xml and the layout
// was silently lost on reopen.
func (sm *SlideMaster) nextLayoutRelIDNum() int {
	maxRel := 0
	for _, l := range sm.layouts {
		var id int
		if _, err := fmt.Sscanf(l.relID, "rId%d", &id); err == nil && id > maxRel {
			maxRel = id
		}
	}
	if sm.presentation != nil && sm.partName != "" {
		if id := nextRelationshipID(sm.presentation.relationships[sm.partName]) - 1; id > maxRel {
			maxRel = id
		}
	}
	return maxRel + 1
}

// registerLayoutRelationships records the relationships a newly added layout
// needs in the presentation's relationship map, which is what the round-trip
// save path writes verbatim: the master -> layout relationship and the
// layout's own relationship back to its master. Created decks are handled by
// saveNew, which rebuilds master and layout rels from the layout list, so
// masters without a part name (never loaded from a file) need no entries here.
func (sm *SlideMaster) registerLayoutRelationships(layout *SlideLayout) {
	if sm.partName == "" || layout.partName == "" {
		return
	}
	p := sm.presentation
	p.relationships[sm.partName] = append(p.relationships[sm.partName], &opc.Relationship{
		ID:         layout.relID,
		Type:       opc.RelTypeSlideLayout,
		Target:     partNameToRelTarget(layout.partName, path.Dir(sm.partName)+"/"),
		TargetMode: opc.TargetModeInternal,
	})
	layoutRels := p.relationships[layout.partName]
	p.relationships[layout.partName] = append(layoutRels, &opc.Relationship{
		ID:         fmt.Sprintf("rId%d", nextRelationshipID(layoutRels)),
		Type:       opc.RelTypeSlideMaster,
		Target:     partNameToRelTarget(sm.partName, path.Dir(layout.partName)+"/"),
		TargetMode: opc.TargetModeInternal,
	})
}
