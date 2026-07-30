package xlsx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Built-in number format IDs.
const (
	NumberFormatGeneral  = 0
	NumberFormatInteger  = 1  // "0"
	NumberFormatDecimal  = 2  // "0.00"
	NumberFormatComma    = 3  // "#,##0"
	NumberFormatPercent  = 9  // "0%"
	NumberFormatDate     = 14 // "mm-dd-yy"
	NumberFormatTime     = 20 // "h:mm"
	NumberFormatDateTime = 22 // "m/d/yy h:mm"
	NumberFormatText     = 49 // "@"
)

// firstCustomNumFmtID is the starting ID for user-defined number formats.
// IDs 0-163 are reserved for built-in formats.
const firstCustomNumFmtID = 164

// builtinNumFmtCodes maps built-in format IDs to their format strings.
var builtinNumFmtCodes = map[string]uint32{
	"General":       0,
	"0":             1,
	"0.00":          2,
	"#,##0":         3,
	"#,##0.00":      4,
	"0%":            9,
	"0.00%":         10,
	"0.00E+00":      11,
	"# ?/?":         12,
	"# ??/??":       13,
	"mm-dd-yy":      14,
	"d-mmm-yy":      15,
	"d-mmm":         16,
	"mmm-yy":        17,
	"h:mm AM/PM":    18,
	"h:mm:ss AM/PM": 19,
	"h:mm":          20,
	"h:mm:ss":       21,
	"m/d/yy h:mm":   22,
	"@":             49,
}

// StyleManager manages the workbook stylesheet.
type StyleManager struct {
	stylesheet *oxml.CT_Stylesheet
	// onModify is invoked by mutating operations so the owning workbook can
	// mark styles.xml dirty. Merely reading styles must not trigger it.
	onModify func()
}

// newStyleManager creates a StyleManager that wraps the given stylesheet.
// If ss is nil, a new default stylesheet is created with the required
// two fills (none + gray125) and a default font. onModify (may be nil) is
// called when a mutating method changes the stylesheet.
func newStyleManager(ss *oxml.CT_Stylesheet, onModify func()) *StyleManager {
	if ss == nil {
		ss = defaultStylesheet()
	}
	return &StyleManager{stylesheet: ss, onModify: onModify}
}

// markModified notifies the owner that the stylesheet changed.
func (sm *StyleManager) markModified() {
	if sm.onModify != nil {
		sm.onModify()
	}
}

// defaultStylesheet creates a minimal stylesheet that Excel requires.
func defaultStylesheet() *oxml.CT_Stylesheet {
	one := uint32(1)
	two := uint32(2)
	zero := uint32(0)
	xfID := uint32(0)

	return &oxml.CT_Stylesheet{
		Fonts: &oxml.CT_Fonts{
			Count: &one,
			Font: []oxml.CT_Font{
				{
					Sz:   &oxml.CT_FontSize{Val: 11},
					Name: &oxml.CT_FontName{Val: "Calibri"},
				},
			},
		},
		Fills: &oxml.CT_Fills{
			Count: &two,
			Fill: []oxml.CT_Fill{
				{PatternFill: &oxml.CT_PatternFill{PatternType: "none"}},
				{PatternFill: &oxml.CT_PatternFill{PatternType: "gray125"}},
			},
		},
		Borders: &oxml.CT_Borders{
			Count: &one,
			Border: []oxml.CT_Border{
				{}, // empty border
			},
		},
		CellStyleXfs: &oxml.CT_CellStyleXfs{
			Count: &one,
			Xf: []oxml.CT_Xf{
				{NumFmtId: &zero, FontId: &zero, FillId: &zero, BorderId: &zero},
			},
		},
		CellXfs: &oxml.CT_CellXfs{
			Count: &one,
			Xf: []oxml.CT_Xf{
				{NumFmtId: &zero, FontId: &zero, FillId: &zero, BorderId: &zero, XfId: &xfID},
			},
		},
		CellStyles: &oxml.CT_CellStyles{
			Count: &one,
			CellStyle: []oxml.CT_CellStyle{
				{Name: "Normal", XfId: 0, BuiltinId: &zero},
			},
		},
	}
}

// NewCellStyle creates a new cell format from the given style definition and
// returns its 0-based index into cellXfs. The cell format index can be applied
// to cells via Cell.SetStyleIndex.
func (sm *StyleManager) NewCellStyle(style CellStyle) (uint32, error) {
	if err := validateCellStyle(&style); err != nil {
		return 0, err
	}
	if err := sm.checkNumberFormatID(&style); err != nil {
		return 0, err
	}
	ss := sm.stylesheet

	// Build the Xf record, linking it to the default (Normal) named style.
	// resolveXf marks the stylesheet modified only when it actually appends a
	// font/fill/border/numFmt record; a new record forces a new xf here too, so
	// dedup can never short-circuit after resolveXf added something.
	xfID := uint32(0)
	xf := sm.resolveXf(&style, &xfID)

	// De-duplicate: check if an identical xf already exists. A no-op call (the
	// requested style already exists) must not mark styles modified, or an
	// unmodified opened workbook would regenerate styles.xml and lose its
	// byte-identical round-trip.
	if ss.CellXfs != nil {
		for i, existing := range ss.CellXfs.Xf {
			if xfEqual(&existing, &xf) {
				return uint32(i), nil
			}
		}
	}

	// Add new xf
	sm.markModified()
	if ss.CellXfs == nil {
		ss.CellXfs = &oxml.CT_CellXfs{}
	}
	ss.CellXfs.Xf = append(ss.CellXfs.Xf, xf)
	idx := uint32(len(ss.CellXfs.Xf) - 1)
	count := uint32(len(ss.CellXfs.Xf))
	ss.CellXfs.Count = &count

	return idx, nil
}

