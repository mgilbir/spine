package xlsx

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// CellType represents the type of value in a cell.
type CellType int

const (
	CellTypeEmpty CellType = iota
	CellTypeString
	CellTypeNumber
	CellTypeBoolean
	CellTypeFormula
	CellTypeError
	CellTypeDate
)

// Cell represents a cell in a worksheet.
type Cell struct {
	sheet *Sheet
	cell  *oxml.CT_Cell
}

// Ref returns the cell reference (e.g., "A1").
func (c *Cell) Ref() string {
	return c.cell.R
}

// Value returns the cell value as an interface{}.
func (c *Cell) Value() interface{} {
	switch c.Type() {
	case CellTypeString:
		return c.String()
	case CellTypeNumber:
		return c.Float()
	case CellTypeDate:
		return c.Time()
	case CellTypeBoolean:
		return c.Bool()
	case CellTypeFormula:
		// Return the cached formula result typed by its cached-value type
		// (c.cell.T), the same way a literal cell of that type reads back: a
		// numeric result (t="n" or absent) yields float64, t="b" yields bool,
		// and a string/error result yields string. Without this a numeric
		// formula like =1+1 would read back as the string "2". When no cached
		// value is present, fall back to the formula text.
		if c.cell.V == nil {
			return c.Formula()
		}
		switch c.cell.T {
		case "n", "":
			return c.Float()
		case "b":
			return c.Bool()
		case "s", "str", "inlineStr":
			return c.String()
		default:
			return *c.cell.V
		}
	default:
		return nil
	}
}

// SetValue sets the cell value, automatically detecting the type.
func (c *Cell) SetValue(value interface{}) {
	c.markSheetDirty()
	if value == nil {
		c.Clear()
		return
	}

	switch v := value.(type) {
	case string:
		c.SetString(v)
	case int:
		c.SetInt(v)
	case int8:
		c.SetInt(int(v))
	case int16:
		c.SetInt(int(v))
	case int32:
		c.SetInt(int(v))
	case int64:
		c.SetInt64(v)
	case uint:
		c.SetUint64(uint64(v))
	case uint8:
		c.SetInt(int(v))
	case uint16:
		c.SetInt(int(v))
	case uint32:
		c.SetUint64(uint64(v))
	case uint64:
		c.SetUint64(v)
	case float32:
		c.SetFloat(float64(v))
	case float64:
		c.SetFloat(v)
	case bool:
		c.SetBool(v)
	case time.Time:
		c.SetTime(v)
	default:
		c.SetString(fmt.Sprint(v))
	}
}

// Type returns the cell type.
func (c *Cell) Type() CellType {
	if c.cell.F != nil {
		return CellTypeFormula
	}

	switch c.cell.T {
	case "s":
		return CellTypeString
	case "str":
		return CellTypeString
	case "inlineStr":
		return CellTypeString
	case "b":
		return CellTypeBoolean
	case "e":
		return CellTypeError
	case "n", "":
		if c.cell.V == nil {
			return CellTypeEmpty
		}
		if c.hasDateNumberFormat() {
			return CellTypeDate
		}
		return CellTypeNumber
	default:
		if c.cell.V == nil {
			return CellTypeEmpty
		}
		return CellTypeString
	}
}

// hasDateNumberFormat reports whether the cell's style applies one of the
// built-in date/time number formats (ids 14-22 and 45-47), which is how Excel
// distinguishes date cells from plain numbers (C132).
func (c *Cell) hasDateNumberFormat() bool {
	if c.cell.S == nil || c.sheet == nil || c.sheet.workbook == nil {
		return false
	}
	ss := c.sheet.workbook.stylesheet
	if ss == nil || ss.CellXfs == nil {
		return false
	}
	idx := int(*c.cell.S)
	if idx < 0 || idx >= len(ss.CellXfs.Xf) {
		return false
	}
	xf := &ss.CellXfs.Xf[idx]
	if xf.NumFmtId == nil {
		return false
	}
	id := *xf.NumFmtId
	return (id >= 14 && id <= 22) || (id >= 45 && id <= 47)
}

