package xlsx

import (
	"time"
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
	sheet     *Sheet
	ref       string
	value     interface{}
	cellType  CellType
	formula   string
	style     *CellStyle
}

// Ref returns the cell reference (e.g., "A1").
func (c *Cell) Ref() string {
	return c.ref
}

// Value returns the cell value.
func (c *Cell) Value() interface{} {
	return c.value
}

// SetValue sets the cell value.
func (c *Cell) SetValue(value interface{}) {
	c.value = value
	c.cellType = detectCellType(value)
}

// Type returns the cell type.
func (c *Cell) Type() CellType {
	return c.cellType
}

// String returns the cell value as a string.
func (c *Cell) String() string {
	if c.value == nil {
		return ""
	}
	switch v := c.value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

// SetString sets the cell value to a string.
func (c *Cell) SetString(value string) {
	c.value = value
	c.cellType = CellTypeString
}

// Float returns the cell value as a float64.
func (c *Cell) Float() float64 {
	if c.value == nil {
		return 0
	}
	switch v := c.value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

// SetFloat sets the cell value to a float64.
func (c *Cell) SetFloat(value float64) {
	c.value = value
	c.cellType = CellTypeNumber
}

// Int returns the cell value as an int.
func (c *Cell) Int() int {
	return int(c.Float())
}

// SetInt sets the cell value to an int.
func (c *Cell) SetInt(value int) {
	c.value = value
	c.cellType = CellTypeNumber
}

// Bool returns the cell value as a bool.
func (c *Cell) Bool() bool {
	if c.value == nil {
		return false
	}
	if v, ok := c.value.(bool); ok {
		return v
	}
	return false
}

// SetBool sets the cell value to a bool.
func (c *Cell) SetBool(value bool) {
	c.value = value
	c.cellType = CellTypeBoolean
}

// Time returns the cell value as a time.Time.
func (c *Cell) Time() time.Time {
	if c.value == nil {
		return time.Time{}
	}
	if v, ok := c.value.(time.Time); ok {
		return v
	}
	return time.Time{}
}

// SetTime sets the cell value to a time.Time.
func (c *Cell) SetTime(value time.Time) {
	c.value = value
	c.cellType = CellTypeDate
}

// Formula returns the cell formula.
func (c *Cell) Formula() string {
	return c.formula
}

// SetFormula sets the cell formula.
func (c *Cell) SetFormula(formula string) {
	c.formula = formula
	c.cellType = CellTypeFormula
}

// Style returns the cell style.
func (c *Cell) Style() *CellStyle {
	return c.style
}

// SetStyle sets the cell style.
func (c *Cell) SetStyle(style *CellStyle) {
	c.style = style
}

// IsEmpty returns true if the cell has no value.
func (c *Cell) IsEmpty() bool {
	return c.value == nil && c.formula == ""
}

// Clear clears the cell value and formula.
func (c *Cell) Clear() {
	c.value = nil
	c.formula = ""
	c.cellType = CellTypeEmpty
}

// detectCellType determines the cell type from a value.
func detectCellType(value interface{}) CellType {
	if value == nil {
		return CellTypeEmpty
	}
	switch value.(type) {
	case string:
		return CellTypeString
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return CellTypeNumber
	case bool:
		return CellTypeBoolean
	case time.Time:
		return CellTypeDate
	default:
		return CellTypeString
	}
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