// resolveXf builds a CT_Xf for the given style, resolving (and adding when
// necessary) the font, fill, border and number-format records it references.
// xfID, when non-nil, is stored as the record's xfId — the link to a named
// style in cellStyleXfs (0 for the default Normal style). The corresponding
// applyX flag is set for every component the style specifies.
func (sm *StyleManager) resolveXf(style *CellStyle, xfID *uint32) oxml.CT_Xf {
	fontID := uint32(0) // default font
	if style.Font != nil {
		fontID = sm.findOrAddFont(fontStyleToOxml(style.Font))
	}

	fillID := uint32(0) // "none" fill
	if style.Fill != nil {
		fillID = sm.findOrAddFill(fillStyleToOxml(style.Fill))
	}

	borderID := uint32(0) // empty border
	if style.Border != nil {
		borderID = sm.findOrAddBorder(borderStyleToOxml(style.Border))
	}

	// Number format: a format code string wins over a raw id.
	numFmtID := uint32(0)
	switch {
	case style.Format != "":
		numFmtID = sm.resolveNumberFormat(style.Format)
	case style.NumberFormatID != 0:
		numFmtID = uint32(style.NumberFormatID)
	}

	xf := oxml.CT_Xf{
		NumFmtId: &numFmtID,
		FontId:   &fontID,
		FillId:   &fillID,
		BorderId: &borderID,
		XfId:     xfID,
	}

	t := true
	if style.Font != nil {
		xf.ApplyFont = &t
	}
	if style.Fill != nil {
		xf.ApplyFill = &t
	}
	if style.Border != nil {
		xf.ApplyBorder = &t
	}
	if style.Format != "" || style.NumberFormatID != 0 {
		xf.ApplyNumberFormat = &t
	}
	if style.Alignment != nil {
		xf.ApplyAlignment = &t
		xf.Alignment = alignmentStyleToOxml(style.Alignment)
	}
	if style.Protection != nil {
		locked := style.Protection.Locked
		hidden := style.Protection.Hidden
		xf.ApplyProtection = &t
		xf.Protection = &oxml.CT_CellProtection{Locked: &locked, Hidden: &hidden}
	}
	return xf
}

// Built-in cell style IDs (CT_CellStyle builtinId, ST_BuiltinStyle). These name
// the styles Excel ships in the Cell Styles gallery ("Good", "Bad", "Heading 1"
// …); pass one as NamedStyle.BuiltinId when defining a style that mirrors a
// built-in.
const (
	BuiltinStyleNormal            uint32 = 0
	BuiltinStyleRowLevel          uint32 = 1
	BuiltinStyleColLevel          uint32 = 2
	BuiltinStyleComma             uint32 = 3
	BuiltinStyleCurrency          uint32 = 4
	BuiltinStylePercent           uint32 = 5
	BuiltinStyleCommaZero         uint32 = 6
	BuiltinStyleCurrencyZero      uint32 = 7
	BuiltinStyleHyperlink         uint32 = 8
	BuiltinStyleFollowedHyperlink uint32 = 9
	BuiltinStyleNote              uint32 = 10
	BuiltinStyleWarningText       uint32 = 11
	BuiltinStyleTitle             uint32 = 15
	BuiltinStyleHeading1          uint32 = 16
	BuiltinStyleHeading2          uint32 = 17
	BuiltinStyleHeading3          uint32 = 18
	BuiltinStyleHeading4          uint32 = 19
	BuiltinStyleInput             uint32 = 20
	BuiltinStyleOutput            uint32 = 21
	BuiltinStyleCalculation       uint32 = 22
	BuiltinStyleCheckCell         uint32 = 23
	BuiltinStyleLinkedCell        uint32 = 24
	BuiltinStyleTotal             uint32 = 25
	BuiltinStyleGood              uint32 = 26
	BuiltinStyleBad               uint32 = 27
	BuiltinStyleNeutral           uint32 = 28
	BuiltinStyleAccent1           uint32 = 29
	BuiltinStyleExplanatoryText   uint32 = 53
)

// NamedStyle is a named (or built-in) cell style: a reusable format that shows
// up in Excel's Cell Styles gallery and can be applied to cells by name.
type NamedStyle struct {
	// Name is the style's display name (e.g. "Good", "Heading 1", "My Style").
	Name string
	// Style is the formatting the named style applies.
	Style CellStyle
	// BuiltinId, when non-nil, links the style to one of Excel's built-in
	// styles (see the BuiltinStyle* constants).
	BuiltinId *uint32
	// Hidden hides the style from the gallery.
	Hidden bool
	// CustomBuiltin marks a built-in style that the user has customized.
	CustomBuiltin bool
}

// AddNamedStyle defines a named cell style and returns its xfId (its index into
// cellStyleXfs), the value ApplyNamedStyle and Cell.SetNamedStyle use to apply
// it. If a style with the same name already exists it is left untouched and its
// existing xfId is returned.
func (sm *StyleManager) AddNamedStyle(ns NamedStyle) (uint32, error) {
	if ns.Name == "" {
		return 0, fmt.Errorf("xlsx: named style must have a name")
	}
	if err := validateCellStyle(&ns.Style); err != nil {
		return 0, err
	}
	if xfID, ok := sm.NamedStyleXfId(ns.Name); ok {
		return xfID, nil
	}
	sm.markModified()
	ss := sm.stylesheet

	// The named style's master format lives in cellStyleXfs; unlike a cellXfs
	// record it does not carry an xfId back-reference.
	xf := sm.resolveXf(&ns.Style, nil)
	if ss.CellStyleXfs == nil {
		ss.CellStyleXfs = &oxml.CT_CellStyleXfs{}
	}
	ss.CellStyleXfs.Xf = append(ss.CellStyleXfs.Xf, xf)
	xfID := uint32(len(ss.CellStyleXfs.Xf) - 1)
	csxfCount := uint32(len(ss.CellStyleXfs.Xf))
	ss.CellStyleXfs.Count = &csxfCount

	cs := oxml.CT_CellStyle{Name: ns.Name, XfId: xfID}
	if ns.BuiltinId != nil {
		b := *ns.BuiltinId
		cs.BuiltinId = &b
	}
	if ns.Hidden {
		h := true
		cs.Hidden = &h
	}
	if ns.CustomBuiltin {
		c := true
		cs.CustomBuiltin = &c
	}
	if ss.CellStyles == nil {
		ss.CellStyles = &oxml.CT_CellStyles{}
	}
	ss.CellStyles.CellStyle = append(ss.CellStyles.CellStyle, cs)
	csCount := uint32(len(ss.CellStyles.CellStyle))
	ss.CellStyles.Count = &csCount

	return xfID, nil
}

// NamedStyleXfId returns the xfId of the named style with the given name.
func (sm *StyleManager) NamedStyleXfId(name string) (uint32, bool) {
	ss := sm.stylesheet
	if ss.CellStyles == nil {
		return 0, false
	}
	for _, cs := range ss.CellStyles.CellStyle {
		if cs.Name == name {
			return cs.XfId, true
		}
	}
	return 0, false
}

// NamedStyles returns every named cell style defined in the workbook.
func (sm *StyleManager) NamedStyles() []NamedStyle {
	ss := sm.stylesheet
	if ss.CellStyles == nil {
		return nil
	}
	result := make([]NamedStyle, 0, len(ss.CellStyles.CellStyle))
	for _, cs := range ss.CellStyles.CellStyle {
		ns := NamedStyle{
			Name:          cs.Name,
			Hidden:        cs.Hidden != nil && *cs.Hidden,
			CustomBuiltin: cs.CustomBuiltin != nil && *cs.CustomBuiltin,
		}
		if cs.BuiltinId != nil {
			b := *cs.BuiltinId
			ns.BuiltinId = &b
		}
		if ss.CellStyleXfs != nil && int(cs.XfId) < len(ss.CellStyleXfs.Xf) {
			ns.Style = sm.cellStyleFromXf(&ss.CellStyleXfs.Xf[cs.XfId])
		}
		result = append(result, ns)
	}
	return result
}