// String returns the cell value as a string.
func (c *Cell) String() string {
	switch c.cell.T {
	case "s":
		// Shared string: V contains the index
		if c.cell.V != nil && c.sheet != nil && c.sheet.workbook != nil {
			idx, err := strconv.Atoi(*c.cell.V)
			if err == nil {
				return c.sheet.workbook.resolveSharedString(idx)
			}
		}
		return ""
	case "inlineStr":
		if c.cell.Is == nil {
			return ""
		}
		if c.cell.Is.T != nil {
			return *c.cell.Is.T
		}
		// Rich inline string: concatenate the run texts.
		if len(c.cell.Is.R) > 0 {
			var sb strings.Builder
			for i := range c.cell.Is.R {
				sb.WriteString(c.cell.Is.R[i].T)
			}
			return sb.String()
		}
		return ""
	case "str":
		if c.cell.V != nil {
			return *c.cell.V
		}
		return ""
	default:
		if c.cell.V != nil {
			return *c.cell.V
		}
		return ""
	}
}

// SetString sets the cell value to a literal string, stored as an inline
// string (t="inlineStr"). The previous encoding t="str" is the CACHED
// FORMULA RESULT type and is not valid for literal strings (C129). Strings
// with leading or trailing whitespace are marshaled with
// xml:space="preserve" so the spaces survive an Excel round-trip.
func (c *Cell) SetString(value string) {
	c.markSheetDirty()
	c.cell.T = "inlineStr"
	c.cell.V = nil
	c.cell.Is = &oxml.CT_Rst{T: &value}
	c.clearFormula()
}

// Float returns the cell value as a float64.
func (c *Cell) Float() float64 {
	if c.cell.V == nil {
		return 0
	}
	f, err := strconv.ParseFloat(*c.cell.V, 64)
	if err != nil {
		return 0
	}
	return f
}

// SetFloat sets the cell value to a float64. NaN and ±Inf are not
// representable as a numeric cell, so they are written as a #NUM! error cell
// rather than an invalid <v>NaN</v>.
func (c *Cell) SetFloat(value float64) {
	c.markSheetDirty()
	if math.IsNaN(value) || math.IsInf(value, 0) {
		c.cell.T = "e"
		v := "#NUM!"
		c.cell.V = &v
		c.cell.Is = nil
		c.clearFormula()
		return
	}
	c.setNumeric(strconv.FormatFloat(value, 'f', -1, 64))
}

// setNumeric writes a pre-formatted numeric literal to the cell.
func (c *Cell) setNumeric(v string) {
	c.cell.T = "n"
	c.cell.V = &v
	c.cell.Is = nil
	c.clearFormula()
}

// Int returns the cell value as an int.
func (c *Cell) Int() int {
	return int(c.Float())
}

// SetInt sets the cell value to an int.
func (c *Cell) SetInt(value int) {
	c.markSheetDirty()
	c.setNumeric(strconv.Itoa(value))
}

// SetInt64 sets the cell value to an int64, formatting it exactly rather than
// routing through float64 (which loses precision above 2^53).
func (c *Cell) SetInt64(value int64) {
	c.markSheetDirty()
	c.setNumeric(strconv.FormatInt(value, 10))
}

// SetUint64 sets the cell value to a uint64, formatting it exactly.
func (c *Cell) SetUint64(value uint64) {
	c.markSheetDirty()
	c.setNumeric(strconv.FormatUint(value, 10))
}

// Bool returns the cell value as a bool.
func (c *Cell) Bool() bool {
	if c.cell.V == nil {
		return false
	}
	return *c.cell.V == "1" || *c.cell.V == "true"
}

// SetBool sets the cell value to a bool.
func (c *Cell) SetBool(value bool) {
	c.markSheetDirty()
	c.cell.T = "b"
	v := "0"
	if value {
		v = "1"
	}
	c.cell.V = &v
	c.cell.Is = nil
	c.clearFormula()
}

// Time returns the cell value as a time.Time.
// Excel stores dates as serial numbers (days since January 1, 1900).
func (c *Cell) Time() time.Time {
	if c.cell.V == nil {
		return time.Time{}
	}
	f, err := strconv.ParseFloat(*c.cell.V, 64)
	if err != nil {
		return time.Time{}
	}
	return excelDateToTime(f)
}

// SetTime sets the cell value to a time.Time, stored as an Excel serial date.
// The serial is computed from the wall-clock date/time in the value's own
// location, so the stored day does not shift by the zone offset.
//
// Note: this sets only the numeric value, not a date number format, so the cell
// displays the raw serial number until a date format is applied via SetStyle.
func (c *Cell) SetTime(value time.Time) {
	c.markSheetDirty()
	c.setNumeric(strconv.FormatFloat(timeToExcelDate(value), 'f', -1, 64))
}

