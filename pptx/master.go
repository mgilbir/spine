package pptx

import (
	"fmt"
	"math"
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
	idOmitted    bool   // the source entry carried no id attribute
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

// EditablePlaceholders returns mutable handles to every placeholder in the
// master's shape tree, in document order. Their geometry setters write back to
// the master part on save.
func (sm *SlideMaster) EditablePlaceholders() []*EditablePlaceholder {
	if sm.masterXML == nil || sm.masterXML.CSld == nil {
		return nil
	}
	return editablePlaceholdersFromSpTree(sm.masterXML.CSld.SpTree)
}

// EditablePlaceholder returns a mutable handle to the first master placeholder
// of the given type, or nil when none matches.
func (sm *SlideMaster) EditablePlaceholder(phType PlaceholderType) *EditablePlaceholder {
	for _, ep := range sm.EditablePlaceholders() {
		if ep.Type() == phType {
			return ep
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

// TitleStyle returns an editor for the master's title text style
// (p:txStyles/p:titleStyle). Edits mutate the underlying a:lvlNpPr in place, so
// the master's other styles and any unmodeled properties round-trip unchanged.
func (sm *SlideMaster) TitleStyle() *MasterTextStyle {
	return &MasterTextStyle{sm: sm, kind: titleTextStyle}
}

// BodyStyle returns an editor for the master's body text style
// (p:txStyles/p:bodyStyle). See TitleStyle.
func (sm *SlideMaster) BodyStyle() *MasterTextStyle {
	return &MasterTextStyle{sm: sm, kind: bodyTextStyle}
}

// OtherStyle returns an editor for the master's other text style
// (p:txStyles/p:otherStyle). See TitleStyle.
func (sm *SlideMaster) OtherStyle() *MasterTextStyle {
	return &MasterTextStyle{sm: sm, kind: otherTextStyle}
}

// textStyleKind selects one of a master's three p:txStyles trees.
type textStyleKind int

const (
	titleTextStyle textStyleKind = iota
	bodyTextStyle
	otherTextStyle
)

// MasterTextStyle edits one of a slide master's text-style trees
// (title/body/other). Each tree carries up to nine indentation levels
// (a:lvl1pPr…a:lvl9pPr); the level accessors read and mutate them in place so
// an edit to one property leaves the rest of the level, and every other level,
// byte-identical on save.
type MasterTextStyle struct {
	sm   *SlideMaster
	kind textStyleKind
}

// TextLevelStyle is a read snapshot of the styling for one indentation level.
type TextLevelStyle struct {
	FontName   string
	FontSize   float64 // points; 0 when the level sets no size
	Bold       bool
	Italic     bool
	Color      dml.Color
	HasColor   bool
	Alignment  string
	MarginLeft dml.EMU
	Indent     dml.EMU
	BulletType BulletType
	BulletChar string
}

// lst returns the underlying list style for this tree, or nil when the master
// has no p:txStyles or no entry for this kind.
func (ts *MasterTextStyle) lst() *dml.LstStyle {
	if ts.sm.masterXML == nil || ts.sm.masterXML.TxStyles == nil {
		return nil
	}
	switch ts.kind {
	case titleTextStyle:
		return ts.sm.masterXML.TxStyles.TitleStyle
	case bodyTextStyle:
		return ts.sm.masterXML.TxStyles.BodyStyle
	default:
		return ts.sm.masterXML.TxStyles.OtherStyle
	}
}

// ensureLst returns the underlying list style, allocating the master XML, its
// p:txStyles, and the tree entry as needed.
func (ts *MasterTextStyle) ensureLst() *dml.LstStyle {
	sm := ts.sm
	if sm.masterXML == nil {
		sm.masterXML = &oxml.SlideMaster{}
	}
	if sm.masterXML.TxStyles == nil {
		sm.masterXML.TxStyles = &oxml.TxStyles{}
	}
	txs := sm.masterXML.TxStyles
	switch ts.kind {
	case titleTextStyle:
		if txs.TitleStyle == nil {
			txs.TitleStyle = &dml.LstStyle{}
		}
		return txs.TitleStyle
	case bodyTextStyle:
		if txs.BodyStyle == nil {
			txs.BodyStyle = &dml.LstStyle{}
		}
		return txs.BodyStyle
	default:
		if txs.OtherStyle == nil {
			txs.OtherStyle = &dml.LstStyle{}
		}
		return txs.OtherStyle
	}
}

// lstLevelField returns the address of the a:lvlNpPr field for the given
// 0-based level (0 -> lvl1pPr … 8 -> lvl9pPr), or nil when out of range.
func lstLevelField(ls *dml.LstStyle, level int) **dml.PPr {
	switch level {
	case 0:
		return &ls.Lvl1pPr
	case 1:
		return &ls.Lvl2pPr
	case 2:
		return &ls.Lvl3pPr
	case 3:
		return &ls.Lvl4pPr
	case 4:
		return &ls.Lvl5pPr
	case 5:
		return &ls.Lvl6pPr
	case 6:
		return &ls.Lvl7pPr
	case 7:
		return &ls.Lvl8pPr
	case 8:
		return &ls.Lvl9pPr
	}
	return nil
}

// Level returns a read snapshot of the style at the given level (0-8), or nil
// when the level is out of range or the tree defines no properties for it.
func (ts *MasterTextStyle) Level(level int) *TextLevelStyle {
	ls := ts.lst()
	if ls == nil {
		return nil
	}
	fp := lstLevelField(ls, level)
	if fp == nil || *fp == nil {
		return nil
	}
	return levelStyleFromPPr(*fp)
}

// levelStyleFromPPr reads the modeled properties of a level paragraph.
func levelStyleFromPPr(pp *dml.PPr) *TextLevelStyle {
	s := &TextLevelStyle{Alignment: pp.Algn}
	if pp.MarL != nil {
		s.MarginLeft = dml.EMU(*pp.MarL)
	}
	if pp.Indent != nil {
		s.Indent = dml.EMU(*pp.Indent)
	}
	if rpr := pp.DefRPr; rpr != nil {
		if rpr.Sz != 0 {
			s.FontSize = float64(rpr.Sz) / 100
		}
		if rpr.B != nil {
			s.Bold = *rpr.B
		}
		if rpr.I != nil {
			s.Italic = *rpr.I
		}
		if rpr.Latin != nil {
			s.FontName = rpr.Latin.Typeface
		}
		if c := oxmlToColor(rpr.SolidFill); c != nil {
			s.Color = *c
			s.HasColor = true
		}
	}
	switch {
	case pp.BuNone != nil:
		s.BulletType = BulletNone
	case pp.BuAutoNum != nil:
		s.BulletType = BulletAuto
	case pp.BuChar != nil:
		s.BulletType = BulletChar
		s.BulletChar = pp.BuChar.Char
	default:
		s.BulletType = BulletInherit
	}
	return s
}

// ensureLevel returns the level paragraph, allocating the tree and the
// a:lvlNpPr as needed. It returns nil for an out-of-range level. A level absent
// from the source is inserted at its schema position in the parsed child order
// (dml.LstStyle.EnsureLevel), so adding a brand-new level round-trips in lvlN
// order rather than after a later sibling or a captured a:extLst.
func (ts *MasterTextStyle) ensureLevel(level int) *dml.PPr {
	if level < 0 || level > 8 {
		return nil
	}
	return ts.ensureLst().EnsureLevel(level)
}

// ensureDefRPr returns the level's default run properties, allocating as needed.
func ensureDefRPr(pp *dml.PPr) *dml.RPr {
	if pp.DefRPr == nil {
		pp.DefRPr = &dml.RPr{}
	}
	return pp.DefRPr
}

// SetLevelFont sets the Latin typeface for the given level (0-8).
func (ts *MasterTextStyle) SetLevelFont(level int, name string) {
	pp := ts.ensureLevel(level)
	if pp == nil {
		return
	}
	rpr := ensureDefRPr(pp)
	if rpr.Latin == nil {
		rpr.Latin = &dml.TextFont{}
	}
	rpr.Latin.Typeface = name
}

// SetLevelFontSize sets the font size in points for the given level (0-8).
func (ts *MasterTextStyle) SetLevelFontSize(level int, points float64) {
	pp := ts.ensureLevel(level)
	if pp == nil {
		return
	}
	ensureDefRPr(pp).Sz = int32(math.Round(points * 100))
}

// SetLevelBold sets whether the given level (0-8) is bold.
func (ts *MasterTextStyle) SetLevelBold(level int, bold bool) {
	pp := ts.ensureLevel(level)
	if pp == nil {
		return
	}
	v := bold
	ensureDefRPr(pp).B = &v
}

// SetLevelItalic sets whether the given level (0-8) is italic.
func (ts *MasterTextStyle) SetLevelItalic(level int, italic bool) {
	pp := ts.ensureLevel(level)
	if pp == nil {
		return
	}
	v := italic
	ensureDefRPr(pp).I = &v
}

// SetLevelColor sets the solid text color for the given level (0-8).
func (ts *MasterTextStyle) SetLevelColor(level int, c dml.Color) {
	pp := ts.ensureLevel(level)
	if pp == nil {
		return
	}
	ensureDefRPr(pp).SolidFill = colorToOxml(&c)
}

// SetLevelBullet sets the bullet kind for the given level (0-8). BulletChar
// applies a default character; use SetLevelBulletChar for a specific one.
// BulletInherit clears any explicit bullet so the level inherits it.
func (ts *MasterTextStyle) SetLevelBullet(level int, bt BulletType) {
	pp := ts.ensureLevel(level)
	if pp == nil {
		return
	}
	pp.BuNone, pp.BuAutoNum, pp.BuChar = nil, nil, nil
	switch bt {
	case BulletNone:
		pp.BuNone = &dml.BuNone{}
	case BulletAuto, BulletNumber:
		pp.BuAutoNum = &dml.BuAutoNum{Type: "arabicPeriod"}
	case BulletChar:
		pp.BuChar = &dml.BuChar{Char: "•"}
	case BulletInherit:
		// all cleared: inherit from the parent style
	}
}

// SetLevelBulletChar sets a literal bullet character for the given level (0-8).
func (ts *MasterTextStyle) SetLevelBulletChar(level int, char string) {
	pp := ts.ensureLevel(level)
	if pp == nil {
		return
	}
	pp.BuNone, pp.BuAutoNum = nil, nil
	pp.BuChar = &dml.BuChar{Char: char}
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
func createDefaultMaster(w, h dml.EMU) *SlideMaster {
	master := &SlideMaster{
		masterXML: newMasterXML(),
		layouts:   make([]*SlideLayout, 0),
	}

	// Add title placeholder to master, sized to the slide (C139).
	titlePh := NewPlaceholderShape(PlaceholderTitle)
	titleRect(w, h).apply(titlePh)
	titlePh.SetIndex(0)

	// Add body placeholder to master.
	bodyPh := NewPlaceholderShape(PlaceholderBody)
	bodyRect(w, h).apply(bodyPh)
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
	w, h := dml.EMU(9144000), dml.EMU(6858000)
	if sm.presentation != nil {
		w, h = sm.presentation.slideDimensions()
	}
	layout := createDefaultLayout(layoutType, sm, w, h)
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
