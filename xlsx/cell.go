package xlsx

import (
	"fmt"
	"math"
	"strconv"
	"time"

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
	case CellTypeBoolean:
		return c.Bool()
	case CellTypeFormula:
		// Return the cached value if available
		if c.cell.V != nil {
			return *c.cell.V
		}
		return c.Formula()
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
		return CellTypeNumber
	default:
		if c.cell.V == nil {
			return CellTypeEmpty
		}
		return CellTypeString
	}
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
		if c.cell.Is != nil && c.cell.Is.T != nil {
			return *c.cell.Is.T
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

// SetString sets the cell value to an inline string.
func (c *Cell) SetString(value string) {
	c.markSheetDirty()
	c.cell.T = "str"
	c.cell.V = &value
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
		c.clearFormula()
		return
	}
	c.setNumeric(strconv.FormatFloat(value, 'f', -1, 64))
}

// setNumeric writes a pre-formatted numeric literal to the cell.
func (c *Cell) setNumeric(v string) {
	c.cell.T = "n"
	c.cell.V = &v
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
	Format    string // number format
}

// FontStyle represents font styling.
type FontStyle struct {
	Name      string
	Size      float64
	Bold      bool
	Italic    bool
	Underline bool
	Color     string // hex color
}

// FillStyle represents fill styling.
type FillStyle struct {
	Pattern string
	FgColor string // hex color
	BgColor string // hex color
}

// BorderStyle represents border styling.
type BorderStyle struct {
	Left   *BorderSide
	Right  *BorderSide
	Top    *BorderSide
	Bottom *BorderSide
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