// Formula returns the cell formula.
func (c *Cell) Formula() string {
	if c.cell.F != nil {
		return c.cell.F.Value
	}
	return ""
}

// SetFormula sets the cell formula. If the cell was the master of a shared
// formula group, the group's followers are first converted to plain formulas
// (see clearFormula) so replacing the master does not orphan them.
func (c *Cell) SetFormula(formula string) {
	c.markSheetDirty()
	c.detachSharedGroup()
	c.cell.T = ""
	c.cell.F = &oxml.CT_CellFormula{Value: formula}
	c.cell.V = nil
	c.cell.Is = nil
}

// SetArrayFormula sets the cell to a legacy (Ctrl+Shift+Enter) array formula
// spilling over ref, the range the formula fills — e.g.
// SetArrayFormula("A1:A3*B1:B3", "C1:C3"). This cell becomes the array master
// (`<f t="array" ref="C1:C3">`); Excel fills the other cells of ref when it
// recalculates. If this cell was the master of a shared-formula group its
// followers are first detached (see clearFormula) so they are not orphaned.
func (c *Cell) SetArrayFormula(formula, ref string) {
	c.markSheetDirty()
	c.detachSharedGroup()
	c.cell.T = ""
	c.cell.F = &oxml.CT_CellFormula{T: "array", Ref: ref, Value: formula}
	c.cell.V = nil
	c.cell.Is = nil
}

// SetDynamicArrayFormula sets the cell to a dynamic-array (spill) formula, the
// modern spilling form Excel writes for functions such as SORT, FILTER and
// UNIQUE. ref is the anchor cell (usually this cell's own reference); Excel
// grows the spill range from the anchor as the result changes. The formula is
// stored as `<f t="array" ref="…" aca="1" ca="1">`, the alwaysCalcArray /
// calculateCell marking Excel uses for a dynamic array.
//
// Note: the cell-metadata linkage Excel adds for a dynamic array (a `cm`
// attribute pointing into xl/metadata.xml) is not synthesized here; Excel still
// evaluates the formula as a spilling array and rewrites the metadata itself on
// the next save.
func (c *Cell) SetDynamicArrayFormula(formula, ref string) {
	c.markSheetDirty()
	c.detachSharedGroup()
	if ref == "" {
		ref = c.cell.R
	}
	on := true
	c.cell.T = ""
	c.cell.F = &oxml.CT_CellFormula{T: "array", Ref: ref, Aca: &on, Ca: &on, Value: formula}
	c.cell.V = nil
	c.cell.Is = nil
}

// SetSharedFormula sets the cell to the master of a shared-formula group
// spanning ref, then fills every other cell of ref with a follower stub
// (`<f t="shared" si="N"/>`) that shares this master's index. This is the
// compact encoding Excel uses when one formula is copied down or across a
// range: only the master carries the formula text, and Excel derives each
// follower by translating the master's relative references by the follower's
// offset.
//
// This cell must be the top-left (anchor) cell of ref, matching Excel's
// requirement that the master anchor the group; ref is returned unchanged as
// the master's ref attribute. A fresh, unused shared index is allocated for the
// group. If this cell was already a shared-formula master its old followers are
// detached first (see clearFormula).
func (c *Cell) SetSharedFormula(formula, ref string) error {
	if c.sheet == nil {
		return fmt.Errorf("xlsx: cell is not associated with a sheet")
	}
	rng, err := parseCellRangeRef(ref)
	if err != nil {
		return fmt.Errorf("xlsx: SetSharedFormula: %w", err)
	}
	mRow, mCol, err := ParseCellRef(c.cell.R)
	if err != nil {
		return fmt.Errorf("xlsx: SetSharedFormula: %w", err)
	}
	if mRow != rng.minRow || mCol != rng.minCol {
		return fmt.Errorf("xlsx: SetSharedFormula: cell %s must be the top-left cell of range %s", c.cell.R, ref)
	}

	si := c.sheet.nextSharedFormulaSi()
	c.markSheetDirty()
	c.detachSharedGroup()
	siCopy := si
	c.cell.T = ""
	c.cell.F = &oxml.CT_CellFormula{T: "shared", Ref: ref, Si: &siCopy, Value: formula}
	c.cell.V = nil
	c.cell.Is = nil

	for row := rng.minRow; row <= rng.maxRow; row++ {
		for col := rng.minCol; col <= rng.maxCol; col++ {
			if row == mRow && col == mCol {
				continue
			}
			fref := FormatCellRef(row, col)
			follower, err := c.sheet.Cell(fref)
			if err != nil {
				return err
			}
			follower.detachSharedGroup()
			fsi := si
			follower.cell.T = ""
			follower.cell.F = &oxml.CT_CellFormula{T: "shared", Si: &fsi}
			follower.cell.V = nil
			follower.cell.Is = nil
		}
	}
	return nil
}

