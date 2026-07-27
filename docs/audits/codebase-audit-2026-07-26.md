# Codebase audit — 2026-07-26

Third full adversarial pass over spine (Go library reading/writing OOXML —
docx/xlsx/pptx — over an OPC core). Method: 16 parallel area auditors, each
reading its files in full and adversarially verifying every candidate (runtime
probes for criticals/highs), deduped against the prior audits (C1–C235 on
2026-07-07 and 2026-07-11; D1–D49 docs on 2026-07-07 and 2026-07-26). New IDs
continue at **C236**. Baseline health at HEAD: `go build ./...`, `go vet ./...`,
`go test ./...` (all 24 packages), and `make lint` (golangci-lint, **0 issues**)
are all green; all 10 examples and all doc snippets compile and run. Every
finding below is a defect *within* that green baseline — the test suite does not
exercise these paths.

Status key: **CONFIRMED** = fully traced (many reproduced with a runtime probe,
noted inline); **PLAUSIBLE** = mechanism confirmed, trigger not reproduced.
Novelty: **NEW**, or **KNOWN(Cnn)** = still present since a prior audit.

---

## 1. Summary table

Severity order within each block. Area key: `opc`, `xml` (common/xml), `dml`
(common/dml), `crypto` (common/crypto+vml+omml), `docx`, `docx-oxml`
(docx/internal/oxml), `xlsx`, `xlsx-oxml`, `pptx`, `pml` (pptx/internal/oxml),
`chart`, `dx` (docs/tooling).

### Critical

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C236 | pptx | Fresh-master import writes duplicate relationship Ids in the imported master's .rels — every real deck's `ExtractSlides`/`AppendSlidesFrom` output is OPC-invalid (PowerPoint repair) | pptx/merge.go:312-356; presentation.go:1548-1624 | CONFIRMED-runtime |
| C237 | pptx | `importMaster`/`addImportedLayout` drop all non-theme master/layout rels — master/layout image backgrounds lost, `r:embed` re-bound to the theme part | pptx/merge.go:289-388 | CONFIRMED-runtime |

### High (selected — full list in §3)

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C238 | xml | c14n tree builder panics (index out of range) on a multi-root signature part — DoS via untrusted opened package | common/xml/c14n.go:179 | CONFIRMED-runtime |
| C239 | xml | windows-1252/latin1 parts corrupt every InputOffset-based fidelity capture — garbage bytes replayed into regenerated docx/pptx/xlsx parts | common/xml/emptystyle.go:49-55; charset.go:26-30 | CONFIRMED-runtime |
| C240 | chart | docx charts never emit `c:externalData` — embedded workbook orphaned, Word "Edit Data" broken (docs claim otherwise) | docx/chart.go:102-106 | CONFIRMED-runtime |
| C241 | xlsx | Any edit to a workbook containing a chartsheet corrupts its wiring; writing a cell overwrites the part with a `<worksheet>` root | xlsx/workbook.go:314-363,1074-1115 | CONFIRMED-runtime |
| C242 | docx-oxml | `AcceptAllRevisions`/`RejectAllRevisions` wipe the content of API-built fields and hyperlinks (childOrder read without backfill) | docx/internal/oxml/revisions.go:153-247 | CONFIRMED-runtime |
| C243 | pptx | `CreateFromTemplate` leaks cleared slides' rels — new slides inherit the template slide's notes (data leak) and package keeps dangling parts/overrides | pptx/presentation.go:809-835 | CONFIRMED-runtime |
| C244 | pptx | Editing one run rebuilds the whole `a:txBody` from the lossy model — `<a:br>`/`<a:fld>`/`endParaRPr`/explicit-off run props dropped | pptx/shape_sync.go:226-234 | CONFIRMED-runtime |

*(Full high/medium/low tables are inlined per finding in §3; this header lists
only the standouts a reader should see first.)*

---

## 2. System map

**Layering** (bottom-up): `common/xml` (Builder + reflection marshaler, the
write substrate; stdlib-only) → the shared typed models (`common/dml` and its
`chart`/`diagram` children, `oxml`, `omml`, `enum`, `vml`) and `common/crypto`
→ `opc` (parts, relationships, `[Content_Types].xml`, signatures, CFB-wrapped
encrypted containers) → `common/validate` → the format packages
(`docx`/`xlsx`/`pptx`, each with an `internal/oxml` typed model) and the public
`chart` builder.

**Real execution paths.** Two lifecycles, and nearly every serious finding
lives on the *Open* one:

- **Create → Save (`saveNew`)**: every part generated from the model. Simpler;
  the bugs here are id-allocation (C236, C245-class rel collisions) and
  inherit-vs-explicit zero-value emission (C244, algn/insets).
- **Open → mutate → Save (`saveRoundTrip`)**: untouched parts copied verbatim;
  touched parts regenerated from the model. The dominant defect class is
  **regeneration-without-lossless-capture**: a typed model narrower than the
  schema silently drops the unmodeled remainder the instant one sibling is
  edited (C241, C244, xlsx-oxml data-table/filter/extLst family, dml effect
  containers). The second class is **mutation-not-flagged**: an edit through a
  handle into a preserved part (headers/footers, core-props psmdcp) never marks
  the part dirty, so the preserved bytes win and the edit vanishes.

**Key invariants** (asserted, and where they break):
1. *Byte-identity for untouched content* — holds for never-accessed parts
   (verbatim passthrough); **weaker than documented** for accessed-but-unedited
   pptx/docx parts, which re-marshal and can drift on whitespace-preserving
   producers (architecture.md overstates this — see docs findings).
2. *childOrder tracks every child's position* — maintained by marshal-path
   mutators, but **read without backfill** by the revisions/moves transforms
   (C242) and violated by post-parse property additions that append out of
   schema order.
3. *Edits win over captured fidelity* — true for value changes, **false for
   deletions**: clearing a modeled attribute lets the captured original replay
   (ReplayCapturedAttrs, independently confirmed at common/xml/marshal.go:460).
4. *Relationship ids are unique per rels scope* — held by the main-part
   allocator, **violated** when one global counter feeds per-part header/master
   scopes (C236, docx nextRelID, xlsx VBA-rel).

---

## 3. Findings

Several defects were reported independently by two area auditors; those are
merged under one ID with both surfaces noted. "runtime" = reproduced with a
throwaway probe against the public API.

### Critical

**C236 — pptx — Fresh-master import writes duplicate relationship Ids.**
`pptx/merge.go:312-356` + `presentation.go:1548-1624`. `importMaster` pre-registers
the imported master's layout/theme/presentation rels, but `saveNew` then rebuilds
those same rels *and* re-appends generated layout rels with the identical
`layout.relID`s plus a second theme rel. Probe: `ExtractSlides([]int{0})` from a
deck whose master has a background image → `slideMaster2.xml.rels` contains
rId1–rId6 twice each; `AppendSlidesFrom` into a `Create()` deck adds duplicate
theme and presentation→master rels. Duplicate `Id` violates OPC xsd:ID
uniqueness → PowerPoint repair, on **every** source master not byte-identical to
the destination (i.e. every real deck). CONFIRMED-runtime. NEW. *Fix:* saveNew
should dedupe against rels already in `p.relationships[masterPartName]`, or
importMaster should not pre-register when the destination saves from scratch.

**C237 — pptx — Master/layout non-theme rels dropped; image backgrounds lost.**
`pptx/merge.go:289-388`. Unlike the notes/handout master importers (which carry
all non-theme targets via `importPart`), `importMaster`/`addImportedLayout` copy
only layouts + theme. A master/layout with an image relationship is cloned
verbatim with its `r:embed="rIdN"`, but the image part is never imported and the
new rels bind that id to `theme2.xml`. Probe: merged master keeps
`r:embed="rId7"` while rId7 now points at the theme part; the PNG is absent from
the package. CONFIRMED-runtime, both created and opened destinations. NEW.
*Fix:* import the master's/layout's remaining rels through `importPart`, as the
notes/handout importers already do.

### High

**C238 — xml — c14n tree builder panics on a multi-root signature part.**
`common/xml/c14n.go:179`. After the first root closes, a second top-level element
hits `parent := stack[len(stack)-1]` with an empty stack → index out of range.
`opc/signature.go:297` calls `ParseC14N` on the raw signature part of any opened
package; a part `<Signature>…</Signature><x/>` passes the preceding `Unmarshal`
(which reads one element) and panics during verification. `Canonicalize([]byte("<a/><b/>"))`
panics. CONFIRMED-runtime. NEW. *Fix:* return an error on a second root instead
of indexing.

**C239 — xml — Transcoded charsets corrupt every InputOffset-based capture.**
`common/xml/emptystyle.go:49-55`, `charset.go:26-30`. When the prolog declares
windows-1252/iso-8859-1, `CharsetReader` wraps the reader in a transcoder, so
`d.InputOffset()` indexes the transcoded stream while the raw buffer is the
original bytes — every high byte shifts offsets. Probe: a windows-1252 part with
four `0xE9` bytes made `UnmarshalOrderedChildren` capture an unknown child as
`nown x="2"/></ro` (garbage) which is then replayed verbatim into the regenerated
part. This is the mainline parse for docx document.xml, pptx slides/presentation,
and xlsx workbook. charset.go's comment claims such files "are handled by
preserved raw bytes" but nothing enforces it. CONFIRMED-runtime. NEW. *Fix:* in
`UnmarshalWithSource`, skip source registration whenever the decoder wraps the
reader, so capture helpers degrade to their no-source path.

**C240 — chart — docx charts never emit `c:externalData`.**
`docx/chart.go:102-106` (vs `pptx/chart.go:149-161`). `Document.AddChart` writes
the embedded .xlsx and a package rel, but the chart.xml has no
`<c:externalData r:id="…">`. Probe: saved chart1.xml has no externalData while
its .rels carries the package relationship — Word's "Edit Data" cannot find the
workbook and the embedded part is orphaned dead weight. docs/charts.md:153-155
claims "the chart part references it, so Office can open and edit the values" —
false. CONFIRMED-runtime. NEW. *Fix:* inject externalData at the schema-correct
position as the pptx path does (hoist into a shared helper).