// ApplyNamedStyle creates (or reuses) a cellXfs record linked to the named
// style and returns its index, ready to pass to Cell.SetStyleIndex. It fails if
// no style with that name exists.
func (sm *StyleManager) ApplyNamedStyle(name string) (uint32, error) {
	xfID, ok := sm.NamedStyleXfId(name)
	if !ok {
		return 0, fmt.Errorf("xlsx: named style %q not found", name)
	}
	ss := sm.stylesheet

	// A cellStyle whose xfId points past (or into an absent) cellStyleXfs is
	// malformed but occurs in the wild; report it rather than panic (C544).
	if ss.CellStyleXfs == nil || int(xfID) >= len(ss.CellStyleXfs.Xf) {
		return 0, fmt.Errorf("xlsx: named style %q references cellStyleXfs index %d, which does not exist", name, xfID)
	}
	master := &ss.CellStyleXfs.Xf[xfID]
	linkID := xfID
	xf := oxml.CT_Xf{
		NumFmtId: cloneUint32(master.NumFmtId),
		FontId:   cloneUint32(master.FontId),
		FillId:   cloneUint32(master.FillId),
		BorderId: cloneUint32(master.BorderId),
		XfId:     &linkID,
	}

	// De-duplicate before marking modified: a reuse-only call must not
	// regenerate styles.xml, or an unmodified opened workbook loses its
	// byte-identical round-trip — the discipline NewCellStyle and
	// AddNumberFormat already document (C544).
	if ss.CellXfs != nil {
		for i, existing := range ss.CellXfs.Xf {
			if xfEqual(&existing, &xf) {
				return uint32(i), nil
			}
		}
	}
	sm.markModified()
	if ss.CellXfs == nil {
		ss.CellXfs = &oxml.CT_CellXfs{}
	}
	ss.CellXfs.Xf = append(ss.CellXfs.Xf, xf)
	idx := uint32(len(ss.CellXfs.Xf) - 1)
	count := uint32(len(ss.CellXfs.Xf))
	ss.CellXfs.Count = &count
	return idx, nil
}

// cellStyleFromXf reconstructs the public the public CellStyle carried by an xf record
// (shared by GetCellStyle and NamedStyles).
func (sm *StyleManager) cellStyleFromXf(xf *oxml.CT_Xf) CellStyle {
	ss := sm.stylesheet
	var style CellStyle
	if xf.FontId != nil && ss.Fonts != nil && int(*xf.FontId) < len(ss.Fonts.Font) {
		style.Font = oxmlToFontStyle(&ss.Fonts.Font[*xf.FontId])
	}
	if xf.FillId != nil && ss.Fills != nil && int(*xf.FillId) < len(ss.Fills.Fill) {
		style.Fill = oxmlToFillStyle(&ss.Fills.Fill[*xf.FillId])
	}
	if xf.BorderId != nil && ss.Borders != nil && int(*xf.BorderId) < len(ss.Borders.Border) {
		style.Border = oxmlToBorderStyle(&ss.Borders.Border[*xf.BorderId])
	}
	if xf.NumFmtId != nil && *xf.NumFmtId != 0 {
		style.NumberFormatID = int(*xf.NumFmtId)
		style.Format = sm.resolveNumFmtCode(*xf.NumFmtId)
	}
	if xf.Alignment != nil {
		style.Alignment = oxmlToAlignmentStyle(xf.Alignment)
	}
	if xf.Protection != nil {
		style.Protection = &ProtectionStyle{
			Locked: xf.Protection.Locked == nil || *xf.Protection.Locked,
			Hidden: xf.Protection.Hidden != nil && *xf.Protection.Hidden,
		}
	}
	return style
}

// checkNumberFormatID rejects a NumberFormatID in the custom range
// (>= firstCustomNumFmtID) that no <numFmt> in the stylesheet defines. The
// Format-string path registers the format it needs; the raw-id path did not
// check, so the xf referenced a numFmtId that existed nowhere and the cell fell
// back to General in Excel (C550). Ids below the custom range are the built-in
// formats, which need no <numFmt> entry.
func (sm *StyleManager) checkNumberFormatID(style *CellStyle) error {
	if style.Format != "" || style.NumberFormatID < firstCustomNumFmtID {
		return nil
	}
	id := uint32(style.NumberFormatID)
	if ss := sm.stylesheet; ss != nil && ss.NumFmts != nil {
		for _, nf := range ss.NumFmts.NumFmt {
			if nf.NumFmtId == id {
				return nil
			}
		}
	}
	return fmt.Errorf("xlsx: number format id %d is not defined by any numFmt; register the format code with AddNumberFormat (or set CellStyle.Format) first", id)
}

// validateCellStyle rejects style values that would be silently corrupted on
// serialization. Alignment indent and rotation are unsigned in the schema, so
// negative Go values would wrap to huge numbers via uint conversion (C133);
// text rotation must be 0-180 or the special value 255 (vertical text).
func validateCellStyle(style *CellStyle) error {
	if style.NumberFormatID < 0 {
		return fmt.Errorf("xlsx: number format id %d must not be negative", style.NumberFormatID)
	}
	if a := style.Alignment; a != nil {
		if a.Indent < 0 || a.Indent > 250 {
			return fmt.Errorf("xlsx: alignment indent %d out of range 0-250", a.Indent)
		}
		if a.Rotation < 0 || (a.Rotation > 180 && a.Rotation != 255) {
			return fmt.Errorf("xlsx: text rotation %d out of range (0-180 or 255)", a.Rotation)
		}
	}
	return nil
}

// CellStyleAt returns the CellStyle for the given style index. It is the
// Get-less spelling of GetCellStyle (C565); the name carries the "At" suffix
// because CellStyle is also the name of the returned type.
func (sm *StyleManager) CellStyleAt(index uint32) (CellStyle, error) {
	return sm.GetCellStyle(index)
}

// GetCellStyle returns the CellStyle for the given style index.
//
// Deprecated: use CellStyleAt. Go accessors do not carry a Get prefix (C565).
func (sm *StyleManager) GetCellStyle(index uint32) (CellStyle, error) {
	ss := sm.stylesheet

	if ss.CellXfs == nil || int(index) >= len(ss.CellXfs.Xf) {
		return CellStyle{}, fmt.Errorf("xlsx: style index %d out of range", index)
	}

	return sm.cellStyleFromXf(&ss.CellXfs.Xf[index]), nil
}