// Style returns the cell's style index, or nil if not set.
func (c *Cell) StyleIndex() *uint32 {
	return c.cell.S
}

// SetStyleIndex sets the cell's style index.
func (c *Cell) SetStyleIndex(index uint32) {
	c.markSheetDirty()
	c.cell.S = &index
}

// IsEmpty returns true if the cell has no value.
func (c *Cell) IsEmpty() bool {
	return c.cell.V == nil && c.cell.F == nil && c.cell.Is == nil
}

// Clear clears the cell value and formula.
func (c *Cell) Clear() {
	c.markSheetDirty()
	c.cell.V = nil
	c.clearFormula()
	c.cell.T = ""
	c.cell.Is = nil
}

// clearFormula removes the cell's formula. If the cell is the master of a
// shared-formula group (`<f t="shared" ref="..." si="N">`), the group's
// followers are first materialized as plain formulas: silently nilling the
// master would leave them as si-only stubs with no master anywhere, which is
// spec-invalid and triggers Excel's repair prompt (C176).
func (c *Cell) clearFormula() {
	c.detachSharedGroup()
	c.cell.F = nil
}

// detachSharedGroup materializes the followers of this cell's shared-formula
// group when the cell is the group's master. Overwriting a follower needs no
// handling: it just drops that follower's stub and the group stays intact.
func (c *Cell) detachSharedGroup() {
	f := c.cell.F
	if f == nil || f.T != "shared" || f.Ref == "" {
		return
	}
	if c.sheet != nil {
		c.sheet.materializeSharedGroup(c.cell)
	}
}

// excelEpoch is the base date of the Excel 1900 serial-date system. Serials
// count days from 1899-12-30 for dates on or after 1900-03-01, which accounts
// for Excel's fictitious 1900-02-29 leap day.
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// excelDateToTime converts an Excel serial date to a time.Time (in UTC).
func excelDateToTime(serial float64) time.Time {
	whole := math.Floor(serial)
	frac := serial - whole
	days := int(whole)
	// Dates before 1900-03-01 (serial < 61) predate the fictitious leap day, so
	// they map one day later than the raw offset from the epoch.
	if days < 61 {
		days++
	}
	t := excelEpoch.AddDate(0, 0, days)
	// Add the time-of-day fraction, rounded to the nearest second.
	t = t.Add(time.Duration(math.Round(frac*86400)) * time.Second)
	return t
}

// timeToExcelDate converts a time.Time to an Excel serial date. It uses the
// wall-clock calendar fields of t (in t's own location) so the serial is
// independent of the time-zone offset.
func timeToExcelDate(t time.Time) float64 {
	year, month, day := t.Date()
	dateOnly := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	days := int(math.Round(dateOnly.Sub(excelEpoch).Hours() / 24))
	// Inverse of the leap-day adjustment in excelDateToTime.
	if days < 61 {
		days--
	}
	hour, min, sec := t.Clock()
	frac := (float64(hour)*3600 + float64(min)*60 + float64(sec) + float64(t.Nanosecond())/1e9) / 86400
	return float64(days) + frac
}

// CellStyle represents the style of a cell.
type CellStyle struct {
	Font      *FontStyle
	Fill      *FillStyle
	Border    *BorderStyle
	Alignment *AlignmentStyle
	// Format is a number format code string (e.g. "0.00" or a custom code).
	// It takes precedence over NumberFormatID when both are set.
	Format string
	// NumberFormatID applies a number format by its id, letting callers use
	// the built-in NumberFormat* constants (e.g. NumberFormatDate) directly
	// without spelling out the format code (C131). Zero means General.
	NumberFormatID int
	// Protection controls the cell's locked/hidden flags. It only takes effect
	// once the sheet itself is protected (see Sheet.Protect); on an unprotected
	// sheet every cell is editable regardless of this setting. A nil value
	// leaves the format's protection unset (Excel then treats the cell as
	// locked by default).
	Protection *ProtectionStyle
}

