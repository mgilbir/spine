# Spine

A Go library for reading and writing Microsoft Office documents (PPTX, DOCX, XLSX) using the Open Packaging Conventions (OPC) standard.

## Features

- **OPC Package Support**: Low-level API for working with Open Packaging Convention packages
- **Round-Trip Preservation**: Byte-identical round-trip fidelity for unmodified parts across all formats
- **PowerPoint (PPTX)**: Create and modify PowerPoint presentations
  - Create presentations from scratch or from templates
  - Add, remove, and reorder slides
  - Work with slide masters and layouts
  - Add shapes, text, tables, and images
  - Auto shapes with solid/gradient fills, lines, and shadows
  - Slide transitions (fade, push, wipe, and more)
  - Support for placeholders and themes
- **Word (DOCX)**: Create and modify Word documents
  - Create documents from scratch
  - Add headings, paragraphs, and tables
  - Rich text formatting (bold, italic, color, font, size)
  - Paragraph alignment, spacing, and indentation
  - Bullet and numbered lists
  - Headers and footers
  - Inline images
  - Page setup (size, margins)
- **Excel (XLSX)**: Create and modify Excel workbooks
  - Create workbooks with multiple sheets
  - Read and write cell values (strings, numbers, booleans)
  - Formula support
  - Cell styling (fonts, fills, borders, number formats, alignment)
  - Freeze panes, auto-filter, and data validation
  - Merged cells and named ranges
  - Column widths and row heights

## Installation

```bash
go get github.com/mgilbir/spine
```

## Quick Start

### Creating a PowerPoint Presentation

```go
package main

import (
    "github.com/mgilbir/spine/common/dml"
    "github.com/mgilbir/spine/pptx"
)

func main() {
    // Create a new presentation
    p := pptx.Create()
    p.Properties.Title = "My Presentation"
    p.Properties.Creator = "Author Name"

    // Add a slide
    slide := p.AddSlide()
    slide.SetName("Introduction")

    // Add a text box
    textBox := slide.AddTextBox()
    textBox.SetPosition(dml.Inches(1), dml.Inches(1))
    textBox.SetSize(dml.Inches(8), dml.Inches(1))
    textBox.SetText("Hello, World!")

    // Save the presentation
    if err := p.Save("presentation.pptx"); err != nil {
        panic(err)
    }
}
```

### Creating a Word Document

```go
package main

import (
    "github.com/mgilbir/spine/docx"
)

func main() {
    // Create a new document
    doc := docx.Create()
    doc.Properties.Title = "My Document"

    // Add a heading
    doc.AddHeading("Welcome", 1)

    // Add a paragraph
    doc.AddParagraphWithText("This is a simple document created with Spine.")

    // Add formatted text
    p := doc.AddParagraph()
    bold := p.AddRun()
    bold.SetText("Bold text")
    bold.SetBold(true)

    // Save the document
    if err := doc.Save("document.docx"); err != nil {
        panic(err)
    }
}
```

### Creating an Excel Spreadsheet

```go
package main

import (
    "fmt"
    "github.com/mgilbir/spine/xlsx"
)

func main() {
    // Create a new workbook
    wb := xlsx.Create()

    // Add a sheet
    sheet := wb.AddSheet("Sales")

    // Set cell values
    sheet.SetCellValue("A1", "Product")
    sheet.SetCellValue("B1", "Revenue")
    sheet.SetCellValue("A2", "Widgets")
    sheet.SetCellValue("B2", 1500.0)
    sheet.SetCellValue("A3", "Gadgets")
    sheet.SetCellValue("B3", 3200.0)

    // Add a formula
    cell, _ := sheet.Cell("B4")
    cell.SetFormula("SUM(B2:B3)")

    // Save the workbook
    if err := wb.Save("spreadsheet.xlsx"); err != nil {
        panic(err)
    }

    fmt.Printf("Sheets: %d\n", wb.SheetCount())
}
```

### Writing an XLSX Workbook to Memory