// AddNumberFormat registers a custom number format string and returns its ID.
// If the format string matches a built-in format, the built-in ID is returned.
func (sm *StyleManager) AddNumberFormat(code string) uint32 {
	// resolveNumberFormat marks the stylesheet modified only when it appends a
	// new custom format. Registering a built-in or already-present format is a
	// no-op and must not dirty styles.xml (which would break an unmodified
	// opened workbook's byte-identical round-trip).
	return sm.resolveNumberFormat(code)
}

// resolveNumberFormat returns the numFmtId for a format code string.
// It checks built-in formats first, then existing custom formats, and finally
// creates a new custom format if needed.
func (sm *StyleManager) resolveNumberFormat(code string) uint32 {
	// Check built-in formats
	if id, ok := builtinNumFmtCodes[code]; ok {
		return id
	}

	ss := sm.stylesheet

	// Check existing custom formats
	if ss.NumFmts != nil {
		for _, nf := range ss.NumFmts.NumFmt {
			if nf.FormatCode == code {
				return nf.NumFmtId
			}
		}
	}

	// Create new custom format
	sm.markModified()
	if ss.NumFmts == nil {
		ss.NumFmts = &oxml.CT_NumFmts{}
	}

	nextID := uint32(firstCustomNumFmtID)
	for _, nf := range ss.NumFmts.NumFmt {
		if nf.NumFmtId >= nextID {
			nextID = nf.NumFmtId + 1
		}
	}

	ss.NumFmts.NumFmt = append(ss.NumFmts.NumFmt, oxml.CT_NumFmt{
		NumFmtId:   nextID,
		FormatCode: code,
	})
	count := uint32(len(ss.NumFmts.NumFmt))
	ss.NumFmts.Count = &count

	return nextID
}

// resolveNumFmtCode returns the format code string for a numFmtId.
func (sm *StyleManager) resolveNumFmtCode(id uint32) string {
	// Check built-in
	for code, fmtID := range builtinNumFmtCodes {
		if fmtID == id {
			return code
		}
	}
	// Check custom
	if sm.stylesheet.NumFmts != nil {
		for _, nf := range sm.stylesheet.NumFmts.NumFmt {
			if nf.NumFmtId == id {
				return nf.FormatCode
			}
		}
	}
	return ""
}

// findOrAddFont finds an existing font matching f or adds a new one. Returns the index.
func (sm *StyleManager) findOrAddFont(f oxml.CT_Font) uint32 {
	ss := sm.stylesheet
	if ss.Fonts == nil {
		ss.Fonts = &oxml.CT_Fonts{}
	}

	for i, existing := range ss.Fonts.Font {
		if fontEqual(&existing, &f) {
			return uint32(i)
		}
	}

	ss.Fonts.Font = append(ss.Fonts.Font, f)
	count := uint32(len(ss.Fonts.Font))
	ss.Fonts.Count = &count
	return count - 1
}

// findOrAddFill finds an existing fill matching f or adds a new one. Returns the index.
func (sm *StyleManager) findOrAddFill(f oxml.CT_Fill) uint32 {
	ss := sm.stylesheet
	if ss.Fills == nil {
		ss.Fills = &oxml.CT_Fills{}
	}

	for i, existing := range ss.Fills.Fill {
		if fillEqual(&existing, &f) {
			return uint32(i)
		}
	}

	ss.Fills.Fill = append(ss.Fills.Fill, f)
	count := uint32(len(ss.Fills.Fill))
	ss.Fills.Count = &count
	return count - 1
}

// findOrAddBorder finds an existing border matching b or adds a new one. Returns the index.
func (sm *StyleManager) findOrAddBorder(b oxml.CT_Border) uint32 {
	ss := sm.stylesheet
	if ss.Borders == nil {
		ss.Borders = &oxml.CT_Borders{}
	}

	for i, existing := range ss.Borders.Border {
		if borderEqual(&existing, &b) {
			return uint32(i)
		}
	}

	ss.Borders.Border = append(ss.Borders.Border, b)
	count := uint32(len(ss.Borders.Border))
	ss.Borders.Count = &count
	return count - 1
}

// --- Conversion helpers: public types → internal oxml types ---

func fontStyleToOxml(fs *FontStyle) oxml.CT_Font {
	f := oxml.CT_Font{}
	if fs.Name != "" {
		f.Name = &oxml.CT_FontName{Val: fs.Name}
	}
	if fs.Size > 0 {
		f.Sz = &oxml.CT_FontSize{Val: fs.Size}
	}
	if fs.Bold {
		f.B = &oxml.CT_BooleanProperty{}
	}
	if fs.Italic {
		f.I = &oxml.CT_BooleanProperty{}
	}
	if fs.Strike {
		f.Strike = &oxml.CT_BooleanProperty{}
	}
	if u := fontUnderlineToOxml(fs); u != nil {
		f.U = u
	}
	if fs.VertAlign != "" {
		f.VertAlign = &oxml.CT_VerticalAlignFontProperty{Val: string(fs.VertAlign)}
	}
	if fs.Color != "" {
		f.Color = &oxml.CT_Color{Rgb: normalizeHexColor(fs.Color)}
	}
	return f
}

// fontUnderlineToOxml builds the underline property element for a font style,
// or nil when the style has no underline. UnderlineStyle wins over the plain
// Underline bool when both are set.
func fontUnderlineToOxml(fs *FontStyle) *oxml.CT_UnderlineProperty {
	if fs.UnderlineStyle != "" {
		if fs.UnderlineStyle == UnderlineNone {
			return nil
		}
		return &oxml.CT_UnderlineProperty{Val: string(fs.UnderlineStyle)}
	}
	if fs.Underline {
		return &oxml.CT_UnderlineProperty{Val: "single"}
	}
	return nil
}

// applyOxmlUnderline reads an underline property element into a FontStyle,
// setting the plain bool and, when the source names an explicit style, the
// richer UnderlineStyle.
func applyOxmlUnderline(fs *FontStyle, u *oxml.CT_UnderlineProperty) {
	if u == nil {
		return
	}
	fs.Underline = strings.ToLower(u.Val) != "none"
	if u.Val != "" {
		fs.UnderlineStyle = UnderlineStyle(u.Val)
	}
}