**C241 — xlsx — Any edit to a workbook containing a chartsheet corrupts it.**
`xlsx/workbook.go:314-363,1074-1115`. `loadSheets` treats every `<sheet>` entry
as a worksheet (the custom UnmarshalXML skips root-name checking), so a chartsheet
parses into `CT_Worksheet`. Editing a cell on *any* worksheet makes
`rebuildWorksheetRelationships` emit a second `Type=…/worksheet` rel for the
chartsheet's target while [Content_Types] still says chartsheet — both a
chartsheet and a worksheet rel for one part. Writing a cell on the chartsheet
handle replaces its part bytes with a `<worksheet>` root. Dialogsheets/macrosheets
share the class (grep confirms zero handling anywhere). CONFIRMED-runtime. NEW.
*Fix:* detect non-worksheet `<sheet>` entries at load and model them as opaque
preserved sheets excluded from rel rebuilding and the mutation surface.

**C242 — docx-oxml — Accept/RejectAllRevisions wipe API-built fields & hyperlinks.**
`docx/internal/oxml/revisions.go:153-247`. `itemsOf` reads only `*refs.childOrder`;
a container whose typed children were appended without a childOrder entry
(`field.go:39`, `hyperlink.go:175`) yields zero items, so `AcceptAllInContainer`
rebuilds it empty. Probe: `Create(); p.AddText("Dear "); p.AddMergeField("FirstName");
p.AddHyperlink(...); doc.AcceptAllRevisions()` → paragraph text goes from
`"Dear «FirstName»example"` to `"Dear "`; the hyperlink text is gone. New,
unfixed call site of the C153 "childOrder-gating without backfill" family.
CONFIRMED-runtime. NEW. *Fix:* backfill childOrder from typed slices before
reading (mirror `backfillChildOrder`).

**C243 — pptx — `CreateFromTemplate` leaks cleared slides' rels (notes data leak).**
`pptx/presentation.go:809-835`. The method does only `p.slides = nil;
p.presentation.SlideIDs = nil` — never deleting `p.relationships[slidePart]` or
running `RemoveSlide`'s cleanup. A new `AddSlide` reuses `/ppt/slides/slide1.xml`
and inherits the template slide's surviving notesSlide rel wholesale. Probe:
template with `SetNotes("SECRET template notes")` → CreateFromTemplate → AddSlide
→ Save → reopen: the brand-new slide reports `Notes()=="SECRET template notes"`,
and the orphan notesSlide1.xml + its dangling [Content_Types] overrides survive.
CONFIRMED-runtime. NEW (C88 fixed only the RemoveSlide entry point). *Fix:* route
CreateFromTemplate's slide clearing through RemoveSlide's cleanup path.

**C244 — pptx — Editing one run rebuilds the whole `a:txBody`, dropping br/fld/props.**
`pptx/shape_sync.go:226-234` + `oxml_to_domain.go:672-755`. `updateTxBody` replaces
`body.P` wholesale from `paragraphToOxml`, but `oxmlToParagraph` reads only `p.R`
— `dml.P` models `Br`/`Fld`/`EndParaRPr` with interleaved order, which the pptx
materializer ignores. Probe: loaded slide with `run — <a:br/> — run`; `Runs()[0].
SetBold(true)` → saved paragraph has two adjacent runs, the line break gone and
the lines merged. Same flush drops explicit `b="0"`/`i="0"` and color transforms.
`Paragraph.Text()` is consistently lossy (skips field/break text). CONFIRMED-runtime.
NEW. *Fix:* materialize Br/Fld/EndParaRPr (and child order), or patch only dirty
runs in place.

**C245 — xlsx — `AddComment` regenerates the sheet VML and destroys form controls.**
`xlsx/comment_opened.go:49`, `comment_vml.go:19`. `writeSheetComments` drops the
preserved VML part and rewrites it via `buildCommentVML`, which emits only note
shapes. Probe: a sheet with a checkbox form control + `AddComment("D4",…)` →
saved vmlDrawing1.vml has no checkbox, while the worksheet still carries
`<control … shapeId="1025">` — which now resolves to the comment note box
(assignShapeIDs also starts at 1025). Control lost + shape-id cross-wiring.
CONFIRMED-runtime. NEW. *Fix:* preserve non-comment shapes when regenerating VML
and pick shape ids above the existing max.

**C246 — xlsx — Hyperlink handles dangle after later mutations; tooltip lands on wrong cell.**
`xlsx/hyperlink.go:188-215`. `SetHyperlink` returns a handle holding `&hl[len-1]`
into a value slice; subsequent `SetHyperlink`/`removeHyperlinkForRef` reallocate
or compact it. Probe 1: tooltip set after five more links is absent from output.
Probe 2: replace A1's link, then set B1's tooltip via a pre-replacement handle →
output `<hyperlink ref="A1" … tooltip="B-TIP"/>` — the tooltip lands on the other
cell's link. Same class as the fixed C11 (cells became `[]*CT_Cell`; hyperlinks
did not). CONFIRMED-runtime. NEW. *Fix:* store `[]*CT_Hyperlink` or re-resolve
the handle by ref on each access.

**C247 — pptx/pml — Root-attribute verbatim replay silently discards modeled writes.**
`pptx/internal/oxml/root_marshal.go:222-554` + `pptx/marshal.go:121-125`. All four
part roots do `if OriginalRootAttrs != nil { StartElementWithRootAttrs(...) }`,
which replays the capture verbatim with no override — unlike `ReplayCapturedAttrs`,
where modeled values win. Every opened deck has `OriginalRootAttrs` set, so root
setters are no-ops. Probes: `SlideLayout.SetName("X")` → saved `matchingName` still
old; `Presentation.SetEmbedTrueTypeFonts(true)` → no `embedTrueTypeFonts` attr at
all, so `EmbedFont` writes font parts PowerPoint ignores. Latent for
`Slide.Show`, `SlideLayout.Type/Preserve`, all `Presentation` attrs. CONFIRMED-runtime
(both areas). NEW (inverse of the fixed C4). *Fix:* merge modeled root-attr values
into the captured list (modeled-authoritative), as ReplayCapturedAttrs does below
the root.

**C248 — chart — xlsx bubble charts write the data sheet in the wrong layout.**
`xlsx/chart.go:142-174`, `chart/refs.go:97-111`. `writeChartData` has only the
category/scatter layouts, but `MarshalChartXML` uses `bubbleLayout` (X in A,
per-series Y/size column pairs). Probe: refs are `xVal=$A$2:$A$3`, S1 sizes
`$C$2:$C$3`, but the data sheet holds nothing in A and S2's Y in C. Cached values
render; "Edit Data" shows wrong data and any refresh corrupts the chart.
CONFIRMED-runtime. NEW. *Fix:* add a bubble branch to writeChartData (share one
layout-driven writer).

**C249 — chart/xlsx — Adding a chart/image to an opened sheet with an existing drawing removes the existing charts/images.**
`xlsx/image_opened.go:81-117`. `saveOpenedSheetAttachments` re-points the sheet's
single `<drawing>` at a freshly generated part containing only session-added
anchors; the original drawing/chart parts remain unreferenced. Probe: 1 chart
before → after `AddChart`+save+reopen only the new chart is visible. Contradicts
`xlsx/chart.go:44` and docs/charts.md:146 ("works on both created and opened
workbooks"). AddChart's godoc omits the caveat AddImage documents. CONFIRMED-runtime.
NEW. *Fix:* parse the preserved wsDr and append new anchors to it.

**C250 — chart — Sparse caches collapse/misalign; series values shift vs categories.**
`chart/parse.go:536-550,482-506`. Excel omits `c:pt` for blank cells; `numPoints`
sizes output by `len(nd.Pt)` and remaps `idx>=len(out)` to positional `i`. Probe:
cache `ptCount=3` with pts idx 0 and 2 parses to `Values=[10 30]` against
`Categories=[a b c]` — 30 aligns with "b" instead of "c". Any real chart over data
with blanks reads back wrong. CONFIRMED-runtime. NEW. *Fix:* size output from
`PtCount` and leave absent points as placeholders.

**C251 — chart — Multi-series scatter/bubble: all X refs point at series 0's column.**
`chart/refs.go:84-88`, `data.go:77-82`, `xlsx/chart.go:146`. `AddXYSeries` accepts
distinct X per series and caches each, but every series' `c:f` is
`Sheet1!$A$2:$A$4` and only series[0]'s X is written to A. Probe: cache says S2
X=100/200/300, referenced cells say 1/2/3 — renders right until Office refreshes,
then S2's points jump. CONFIRMED-runtime (upgrades the xlsx auditor's PLAUSIBLE).
NEW. *Fix:* give each scatter series its own X column, or validate shared X.

**C252 — docx — `Append` leaves dangling/aliased rels for chart/OLE/ActiveX content.**
`docx/merge.go:98-127,468-486`. `importRelationships` copies only external and
`RelTypeImage` rels; everything else is skipped while the copied body keeps its
`r:id`. Probe: `dst.Append(chart.docx)` → spine's own validator reports
`dangling-rel … chart drawing references "rId14"`; the chart part is absent from
the zip. A freshly assigned id can also equal an un-remapped source id, aliasing
the source's chart onto an imported image rel. docs/docx.md:71 promises "no
dangling references". CONFIRMED-runtime. NEW. *Fix:* import (or strip) every
internal rel the copied content references; allocate remap targets disjoint from
surviving source ids.

**C253 — docx — `Append` doesn't merge footnotes/endnotes/comments; refs alias.**
`docx/merge.go:51-93`. Note/comment reference runs survive the copy but Append
never copies the source's footnotes.xml/endnotes.xml/comments.xml or remaps the
`w:id`s. Merging two docs that each have footnote id 2 makes the appended text's
mark display the destination's footnote 2 — wrong content, no warning
(validateNoteRefs fires only on an absent id, not an aliasing one). CONFIRMED
(mechanism unambiguous). NEW. *Fix:* import note/comment parts with id remapping,
or strip the reference runs and document the limitation.

**C254 — docx/docx-oxml — `MarkDeleted` on a non-top-level run emits invalid `w:delText`.**
`docx/revisions.go:95-99`, `docx/internal/oxml/revisions.go:386-389`.
`WrapRunDeletion` calls `convertTextToDelText()` before checking the run is a
direct paragraph child; for a run from `Hyperlink.Runs()` the wrap fails but the
conversion sticks and both bools are discarded. Probe:
`Hyperlinks()[0].Runs()[0].MarkDeleted("x")` saves `<w:hyperlink><w:r><w:delText>…`
— `w:delText` outside any `w:del` is invalid WML, the text vanishes from
extraction, and no revision exists to accept/reject. `MarkInserted` is a silent
no-op. CONFIRMED-runtime. NEW. *Fix:* check membership before mutating; surface
the bool.

