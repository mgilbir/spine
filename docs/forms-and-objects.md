# Forms and embedded objects

Form controls, ActiveX controls, embedded OLE objects, ink annotations, and 3D
models each span more than one format, so they share this guide rather than
living in one format's. Each section names the per-format entry points.

## Form controls and ActiveX

Read/enumerate interactive controls across formats — Word legacy form fields
(`docx.Document.FormFields()`: FORMTEXT/FORMCHECKBOX/FORMDROPDOWN with name,
value, checkbox state, and dropdown entries) with basic authoring via
`docx.Paragraph.AddFormField`; Excel form controls (`xlsx.Sheet.FormControls()`:
buttons, checkboxes, dropdowns, list boxes, option buttons, spinners, scroll
bars, each with its linked cell); and ActiveX controls (`ActiveXControls()` on
all three formats, reporting each control's COM class id, persistence mode, and
`activeXN.bin` binary). All control parts are preserved verbatim on save;
authoring ActiveX controls is out of scope.

Excel's own legacy form controls read back through `Sheet.FormControls()` —
buttons, checkboxes, dropdowns, list boxes, option buttons, spinners, and scroll
bars — reading each control's type, linked cell (`x:FmlaLink`), source range,
checkbox state, and its VML/`ctrlProps` parts; the control parts are preserved
verbatim on save.

## Embedded OLE objects

Read embedded OLE objects (`embeddings/oleObjectN.bin`) with `OLEObjects()`,
returning each object's name, content type, raw bytes, and best-effort ProgID;
embed new objects with `docx.Paragraph.AddOLEObject` or
`pptx.Slide.AddOLEObject(data, progID, opts…)` (which writes the object part, a
`p:oleObj` graphic frame with a fallback preview picture, and the wiring
relationships); unmodified objects round-trip verbatim.

In xlsx, `Sheet.AddOLEObject(spec)` embeds an object and `Workbook.OLEObjects()`
extracts them; see the [xlsx capabilities](xlsx.md#graphics-tables-and-pivots).

## Ink annotations and 3D models

Read/extract pen-stroke ink annotations and embedded 3D models.
`Slide.InkAnnotations()`/`Presentation.InkAnnotations()` (pptx) and
`Document.InkAnnotations()` (docx) enumerate ink — the InkML content parts
(`application/inkml+xml`, referenced by a `contentPart` element through a
`customXml` relationship) — reporting each part's name, content type, raw InkML
bytes, and referencing relationship id. `Slide.Model3D()`/`Presentation.Model3D()`
(pptx) and `Document.Model3D()` (docx) extract the opaque glTF-binary 3D model
parts (`model/gltf-binary`, e.g. `media/*.glb`, referenced by an `am3d:model3D`
element), returning each part's name, content type, and raw bytes. Extraction is
read-only and both kinds of part round-trip byte-for-byte; spine never parses
stroke geometry or model data. Authoring new ink strokes and embedding new 3D
models are not yet supported.