func fillStyleToOxml(fs *FillStyle) oxml.CT_Fill {
	if fs.Gradient != nil {
		return oxml.CT_Fill{GradientFill: gradientFillToOxml(fs.Gradient)}
	}
	pf := &oxml.CT_PatternFill{}
	if fs.Pattern != "" {
		pf.PatternType = fs.Pattern
	} else {
		pf.PatternType = "solid"
	}
	if fs.FgColor != "" {
		pf.FgColor = &oxml.CT_Color{Rgb: normalizeHexColor(fs.FgColor)}
	}
	if fs.BgColor != "" {
		pf.BgColor = &oxml.CT_Color{Rgb: normalizeHexColor(fs.BgColor)}
	}
	return oxml.CT_Fill{PatternFill: pf}
}

// gradientFillToOxml converts a public GradientFill to its oxml form. The
// position attributes (type/degree/left/right/top/bottom) are only emitted
// when non-zero so a plain linear gradient stays minimal.
func gradientFillToOxml(g *GradientFill) *oxml.CT_GradientFill {
	gf := &oxml.CT_GradientFill{}
	if g.Type != "" {
		gf.Type = g.Type
	}
	if g.Degree != 0 {
		v := g.Degree
		gf.Degree = &v
	}
	if g.Left != 0 {
		v := g.Left
		gf.Left = &v
	}
	if g.Right != 0 {
		v := g.Right
		gf.Right = &v
	}
	if g.Top != 0 {
		v := g.Top
		gf.Top = &v
	}
	if g.Bottom != 0 {
		v := g.Bottom
		gf.Bottom = &v
	}
	for _, s := range g.Stops {
		gf.Stop = append(gf.Stop, oxml.CT_GradientStop{
			Position: s.Position,
			Color:    oxml.CT_Color{Rgb: normalizeHexColor(s.Color)},
		})
	}
	return gf
}

func borderStyleToOxml(bs *BorderStyle) oxml.CT_Border {
	b := oxml.CT_Border{}
	if bs.Left != nil {
		b.Left = borderSideToOxml(bs.Left)
	}
	if bs.Right != nil {
		b.Right = borderSideToOxml(bs.Right)
	}
	if bs.Top != nil {
		b.Top = borderSideToOxml(bs.Top)
	}
	if bs.Bottom != nil {
		b.Bottom = borderSideToOxml(bs.Bottom)
	}
	if bs.Diagonal != nil {
		b.Diagonal = borderSideToOxml(bs.Diagonal)
	}
	if bs.DiagonalUp {
		up := true
		b.DiagonalUp = &up
	}
	if bs.DiagonalDown {
		down := true
		b.DiagonalDown = &down
	}
	return b
}

func borderSideToOxml(side *BorderSide) *oxml.CT_BorderPr {
	bp := &oxml.CT_BorderPr{}
	if side.Style != "" {
		bp.Style = side.Style
	}
	if side.Color != "" {
		bp.Color = &oxml.CT_Color{Rgb: normalizeHexColor(side.Color)}
	}
	return bp
}

func alignmentStyleToOxml(as *AlignmentStyle) *oxml.CT_CellAlignment {
	a := &oxml.CT_CellAlignment{}
	if as.Horizontal != "" {
		a.Horizontal = as.Horizontal
	}
	if as.Vertical != "" {
		a.Vertical = as.Vertical
	}
	if as.WrapText {
		t := true
		a.WrapText = &t
	}
	if as.Indent != 0 {
		indent := uint32(as.Indent)
		a.Indent = &indent
	}
	if as.Rotation != 0 {
		rotation := uint32(as.Rotation)
		a.TextRotation = &rotation
	}
	if as.ShrinkToFit {
		t := true
		a.ShrinkToFit = &t
	}
	if as.JustifyLastLine {
		t := true
		a.JustifyLastLine = &t
	}
	if as.ReadingOrder != 0 {
		ro := as.ReadingOrder
		a.ReadingOrder = &ro
	}
	if as.RelativeIndent != 0 {
		ri := int32(as.RelativeIndent)
		a.RelativeIndent = &ri
	}
	return a
}

// --- Conversion helpers: internal oxml types → public types ---

func oxmlToFontStyle(f *oxml.CT_Font) *FontStyle {
	fs := &FontStyle{}
	if f.Name != nil {
		fs.Name = f.Name.Val
	}
	if f.Sz != nil {
		fs.Size = f.Sz.Val
	}
	if f.B != nil {
		fs.Bold = f.B.Val == nil || *f.B.Val
	}
	if f.I != nil {
		fs.Italic = f.I.Val == nil || *f.I.Val
	}
	if f.Strike != nil {
		fs.Strike = f.Strike.Val == nil || *f.Strike.Val
	}
	applyOxmlUnderline(fs, f.U)
	if f.VertAlign != nil {
		fs.VertAlign = enum.VerticalAlignRun(f.VertAlign.Val)
	}
	if f.Color != nil && f.Color.Rgb != "" {
		fs.Color = stripAlphaFromRGB(f.Color.Rgb)
	}
	// Return nil if it's just a default font with no user-visible properties
	if fs.Name == "" && fs.Size == 0 && !fs.Bold && !fs.Italic && !fs.Underline &&
		!fs.Strike && fs.UnderlineStyle == "" && fs.VertAlign == "" && fs.Color == "" {
		return nil
	}
	return fs
}

func oxmlToFillStyle(f *oxml.CT_Fill) *FillStyle {
	if f.GradientFill != nil {
		return &FillStyle{Gradient: oxmlToGradientFill(f.GradientFill)}
	}
	if f.PatternFill == nil {
		return nil
	}
	pf := f.PatternFill
	if pf.PatternType == "none" || pf.PatternType == "" {
		return nil
	}
	fs := &FillStyle{Pattern: pf.PatternType}
	if pf.FgColor != nil && pf.FgColor.Rgb != "" {
		fs.FgColor = stripAlphaFromRGB(pf.FgColor.Rgb)
	}
	if pf.BgColor != nil && pf.BgColor.Rgb != "" {
		fs.BgColor = stripAlphaFromRGB(pf.BgColor.Rgb)
	}
	return fs
}

// oxmlToGradientFill converts an oxml gradient fill to its public form.
func oxmlToGradientFill(gf *oxml.CT_GradientFill) *GradientFill {
	g := &GradientFill{Type: gf.Type}
	if gf.Degree != nil {
		g.Degree = *gf.Degree
	}
	if gf.Left != nil {
		g.Left = *gf.Left
	}
	if gf.Right != nil {
		g.Right = *gf.Right
	}
	if gf.Top != nil {
		g.Top = *gf.Top
	}
	if gf.Bottom != nil {
		g.Bottom = *gf.Bottom
	}
	for _, s := range gf.Stop {
		g.Stops = append(g.Stops, GradientStop{
			Position: s.Position,
			Color:    stripAlphaFromRGB(s.Color.Rgb),
		})
	}
	return g
}