**C255 — pptx — `saveNew` relID reassignment corrupts `AddCustomShow` references.**
`pptx/presentation.go:1477-1486`, `presentation_lists.go:257-267`. `AddCustomShow`
snapshots `s.relID`, but `saveNew` reassigns every slide's relID by current order.
Probe: Create → AddSlide s1,s2 → `AddCustomShow("show", s1)` (records rId2) →
`MoveSlide(0,1)` → Save: sldIdLst emits s1 as rId3 while custShow still says
rId2 — the show plays the wrong slide (an `EmbedFont` first makes rId2 resolve to
the slideMaster). CONFIRMED-runtime. NEW. *Fix:* stop reassigning relIDs in
saveNew (eager ids are already collision-safe), or remap CustShowLst through the
old→new table.

**C256 — pptx — `Duplicate` before save yields a picture with no embed.**
`pptx/slide.go:1489-1534`, `image_replace.go:19-42`. Pending image data is embedded
only at marshal time, but Duplicate snapshots the XML after `syncShapesToXML` (blip
with empty relID) and before embed. Probe: `AddPictureFromBytes` → Duplicate → Save
→ slide1 gets `<a:blip r:embed="rId2"/>` + rel; slide2 gets bare `<a:blip/>` and
no image rel — a broken frame, contradicting docs/pptx.md:16. CONFIRMED-runtime.
NEW (C193 fixed only autoplay timing for this ordering). *Fix:* run
`processPendingImages` before the Duplicate snapshot.

**C257 — xlsx — Double `Save` after `AddTable` emits duplicate `<tablePart>` with duplicate r:id.**
`xlsx/table.go:437-460`. `writeSheetTables` appends to the durable
`ws().TableParts` and `sheet.newTables` is never cleared. AddTable → SaveBytes →
SaveBytes yields `<tableParts count="2"><tablePart r:id="rId1"/><tablePart
r:id="rId1"/>` in the second output (both Create and Open paths) — invalid package,
and the model keeps growing each save. Save-to-disk-then-SaveBytes is mainstream.
CONFIRMED-runtime. NEW. *Fix:* build tableParts into a per-save scratch copy or
clear `newTables` each pass (mirror `pendingPivotCaches`).

**C258 — xlsx — `saveNew` gives the VBA relationship a stale counter id, colliding.**
`xlsx/workbook.go:986-1015`. The person-list rel is appended without incrementing,
pivot rels use scan-based `ensureRelationship`, but the VBA rel uses the literal
`fmt.Sprintf("rId%d", nextRelID)`. Probe: Create → AddPivotTable → SetVBAProject →
Save → `Id="rId3"` twice in workbook.xml.rels. CONFIRMED-runtime. NEW. *Fix:*
allocate VBA/person rel ids via `nextRelationshipID(relIDSet(wbRels))`.

**C259 — docx-oxml — Nested `m:oMath` inside ins/del/hyperlink/fldSimple/run-SDT is deleted.**
`docx/internal/oxml/paragraph.go:96-118,711-724`. CT_P handles oMath/oMathPara, but
the shared `unmarshalPContent` (used by CT_Hyperlink, CT_RunTrackChange,
CT_SimpleField, CT_SdtContentRun) has no math case and `isRawPChild` omits them, so
`d.Skip()` drops them — content the XSD explicitly allows (Word writes it for an
equation inserted with track-changes on). Probe: `<w:ins><m:oMath>…E=mc2…` vanishes
from the regenerated document.xml. CONFIRMED-runtime. NEW (C173 fixed math
*prefixing*, not nested capture). *Fix:* add `oMath`/`oMathPara` to isRawPChild.

**C260 — docx-oxml — `w:dir`/`w:bdo` bidi wrappers dropped with all their runs.**
`docx/internal/oxml/paragraph.go:96-118`; dead types `run_types.go:18-29`. These
EG_PContent wrappers hit the default `d.Skip()` in CT_P.UnmarshalXML — the visible
text inside is deleted on any modification save. Probe: `<w:dir w:val="rtl"><w:r>
<w:t>shalom</w:t>` → "shalom" absent from output. `CT_DirContentRun`/
`CT_BdoContentRun` are declared but referenced nowhere. CONFIRMED-runtime. NEW.
*Fix:* add `dir`/`bdo` to isRawPChild (and wire or delete the dead types).

**C261 — docx-oxml — `w:fldSimple` models `fldLock` under the wrong name; locked fields become editable.**
`docx/internal/oxml/fields.go:101,114-128`. The XSD attributes are `instr`,
`fldLock`, `dirty`; the model matches `"lock"` (which no producer writes) and has
no CapturedAttrs, so `w:fldLock="true"` is stripped (a locked field becomes
editable) and `w:fldData` is dropped. Probe: both absent after open→edit→save; if
a caller set `Lock`, marshal would emit a nonexistent `w:lock`. CONFIRMED-runtime.
NEW. *Fix:* rename to `fldLock`, add `fldData` to isRawPChild, give CT_SimpleField
CapturedAttrs.

**C262 — pptx — `SetText`/`AddParagraph` force `algn="l"`, clobbering inherited alignment.**
`pptx/text.go:270-275`, `slide.go:1016-1027`. `NewParagraph` sets alignment to
`TextAlignLeft` and `paragraphToOxml` emits PPr whenever alignment≠"". So
`SetText` on a centered title placeholder in a loaded template writes
`<a:pPr algn="l"/>`, overriding the layout's centering. Probe: centered title +
SetText emits `algn="l"`. Defeats the codebase's own inherit-by-default doctrine
(runs/bullets/lineSpacing default to unset). CONFIRMED-runtime. NEW. *Fix:*
default NewParagraph alignment to the empty inherit value.

**C263 — pptx — `SetFill` on a loaded shape unions fills into schema-invalid XML.**
`pptx/slide.go:815-852`, `shape_sync.go:181-183`. The domain spPr is a sparse
overlay; `applyShapeStyle` copies each non-nil field onto the parsed node but never
clears the parsed node's competing fill kind. Probe: shape saved with `<a:noFill/>`,
reopened, `SetFill(solid)` → spPr has both `<a:noFill/>` and `<a:solidFill>`
(EG_FillProperties is a choice — invalid, renderer-dependent). Any parsed
gradFill/pattFill + SetFill hits it. CONFIRMED-runtime. NEW (C22 was the opposite
defect, fixed with this flawed mechanism). *Fix:* clear all six fill fields on the
destination before copying when the overlay carries any fill. The sibling
`SetGlow`/`SetShadow`/`SetReflection` effect-list replacement (C308, medium) is the
same class.

**C264 — docs — architecture.md overstates byte-identity for parsed-but-unedited parts.**
`docs/architecture.md:79,87-101`. The state diagram says "Parsed → Save writes
byte-identical output"; the prose says pptx/docx "regenerate … still byte-identical".
The code contradicts it: `presentation.go:1030-1033` passes never-materialized
slides through verbatim *because* "regeneration occasionally drifts from a
whitespace-preserving producer's exact bytes", and CHANGELOG quantifies it (pptx
1200/1200 via pass-through vs 1189 regenerated; docx 1196 vs 1194). Merely reading
a slide materializes it and can break byte-identity on ~1% of wild files.
CONFIRMED. NEW. *Fix:* state the three-tier truth the CHANGELOG already states.

**C265 — docs/dx — Encrypted-read path is docx-only but documented as format-generic.**
`README.md:13`, `docs/troubleshooting.md:34`, `encryption-and-signing.md:16`. Only
docx has an `OpenEncrypted` wrapper; xlsx/pptx have none and there is no public
bridge from the `*opc.Reader` that `opc.OpenEncrypted` returns to a
`Workbook`/`Presentation`. A user with an encrypted .xlsx following the
troubleshooting entry hits a documented-looking dead end. CONFIRMED (grep: wrapper
exists only in docx/ and opc/). NEW (D34 covered README absence, not the gap).
*Fix:* add xlsx/pptx wrappers, or state the Word-only support plainly.

**C266 — docx — Edits to reopened header/footer content via handles are silently dropped.**
`docx/document.go:817` gate + `image.go:316-375`, `hyperlink.go:77`, `chart.go:53`,
`comment.go:147`, `footnote.go:78`. `Images()`/`Hyperlinks()` hand out live handles
into parsed header/footer models (`hfPart` set), but no mutator on
Paragraph/Run/Hyperlink calls `markHdrFtrModified` (only ReplaceText, watermarks,
revisions do). Probe: reopened header image + `SetAltText("EDITED")` → saved
header1.xml lacks it (a body image persists); `Run.AddImage` additionally leaves an
orphan media part with an in-memory-only rel. CONFIRMED-runtime. NEW (C180 was the
session-new-header sibling). *Fix:* route header-scoped mutations through
`markHdrFtrModified` (the handle already carries `hfPart`).

**C267 — docx — Comment threading is blind to table/header paragraphs.**
`docx/comment.go:384-390,365-382,131-134`. `nestReplyAnchor`/`AnchorText` search
`Body.Paragraphs()` (top-level + SDT only, no table descent), unlike bookmarks
which use `AllParagraphs()`. Probe: `cellPara.AddComment(...)` then `c.Reply(...)`
in a table cell → comments.xml carries the reply but document.xml has zero markers
for its id (Word treats it as orphaned); `AnchorText()` returns "" for any
table-anchored comment. CONFIRMED-runtime. NEW. *Fix:* use `Body.AllParagraphs()`
(and header/footer paragraphs) in these three functions.

**C268 — pptx — Merge drops internal slide-jump hyperlink rels, leaving dangling r:id.**
`pptx/merge.go:159-205`. A slide-jump hyperlink is a `RelTypeSlide` rel; `importSlide`
has no case for it, so `importPart` (which searches only `otherParts`) returns "" and
the rel is dropped while the copied slide XML keeps `<a:hlinkClick r:id="rId2"
action="ppaction://hlinksldjump"/>`. Probe: imported slide references rId2 with no
matching rel. Validate only warns; Save succeeds → PowerPoint repair. CONFIRMED-runtime.
NEW. *Fix:* remap RelTypeSlide rels to the imported target, else strip the r:id.

**C269 — pptx — `CloneShape`/`CloneRow` share the `Hyperlink` pointer with the original.**
`pptx/clone.go:103-108,191-194`. `Run` and `BaseShape` shallow-copy their
`hyperlink *Hyperlink`, contradicting "The clone shares no state". Probe 1: clone a
TextBox with an external hyperlink onto another slide → `allocateHyperlinkRels`
fills relID once and skips the second slide, so slide2.xml emits `r:id="rId2"` with
no matching rel → repair. Probe 2: `clone.…Hyperlink().SetTooltip("X")` changes the
original's tooltip. CONFIRMED-runtime. NEW. *Fix:* deep-copy the Hyperlink (minus
relID/slide/markDirty) in Run.clone and cloneBaseShape.

