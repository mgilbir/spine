# Spine

A Go library for reading and writing Microsoft Office documents (PPTX, DOCX, XLSX) using the Open Packaging Conventions (OPC) standard.

## Features

- **OPC Package Support**: Low-level API for working with Open Packaging Convention packages
- **PowerPoint (PPTX)**: Create and modify PowerPoint presentations
  - Create presentations from scratch or from templates
  - Add, remove, and reorder slides
  - Work with slide masters and layouts
  - Add shapes, text, tables, and images
  - Support for placeholders and themes
- **Word (DOCX)**: Basic document structure (in development)
- **Excel (XLSX)**: Basic workbook structure (in development)

## Installation

```bash
go get github.com/mgilbir/spine
```

## Quick Start

### Creating a PowerPoint Presentation

```go
package main

import (
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
    textBox.SetPosition(pptx.Inches(1), pptx.Inches(1))
    textBox.SetSize(pptx.Inches(8), pptx.Inches(1))
    textBox.SetText("Hello, World!")

    // Save the presentation
    if err := p.Save("presentation.pptx"); err != nil {
        panic(err)
    }
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
    slide := p.AddSlideFromLayout(layout)

    // Save the new presentation
    if err := p.Save("from_template.pptx"); err != nil {
        panic(err)
    }
}
```

## Package Structure

- `opc/` - Open Packaging Conventions implementation
- `common/` - Shared types and utilities
  - `dml/` - DrawingML types (colors, geometry)
  - `enum/` - Common enumerations
  - `xml/` - XML namespace handling
- `pptx/` - PowerPoint document support
- `docx/` - Word document support (in development)
- `xlsx/` - Excel document support (in development)

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

## Requirements

- Go 1.21 or later

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.