func oxmlToBorderStyle(b *oxml.CT_Border) *BorderStyle {
	bs := &BorderStyle{}
	if b.Left != nil && b.Left.Style != "" {
		bs.Left = oxmlToBorderSide(b.Left)
	}
	if b.Right != nil && b.Right.Style != "" {
		bs.Right = oxmlToBorderSide(b.Right)
	}
	if b.Top != nil && b.Top.Style != "" {
		bs.Top = oxmlToBorderSide(b.Top)
	}
	if b.Bottom != nil && b.Bottom.Style != "" {
		bs.Bottom = oxmlToBorderSide(b.Bottom)
	}
	if b.Diagonal != nil && b.Diagonal.Style != "" {
		bs.Diagonal = oxmlToBorderSide(b.Diagonal)
	}
	if b.DiagonalUp != nil && *b.DiagonalUp {
		bs.DiagonalUp = true
	}
	if b.DiagonalDown != nil && *b.DiagonalDown {
		bs.DiagonalDown = true
	}
	// Return nil if no sides are set
	if bs.Left == nil && bs.Right == nil && bs.Top == nil && bs.Bottom == nil &&
		bs.Diagonal == nil && !bs.DiagonalUp && !bs.DiagonalDown {
		return nil
	}
	return bs
}

func oxmlToBorderSide(bp *oxml.CT_BorderPr) *BorderSide {
	side := &BorderSide{Style: bp.Style}
	if bp.Color != nil && bp.Color.Rgb != "" {
		side.Color = stripAlphaFromRGB(bp.Color.Rgb)
	}
	return side
}

func oxmlToAlignmentStyle(a *oxml.CT_CellAlignment) *AlignmentStyle {
	as := &AlignmentStyle{}
	as.Horizontal = a.Horizontal
	as.Vertical = a.Vertical
	if a.WrapText != nil && *a.WrapText {
		as.WrapText = true
	}
	if a.Indent != nil {
		as.Indent = int(*a.Indent)
	}
	if a.TextRotation != nil {
		as.Rotation = int(*a.TextRotation)
	}
	if a.ShrinkToFit != nil && *a.ShrinkToFit {
		as.ShrinkToFit = true
	}
	if a.JustifyLastLine != nil && *a.JustifyLastLine {
		as.JustifyLastLine = true
	}
	if a.ReadingOrder != nil {
		as.ReadingOrder = *a.ReadingOrder
	}
	if a.RelativeIndent != nil {
		as.RelativeIndent = int(*a.RelativeIndent)
	}
	// Return nil if empty
	if as.Horizontal == "" && as.Vertical == "" && !as.WrapText && as.Indent == 0 &&
		as.Rotation == 0 && !as.ShrinkToFit && !as.JustifyLastLine &&
		as.ReadingOrder == 0 && as.RelativeIndent == 0 {
		return nil
	}
	return as
}

// --- Equality helpers for de-duplication ---

// xfEqual compares every CT_Xf field. NewCellStyle dedupes against parsed
// xfs, so skipping fields (quotePrefix, pivotButton, protection) would let a
// plain style silently reuse an xf carrying locked/hidden protection or a
// quote prefix (C232).
func xfEqual(a, b *oxml.CT_Xf) bool {
	return ptrUint32Equal(a.NumFmtId, b.NumFmtId) &&
		ptrUint32Equal(a.FontId, b.FontId) &&
		ptrUint32Equal(a.FillId, b.FillId) &&
		ptrUint32Equal(a.BorderId, b.BorderId) &&
		ptrUint32Equal(a.XfId, b.XfId) &&
		ptrBoolEqual(a.QuotePrefix, b.QuotePrefix) &&
		ptrBoolEqual(a.PivotButton, b.PivotButton) &&
		ptrBoolEqual(a.ApplyNumberFormat, b.ApplyNumberFormat) &&
		ptrBoolEqual(a.ApplyFont, b.ApplyFont) &&
		ptrBoolEqual(a.ApplyFill, b.ApplyFill) &&
		ptrBoolEqual(a.ApplyBorder, b.ApplyBorder) &&
		ptrBoolEqual(a.ApplyAlignment, b.ApplyAlignment) &&
		ptrBoolEqual(a.ApplyProtection, b.ApplyProtection) &&
		cellAlignmentEqual(a.Alignment, b.Alignment) &&
		cellProtectionEqual(a.Protection, b.Protection)
}

// cellProtectionEqual compares optional cell protection blocks.
func cellProtectionEqual(a, b *oxml.CT_CellProtection) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return ptrBoolEqual(a.Locked, b.Locked) && ptrBoolEqual(a.Hidden, b.Hidden)
}

func fontEqual(a, b *oxml.CT_Font) bool {
	return fontNameEqual(a.Name, b.Name) &&
		fontSizeEqual(a.Sz, b.Sz) &&
		boolPropEqual(a.B, b.B) &&
		boolPropEqual(a.I, b.I) &&
		boolPropEqual(a.Strike, b.Strike) &&
		underlineEqual(a.U, b.U) &&
		vertAlignFontEqual(a.VertAlign, b.VertAlign) &&
		colorEqual(a.Color, b.Color)
}

func vertAlignFontEqual(a, b *oxml.CT_VerticalAlignFontProperty) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Val == b.Val
}

func fillEqual(a, b *oxml.CT_Fill) bool {
	if (a.PatternFill == nil) != (b.PatternFill == nil) {
		return false
	}
	if a.PatternFill != nil && b.PatternFill != nil {
		if a.PatternFill.PatternType != b.PatternFill.PatternType {
			return false
		}
		if !colorEqual(a.PatternFill.FgColor, b.PatternFill.FgColor) {
			return false
		}
		if !colorEqual(a.PatternFill.BgColor, b.PatternFill.BgColor) {
			return false
		}
	}
	return gradientFillEqual(a.GradientFill, b.GradientFill)
}

func gradientFillEqual(a, b *oxml.CT_GradientFill) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.Type != b.Type ||
		!ptrFloat64Equal(a.Degree, b.Degree) ||
		!ptrFloat64Equal(a.Left, b.Left) ||
		!ptrFloat64Equal(a.Right, b.Right) ||
		!ptrFloat64Equal(a.Top, b.Top) ||
		!ptrFloat64Equal(a.Bottom, b.Bottom) ||
		len(a.Stop) != len(b.Stop) {
		return false
	}
	for i := range a.Stop {
		if a.Stop[i].Position != b.Stop[i].Position ||
			!colorEqual(&a.Stop[i].Color, &b.Stop[i].Color) {
			return false
		}
	}
	return true
}

