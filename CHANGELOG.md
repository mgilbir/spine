# Changelog

## Unreleased

User-visible changes from the feature series (#54–#58) and the audit
remediation series (#59–#75).

### Fixed

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

### Added

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