**C270 — pptx — Autoplay-timing rebuild silently wipes animations added via `AddAnimation`.**
`pptx/media_timing.go:46-56`, `animation.go:417-463`. `applyAnimations` appends
effect nodes into the existing tree but never clears `timingAutoGenerated`; a later
event that regenerates the autoplay tree (a second autoplay medium, a full rebuild,
`SetPlayMode`) replaces the whole `p:timing` via `buildAutoplayTiming`, which emits
only mediacall nodes. Probe: autoplay video + `AddAnimation(FadeIn)` + second
autoplay video + save → the entrance effect is gone. CONFIRMED-runtime. NEW
(distinct from C163/C191). *Fix:* freeze the tree (clear `timingAutoGenerated`) once
foreign effect nodes are appended, or preserve non-mediacall groups on rebuild.

**C271 — dx — A transient DoH-resolver outage permanently retires live corpus references.**
`tools/ccrun/worker.go:173-176`, `internal/ccharvest/fetch.go:69-71`,
`tools/ccfetch/main.go:626-629`. `gate.Check` errors on resolver-endpoint failure;
ccrun wraps it as `ErrGateDead`, and `ClassifyFetchError` matches `ErrGateDead`
*before* the `retryable` hint → `DispPermanent, "fetch:dns"`. The worker emits a
terminal fail written to the committed batch-quarantine. A resolver blip mid-batch
mass-retires every live row it touches, indistinguishable from dead hosts; only
hand-editing the ledger recovers them. CONFIRMED. NEW. *Fix:* wrap gate-infra
errors in a distinct transient sentinel that `ClassifyFetchError` defers.

**C272 — xlsx-oxml — Dirty worksheet/stylesheet drops xmlns captured on `<extLst>` → malformed XML.**
`xlsx/marshal.go:475,243`. `CT_ExtensionList.CapturedAttrs` exists to preserve
`<extLst xmlns:x14="…">`, and the workbook path replays it, but
`marshalWorksheetExtLst`/the stylesheet path write `StartElement(nsSML, "extLst")`
with no attrs. Probe: sheet with `<extLst xmlns:x14="…"><ext><x14:sparklineGroups/>`
+ one cell written → output has undeclared `x14:` prefix; the part is not
namespace-well-formed. CONFIRMED-runtime. NEW. *Fix:* pass
`RawAttrList(ExtLst.CapturedAttrs)` in both marshalers.

**C273 — xlsx-oxml — `CT_CellFormula` drops data-table attributes; editing corrupts what-if tables.**
`xlsx/internal/oxml/worksheet.go:945-991`. Unmarshal/marshal handle only
`t/aca/ref/ca/si`; a `<f t="dataTable" ref="B2:B5" dt2D="1" r1="A1" r2="A2"…/>` on a
sheet loses `r1`/`r2`/`dt2D`/`dtr`/`del1`/`del2`/`bx` when an unrelated cell is
written, keeping `t="dataTable"` but losing its input-cell refs. CONFIRMED-runtime.
NEW (C176 covered shared formulas). *Fix:* model or raw-capture the remaining
CT_CellFormula attributes.

**C274 — xlsx-oxml — Filter/CF sub-models narrower than schema; date filters & x14 CF pairing dropped.**
`xlsx/internal/oxml/worksheet.go:1327-1410`. `CT_FilterColumn` models only
filters/customFilters (schema also has top10/dynamicFilter/colorFilter/iconFilter/
extLst); `CT_Filters` lacks dateGroupItem; `CT_CfRule` lacks extLst (which carries
the `x14:id` linking a rule to its x14 dataBar). Probe: all present, one cell
written → `<filters/>`, `<filterColumn colId="1"/>`, cfRule without extLst — date
filters and top-10 vanish, x14 pairing severed. CONFIRMED-runtime. NEW. *Fix:* add
typed/raw capture for the remaining choice members, dateGroupItem, and cfRule extLst.

**KNOWN(C194) — pptx — `SetAnchor`/`SetWordWrap`/`SetMargins` still write explicit `lIns=0…bIns=0`.**
`pptx/shape_sync.go:236-253`, `oxml_to_domain.go:624-661`. `oxmlToTextFrame` builds
zero-value margins when the parsed bodyPr has no insets; the flush writes all four
unconditionally, replacing inherited ~91440/45720 defaults with zeros and shifting
text. Probe reproduced. CONFIRMED-runtime, **still open** since 2026-07-11 (that
report's T6). *Fix:* track "margins explicitly set" separately (as autofitDirty
already is).


### Medium

