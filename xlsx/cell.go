package xlsx

import (
	"fmt"
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
		c.SetFloat(float64(v))
	case uint:
		c.SetFloat(float64(v))
	case uint8:
		c.SetInt(int(v))
	case uint16:
		c.SetInt(int(v))
	case uint32:
		c.SetFloat(float64(v))
	case uint64:
		c.SetFloat(float64(v))
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
	c.cell.T = "str"
	c.cell.V = &value
	c.cell.F = nil
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

// SetFloat sets the cell value to a float64.
func (c *Cell) SetFloat(value float64) {
	c.cell.T = "n"
	v := strconv.FormatFloat(value, 'f', -1, 64)
	c.cell.V = &v
	c.cell.F = nil
}

// Int returns the cell value as an int.
func (c *Cell) Int() int {
	return int(c.Float())
}

// SetInt sets the cell value to an int.
func (c *Cell) SetInt(value int) {
	c.cell.T = "n"
	v := strconv.Itoa(value)
	c.cell.V = &v
	c.cell.F = nil
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
	c.cell.T = "b"
	v := "0"
	if value {
		v = "1"
	}
	c.cell.V = &v
	c.cell.F = nil
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

// SetTime sets the cell value to a time.Time.
func (c *Cell) SetTime(value time.Time) {
	c.cell.T = "n"
	v := strconv.FormatFloat(timeToExcelDate(value), 'f', -1, 64)
	c.cell.V = &v
	c.cell.F = nil
}

// Formula returns the cell formula.
func (c *Cell) Formula() string {
	if c.cell.F != nil {
		return c.cell.F.Value
	}
	return ""
}

// SetFormula sets the cell formula.
func (c *Cell) SetFormula(formula string) {
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
	c.cell.S = &index
}

// IsEmpty returns true if the cell has no value.
func (c *Cell) IsEmpty() bool {
	return c.cell.V == nil && c.cell.F == nil && c.cell.Is == nil
}

// Clear clears the cell value and formula.
func (c *Cell) Clear() {
	c.cell.V = nil
	c.cell.F = nil
	c.cell.T = ""
	c.cell.Is = nil
}

// excelDateToTime converts an Excel serial date to time.Time.
// Excel uses a base date of January 1, 1900.
func excelDateToTime(serial float64) time.Time {
	// Excel incorrectly considers 1900 to be a leap year,
	// so dates after February 28, 1900 are off by one day.
	if serial > 59 {
		serial--
	}
	// Serial 1 = January 1, 1900
	days := int(serial) - 1
	fraction := serial - float64(int(serial))

	base := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	t := base.AddDate(0, 0, days)
	// Add fractional day
	t = t.Add(time.Duration(fraction * 24 * float64(time.Hour)))
	return t
}

// timeToExcelDate converts a time.Time to an Excel serial date.
func timeToExcelDate(t time.Time) float64 {
	base := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	days := t.Sub(base).Hours() / 24
	serial := days + 1 // Serial 1 = January 1, 1900

	// Excel incorrectly considers 1900 to be a leap year
	if serial > 59 {
		serial++
	}
	return serial
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
