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
	// resolvedThemePart is the part name of the theme this master references,
	// resolved at open from its relationships. Theme() looks the editor up by
	// this name so masters sharing a theme part share one editor (C571).
	resolvedThemePart string
	layouts           []*SlideLayout
	relID        string
	numericID    uint32 // original numeric ID from presentation.xml
	idOmitted    bool   // the source entry carried no id attribute
	// idExtLst preserves the extLst child of this master's p:sldMasterId
	// entry in presentation.xml, which is regenerated on every save (C225).
	idExtLst        *oxml.ExtensionList
	layoutsModified bool // true if layouts changed via Go API
	// themePartName names a theme part imported for this master by a merge
	// (AppendSlidesFrom/ExtractSlides). When set, the saveNew path writes that
	// theme and points the master's theme relationship at it instead of the
	// hardcoded default theme1.xml. Empty for masters built by Create or loaded
	// from a file (whose theme is handled by the round-trip save path).
	themePartName string
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
	sm.presentation.markModelEdited()
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

// LayoutByType returns this master's first layout of the given type, or
// ErrLayoutNotFound. See Presentation.LayoutByType (C565).
func (sm *SlideMaster) LayoutByType(layoutType SlideLayoutType) (*SlideLayout, error) {
	if l := sm.GetLayout(layoutType); l != nil {
		return l, nil
	}
	return nil, fmt.Errorf("%w: no layout of type %v on this master", ErrLayoutNotFound, layoutType)
}

// LayoutByName returns this master's layout with the given name, or
// ErrLayoutNotFound. See Presentation.LayoutByName (C565).
func (sm *SlideMaster) LayoutByName(name string) (*SlideLayout, error) {
	if l := sm.GetLayoutByName(name); l != nil {
		return l, nil
	}
	return nil, fmt.Errorf("%w: %q on this master", ErrLayoutNotFound, name)
}

// GetLayout returns the layout with the specified type.
//
// Deprecated: use LayoutByType, which reports a miss as an error (C565).
func (sm *SlideMaster) GetLayout(layoutType SlideLayoutType) *SlideLayout {
	for _, layout := range sm.layouts {
		if layout.Type() == layoutType {
			return layout
		}
	}
	return nil
}

// GetLayoutByName returns the layout with the specified name.
//
// Deprecated: use LayoutByName, which reports a miss as an error (C565).
func (sm *SlideMaster) GetLayoutByName(name string) *SlideLayout {
	for _, layout := range sm.layouts {
		if layout.Name() == name {
			return layout
		}
	}
	return nil
}