```go
package main

import (
    "bytes"
    "github.com/mgilbir/spine/xlsx"
)

func main() {
    wb := xlsx.Create()
    sheet := wb.AddSheet("Export")
    sheet.SetCellValue("A1", "hello")

    buf, err := wb.WriteToBuffer()
    if err != nil {
        panic(err)
    }

    _ = bytes.NewReader(buf.Bytes())
}
```

### Opening an XLSX Workbook from Memory

```go
package main

import (
    "bytes"
    "github.com/mgilbir/spine/xlsx"
)

func main() {
    data := []byte{ /* xlsx bytes */ }

    wb, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
    if err != nil {
        panic(err)
    }
    defer wb.Close()
}
```

### Opening and Modifying a Presentation

```go
package main

import (
    "github.com/mgilbir/spine/pptx"
)

func main() {
    // Open an existing presentation
    p, err := pptx.Open("existing.pptx")
    if err != nil {
        panic(err)
    }
    defer p.Close()

    // Modify properties
    p.Properties.Title = "Updated Title"

    // Add a new slide
    p.AddSlide()

    // Save to a new file
    if err := p.SaveAs("modified.pptx"); err != nil {
        panic(err)
    }
}
```

### Creating from a Template

```go
package main

import (
    "github.com/mgilbir/spine/pptx"
)

func main() {
    // Create a new presentation based on a template
    // This preserves the template's masters, layouts, and themes
    p, err := pptx.CreateFromTemplate("template.pptx")
    if err != nil {
        panic(err)
    }
    defer p.Close()

    // Add slides using template layouts
    layout := p.GetLayoutByType(pptx.LayoutTitleAndContent)
    _ = p.AddSlideFromLayout(layout)

    // Save the new presentation
    if err := p.Save("from_template.pptx"); err != nil {
        panic(err)
    }
}
```

## Package Structure

- `opc/` - Open Packaging Conventions implementation
- `common/` - Shared types and utilities
  - `dml/` - DrawingML types (colors, geometry, fills, lines)
    - `chart/` - Chart types
    - `diagram/` - Diagram types
  - `docprops/` - Document properties (core and extended)
  - `enum/` - Common enumerations
  - `omml/` - Office Math Markup Language types
  - `oxml/` - Shared Office XML types
  - `vml/` - Vector Markup Language types
  - `xml/` - XML namespace handling and Builder-based serialization
- `pptx/` - PowerPoint document support
- `docx/` - Word document support
- `xlsx/` - Excel document support

## Units

The library uses EMUs (English Metric Units) internally, which is the standard unit in Office Open XML. Helper functions are provided for common conversions:

```go
import "github.com/mgilbir/spine/common/dml"

// Convert from inches to EMUs
width := dml.Inches(10.5)

// Convert from centimeters to EMUs
height := dml.Centimeters(5.0)

// Convert from points to EMUs
fontSize := dml.Points(12)
```

## Slide Layouts

The following standard slide layout types are supported:

| Type | Description |
|------|-------------|
| `LayoutTitle` | Title slide |
| `LayoutTitleAndContent` | Title and content |
| `LayoutSectionHeader` | Section header |
| `LayoutTwoContent` | Two content areas |
| `LayoutComparison` | Comparison layout |
| `LayoutTitleOnly` | Title only |
| `LayoutBlank` | Blank slide |
| `LayoutContentWithCaption` | Content with caption |
| `LayoutPictureWithCaption` | Picture with caption |
| `LayoutTitleAndVerticalText` | Title and vertical text |
| `LayoutVerticalTitleAndText` | Vertical title and text |

## Testing

Unit tests run against both small synthetic fixtures (committed to git) and a set of real-world Office files sourced from the internet. The external files are used for round-trip compatibility testing: each file is parsed, serialized back, and compared byte-for-byte against the original.

External fixtures are not checked into the repository. To download them:

```bash
make fetch
```

This reads `testdata/external.txt` (a list of destination paths and URLs) and downloads any missing files. Use `bash testdata/fetch.sh --force` to re-download all of them.

To run the full test suite (fetches external files first):

```bash
make test
```

To lint:

```bash
make lint
```

## Requirements

- Go 1.25 or later

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.