func ptrFloat64Equal(a, b *float64) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return *a == *b
}

func borderEqual(a, b *oxml.CT_Border) bool {
	return borderPrEqual(a.Left, b.Left) &&
		borderPrEqual(a.Right, b.Right) &&
		borderPrEqual(a.Top, b.Top) &&
		borderPrEqual(a.Bottom, b.Bottom) &&
		borderPrEqual(a.Diagonal, b.Diagonal) &&
		ptrBoolEqual(a.DiagonalUp, b.DiagonalUp) &&
		ptrBoolEqual(a.DiagonalDown, b.DiagonalDown)
}

func borderPrEqual(a, b *oxml.CT_BorderPr) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.Style != b.Style {
		return false
	}
	return colorEqual(a.Color, b.Color)
}

func colorEqual(a, b *oxml.CT_Color) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return strings.EqualFold(a.Rgb, b.Rgb) &&
		ptrUint32Equal(a.Theme, b.Theme) &&
		ptrUint32Equal(a.Indexed, b.Indexed)
}

func cellAlignmentEqual(a, b *oxml.CT_CellAlignment) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Horizontal == b.Horizontal &&
		a.Vertical == b.Vertical &&
		ptrBoolEqual(a.WrapText, b.WrapText) &&
		ptrUint32Equal(a.Indent, b.Indent) &&
		ptrUint32Equal(a.TextRotation, b.TextRotation) &&
		ptrBoolEqual(a.ShrinkToFit, b.ShrinkToFit) &&
		ptrBoolEqual(a.JustifyLastLine, b.JustifyLastLine) &&
		ptrUint32Equal(a.ReadingOrder, b.ReadingOrder) &&
		ptrInt32Equal(a.RelativeIndent, b.RelativeIndent)
}

func ptrInt32Equal(a, b *int32) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return *a == *b
}

func fontNameEqual(a, b *oxml.CT_FontName) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Val == b.Val
}

func fontSizeEqual(a, b *oxml.CT_FontSize) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Val == b.Val
}

func boolPropEqual(a, b *oxml.CT_BooleanProperty) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	// nil val means true in CT_BooleanProperty
	aVal := a.Val == nil || *a.Val
	bVal := b.Val == nil || *b.Val
	return aVal == bVal
}

func underlineEqual(a, b *oxml.CT_UnderlineProperty) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Val == b.Val
}

func ptrUint32Equal(a, b *uint32) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return *a == *b
}

func ptrBoolEqual(a, b *bool) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return *a == *b
}

// SetNamedStyle applies a previously defined named style (see
// StyleManager.AddNamedStyle) to the cell by name.
//
// An unknown name leaves the cell and the sheet untouched: the dirty flag is
// set by SetStyleIndex on the success path only, so a failed lookup cannot
// force the worksheet part to be regenerated on save (C544 shape).
func (c *Cell) SetNamedStyle(name string) error {
	if c.sheet == nil || c.sheet.workbook == nil {
		return fmt.Errorf("xlsx: cell is not associated with a workbook")
	}
	idx, err := c.sheet.workbook.Styles().ApplyNamedStyle(name)
	if err != nil {
		return err
	}
	c.SetStyleIndex(idx)
	return nil
}

// ---------------------------------------------------------------------------
// Cross-workbook style import
// ---------------------------------------------------------------------------

// importXf registers, in this stylesheet, a cell format equivalent to index
// srcIdx of src, and returns its index.
//
// It clones the referenced font, fill and border records verbatim rather than
// round-tripping them through the public CellStyle. CellStyle carries colours
// only as an RGB string, so a theme- or indexed-coloured font or fill lost its
// colour entirely on the way through (C549c); cloning also carries every
// property CellStyle does not model. Custom number formats are re-registered
// by their format code so the imported id is valid in this stylesheet.
func (sm *StyleManager) importXf(src *StyleManager, srcIdx uint32) (uint32, error) {
	if src == nil || src.stylesheet == nil {
		return 0, fmt.Errorf("xlsx: source workbook has no stylesheet")
	}
	sss := src.stylesheet
	if sss.CellXfs == nil || int(srcIdx) >= len(sss.CellXfs.Xf) {
		return 0, fmt.Errorf("xlsx: style index %d out of range", srcIdx)
	}
	srcXf := &sss.CellXfs.Xf[srcIdx]

	// Every imported format links to the destination's default (Normal) named
	// style: the source's cellStyleXfs chain is not carried across.
	zero := uint32(0)
	xf := oxml.CT_Xf{
		XfId:              &zero,
		QuotePrefix:       cloneBoolPtr(srcXf.QuotePrefix),
		PivotButton:       cloneBoolPtr(srcXf.PivotButton),
		ApplyNumberFormat: cloneBoolPtr(srcXf.ApplyNumberFormat),
		ApplyFont:         cloneBoolPtr(srcXf.ApplyFont),
		ApplyFill:         cloneBoolPtr(srcXf.ApplyFill),
		ApplyBorder:       cloneBoolPtr(srcXf.ApplyBorder),
		ApplyAlignment:    cloneBoolPtr(srcXf.ApplyAlignment),
		ApplyProtection:   cloneBoolPtr(srcXf.ApplyProtection),
		Alignment:         cloneCellAlignment(srcXf.Alignment),
		Protection:        cloneCellProtection(srcXf.Protection),
	}

	fontID := uint32(0)
	if srcXf.FontId != nil && sss.Fonts != nil && int(*srcXf.FontId) < len(sss.Fonts.Font) {
		fontID = sm.findOrAddFont(cloneFont(&sss.Fonts.Font[*srcXf.FontId]))
	}
	xf.FontId = &fontID

	fillID := uint32(0)
	if srcXf.FillId != nil && sss.Fills != nil && int(*srcXf.FillId) < len(sss.Fills.Fill) {
		fillID = sm.findOrAddFill(cloneFill(&sss.Fills.Fill[*srcXf.FillId]))
	}
	xf.FillId = &fillID

	borderID := uint32(0)
	if srcXf.BorderId != nil && sss.Borders != nil && int(*srcXf.BorderId) < len(sss.Borders.Border) {
		borderID = sm.findOrAddBorder(cloneBorder(&sss.Borders.Border[*srcXf.BorderId]))
	}
	xf.BorderId = &borderID

	numFmtID := uint32(0)
	if srcXf.NumFmtId != nil && *srcXf.NumFmtId != 0 {
		id := *srcXf.NumFmtId
		if id < firstCustomNumFmtID {
			numFmtID = id // built-in: the same id means the same format everywhere
		} else if code := src.resolveNumFmtCode(id); code != "" {
			numFmtID = sm.resolveNumberFormat(code)
		}
	}
	xf.NumFmtId = &numFmtID

	ss := sm.stylesheet
	if ss.CellXfs != nil {
		for i, existing := range ss.CellXfs.Xf {
			if xfEqual(&existing, &xf) {
				return uint32(i), nil
			}
		}
	}
	sm.markModified()
	if ss.CellXfs == nil {
		ss.CellXfs = &oxml.CT_CellXfs{}
	}
	ss.CellXfs.Xf = append(ss.CellXfs.Xf, xf)
	count := uint32(len(ss.CellXfs.Xf))
	ss.CellXfs.Count = &count
	return count - 1, nil
}