| ID | Area | Issue & scenario | Location | Status | Novelty |
|----|------|------------------|----------|--------|---------|
| C275 | crypto | Attacker-controlled agile `spinCount` drives unbounded SHA-512 work before password check: a hostile encrypted file with `spinCount="2000000000"` forces ~2B iterations ×3 keys on `OpenEncrypted` — a per-request core-hang DoS. Only guard is `<0`. | common/crypto/agile.go:611-627 | CONFIRMED | NEW |
| C276 | opc | CFB stream reads pre-allocate an attacker-controlled size: `miniCutoff=0xFFFFFFFF` + a ~4 GiB declared entry forces a single ~4 GiB alloc before any chain walk, bypassing all `MaxDecompressed*` zip-bomb knobs on the `OpenEncrypted` path. | opc/cfb.go:213,309 | CONFIRMED | NEW |
| C277 | dml | Percent-typed attrs modeled as raw int32 hard-fail whole-part parse on `thresh="50%"`/`zoom="50%"` (transitional lexical form), contradicting the package's own graceful-degradation policy; on the live blip path. | common/dml/xml_effect.go:424-448; xml_3d.go:22 | CONFIRMED | NEW |
| C278 | xml | Reflection marshaler's chardata branch never `flushOpenTag()` under CollapseEmptyElements → malformed `<p:texthello/>` with text fused/lost. No current production chardata type triggers it; a loaded trap. | common/xml/marshal.go:600-606 | CONFIRMED | NEW |
| C279 | xml | Inline/literal element prefix bindings never scope-restored (C186 fixed only the declared-state half): a later element in an aliased namespace (`ve`) emits an unbound/wrong prefix. | common/xml/builder.go:610-612; rawattrs.go:273-280 | CONFIRMED (Builder); PLAUSIBLE (prod) | NEW |
| C280 | xml | `CaptureProlog` mis-captures RootEnd when trailing bytes after the root close contain `</` (e.g. a comment) → replay writes garbage instead of the real close tag → malformed part. | common/xml/prolog.go:80-91 | CONFIRMED | NEW |
| C281 | xml | Ordered-child capture silently deletes XML comments and PIs inside captured elements (`<!-- reviewer note -->` in w:body/w:p) on a zero-modification save — the exact class the capture kit exists to prevent. | common/xml/children.go:126-198 | CONFIRMED | NEW |
| C282 | xml | `ChildCapture.Raw` aliases the full source buffer (no clone), so one captured whitespace/unknown child pins the entire part's bytes for the document's lifetime — contradicts CaptureRawInner's own clone-for-this-reason policy; ~every opened document. | common/xml/children.go:172,191 | CONFIRMED | NEW |
| C283 | xlsx | `AddOLEObject` rejects a sheet with comments, but `AddComment` after `AddOLEObject` is accepted; both write the one legacy VML part and `<legacyDrawing>` points only at the OLE one, so note boxes never render (both also use shape id 1025). | xlsx/ole.go:156-282; comment_opened.go:94 | CONFIRMED-runtime | NEW |
| C284 | xlsx | Merely reading `Comments()` (or `Cell.Comment()`) sets `s.comments`, and `AddOLEObject` guards on `s.comments != nil` — a read-only inspection permanently blocks a writer API on a comment-free sheet. | xlsx/ole.go:156-158; comment.go:150 | CONFIRMED-runtime | NEW |
| C285 | xlsx | `translateFormula` corrupts scientific-notation literals when materializing shared-formula followers: `1.5E2` lexes `E2` as a cell ref → follower gets `1.5E3` (150 → 1500). | xlsx/formula.go:110-206 | CONFIRMED-runtime | NEW |
| C286 | xlsx | Conditional-format priority docs claim the opposite of Excel: the code assigns new rules *higher* numbers (which *lose* conflicts per §18.3.1.10) while the godoc says "later calls layer on top". Allocation is fine; both comments are inverted. | xlsx/conditional_format_write.go:294-346 | CONFIRMED | NEW |
| C287 | xlsx | `SetPrintArea`/`SetPrintTitles` accept garbage and already-qualified ranges: `SetPrintArea("Sheet1!A1:B2")` stores `Sheet1!$SHEET$1!A1:$B$2`; any non-ref stores `$GARBAGE`. Returns error but never validates. | xlsx/page_setup.go:233-441 | CONFIRMED | NEW |
| C288 | xlsx | `AddComment`/`AddNote` accept duplicate and invalid cell refs: two `<comment ref="A1">` on one cell; `AddComment("NOTACELL",…)` emits a ref-less note; refs not canonicalized while `Cell.Comment()` matches case-sensitively. | xlsx/comment.go:314-372 | CONFIRMED-runtime | NEW |
| C289 | docx | `Protect()` silently deletes an existing `w:writeProtection` when `ReadOnlyRecommended` is false (unconditional `RemoveChild`), contrary to its godoc — an unrelated edit-restriction call destroys independent write-protection (and its password hash). | docx/protection.go:152-159 | CONFIRMED-runtime | NEW |
| C290 | docx | `Validate` emits false-positive `rel-target-missing` warnings for parts added this session: `partExists` omits `d.imageParts`, so open→AddImage→Validate warns on the image it will write, contradicting the function's own "never a false positive" contract. | docx/validate.go:463-489 | CONFIRMED-runtime | NEW |
| C291 | docx | `Revisions()` godoc says "in document order" but appends all table structural revisions in a second pass after all paragraph revisions; a `tblPrChange` on table 1 sorts after the last paragraph's insertions. SDT-nested tables skipped entirely. | docx/revisions.go:319-349 | CONFIRMED | NEW |
| C292 | docx | Merge rewrites only the serialized body; copied style/numbering *definitions* keep old numId/pStyle cross-refs, so a copied style's `w:numId` resolves to the destination's unrelated list — lists change shape after merge. | docx/merge.go:149-256,462-486 | CONFIRMED | NEW |
| C293 | docx | `Run.SetBold/SetItalic/SetFontSize` set only the non-CS element (Style setters set the Cs twin), so complex-script text keeps old weight/size; also `SetBold(false)` writes explicit-off while `SetUnderline(false)` removes the element — two "false" semantics in one type. | docx/run.go:43-164 | CONFIRMED | NEW |
| C294 | docx | Editing `Properties` of a psmdcp-flavored package (System.IO.Packaging core props) writes an orphan `docProps/core.xml` whose rel is never written and keeps the stale psmdcp — edit invisible to Word, two core-property parts (OPC M4.1 violation). | docx/document.go:686-688 | CONFIRMED-runtime | NEW |
| C295 | docx | Images and charts each derive `wp:docPr id` from their own part number, so `AddImageFromBytes`+`AddChart` emit two `<wp:docPr id="1">` (ECMA requires document-unique ids); duplicate placement of identical bytes reuses the id too. | docx/image.go:363; chart.go:108 | CONFIRMED-runtime | NEW |
| C296 | docx | `AddBookmarkOnRange`/`AddCommentOnRange` on nested end runs leave a dangling `w:bookmarkStart` (no end) or an orphan comment (comments.xml entry, no markers); both ignore the insertion bools. | docx/bookmark.go:71-80; comment.go:166-171 | CONFIRMED | NEW |
| C297 | docx | `nextRelID` seeds only from the main part's rIds but allocates ids written into header/footer rel scopes; a wild header whose rels exceed the main max gets a duplicate `Id` in headerN.xml.rels. | docx/document.go:1484-1500 | PLAUSIBLE | NEW |
| C298 | docx | `Document.Sources()` reads only `/word/bibliography/sources.xml`; Word stores per-doc citation sources in a `customXml` item part, so typical Word docs return nil and `AddSource` creates a parallel store. | docx/bibliography.go:16,52-61 | PLAUSIBLE | NEW |
| C299 | docx | `Images()`/`Hyperlinks()` header/footer loops walk `hdr.P` directly (not `AllParagraphs()`), so images/links inside a header *table* are omitted though the godoc says "including … headers"; `Charts()`/`MergeFields()` skip headers entirely. | docx/image_read.go:109-133; chart.go:207-234 | CONFIRMED | NEW |
| C300 | docx | Session-authored building blocks and framesets are invisible to `BuildingBlocks()`/`Frameset()` in the same session (readers parse preserved raw bytes, ignore the pending set) while the sibling `CustomXMLParts()` does merge them — read-your-writes disagreement. | docx/buildingblocks.go:121-152; frameset.go:101-122 | CONFIRMED | NEW |
| C301 | pptx | Dangling `sldId` entries (missing rel/part) are skipped in `loadSlides` but still increment the `index` counter, misaligning `p.slides` positions vs `Index()` — `Delete()`/`Index()` then target the wrong slide on repairable wild files. | pptx/presentation.go:470-509 | CONFIRMED-runtime | NEW |
| C302 | pptx | A stale slide handle (after `RemoveSlide`) keeps its old `index`; a second `Delete()` passes the range check and removes whichever slide now occupies that index. `Duplicate` on a removed handle inserts at the wrong position. | pptx/slide.go:1537-1539; presentation.go:2133-2156 | CONFIRMED-runtime | NEW |
| C303 | pptx | `CreateFromTemplate` leaves dangling `[Content_Types].xml` overrides for the removed template slides (override cleanup is driven only by `removedParts`, which it never populates) → OPC-invalid content-types stream. | pptx/presentation.go:809-974 | CONFIRMED-runtime | NEW |
| C304 | pptx | `Slide.RelID` godoc says created slides report "" until first save; probe shows `AddSlide().RelID()=="rId2"` — and that id is provisional (saveNew reassigns, see C255), so callers following the doc get corrupted references. | pptx/slide.go:151-158 | CONFIRMED-runtime | NEW |
| C305 | pptx | `Presentation.ReplaceText`/`Slide.ReplaceText` skip tables nested inside unnamed groups (the group walker has no GraphicFrames loop) while `ReplaceTextInShape` and `Text()` reach them — a `{{key}}` in a grouped table cell is visible but unreplaceable, no error. | pptx/template.go:175-229 | CONFIRMED | NEW |
| C306 | pptx | `CreateFromTemplate` keeps the opened file's flavor, so `CreateFromTemplate("x.potx")` + `Save("y.pptx")` re-emits the template content type — PowerPoint opens "y.pptx" as a template. Directly conflicts with the API's "creates a new presentation" promise. | pptx/presentation.go:809-875 | CONFIRMED | NEW |
| C307 | pptx | `RemoveSlide`/`Delete` never strip the removed slide's id from `p14:sectionLst`; `marshalSectionLst` writes the stale id on save (Validate doesn't check sections) — a dangling `p14:sldId` not in `sldIdLst`. | pptx/presentation.go:2133-2160; section.go:78-97 | CONFIRMED (stale id written) | NEW |
| C308 | pptx | `SetGlow`/`SetShadow`/`SetReflection`/`SetSoftEdge` on a loaded shape replace the parsed `a:effectLst` wholesale (the overlay starts empty and `applyShapeStyle` does `dst.EffectLst = src.EffectLst`), dropping effects already in the file — same class as C263. | pptx/slide.go:840-841; shape_effects.go:9-14 | CONFIRMED | NEW |
| C309 | pptx | `SetOrientation` is a total no-op (`placeholderToOxml` never writes `Orient`); `SetIndex`/`SetPlaceholderSize` set no dirty flag and `updateShapeNode` never touches `Ph`, so they're dropped on materialized placeholders (work only on API-created shapes' first marshal). | pptx/placeholder.go:121-144; shape_sync.go:154-193 | CONFIRMED | NEW |
| C310 | pptx | `SetColSpan`/`SetRowSpan` emit `gridSpan` but no `hMerge`/`vMerge` continuation cells (no API for them), producing a row with more grid columns than the table has — invalid merged table, no validate check. | pptx/table.go:483-504 | CONFIRMED | NEW |
| C311 | pptx | `GroupShape.AddChild` accepts any `Shape` but `appendGroupChild` has no case for `*ChartFrame`/`*SmartArtFrame`/`*OLEObjectFrame`; the child stays in `Children()` yet is never serialized — the silent-drop that `Slide.AddShape`'s error return was added to prevent. | pptx/shape.go:310-313; shape_sync.go:638-689 | CONFIRMED | NEW (residue of C175) |
| C312 | pptx | Morph transition silently discards `Transition.Sound`: the morph branch returns before `soundActionToOxml`, so the AlternateContent carries no `p:sndAc`. docs/pptx.md:48 advertises transitions "with sound actions" without the morph exception. | pptx/transition.go:162-252 | CONFIRMED | NEW |
| C313 | pptx | Merge carries comment *parts* but not `commentAuthors.xml`/`authors.xml` or the presentation→authors rel, so imported comments lose their authors (`Comments()` → author=""); the modern comment's `sldMk sldId` also still names the source slide. | pptx/merge.go:164-174; comment.go:335 | CONFIRMED-runtime | NEW |
| C314 | pptx | `replacePictureImage` embeds the new image but never GCs the old rel/part (unlike the poster-swap path), so bulk template image-replacement — the API's stated use — accretes one dead image per replacement forever. `SetBackgroundImage` twice leaks the same way. | pptx/image_replace.go:207-289 | CONFIRMED-runtime | NEW |
| C315 | pptx | `initialsOf` slices the first *byte* of each name field: "Émile Zola" → `initials="�"` (U+FFFD) in authors.xml, displayed as the author badge for any non-ASCII first name. | pptx/comment_write.go:296-306 | CONFIRMED-runtime | NEW |
| C316 | pml | `SetNotes` deletes explicit `showMasterSp="0"`/`showMasterPhAnim="0"` on notes slides (plain bool + emit-only-if-true, default-true semantics), so master furniture reappears on notes pages — the C29 "latent" instance is now live via notes.go's marshal path. | pptx/internal/oxml/comments.go:63-64; pptx/notes.go:232-245 | CONFIRMED | KNOWN(C29), escalated |
| C317 | pml | `BuildOleChart.AnimBg` is plain bool+omitempty though `animBg` defaults true, so explicit `animBg="0"` is deleted on the always-remarshaled timing path (the sibling `a:bldChart` was fixed, this was skipped). | pptx/internal/oxml/animation.go:865-871 | CONFIRMED | NEW (C29 family) |
| C318 | xlsx-oxml | `IsPivotCachesElement` is prefix-blind (`HasPrefix "<pivotCaches"`), so a prefixed `<x:workbook>` with `<x:pivotCaches>` misses the existing caches and `AddPivotTable` emits a *second* `<pivotCaches>` (schema allows one) with a colliding cacheId. | xlsx/internal/oxml/pivot_cache.go:452-456 | CONFIRMED-runtime | NEW |
| C319 | xlsx-oxml | Workbook `AlternateContent` is a singleton field but the element repeats; two ACs collapse to the last while both ChildOrder entries remain, so a *zero-modification* save duplicates one AC and loses the other (workbook.xml is always regenerated). | xlsx/internal/oxml/workbook.go:47,176-181 | CONFIRMED-runtime | NEW |
| C320 | xlsx-oxml | `CT_BookView` skips its children and always emits empty, so a `<workbookView><extLst>…` loses the extLst on every save (unconditional, workbook.xml always regenerated). | xlsx/internal/oxml/workbook.go:664 | CONFIRMED-runtime | NEW |
| C321 | xlsx-oxml | `CT_SheetView` decodes only pane/selection, dropping `<pivotSelection>` (written in every pivot sheet's sheetView) and `extLst` on dirty sheets. | xlsx/internal/oxml/worksheet.go:511-533 | CONFIRMED-runtime | NEW |
| C322 | xlsx-oxml | `BoolLex.UnmarshalXMLAttr` hard-errors on out-of-schema booleans (`date1904="on"`), failing the whole Open — contradicting the package's lenient `parseOnOff` policy applied to styles booleans. Backs workbookPr/calcPr/bookView/definedName. | xlsx/internal/oxml/lexical.go:78-89 | CONFIRMED-runtime | NEW |
| C323 | xlsx-oxml | A present-but-empty `<extLst/>` in workbook.xml is dropped on a zero-modification save (`len(Ext)==0` early return) though ChildOrder recorded it — byte-identity break (contrast the explicit empty-`definedNames` handling). | xlsx/marshal.go:183-186 | CONFIRMED-runtime | NEW |
| C324 | xlsx-oxml | Worksheet/stylesheet root-attr capture resolves prefixes only from decls seen *so far*, so `<worksheet foo:tag="v" xmlns:foo="…">` (decl after use) loses the attribute's namespace on a dirty save; the workbook root (re-lexed) is immune. | xlsx/internal/oxml/worksheet.go:84-104; styles.go:49-69 | CONFIRMED-runtime | NEW |
| C325 | xlsx | `ApplyNamedStyle`/`Cell.SetNamedStyle` index `CellStyleXfs.Xf[xfID]` with no bounds/nil check, so a file whose `<cellStyle xfId="99">` exceeds cellStyleXfs panics — while `NamedStyles()` guards the same bound. | xlsx/style.go:372-405,1178-1189 | CONFIRMED-runtime | NEW |
| C326 | xlsx | `SparklineGroup` handles point into a value slice; `AddSparklineGroup` reallocs it, so an earlier handle's `SetSeriesColor` writes to the abandoned array and vanishes on save. The type doc calls the result a "read-only view" though it has setters. | xlsx/sparkline.go:264-292 | CONFIRMED-runtime | NEW |
| C327 | xlsx | `AddDefinedName` validates nothing: `AddDefinedName("", …)` saves `<definedName name="">`; duplicate name+scope pairs accepted — both forbidden by Excel's Name Manager (repair risk), while sheet/table/scenario names are all validated. | xlsx/workbook.go:1585-1615 | CONFIRMED (emission) | NEW |
| C328 | xlsx | `SaveTo`'s "sheets never accessed are never even parsed" contract is false: the mandatory `Validate` gate calls `ws()`/`loadComments()` for every sheet, fully materializing the lazily-parsed workbook — the memory claim holds only for `SaveToUnvalidated`. | xlsx/workbook.go:474-479; validate.go:59-282 | CONFIRMED | NEW |
| C329 | docx-oxml | Post-parse property additions to rPr/pPr append after all captured children, violating XSD sequence: parsed run + `SetBold` → `<w:rPr><w:sz/><w:b/>`; parsed pPr with sectPr + `SetAlignment` → `<w:jc>` after `<w:sectPr>` (schema-invalid, strict consumers reject). `InsertTypedField` exists but has one caller repo-wide. | common/xml/children.go:284-317; docx run.go:71, paragraph.go:61 | CONFIRMED-runtime | NEW |
| C330 | docx-oxml | Paragraphs inside a table wrapped in a block-level SDT are invisible to `Paragraphs()`/`AllParagraphs()` (`contentParagraphs` has no `bodyChildTbl` case), so ReplaceText/Revisions/`MaxRevisionID` all skip them — and MaxRevisionID can allocate a colliding revision id. | docx/internal/oxml/sdt.go:101-137; revisions.go:423-429 | CONFIRMED-runtime | NEW (residue of C228) |
| C331 | docx-oxml/xml | Clearing a modeled attribute is resurrected by CapturedAttrs replay: modeled lists are built `if x != ""`, so `SetTooltip("")`/`Slide.SetName("")` produce no modeled entry and the captured value replays. "Edits win" is value-only; deletion is inexpressible. Independently confirmed at marshal.go:460. | common/xml/marshal.go:440-463; docx hyperlink.go:77; pptx slide.go:254 | CONFIRMED | NEW |
| C332 | dx | ccrun ledgers no-doh `skip` outcomes, and `Ledger.Has` treats any outcome as processed forever, so running `make harvest-batch` once without `SPINE_DOH_URL` (its default) permanently retires all ~9.7k live references; no warning that the decision is irreversible. | tools/ccrun/main.go:238-246; ledger.go:75 | CONFIRMED | NEW |
| C333 | dx | HARVEST.md claims WARC rows are "verified by content_digest", but no code computes a digest of the fetched payload and compares — a wrong-offset range read that still gunzips to valid OOXML is silently accepted and mis-attributed. | testdata/cc/HARVEST.md:76-80 | CONFIRMED | NEW |
| C334 | dx | ccfetch journals transient live-fetch failures terminally as `rot` (timeout/429/5xx after one retry), unlike its own WARC phase and unlike ccrun's `ClassifyFetchError` — flaky-but-alive origins are burned on first contact. | tools/ccfetch/main.go:551-572 | CONFIRMED | NEW |
| C335 | dx | ccfetch `type-full` journal rows are terminal, so raising `-n` on an existing corpus fetches nothing new from capped types — `loadState` replays them as done; no output explains the 0-new-files run. | tools/ccfetch/main.go:595-599 | CONFIRMED | NEW |
| C336 | dx | ccrun discards worker stderr (`cmd.Stderr = io.Discard`), so every library panic collapses to the signature "worker exited with status 2" — distinct crash bugs across a 10k harvest cluster into one quarantine signature, defeating the signature column. | tools/ccrun/main.go:347 | CONFIRMED | NEW |
| C337 | dx | `SPINE_CC_UPDATE_QUARANTINE` regenerates known_failures.tsv from the local corpus only; rows (incl. hand-written `wontfix`) for files not present on this machine are silently dropped — the corpus is machine-dependent. | cctest/corpus_test.go:177-266 | CONFIRMED | NEW |
| C338 | dx | spectest's round-trip zeros `CapturedAttrs`/`OriginalNSDecls` on both sides before `DeepEqual`, so a marshaler that drops the replay entirely (losing unmodeled attributes — the one thing those fields carry) still passes the test. | spec/spectest/spectest.go:276-324 | CONFIRMED | NEW (overlaps C111) |
| C339 | dx | The symmetry guard excludes methods by name-match against embedded types, so a `Picture.AltText` overriding `BaseShape.AltText` (a natural Image move) escapes `TestNoUnclassifiedMethods`/`TestAllowListsAreAccurate` — the divergence the guard exists to catch. | internal/symmetry/symmetry_test.go:279-301 | CONFIRMED | NEW |

### Low

| ID | Area | Issue | Location | Status | Novelty |
|----|------|-------|----------|--------|---------|
| C340 | dml | EffectContainer/EffectDag model ~1 of ~30 children — `a:alphaMod`'s `alphaModFix` silently deleted on the live blip path (typed dispatch lossier than raw capture, the class C189 fixed for imgProps). | common/dml/xml_effect.go:136-455 | CONFIRMED | KNOWN(C146) |
| C341 | dml | `dml.Ext` has no `MarshalXML`, so the encoding/xml path strips all extension content (sibling types provide one; chart's Ext does too) — spectest-only, no production output. | common/dml/xml_extension.go:21-49 | CONFIRMED | NEW |
| C342 | dml | `GraphicData.RawContent`/`TblPr` inline `tableStyle`/percentage integer-form/sibling CapturedAttrs gaps: assorted byte-drift and spectest-only round-trip losses. | common/dml (xml_media.go:97, xml_table.go:19, percentage.go:66, xml_color_order.go:275) | CONFIRMED | KNOWN(C100)/NEW |
| C343 | crypto | `vml.Textbox`/`ClientData` presence-flag children collapse to one field / drop on marshal (`string,omitempty`) — spectest-only typed model, production VML is raw templates. | common/vml/vml.go:299-780 | CONFIRMED | KNOWN(C107/C108) |
| C344 | opc | `UnmarshalCoreProperties` swallows `DecodeElement` errors (`err == nil`-gated), leaving a present-but-empty element on regenerate with no diagnostic. | opc/package.go:669-720 | CONFIRMED | KNOWN(C120) |
| C345 | opc | `itoa` infinite-recurses on `math.MinInt` (`-n` overflows); reachable only via a hostile `<Words>-9223372036854775808</Words>` in app.xml. | opc/package.go:506-519 | CONFIRMED (logic) | NEW |
| C346 | opc | `doc.go` package overview still says WriteRawFile bypasses "content-type registration" — no longer true for `[Content_Types].xml` (C46 merges late registrations). | opc/doc.go:32-36 | CONFIRMED | NEW |
| C347 | xml | Innerxml/chardata/self-closing-space edge cases: reflection innerxml bypasses WriteRaw's trailingWS; zero-valued non-string chardata dropped; tag re-lex loses pre-`>` whitespace; `DetectSelfClosingSpace` doc misdescribes its scan. | common/xml/marshal.go:592; style.go:9; rawattrs.go:65 | CONFIRMED | NEW |
| C348 | xml | Unmodeled captured attr in an unmapped namespace (xr/xr2/x12ac gaps) loses its prefix on sourceless replay → attribute silently changes namespace. | common/xml/rawattrs.go:147-244 | CONFIRMED (narrow) | KNOWN(C147) |
| C349 | docx | Text box / WordArt / watermark builders don't escape/strip `\r` (C177-class re-introduced at four private escapers); duplicated escaper implementations invite the divergence. | docx/textbox.go:409; wordart.go:169; watermark.go:682 | CONFIRMED | NEW (kin of C177) |
| C350 | docx | `buildItemProps` concatenates the schema URI into `ds:uri="…"` unescaped — a namespace URI with `&` yields a malformed itemProps part. | docx/customxml.go:250-255 | CONFIRMED | NEW |
| C351 | docx | Assorted: `RemoveWatermark` dead second loop; `Text()` header ordering doc wrong (part name, not rel id); `parseTextBox` `:anchor` substring false-positive; `nextHdrFtrPartName` case-sensitive vs OPC; AddChart embed-name collision; `doc()` swallows re-parse error → nil panic; dead mainPartName case; building-block/frameset "read-only" godoc stale. | docx (watermark.go:209; text.go:18; textbox.go:504; document.go:1560,185,411; chart.go:78; buildingblocks.go:29) | CONFIRMED | NEW |
| C352 | docx-oxml | `attr.Name.Local=="r:id"` dead branches; CT_Compat/CT_WebSettings map-iteration nondeterministic order; CT_Document RootExtras slot; CT_Tbl backfill omits Raw; CT_Numbering "byte-for-byte" comment vs grouped emit; CT_Settings partial root-attr capture. | docx/internal/oxml (fields.go:45; settings_types.go:45; document.go:61; table.go:581; numbering.go:9; settings.go:27) | CONFIRMED | NEW/KNOWN(C124) |
| C353 | pptx | `Transition()` misreads absent `spd` as "med" (1.0s) vs ECMA "fast" (0.5s); `buildMorphAlternateContent` concatenates `MorphOption` unescaped; parsed cells report span 0 vs created cells' 1; SyncXML docs contradict the auto-flush; `AddSlideFromLayout`≡`AddSlideWithLayout` duplicate; `SlideSizeCustom` dead. | pptx (transition.go:317,435; oxml_to_domain.go:590; table.go:35; presentation.go:898; options.go:35) | CONFIRMED/PLAUSIBLE | NEW/KNOWN(C83) |
| C354 | pptx | `SetHyperlinkToSlide` out-of-range index emits a jump action with no target (no error channel); `AddAnimation(tb.ID())` before first save emits `spid="0"`; image dedup ignores content type; build-by-paragraph ignored inside groups; `AppendSlidesFrom` mutates the source (undocumented). | pptx (hyperlink.go:297; animation.go:136,510; image_replace.go:67; merge.go:116) | CONFIRMED | NEW |
| C355 | pml | Root-attr `ReplayCapturedAttrs`-style clears unsupported; remaining timing explicit-zero drops (Concurrent/By/From/To/ZoomContents); P14Media.Link programmatic drop; strict extension-bool aborts Open; SlideMasterID `id="0"`; dead XmlnsA/R/P + duplicate stdlib serializer + TimeNodeList missing Append helpers. | pptx/internal/oxml (animation.go:368,320; extension.go:82,297; presentation.go:30,220; slide.go:166) | CONFIRMED/PLAUSIBLE | NEW/KNOWN(C224/C90) |
| C356 | xlsx | `NewCellStyle`/`AddNumberFormat` mark dirty before the dedup hit (no-op calls force styles.xml regen); `ParseCellRef` rejects mixed-case ("Aa1"); `SetColWidth` carves only Cols[0]; pivot source scan uses the mutating `Cell()` getter (phantom cells); `DeleteSheet` leaves sheet-owned parts + name refs dangling; `Sheets()` returns the internal slice. | xlsx (style.go:132; workbook.go:1529,1204,1355; sheet.go:260; pivot_build.go:469) | CONFIRMED/PLAUSIBLE | NEW/KNOWN(C75/C127) |
| C357 | xlsx | `Cell.Value()` returns the cached formula result as a raw string regardless of type; multi-series scatter shares X (see C251); SVG images read back as the 1×1 PNG fallback; orphan threaded-comment replies dropped from `Comments()`; single-cell chart anchors can exceed the grid; AddChart failure leaves an orphan hidden data sheet; replaced hyperlink's old rel orphaned. | xlsx (cell.go:49; image_reader.go:76; comment.go:216; chart.go:59-103; hyperlink.go:196) | CONFIRMED | NEW/KNOWN(C132) |
| C358 | xlsx-oxml | `CT_Cell` drops `ph`; `CT_Rst` drops `rPh` (KNOWN C134); dead `r:id` branches; `CT_Row` numeric-parse-fail emits `r="0"`; dead fields (Ignorable, CT_Sst.OriginalNSDecls, DataConsolidate) + duplicate nsSML/nsSpreadsheetML const; CT_Extension misses default-xmlns; empty numFmts dropped. | xlsx/internal/oxml (worksheet.go:883,745; shared_strings.go:85; workbook.go:43,1193) | CONFIRMED/PLAUSIBLE | NEW/KNOWN(C134) |
| C359 | dx | DoH gate queries only A records (IPv6-only hosts verdicted dead); orchestrator ignores all ledger/quarantine write errors; python-tests committed yet `.gitignore` still lists it + testdata/README says "copy (gitignored)"; msoi_notes.json consumed by nothing; `signature()` truncates mid-rune (both harness copies); no CI. | tools/ccrun (main.go:243), internal/ccharvest/doh.go:54; .gitignore:41; spec/testdata; cctest/corpus_test.go:399 | CONFIRMED | NEW/KNOWN(C9/C210) |

---

## 4. Design tensions (structural — the approach, not the line)

**T1 — Regeneration-from-a-narrower-model is the dominant corruption engine.**
The round-trip design promises "unknown content survives", and for *unknown
elements* the raw-capture kit largely delivers. But wherever a typed model is
*narrower than the schema for a modeled element*, editing any sibling silently
deletes the unmodeled remainder: C241 (chartsheets as worksheets), C244 (txBody
br/fld), C259–C261 (docx nested math / dir/bdo / fldSimple attrs), C273–C274
(xlsx data-table / filter / CF), C308/C263 (effect & fill overlays), C340 (dml
effect containers). Every new schema element not on an allowlist is a latent
data-loss bug. *Alternative:* invert the default — capture-raw everything
unrecognized *within* a modeled element too (CT_R already does this for runs and
is provably lossless), rather than extending allowlists one finding at a time.

**T2 — The mutation-tracking dichotomy has no enforcement.** "Body/slides always
regenerated; every other part preserved-unless-flagged" makes correctness depend
on each mutator remembering to flag. Handles that reach into preserved parts
(headers/footers via `Images()`/`Hyperlinks()`, core-props psmdcp) mutate the
model but never flag it, so the preserved bytes win and the edit vanishes (C266,
C294). The handles already know their owning part (`hfPart`). *Alternative:* an
invariant that *any* mutation through a handle marks its owner dirty — a single
notifier on the handle, as `Revision` already has — instead of per-mutator
discipline.

**T3 — Verbatim capture and modeled mutation have no merge policy at the root or
for deletions.** Two independent surfaces of one gap: root-element attrs are
replayed all-or-nothing (`StartElementWithRootAttrs`), so any root setter is a
silent no-op on opened files (C247); and attribute *deletion* is inexpressible in
the capture convention, so clearing a field replays the captured original (C331).
Non-root elements solved the first half with `ReplayCapturedAttrs` (modeled-wins
merge). *Alternative:* extend that same merge to the root, and add a deletion
tombstone so "edits win" covers clears too.

**T4 — One id counter, many id spaces.** Relationship ids are allocated from a
single main-part-seeded counter but written into per-part rels scopes (headers,
masters, VBA), and drawing `docPr` ids are derived from unrelated part numbers.
The results are duplicate ids across scopes (C236 critical, C258, C295, C297).
*Alternative:* allocate per rels-scope (the actual OPC uniqueness domain) and give
docPr its own document-wide sequence (the codebase already has `shapeIDSeq` as the
pattern).

**T5 — Deferred-at-save work is a race that every new feature must join.**
`Duplicate` snapshots XML mid-pipeline; pending images (C256), custom-show ids
(C255), and autoplay/animation trees (C270) each get corrupted because they are
resolved at a different phase than the snapshot. `CreateFromTemplate` re-implements
slide removal as field resets and misses all of RemoveSlide's hygiene (C243, C303,
C306). *Alternative:* a single "flush all deferred work" hook shared by `marshal`,
`Duplicate`, and template clearing, so ordering is defined once.

**T6 — The inherit-vs-explicit tri-state keeps collapsing to zero.** Fixed for
lineSpacing/underline/strike/autofit, still broken for insets (C194), paragraph
alignment (C262), run bold/italic-off, and the C29 default-true boolean family
across pml (C316/C317). Each new property re-imports the bug. *Alternative:* a
shared "optional with explicit-set flag" primitive and a one-time XSD-driven sweep
of default-carrying attributes, rather than per-finding pointer-ification.

## 5. Expectation gaps (expected X, found Y)

- **Merge "no dangling references or duplicate parts"** (docs/docx.md:71,
  pptx.md:65): found dangling chart/OLE rels and aliasing (C252), unmerged
  footnotes (C253), duplicate master rel ids (C236), dropped slide-jump links
  (C268), lost comment authors (C313).
- **"AddChart works on both created and opened workbooks"** (docs/charts.md:146):
  found it destroys pre-existing charts/images on an opened sheet (C249); and
  docx "Edit Data opens the real cells" is false (C240 — no externalData).
- **"Read/write encrypted documents"** (README:13, format-generic): found the
  end-to-end read path exists for Word only (C265).
- **Byte-identity for parsed parts** (architecture.md:79): found re-marshaling can
  drift on ~1% of wild files; only the never-accessed path is guaranteed (C264).
- **`Validate` "never a false positive" / "checks are sound"**: found spurious
  rel-target warnings for session-added images (C290).
- **`Revisions()` "in document order"**: found table revisions batched after all
  paragraph revisions (C291); **`Slide.RelID` "empty until saved"**: found a
  provisional non-empty id (C304); **table `SyncXML` "required"**: found the save
  auto-flushes and calling SyncXML forces a *more* destructive path (C353).
- **"WARC rows verified by content_digest"** (HARVEST.md): found no verification
  exists (C333). **A read-only `Comments()`** disables a writer API (C284). **A
  handle returned as "a read-only view"** (sparkline) is a live setter into a
  slice that invalidates it (C326).

## 6. Open questions (not resolvable from code alone)

1. **Real-PowerPoint severity of the merge criticals.** Does PowerPoint
   first-wins on duplicate relationship ids (C236), or hard-repair? And does it
   load `p:embeddedFontLst` fonts when `embedTrueTypeFonts` is absent (C247) —
   which would upgrade that finding from "flag dropped" to "feature inert"?
2. **Word tolerance of out-of-schema-order pPr children** (C329): does Word drop a
   `w:jc` that follows `w:sectPr`, escalating byte-drift to visible formatting
   loss?
3. **Excel's reaction** to duplicate `<comment ref>` (C288), `name=""` defined
   names (C327), duplicate `tablePart r:id` (C257) — repair prompt, silent
   first-wins, or merge?
4. **Threat model for `OpenEncrypted`.** Are the CFB/spinCount resource attacks
   (C275, C276) in scope (untrusted uploads), or is the encrypted path assumed to
   handle only Office-produced files? The `MaxDecompressed*` knobs suggest the
   former is intended but they don't cover this path.
5. **Bibliography storage** (C298): does the ECMA sources-part location this
   library writes surface in Word's Manage Sources, or only customXml-stored ones?
   No Word-authored citations fixture exists to settle it.
6. **Is the harvest quarantine the durable failure catalog** (committed
   batch-quarantine.tsv is empty while audits cite 10k-run findings), or do those
   results live only in a local, uncommitted ledger (C332/C337)?

## 7. What held up under adversarial probing

Recorded so future audits don't re-litigate. The OPC decompression budget is
correctly mutex-guarded and charge-once (C181/C183 genuinely fixed); content-types
and writer-lifecycle (Abort vs Close, deferred CT merge) round-trip meticulously;
signature verify/sign uses constant-time compares and reconciles standalone-vs-
embedded C14N. The crypto key-derivation (agile SHA-512, standard SHA-1 X1‖X2, RC4
CryptoAPI) matches [MS-OFFCRYPTO] and is cross-validated against msoffcrypto-tool
/ RFC 6229; all verifier and package-HMAC compares use `hmac.Equal`; modern write
defaults (AES-256, fresh salts) are strong. The escaping stack is spec-correct
(C177/C103/C106/C213 fixed) and `Finish()` is now wired (C187). The
`UnmarshalWithSource` sync.Map registry cannot leak (paired deferred Delete).
`ReplayCapturedAttrs` handles source-order-wins, boolean lexical forms, and
duplicate-match correctly (its *deletion* gap is C331, not a value bug). docx
`replace.go` (rune-safe splitting, longest-key single pass), the revisions
accept/reject engine on body content, the pptx surgical shape-sync reindexing,
the C150 slide reindex, xlsx shared-formula lifecycle (C176 fixed), style dedup
(C232 fixed, now all-field), the color-transform order machinery (C95/C38/C178
closed), and omml (C42 rewrite) all survived direct attack. The full corpus test
suite, all examples, all doc snippets, `vet`, and `lint` (0 issues) are green.
Every finding above lives outside that green envelope.

---

## 9. Remediation status (PRs #188–#220)

All critical, high, and medium findings were fixed across 25 stacked/independent PRs
(#188–#212), followed by the low-severity "real but minor" tranche (#213–#216, addendum
1) and the pure edge/dead-code/spectest-only tranche (#217–#220, addendum 2) — 33 PRs
total. Every PR: fail-before/pass-after regression tests, full `go test ./...` +
`make lint` (0 issues) green; byte-identity-critical PRs verified against the Common
Crawl corpus. The fully-integrated 33-PR stack was verified end-to-end: build + vet + all
unit tests + lint (`0 issues`) + corpus byte-identity — zero textual conflicts, no byte
drift, one documented cross-chain errcheck fix (see addendum 2).

### Findings → PR

**Independent (branch off `main`):**
- **#188** `untrusted-input-hardening` — C238, C275, C276
- **#190** `xml-capture-robustness` — C239, C279, C280, C281, C282
- **#189** `pptx-master-merge` — **C236, C237** (criticals); base of both pptx chains
- **#193** `chart-fidelity` — C240, C248, C249, C250, C251
- **#203** `docs-truth` — C264, C265, C346
- **#205** `harness-integrity` — C271, C332, C336, C337, C338; C333 (doc-fixed)
- **#209** `opc-lows` — C344, C345
- **#210** `docx-escaping-safety` — C349, C350
- **#211** `ci-workflow` — C9, C233, C359 (CI + gitignore)

**xlsx-comment chain:** #191 `xlsx-workbook-integrity` (C241, C257, C258) → #196 `xlsx-comment-vml-ownership` (C245, C283, C284, C288)

**xlsx-oxml chain:** #199 `xlsx-oxml-capture` (C272, C273, C319, C320, C321, C322, C323, C324) → #202 `xlsx-hyperlink-formula` (C246, C285) → #206 `xlsx-filter-cf-capture` (C274)

**docx-public chain:** #192 `docx-merge-fidelity` (C252, C253, C292) → #198 `docx-open-path-fidelity` (C266, C267, C289, C290, C294, C295) → #207 `docx-readapi-robustness` (C296, C297, C299, C300)

**docx-oxml chain:** #195 `docx-oxml-content-model` (C242, C259, C260, C261, C330) → #201 `docx-markdeleted-membership` (C254)

**pptx chain A:** #189 → #194 `pptx-lifecycle-hygiene` (C243, C301, C302, C303, C306) → #197 `pptx-save-fidelity` (C247, C255, C256, C304) → #200 `pptx-shape-sync` (C194, C244, C262, C263, C308) → #212 `pptx-feature-mediums` (C307, C309, C310, C311, C312)

**pptx chain B:** #189 → #204 `pptx-merge-clone-media` (C268, C269, C270, C313) → #208 `pptx-scattered-mediums` (C314, C315, C316, C317)

### Suggested merge order
Independents in any order; each chain merges base→tip. #189 must precede both pptx
chains. All tips merge into `main` conflict-free (verified). Merge #211 (CI) early so it
gates the rest.

### Deferred (low-severity long-tail — not fixed; itemized for triage)
- **C329** — schema-order insertion for post-parse property additions (`w:jc` after
  `w:sectPr` etc.): complex per-type rank tables; Word tolerates the order in practice.
- **C298** — bibliography sources in `customXml` items: speculative fix, needs a
  Word-authored citations fixture to validate.
- **C333** — WARC `content_digest` verification (impl): needs live-corpus validation to
  avoid rejecting valid payloads; doc corrected in #205 meanwhile.
- **Genuinely real but minor** (worth a follow-up sweep): C351 (`nextHdrFtrPartName`
  case-collision; AddChart embed-name collision; `doc()` nil-panic), C353 (transition
  `spd` default read-modify-write; morph `MorphOption` unescaped), C356 (DeleteSheet
  leaves sheet-owned parts dangling — KNOWN C75; `Sheets()` returns internal slice),
  C357 (orphan threaded-comment replies dropped from `Comments()`), C358 (CT_Cell `ph`,
  CT_Rst `rPh` preservation).
- **Cosmetic / spectest-only / dead-code / latent** (low value): C340–C343 (dml/vml
  spectest-only marshal gaps), C347/C348 (latent xml edges), C352/C354/C355 (dead code,
  edge cases), residual C359 items (IPv6 DoH, ignored write errors, mid-rune truncation).

### Remediation addendum — low-severity real-but-minor tranche (PRs #213–#216)

After the core 25, the "real but minor" long-tail (Tier 1 + Tier 2, per maintainer
decision) was fixed in four more PRs. `xlsx-lows-base` is a scaffold branch (= main +
#196 + #193 + #206) that #215 targets; merge those three first.
- **#213** `pptx-lows` (on #212) — C353: transition `spd` default (0.5s), morph-option
  escaping, loaded-cell span 0→1, `SlideSizeCustom` wired to explicit W×H, dedupe/doc.
- **#214** `docx-lows` (on #207) — C351: case-safe `nextHdrFtrPartName`, collision-safe
  chart embed name, `doc()` error surfacing, decoder-based textbox parse, dead-code/doc.
- **#215** `xlsx-lows` (on `xlsx-lows-base`) — C356/C357: `Sheets()` copy, mixed-case
  `ParseCellRef`, styles-dirty-on-append, `SetColWidth` all `<cols>` groups, pivot
  read-only scan, orphan threaded replies, replaced-hyperlink rel cleanup, AddChart
  orphan-sheet + anchor clamp; **Tier 2:** `Cell.Value()` formula typing, `SVGData()`
  accessor, `DeleteSheet` cascade (safe subset — drawings/tables/VML/comments + owned
  media, shared-reference-guarded).
- **#216** `xlsx-oxml-lows` (on #215) — C358: `CT_Cell` `ph`, `CT_Rst` `rPh`, `CT_Row`
  parse guard, `CT_Extension` default-xmlns, empty `numFmts`, dead-code cleanup.

### Remediation addendum 2 — pure edge / dead-code / spectest-only tranche (PRs #217–#220)

The maintainer then asked for the remaining "pure edge / dead-code / spectest-only"
residuals to be closed out too. Four more PRs land them; each is individually green
(build + vet + package tests + lint + corpus byte-identity):
- **#217** `common-models-lows` (on main) — C340–C343: raw-capture EffectContainer/
  EffectDag children (`alphaModFix` no longer dropped), `dml.Ext` marshal, graphicData/
  tblPr/percentage completeness, VML round-trip gaps.
- **#218** `common-xml-lows` (on #190) — C347/C348: verbatim fallback when a start tag
  carries pre-`>` whitespace, innerxml routed through `WriteRaw`, zero-valued non-string
  chardata emitted (with `flushOpenTag`), `DetectSelfClosingSpace` doc corrected; plus
  captured-attr prefix resolution for corpus-confirmed extension namespaces (xr/xr2/xr3/
  xr6/xr9/xr10/xr16/x16r2/p188 — bare `x16` omitted as unconfirmable).
- **#219** `docx-oxml-lows` (on #201) — C352: corrected dead r:id lenience branches,
  deterministic CT_Compat/CT_WebSettings marshal, CT_Document inter-child extras slot,
  CT_Tbl backfill includes Raw, CT_Numbering doc, full CT_Settings root-attr capture.
- **#220** `pptx-pml-lows` (on #208) — C354/C355: out-of-range slide-jump dropped (not a
  dangling action), `AddAnimation` spid=0 guard + group-recursive build-by-paragraph,
  content-type-aware image dedup, `AppendSlidesFrom` mutation documented; explicit-zero
  timing attrs preserved via CapturedAttrs, `P14Media.Link` emitted, lenient extension
  booleans, `SlideMasterID` id=0 omitted, dead XmlnsA/R/P + duplicate stdlib serializers
  removed.

### Final full-stack integration (all 33 PRs, #188–#220)

Merging every chain tip into one tree (#188, #218, #213, #220, #196, #206, #216, #214,
#219, #193, #203, #205, #209, #210, #211, #217) integrates with **zero textual
conflicts** and passes build + vet + all unit tests + golangci-lint (`0 issues`) +
corpus byte-identity as one tree.

One **cross-chain semantic interaction** surfaced only at integration (not in any PR in
isolation): the two pptx chains both root at #189 but split — chain A (…→#212→#213)
changes `GroupShape.AddChild` to return `error` (group-child validation), while chain B
(…→#208→#220) adds a by-paragraph test that calls `AddChild` in statement form. Clean in
each PR alone; once combined, errcheck flags the unchecked error. Fixed in #220 with a
documented `//nolint:errcheck` on that one test line — the only source form that both
compiles on #220's no-error-return base *and* stays lint-clean once chain A merges, so
`main` is green regardless of the order the two pptx chains land in. (The codebase idiom
`_ = g.AddChild(x)` would be a compile error on #220's isolated base, so it is not usable
there.) Note: `TestFurnitureDeterministic` remains pre-existing-flaky (~1/300).

**Still deferred (genuinely optional):** C329 (schema-order insertion — Word-tolerated),
C298 (bibliography customXml — needs a Word fixture), C333 (WARC digest impl — needs
live corpus), and the `DeleteSheet` pivot-cache + defined-name-string portion (risky).
