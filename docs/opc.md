# Low-level package access (`opc`)

`opc` reads and writes Open Packaging Conventions packages: the zip container
under every `.pptx`, `.docx` and `.xlsx`, with its parts, content types and
relationships. Reach for it when a document model is the wrong tool: to list or
extract the parts of any Office file, to inspect relationships the format
packages do not model, to verify signatures, or to assemble a package yourself.
For editing documents, use [`pptx`](pptx.md), [`docx`](docx.md) or
[`xlsx`](xlsx.md); they read and write through `opc` and keep what they do not
model.

## Reading a package

`opc.OpenReader` opens a file (`opc.NewReader` takes an `io.ReaderAt`). Content
types and package relationships are parsed up front; parts are read on demand
through `File.Open` or `File.ReadAll`. Part names are absolute (`/word/document.xml`),
and `GetFile` accepts a relationship target as written:

```go
package main

import (
    "fmt"

    "github.com/mgilbir/spine/opc"
)

func main() {
    pkg, err := opc.OpenReader("report.docx")
    if err != nil {
        panic(err)
    }
    defer pkg.Close()

    // Every part, with its content type.
    for _, f := range pkg.Files {
        fmt.Println(f.Name, f.ContentType)
    }

    // The package's main part is the target of its officeDocument
    // relationship; read it whole.
    for _, rel := range pkg.GetRelationshipsByType(opc.RelTypeOfficeDocument) {
        main := pkg.GetFile(rel.Target)
        data, err := main.ReadAll()
        if err != nil {
            panic(err)
        }
        fmt.Printf("main part %s: %d bytes\n", main.Name, len(data))

        // A part's own relationships live in its .rels part.
        rels, err := pkg.GetPartRelationships(main.Name)
        if err != nil {
            panic(err)
        }
        fmt.Printf("%d relationships from it\n", len(rels))
    }

    if pkg.Properties != nil {
        fmt.Println("title:", pkg.Properties.Title)
    }
}
```

- `Reader.Files` lists every part with its `ContentType`; `Reader.Relationships`
  and `GetRelationshipsByType` cover the package-level relationships, and
  `GetPartRelationships` reads a part's own `.rels`.
- `Reader.Properties`, `ExtendedProperties` and `CustomProperties` are the
  parsed `docProps` parts, or nil when absent or unparseable.
- `ResolvePartName(base, target)` turns a relationship target into a part name.
- The open is bounded by the [resource limits](../README.md#resource-limits) and
  decrypts with `opc.WithPassword`; `Reader.VerifySignatures` checks package
  signatures. See [Encryption and signing](encryption-and-signing.md).
- A `Reader` is safe for concurrent part reads.

## Writing a package

`opc.NewWriter` streams parts into an `io.Writer` as they are added. Each part
needs its content type; `Close` then writes `[Content_Types].xml`, the package
relationships and the properties set on the `Writer`:

```go
package main

import (
    "os"

    "github.com/mgilbir/spine/opc"
)

func main() {
    f, err := os.Create("hello.docx")
    if err != nil {
        panic(err)
    }
    defer f.Close()

    w := opc.NewWriter(f)
    body := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
        `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
        `<w:body><w:p><w:r><w:t>Hello from opc</w:t></w:r></w:p></w:body></w:document>`)
    if err := w.WritePart("/word/document.xml",
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml", body); err != nil {
        w.Abort()
        panic(err)
    }
    // The package relationship that makes it a Word document.
    if _, err := w.AddRelationship(opc.RelTypeOfficeDocument, "word/document.xml", opc.TargetModeInternal); err != nil {
        w.Abort()
        panic(err)
    }
    w.Properties = &opc.CoreProperties{Title: "Written with opc"}
    // Close writes [Content_Types].xml, the package relationships and the
    // properties; output from a failed Close must be discarded.
    if err := w.Close(); err != nil {
        panic(err)
    }
}
```

- `WritePart` writes a part in one call; `CreatePart` returns an `io.Writer`
  for a large one, valid only until the next part is added, because the zip
  stream is sequential.
- `WritePartRelationships` writes a part's `.rels`; `AddRelationship` adds a
  package-level relationship.
- If writing fails part-way, call `Abort` rather than `Close`, and discard the
  output. A `Close` that returns an error leaves the package incomplete too.
- `SaveEncrypted` and `SignPackage` produce an encrypted or signed copy of a
  finished package; see [Encryption and signing](encryption-and-signing.md).

The package written above opens with `docx.Open` as a one-paragraph document.
