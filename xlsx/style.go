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
	sm.markModified()
	ss := sm.stylesheet

	// Resolve font
	fontID := uint32(0) // default font
	if style.Font != nil {
		font := fontStyleToOxml(style.Font)
		fontID = sm.findOrAddFont(font)
	}

	// Resolve fill
	fillID := uint32(0) // "none" fill
	if style.Fill != nil {
		fill := fillStyleToOxml(style.Fill)
		fillID = sm.findOrAddFill(fill)
	}

	// Resolve border
	borderID := uint32(0) // empty border
	if style.Border != nil {
		border := borderStyleToOxml(style.Border)
		borderID = sm.findOrAddBorder(border)
	}

	// Resolve number format: a format code string wins over a raw id.
	numFmtID := uint32(0)
	switch {
	case style.Format != "":
		numFmtID = sm.resolveNumberFormat(style.Format)
	case style.NumberFormatID != 0:
		numFmtID = uint32(style.NumberFormatID)
	}

	// Build the Xf record
	xfID := uint32(0)
	xf := oxml.CT_Xf{
		NumFmtId: &numFmtID,
		FontId:   &fontID,
		FillId:   &fillID,
		BorderId: &borderID,
		XfId:     &xfID,
	}

	if style.Font != nil {
		t := true
		xf.ApplyFont = &t
	}
	if style.Fill != nil {
		t := true
		xf.ApplyFill = &t
	}
	if style.Border != nil {
		t := true
		xf.ApplyBorder = &t
	}
	if style.Format != "" || style.NumberFormatID != 0 {
		t := true
		xf.ApplyNumberFormat = &t
	}
	if style.Alignment != nil {
		t := true
		xf.ApplyAlignment = &t
		xf.Alignment = alignmentStyleToOxml(style.Alignment)
	}

	// De-duplicate: check if an identical xf already exists
	if ss.CellXfs != nil {
		for i, existing := range ss.CellXfs.Xf {
			if xfEqual(&existing, &xf) {
				return uint32(i), nil
			}
		}
	}

	// Add new xf
	if ss.CellXfs == nil {
		ss.CellXfs = &oxml.CT_CellXfs{}
	}
	ss.CellXfs.Xf = append(ss.CellXfs.Xf, xf)
	idx := uint32(len(ss.CellXfs.Xf) - 1)
	count := uint32(len(ss.CellXfs.Xf))
	ss.CellXfs.Count = &count

	return idx, nil
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

// GetCellStyle returns the CellStyle for the given style index.
func (sm *StyleManager) GetCellStyle(index uint32) (CellStyle, error) {
	ss := sm.stylesheet

	if ss.CellXfs == nil || int(index) >= len(ss.CellXfs.Xf) {
		return CellStyle{}, fmt.Errorf("xlsx: style index %d out of range", index)
	}

	xf := &ss.CellXfs.Xf[index]
	var style CellStyle

	// Font
	if xf.FontId != nil && ss.Fonts != nil && int(*xf.FontId) < len(ss.Fonts.Font) {
		style.Font = oxmlToFontStyle(&ss.Fonts.Font[*xf.FontId])
	}

	// Fill
	if xf.FillId != nil && ss.Fills != nil && int(*xf.FillId) < len(ss.Fills.Fill) {
		style.Fill = oxmlToFillStyle(&ss.Fills.Fill[*xf.FillId])
	}

	// Border
	if xf.BorderId != nil && ss.Borders != nil && int(*xf.BorderId) < len(ss.Borders.Border) {
		style.Border = oxmlToBorderStyle(&ss.Borders.Border[*xf.BorderId])
	}

	// Number format
	if xf.NumFmtId != nil && *xf.NumFmtId != 0 {
		style.NumberFormatID = int(*xf.NumFmtId)
		style.Format = sm.resolveNumFmtCode(*xf.NumFmtId)
	}

	// Alignment
	if xf.Alignment != nil {
		style.Alignment = oxmlToAlignmentStyle(xf.Alignment)
	}

	return style, nil
}

// AddNumberFormat registers a custom number format string and returns its ID.
// If the format string matches a built-in format, the built-in ID is returned.
func (sm *StyleManager) AddNumberFormat(code string) uint32 {
	sm.markModified()
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
	// Return nil if no sides are set
	if bs.Left == nil && bs.Right == nil && bs.Top == nil && bs.Bottom == nil {
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
	// Return nil if empty
	if as.Horizontal == "" && as.Vertical == "" && !as.WrapText && as.Indent == 0 && as.Rotation == 0 {
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
	return true
}

func borderEqual(a, b *oxml.CT_Border) bool {
	return borderPrEqual(a.Left, b.Left) &&
		borderPrEqual(a.Right, b.Right) &&
		borderPrEqual(a.Top, b.Top) &&
		borderPrEqual(a.Bottom, b.Bottom)
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
		ptrUint32Equal(a.TextRotation, b.TextRotation)
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
