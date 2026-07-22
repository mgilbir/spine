# Changelog

## Unreleased

User-visible changes from the feature series (#54–#58) and the audit
remediation series (#59–#75).

### Performance

- docx document bodies are now parsed lazily, and an unmodified body is written
  back verbatim. Opening a document previously parsed the entire `document.xml`
  body into a model held for the document's lifetime and regenerated it from
  that model on every save. The body is now parsed on first access; a document
  that is round-tripped without touching its body never builds a body model and
  writes its original main-part bytes through unchanged. On a 51 MB body the
  retained heap after open drops from ~506 MB to ~1 MB and a clean round-trip's
  peak RSS drops ~40%. Open still validates the body up front, so a malformed
  document fails Open exactly as before; the save-time validation skips the
  body-referential checks for an untouched body (it cannot have acquired new
  problems and its bytes pass through unchanged). Byte-for-byte round-trip is
  unaffected — and in fact slightly improved, since a passed-through body is
  always identical where a regenerated one occasionally drifted (corpus: 1196 of
  1200 identical, up from 1194).
- xlsx worksheets are now parsed lazily. Opening a workbook previously built a
  full worksheet model for every sheet up front, even though unmodified sheets
  are written straight from their preserved raw bytes on save — so for a
  workbook that is inspected lightly or round-tripped unmodified, every model
  was built and thrown away. Each sheet's model is now parsed on first access
  and never marked dirty by reads, so a clean round-trip holds only the raw
  sheet bytes. On a 21 MB worksheet the retained heap after open drops ~75%
  (92 MB → 23 MB). Open still validates every sheet up front, so a malformed
  sheet still fails Open exactly as before, and all 1200 corpus workbooks
  round-trip byte-for-byte.
- Large documents open with substantially less memory. Three "a tiny captured
  value pins the whole part" allocations were removed, and one redundant full
  copy of the main part. On a 51.8 MB `word/document.xml` the live heap after
  open drops ~23% (661 MB → 506 MB) and peak RSS ~17%.
  - `common/xml`: `CaptureProlog` built a `string(data)` of the entire part and
    then sliced tiny fields (declaration, separator, trailer) out of it, so the
    whole part stayed alive through those few-byte fields. It now scans the
    bytes directly and copies only the small spans it keeps.
  - `common/xml`: `CaptureRawInner` returned a sub-slice of the source buffer;
    every run/cell that captured its verbatim text pinned the entire part in
    memory. It now returns an independent copy, and callers keep that copy only
    when the normal escaper cannot reproduce the source (new
    `TextEscapeReproduces`) — ordinary text no longer stores a second copy
    alongside the decoded value. Applied to docx run text and DrawingML `a:t`.
  - `docx`: the main `document.xml` was stored as raw bytes for round-trip even
    though it is always regenerated from the parsed model on save. The raw copy
    is no longer retained; the one accessor that scans its raw body XML (OLE
    ProgID resolution) re-reads it on demand from the still-open source.

### Fixed

- common/xml: `Builder.WriteElement` now preserves a captured verbatim
  attribute rendering (`Attr.Raw`) instead of always re-quoting with double
  quotes, matching `StartElement`. A producer that wrote `xml:space='preserve'`
  with single quotes on a `w:t` now round-trips it byte-for-byte even when the
  run's text is canonical (so the verbatim-text fast path is skipped and the
  element is written through `WriteElement`). Surfaced by a robustness sweep of
  the corpus regenerate path (Common Crawl fa465a63).
- docx,dml,opc: five fidelity/correctness bugs surfaced by Common Crawl
  validation.
  - docx: unmodeled `w:sectPr` children (seen in the wild as trailing
    `w:mirrorMargins`/`w:tmGutter`) were dropped on save — data loss. They are
    now captured verbatim in document order and replayed, matching the raw
    fallback already used for other sectPr content.
  - docx: a `w:contentPart` (the in-body reference to an ink/customXML part) was
    dropped, so a paragraph carrying only a contentPart round-tripped as an
    empty `<w:p/>`. It is now preserved verbatim in paragraph content.
  - dml: `CT_EmbeddedWAVAudioFile` (`a:snd`/`p:snd`) gained its schema `builtIn`
    attribute, so a built-in system sound (`builtIn="1"`) is no longer dropped.
  - dml: the typed a14 image-adjustment effects (`brightnessContrast`,
    `saturation`, `sharpenSoften`, `colorTemperature`) now capture and replay
    their verbatim attribute list, so an attribute the model does not type (a
    producer's off-spec `amount` on `brightnessContrast`) survives re-marshal.
  - opc,docx,xlsx,pptx: a valid ISO-Strict (ISO/IEC 29500 Strict) package — its
    officeDocument relationship under the `purl.oclc.org/ooxml` namespace — is
    now reported as the distinct `opc.ErrStrictOOXML` instead of the generic
    "not a valid Word/Excel/PowerPoint file". Reading Strict content (all
    element namespaces differ from the transitional schemas) remains
    unsupported; this only classifies such files honestly.
- dml: `a:alphaModFix` now preserves an explicit `amt="0"`. The `amt`
  attribute defaults to 100000 (100%) in the XSD, so `amt="0"` (0% alpha)
  is semantically distinct from an absent `amt`; a non-pointer `Percentage`
  with `omitempty` dropped the strict-integer form `"0"` (its `Val` is 0 and
  `orig` empty, so `IsZeroAttr` reported zero), silently reinterpreting an
  explicit 0% as the 100% default and breaking byte-identical round-trip.
  `AlphaModFix.Amt` is now a pointer, matching the existing default-bearing
  attribute pattern, so `nil` stays omitted while an explicit zero is emitted
  (surfaced by real-world Common Crawl pptx slide layouts).
- tools/ccrun: the batched harvest now classifies every fetch outcome as
  permanent or transient and caps the retry, so a batch always makes forward
  progress. Permanent failures (DNS NXDOMAIN, connection refused, TLS failure,
  HTTP 4xx, a blocked/dead gate verdict, a non-OOXML body) terminate on the
  first attempt with a specific `fetch:*` signature; transient failures
  (timeout, 429, 5xx, connection reset, temporary DNS) are deferred at most
  three times — the count persisted in a durable sidecar — then retired as
  `fetch:transient-exhausted`. Previously dead origins were re-selected and
  re-timed-out every batch, so the tail of a large harvest could loop for weeks
  without draining (#90).
- tools/ccrun,internal/ccharvest: fetched payloads are validated as real OOXML
  packages (zip magic, readable zip, `[Content_Types].xml`) before the library
  `Open`, on both the WARC-decode and live-HTTP paths. A dead origin's HTML
  error page or login redirect (HTTP 200, non-file body) is now recorded as a
  `fetch:not-ooxml` fetch failure instead of being mis-categorized as an `open`
  failure and wasting an `Open` attempt on non-zip bytes (#90).
- docx,xlsx: `Open` fails, naming the part, when a part referenced from the
  document is missing or unreadable — a worksheet reached through a dangling
  `r:id`, a missing referenced header/footer/numbering part — instead of
  silently materializing empty content that the next save would write over
  the original data; read and parse failures of model-parsed parts also
  surface with the part name (#85).
- opc: content types registered after a raw `[Content_Types].xml` write are
  merged into the emitted file (preserving the raw formatting) instead of
  silently never being serialized, which left later-created parts without a
  content-type entry; the raw bytes still win verbatim when nothing new was
  registered (#85).
- opc: extended properties (`docProps/app.xml`) marshal the actual field
  values instead of hardcoded zeros and falses, and are parsed into
  `Reader.ExtendedProperties` at open (#85).
- docx: in a new document, adding the same image bytes twice no longer
  leaves the second placement's `r:embed` dangling (#85).
- fidelity: unchanged `.rels` sets are written back verbatim (BOM, prolog
  form, attribute order) even when a save rebuilt the set in a different
  order or the source spelled slide targets in absolute form; the last
  fidelity quarantine row now passes, leaving zero fidelity residuals over
  the 3,600-file corpus (#85).

- Property edits made through `Properties` after `Open` now persist on save
  in docx and xlsx; previously they were silently dropped (#66).
- opc: decompression limits are enforced per reader and are safe under
  concurrent part reads; `File.Open` streams are bounded by the same budget.
  Unknown core-property children survive round-trips, and foreign-namespace
  core properties are no longer rewritten into Dublin Core elements (#59, #66).
- common/xml: carriage returns in text content survive round-trips, inline
  xmlns declarations are scoped to their element, and Builder errors from
  every part marshal are surfaced instead of swallowed (#60, #67).
- dml: single-color effects accept all six color kinds, the full set of
  color transforms round-trips, and extension lists are preserved with
  their inline namespace declarations (#61, #68).
- docx: run-level content (`delText`, `pict`, `object`, comment references),
  OMML equations, and header/footer image relationships are preserved;
  unmodeled body, inline, and row markup is captured raw and written back
  verbatim; numbering, styles, and settings parts are written on both save
  lifecycles, so documents built with `Create` now include `styles.xml`
  (#62, #72).
- pptx: `Open` reports parse errors in referenced slide, layout, and master
  parts instead of silently dropping or fabricating content; unreferenced
  slide parts and unknown `graphicData` content are preserved; table, group,
  and media mutations patch parts surgically; `ReplaceText` splices are
  rune-safe (#64, #65).
- pptx: extension lists, `AlternateContent`, custom shows, slide flags, and
  animation timing details survive open/save cycles (#69, #70).
- pptx: group shapes added via `AddShape` are serialized (previously a
  silent no-op), text replacement works across `br`/`fld` paragraphs, and
  removing slides no longer leaves orphan notes or media parts (#64, #71).
- xlsx: overwriting a shared-formula master no longer corrupts follower
  cells; a stale `calcChain.xml` is dropped when sheet data changes;
  workbook mutators persist on files that lacked the target element;
  data-validation alerts, freeze panes, dimension refresh, date detection,
  and sheet-deletion cleanup fixed (#63, #73).
- common/omml: the old `OMath` model never worked in either direction —
  unmarshal left every element empty and re-marshal emitted garbage; it is
  replaced by a working, order-preserving typed model (#75).
- docx: `w:fldChar` no longer drops its children (`w:ffData` form-field
  definitions, `w:fldData`); `w:customMarkFollows` and `w14:ligatures`
  are preserved; hyperlink, paragraph-rsid, and spacing attributes follow
  Word's order; a bare `<w:vMerge/>` no longer gains `w:val=""` (#81).
- opc,docx,xlsx,pptx: a zero-modification save no longer synthesizes
  `docProps/core.xml` for packages that never had one, and a regenerated
  `[Content_Types].xml` keeps the source's `" />"` self-closing style
  (#81).
- xlsx: unknown workbook children round-trip verbatim, and xmlns
  declarations carried on `<sheet>` elements survive (#81).
- fidelity: a zero-modification save now reproduces the producer's bytes
  across the wild-file corpus. Capture mechanisms record what the source
  wrote and replay it — per-element attribute order, inline xmlns
  declarations, per-instance empty-element style (#82); per-instance child
  order with verbatim raw preservation of unmodeled children, duplicated
  singletons, and inter-child whitespace; verbatim attribute renderings
  (producer prefix aliases like `ve:`/`wp14:`, quote style, spacing,
  boolean lexical forms); verbatim text forms (entity choices, raw CR/LF);
  prolog comments, byte-order marks, and non-canonical root end tags (#83).
- docx: table cells and rows wrapped in structured document tags are no
  longer dropped on save (a data-loss bug: `w:tc`/`w:tr` inside
  `w:sdtContent`, `w:sdt` inside `w:tbl`); unmodeled run children
  (`w:pgNum`, `w:ruby`, `w:footnoteRef`), the `w14:*` run-property
  extensions, rPr-level tracked-change markers, `w:tblGridChange`,
  body/table-level `w:permStart`/`w:permEnd`, the `customXml*Range*`
  markers, and the `v:background` VML fill now survive round trips (#83).
- pptx: a `mc:AlternateContent` wrapping a picture's blip fill
  (Mac-authored decks) is no longer dropped; `a14:hiddenFill` keeps
  blip/group fill payloads; `p14:media` keeps `r:link` references;
  `a:lstStyle` keeps its `a:extLst`; `p:sldMasterId` entries without an id
  no longer gain a synthesized one (#83).
- cctest: quarantine rows can be classified `wontfix` (permanent-by-design:
  corrupt source archives, XML the decoder normalizes before the model sees
  it, whitespace-preserving producers) with a curated reason; they are
  skip-counted like quarantined rows but reported separately, and survive
  quarantine regeneration (#83).

### Changed

- opc: `ValidatePartName` enforces the OPC part-name grammar for newly
  created parts (backslash, space, unencoded `%`, control characters,
  non-ASCII bytes, and trailing-dot segments are rejected); parts preserved
  verbatim from a source package keep the lenient structural rules so wild
  names like `/[trash]/0000.dat` still round-trip (#85).
- opc: a failed save now aborts the package instead of finalizing it — the
  new `Writer.Abort` discards the output without emitting metadata, and the
  three formats' error paths use it; `Close` documents that a failed close's
  output must be discarded, and `CreatePart` documents that each new entry
  invalidates the previously returned part writer (#85).
- Save fails loudly instead of writing corrupt output: saving a workbook
  with zero sheets is an error, and unbalanced marshals abort the save
  (#67, #73).
- Error returns added to APIs that previously failed silently:
  `Slide.AddShape` (pptx), `Sheet.SetName` and `Sheet.MergeCells` (xlsx,
  both now validate their inputs), and `Writer.AddRelationship` (opc)
  (#66, #71, #73).
- xlsx: `DataValidation.ShowDropDown`, whose semantics were inverted and
  had no effect, is replaced by `HideDropDown` (#73).
- pptx: `CreateWithOptions` honors `Options.SlideSize` (widescreen and
  paper sizes included); `Create` keeps its documented 4:3 default (#71).
- pptx: the baked default master and layouts derive their placeholder
  geometry from the slide size, so `Create` (4:3) and `CreateWidescreen`
  (16:9) both produce internally consistent decks; previously the master and
  several layouts hardcoded widescreen extents (12.33" wide) that overflowed a
  4:3 slide (C139, #88).
- enum: `UnderlineStyle` gains the ECMA `ST_TextUnderlineType` values it was
  missing (`words`, `dottedHeavy`, `dashHeavy`, `dashLongHeavy`,
  `dotDashHeavy`, `dotDotDashHeavy`) and `TextAlign` gains `justLow` and
  `thaiDist`; `FontStyle`'s flags now start at bit 0 (the zero-value
  `FontStyleNormal` was moved out of the `1<<iota` block, which had left bit 0
  unused). The flags are never serialized as their integer value, so callers
  using the named constants are unaffected (C148, #88).
- common/xml: the reflection marshaler no longer silently ignores a type that
  implements the stdlib `xml.Marshaler` / `xml.MarshalerAttr` but not the
  Builder's `BuilderMarshaler` / `AttrValuer`; it records an error surfaced via
  `Builder.Err`/`Finish` instead of emitting likely-wrong bytes. This flushed
  out a latent case (`SlideLayoutID`), now given a `MarshalToBuilder` that
  preserves byte-identical output (C106, #88).

### Added

- chart: **nine more chart types** exposed on the format-agnostic builder —
  `NewBubble` (x/y/size points, added with `AddBubbleSeries`), `NewStock`
  (high-low-close), `NewSurface` (filled contour), `NewOfPie` (pie-of-pie), and
  the 3D variants `NewColumn3D`, `NewBar3D`, `NewLine3D`, `NewPie3D`, and
  `NewArea3D`. Each serializes to its `c:` chart-type group (with a `c:view3D`
  perspective and a `c:serAx` depth axis where the type needs one, and a
  `c:bubbleSize` source and second value axis for bubbles), is read back by
  `Parse`/`Charts()` with the right `Kind`, and flows through every format's
  `AddChart` unchanged — the embedded workbook now lays out bubble sizes in a
  column next to each series' Y values (#162).
- xlsx: **pivot date grouping and discrete item grouping** — `PivotOptions`
  gains `DateGroups` and `ItemGroups`. `DateGroups` buckets a date/time source
  field by year, quarter, month or day (`PivotByYear`/`PivotByQuarter`/
  `PivotByMonth`/`PivotByDay`), emitting the base field's cached dates plus a
  derived group field with a `fieldGroup`/`rangePr groupBy` and the calendar
  bucket labels. `ItemGroups` folds selected items of a field into named parent
  groups (`PivotNamedGroup{Name, Items}`), emitting a `fieldGroup`/`groupItems`
  with a `discretePr` mapping each base item to its group; items left unnamed
  remain as themselves. Each grouped field is placed on the row axis (or the
  column axis with `OnColumn`). New pivots reopen cleanly and validate; existing
  pivot parts still round-trip byte-for-byte. Pivot slicers and timelines remain
  out of scope (they require injecting `x14`/`x15` extension lists into the
  shared `workbook.xml`/`worksheet.xml` marshaling paths).
- xlsx: **external-data, what-if, dynamic-array and OLE tail features** — five
  previously-deferred SpreadsheetML data capabilities, all additive and
  fidelity-preserving (parts untouched by an edit round-trip byte-for-byte).
  (1) **External data connections** — `Workbook.Connections()` reads and
  enumerates `xl/connections.xml`, exposing each connection's id, name,
  description, type, connection string, command, and web/source URL; the part is
  preserved verbatim. Authoring or refreshing a live query (its provider
  round-trip and credentials) is deferred.
  (2) **Data model / Power Query** — `Workbook.DataModel()` reports the presence
  and locations of a Power Pivot data model (`xl/model/*`) and Power Query
  content (a `DataMashup` blob in a `customXml` item); the parts round-trip
  unchanged. Editing the model or mashup is deferred.
  (3) **What-if scenarios** — `Sheet.Scenarios()` reads and `Sheet.AddScenario`
  writes worksheet `<scenarios>` (name, comment, user, hidden/locked, and the
  changing cells with their substitute values); an existing scenarios element is
  re-emitted verbatim, an authored/modified one from the typed model.
  (4) **Dynamic-array spill metadata** — saving a workbook that gained a
  `Cell.SetDynamicArrayFormula` now synthesizes `xl/metadata.xml` (the single
  `XLDAPR` record), tags each spill master cell with `cm="1"`, and wires the
  workbook→metadata relationship, so Excel shows the spill without a recalc. A
  workbook that already carries a metadata part is left untouched (deferred).
  (5) **OLE object embedding** — `Sheet.AddOLEObject(OLEObjectSpec{...})` embeds
  an object as an `xl/embeddings/*` part, wires the worksheet `<oleObjects>`
  reference and its relationship, and generates a legacy VML Pict shape (with an
  optional preview image) so Excel renders it; the object re-extracts through
  `Workbook.OLEObjects`. (Extraction shipped earlier; this adds authoring.)
- pptx: **SmartArt cycle and process kinds** — `Slide.AddSmartArt` now accepts
  `SmartArtProcess` (a left-to-right sequence of rounded-rectangle steps,
  `dgm:alg type="lin" linDir="fromL"`) and `SmartArtCycle` (ellipse nodes
  arranged around a circle, `dgm:alg type="cycle"`) in addition to
  `SmartArtList` and `SmartArtHierarchy`. Each generates a complete, schema-valid
  `dgm:layoutDef`/`dgm:styleDef`/`dgm:colorsDef` set; the node outline reads
  back after save and reopen. Additive — existing diagrams are untouched.
- pptx: **morph transition** — `Slide.SetTransition(Transition{Type:
  TransitionMorph, MorphOption: …})` writes the PowerPoint 2016+ morph
  transition. Because morph is not part of the base PresentationML schema, it is
  emitted as an `mc:AlternateContent` standing in for `p:transition`: an
  `mc:Choice` (`Requires="p159"`) wrapping a `p:transition` with a
  `<p159:morph option="byObject|byWord|byChar">` child and the exact duration in
  `p14:dur`, plus an `mc:Fallback` fade for older readers. `MorphOption` selects
  object/word/character morphing; `Transition()` reads the morph back. Setting
  any other transition (or `TransitionNone`) removes the morph wrapper.
- pptx: **embed OLE objects** — `Slide.AddOLEObject(data, progID, opts…)` embeds
  an OLE object (its binary payload as an `/ppt/embeddings/oleObjectN.bin` part)
  and appends a `p:graphicFrame` (`a:graphicData uri=".../ole"`) whose
  `p:oleObj` references the object by relationship id and carries the required
  fallback preview picture (a minimal transparent placeholder unless
  `WithOLEPreviewImage` supplies one). Options set the frame bounds
  (`WithOLEBounds`), display name (`WithOLEName`), part content type
  (`WithOLEContentType`), and icon display (`WithOLEShowAsIcon`). `OLEObjects()`
  reports created objects and recovers their `progID` from the graphic frame.
  (The prior OLE wave shipped extract-only.)
- docx,xlsx,pptx: **form controls and ActiveX controls** — read/enumerate
  support across formats, plus basic authoring of Word legacy form fields.
  - docx: `Document.FormFields()` enumerates legacy Word form fields (the
    Developer > Legacy Tools kind: `w:fldChar` FORMTEXT/FORMCHECKBOX/FORMDROPDOWN
    with a `w:ffData` definition) anywhere in the body, tables, headers, and
    footers, returning each field's name, kind, current value, checkbox state,
    and dropdown entries/selection. `Paragraph.AddFormField(FormFieldOptions{...})`
    authors a new text, checkbox, or dropdown form field as the standard
    begin/separate/end run sequence and returns the run holding the displayed
    result. Fields read from a file round-trip byte-identical.
  - xlsx: `Sheet.FormControls()` enumerates legacy form controls (buttons,
    checkboxes, dropdowns, list boxes, option buttons, spinners, scroll bars)
    from the sheet's VML drawing, reporting each control's type, linked cell
    (`x:FmlaLink`), source range, checkbox state, VML part, and — joined through
    the worksheet `<control>` block — its name and `ctrlProps` part.
  - docx/xlsx/pptx: `ActiveXControls()` enumerates embedded ActiveX controls,
    reporting each `ax:ocx` part's COM class id, persistence mode, and its
    `activeXN.bin` persistence binary (resolved through the control part's
    relationships). All control parts (VML, `ctrlProps`, `activeX`) are
    preserved verbatim on save. Authoring ActiveX controls (which requires
    writing the OLE persistence binary) is out of scope.
- pptx,docx: **read/extract ink annotations and embedded 3D models**.
  `Slide.InkAnnotations()` / `Presentation.InkAnnotations()` (pptx) and
  `Document.InkAnnotations()` (docx) enumerate ink (pen-stroke) annotations —
  the InkML content parts (`application/inkml+xml`) referenced by a
  `contentPart` element through a `customXml` relationship — reporting each
  part's name, content type, raw InkML bytes, and referencing relationship id.
  `Slide.Model3D()` / `Presentation.Model3D()` (pptx) and `Document.Model3D()`
  (docx) extract embedded 3D models — the opaque glTF-binary parts
  (`model/gltf-binary`, e.g. `media/*.glb`) referenced by an `am3d:model3D`
  element — returning each part's name, content type, and raw bytes. Extraction
  is read-only; both kinds of part round-trip byte-for-byte through a save. New
  `opc` constants: `ContentTypeInk`, `ContentTypeModel3D`, `RelTypeCustomXML`.
  Authoring new ink strokes and embedding new 3D models are deferred (pen/binary
  authoring is out of scope). In pptx the referencing `p:contentPart` /
  `p:graphicFrame` shape-tree elements are preserved verbatim; in docx the ink
  part and its relationship are preserved, but the run-level `w:contentPart`
  body reference is not yet re-emitted by the paragraph marshaler (a pre-existing
  docx body-preservation limitation).
- docx: **document structure features** — building blocks, custom-XML data
  binding, and framesets. `Document.BuildingBlocks()` lists the glossary
  document's docParts (name, gallery, category, types, description, guid) as
  read-only `BuildingBlock` values (`Document.HasGlossary()` reports presence);
  the glossary part is preserved verbatim. `Document.CustomXMLParts()` reads the
  `customXml/itemN.xml` data parts, resolving each part's datastore item id
  (`storeItemID`) and schema refs through its itemProps, and
  `Document.AddCustomXMLPart(data)` adds a new data part — generating its
  itemProps with a fresh GUID item id, the item→itemProps relationship, the
  document relationship, and the content-type override. `ContentControl`
  gains `SetDataBinding(xpath, storeItemID)` (plus
  `SetDataBindingWithPrefixMappings`, `DataBinding`, and `RemoveDataBinding`) to
  bind a content control to a custom-XML node via `w:sdtPr/w:dataBinding`, the
  new element inserted before the control-type child in schema order.
  `Document.Frameset()` reads the web-settings part's `w:frameset` tree
  (nested framesets and leaf `Frame`s with name/title/size/scrollbar and the
  resolved source document). Unmodified glossary, custom-XML, and web-settings
  parts round-trip byte-for-byte. (Building-block and frameset **authoring**
  ships separately, below.)
- docx: **building-block and frameset authoring** — the write side of the
  glossary and web-settings parts. `Document.AddBuildingBlock(BuildingBlockDef
  {Name, Gallery, Category, Types, Style, Description, GUID})` appends a glossary
  `w:docPart` (with its `w:docPartPr` and an empty `w:docPartBody`), creating the
  glossary part, its `glossaryDocument` relationship, and its content-type
  override when the document has none; a fresh GUID is assigned when omitted.
  When a glossary part already exists the new docPart is spliced in ahead of the
  closing `</w:docParts>`, so every existing docPart (and its body) is preserved
  verbatim. `Document.SetFrameset(FramesetDef{Layout, Size, Title, Framesets,
  Frames})` authors the `w:frameset` tree (nested framesets and leaf
  `FrameDef`s), each frame's `SourceTarget` becoming an external `frame`
  relationship in the part's `.rels`; a document with no web-settings part gets
  one (plus its relationship and content-type override), and an existing part has
  its frameset replaced (or inserted) while every other setting is preserved
  verbatim. A part untouched by an authoring call still round-trips
  byte-for-byte; authored parts reopen cleanly and validate. The authored
  docPart body is a single empty paragraph — this API models a building block's
  metadata, not its reusable body content — and frame sources are external
  references.
- docx: **visible signature lines** — `Document.AddSignatureLine`/
  `Paragraph.AddSignatureLine(SignatureLineOptions{Signer, Title, Email,
  Instructions})` insert a "Microsoft Office Signature Line" placeholder (a VML
  shape carrying an `o:signatureline` element) into the body, inline or as a
  fresh paragraph; `Document.SignatureLines()` reads them back with their
  suggested-signer fields and GUID. This is the in-document request for a
  signature, distinct from actually signing the package (`opc.SignPackage`).
- docx: **bibliography and citations** — read/write bibliography sources in
  `word/bibliography/sources.xml` (`b:Sources`/`b:Source`) with
  `Document.AddSource(Source{Tag, Type, Author, Title, Year, City, Publisher})`,
  `Document.Sources()`, and `Document.RemoveSource(tag)`; cite a source with
  `Paragraph.AddCitation(tag)`, which emits a `CITATION` field (`w:fldSimple`)
  with a cached `(Author, Year)` placeholder Word replaces on field update. The
  sources part, its document relationship, and its content-type entry are
  created on first use; a document with no sources round-trips byte-identically.
- opc,common/crypto: **open legacy RC4 CryptoAPI-encrypted OOXML packages**
  ([MS-OFFCRYPTO] §2.3.5). `OpenEncrypted` now auto-detects and decrypts the
  RC4 CryptoAPI scheme in addition to the agile (AES-256/SHA-512) and ECMA-376
  standard (AES) schemes it already opened. The RC4 stream cipher (KSA + PRGA)
  is implemented from its public specification — the Go standard library omits
  RC4 and external crypto modules are not used — and is validated against the
  published RFC 6229 and canonical known-answer vectors; the full RC4 CryptoAPI
  path (SHA-1 key derivation, 512-byte per-block rekeying) is cross-validated
  against the independent `msoffcrypto-tool` reference implementation. Saving
  RC4 is intentionally **not** offered (`SaveEncrypted` still writes only agile
  or standard): RC4 is cryptographically broken and exists here only to read
  obsolete documents. The older version-1.1 binary-format RC4 scheme (§2.3.6),
  which never wraps an OOXML `.zip`, remains reported as unsupported.
- pptx: `AppendSlidesFrom` and `ExtractSlides` now **carry the source deck's
  handout master** — its part, the theme it references, its relationships, the
  presentation relationship, and the `handoutMasterIdLst` entry — when the
  source has one and the destination does not (a deck carries at most one, so a
  second source with a handout master does not add a duplicate). This completes
  the previously-deferred handout-master half of the merge furniture
  reconciliation (the notes master remains deferred).
- pptx: **create SmartArt diagrams** — `Slide.AddSmartArt(kind, nodes...)`
  builds a diagram from a node outline (a flat list, or a nested tree for a
  hierarchy) and generates everything Office needs to accept and render it: the
  data part (`dgm:dataModel`) carrying the node text and parent-of connections,
  a layout definition (`dgm:layoutDef`) with the kind's algorithm (`lin` for
  `SmartArtList`, `hierRoot`/`hierChild` for `SmartArtHierarchy`), a quick-style
  (`dgm:styleDef`) and color transform (`dgm:colorsDef`), the four content-type
  overrides, the slide relationships, and the `p:graphicFrame` whose
  `dgm:relIds` ties them together. It returns the diagram as a `SmartArt`, so
  `Nodes()` reads the outline back immediately; `SetBounds` overrides the
  default placement. The additive change leaves diagrams read from a file
  byte-identical on save.
- pptx: **five previously-deferred authoring paths** now write real content
  instead of only round-tripping what a file already had.
  (1) **Connectors inside groups** — `GroupShape.AddConnector` /
  `GroupShape.Connectors` create and read connectors as group children; a
  grouped connector's endpoint bindings resolve to the assigned cNvPr ids on
  save (descending into nested groups), group children materialize connectors,
  and an untouched group connector still round-trips byte-for-byte.
  (2) **New master text-style levels** — adding a level (`SetLevelFont` and
  friends, 0–8) absent from the source list style now inserts it at its schema
  position (before a higher level and before a captured `a:extLst`) via the new
  `dml.LstStyle.EnsureLevel` / `xml.ChildCapture.InsertTypedField`, instead of
  appending it after every captured child.
  (3) **Image backgrounds** — `Slide`/`SlideLayout`/`SlideMaster`
  `SetBackgroundImage(data, contentType)` embed a media part and point an
  `a:blipFill` at it (the master allocates a rel id clear of its layout rels).
  (4) **Transition start sounds** — `TransitionSound.StartSoundData` /
  `StartSoundContentType` embed an audio part (content type inferred when
  omitted) and author `p:sndAc/p:stSnd`; re-setting the same sound does not
  embed twice.
  (5) **Embedding fonts from bytes** — `Presentation.EmbedFont(name, regular,
  bold, italic, boldItalic)` creates the `/ppt/fonts/fontN.fntdata` parts and
  presentation-level `font` relationships, appends (or replaces) the
  `p:embeddedFont` entry, and sets `embedTrueTypeFonts` — unlike
  `SetEmbeddedFonts`, which only references pre-existing rel ids. Untouched
  content stays byte-identical.
- xlsx: **pivot table calculated fields, grouping, and extending existing
  caches** — `Sheet.AddPivotTable` gains three `PivotOptions` capabilities.
  `CalculatedFields` adds formula-derived value fields (e.g. `Profit =
  "Sales-Cost"`): each becomes a `databaseField="0"` cache field carrying the
  formula and is placed on the value axis. `NumericGroups` buckets a numeric
  source field into equal-width value ranges (a leading `<start` bucket, one
  `lo-hi` bucket per interval, and a trailing `>end` bucket) emitted as a derived
  group cache field (`fieldGroup`/`rangePr`/`groupItems`) placed on the row (or
  column) axis, with the base field enumerated as discrete numeric shared items.
  A workbook that already contains pivot caches is now **extended** rather than
  refused: the new cache is allocated an id and part names that do not collide
  with the existing pivots, its workbook `<pivotCaches>` entry is merged with the
  preserved entries (whose relationships are kept intact), and existing pivots
  continue to round-trip. `refreshOnLoad` still drives Excel to rebuild the
  rendered layout on open. Deferred: date grouping, discrete (manual) item
  grouping, multiple consolidation ranges, and external-data caches.
- xlsx: **array, shared and dynamic-array formula authoring** —
  `Cell.SetArrayFormula(formula, ref)` writes a legacy CSE array master
  (`<f t="array" ref="…">`); `Cell.SetSharedFormula(formula, ref)` writes a
  shared-formula master (`t="shared"` with a freshly allocated `si` and a `ref`
  over the range) and fills the rest of the range with follower stubs, the
  compact copy-down encoding Excel uses; `Cell.SetDynamicArrayFormula` writes
  the modern spill form (`t="array"` with `aca`/`ca`) for SORT/FILTER/UNIQUE and
  friends. The `aca`/`ca` dynamic-array flags on `<f>` are now captured on read
  so an existing dynamic-array formula round-trips instead of losing its
  marking. (The dynamic-array cell-metadata linkage — the `cm` attribute into
  xl/metadata.xml — is left for Excel to rewrite and is not synthesized.)
- xlsx: **data-validation error style and IME mode** — `DataValidation` now
  exposes `ErrorStyle` (`ValidationErrorStop`/`Warning`/`Information`) and
  `ImeMode`, read and written through `AddDataValidation` / `DataValidations`.
- xlsx: **cell-comment rich text** — `Comment.RichText()` reads a note's body as
  per-run formatting (bold labels, colored text), `Sheet.AddNoteRichText`
  authors one, and `Comment.SetRichText` rewrites an existing comment's body;
  `Comment.Text()` continues to return the flattened plain text.
- xlsx: **sparkline group mutation** — `Sheet.Sparklines` now returns live
  handles: `SparklineGroup` gains color setters for every slot
  (`SetSeriesColor`/`SetNegativeColor`/`SetAxisColor`/`SetMarkersColor`/`SetFirstColor`/`SetLastColor`/`SetHighColor`/`SetLowColor`),
  per-group boolean toggles (`SetMarkers`/`SetHigh`/`SetLow`/`SetFirst`/`SetLast`/`SetNegative`,
  plus a `Markers` reader), and `Delete` (removing the last group drops the
  sparkline extension). Unmutated sparklines still round-trip byte-for-byte.
- docx: **WordArt, shape groups, down-level text-box fallbacks, and OLE
  embedding** — the remaining drawing authoring gaps left after text boxes.
  `Paragraph.AddWordArt`/`Document.AddWordArt` create a DrawingML (wps) text
  effect: a fill-less, outline-less shape whose text carries a solid fill and an
  optional preset text warp (`WarpArchUp`, `WarpCircle`, ...), inline or
  anchored. `Paragraph.AddShapeGroup`/`Document.AddShapeGroup` group several
  shapes/text boxes into a `wpg:wgp` group, each member positioned in the
  group's child coordinate space (extent defaults to the members' bounding box).
  `TextBoxOptions.VMLFallback` wraps an authored text box in the
  `mc:AlternateContent` Choice(DrawingML)+Fallback(VML `w:pict`) pair Word emits
  for pre-2010 readers. `Paragraph.AddOLEObject`/`Document.AddOLEObject` embed an
  OLE object stream as `/word/embeddings/oleObjectN.bin` with its content-type
  override, an `oleObject` relationship, a presentation-icon image part, and a
  `w:object` (`v:shape` + `o:OLEObject`) reference declaring the ProgID; the
  object is reported by `OLEObjects()` after a round trip. All are additive and
  leave existing drawings byte-identical.
- docx: **tracked-move revisions** — `Document.Revisions` now enumerates tracked
  moves (`w:moveFrom`/`w:moveTo`) as `RevisionMoveFrom` and `RevisionMoveTo`
  (with `Revision.MoveName` linking the two halves), and `Accept`/`Reject`
  transform them correctly: accepting keeps the destination (`moveTo`) content
  and drops the source (`moveFrom`); rejecting does the inverse.
  `Paragraph.AddMoveFromRun`/`AddMoveToRun` (and `…WithDate` variants) author a
  move. Move content is still preserved byte-for-byte when untouched; the range
  markers are left in place across accept/reject.
- docx: **header/footer revisions** — `Document.Revisions`,
  `AcceptAllRevisions`, and `RejectAllRevisions` now cover the header and footer
  parts, not just the main body. Header/footer revisions follow the body
  revisions, ordered by part name, and accepting/rejecting one flags only the
  parts it actually rewrites for regeneration (parts without revisions stay
  byte-identical).
- docx: **per-section watermarks** — `Document.SetSectionTextWatermark` and
  `SetSectionImageWatermark` stamp a watermark on a specific section's headers
  (as returned by `Sections`/`DefaultSection`), allowing distinct watermarks per
  section, alongside the existing document-wide `SetTextWatermark`/
  `SetImageWatermark`.
- docx: **DrawingML watermark emission** — `WatermarkOptions.DrawingML` emits a
  text watermark as a DrawingML text box wrapped in `mc:AlternateContent` with
  the classic VML shape as the fallback (the form newer Word versions write),
  so DrawingML-aware consumers render the text box and others fall back to VML.
- docx: **document-level footnote/endnote numbering** —
  `Document.FootnoteProperties`/`SetFootnoteProperties`/`ClearFootnoteProperties`
  and the endnote equivalents expose the document-wide numbering defaults
  (`w:settings/w:footnotePr`, `w:settings/w:endnotePr`), complementing the
  existing per-section `Section.FootnoteProperties`. Setting the numbering
  preserves any separator `w:footnote` references already present.
- opc,common/crypto: **ECMA-376 "standard" password encryption**
  ([MS-OFFCRYPTO] §2.3.4.5–§2.3.4.9) — the AES/SHA-1 scheme Office 2007 wrote,
  now supported for both open and save alongside the existing agile scheme.
  `opc.OpenEncrypted` auto-detects standard vs agile from the EncryptionInfo
  version, so encrypted `docx`/`xlsx`/`pptx` written by older Office open with
  no API change. A new `opc.SaveEncryptedWithOptions(w, data, password, opts)`
  chooses the scheme (`opc.SchemeAgile` default, or `opc.SchemeStandard` with a
  128/192/256-bit AES key) and whether to emit DataSpaces streams;
  `opc.SaveEncrypted` is unchanged (agile, AES-256). Standard uses AES-ECB and
  SHA-1 key derivation as the spec mandates and is weaker than agile (no
  per-block IV, no integrity HMAC), so agile stays the recommended default. All
  crypto is Go-stdlib only (`crypto/aes`, `crypto/sha1`). Cross-validated
  against `msoffcrypto-tool`: files this library encrypts decrypt cleanly with
  that independent tool, and the key derivation matches its published reference
  vector.
- opc: encrypted saves can emit the optional `\x06DataSpaces` metadata streams
  ([MS-OFFCRYPTO] §2.1) via `EncryptOptions.IncludeDataSpaces`. These are not
  needed to decrypt (readers ignore them), but some Office builds expect them;
  the stream payloads are the canonical reference bytes and are placed in a
  nested CFB storage tree (new storage-aware CFB writer). Off by default, so the
  standard encrypted-save output is unchanged. Cross-validated: the emitted
  streams are byte-identical to the reference implementation and an independent
  CFB reader (`olefile`) navigates the storage tree correctly.
- opc: package digital signatures now include Microsoft Office's
  application-specific signature `Object` (`idOfficeObject` with a
  `SignatureInfoV1` in the office digsig namespace), covered by the signature
  via its own `SignedInfo` reference, so Office's signature UI recognizes the
  signature. The environment fields carry neutral placeholder values. The prior
  standards-compliant package `Object` is unchanged; existing signatures still
  verify. Cross-validated with an independent `lxml`/`cryptography` toolchain
  that checks the `SignedInfo` signature and every object digest.
- common/crypto: the obsolete legacy RC4/CryptoAPI encryption schemes
  ([MS-OFFCRYPTO] §2.3.5/§2.3.6) remain deliberately **not decoded** — they are
  cryptographically broken, effectively unused for OOXML packages, and have no
  reference decoder to cross-validate an implementation against. `Decrypt` now
  identifies them precisely and returns `ErrUnsupportedEncryption` with an
  actionable message rather than an opaque one.

- merge/split: the append/extract feature now reconciles the referenced
  furniture each format was previously dropping, so merged packages open with no
  dangling references or duplicate part names. **pptx**:
  `Presentation.AppendSlidesFrom` / `ExtractSlides` carry each slide's own slide
  layout — and the slide master and theme that layout depends on — with remapped
  part names and relationships, so a slide keeps its original look instead of
  being forced onto a destination layout; masters (and layouts) byte-for-byte
  identical to ones already in the destination are reused rather than
  duplicated, and furniture shared by several slides is imported once. Each
  slide's notes slide is carried too, re-wired to the new slide (importing the
  source's notes master and handout master is deferred). **docx**:
  `Document.Append` carries the header and footer parts referenced by the
  section breaks it copies (with the images they embed) instead of dropping the
  references, and imports numbering definitions that live as preserved raw XML
  in an opened source (not only the typed session-added definitions). **xlsx**:
  `Workbook.CopySheetFrom` copies the images embedded in the source sheet —
  both those added this session and those parsed from an opened source's drawing
  part — re-embedding their media under non-colliding part names (the SVG
  variant of an SVG-with-raster-fallback image and embedded charts remain
  deferred).
- all formats: **whole-document text extraction** — a symmetric, read-only
  "give me all the text" API for search, indexing, and LLM ingestion, reusing
  the existing per-element text accessors and mutating nothing.
  `docx.Document.Text()` returns the body in document order (paragraphs, with
  the text of runs nested in hyperlinks, simple fields, tracked insertions, and
  inline content controls; tables as tab-separated cells / newline-separated
  rows; block-level content controls), then each header and footer, footnotes
  and endnotes, and text boxes. `xlsx.Workbook.Text()` and `xlsx.Sheet.Text()`
  return cell values row-major (shared strings and rich text resolved to their
  text; tab-separated cells, newline-separated rows) followed by cell comments;
  sheets are separated by a blank line. `pptx.Presentation.Text()` and
  `pptx.Slide.Text()` return the text of every shape (text boxes, placeholders,
  auto shapes, and tables, descending into groups) in shape order, then speaker
  notes and comments; slides are separated by a blank line, with
  `Presentation.SlideTexts()` returning the per-slide strings. Output is plain
  concatenation with no markup and deterministic ordering.
- docx,xlsx: **search & replace** — `Document.ReplaceText(map[string]string)`
  (docx) and `Workbook.ReplaceText` / `Sheet.ReplaceText` (xlsx), mirroring the
  existing `pptx.Presentation.ReplaceText` / `Slide.ReplaceText` for cross-format
  symmetry. Keys are matched exactly; a longest-match, single-pass replacement
  keeps the result independent of map order and never re-replaces a value that
  contains another key. docx replaces across the body (including tables and
  structured document tags) and every header/footer, and — the hard case —
  matches a key that Word has split across several `w:r` runs: the paragraph's
  text-only runs are concatenated, the match spliced back, and the replacement
  inherits the first affected run's formatting while surrounding runs keep
  theirs; line breaks, tabs, fields, drawings, and hyperlink/field boundaries
  delimit distinct content and are never crossed. xlsx replaces in string cells
  (shared-string cells are converted to inline so the shared table is left
  untouched) and across a rich cell's runs; formula cells are not touched (their
  string value is a cached result, not literal text). A document with no
  matching text round-trips byte-for-byte.
- pptx,docx,xlsx: **document merge / append / split** — combine or divide
  packages while remapping relationships, part names, and ids so the result has
  no dangling references or duplicate part names. `Presentation.AppendSlidesFrom`
  copies another deck's slides in order, carrying their media, charts, and
  embedded workbooks under fresh, non-colliding part names with remapped
  relationship targets; `Presentation.ExtractSlides(indices)` returns a new deck
  containing just the requested slides with their referenced parts. `Document.Append`
  appends another document's body content (paragraphs, tables, block SDTs),
  bringing its images and copying its style and numbering definitions, remapping
  style ids and numbering ids on collision and rewriting every reference in the
  copied content. `Workbook.CopySheetFrom(other, name)` copies a sheet under a
  unique name with its shared-string-resolved cell values, formulas, remapped
  cell styles, merged ranges, and column/row sizes. Combined packages are
  validated (`Validate()` clean) and reopen without dangling references. Deferred:
  full slide-master/theme reconciliation (appended pptx slides adopt the
  destination's matching layout; notes slides are not carried), docx
  header/footer carry-over (section-break header/footer references are dropped)
  and raw (opened-source) numbering carry-over, and copying images embedded in an
  opened xlsx source sheet (#108).
- docx,xlsx,pptx: **VBA macro** authoring — `HasMacros()`, `VBAProject() []byte`
  (extract), `SetVBAProject([]byte)` (inject/replace), and `RemoveVBAProject()`
  on `Document`/`Workbook`/`Presentation`. Injecting wires the `vbaProject.bin`
  content type and main-part relationship and flips the package to the
  macro-enabled flavor (`.docm`/`.xlsm`/`.pptm` and their template/slideshow
  variants); removal flips it back and drops the part, its override, and its
  relationship. The VBA project is an opaque MS-OVBA/CFB blob: spine carries it
  verbatim and never parses or executes it — an injected project transplants
  its source's macros and their trust unchanged, so only inject bytes you
  trust. An unmodified macro-enabled file round-trips its `vbaProject.bin`
  byte-for-byte.
- docx,xlsx,pptx: **embedded OLE object** extraction — `OLEObjects()` returns
  each embedded object (`embeddings/oleObjectN.bin`) as `{Name, ContentType,
  Data, ProgID}`, locating them through the package's `oleObject` relationships
  with a fallback for embedded parts typed as OLE objects and a best-effort
  ProgID read from the referencing element. Objects are carried verbatim on
  save. Embedding a *new* OLE object (part plus graphic-frame/object reference)
  is deferred: it requires synthesizing a visible reference in the content
  model and is out of scope for this change.
- opc,docx,xlsx,pptx: **custom document properties** (`docProps/custom.xml`)
  read + write. Each format exposes `CustomProperties() map[string]any`,
  `SetCustomProperty(name string, value any) error`, and
  `RemoveCustomProperty(name string) bool` on `docx.Document`, `xlsx.Workbook`,
  and `pptx.Presentation`. Values are typed as string (`vt:lpwstr`), int64
  (`vt:i4`/`vt:i8`), float64 (`vt:r8`), bool (`vt:bool`), and time.Time
  (`vt:filetime`); `int`/`int32` and `float32` are accepted and widened. Setting
  a property on a package that has none creates the part, its content-type
  override, and the package relationship on save; existing properties round-trip
  byte-identically when left untouched, and an unmodeled variant type (e.g.
  `vt:vector`) is preserved verbatim across a modify-and-save. The package layer
  parses the part into `opc.Reader.CustomProperties` and writes it from
  `opc.Writer.CustomProperties` (#108).
- opc,docx,common/crypto: **password encryption** — read and write documents
  protected with Office's real AES encryption (not the legacy 16-bit
  obfuscation in `common/crypto/legacy.go`, which guards nothing). An encrypted
  OOXML file is a Compound File Binary (CFB/OLE2) container holding an
  `EncryptionInfo` descriptor and the AES-encrypted `EncryptedPackage`; a
  minimal CFB reader/writer (`opc/cfb.go`) and the agile-encryption primitives
  (`common/crypto/agile.go`, [MS-OFFCRYPTO] §2.3.4.10–15, AES-256 in CBC mode
  with SHA-512 key derivation) implement both directions. `opc.OpenEncrypted`
  decrypts a package and returns a normal `opc.Reader`; `opc.SaveEncrypted`
  writes an encrypted container with a freshly generated random salt; the plain
  `OpenReader`/`NewReader` path now detects a CFB input and returns
  `opc.ErrEncrypted` pointing to `OpenEncrypted`. Format-level convenience is
  exposed for Word: `docx.OpenEncrypted`/`OpenEncryptedReader` and
  `Document.SaveEncrypted`/`SaveEncryptedTo`. A wrong password returns
  `crypto.ErrWrongPassword`; the older ECMA-376 "standard" scheme (§2.3.4.5) and
  legacy RC4 schemes are detected and rejected with
  `crypto.ErrUnsupportedEncryption`. The implementation uses only the standard
  library's audited primitives (`crypto/aes`, `crypto/cipher`, `crypto/sha512`,
  `crypto/hmac`, `crypto/rand`) and verifies the package HMAC integrity block on
  read. Round-trip is cross-validated against `msoffcrypto-tool`, which decrypts
  the library's encrypted output to byte-identical plaintext.
- opc: **OPC package digital signatures** (ECMA-376 Part 2 §13) — sign and
  verify XML-DSig package signatures using only the Go standard library.
  `Reader.VerifySignatures()` discovers the signature parts under
  `_xmlsignatures/`, recomputes every part digest (SHA-256/SHA-1), replays the
  OPC Relationship Transform for signed `.rels` parts, canonicalizes the signed
  objects, and checks the `SignatureValue` over the canonicalized `SignedInfo`
  against the embedded X.509 certificate — reporting per-signature validity, the
  signer subject/issuer and validity window, the signing time, and every covered
  part. `SignPackage(src, dst, signer, cert, parts)` writes a copy of the package
  with a fresh `sig1.xml` signing the requested parts (all parts when the list is
  empty) plus the package relationships, using SHA-256 with RSA-SHA256 or
  ECDSA-SHA256. A new inclusive Canonical XML 1.0 implementation lives in
  `common/xml` (`Canonicalize`, `ParseC14N`), validated against the lxml
  reference serializer; the RSA/ECDSA primitives live in `common/crypto`. The
  emitted signatures are standards-compliant XML-DSig (independently verified
  with OpenSSL); interoperability with Microsoft Office's signature UI, which
  adds an optional Office-specific signature object, is best-effort and has not
  been validated against Office itself.
- xlsx: **sparklines** read + write — `Sheet.Sparklines()` returns the sparkline
  groups defined in the worksheet extension list (`x14:sparklineGroups`), each
  exposing its type (`line`/`column`/`winloss`), series color and the
  `(dataRange, locationCell)` mappings; `Sheet.AddSparklineGroup(SparklineOptions)`
  writes a new group under the correct `x14`/`xm` namespaces (matching Excel's
  own `<ext>`-scoped declarations; no `mc:AlternateContent` wrapper, which Excel
  does not use for sparklines). Adding to a sheet that already has sparklines
  appends to the existing extension. Worksheets whose sparklines are not modified
  round-trip byte-for-byte: sparklines are captured raw in `extLst` and only
  re-serialized to typed XML when a caller adds or modifies one.
- xlsx: **pivot tables** — read and create. `Sheet.PivotTables()` and
  `Workbook.PivotTables()` return each pivot table with its name, on-sheet
  location, source range/sheet, cache id, and the field layout (`RowFields`,
  `ColumnFields`, `Filters`, and `ValueFields` with each value field's
  aggregation). `Sheet.AddPivotTable(sourceRange, anchor, PivotOptions)` builds
  a basic pivot table from a (optionally sheet-qualified) source range: it
  derives the cache fields from the header row and a shared-items scan, writes
  the pivot cache definition + records, the pivot table definition, the
  workbook `<pivotCaches>` entry, all relationships and the
  `[Content_Types].xml` overrides. Value fields support sum/count/average
  (plus min/max/product/countNums); the cache is written with `refreshOnLoad`
  so Excel rebuilds the rendered layout on open. Existing pivot tables in an
  opened workbook round-trip byte-for-byte (their parts are preserved raw);
  typed XML is emitted only for pivots created this session. (Calculated fields,
  numeric grouping, and extending a workbook that already has pivot caches were
  added in a follow-up — see above. Still deferred: date grouping, discrete item
  grouping, multiple consolidation ranges, and external-data caches.)
- pptx: **read SmartArt / diagrams** — `Slide.SmartArt()` and
  `Presentation.SmartArt()` return the SmartArt graphics on a slide (each a
  `p:graphicFrame` whose `a:graphicData` carries the diagram namespace URI and a
  `dgm:relIds` reference to the data/layout/quickStyle/colors parts). Each
  `SmartArt` exposes the diagram's text nodes and their hierarchy, derived from
  the data part's `dgm:dataModel` (its `dgm:pt` points and `dgm:cxn` parent-of
  connections, ordered by `srcOrd`), as a tree of `SmartArtNode{Text, Children}`
  via `SmartArt.Nodes()`. The four raw diagram parts are preserved verbatim, so
  an unmodified deck with SmartArt round-trips byte-for-byte. The data-model
  parser and tree view live in `common/dml/diagram` (`ParseDataModel`,
  `DataModel.TextTree`). Creating diagrams from scratch is not yet supported: a
  valid diagram also needs the layout/quickStyle/colors definition parts and a
  `dsp` drawing fallback, which Office rejects if malformed.
- pptx: **slide master / layout editing** — masters and layouts, previously
  read-only, can now be edited. `SlideLayout` gains a background fill API
  mirroring the slide and master ones (`SetBackgroundFill`, `ClearBackground`,
  `HasBackground`, `BackgroundColor`, reusing `dml.Fill`). A slide master's three
  text-style trees are read + writable through `SlideMaster.TitleStyle()`,
  `BodyStyle()`, and `OtherStyle()`, returning a `MasterTextStyle` whose per-level
  (0–8) setters (`SetLevelFont`, `SetLevelFontSize`, `SetLevelBold`,
  `SetLevelItalic`, `SetLevelColor`, `SetLevelBullet`, `SetLevelBulletChar`) mutate
  the underlying `a:lvlNpPr` in place and `Level(n)` reads a snapshot. Layout and
  master placeholder geometry is editable via `EditablePlaceholders()` /
  `EditablePlaceholder(type)`, whose `SetPosition`/`SetSize` write the shape's
  `a:xfrm` off/ext. Masters and layouts that are not edited round-trip
  byte-for-byte, and an edit touches only its own part — editing one layout or the
  master text styles leaves every other master/layout part byte-identical.
  (`SlideMaster.AddLayout`, which wires the master→layout relationship and content
  type so `AddSlideWithLayout` can target the new layout, was already present and
  is now covered by round-trip tests.)
- pptx: **connectors** — connection shapes (`p:cxnSp`) are now materialized and
  authorable. `Slide.Connectors()` returns the connectors on a slide, each
  reporting its routing (`Kind`: straight / elbow / curved), endpoint bindings
  (`StartConnection`/`EndConnection` give the bound cNvPr id + connection-site
  index, or report a free endpoint), and line style (`LineWidth`/`LineColor`/
  `LineDash`). `Slide.AddConnector(kind)` draws a new connector: bind its ends to
  shapes with `Connect`/`SetStartShape`/`SetEndShape` (the target ids are
  resolved on save, so it can link API-created shapes whose ids are assigned
  then) or place it freely with `SetPoints`, and style its line with `SetLine`/
  `SetLineWidth`/`SetLineColor`/`SetLineDash`. Connectors participate in the
  shape sync, so they coexist with other shapes; a deck with existing connectors
  round-trips byte-for-byte when untouched, and edits flush into the parsed node
  in place.
- docx: **text boxes and basic shapes** — `Paragraph.AddTextBox(text, opts)` and
  `Document.AddTextBox` insert a DrawingML (`wps:wsp`) text box, inline or
  anchored (`TextBoxOptions.Floating`/`Anchor`), with a size in EMU, a preset
  geometry (`ShapeRectangle`/`ShapeRoundRectangle`/`ShapeEllipse`/`ShapeLine`),
  a fill and border, and the text as real WordprocessingML paragraphs in
  `w:txbxContent` (one `w:p` per line). `Paragraph.AddShape`/`Document.AddShape`
  reuse the same drawing path for a shape with optional text. `Document.TextBoxes()`
  reads every text box back — text and geometry — for both modern DrawingML
  (`wps:txbx`) and legacy VML (`w:pict/v:textbox`) boxes, including boxes wrapped
  in `mc:AlternateContent` (the wps choice and its VML fallback are read once, not
  double-counted). Text boxes need no extra parts or relationships, so a box added
  to an opened document, and any existing drawing left untouched, round-trip
  byte-for-byte. Deferred: the `mc:AlternateContent`/VML down-level fallback on
  authored boxes, complex adjust-handle geometry, grouping/connectors, and WordArt.
- docx: **mail merge** — read and write a document's mail-merge configuration
  through `Document.MailMerge()`/`Document.SetMailMerge()`, exposing the merge
  main-document type (`MailMergeFormLetters`, `MailMergeEmail`, …), data type,
  connection/query, destination, and the Office Data Source Object (`w:odso`):
  source relationship, table, connection type, first-row-header flag, column
  delimiter, and the data-source-column→field-name mappings. Insert MERGEFIELD
  fields in the body with `Paragraph.AddMergeField(name)` (a `w:fldSimple`
  MERGEFIELD with a «name» placeholder result; whitespace names are quoted), and
  enumerate the distinct merge-field names present anywhere in the body — simple
  and complex (`w:fldChar`/`w:instrText`) fields, including inside tables — with
  `Document.MergeFields()`. A document with existing mail-merge settings or
  fields that are not modified round-trips byte-for-byte (the settings element is
  preserved verbatim and only regenerated on an explicit `SetMailMerge`).
- docx: **watermarks** — `Document.SetTextWatermark(text, WatermarkOptions)`
  stamps a WordArt text watermark (legacy VML `w:pict`/`v:shape` on the
  `#_x0000_t136` text-path shape) and `Document.SetImageWatermark(imageBytes,
  WatermarkOptions)` a washed-out image watermark (`v:imagedata` referencing a
  media part; PNG/JPEG/GIF content type and size sniffed from the bytes). Both
  insert the shape into the default header — created if the document has none,
  reusing the existing header/media/relationship infrastructure — and into any
  first-page/even-page headers the default section already references, so the
  watermark shows on every page; calling either again replaces the current
  watermark. `WatermarkOptions` carries font, color, and diagonal/rotation.
  `Document.Watermark()` detects and reports an existing text or image
  watermark, and `Document.RemoveWatermark()` removes it. A document whose
  header/watermark is left untouched round-trips byte-for-byte.
- docx: **author tracked changes** — the revisions API can now create insertions
  and deletions, not only read and accept/reject them. `Paragraph.AddInsertedRun`
  appends a new run wrapped in `w:ins`; `Run.MarkInserted` wraps an existing run;
  `Run.MarkDeleted` wraps a run in `w:del`, converting its `w:t` to `w:delText`.
  Each revision gets a unique, monotonic `w:id` (scanned above any id already in
  the document) and a `w:date` defaulting to now; `...WithDate` variants pin the
  timestamp for deterministic output. Authored revisions round-trip through
  `Document.Revisions()` and `Accept`/`Reject`/`AcceptAllRevisions`/
  `RejectAllRevisions`.
- pptx: **speaker notes** read + write on a slide — `Slide.Notes()` returns the
  notes slide's body-placeholder text (paragraphs joined by `\n`, or `""` when
  the slide has no notes), and `Slide.SetNotes(text)` writes it. When the slide
  already has a notes slide, only the body text is rewritten and the rest of the
  notes slide is preserved; otherwise a new `notesSlideN.xml` part is created,
  wired to the slide (and to the notes master when the deck has one), given its
  `[Content_Types].xml` override, and persisted on both the created and opened
  save paths. Existing notes slides that are not edited still round-trip
  byte-for-byte (they are preserved as raw parts); `SetNotes` only rewrites the
  one affected part.
- docx: section, table, paragraph, and run **depth** — modeled structures that
  previously round-tripped but had no public accessor are now read + write.
  Sections gain page borders (`Section.PageBorders`/`SetPageBorders`), line
  numbering (`LineNumbering`/`SetLineNumbering`), vertical alignment
  (`VerticalAlignment`/`SetVerticalAlignment`), paper source
  (`PaperSource`/`SetPaperSource`), document grid
  (`DocumentGrid`/`SetDocumentGrid`), and per-section footnote/endnote numbering
  (`FootnoteProperties`/`EndnoteProperties` and their setters: position, number
  format, start, restart). Document settings gain `DefaultTabStop`,
  `EvenAndOddHeaders`, `Zoom`, and document variables (`DocumentVariables`,
  `DocumentVariable`, `SetDocumentVariable`, `RemoveDocumentVariable`), each with
  getters and setters. Tables gain **vertical cell merge**
  (`TableCell.SetVerticalMerge` restart/continue, the counterpart to the existing
  horizontal `SetGridSpan`), plus table look (`SetTableLook`/`TableLook`), layout
  (`SetLayout`/`Layout` fixed/autofit), indent (`SetIndent`/`Indent`), and
  alignment (`SetAlignment`/`Alignment`), and read accessors for the existing
  border/width/shading setters (`Table.Borders`/`Width`/`Shading`,
  `TableCell.Borders`/`Width`/`Shading`/`VerticalAlignment`/`GridSpan`).
  Paragraphs gain borders and shading (`SetBorders`/`Borders`,
  `SetShading`/`Shading`). Runs gain a character style
  (`Run.SetStyle`/`Style`) and symbol glyphs (`Run.AddSymbol`). All additions are
  additive: a file that does not use a feature is not perturbed, and edits
  round-trip through save/reopen.
- pptx: shape effects & visuals capability bundle (**read + write**).
  - **Shape effects**: `AutoShape` and `TextBox` gain `SetGlow`/`Glow`,
    `SetReflection`/`Reflection`, `SetSoftEdge`/`SoftEdge`, and a basic 3D
    `SetBevel`/`Bevel` (top bevel), alongside the existing `SetShadow`. Each
    routes through a `dml` value type (`dml.Glow`, `dml.Reflection`,
    `dml.SoftEdge`, `dml.Bevel3D` with `BevelCircle`/… presets) that writes
    `a:effectLst`/`a:sp3d` without disturbing effects already present.
  - **Background formatting**: `Slide` and `SlideMaster` gain
    `SetBackgroundFill(dml.Fill)`, `BackgroundColor()`, `HasBackground()`, and
    `ClearBackground()` (`p:bg`/`p:bgPr`), reusing `dml.Fill` so a solid,
    gradient, or pattern fill can be applied as a background.
  - **Transitions**: eight new `TransitionType` variants (Circle, Comb,
    Newsflash, Pull, RandomBar, Strips, Wedge, Zoom) plus `Direction`,
    `Orientation`, `Spokes`, `ThroughBlack`, and `Sound` (`TransitionSound`,
    with a settable stop-previous-sound and read-back of a parsed start sound)
    on the `Transition` struct, all read and written.
  - **Tables**: `Table.SetStyleID`/`StyleID` for the built-in/theme table-style
    reference (`a:tblPr/a:tableStyleId`) and `TableCell.SetMargins`/`Margins`/
    `ClearMargins` for per-cell text insets (`a:tcPr/@marL,marT,marR,marB`).
  - **Embedded fonts**: `Presentation.EmbeddedFonts`/`SetEmbeddedFonts` and
    `EmbedTrueTypeFonts`/`SetEmbedTrueTypeFonts` (`p:embeddedFontLst`).
  - **Custom shows**: `Presentation.CustomShows`/`SetCustomShows`/
    `AddCustomShow` and `Slide.RelID` (`p:custShowLst`).
  - All accessors are additive: a file that does not use them round-trips
    byte-for-byte.
- xlsx: sheet view & structure **read + write**. Sheet visibility
  (`Sheet.Visibility`/`Visible` and `SetVisibility`/`SetVisible` with the
  `SheetVisibility` enum — visible, hidden, veryHidden — refusing to hide the
  workbook's last visible sheet). Row/column hide setters
  (`SetRowHidden`/`SetColumnHidden`, the write counterparts of the existing
  `RowHidden`/`ColumnHidden`; the column setter carves the target column out of
  a spanned `<col>` entry). Sheet-view toggles with getters and setters:
  `ShowRowColHeaders`, `RightToLeft`, `ShowFormulas`, `ShowZeros`, `ShowRuler`
  (each honoring its OOXML default when unset) and the view mode
  (`View`/`SetView`: `ViewNormal`, `ViewPageLayout`, `ViewPageBreakPreview`).
  Scrolling split panes (`Sheet.SplitPanes`, writing a `state="split"` pane with
  twip offsets and an active-pane selection, distinct from the row/column-count
  `FreezePanes`; `SplitPanePosition` reads it back). Row and column grouping and
  outline levels (`GroupRows`/`UngroupRows`, `GroupColumns`/`UngroupColumns`,
  `SetRowOutlineLevel`/`RowOutlineLevel`, `SetColumnOutlineLevel`/
  `ColumnOutlineLevel`, collapsed flags, and `SetOutlineSummary`/`OutlineSummary`
  for summary-row/column placement). Force recalculation on next open
  (`Workbook.SetForceFullCalc`/`ForceFullCalc`, the workbook `calcPr`
  `fullCalcOnLoad` flag) so edited formulas refresh their cached results. All
  additive: a file that uses none of these round-trips byte-for-byte, and
  clearing a flag reconciles the captured source attributes so a hidden→visible
  or force-recalc-off change actually takes effect on save.
- xlsx: style and auto-filter depth, **read + write**. Named/built-in cell
  styles — `StyleManager.AddNamedStyle` / `NamedStyles` / `ApplyNamedStyle` /
  `NamedStyleXfId` and `Cell.SetNamedStyle`, with `BuiltinStyle*` id constants
  (Good, Bad, Heading 1, …) — wire `cellStyleXfs`/`cellStyles`. `FillStyle`
  gains a `Gradient` option (`GradientFill`/`GradientStop`, linear and path);
  `BorderStyle` gains `Diagonal`, `DiagonalUp` and `DiagonalDown`;
  `AlignmentStyle` gains `ShrinkToFit`, `JustifyLastLine`, `ReadingOrder` and
  `RelativeIndent`. Auto-filter predicates — `Sheet.SetFilterColumn` /
  `FilterColumns` / `ClearFilterColumns` read and write per-column value-list
  filters (`FilterColumn.Values`/`Blank`) and custom comparison filters
  (`CustomFilter` with the `Filter*` operator constants). Sort state —
  `Sheet.SetSortState` / `SortState` / `RemoveSortState` read and write
  `sortState` conditions on a range or auto-filter. Defined names —
  `Workbook.RemoveDefinedName` / `RemoveDefinedNameScoped` and
  `AddDefinedNameFull`, and `DefinedName` now surfaces `Hidden`, `Comment` and
  `Description`. All accessors round-trip through save/reopen; files that use
  these features and are not modified still round-trip byte-for-byte.
- xlsx: tables (ListObjects) **read + write**. `Sheet.Tables()` and
  `Workbook.Tables()` surface each table's name, range, columns (id, name,
  totals-row function/label, calculated-column formula), header/totals rows and
  built-in style (`Table.Style()`: style name plus row/column-stripe and
  first/last-column banding). `Sheet.AddTable(cellRange, TableOptions)` creates a
  table over a range — writing the `xl/tables/tableN.xml` part, the
  worksheet→table relationship, the worksheet `<tableParts>` entry and the
  `[Content_Types].xml` override. It validates the range, derives column names
  from the header row (or `TableOptions.Columns`), and supports a totals row with
  per-column functions/labels (`TableOptions.ColumnTotals`) and a built-in
  `TableStyle`. Tables in an opened workbook are parsed read-only; a table-bearing
  file that is not modified still round-trips byte-for-byte.
- pptx: slide **sections** (the groups shown in PowerPoint's thumbnail pane),
  stored in the presentation part's `p14:sectionLst` extension.
  `Presentation.Sections()` reads the existing sections in order; `Section`
  exposes `Name()`, `SetName()`, `ID()` (the section GUID), and `Slides()`
  (member slides in order). `Presentation.AddSection(name)` appends a new
  section, `Section.AddSlide(slide)` / `Presentation.MoveSlideToSection(slide,
  section)` assign a slide to a section (membership is by slide id, kept
  consistent with `sldIdLst`, and a slide belongs to at most one section). A
  presentation with existing sections round-trips byte-for-byte when unmodified,
  and a presentation without sections never gains an empty `sectionLst`.
- pptx: slide animation authoring and reading. `Slide.AddAnimation(shapeID,
  effect, trigger)` builds the `p:timing` main sequence for the common entrance
  (appear, fade, fly-in, wipe, zoom), emphasis (pulse, spin, grow/shrink), and
  exit (disappear, fade, fly-out) effects, with on-click / with-previous /
  after-previous sequencing (`AnimationEffect`, `AnimationTrigger`). The first
  animation builds a valid tmRoot→mainSeq tree; further animations append into
  the existing main sequence. `Animation.SetByParagraph` animates a text
  placeholder one paragraph at a time (adds a `p:bldP` build entry).
  `Slide.Animations()` reads existing animations back (effect, trigger, target),
  and `Shape.ID()` exposes the cNvPr id used to target a shape. A slide whose
  `p:timing` is never touched round-trips byte-for-byte, and media auto-play
  timing is unaffected.
- docx: style and numbering definition managers. `Document.Styles()` returns a
  `StyleManager` to create and modify paragraph/character styles —
  `AddParagraphStyle`/`AddCharacterStyle`/`AddStyle` return a chainable `Style`
  builder (id/name, `SetType`/`SetBasedOn`/`SetNext`/`SetLink`/`SetQuickFormat`/
  `SetUIPriority`, plus the style's paragraph properties `SetAlignment`/
  `SetSpaceBefore`/`SetSpaceAfter`/`SetLineSpacing`/`SetIndentLeft`/
  `SetIndentFirstLine`/`SetIndentHanging` and run properties `SetFont`/
  `SetFontSize`/`SetBold`/`SetItalic`/`SetColor`), with `Style(id)` to
  fetch/modify and `List()` to enumerate. `Document.Numbering()` (alias
  `ListDefinitions()`) returns a `NumberingManager` whose `AddDefinition()`
  builds a custom multi-level `ListDefinition`: `Level(i)`/`SetLevel(i, format,
  lvlText)` configure per-level `SetFormat` (decimal/bullet/lowerRoman/…),
  `SetText`, `SetStart`, `SetFont`, `SetIndent`/`SetHanging`, and `SetAlignment`,
  and `ListStyle()` returns a `ListStyle` that drives paragraphs through the
  existing `Paragraph.SetListStyle`. Created styles/numbering are written into
  `word/styles.xml`/`word/numbering.xml` on save; an unmodified styles or
  numbering part still round-trips byte-for-byte.
- docx: tracked-changes (revisions) read plus accept/reject.
  `Document.Revisions()` enumerates every tracked change in the document body
  in document order — descending into tables, hyperlinks, fields, and
  structured document tags — each `*Revision` reporting `Author`, `Date`,
  `Type`, and the affected `Text`. `Revision.Accept()`/`Reject()` and
  `Document.AcceptAllRevisions()`/`RejectAllRevisions()` transform the document:
  accepting an insertion (`w:ins`) unwraps it to normal runs and rejecting it
  removes the content; accepting a deletion (`w:del`/`w:delText`) removes it and
  rejecting it restores the text as normal runs; run- and paragraph-property
  changes (`w:rPrChange`/`w:pPrChange`) drop the change record on accept and
  revert to the recorded old properties on reject. Table/row/cell/section
  property changes, cell merges, and row/cell insertions/deletions are reported
  read-only (`Revision.Editable()` is false; `Accept`/`Reject` return an error);
  tracked moves (`w:moveFrom`/`w:moveTo`) are preserved but not enumerated. A
  revision-bearing file that is opened and saved without accept/reject still
  round-trips byte-for-byte.
- docx: content controls (structured document tags, `w:sdt`). The previously
  raw `w:sdtPr` is now typed — `w:alias`, `w:tag`, `w:id`, `w:lock`, and the
  control-type child (`w:text`, `w:richText`, `w:dropDownList`, `w:comboBox`,
  `w14:checkbox`, `w:date`, `w:picture`) — while any unmodeled child
  (`w:rPr`, `w:dataBinding`, `w:placeholder`, ...) is captured and replayed
  verbatim, so an unmodified content control still round-trips byte-for-byte.
  `Document.ContentControls()` returns every block-level and inline control in
  document order (descending into nested controls and tables); each
  `ContentControl` exposes `Tag`/`SetTag`, `Alias`/`SetAlias`, `ID`, `Type`,
  `Value`/`SetValue`, `Options` (drop-down/combo items), `DateFormat`,
  `Checked`, and `IsInline`. `Document.AddContentControl` and
  `Paragraph.AddContentControl` insert new rich-text controls.
- docx,xlsx: theme **read/write**, mirroring pptx's existing theme accessor for
  cross-format symmetry. `Document.Theme()` (docx) and `Workbook.Theme()` (xlsx)
  return a shared `dml.ThemeEditor` over the theme part
  (`word/theme/theme1.xml`, `xl/theme/theme1.xml` — the same DrawingML
  `a:theme`), with color-scheme accent read/write (`dk1`/`lt1`/`dk2`/`lt2`/
  `accent1`–`accent6`/`hlink`/`folHlink`) and font-scheme major/minor Latin
  typeface read/write. A file created from scratch (no theme part) returns nil,
  matching pptx. An unmodified theme round-trips byte-for-byte from its
  preserved source bytes; only an edited theme re-serializes on save.
- xlsx: conditional-formatting **write** (reading was already supported).
  `Sheet.AddConditionalFormat(cellRange, rules...)` takes typed rule
  constructors — `NewCellIsRule`, `NewExpressionRule`, `NewTextRule`,
  `NewDuplicateValuesRule`/`NewUniqueValuesRule`, `NewTop10Rule`,
  `NewAboveAverageRule`, `NewTimePeriodRule`, `NewColorScaleRule` (2- and
  3-color), `NewDataBarRule`, and `NewIconSetRule`. Rules that apply direct
  formatting allocate a deduplicated `x:dxf` in `x:dxfs` and wire `DxfId`;
  self-formatting rules carry their own. `ConditionalFormatRule.DifferentialFormat()`
  resolves a rule's `DxfId` to its fill/font/border on read. An unmodified
  conditional-format-bearing file still round-trips byte-for-byte.
- xlsx,docx,pptx: page & print setup, wiring already-modeled structures to public
  read+write accessors. xlsx `Sheet` gains `PageSetup`/`PageMargins`/
  `HeaderFooter`/`PrintOptions` (with setters) and `SetPrintArea`/`SetPrintTitles`
  (stored as the reserved `_xlnm.Print_Area`/`_xlnm.Print_Titles` defined names).
  docx `Section` gains `Columns` (multi-column layout), `PageNumbering` (format +
  start), `TitlePage`, and `SectionType`, plus `Document.Sections()`. pptx adds
  getters (`SlideFooter`, `SlideNumbersVisible`, `SlideDate`, `SlideDateIsAuto`)
  mirroring the existing furniture writers.
- docx,xlsx: protection. `Document.Protect`/`Unprotect`/`Protection` enforce
  Word's editing restrictions (`w:documentProtection`: read-only, comments,
  tracked-changes, or forms; optional formatting restriction and password),
  mirroring the shape of xlsx's sheet-protection API. `Workbook.Protect`/
  `Unprotect`/`Protection` set workbook structure/window locks
  (`x:workbookProtection`), and `CellStyle.Protection` exposes per-cell
  locked/hidden (effective when the sheet is protected). The legacy 16-bit
  password-obfuscation hash (weak by design, not encryption) is shared via
  `common/crypto`.
- docx,pptx,xlsx: rich-text formatting depth (additive; existing setters keep
  working). docx `Run` gains highlight, super/subscript, caps/small-caps,
  underline style + color, character spacing/position/kerning, and `Paragraph`
  tab stops. pptx `TextFrame` gains autofit and `Paragraph` gains auto-numbered
  bullets, bullet color/size/font, and indent/tabs. xlsx `FontStyle` gains
  strikethrough, sub/superscript, and underline styles (single/double/accounting),
  on both cell fonts and in-cell rich-text runs. Strike, underline styles, and
  super/subscript share a vocabulary across the three formats.
- chart: two new chart types and two formatting controls, all symmetric with the
  existing constructors and setters and read back through `Parse` and every
  format's `Charts()`. `NewDoughnut` builds a `c:doughnutChart` (a pie with a
  hole; single series) and `NewRadar` builds a `c:radarChart` (category and value
  axes, multi-series). `Chart.SetDataLabels(true)` emits `c:dLbls` (showVal) so
  each point's value renders on the chart, recovered via `Chart.DataLabels()`.
  `Series.SetColor("#1F77B4")` gives a series a solid RGB fill
  (`c:spPr`/`a:solidFill`/`a:srgbClr`; the leading `#` is optional), recovered as
  `Series.Color`. The new types and options flow through docx/pptx/xlsx
  `AddChart` unchanged, since every format serializes via `MarshalChartXML`.
- chart: combination charts. `NewCombo` builds a chart whose series render as
  mixed types on a shared category axis: give each series a plot type with
  `Series.SetType` (`KindColumn`, `KindLine`, or `KindArea`) and, optionally,
  move it to the secondary (right-hand) value axis with
  `Series.SetSecondaryAxis(true)`. Series are grouped by (type, axis) into the
  matching `c:barChart`/`c:lineChart`/`c:areaChart` groups; a secondary axis adds
  a right-crossing value axis and a hidden secondary category axis. `Charts()`
  reads a combo back as `KindCombo` with each series' `PlotType` and
  `SecondaryAxis` recovered (a non-combinable series type is rejected at marshal
  time). Flows through docx/pptx/xlsx `AddChart` unchanged.
- docx: charts. `Document.AddChart(c, widthEMU, heightEMU)` appends a paragraph
  holding an inline chart, and `Paragraph.AddChart(...)` places one inline among
  a paragraph's other runs. The chart's data is written to an embedded workbook
  (`word/embeddings/…xlsx`) that the chart part references via a package
  relationship, so Office can edit the values (a docx has no host worksheet); the
  chart's `c:f` references line up with the workbook's `Sheet1` ranges.
  `Document.Charts()` reads every chart in the document back into `chart.Chart`
  definitions. The chart part, embedded workbook, relationships, inline
  `w:drawing`, and content-type overrides are written on both the created and
  opened save paths; a zero-modification open→save of a chart-bearing document
  stays byte-identical (chart and embedding parts round-trip verbatim, and are
  regenerated only when a chart is added). `Validate` warns on a chart drawing
  whose relationship has no target part. `opc` gains `RelTypePackage`,
  `ContentTypeChart`, and `ContentTypeSpreadsheetPackage`.
- pptx: `Slide.AddChart(c *chart.Chart, x, y, width, height int64)` adds a chart
  to a slide (position and size in EMUs), and `Slide.Charts()` /
  `Presentation.Charts()` read the chart definitions back. This is the Phase B
  PowerPoint integration of the shared `chart` package: since a presentation has
  no host workbook, the chart's data is embedded as an `.xlsx` package under
  `ppt/embeddings/` (from `chart.EmbeddedWorkbook`) whose `Sheet1` ranges match
  the chart's `c:f` references, and the chart part serialized from
  `MarshalChartXML` references it via `c:externalData`. AddChart creates the
  chart part, the embedded workbook, the chart→workbook package relationship,
  the slide→chart relationship, the slide `p:graphicFrame`, and the content-type
  overrides — on both created and opened decks. The chart's graphic frame is
  appended through the shape sync, so it coexists with the slide's existing
  shapes; a zero-modification open→save of a chart-bearing deck preserves the
  chart and embedding parts byte-for-byte. `Validate()` warns when a chart
  graphic frame's relationship has no target part.
- chart: a new public, format-agnostic `chart` package for building DrawingML
  charts and serializing them to a `chart.xml` part (`c:chartSpace`) reusable by
  the xlsx, docx, and pptx integrations. Build a chart with a constructor
  (`NewColumn`, `NewBar`, `NewLine`, `NewPie`, `NewScatter`, `NewArea`),
  configure it (`SetTitle`, `SetCategories`, `AddSeries` / `AddXYSeries`,
  `SetLegend` / `HideLegend`, `SetAxisTitles`, `SetDataRef`), then serialize with
  `MarshalChartXML`. Cached values (`c:numCache` / `c:strCache`) are populated
  from the supplied data so the chart renders standalone, and `c:f` formula
  references are built against a configurable `DataRef` sheet (default
  `Sheet1`). `EmbeddedWorkbook` returns a minimal `.xlsx` (bytes) laid out to
  match those references — what docx/pptx charts embed — together with a
  `DataLayout` cell-range map. `Parse` reads a `chart.xml` back into the model.
  This is Phase A (the reusable core); wiring it into each format's Open/Save
  path (an `AddChart` method per format, a `Charts()` reader) is Phase B.
- xlsx: charts (Phase B). `Sheet.AddChart(anchor string, c *chart.Chart)` anchors
  a chart on a sheet at a cell ("E2") or range ("E2:L20"), and `Sheet.Charts()` /
  `Workbook.Charts()` read charts back as parsed `*chart.Chart` values (type,
  title, categories, series). An xlsx chart references the host workbook's cells
  rather than an embedded workbook: `AddChart` writes the chart's data into a
  dedicated hidden worksheet (one per chart) and points the chart's `c:f`
  references at it, so Excel's "Edit Data" opens real cells while the cached
  values render the chart standalone; the sheet's own cells are untouched.
  Charts and images coexist in one drawing part per sheet. Charts persist on both
  the `Create` and `Open` save paths, and a zero-modification open→save of a
  chart-bearing workbook stays byte-identical (chart and drawing parts preserved
  verbatim unless a chart is added or modified). `Validate` warns
  (`chart-target-missing`) on a drawing chart relationship whose target part is
  absent. To let the xlsx package import `chart` for these APIs,
  `chart.EmbeddedWorkbook`'s xlsx-backed builder is now installed via
  `chart.RegisterEmbedder` from the xlsx package (breaking a would-be import
  cycle); the public `EmbeddedWorkbook` method is unchanged.
- xlsx: read/write APIs for hyperlinks, images, sheet protection, and read
  accessors for the previously write-only feature surface. Hyperlinks are read
  and written through one `*Hyperlink` type sharing the cross-format surface
  (`URL`, `Anchor`, `Tooltip`, `SetTooltip`) plus an xlsx-specific `Ref` for the
  anchor cell: `Cell.Hyperlink()`, `Sheet.Hyperlinks()`, and
  `Cell.SetHyperlink(url)` (external, allocating an `External` relationship in
  the sheet rels on save) / `Cell.SetInternalHyperlink(location)` (an in-workbook
  jump, no relationship). `Sheet.Images()` enumerates worksheet drawing images as
  a `*Image` (`AltText`, `Data`, `ContentType`, plus xlsx-specific `AnchorCell`,
  `WidthEMU`, `HeightEMU`), matching the docx/pptx image readers. Sheet
  protection is read via `Sheet.Protection()` (a `*SheetProtection` reporting the
  effective locked/allowed state of each operation and whether a password guard
  is present) and written via `Sheet.Protect(SheetProtectionOptions)` /
  `Sheet.Unprotect()`, using Excel's documented legacy 16-bit password hash (a UI
  guard, not encryption). Read accessors were added for the write-only surface:
  `Sheet.MergedCells()`, `Sheet.FrozenPanes()`, `Sheet.AutoFilterRange()`,
  `Sheet.DataValidations()` / `Cell.DataValidation()`, `Sheet.ColumnWidth` /
  `ColumnHidden` and `Sheet.RowHeight` / `RowHidden`, and read-only
  `Sheet.ConditionalFormats()` surfacing cellIs/expression/colorScale/dataBar/
  iconSet/top10 rules. Every write persists on both the `Create` and `Open` save
  paths; a zero-modification open→save of a feature-bearing workbook stays
  byte-identical (only modified parts regenerate). `Validate()` warns on a
  hyperlink whose `r:id` has no matching relationship and on a data validation
  with a malformed `sqref` (#101).
- pptx: a hyperlink API symmetric with the docx/xlsx hyperlink APIs, plus a
  picture-enumeration reader. Hyperlinks are read and written through one
  `*Hyperlink` type sharing the cross-format surface (`URL`, `Anchor`,
  `Tooltip`, `SetTooltip`); pptx anchors are a destination slide number (for an
  internal slide jump) or a `ppaction://` verb (`ActionNextSlide`,
  `ActionPreviousSlide`, `ActionFirstSlide`, `ActionLastSlide`, `ActionEndShow`).
  Read: `Run.Hyperlink()`, a shape-level `Hyperlink()` on every shape (populated
  for text boxes, auto shapes, placeholders, and pictures), `Slide.Hyperlinks()`
  and `Presentation.Hyperlinks()` (descending into groups and table cells).
  Write: `Run`/`TextBox`/`AutoShape`/`PlaceholderShape`/`Picture` each expose
  `SetHyperlink(url)` (external, allocating an External relationship in the
  slide rels on save), `SetHyperlinkToSlide(index)` (an internal jump allocating
  a slide relationship), and `SetActionHyperlink(action)` (a `ppaction://` verb,
  no relationship). `Slide.Pictures()`/`Presentation.Pictures()` return every
  picture (excluding video/audio poster images), and `Picture` gains `AltText()`,
  `Data()`, and content-type resolution from the embedded media part.
  A zero-modification open→save of a hyperlink- or picture-bearing deck stays
  byte-identical; setting a hyperlink on a run in an opened slide patches that
  slide's node in place without regenerating the others. `Validate()` warns when
  a hyperlink references a slide relationship id with no matching relationship.
- docx: read/write APIs for hyperlinks, images, bookmarks, and footnotes.
  `Document.Hyperlinks()`, `Paragraph.Hyperlinks()`, and `Run.Hyperlink()` read
  links through a `*Hyperlink` type (`URL`, `Anchor`, `Tooltip`) shared with the
  xlsx/pptx APIs; `Paragraph.AddHyperlink`/`AddInternalHyperlink` and
  `Hyperlink.SetTooltip` write external (an `External` relationship) and internal
  (`w:anchor`) links. `Document.Images()` returns every inline and floating image
  with read accessors (`AltText`, `Width`/`Height`, `ContentType`, `Data`,
  `PartName`, `Floating`). `Document.Bookmarks()` reads bookmarks (`Name`,
  `Text`); `Paragraph.AddBookmark` and `Document.AddBookmarkOnRange` write them,
  and internal hyperlinks can target them by name. `Document.Footnotes()` /
  `Document.Endnotes()` read notes through a `*Footnote` type (`ID`, `Text`,
  `Paragraphs`); `Run.AddFootnote` / `Run.AddEndnote` insert the reference in the
  run stream and append the note to `word/footnotes.xml` / `word/endnotes.xml`,
  creating the part — with the mandatory separator notes, the relationship, and
  the content-type override — on first use across both save lifecycles. `Validate`
  warns on an internal hyperlink anchoring to an absent bookmark and on a
  footnote/endnote reference with no matching note. A zero-modification open→save
  of a document using any of these features stays byte-identical; parts are
  regenerated only when the feature is modified.
- pptx: a slide comments API symmetric with the docx/xlsx comment APIs.
  `Slide.Comments()` and `Presentation.Comments()` read both PowerPoint comment
  mechanisms — legacy per-slide comments and modern (2018) threaded comments —
  through one `*Comment` type (`ID`, `Author`, `Text`, `Date`, `Resolved`,
  `Replies`, `Parent`, plus pptx-specific `Slide`, `Position`, and
  `AnchorShapeID`). `Slide.AddComment`/`AddCommentAt`, `Comment.Reply`, and
  `Comment.Resolve`/`SetResolved` write modern threaded comments (the mechanism
  current PowerPoint emits and the only one supporting replies and resolution),
  registering authors in `ppt/authors.xml` deduplicated by name. Legacy comments
  are read-only for threading/resolution (documented no-ops). A
  zero-modification open→save of a comment-bearing deck stays byte-identical;
  only the parts a comment write touches are regenerated, and unmodeled modern
  data (anchor marker lists, task details, reactions) is preserved verbatim.
  `Validate()` warns when a comment references an author with no matching entry.
- xlsx: a comments API covering both SpreadsheetML comment mechanisms through
  one unified `Comment` type. `Sheet.Comments()` and `Cell.Comment()` read
  legacy notes (`xl/comments*.xml` + VML drawing) and modern threaded comments
  (`xl/threadedComments/*` + `xl/persons/*`), merging them so a cell's legacy
  back-compat note is not double-reported alongside its thread. `Comment`
  exposes the cross-format shared surface (`ID`, `Author`, `Text`, `Date`,
  `Resolved`, `Replies`, `Parent`) plus xlsx-specific `Ref` and `Threaded`.
  `Cell.AddComment(author, text)` / `Sheet.AddComment(ref, author, text)` create
  a threaded comment together with a legacy-note fallback (matching modern
  Excel, so old Excel still renders the text) — allocating the comments,
  threaded-comments, person-list and VML-drawing parts, their relationships and
  content-type overrides, and the worksheet `<legacyDrawing>` reference, and
  registering the author as a person (deduplicated by display name).
  `Comment.Reply`, `Comment.Resolve`/`SetResolved`, and `Sheet.AddNote` (a
  legacy-only note) complete the write surface. A zero-modification open→save of
  a comment-bearing workbook stays byte-identical (comment parts preserved raw);
  only a touched sheet's comment parts are regenerated. Comments coexist with an
  image/drawing on the same sheet (distinct VML/legacy vs DrawingML drawings).
  `Validate()` warns on a threaded comment whose `personId` has no matching
  person and on a comment anchored to an unparseable cell ref.
- docx: a comments API for the full review flow — read, add, reply in threads,
  and resolve. `Document.Comments()` returns every comment; each `Comment`
  exposes `ID`, `Author`, `Initials`, `Text`, `Date`, `Paragraphs`, `Resolved`,
  `Replies`, `Parent`, and `AnchorText` (the document text the comment brackets).
  Writers: `Paragraph.AddComment(author, text)` anchors over a paragraph,
  `Run.AddComment` over a single run, and `Document.AddCommentOnRange(start, end,
  author, text)` over an arbitrary run span; `Comment.Reply(author, text)` adds a
  threaded reply and `Comment.Resolve()` / `SetResolved(bool)` set the thread's
  done state. The core method set (`ID`/`Author`/`Text`/`Date`/`Resolved`/
  `Replies`/`Parent` plus `AddComment`/`Reply`/`Resolve`) matches the xlsx and
  pptx comment APIs. Spine writes and round-trips the ECMA-376 comments part
  along with the Microsoft extensions Word relies on — `commentsExtended.xml`
  (threading and resolved state, keyed by `w14:paraId`), `people.xml` (author
  registry) — creating each part with its relationship and content-type override
  on both the new-document and opened-document save paths, and preserves
  `commentsIds.xml`/`commentsExtensible.xml` verbatim. A zero-modification
  open→save of a comment-bearing document is byte-identical; only added or
  modified comments regenerate the affected parts. New comment ids are the
  highest existing id plus one and `w14:paraId` values are collision-free 8-hex
  ids. `Validate()` warns on a `commentRangeStart`/`commentReference` with no
  matching comment.
- testdata: a second, fully distinct 10k-each docx/xlsx Common Crawl corpus
  under `testdata/cc/stress/` for library stress testing — a fresh batch of
  real-world files that shares **no `content_digest`** with the canonical set,
  so `ccrun` exercises 10k new docx and 10k new xlsx without re-testing anything
  already covered. Swept from the same six crawls, deduplicated across them, at
  the same `-d 15` per-domain cap, with every canonical docx/xlsx digest
  excluded up front. `sweep-multi.sh` gained two flags to make this
  reproducible: `-T <types>` sweeps/emits a subset of `pptx,xlsx,docx`, and
  `-x <digest-file>` excludes a list of `content_digest`s from every emitted
  manifest and from the early-stop count (a DuckDB anti-join). `ccrun`/`ccfetch`
  read the stress manifests via `-manifest testdata/cc/stress` unchanged (#92).
- testdata,tools: a scaled, batched Common Crawl harvest pipeline that commits
  only *references* (crawl id + WARC filename/offset/length + content_digest +
  URL), never binaries. Shared harvest logic (WARC range fetch/decode,
  OPC/OOXML classification, the DoH blocklist gate, digest helpers) moved into
  a stdlib-only `internal/ccharvest` package. `testdata/cc/sweep-multi.sh`
  sweeps several recent crawls, deduplicates across them by `content_digest`,
  and writes 10k/type manifests with a self-describing `crawl` column (the
  committed set: docx/xlsx 10000 each from 4 crawls at `-d 15`; pptx 10000 from
  6 crawls at `-p 25`, since pptx is scarcer and diversity-limited by the
  per-domain cap). A new
  `tools/ccrun` runner processes those manifests one bounded batch per
  invocation: it fetches, tests (`Open`/`Validate`/`SaveBytes`/reopen/part
  fidelity), records the outcome to a durable resumable ledger, catalogs
  failures in a reference-keyed quarantine, and discards the binary. Each file
  is tested in a separate worker subprocess so a pathological file — under a
  `systemd-run` `MemoryMax` cgroup — is OOM-killed and recorded as one
  quarantine row instead of killing the batch (`make harvest-sweep`,
  `make harvest-batch`; see testdata/cc/README.md) (#89).

- validation: each format's top-level type (`docx.Document`, `xlsx.Workbook`,
  `pptx.Presentation`) exposes `Validate() validate.Report`, a pre-save pass
  over the in-memory model that reports structural problems as a slice of
  structured findings (`common/validate.Error`: stable `Code`, `Severity`
  error/warning, `Part`, human `Detail`). `Save`/`SaveBytes`/`SaveTo` run it
  first and refuse to write when any error-severity finding is present, so a
  structurally corrupt package is never produced; `SaveToUnvalidated` bypasses
  the gate for advisory cases. Error-severity checks (no false positives across
  the 3,600-file corpus): duplicate shape ids per slide, dangling
  sldLayoutId/sheet/header/footer references, orphaned shared-formula
  followers, duplicate sheetId, out-of-range definedName scope, overlapping
  merged ranges, numPr referencing an undefined numbering definition, and
  duplicate part names. Missing relationship targets, parts without a content
  type, dangling image/hyperlink references, and undefined style references are
  reported as warnings (Office tolerates them) (#88).

- docx,xlsx,pptx: `Open` accepts every ECMA-376 main-part flavor of its
  family — templates (.dotx/.xltx/.potx), slideshows (.ppsx), and the
  macro-enabled variants (.docm/.dotm, .xlsm/.xltm/.xlam,
  .pptm/.ppsm/.potm) — instead of only the regular document type, and a
  save re-emits the recorded flavor so e.g. an .xlsm is not silently
  retyped to .xlsx while still carrying its `vbaProject.bin`; the new
  `Flavor()` accessor reports the main part's content type, and the flavor
  content types are exported as `opc.ContentType*` constants. Converting a
  file to another flavor remains out of scope (#86).
- opc: `ReaderOptions` with `NewReaderWithOptions`/`OpenReaderWithOptions`
  override the decompression limits for a single Reader; the package-level
  variables remain the documented defaults (#85).
- docx: public field API (PAGE/NUMPAGES), table of contents, floating and
  anchored images, and SVG images with a raster fallback (#54, #56).
- pptx: slide furniture API (footers, dates, slide numbers) and
  `AddPictureFromBytes` (#55).
- xlsx: two-cell anchors, aspect-preserving scaling, and SVG images;
  image embedding into opened workbooks; and rich text within a cell
  (`SetRichText`/`RichText`) (#57, #58).
- pptx: read-only `Placeholders()` and `Theme()` (color and font schemes)
  on slide masters and layouts (#71).
- xlsx: exported builtin number-format ids (#73).
- common/omml: a typed model of the full Office Math (OMML) element set —
  math zones and paragraphs, runs, fractions, radicals, scripts, n-ary
  operators, delimiters, equation arrays, matrices, functions, limits, and
  the accent/bar/box family — with ordered content sequences, in-position
  raw capture of WordprocessingML and unknown children, and Builder-based
  serialization; docx gains `Paragraph.MathZones`/`MathParas` for typed
  on-demand access to stored equations and `Paragraph.AddMath`/`AddMathPara`
  for writing typed math, while raw bytes remain the storage format so byte
  fidelity is unaffected (#75).
- testdata: a Common Crawl OOXML corpus pipeline — committed manifests
  pinning real-world candidates per format from crawl CC-MAIN-2026-25
  (`testdata/cc/sweep.sh`), a resumable stdlib-only fetcher
  (`tools/ccfetch`, `make fetch-cc`) that validates and classifies each
  WARC payload — plus a gated live mode that refetches candidates the
  crawler truncated at 1 MiB from their origin, screened through a
  DNS-over-HTTPS blocklist resolver — and a corpus test (`cctest`) running
  open/save/reopen/part-fidelity over every fetched file with a committed
  quarantine of known failures; the corpus itself stays local and
  gitignored (#76).

### Removed

- pptx dead option/export API with no consumers: `OpenOptions`,
  `SaveOptions`, `ExportFormat`/`ExportOptions`, and the unused create
  options (#71).
- `common/docprops`: unused, namespace-incorrect duplicate of the opc
  core-properties support (#74).