// The clone* helpers below deep-copy style records so an imported record
// shares no memory with the source workbook's stylesheet.

func cloneBoolPtr(p *bool) *bool {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneFloat64Ptr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneStyleColor(c *oxml.CT_Color) *oxml.CT_Color {
	if c == nil {
		return nil
	}
	out := oxml.CT_Color{Rgb: c.Rgb}
	out.Auto = cloneBoolPtr(c.Auto)
	out.Indexed = cloneUint32(c.Indexed)
	out.Theme = cloneUint32(c.Theme)
	if c.Tint != nil {
		t := *c.Tint
		out.Tint = &t
	}
	return &out
}

func cloneFont(f *oxml.CT_Font) oxml.CT_Font {
	cloneIntProp := func(p *oxml.CT_IntProperty) *oxml.CT_IntProperty {
		if p == nil {
			return nil
		}
		v := *p
		return &v
	}
	cloneBoolProp := func(p *oxml.CT_BooleanProperty) *oxml.CT_BooleanProperty {
		if p == nil {
			return nil
		}
		v := *p
		return &v
	}
	out := *f
	if f.Name != nil {
		v := *f.Name
		out.Name = &v
	}
	out.Charset = cloneIntProp(f.Charset)
	out.Family = cloneIntProp(f.Family)
	out.B = cloneBoolProp(f.B)
	out.I = cloneBoolProp(f.I)
	out.Strike = cloneBoolProp(f.Strike)
	out.Outline = cloneBoolProp(f.Outline)
	out.Shadow = cloneBoolProp(f.Shadow)
	out.Condense = cloneBoolProp(f.Condense)
	out.Extend = cloneBoolProp(f.Extend)
	out.Color = cloneStyleColor(f.Color)
	if f.Sz != nil {
		v := *f.Sz
		out.Sz = &v
	}
	if f.U != nil {
		v := *f.U
		out.U = &v
	}
	if f.VertAlign != nil {
		v := *f.VertAlign
		out.VertAlign = &v
	}
	if f.Scheme != nil {
		v := *f.Scheme
		out.Scheme = &v
	}
	return out
}

func cloneFill(f *oxml.CT_Fill) oxml.CT_Fill {
	var out oxml.CT_Fill
	if f.PatternFill != nil {
		out.PatternFill = &oxml.CT_PatternFill{
			PatternType: f.PatternFill.PatternType,
			FgColor:     cloneStyleColor(f.PatternFill.FgColor),
			BgColor:     cloneStyleColor(f.PatternFill.BgColor),
		}
	}
	if f.GradientFill != nil {
		gf := oxml.CT_GradientFill{
			Type:   f.GradientFill.Type,
			Degree: cloneFloat64Ptr(f.GradientFill.Degree),
			Left:   cloneFloat64Ptr(f.GradientFill.Left),
			Right:  cloneFloat64Ptr(f.GradientFill.Right),
			Top:    cloneFloat64Ptr(f.GradientFill.Top),
			Bottom: cloneFloat64Ptr(f.GradientFill.Bottom),
		}
		for i := range f.GradientFill.Stop {
			st := &f.GradientFill.Stop[i]
			stop := oxml.CT_GradientStop{Position: st.Position}
			if c := cloneStyleColor(&st.Color); c != nil {
				stop.Color = *c
			}
			gf.Stop = append(gf.Stop, stop)
		}
		out.GradientFill = &gf
	}
	return out
}

func cloneBorder(b *oxml.CT_Border) oxml.CT_Border {
	clonePr := func(p *oxml.CT_BorderPr) *oxml.CT_BorderPr {
		if p == nil {
			return nil
		}
		return &oxml.CT_BorderPr{Style: p.Style, Color: cloneStyleColor(p.Color)}
	}
	return oxml.CT_Border{
		DiagonalUp:   cloneBoolPtr(b.DiagonalUp),
		DiagonalDown: cloneBoolPtr(b.DiagonalDown),
		Outline:      cloneBoolPtr(b.Outline),
		Start:        clonePr(b.Start),
		End:          clonePr(b.End),
		Left:         clonePr(b.Left),
		Right:        clonePr(b.Right),
		Top:          clonePr(b.Top),
		Bottom:       clonePr(b.Bottom),
		Diagonal:     clonePr(b.Diagonal),
		Vertical:     clonePr(b.Vertical),
		Horizontal:   clonePr(b.Horizontal),
	}
}

func cloneCellAlignment(a *oxml.CT_CellAlignment) *oxml.CT_CellAlignment {
	if a == nil {
		return nil
	}
	out := oxml.CT_CellAlignment{Horizontal: a.Horizontal, Vertical: a.Vertical}
	out.TextRotation = cloneUint32(a.TextRotation)
	out.WrapText = cloneBoolPtr(a.WrapText)
	out.Indent = cloneUint32(a.Indent)
	if a.RelativeIndent != nil {
		v := *a.RelativeIndent
		out.RelativeIndent = &v
	}
	out.JustifyLastLine = cloneBoolPtr(a.JustifyLastLine)
	out.ShrinkToFit = cloneBoolPtr(a.ShrinkToFit)
	out.ReadingOrder = cloneUint32(a.ReadingOrder)
	return &out
}

func cloneCellProtection(p *oxml.CT_CellProtection) *oxml.CT_CellProtection {
	if p == nil {
		return nil
	}
	return &oxml.CT_CellProtection{Locked: cloneBoolPtr(p.Locked), Hidden: cloneBoolPtr(p.Hidden)}
}

// normalizeHexColor ensures the hex color string is in AARRGGBB format.
// If a 6-char hex string is given, "FF" alpha is prepended.
func normalizeHexColor(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	hex = strings.ToUpper(hex)
	if len(hex) == 6 {
		return "FF" + hex
	}
	return hex
}

// stripAlphaFromRGB strips the alpha prefix from an AARRGGBB color.
func stripAlphaFromRGB(rgb string) string {
	rgb = strings.ToUpper(rgb)
	if len(rgb) == 8 {
		return rgb[2:]
	}
	return rgb
}