// Theme returns a read/write handle to this master's theme part, as the shared
// dml.ThemeEditor that docx.Document.Theme and xlsx.Workbook.Theme also return
// (C571). Edits are written back on save; an untouched theme round-trips
// byte-for-byte. It returns nil for masters created programmatically or whose
// theme part is missing or unparseable.
func (sm *SlideMaster) Theme() *dml.ThemeEditor {
	if sm.presentation == nil || sm.resolvedThemePart == "" {
		return nil
	}
	return sm.presentation.themeEditorFor(sm.resolvedThemePart)
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

// Placeholder returns the master's placeholder of the given type, or
// ErrPlaceholderNotFound when it has none. See Slide.Placeholder (C565).
func (sm *SlideMaster) Placeholder(phType PlaceholderType) (*PlaceholderShape, error) {
	if ph := sm.GetPlaceholder(phType); ph != nil {
		return ph, nil
	}
	return nil, fmt.Errorf("%w: type %v on master %q", ErrPlaceholderNotFound, phType, sm.Name())
}

// GetPlaceholder returns the master placeholder with the specified type, or nil.
//
// Deprecated: use Placeholder, which reports a miss as an error (C565).
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
	return editablePlaceholdersFromSpTree(sm.presentation, sm.masterXML.CSld.SpTree)
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
// It also records the edit: every SetLevel* setter reaches the master through
// here, and the master part is regenerated on every save, so none of them needs
// a flag to persist and nothing else would notice the deck had changed.
func (ts *MasterTextStyle) ensureLevel(level int) *dml.PPr {
	if level < 0 || level > 8 {
		return nil
	}
	ts.sm.presentation.markModelEdited()
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
	if sm.regeneratesLayoutIDs() {
		if len(sm.layouts) > 0 {
			sm.masterXML.SlideLayoutIDs = &oxml.SlideLayoutIDs{
				SlideLayoutID: make([]oxml.SlideLayoutID, len(sm.layouts)),
			}
			next := sm.layoutIDStart()
			for i, layout := range sm.layouts {
				sm.masterXML.SlideLayoutIDs.SlideLayoutID[i] = oxml.SlideLayoutID{
					ID:  next,
					RID: layout.relID,
				}
				if next < math.MaxUint32 {
					next++
				}
			}
		}
	}

	// Use the namespace-aware marshaler for PowerPoint compatibility
	return marshalSlideMaster(sm.masterXML)
}

// reindexCollidingLayoutIDs renumbers a freshly imported master's preserved
// sldLayoutIdLst when any of its ids is already claimed by another master in
// this deck.
//
// ST_SlideLayoutId is document-unique, but the source's ids were allocated
// against the source deck alone, so importing them verbatim reproduces the
// destination's own range one-for-one — two masters then advertise the same
// layout ids (C419). The numeric id is not a link target (layouts are bound to
// the master by r:id), so renumbering is lossless. Non-colliding lists are left
// exactly as imported.
func (p *Presentation) reindexCollidingLayoutIDs(nm *SlideMaster) {
	if nm == nil || nm.masterXML == nil || nm.masterXML.SlideLayoutIDs == nil {
		return
	}
	taken := make(map[uint32]bool)
	next := slideLayoutIDBase
	for _, m := range p.slideMasters {
		if m == nil || m == nm || m.masterXML == nil || m.masterXML.SlideLayoutIDs == nil {
			continue
		}
		for _, l := range m.masterXML.SlideLayoutIDs.SlideLayoutID {
			taken[l.ID] = true
		}
		next = nextIDAbove(next, m.masterXML.SlideLayoutIDs.SlideLayoutID, func(l oxml.SlideLayoutID) uint32 {
			return l.ID
		})
	}
	ids := nm.masterXML.SlideLayoutIDs.SlideLayoutID
	collides := false
	for _, l := range ids {
		if taken[l.ID] {
			collides = true
			break
		}
	}
	if !collides {
		return
	}
	for i := range ids {
		ids[i].ID = next
		if next < math.MaxUint32 {
			next++
		}
	}
}

// regeneratesLayoutIDs reports whether this master's sldLayoutIdLst is rebuilt
// from sm.layouts on marshal rather than preserved verbatim.
func (sm *SlideMaster) regeneratesLayoutIDs() bool {
	return sm.layoutsModified || sm.masterXML == nil || sm.masterXML.SlideLayoutIDs == nil
}

// layoutIDStart returns the first ST_SlideLayoutId this master may assign when
// it rebuilds its sldLayoutIdLst.
//
// ST_SlideLayoutId is unique per document, not per master, so the fixed
// base+index scheme gave every regenerating master the same range: two modified
// masters in one deck emitted overlapping layout ids, and a modified master
// overlapped any preserved master starting at the same base (C419). Start above
// every id the preserved masters hold, then hand each regenerating master its
// own disjoint block in deck order so the result is deterministic and
// independent of marshal order.
func (sm *SlideMaster) layoutIDStart() uint32 {
	if sm.presentation == nil {
		return slideLayoutIDBase
	}
	next := slideLayoutIDBase
	for _, m := range sm.presentation.slideMasters {
		if m == nil || m == sm || m.regeneratesLayoutIDs() {
			continue
		}
		next = nextIDAbove(next, m.masterXML.SlideLayoutIDs.SlideLayoutID, func(l oxml.SlideLayoutID) uint32 {
			return l.ID
		})
	}
	for _, m := range sm.presentation.slideMasters {
		if m == sm {
			break
		}
		if m == nil || !m.regeneratesLayoutIDs() {
			continue
		}
		if remaining := math.MaxUint32 - next; uint32(len(m.layouts)) >= remaining {
			return math.MaxUint32
		}
		next += uint32(len(m.layouts))
	}
	return next
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
	sm.presentation.markModelEdited()
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
		if id := sm.presentation.nextRelIDNum(sm.partName) - 1; id > maxRel {
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
	p.relationships[layout.partName] = append(p.relationships[layout.partName], &opc.Relationship{
		ID:         fmt.Sprintf("rId%d", p.nextRelIDNum(layout.partName)),
		Type:       opc.RelTypeSlideMaster,
		Target:     partNameToRelTarget(sm.partName, path.Dir(layout.partName)+"/"),
		TargetMode: opc.TargetModeInternal,
	})
}
