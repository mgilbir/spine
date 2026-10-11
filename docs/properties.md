# Document properties

Core (`docProps/core.xml`), extended/app, and custom (`docProps/custom.xml`)
properties are supported across all three formats. `CustomProperties()`,
`SetCustomProperty(name, value)`, and `RemoveCustomProperty(name)` on
`docx.Document`, `xlsx.Workbook`, and `pptx.Presentation` read and write
user-defined properties typed as string, int64, float64, bool, or time.Time; the
part, its content type, and package relationship are created on demand, and
existing properties round-trip byte-identically when untouched.

The API is the same in every format:

- **Core properties** are the `Properties` field, an `opc.CoreProperties`
  (`Title`, `Subject`, `Creator`, `Keywords`, `Description`, `LastModifiedBy`,
  `Category`, `Created`, `Modified`, and the rest). Assign a field and save.
- **Custom properties** go through the three methods above.
  `SetCustomProperty` also accepts int, int32, and float32, stored as int64 and
  float64. `CustomProperties()` returns a copy, or nil when there are none, so
  change values through the setters rather than the map.
  `RemoveCustomProperty` reports whether the name existed.
- **Extended properties** (`docProps/app.xml`: application, word and slide
  counts, and so on) have no accessor on the document types. To read them, open
  the file with `opc.OpenReader` and use `Reader.ExtendedProperties`, which is
  nil when the package has none.

When a session changes a document's content, the save records the time in
`Properties.Modified`, and only then, so saving an unchanged document twice
gives identical bytes. A value you assign to `Properties.Modified` yourself
takes precedence over that stamp.

## Word

```go
package main

import (
    "fmt"
    "time"

    "github.com/mgilbir/spine/docx"
)

func main() {
    doc := docx.Create()
    doc.AddParagraphWithText("Quarterly report")

    doc.Properties.Title = "Q3 report"
    doc.Properties.Creator = "Finance"
    if err := doc.SetCustomProperty("Department", "Finance"); err != nil {
        panic(err)
    }
    if err := doc.SetCustomProperty("Reviewed", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); err != nil {
        panic(err)
    }
    if err := doc.Save("report.docx"); err != nil {
        panic(err)
    }

    opened, err := docx.Open("report.docx")
    if err != nil {
        panic(err)
    }
    defer opened.Close()
    fmt.Println(opened.Properties.Title, opened.Properties.Creator)
    for name, value := range opened.CustomProperties() {
        fmt.Printf("%s = %v (%T)\n", name, value, value)
    }
}
```

## Excel

```go
package main

import (
    "fmt"

    "github.com/mgilbir/spine/xlsx"
)

func main() {
    wb := xlsx.Create()
    if _, err := wb.AddSheet("Data"); err != nil {
        panic(err)
    }

    wb.Properties.Title = "Inventory"
    if err := wb.SetCustomProperty("Rows", 1200); err != nil { // stored as int64
        panic(err)
    }
    if err := wb.SetCustomProperty("Draft", true); err != nil {
        panic(err)
    }
    if err := wb.Save("inventory.xlsx"); err != nil {
        panic(err)
    }

    opened, err := xlsx.Open("inventory.xlsx")
    if err != nil {
        panic(err)
    }
    defer opened.Close()
    fmt.Println(opened.Properties.Title)
    fmt.Println(opened.CustomProperties()["Rows"]) // int64(1200)

    // Remove one and save a copy.
    opened.RemoveCustomProperty("Draft")
    if err := opened.Save("inventory-final.xlsx"); err != nil {
        panic(err)
    }
}
```

## PowerPoint

```go
package main

import (
    "fmt"

    "github.com/mgilbir/spine/pptx"
)

func main() {
    p := pptx.Create()
    p.AddSlide()

    p.Properties.Title = "Kick-off"
    p.Properties.Keywords = "planning; 2027"
    if err := p.SetCustomProperty("Version", 2.1); err != nil {
        panic(err)
    }
    if err := p.Save("kickoff.pptx"); err != nil {
        panic(err)
    }

    opened, err := pptx.Open("kickoff.pptx")
    if err != nil {
        panic(err)
    }
    defer opened.Close()
    fmt.Println(opened.Properties.Title, opened.Properties.Keywords)
    fmt.Println(opened.CustomProperties()["Version"]) // float64(2.1)
}
```