// ProtectionStyle represents a cell format's protection flags (the
// <protection> child of an xf record). It is only meaningful on a protected
// sheet.
type ProtectionStyle struct {
	// Locked reports whether the cell is locked. Excel locks cells by default,
	// so set Locked=false to leave specific cells editable on a protected sheet.
	Locked bool
	// Hidden reports whether the cell's formula is hidden in the formula bar on
	// a protected sheet.
	Hidden bool
}

// UnderlineStyle names a font underline style (CT_UnderlineProperty val,
// ST_UnderlineValues). The string values are the SpreadsheetML tokens.
type UnderlineStyle string

const (
	UnderlineNone             UnderlineStyle = "none"
	UnderlineSingle           UnderlineStyle = "single"
	UnderlineDouble           UnderlineStyle = "double"
	UnderlineSingleAccounting UnderlineStyle = "singleAccounting"
	UnderlineDoubleAccounting UnderlineStyle = "doubleAccounting"
)

// FontStyle represents font styling.
type FontStyle struct {
	Name      string
	Size      float64
	Bold      bool
	Italic    bool
	Underline bool
	Color     string // hex color
	// Strike renders the text with a strikethrough (x:strike).
	Strike bool
	// UnderlineStyle selects a richer underline than the plain Underline bool
	// (e.g. UnderlineDouble, UnderlineSingleAccounting). When set it takes
	// precedence over Underline; the empty value leaves Underline in control.
	UnderlineStyle UnderlineStyle
	// VertAlign renders the text as superscript or subscript (x:vertAlign). The
	// empty value leaves the run on the baseline.
	VertAlign enum.VerticalAlignRun
}

// FillStyle represents fill styling. It carries either a pattern fill (the
// Pattern/FgColor/BgColor fields) or, when Gradient is set, a gradient fill.
// A non-nil Gradient takes precedence over the pattern fields.
type FillStyle struct {
	Pattern string
	FgColor string // hex color
	BgColor string // hex color
	// Gradient, when non-nil, renders a gradient fill instead of a pattern fill.
	Gradient *GradientFill
}

// GradientFill represents a gradient fill (CT_GradientFill). Type is "linear"
// (the default when empty) or "path". For a linear gradient Degree is the
// angle; for a path gradient Left/Right/Top/Bottom (0..1) locate the inner
// rectangle. Stops lists the color stops in ascending position order.
type GradientFill struct {
	Type   string // "linear" (default) or "path"
	Degree float64
	Left   float64
	Right  float64
	Top    float64
	Bottom float64
	Stops  []GradientStop
}

// GradientStop represents a single gradient color stop (CT_GradientStop).
type GradientStop struct {
	Position float64 // 0..1
	Color    string  // hex color
}

// BorderStyle represents border styling.
type BorderStyle struct {
	Left     *BorderSide
	Right    *BorderSide
	Top      *BorderSide
	Bottom   *BorderSide
	Diagonal *BorderSide
	// DiagonalUp / DiagonalDown select which diagonal(s) the Diagonal side is
	// drawn across (Excel's up/down diagonal border toggles).
	DiagonalUp   bool
	DiagonalDown bool
}

// BorderSide represents one side of a border.
type BorderSide struct {
	Style string
	Color string // hex color
}

// AlignmentStyle represents alignment styling.
type AlignmentStyle struct {
	Horizontal string
	Vertical   string
	WrapText   bool
	Indent     int
	Rotation   int
	// ShrinkToFit shrinks the displayed text so it fits within the cell.
	ShrinkToFit bool
	// JustifyLastLine justifies the final line of a justified paragraph.
	JustifyLastLine bool
	// ReadingOrder controls text direction: 0 context-dependent, 1 left-to-right,
	// 2 right-to-left.
	ReadingOrder uint32
	// RelativeIndent is the relative indent used by dxf (differential) records;
	// it may be negative.
	RelativeIndent int
}

// SetStyle creates a style from the given definition and applies it to the cell.
func (c *Cell) SetStyle(style CellStyle) error {
	c.markSheetDirty()
	if c.sheet == nil || c.sheet.workbook == nil {
		return fmt.Errorf("xlsx: cell is not associated with a workbook")
	}
	sm := c.sheet.workbook.Styles()
	idx, err := sm.NewCellStyle(style)
	if err != nil {
		return err
	}
	c.SetStyleIndex(idx)
	return nil
}

func (c *Cell) markSheetDirty() {
	if c != nil && c.sheet != nil {
		c.sheet.markDirty()
	}
}
