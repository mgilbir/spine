# Spine Codebase Audit — 2026-07-07

Adversarial full-codebase audit of `github.com/mgilbir/spine` (Go library for reading/writing PPTX/DOCX/XLSX via OPC; ~57.5k LOC, 211 Go files, zero dependencies). Method: every package read in full by a dedicated reviewer; every doc example compiled and run; top findings verified by executing reproduction programs against the live library (marked **CONFIRMED-runtime**), the rest by code trace (**CONFIRMED**) or flagged **PLAUSIBLE**. Findings that did not survive an attempt to disprove them were discarded (e.g. chart `ErrBars` pointer-vs-slice "drift" is actually XSD-correct; `make test` on a truly fresh clone currently works end-to-end).

**Headline:** the library's read-and-preserve machinery is genuinely good — byte-identical round-trip of *untouched* files passes on a real-world corpus, and the childOrder/unknown-capture design in xlsx's workbook model is state of the art for this problem. The dominant failure mode is everything *adjacent* to that strength: **mutating an opened document**. In all three formats, some or most mutations are silently discarded or produce corrupt packages, because the sync between the three representations (domain shapes ↔ typed oxml models ↔ preserved raw bytes) is partial and, in places, mutually destructive. The second systemic failure is **regeneration lossiness**: parts that are always re-marshaled (pptx `presentation.xml`, docx `document.xml`, dirtied xlsx sheets) silently strip everything the typed model doesn't know about. Third: the test suite validates a serializer (`encoding/xml`) that production never uses (the `common/xml` Builder), and the mutate-after-open quadrant is almost entirely untested — which is exactly where the critical bugs live.

Severity counts: **5 critical, 38 high, 75 medium, 31 low** (149 findings, C1–C149).

> **Same-day update (HEAD `6d82c1c`):** a remediation wave of ~40 commits landed after this report was written, citing 74 of the 149 findings, plus new feature work (real media embedding, deterministic part ordering, the loaded-slide append/surgical-remove machinery). A full verification pass re-audited every claimed fix (runtime probes for criticals/highs) and adversarially audited the new code. Results in **§0** below: 62 findings verified fixed, 2 fixed-with-regression, 10 partial, 75 unaddressed — and **20 new findings (C150–C169: 3 critical, 6 high, 7 medium, 4 low)**, most of them defects introduced by the fixes themselves. Statuses in §1–§5 describe the morning baseline; §0 supersedes them for current state.

---

## 0. Remediation status at HEAD (6d82c1c) — verification pass

Method: every fix commit's diff read; every cited finding re-tested at HEAD — runtime probes (open/create → mutate → save → unzip → inspect XML, plus pre/post-fix comparisons against the parent commit) for all criticals and highs, code trace for the rest. New code (media embedding, clone-and-sync, deterministic ordering) audited as if never reviewed, including XSD validation of emitted timing trees. `go build ./...` and `go test ./...` are green at HEAD; `make lint` is still red (20 issues) and there is still no CI (C9).

### Verdicts for the 74 targeted findings

**FIXED — verified (62):**
C2, C5, C6, C7, C11, C13, C15, C16, C18, C19, C20, C21, C22, C35, C37, C39, C40, C41, C44, C45, C47, C49, C50, C52, C56, C58, C59, C62, C66, C67, C68, C69, C70, C71, C72, C73, C77, C79, C80, C81, C82, C86, C94, C96, C97, C98, C102, C104, C105, C115, C116, C121, C122, C135, C136, C137, C138, C140, C142, C145, C146, C149.
Highlights confirmed by probe: docx opened-document mutations now persist (C2); ReplaceText works on created decks, no longer wipes unsaved shapes, no longer cascades replacements (C5/C19/C20/C86); xlsx `*Cell` handles survive appends (C11, `[]*CT_Cell`); Close-then-Save no longer guts the file (C15); zip-bomb cap (C52); both formerly-red round-trip fixtures (`fred_data`, `abs_australia`) now pass byte-identical (C6/C7); `TimeNodeList` marshals from childOrder (C35, via the media-timing work).

**FIXED but the fix introduced new defects (2):**
- **C1** — the headline scenarios are fixed (append preserves `grpSp`/connectors with unique ids; single surgical remove preserves groups), but the new machinery has a critical wrong-shape-deletion bug (**C150**) and drops edits to parsed shapes (**C156**).
- **C14** — pre-existing unknown worksheet children are now preserved (probed with `oleObjects`), but the ChildOrder-gated marshal now silently drops mutations that introduce a child kind absent from the original sheet (**C157**).

**PARTIAL (10):**
- **C3** — image/header/footer parts are now written on opened docs (probed: valid rels, CT overrides), but part names still ignore existing parts: `AddImage`/`AddHeader` on a doc that already has `image1.png`/`header1.xml` now fails `Save` with "opc: duplicate part" (**C155**) — silent corruption traded for hard failure, not for working.
- **C4** — all 13 parsed presentation.xml attributes and the missing children (`modifyVerifier`, `photoAlbum`, `embeddedFontLst`…) now round-trip (probed), but `custShow` still lacks its XSD-required `sldLst` child and now emits schema-invalid `<p:custShow name id/>` where previously the element vanished (pptx/internal/oxml/presentation.go:99-102).
- **C17** — empty rel IDs gone, layout part + CT override written; but `AddLayout`'s relID scan reads only sibling layouts' relIDs (pptx/master.go:263-270), colliding with the master's theme rel on any real opened deck: the new `<p:sldLayoutId r:id>` resolves to theme1.xml and the layout is silently lost on reopen (probed: 7 parts written, 6 layouts survive).
- **C29** — deliberately scoped: dml `path@stroke/extrusionOk`, `blur@grow`, chart `sourceLinked`, pml `showWhenStopped`/`advClick`, layout `showMasterSp` are now `*bool` (Builder handles `*bool` correctly on both serializers — verified); `NotesSlide.ShowMasterSp/PhAnim`, presProps/viewProps flags, and vml presence flags remain plain `bool` — latent only (no production marshal path today).
- **C33** — `extLst` added to the Slide/Shape family and AlternateContent to Slide/Layout/Master; `Slide` still lacks `showMasterSp/showMasterPhAnim`, `CommonSlideData` still lacks `custDataLst`/`controls`.
- **C38** — both duotone colors now round-trip, but cross-kind order is lost: grouped slices marshal in field order (scrgb, srgb, hsl, sys, scheme, prst), and duotone's two colors are positional — `<a:duotone><a:prstClr/><a:srgbClr/></a:duotone>` re-emits inverted (common/dml/xml_effect.go:135-142).
- **C63** — read side fixed (`Paragraph.Text()` includes hyperlink content); `SetText` still replaces only `p.R`, leaving stale hyperlink/SDT text (docx/paragraph.go:23-27).
- **C99** — system colors degrade to concrete srgbClr, theme alpha carried, `WithAlpha` clamps; but `Alpha<=0` still means "unset", so `WithAlpha(0)` (transparent intent) still yields opaque (common/dml/fill.go:100-106).
- **C103** — 3 of the 4 raw-URI Builder sites escaped; `StartElementInlineNS` still writes the namespace URI raw (common/xml/builder.go:454) despite the commit claiming four. Latent (all current callers pass constants).
- **C124** — `pointsToTwipsSigned`/`nsR` removed; the unreachable `attr.Name.Local == "r:id"` branches (docx/internal/oxml/fields.go:39, section.go:202) and spec-test-only `CT_HeaderReference` remain.

**STILL OPEN — explicitly re-verified at HEAD (15):**
C8 (failing fixtures still commented out of external.txt; fresh clones green-by-omission — only the fetch.sh half, C115, was fixed), C9 (no `.github/`, lint red with the same 20 issues), C10 (Properties-after-Open still a silent no-op in docx/xlsx — probed), C12 (AddDefinedName/SetActiveSheet still dropped on opened workbooks — probed), C23, C24, C25, C26, C27, C28 (all six docx modeling/marshal losses re-probed and still reproduce), C30, C31 (`p:extLst`/`c:extLst` still typed `dml.ExtLst`), C32 (ShapeTree still `d.Skip()`s `mc:AlternateContent`/`p:contentPart` — re-confirmed by probe: a zero-modification save of a deck with spTree AlternateContent deletes it, directly contradicting PR #7's "preserve unmodeled slide content" goal), C34 (improved but `p:graphicEl` children now tagged in the *pml* namespace where the XSD wants DrawingML `a:chart`/`a:dgm` — real files still won't bind), C36 (no `InlineNSDecls` capture in pml/dml extensions).

**NOT ADDRESSED by any commit (60):** C42, C43, C46, C48, C51, C53, C54, C55, C57, C60, C61, C64, C65, C74, C75, C76, C78, C83, C84, C85, C87–C93, C95, C100, C101, C106–C114, C117–C120, C123, C125–C134, C139, C141 (decorative API removed and real embedding added — see new findings C151/C152/C163/C164 for the replacement's own bugs), C143, C144 (dead types removed — verified no dangling refs), C147, C148. Residual: C116's `ErrInvalidRange` is still declared and never returned.

### New findings from the verification pass (C150–C169)

| ID | Sev | Area | Issue | Location | Status |
|----|-----|------|-------|----------|--------|
| C150 | critical | pptx | `shapeRefs` never re-indexed after `RemoveChildren` compacts the parsed tree — remove A, save, remove B, save **deletes C and keeps B**; any two removals separated by a sync (Save/Duplicate/ReplaceText) corrupt the deck. Introduced by 8dc1009; pre-fix code handled this correctly | pptx/slide.go:97-101; pptx/internal/oxml/slide.go:182-236 | CONFIRMED-runtime |
| C151 | critical | pptx | Media relationships keyed by slide **part name**, but `saveNew` reassigns names by index — save, `MoveSlide` (or `RemoveSlide`), save: media rels attach to the wrong slide; the media slide's `p:pic` references r:ids absent from its .rels — corrupt deck | pptx/media_embed.go:50,57; pptx/presentation.go:1173-1176 | CONFIRMED-runtime |
| C152 | critical | pptx | `ReplaceText`/`Duplicate` on a created deck trigger shape sync before the slide has a part name — media rels stored under key `""` and never written; emitted `p14:media r:embed="rId1"` coincidentally resolves to the slideLayout rel — repair-prompt file | pptx/media_embed.go:50; template.go:61-63; slide.go:882-883 | CONFIRMED-runtime |
| C153 | high | docx | `AppendR` flips a paragraph to childOrder-only marshal without backfilling untracked runs — `p.SetText("A"); p.AddRun().SetText("B")` on a new doc saves only "B". Regression from e055228 (the C2 fix) | docx/internal/oxml/paragraph.go:670-673; docx/paragraph.go:23-27 | CONFIRMED-runtime |
| C154 | high | docx | `AddTable` seeds cells with an untracked literal paragraph; the first `cell.AddParagraph()` makes the cell childOrder-only and drops the seed paragraph including user text set on `cell.Paragraphs()[0]`. Regression from e055228 | docx/document.go:606-608; docx/internal/oxml/table.go:497 | CONFIRMED-runtime |
| C155 | high | docx | Added-part names ignore existing package parts (`imageCount`/`hdrFtrCount` start at 0): `AddImage`/`AddHeader` on an opened doc that already has `media/image1.png`/`header1.xml` → `Save` errors "opc: duplicate part"; no output producible | docx/image.go:141-144; headerfooter.go:80-83 | CONFIRMED-runtime |
| C156 | high | pptx | Append-only sync never re-syncs `shapes[:syncedShapes]` — mutating a parsed shape (`TextFrame().SetText`) and adding any shape saves the addition but silently drops the edit; before 64fc618 the edit persisted whenever a sync ran | pptx/slide.go:277-282 | CONFIRMED-runtime |
| C157 | high | xlsx | Worksheet marshal now ChildOrder-gated: mutators that introduce a child kind absent from the original sheet (`MergeCells`, `SetColWidth`, `AddDataValidation`, `FreezePanes` on sheets lacking those elements) are silently dropped. Regression from add7f9b (the C14 fix); pre-fix schema-order marshal emitted them | xlsx/marshal.go:349-362 | CONFIRMED-runtime |
| C158 | high | pptx | Removing a shape appended in a previous save falls into `forceShapeRebuild` (appended shapes never extend `shapeRefs`), and the rebuild drops group shapes and renumbers ids — contradicting `RemoveShape`'s own doc ("group shapes … preserved") | pptx/slide.go:102-105,353-354 | CONFIRMED-runtime |
| C159 | medium | docx | `Paragraph.Clear` doesn't reset childOrder — Clear+AddRun on a parsed paragraph serializes the new run **twice** (stale `{R,0}` ref plus the appended entry both resolve to it). Regression from e055228 | docx/paragraph.go:94-96 | CONFIRMED-runtime |
| C160 | medium | docx | `headerReference`/`footerReference` `type` attribute emitted **unprefixed** (`type=` not `w:type=`) in every regenerated document.xml — first-page/even header typing spec-invalid (pre-existing; surfaced by the C3 fix now that parts are actually written) | docx/internal/oxml/section.go:211 | CONFIRMED-runtime |
| C161 | medium | docx | `AddHeader`/`AddFooter` overwrite `d.headers[partName]` on name collision, clobbering the parsed existing header in memory (currently masked by C155's save failure) | docx/headerfooter.go:107 | CONFIRMED |
| C162 | medium | pptx | `NewParagraph` defaults `lineSpacing=100000` and 41589c8's `needSpacing := p.lineSpacing != 0` is therefore always true — every API-authored paragraph emits explicit `<a:lnSpc><a:spcPct val="100000"/>`, overriding placeholder/master-inherited spacing | pptx/slide.go:561,584-586; text.go:138 | CONFIRMED-runtime |
| C163 | medium | pptx | Autoplay timing tree never rebuilt (skipped when `Timing != nil`) while a full rebuild renumbers shape ids — timing left targeting a nonexistent `spid` (autoplay silently dead); media added after a first save on created decks never autoplays | pptx/media_timing.go:22; slide.go:292 | CONFIRMED-runtime |
| C164 | medium | pptx | `AddVideo`/`AddAudio` accept nil/empty data and empty/unknown content types with no error and no sniffing — empty content type yields `/ppt/media/media3.bin` absent from `[Content_Types].xml`: OPC-invalid package, repair prompt | pptx/slide.go:147-160; media_embed.go:163-193 | CONFIRMED-runtime |
| C165 | medium | pptx | `ReplaceText` re-materializes shapes and sets `syncedShapes>0`, flipping a **created** deck into append-only mode — subsequent `SetText` edits that a full rebuild used to flush are silently dropped, and caller-held shape pointers are detached | pptx/template.go:102-104; slide.go:277 | CONFIRMED-runtime |
| C166 | low | pptx | `AddPicture` embeds the image but defaults the frame to 0×0 (`<a:ext cx="0" cy="0"/>`) — invisible picture unless the caller knows to `SetSize`; no native-size default | pptx/slide.go:127-134 | CONFIRMED-runtime |
| C167 | low | pptx | `NewRun` still defaults underline/strike to explicit `u="none" strike="noStrike"` on every run — same inheritance-clobbering class a260254 fixed for font size (C136) | pptx/text.go:249-252 | CONFIRMED-runtime |
| C168 | low | pptx | Media dedup matches any `/ppt/media/` part by bytes only, ignoring declared content type — identical bytes under a different content type silently reuse the mismatched part | pptx/media_embed.go:37-42 | PLAUSIBLE |
| C169 | low | opc | Decompression cap is per-part only (many honestly-declared ~900 MiB parts still exhaust memory, since every part is eagerly read) and `MaxDecompressedPartSize` is a mutable unsynchronized package global | opc/reader.go:11-51 | PLAUSIBLE |

### What held up under adversarial probing

The core media embedding output is OOXML-correct (videoFile/audioFile/p14:media triple-rel pattern with the DAA4B4D4 URI, poster blip, `ppaction://media`; the emitted timing tree validates against ECMA-376 pml.xsd); media and poster bytes dedup across slides; the append path on loaded slides keeps rels stable across repeated saves. The deterministic-ordering commit is real: three process runs produce byte-identical parts and entry order. Removing the decorative media API and the fabricated p14/dml types left no dangling references. Both formerly-red round-trip fixtures now pass byte-identical with the real files present.

### Synthesis

The remediation wave was largely genuine — 62 of 74 targeted findings verified fixed, several with the exact mechanisms this report recommended (surgical shape sync, `nextAvailableSlidePartName` on the round-trip path, `[]*CT_Cell`, `*bool` for default-true attributes, W3CDTF layouts, decompression caps). But the day's work also **re-proved this report's design tensions rather than resolving them**: T1 (three representations, no owner) produced C150/C156/C165 — the new append/surgical-sync machinery is a third sync protocol whose bookkeeping (`shapeRefs`, `syncedShapes`) desynchronizes from the tree it mirrors after exactly one save cycle; T2/T3 (regeneration + Create-vs-Open split) produced C153/C154/C157 — the childOrder-gating disease was *spread* to new call sites by fixes that wired mutators into childOrder without backfilling untracked siblings; and the new media feature hit T1 again by keying relationships on part names that another subsystem reassigns (C151/C152). Every one of the 20 new findings is invisible to the repo's own test suite (green at HEAD), because the new tests exercise single-sync, single-save scenarios only — the same test-quadrant gap flagged in C111/§3.8. The highest-leverage next steps are unchanged: a single owning representation per part with sync bookkeeping that survives multiple save cycles, childOrder append helpers used by *every* mutator, and multi-cycle (mutate→save→mutate→save) corpus tests as a required pattern.

---

## 1. Summary table

Severity-ordered. IDs are stable; a fixing agent can cite them. Area key: `opc`, `pptx` (public), `pml` (pptx/internal/oxml), `docx` (public+oxml), `xlsx` (public+oxml), `dml` (common/dml core), `common` (common/xml, oxml, vml, omml, chart, diagram, docprops, enum), `dx` (docs/build/CI).

### Critical

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C1 | pptx | Any shape mutation on an opened slide rebuilds the shape tree from the lossy domain model: group shapes, charts/SmartArt, rotations, fills, SVG blips silently deleted; shape IDs renumbered | pptx/slide.go:183-228 | CONFIRMED-runtime |
| C2 | docx | Core mutators (`AddParagraph`, `AddRun`, `TableCell.AddParagraph`) on an **opened** document are silently dropped at save (childOrder-gated marshal never sees appended items) | docx/internal/oxml/document.go:224-256, paragraph.go:297-374 | CONFIRMED-runtime |
| C3 | docx | `AddImage`/`AddHeader`/`AddFooter` on an opened document write relationships/references but never the parts → dangling r:id, corrupt package | docx/document.go:309-460, image.go:134-177, headerfooter.go:71-148 | CONFIRMED-runtime |
| C4 | pptx | `presentation.xml` is regenerated on **every** save by a hand-written marshal that emits ~3 of 13 parsed attributes and drops `modifyVerifier` (password-to-modify), `custShowLst`, `embeddedFontLst`, `photoAlbum`, `smartTags`, `kinsoku`, `custDataLst` | pptx/marshal.go:40-145; struct fields at pptx/internal/oxml/presentation.go:38-45 | CONFIRMED-runtime |
| C5 | pptx | `ReplaceText` re-materializes domain shapes from XML, silently wiping unsaved API-added shapes on the same slide | pptx/template.go:89-93 | CONFIRMED-runtime |

### High

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C6 | opc | Content-type `Default Extension` case is folded to lowercase on parse and re-emit (`JPG`→`jpg`) — breaks the byte-identical round-trip contract; test `TestRoundTripByteIdentical/abs_australia` is red | opc/content_types.go:102,113,268 | CONFIRMED-runtime |
| C7 | opc | `[Content_Types].xml` writer hardcodes CRLF after the XML declaration; files written with LF (fred_data) can't round-trip; test red | opc/content_types.go:154-155 | CONFIRMED-runtime |
| C8 | dx | The two failing round-trip fixtures are commented out in `external.txt` ("URL unknown") — a fresh clone silently *skips* the failing tests (green) while machines with the files are red; regressions invisible | testdata/external.txt:19-20; xlsx/roundtrip_test.go:291-292 | CONFIRMED-runtime |
| C9 | dx | No CI of any kind (no `.github/`); `make lint` fails with 20 pre-existing issues; nothing enforces build/test/lint on merge | repo root; .golangci.yml | CONFIRMED |
| C10 | opc/docx/xlsx | Editing `Properties` (core doc properties) after `Open` is a silent no-op in docx and xlsx: preserved raw core.xml is written first, then the writer's skip-if-written check discards the edit | opc/writer.go:211-215; docx/document.go:310-333; xlsx/workbook.go:334-341 | CONFIRMED-runtime |
| C11 | xlsx | `*Cell` handles are invalidated by any later `Cell()` call on the same row (slice realloc) — writes silently go to a detached copy | xlsx/sheet.go:66-87 | CONFIRMED-runtime |
| C12 | xlsx | `AddDefinedName`/`SetActiveSheet` on an opened workbook are silently dropped when the original workbook.xml lacked that child (ChildOrder-gated marshal) | xlsx/marshal.go:59-85; xlsx/workbook.go:845-855,917-932 | CONFIRMED-runtime |
| C13 | xlsx | `DeleteSheet`+`AddSheet` produces duplicate `sheetId` (allocated as `len(sheets)`, never synced) — schema-invalid workbook | xlsx/workbook.go:789-793 | CONFIRMED-runtime |
| C14 | xlsx | `CT_Worksheet` skips unknown children with no raw capture — editing any cell of a sheet containing `oleObjects`, `controls`, `customSheetViews`, `scenarios`, `protectedRanges`… silently deletes them | xlsx/internal/oxml/worksheet.go:204-208 | CONFIRMED-runtime |
| C15 | xlsx | `Close()` then `Save()` silently switches to the from-scratch writer, discarding every preserved part (themes, sharedStrings, media, calcChain…) with no error | xlsx/workbook.go:287-321 | CONFIRMED-runtime |
| C16 | pptx | Slide part-name collision after `RemoveSlide`+`AddSlide`: new name assigned without checking existing parts → whole `Save` fails with `opc: duplicate part` | pptx/presentation.go:656-671 (fix exists unused at 1417) | CONFIRMED-runtime |
| C17 | pptx | `SlideMaster.AddLayout` never assigns `relID` → output contains `<p:sldLayoutId>` without r:id and `<Relationship Id=""…>` — corrupt package | pptx/master.go:253-259; layout.go:172 | CONFIRMED-runtime |
| C18 | pptx | `AddSlide` on an opened deck produces a slide with **no** `.rels` (no slideLayout relationship) — the README's own "open and modify" flow yields a repair-prompt file | pptx/presentation.go:879-905,1400-1415 | CONFIRMED-runtime |
| C19 | pptx | `ReplaceText` (all 3 variants) silently no-ops on `Create()`-built presentations — it walks `slideXML`, only populated by `Open` | pptx/template.go:52 | CONFIRMED-runtime |
| C20 | pptx | `redistributeText` mishandles overlapping common prefix/suffix: replacement that shortens repeated text across runs is silently not applied (`"a"+"a"` runs, replace `aa`→`a` → still `aa`) | pptx/template.go:350-365 (ineffassign at 361 marks it) | CONFIRMED-runtime |
| C21 | pptx | AutoShape gradient/pattern/no-fill are dropped at shape sync — only `SolidFill` is copied, though `Fill.ApplyToSpPr` sets all four; README promises gradient fills | pptx/slide.go:339-350 | CONFIRMED-runtime |
| C22 | pptx | `TextBox.SetFill/SetLine/SetShadow` are complete no-ops — `textBoxToOxml` never reads `spPr` | pptx/slide.go:231-267 | CONFIRMED-runtime |
| C23 | docx | `Run.AddTab` is never serialized in any document; `AddBreak`/`AddTab` on parsed runs also dropped (childOrder + incomplete fallback marshal) | docx/run.go:156-163; docx/internal/oxml/run.go:279-343 | CONFIRMED-runtime |
| C24 | docx | Mixing `SetText` and `AddImage` in one run silently drops the text (AppendDrawingChild flips marshal to childOrder-only mode; SetText never records childOrder) | docx/image.go:174-175; run.go:26-28 | CONFIRMED-runtime |
| C25 | docx | With 2+ images in one run, `InlineImage.SetSize/SetAltText` overwrites the **first** drawing (first-match on RawContent≠nil) — one image duplicated, one orphaned | docx/image.go:37-48 | CONFIRMED-runtime |
| C26 | docx | Lists added to an opened document never persist numbering.xml (only `saveNew` writes it) → `w:numPr` references nothing; lists render as plain paragraphs | docx/list.go:18-119; document.go:410-420 | CONFIRMED-runtime |
| C27 | docx | `sectPr` children `lnNumType`, `vAlign`, `formProt`, `paperSrc`, `noEndnote`, `textDirection`, `bidi`, `rtlGutter` are parsed-then-skipped and never marshaled — stripped from **every** opened+saved doc (document.xml always regenerated) | docx/internal/oxml/section.go:39-187 | CONFIRMED-runtime |
| C28 | docx | Unknown WML body/paragraph/row children are silently dropped with no raw capture: `w:altChunk`, `w:customXml`, `w:smartTag`, `w:moveFrom/To`, row `w:tblPrEx` | docx/internal/oxml/document.go:210-214; table.go:296-299; paragraph.go:256-259 | CONFIRMED-runtime |
| C29 | systemic | Default-true XSD booleans modeled as `bool` + omitempty: explicit `="0"` is deleted on re-marshal and readers apply default true — semantics inverted. Instances: pml `Transition.AdvClick`/`AdvTm` (public API can't express AdvanceOnClick=false), `ShowWhenStopped` (media), `showMasterSp` family, presProps/viewProps flags; dml `path@stroke/extrusionOk`, `blur@grow`; chart `sourceLinked`; vml presence flags | pptx/internal/oxml/transition.go:10-11, animation.go:596; common/dml/xml_geometry.go:73-74, xml_effect.go:77; common/dml/chart/types.go:246; pptx/transition.go:46 | CONFIRMED-runtime (dml probes) / CONFIRMED (rest) |
| C30 | pml | PML `p:extLst` modeled as `dml.ExtLst`, whose children match `a:ext` — every `p:ext` extension inside timing, transitions, comments, presProps of a re-marshaled part parses empty and is silently deleted | pptx/internal/oxml/animation.go:18, transition.go:36, comments.go, presprops.go; common/dml/xml_extension.go:13 | CONFIRMED |
| C31 | common | Same wrong-namespace bug in charts: `c:extLst` children are `c:ext`, but `dml.ExtLst` matches `a:ext` — all chart extensions (present in virtually every Excel chart) parse empty | common/dml/chart/chart.go:26 et al. | CONFIRMED-runtime |
| C32 | pml | `ShapeTree`/`GroupShape` unmarshal `d.Skip()`s `mc:AlternateContent` and `p:contentPart` — those shapes (ink, 2010+ features) are deleted from any modified slide | pptx/internal/oxml/slide.go:180-243, picture.go:61-124 | CONFIRMED |
| C33 | pml | Slide-family types under-modeled: `Slide` lacks `showMasterSp/showMasterPhAnim`; `CommonSlideData` lacks `custDataLst`/`controls` (ActiveX deleted); `Shape`/`Picture`/`ConnectionShape`/`GroupShape` lack `extLst`, `Shape` lacks `useBgFill`; `GrpSpPr` loses group fills/effects | pptx/internal/oxml/slide.go:16-127, shape.go:10-16, picture.go:11-53,191-194 | CONFIRMED |
| C34 | pml | Animation subtree mismodeled vs XSD: `p:graphicEl` (should contain `a:dgm`/`a:chart`), `p:bldSub` (should contain `a:bldDgm`/`a:bldChart`), `p:progress` (should be `CT_TLAnimVariant`) — chart/diagram animation content dropped | pptx/internal/oxml/animation.go:425-429, 718-722, 515-527 | CONFIRMED |
| C35 | pml | Programmatically built `TimeNodeList` marshals as an **empty** element (marshal driven solely by childOrder; no fallback, unlike ShapeTree) | pptx/internal/oxml/animation.go:193-262 | CONFIRMED |
| C36 | pml/dml | Unknown-URI `<ext>` round-trip drops xmlns declarations carried on the ext element → re-emitted RawContent has unbound prefixes = malformed XML (xlsx solved this with `InlineNSDecls`; pml and dml didn't) | pptx/internal/oxml/extension.go:110-216; common/dml/xml_extension.go | CONFIRMED |
| C37 | dml | `a:rtl` modeled as element chardata instead of `val` attribute — parsed value inverted AND re-marshaled as schema-invalid `<a:rtl>0</a:rtl>` | common/dml/xml_text.go:358 | CONFIRMED-runtime |
| C38 | dml | `Duotone` loses both colors (children decoded *as* ColorChoice containers) → emits `<a:duotone/>`, schema-invalid (requires exactly 2 colors); live path via pptx pictures | common/dml/xml_effect.go:130-132 | CONFIRMED-runtime |
| C39 | dml | Gradient stop `Gs` supports only srgbClr/schemeClr — sysClr/prstClr/hslClr/scrgbClr dropped leaving `<a:gs>` with **no** color child (schema-invalid) | common/dml/xml_types.go:379-384 | CONFIRMED-runtime |
| C40 | dml | `colorToSrgbClr` maps every non-RGB color to literal black `000000` — theme colors in gradients, pattern fills, and shadows silently become black | common/dml/fill.go:122-124 | CONFIRMED-runtime |
| C41 | common | `mc:AlternateContent` models exactly one Choice — multiple `<mc:Choice>` collapse to the last (first's content+Requires discarded); empty `<mc:Fallback/>` dropped (MCE semantics change); multi-prefix `Requires="p14 p15"` produces unbound prefixes. Production path: docx paragraphs, pptx slides | common/oxml/alternate_content.go:13-108 | CONFIRMED-runtime |
| C42 | common | `omml.OMath`/`Element` are completely broken: `xml:",any"` decodes children into the wrong level so all fields stay nil; marshal emits `<Elements></Elements>` junk — the package's core type has never worked; 594-line test file never tests it | common/omml/math.go:6-8, 373-377 | CONFIRMED-runtime |
| C43 | docx | New-document body loses paragraph/table interleaving (fallback marshal writes all `P` then all `Tbl`): "text after table" is emitted **before** the table | docx/internal/oxml/document.go:249-256 | CONFIRMED-runtime |

### Medium

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C44 | opc | `WritePartRelationships` bypasses every Writer invariant: no closed check, no dedupe, not registered in `parts` — double write yields duplicate zip entries; post-Close use gives opaque errors | opc/writer.go:287-312 | CONFIRMED |
| C45 | opc | `CreatePart` with empty/mismatched content type can register an empty-string `<Override ContentType=""/>` — invalid package, no error (dead `ErrInvalidContentType` exists) | opc/writer.go:90-92 | CONFIRMED |
| C46 | opc | After `WriteRawFile("[Content_Types].xml", …)`, content-type regeneration is permanently disabled — a part created afterwards needing a new Default/Override yields a corrupt package silently (docx writes raw CT first; one feature away) | opc/writer.go:111-139,155-158; docx/document.go:321 | CONFIRMED |
| C47 | opc | dcterms dates that aren't full RFC3339 (`2024-01-15`, legal W3CDTF) are silently dropped; element vanishes on re-marshal | opc/package.go:397-417,133-150 | CONFIRMED |
| C48 | opc | Unknown/foreign coreProperties children recorded in elementOrder but dropped by the closed marshal switch — data loss when core.xml is regenerated | opc/package.go:342-361 | CONFIRMED |
| C49 | opc | Duplicate `Default`/`Override` entries in a parsed [Content_Types].xml are re-emitted verbatim (order lists never dedupe) — output violates OPC uniqueness | opc/content_types.go:200-219,267-271 | CONFIRMED |
| C50 | opc | Override lookup is case-sensitive while OPC part names are case-insensitive — mismatched-case parts fall back to the wrong default type and round-trip with it | opc/content_types.go:94-97 | CONFIRMED / impact PLAUSIBLE |
| C51 | opc | `Reader` stores raw names but `GetFile` normalizes only the query — parts with odd-but-real zip names (`./ppt/…`, `a//b`) are unreachable and silently dropped on round-trip | opc/reader.go:130,144 vs 212 | CONFIRMED |
| C52 | opc | No decompressed-size limits anywhere (`io.ReadAll` on every part, CT parsed before user code) — hostile zip bomb OOMs the process | opc/reader.go:104-122,28-36 | CONFIRMED |
| C53 | opc/all | All three consumers alias `writer.ContentTypes = reader.ContentTypes`; `Writer.Close` mutates the shared maps (SetOverride/AddRelationship) — repeated saves mutate the open document's state; concurrent saves are a data race | opc/writer.go:242-243; pptx/presentation.go:675; docx/document.go:316; xlsx/workbook.go:331 | CONFIRMED / race PLAUSIBLE |
| C54 | opc | `Close()` sets `closed=true` before writing metadata; on mid-Close error the zip has no central directory, `zipWriter.Close()` never runs, and there is no Abort — error-path `_ = writer.Close()` in consumers *finalizes* rather than discards | opc/writer.go:314-347 | CONFIRMED |
| C55 | opc | `ExtendedProperties.Marshal` hardcodes `TotalTime/Words/Paragraphs/Notes/HiddenSlides/MMClips/ScaleCrop/LinksUpToDate/SharedDoc/HyperlinksChanged` to 0/false despite documented settable fields; no Unmarshal exists at all | opc/package.go:275-322 | CONFIRMED |
| C56 | docx | `nextRelID` seeded from `len(rels)+1` — collides on documents with non-contiguous rIds (normal after Word edits) → duplicate `Id` in .rels | docx/document.go:578-586 | CONFIRMED |
| C57 | docx | `AddHeader` part-name counter starts at 0 regardless of existing header1.xml — name collision clobbers the parsed entry | docx/headerfooter.go:79-106 | CONFIRMED |
| C58 | docx | Field named `Conformance` actually stores `mc:Ignorable`; the real `w:conformance` attribute is silently dropped (strict-conformance docs demoted) | docx/internal/oxml/document.go:12,24-26; docx/marshal.go:19-21 | CONFIRMED-runtime |
| C59 | docx | `SetBold(false)` (and SetItalic/SetStrike/…) removes the element = "inherit", not "off" — cannot un-bold text inheriting bold from a style; tri-state semantics unrepresented | docx/run.go:39-46 | CONFIRMED |
| C60 | all | Part/rels load errors are swallowed with `continue` in all three formats — a truncated/corrupt part silently vanishes from preservedParts and thus from the saved file; malformed slides/masters/layouts likewise silently omitted | docx/document.go:146-149,234-241; xlsx/workbook.go:119-124; pptx/presentation.go:199-201,299-319 | CONFIRMED |
| C61 | docx | `saveRoundTrip` hardcodes `/word/document.xml` while open resolves the main part from rels — a valid docx with a different main-part name gets both stale-preserved and regenerated parts | docx/document.go:337 vs 84 | CONFIRMED |
| C62 | docx | `AddHeading` level unvalidated: level 10 yields style `"Heading:"` via rune arithmetic | docx/document.go:517-522 | CONFIRMED-runtime |
| C63 | docx | `Paragraph.Text()` ignores hyperlink/SDT/ins text; `SetText` replaces only `p.R`, leaving hyperlink content — replaced and stale text coexist in output | docx/paragraph.go:17-32 | CONFIRMED-runtime |
| C64 | docx | `Create()` never writes styles.xml — `AddHeading`'s "Heading1" style is undefined; README's heading example renders as plain text in Word | docx/document.go:250-263,397-407 | CONFIRMED |
| C65 | docx | Paragraphs from table cells have nil document backref — `AddImage` inside any table cell always errors "run is not attached to a document", even on Create() docs | docx/table.go:181-193 | CONFIRMED-runtime |
| C66 | xlsx | Rows are marshaled in insertion order, never sorted (cells within a row *are* sorted) — `SetCellValue("A5")` then `("A1")` emits row 5 before row 1; breaks streaming readers | xlsx/marshal.go:296-307 | CONFIRMED-runtime |
| C67 | xlsx | NaN/±Inf accepted and emitted as `<v>NaN</v>` — invalid numeric cell | xlsx/cell.go:180-186 | CONFIRMED-runtime |
| C68 | xlsx | `int64`/`uint64` routed through float64 (precision loss above 2^53) while `int` gets exact formatting | xlsx/cell.go:63-96 | CONFIRMED-runtime |
| C69 | xlsx | `SetTime` uses absolute UTC delta (dates shift by zone offset), mis-handles the 1900 leap-day fraction, and applies no date number format (cell displays raw serial) | xlsx/cell.go:304-315 | CONFIRMED-runtime |
| C70 | xlsx | No bounds checks on cell references: rows >1048576, cols >XFD, int overflow on long column strings (`ParseCellRef("AAAAAAAAAAAAAA1")` → garbage, nil error); `FormatCellRef(5,0)` returns invalid `"5"` | xlsx/workbook.go:858-895; sheet.go:284-316 | CONFIRMED-runtime |
| C71 | xlsx | Sheet-name rules unenforced: duplicates, empty, >31 chars, forbidden `[]:*?/\` all accepted; `AddSheet` has no error channel; `SetName` doesn't update references | xlsx/workbook.go:773-796; sheet.go:29-36 | CONFIRMED-runtime |
| C72 | xlsx | `Styles()` is a getter with write side effects — merely *reading* styles forces styles.xml regeneration + new rels, breaking byte-identical round-trip | xlsx/workbook.go:736-742 | CONFIRMED-runtime |
| C73 | xlsx | Rows without the optional `r` attribute (legal SpreadsheetML) are invisible: `GetCellValue` returns "" and `Cell()` creates a duplicate row | xlsx/sheet.go:60-65,121-131 | CONFIRMED-runtime |
| C74 | xlsx | `CT_Worksheet` root captures only xmlns decls — `mc:Ignorable`, `xr:uid` root attrs dropped when a sheet is regenerated (x14ac attrs then reference an undeclared Ignorable) | xlsx/internal/oxml/worksheet.go:54-68 | CONFIRMED |
| C75 | xlsx | `DeleteSheet` leaves an orphan Content-Types Override (and sheet-owned drawings/rels), doesn't clamp `ActiveTab` or reindex `localSheetId` in defined names | xlsx/workbook.go:799-816 | CONFIRMED-runtime |
| C76 | xlsx | `DataValidation.ShowDropDown` mapping inverted vs OOXML (attr means *hide*) — the option can never hide the dropdown; `ShowErrorMessage` never set even when error text provided | xlsx/sheet.go:450-458 | CONFIRMED |
| C77 | xlsx | `encodeUnknownElement` re-emits captured attribute values without escaping — unknown children with `&`/`<`/`"` in attrs produce malformed workbook.xml | xlsx/internal/oxml/workbook.go:236-250 | CONFIRMED |
| C78 | xlsx | Worksheet parse failures are swallowed at open; a later mutation on that sheet fabricates an **empty** worksheet that replaces the original part — total sheet data loss | xlsx/workbook.go:213-219,549-558 | CONFIRMED |
| C79 | pptx | `AddPicture(path)` is a stub: file never read, no media part, `<a:blip/>` with no r:embed, nonexistent path returns nil error | pptx/slide.go:85-93,617 | CONFIRMED-runtime |
| C80 | pptx | `Paragraph.SetLineSpacing/SetSpaceBefore/SetSpaceAfter` and `Run.SetHighlight` are never serialized (asymmetric with the readers) | pptx/slide.go:392-472 | CONFIRMED-runtime |
| C81 | pptx | Table cell border setters (`SetBorderLeft`…`SetBorders`) never serialized; `TableBorder` type consumed by nothing | pptx/table.go:353-379; slide.go:554-576 | CONFIRMED |
| C82 | pptx | `GroupShape.Position/Size` read `ChOff/ChExt` (child coordinate space) instead of `Off/Ext` — wrong placement reported | pptx/oxml_to_domain.go:235-245 | CONFIRMED |
| C83 | pptx | Dead options APIs: `CreateOptions.SlideSize/DefaultFont/Locale` ignored (`DefaultOptions()` says Widescreen; `Create()` hardcodes 4:3; app.xml hardcodes "Widescreen" regardless); `OpenOptions`, `SaveOptions`, `ExportOptions`/`ExportFormat*` consumed by nothing | pptx/options.go:100-215; presentation.go:456-487,1023 | CONFIRMED |
| C84 | pptx | `Theme()`/`SlideMaster.Theme()` always return nil (never assigned); `SlideMaster.Placeholders()`/`SlideLayout.Placeholders()/GetPlaceholder()` are stubs returning nil — the 319-line theme.go getter/setter surface is unreachable; README promises "placeholders and themes" | pptx/presentation.go:1520-1523; master.go:64-94; layout.go:65-79 | CONFIRMED |
| C85 | pptx | `AddShape` accepts any `Shape` (incl. GroupShape) but the sync switch silently discards types it doesn't handle — programmatically built groups never serialized, no error | pptx/slide.go:60-64,202-225 | CONFIRMED |
| C86 | pptx | `ReplaceText` applies replacements sequentially over Go map iteration — values containing other keys get re-replaced; result is order-dependent and formally nondeterministic | pptx/template.go:279-284 | CONFIRMED-runtime |
| C87 | pptx | Paragraphs containing `<a:br>`/`<a:fld>` silently revert multi-run replacements (restore original runs, return false) — no error, no report; API returns nothing | pptx/template.go:308-318 | CONFIRMED |
| C88 | pptx | `RemoveSlide` leaves the slide's notesSlide part + rels (targeting the deleted slide) and stale ContentTypes overrides | pptx/presentation.go:1462-1482,674-676 | PLAUSIBLE (mechanism traced) |
| C89 | pml | `gridSpacing` typed as x/y attrs but XSD says cx/cy (`CT_PositiveSize2D`) — parses zeros, marshals invalid | pptx/internal/oxml/viewprops.go:17 | CONFIRMED (latent) |
| C90 | pml | Latent schema mismatches in unwired types: `HtmlPublishProperties` missing required slide-list choice + has phantom `pubBrowser` attr; `CustomShow` missing required `sldLst`; `ColorMRU` loses order and 4 color kinds; "CT_SlideProperties" family doesn't exist in pml.xsd | pptx/internal/oxml/presprops.go:18-24,88-117; presentation.go:98-102 | CONFIRMED |
| C91 | pml | `defaultTextStyle` pipeline lossy: srgbClr solid fills dropped by writer; `tint>val` tags map to child elements instead of attrs; TextListStyle family duplicates `dml.LstStyle` while omitting bullets/spacing — stripped on every save since presentation.xml always regenerates | pptx/marshal.go:303-307; pptx/internal/oxml/presentation.go:244-330 | CONFIRMED |
| C92 | pml | Table types lossy: `ATblPr` missing rtl/fills/tableStyle element; `ATcPr` missing gradient/pattern fills, diagonal borders, cell3D, headers; `ATc` missing id; `GraphicFrameLocks` only noGrp (loses noChangeAspect) | pptx/internal/oxml/table.go:68-120 | CONFIRMED |
| C93 | dml | `BlipFillXML` (used by SpPr/RPr/TcPr/TblPr/slide background) has no CT_Blip effect children — biLevel/duotone/lum/clrChange on texture fills silently dropped; full-fidelity `BlipFill` exists but only pictures use it | common/dml/xml_types.go:405-436; pptx/internal/oxml/slide.go:124 | CONFIRMED-runtime |
| C94 | dml | `a14:useLocalDpi`/`a14:shadowObscured` val typed `*int32` but attr is xsd:boolean — `val="true"` (valid lexical form) makes the whole part fail to open with a strconv error | common/dml/xml_extension.go:76,81 | CONFIRMED-runtime |
| C95 | dml | Color transforms lossy and inconsistent: `SrgbClr` supports 6 of ~28 transform kinds and reorders them (order is semantic per ECMA-376 §20.1.2.3); `SchemeClrTransform` map missing gamma/red*/green*/blue*; argument-less transforms re-emit bogus `val="0"` | common/dml/xml_types.go:22-30,140-155 | CONFIRMED-runtime |
| C96 | dml | `GradFill` missing the `flip` attribute — dropped | common/dml/xml_types.go:366-372 | CONFIRMED-runtime |
| C97 | dml | `ArcTo.StAng/SwAng` marked omitempty but XSD-required — `stAng="0"` dropped, schema-invalid arc | common/dml/xml_geometry.go:352-353 | CONFIRMED-runtime |
| C98 | dml | `TcTxStyle.Font` typed as `*FontRef` but `a:font` there is `CT_FontCollection` — table-style typefaces lost, emits invalid `<a:font idx=""/>` | common/dml/xml_table.go:132 | CONFIRMED-runtime |
| C99 | dml | Alpha=0 means "unset" (`WithAlpha(0)` silently opaque); theme branch ignores Alpha entirely; `ColorTypeSystem` yields `<a:solidFill/>` with no child (invalid); `WithAlpha` doesn't clamp | common/dml/fill.go:103,127; color.go:156-161 | CONFIRMED-runtime |
| C100 | dml | `GraphicData.RawContent` captured on unmarshal but never marshaled (`xml:"-"`, no custom marshal) despite doc promising a preserve fallback | common/dml/xml_media.go:96-133 | CONFIRMED |
| C101 | dml | `Theme` missing `custClrLst` and `extLst` (thm15:themeFamily lives there) | common/dml/xml_theme.go:5-10 | CONFIRMED (latent) |
| C102 | common | XML-invalid control chars (\x00-\x08 etc.) pass through both escapers — user text containing them produces unparseable parts (not representable even as char refs) | common/xml/builder.go:474-501 | CONFIRMED-runtime |
| C103 | common | Namespace URIs written raw (unescaped) in four Builder entry points — xmlns values containing `&`/`"` re-emit as malformed XML | common/xml/builder.go:134,178,367,404 | CONFIRMED-runtime |
| C104 | common | Builder has no element stack: mismatched/unclosed elements silently produce invalid XML; `level` can go negative; no Finish()/Err() | common/xml/builder.go:220-229,453-468 | CONFIRMED-runtime |
| C105 | common | `hasStructChildren` treats all zero-value fields as absent regardless of omitempty — mandatory zero-valued children silently dropped and element self-closes | common/xml/marshal.go:266-314 | CONFIRMED-runtime |
| C106 | common | Builder reflection ignores `xml.Marshaler`/`MarshalerAttr` — types relying on custom MarshalXML serialize differently in production than in encoding/xml-based tests (root enabler of the pml r:id dead code and the test-path divergence) | common/xml/marshal.go | CONFIRMED |
| C107 | common | `vml.Textbox` content destroyed: unmarshal keeps only the last child's inner XML (wrapper `<div>` lost), and no MarshalXML exists so even that is never written | common/vml/vml.go:281-343 | CONFIRMED-runtime |
| C108 | common | `vml.ClientData` presence-flag elements (`<x:MoveWithCells/>` etc., semantic for every Excel comment) modeled as string+omitempty — parsed as "" and dropped on marshal; repeatable/ordered children collapsed | common/vml/vml.go:749-780 | CONFIRMED-runtime |
| C109 | common | `common/docprops` is a dead package (zero importers) that is also wrong: `category`/`contentStatus` given DC namespace (XSD says cp), invented `Worksheets` element, missing `HyperlinkBase`; its test bakes the namespace bug in; opc has the correct production implementation | common/docprops/core.go:10-11,57 | CONFIRMED |
| C110 | common | Diagram models lossy: `PrSet` missing `presLayoutVars`/`style` children; `LayoutNode` (XSD: ordered choice) rebuilt from grouped slices → semantic evaluation order lost; `ColorList` supports 2 of 6 color kinds | common/dml/diagram/diagram.go:51-80; layout.go:58-73; colors.go:31-36 | CONFIRMED |
| C111 | all | Systemic test antipattern: every oxml/dml test round-trips via `encoding/xml`, never the production Builder path; dominant pattern is "no-error round-trip" with no value assertions — all confirmed silent-loss bugs above pass the existing suites | common/dml/xml_*_test.go; pptx/internal/oxml/*_test.go; common/*/…_test.go | CONFIRMED |
| C112 | dx | Committed Go tests depend on the untracked `python-tests/` directory (python-pptx's test suite, 2.9M, maintainer-machine-only) — 41 tests silently skip on fresh clones; provenance/licensing unattributed | pptx/schema_test.go:16; git status | CONFIRMED-runtime |
| C113 | dx | Tracked `spec/README.md` documents `gen_spec/` which is untracked; the script inside carries a hardcoded third-party Dropbox path (python-pptx author's tooling), unattributed | spec/README.md:119; spec/gen_spec/gen_spec.py | CONFIRMED |
| C114 | dx | `SVG_PPTX_EMBEDDING_PLAN.md` at repo root reads as an open TODO but the feature is fully implemented (same commit 4ebf569 added plan + implementation + tests); README never mentions SVG support | SVG_PPTX_EMBEDDING_PLAN.md | CONFIRMED |
| C115 | dx | `fetch.sh` exits 1 if any download fails and `make test` hard-depends on fetch — one dead third-party URL bricks the suite for newcomers (tests themselves skip gracefully); no checksums, two URLs track a mutable `master` branch | testdata/fetch.sh:75-78; Makefile:6; testdata/external.txt | CONFIRMED |
| C116 | dx | xlsx package doc: "This package is currently a placeholder and will be fully implemented in a future release" — first sentence on pkg.go.dev for a fully-featured package; `ErrNotImplemented`/`ErrInvalidRange` declared, never returned | xlsx/errors.go:1-9 | CONFIRMED |
| C117 | xlsx | `<dimension>` preserved verbatim and never recomputed when cells are added outside it; row `spans` likewise stale | xlsx/marshal.go:214-216 | CONFIRMED-runtime |
| C118 | pptx | Loaders skip slide parts not referenced in `sldIdLst` — unreferenced-but-present slide parts vanish on save | pptx/presentation.go:226-234 | CONFIRMED |

### Low

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C119 | opc | `ValidatePartName` far looser than ECMA-376 Part 2 (allows backslash, unencoded space/%, trailing-dot segments) — round-tripped hostile names become a zip-slip vector for naive extractors | opc/part.go:31-63 | CONFIRMED / impact PLAUSIBLE |
| C120 | opc | Swallowed errors: `parseCoreProperties` bare-continues on read/parse failure; `UnmarshalCoreProperties` never checks the root element name and ignores `DecodeElement` errors | opc/reader.go:184-208; package.go:363-366,397-417 | CONFIRMED |
| C121 | opc | Dead parallel abstraction: `Part`, `PartError`, `RelationshipError`, `ErrPartNotFound`, `ErrInvalidRelationship`, `ErrInvalidContentType` referenced only by their own tests — two competing "part" types confuse consumers | opc/part.go:10-22; errors.go | CONFIRMED |
| C122 | opc | Regenerated core.xml diverges cosmetically: `xmlEscape` over-escapes `'`/`"` in element text (against the project's own escaping rule); dcterms dates forced to UTC Z, sub-second dropped | opc/package.go:203-222,136,142 | CONFIRMED |
| C123 | opc | Writer contract gaps: `CreatePart`'s returned writer silently invalidated by the next Create/Close (undocumented); `AddRelationship` collides with pre-seeded `Relationships`; duplicate (case-insensitive) part names in hostile zips accepted with two-sources-of-truth disagreement; "CT must be last" comment inverts OPC streaming guidance | opc/writer.go:55,142-152,336; reader.go:104-149 | CONFIRMED |
| C124 | docx | Dead/misnamed code: `nsR = xmlb.NSPresentationRels` (unused, pptx-named); `pointsToTwipsSigned` byte-identical to `pointsToTwips`; unreachable `attr.Name.Local == "r:id"` branches; `CT_HeaderReference` spec-test-only | docx/marshal.go:10; section.go:125-129; oxml/fields.go | CONFIRMED |
| C125 | docx | Minor API traps: `SetMargins` skips Header/Footer ≤0 (can't set 0); `AddRow()` creates a cell-less row (schema-invalid if left empty); parsed headers/footers have no public accessors (double-parsed dead weight); `UnmarshalOnOff` swallows `d.Skip()` error; CT_Compat/CT_WebSettings marshal iterates a map (latent nondeterminism) | docx/section.go:106-111; table.go:43-47; document.go:29-30; oxml/properties.go:294; oxml/settings_types.go | CONFIRMED |
| C126 | xlsx | `Cell()` is a mutating getter: creates row+empty cell without dirty-marking (phantom `<c r=…/>` once the sheet is dirtied elsewhere); un-normalized refs (`"A01"` creates a cell distinct from A1 with invalid `r="A01"`) | xlsx/sheet.go:45-88 | CONFIRMED-runtime |
| C127 | xlsx | `SetColWidth` matches only single-column entries — with an existing ranged `<col min=1 max=5>` it appends an overlapping second entry | xlsx/sheet.go:179-199 | CONFIRMED-runtime |
| C128 | xlsx | `MergeCells` accepts duplicates and overlapping ranges (Excel repairs); no ref validation | xlsx/sheet.go:236-255 | CONFIRMED-runtime |
| C129 | xlsx | `SetString` writes `t="str"` (spec: *formula string result*) for plain literals; no `xml:space="preserve"` for leading/trailing whitespace | xlsx/cell.go:160-165 | CONFIRMED |
| C130 | xlsx | Zero-sheet workbook saves without error (schema requires ≥1 sheet) | xlsx/workbook.go:461-547 | CONFIRMED-runtime |
| C131 | xlsx | `NumberFormat*` ID constants unusable (no API accepts an ID); comment drift (`NumberFormatDate=14 // "m/d/yyyy"` vs builtin "mm-dd-yy"); builtin table omits IDs 4-8/37-48; dead double assignment of `CellXfs.Count` | xlsx/style.go:11-49,196-199 | CONFIRMED |
| C132 | xlsx | `CellTypeDate` unreachable (`Type()` never inspects numFmt); `Value()` returns cached string for formula cells, float64 for all numbers — undocumented | xlsx/cell.go:36-53,100-127 | CONFIRMED |
| C133 | xlsx | `FreezePanes("A1")` emits a frozen pane with no splits; selection refs keep caller's case while TopLeftCell is uppercased; negative `Indent`/`Rotation` wrap via uint32 to invalid values | xlsx/sheet.go:320-361; style.go:448-455 | CONFIRMED |
| C134 | xlsx | ~1200 lines of spec-only oxml scaffolding (pivot, connections, ext types) unreachable from load/save; `CT_Rst.PhoneticPr` mislabels `rPh` | xlsx/internal/oxml/pivot.go et al. | CONFIRMED |
| C135 | pptx | Any paragraph with alignment/level set gets explicit `<a:buNone/>` (BulletNone is the zero value), overriding layout-inherited bullets; `BulletAuto` silently unhandled | pptx/slide.go:398-413 | CONFIRMED |
| C136 | pptx | `NewRun` defaults fontSize=12 — API-created runs clobber placeholder-inherited sizes with `sz="1200"` | pptx/text.go:247-253; slide.go:435-437 | CONFIRMED |
| C137 | pptx | `replacePictureImage` matches the first picture whose blip embeds the rel ID — two pictures sharing one image: wrong one replaced; no media dedup (identical bytes re-added per call) | pptx/image_replace.go:181-197 | CONFIRMED |
| C138 | pptx | `Duplicate()` burns a slide part number (name assigned twice) and clones the notesSlide rel so two slides share one notes part | pptx/slide.go:649-681 | CONFIRMED |
| C139 | pptx | Default layouts mix 16:9 placeholder geometry (12.33") with 4:3 helpers on a 10" 4:3 default slide — TwoContent/SectionHeader placeholders overflow | pptx/layout.go:209-236 vs placeholder.go:294-323 | CONFIRMED |
| C140 | pptx | `SetTransition` zero-value trap: `AdvanceOnClick=false` by default (PowerPoint default is true, and per C29 false is unrepresentable anyway); Duration bucketed to fast/med/slow but read back as fabricated 0.5/1.0/2.0 — not round-trip stable | pptx/transition.go:41-48 | CONFIRMED |
| C141 | pptx | Decorative media API: Video/Audio/OLEObject/MediaPosition have full getter/setter surfaces with no serialization path | pptx/media.go:172-334 | CONFIRMED |
| C142 | pptx | No synchronization anywhere; `marshalPresentation` mutates shared state in place — concurrent Save+mutate is a data race; undocumented | pptx/presentation.go:1340-1367 | CONFIRMED |
| C143 | pptx | Byte-level `commonPrefixLen/commonSuffixLen` can split runs mid-rune (multi-byte UTF-8 sharing prefix bytes) → invalid UTF-8 in run text | pptx/template.go:424-451 | PLAUSIBLE |
| C144 | pml | Dead/fabricated types: ~25 p14 Transition* structs (real p14 transitions flow through AlternateContent), ModernComment* (wrong ns, no tags), CommentList/TagList, "CT_SlideProperties" family, write-only `Xmlns*` fields, dead `r:id` MarshalXML hacks (Builder ignores xml.Marshaler); required-element omitempty traps (`Comment.Pos/Text`, `notesSz`, `Template.TnLst`) | pptx/internal/oxml/transition.go:95-227; comments.go; presentation.go:13-15,164-235 | CONFIRMED |
| C145 | dml | Unit converters truncate instead of round (`Inches(0.57)` = 521207 EMU, off by one); `ToPixels` floor-divides (1.9999px → 1) | common/dml/geometry.go:20-67 | CONFIRMED-runtime |
| C146 | dml | Dead/duplicated/fabricated types: `Blip` vs `BlipXML`, `Tile` vs `TileXML`, three CT_RelativeRect models, `SchemeClr` (zero uses); `Ligatures/NumForm/WordArt/RichText/Wsp/PicProps…` have no XSD basis or wrong namespaces; `WPAnchor` required attrs omitempty; `XDRTwoCellAnchor` lacks graphicFrame (xlsx bypasses with printf XML); `EffectContainer/EffectDag` model 1 of ~30 children; `NewGradientFill` accepts out-of-range angles; `PathXML2D` w/h parsed via error-ignoring Sscanf; constructor doc comments name the wrong functions | common/dml/xml_misc.go, xml_text.go:509-596, xml_wpdrawing.go, xml_xdrdrawing.go, xml_effect.go:89-104, fill.go:137, xml_geometry.go:90-99 | CONFIRMED |
| C147 | common | Builder/misc long tail: hand-rolled `itoa(math.MinInt64)` returns `"-"`; `writeQName` silently emits unprefixed names for unregistered namespaces; `marshalReflect` has no Interface/Map/Float element cases (silently skipped); `RelsPathToSourcePart` mangles folders ending in `_rels`; `ExtensionPrefixToNS` missing common prefixes (wps/wpg/x14ac/v); dead NS constants; `NSPresentationRels` misnames the generic officeDocument rels NS used by all three formats | common/xml/builder.go:611-628,347-354; marshal.go:189-230; common/oxml/rawpart.go:13-21; common/xml/namespace.go:22,33,69,83-91 | CONFIRMED-runtime (itoa) / CONFIRMED |
| C148 | common | vml/enum long tail: misleading VML field names (`MiniGo`→minusx, `FaceAt`→facet, SignatureLine fields mismapped); `FontStyleBold = 1<<iota` starts at bit 1 (bit 0 dead); UnderlineStyle missing 6 of 18 ECMA values, TextAlign missing justLow/thaiDist | common/vml/vml.go:461,490,507-509; common/enum/alignment.go:7-13,44-83 | CONFIRMED |
| C149 | dx | Onboarding long tail: go.mod pins `go 1.25.5` while README says "Go 1.25 or later"; `.golangci.yml` is 13 bytes with no version pin (v1 binaries fail on it); README never mentions `examples/`, `SaveBytes`, SVG support; lifecycle verbs inconsistent (`SaveAs` pptx-only, `WriteToBuffer` xlsx-only and redundant with `SaveBytes`, template/options constructors pptx-only) | go.mod:3; README.md:329; .golangci.yml | CONFIRMED |

---

## 2. System map

### Layering

1. **`opc/`** — thin layer over `archive/zip`. Read: `[Content_Types].xml` parsed first (mandatory), `Files` list built, `/_rels/.rels` parsed (fatal on error), core properties best-effort (errors swallowed). Write: a lowercased `parts` map provides both duplicate detection and the **skip-if-already-written contract**: `Close()` writes core.xml, app.xml, `.rels`, and `[Content_Types].xml` only if a consumer hasn't already written those paths. `WriteRawFile` bypasses part-name validation for round-trip fidelity. This skip contract is load-bearing and double-edged (C10, C46).
2. **`common/xml`** — the Builder, the production serializer for every regenerated part. Reflection-based (`MarshalRoot`/`MarshalElement`), honors `BuilderMarshaler`, **ignores `xml.Marshaler`/`MarshalerAttr`** (C106). Bools as `"1"/"0"`. Deliberate context-aware escaping (attr vs text) that diverges from `xml.EscapeText` — correct in design, incomplete at the edges (C102, C103).
3. **`common/*` models** — dml (DrawingML core), chart, diagram, vml, omml, oxml (AlternateContent), docprops, enum. Reality check: **only pptx consumes dml**; xlsx builds drawing XML with `fmt.Fprintf` templates; docx keeps drawings as raw bytes; vml/omml/chart/diagram are effectively spec-test-only today. docprops is dead and wrong (C109); the correct core-properties implementation lives in opc.
4. **`{pptx,docx,xlsx}/internal/oxml`** — typed models with custom `UnmarshalXML` + `MarshalToBuilder` and childOrder machinery. Maturity is wildly uneven: xlsx `CT_Workbook` has the full fidelity kit (OriginalRootAttrs, UnknownChildren raw capture, ElemSeparator, SelfClosingSpace); xlsx `CT_Worksheet`, all of docx, and all of pml lack unknown-child capture — everything they don't model is silently deleted when their part is re-marshaled (C14, C27-C28, C32-C33).
5. **Public packages** — domain wrappers. pptx additionally maintains a third representation (materialized domain shapes) synced *bidirectionally* with the oxml tree, and both sync directions are destructive (C1, C5).

### Real execution paths

- **Create** → in-memory model only → `saveNew` (writes everything from the model + baked defaults). This is the well-tested happy path; the examples and README all live here.
- **Open** → every part's raw bytes eagerly copied into memory (which is why save-over-source is safe in all three formats — the fd is never re-read); selected parts parsed into models; the rest bucketed as preserved raw parts.
- **Save on an opened document** → `saveRoundTrip`: preserved raw bytes win for untouched parts. What gets regenerated differs per format and is the source of most critical findings:
  - **pptx** re-marshals slides, masters, layouts, and presentation.xml **unconditionally** — fidelity = oxml-model completeness (good for common content, per the corpus test; lossy for everything in C4/C32-C36).
  - **docx** regenerates **only** document.xml (always) and writes **no** new parts — so mutations needing new parts corrupt (C3, C26), and regeneration strips whatever the model skipped (C27, C28).
  - **xlsx** regenerates workbook.xml always (with full fidelity kit) + dirty sheets (without it, C14) + styles when *touched* (even read-only, C72). `[Content_Types].xml` is regenerated from the parsed model in xlsx but preserved raw in pptx/docx — three formats, three strategies (C6/C7 exist because of the xlsx strategy).

### Key invariants and where they hold

- "Byte-identical round-trip for unmodified parts" (README): holds for pptx/docx across the corpus; **broken in xlsx** for files with uppercase content-type extensions or LF prologs (C6, C7) — and the maintainer's red tests prove it.
- "childOrder is maintained for every mutation": holds only for xlsx sheet cells and three docx table helpers; every other docx mutator violates it (C2).
- "Everything parsed is re-emitted": violated in every hand-modeled type flagged above; nothing enforces marshal/unmarshal symmetry (no Builder-vs-encoding/xml equivalence tests, no golden-byte tests for programmatic marshal — C111).
- "Errors surface": violated pervasively — part-load failures, sheet parse failures, core-props parse failures are all swallowed (C60, C78, C120); most mutators return nothing.

---

## 3. Findings by category

Full detail for critical/high findings; medium/low carry their table one-liners plus a scenario where non-obvious. Category assignment is primary-cause; the summary table above is the severity-ordered index.

### 3.1 Correctness (logic errors, silent data loss, wrong output)

**C1 — pptx/slide.go:183-228 — shape sync deletes what it doesn't model.** `syncShapesToXML` runs whenever any shape on a loaded slide was touched (`AddTextBox`, `SetName`, `RemoveShape`…). It nils `spTree.Sp/GraphicFrame/Pic/GrpSp` and rebuilds from the materialized domain shapes — but materialization returns nil for non-table graphic frames (charts, SmartArt, OLE) and the rebuild switch has no GroupShape case. Reproduced: opening a real deck with a group shape and calling `slide.AddTextBox()` removed the group from the output (`grpSp 1→0`). The rebuild also strips rotation/fill/style/SVG-blip detail the domain model doesn't carry, renumbers all shape IDs from 2 (breaking animation `spid` references), and never clears `CxnSp` (connectors keep stale z-order). Direction: preserve unmodeled spTree children verbatim and do surgical per-shape sync instead of clear-and-rebuild.

**C2/C3/C26/C43 — docx mutation API is only wired to the Create() path.** The marshal for parsed containers iterates `childOrder` exclusively; only `AppendRow`/`AppendCell`/`AppendDrawingChild` maintain it. Every other public mutator appends to typed slices the marshal never reads. Reproduced end-to-end: open→`AddParagraphWithText("NEW")`→save→reopen = paragraph count unchanged. Worse, `saveRoundTrip` writes no image/header/footer/numbering parts (only `saveNew` does) while relationships and references *are* written → dangling r:id, Word repair prompt (C3). On the Create path, the fallback marshal emits all paragraphs then all tables, reordering interleaved content (C43). Direction: childOrder-aware append helpers on CT_Body/CT_P/CT_Tc used by all mutators, plus dirty-part tracking so round-trip saves write new parts.

**C4 — pptx/marshal.go:40-145 — presentation.xml regenerated lossily every save.** There is no modification check; the hand-written marshal emits sldMasterIdLst/notesMasterIdLst/handoutMasterIdLst/sldIdLst/sldSz/notesSz/defaultTextStyle/extLst plus three attributes. The oxml struct parses `modifyVerifier`, `custShowLst`, `embeddedFontLst`, `photoAlbum`, `smartTags`, `kinsoku`, `custDataLst`, and ten more attributes (`firstSlideNum`, `rtl`, `conformance`…) — all dropped. Reproduced with an injected `firstSlideNum="5"`. Dropping `embeddedFontLst` also strands font parts/rels. Direction: drive presentation.xml through the Builder over the full struct (like slides), or gate regeneration on actual modification.

**C6/C7 — opc/content_types.go — the two red tests.** `strings.ToLower` at parse (line 268) and lookup (102/113) erases original extension case; `Marshal` hardcodes `\r\n` after the XML declaration (154-155). Verified by hexdump: fred_data's original prolog separator is `0a`, round-trip emits `0d0a` (1303→1304 bytes). Direction: store original-case extensions (compare case-insensitively per OPC), and capture the original prolog separator like xlsx's `OriginalXMLSep` does.

**C11 — xlsx/sheet.go:66-87 — `*Cell` handles dangle.** `Cell()` returns `&targetRow.C[len-1]`; the next `Cell()` on the same row can realloc the slice, detaching every prior handle. Reproduced: hold A1's handle, create B1..I1, write through the handle → A1 empty. Direction: store (rowIdx, ref) and re-resolve per access, or use `[]*CT_Cell`.

**C13/C75 — xlsx sheet lifecycle:** `SheetId: uint32(len(w.sheets))` collides after delete+add (duplicate sheetId in output, reproduced); DeleteSheet leaves the dead part's Content-Types override and doesn't fix ActiveTab/localSheetId.

**C16/C17/C18 — pptx slide/layout lifecycle on opened decks:** part-name collision makes Save *fail outright* after RemoveSlide+AddSlide (the correct `nextAvailableSlidePartName` helper exists but only `Duplicate` uses it); `AddLayout` emits empty rel IDs; `AddSlide` produces a slide with no layout relationship — the exact flow the README demonstrates.

**C20/C86/C87 — ReplaceText correctness family.** The prefix/suffix overlap case double-emits runs (reproduced: `aa`→`a` not applied); replacements cascade through each other in Go-map order (reproduced: `{"A":"B","B":"C"}` yields `C`); paragraphs with breaks/fields silently revert. Direction: clamp prefix+suffix to text length; single-pass simultaneous replacement; return a result/count so callers can detect non-application.

**C29 — the default-true boolean class.** ECMA-376 is full of attributes whose absence means *true*. Modeling them as `bool` + omitempty makes explicit `="0"` unrepresentable: parse → false → omitted → readers see true. The public symptom: `Transition.AdvanceOnClick=false` cannot be expressed at all (pptx/transition.go:46). The codebase already knows the fix — `HeaderFooter`, `SlideLayout`, `BuildParagraph`, `OuterShdw` use `*bool` correctly; this is copy-paste drift. A Builder-level lint (flag non-pointer bool with omitempty on types with XSD default true) would close the class.

**C30/C31 — `dml.ExtLst` reused across namespaces.** Its `Ext` field matches `a:ext`; PML and ChartML extension lists contain `p:ext`/`c:ext`. Everywhere the type is borrowed outside DrawingML-main, extensions parse empty and are deleted on re-marshal. xlsx built its own correct `CT_Extension`; pml and chart need the same.

**C37-C40, C93-C99 — DrawingML modeling defects** (all runtime-probed): `a:rtl` chardata inversion; `Duotone`/`Gs` emitting schema-invalid empty elements; theme colors silently black in gradients/patterns/shadows; `BlipFillXML` dropping blip effects; a14 bool-as-int parse hard-failure; transform reorder/drop/bogus-`val`; missing `flip`; required arc angles dropped; table-style font type confusion; alpha-zero ambiguity.

**C41/C42 — common/oxml + omml:** AlternateContent collapses multiple Choices (last wins) and drops empty Fallbacks — production path for docx paragraphs and pptx slides; omml's core `OMath` type has simply never worked in either direction.

**C47/C48 — opc core properties:** legal W3CDTF dates (`2024-01-15`) silently dropped; unknown children (vendor extensions) dropped on regeneration.

**C58 — docx:** the field named `Conformance` stores `mc:Ignorable`; real `w:conformance` is discarded — a name that lies twice.

**C66-C70, C73, C117 — xlsx cell/data long tail** (all reproduced): unsorted rows; `NaN`/`Inf` emitted verbatim; int64 precision loss; timezone-shifted and mis-epoched `SetTime` with no date format; unbounded/overflowing reference parsing; r-less rows invisible; stale `dimension`.

**C82 — pptx:** GroupShape position read from child-space coordinates.

**C105 — common/xml/marshal.go:266-314:** `hasStructChildren` ignores omitempty — a struct whose only child is a mandatory zero-valued element self-closes, silently dropping the child.

**C145 — dml:** unit conversions truncate rather than round; `Inches(0.57)` is off by one EMU, `ToPixels` floors 1.9999px to 1.

### 3.2 Unintended paths (second calls, lifecycle misuse, holding it wrong)

**C15 — xlsx Close-then-Save** silently discards every preserved part: `Close()` nils the reader, and `Save` interprets nil reader as "created workbook" → `saveNew`. No error at any point. Direction: keep a `wasOpened` flag and error, or retain preserved parts (already in memory).

**C5/C19 — pptx ReplaceText's two lifecycle traps** (both reproduced): on opened decks it wipes unsaved domain-side additions via re-materialization; on created decks it does nothing at all. The same method is a destructive operation or a no-op depending on how the presentation was constructed — nothing in the signature or docs distinguishes the two.

**C10 — Properties-after-Open** (docx, xlsx; reproduced): the preserved core.xml wins and the assignment is silently ignored. pptx escapes only because it regenerates core properties when `hasModifications` — a third behavior for the same field.

**C46 — WriteRawFile CT freeze:** once a consumer writes `[Content_Types].xml` raw, every later `CreatePart` override is accumulated into a map that is never serialized. docx does exactly this; it survives today only because docx's round-trip path never adds parts (C3) — the two bugs mask each other.

**C53/C54 — opc writer lifecycle:** `Close()` mutates the *reader's* ContentTypes through the alias (repeated saves observe each other's side effects; concurrent saves race); a mid-Close error leaves a truncated non-zip with no retry and no Abort — and the consumers' error-path `_ = writer.Close()` *finalizes* rather than discards.

**C44 — WritePartRelationships** stands outside every writer invariant (no closed check, no dedupe, no parts registration): double-write → duplicate zip entries, silently.

**C72/C126 — xlsx getters that mutate:** `Styles()` dirties styles.xml (a read breaks byte-identity); `Cell()` creates phantom cells and treats `A01` as distinct from `A1`.

**C60/C78/C118 — swallowed load errors as deferred data loss:** corrupt or parser-rejected parts vanish silently at open; in xlsx a later innocent mutation replaces a parse-failed sheet with an *empty* one. Unreferenced slide parts are dropped by design with no diagnostic.

**C52 — zip bombs:** every part (including CT, parsed before user code) is `io.ReadAll`'d with no size cap.

**C142 — concurrency:** no locks anywhere, mutable shared state during marshal; concurrent use is undefined and undocumented.

### 3.3 Incoherences (names that lie, drift, duplicated truth, dead code)

**Three round-trip strategies for one problem** (see §2 and Design Tension T1): the same question — "which parts get regenerated on save?" — has a different answer in every format, and each answer produces a distinct data-loss class. The `[Content_Types].xml` strategy alone differs three ways (xlsx regenerates from model; pptx/docx preserve raw).

**C83/C84/C141/C121/C144/C146 — dead or decorative API surface:** pptx options/export types consumed by nothing; Theme()/Placeholders() stubs returning nil; a full media getter/setter suite with no serialization; opc's parallel `Part` abstraction; ~25 fabricated p14 transition types; duplicated dml type families (`Blip`/`BlipXML` where the lossy twin is the one production uses). Dead code here is not inert — it advertises capabilities (see §5) and its passing tests manufacture false confidence.

**C109 — two CoreProperties implementations**, one dead and namespace-wrong (docprops), one correct (opc). The dead one's tests enshrine its bugs.

**C111 — the test-path split:** production marshals through the Builder; tests marshal through `encoding/xml`. These differ observably (bool encoding, marshaler dispatch, C106). Every silent-loss bug in this report passes the existing suites; the suites validate a serializer that never ships.

**C147/C124 — naming:** `NSPresentationRels` is the format-agnostic officeDocument relationship namespace, used by all three formats under a pptx-specific name; docx re-exports it as its own unused constant.

**C59 — tri-state modeled as bool:** `SetBold(false)` means "inherit", not "off" — the API vocabulary can't say what WML can.

### 3.4 Affordance mismatches (API promises vs delivery)

**C22/C21/C80/C81 — setters that accept and discard:** TextBox fill/line/shadow; AutoShape gradient/pattern/no-fill (while `Fill.ApplyToSpPr` happily sets them); paragraph spacing; run highlight; table cell borders. All compile, all silently vanish. The easy path (call the obvious setter) is the broken path.

**C79 — `AddPicture(path)`** looks like the primary image API and is a stub returning nil error for nonexistent files; the working path (`pic.SetImage`) is undiscoverable from the method that fails.

**C85 — `AddShape` accepts interfaces it can't serialize** and reports nothing.

**C12 — xlsx workbook-level adds** (`AddDefinedName`, `SetActiveSheet`) succeed as method calls and are dropped at marshal when the original file lacked that element — the *file's history* determines whether the API works.

**C87 — ReplaceText returns nothing**, so partial application (br/fld paragraphs, C87) and non-application (C19, C20) are indistinguishable from success.

**C76 — `ShowDropDown`** cannot do what its name says in either direction.

**C136/C135 — inheritance clobbering by default:** any formatting touch emits `sz="1200"`/`<a:buNone/>`, overriding placeholder/layout inheritance the user never asked to override.

### 3.5 Missing functionality (validation, error channels, cleanup)

- **No error channels where they're needed most:** `AddSheet` (C71), `ReplaceText` (C87), most setters. Silent no-op is the default failure mode across the API.
- **No validation:** sheet names (C71), cell refs (C70), heading levels (C62), merge overlaps (C128), gradient angles (C146), NaN (C67).
- **No cleanup:** RemoveSlide leaves notes parts + CT overrides (C88, C75); no dedup of replacement media (C137).
- **No limits:** decompressed sizes (C52).
- **No Abort on the writer** (C54); no dirty tracking in docx at all.

### 3.6 Boundary & safety

- **C102/C103/C77 — escaping gaps** at the Builder (control chars, namespace URIs) and in xlsx's unknown-element re-encoder (attribute values re-emitted raw — the one place the fidelity machinery itself can emit malformed XML).
- **C119 — part-name validation** loose enough to round-trip zip-slip-shaped names (spine itself never extracts; downstream extractors might).
- **C52 — zip bombs** (above). **C142 — data races** under concurrent use (above).
- Nothing here rises to "exposed secrets/authz" — the library has no network or exec surface; its safety story is bounded to hostile-file handling, where the gaps are memory (C52) and silent structural damage rather than code execution.

### 3.7 Documentation

- **README:** promises byte-identical round-trip "across all formats" (red tests say otherwise, C6-C8); promises gradient fills (C21), placeholders and themes (C84); demonstrates an open-and-modify pptx flow that emits a repair-prompt file (C18); teaches xlsx-only `WriteToBuffer` while the cross-format `SaveBytes` goes unmentioned (C149); never mentions `examples/` or SVG support (C114).
- **Doc comments that lie:** `ReplaceText` "modifies the underlying XML … also updates the materialized shapes" (true only for opened decks, C19); `GraphicData` "preserves unknown content" (never marshaled, C100); xlsx package doc calls the package a placeholder (C116); `pointsToTwipsSigned` claims a difference that doesn't exist (C124); `NumberFormatDate` comment shows the wrong format string (C131); dml constructor comments name the wrong functions (C146); "CT must be last" inverts the spec's guidance (C123).
- **Stale planning doc** at repo root for a shipped feature (C114).

### 3.8 Developer experience

- **C9 — no CI**, red lint, and (on fixture-bearing machines) red tests: a contributor cannot distinguish their breakage from baseline.
- **C8/C112/C115 — irreproducible test inputs:** unfetchable fixtures, an untracked 2.9M python-tests dependency, fetch.sh that hard-fails the whole suite on one dead URL, no checksums. The maintainer and contributors run *different* test suites.
- **C149 — onboarding friction:** go version pin mismatch, unpinned linter with empty config, lifecycle-verb asymmetry across the three packages (matrix: `SaveAs` pptx-only; `WriteToBuffer` xlsx-only; `CreateFromTemplate`/`CreateWithOptions` pptx-only; core verbs Create/Open/OpenReader/Save/SaveBytes/Close are consistent).
- **What's good:** all 8 README examples compile and run as written; the three examples/ programs work and are well-commented; godoc coverage of exported symbols is complete; `go vet` clean; zero dependencies; the spec/spectest conformance harness runs from tracked data out of the box.

---

## 4. Design tensions

**T1 — Three representations, no owner.** pptx keeps domain shapes, oxml models, and raw bytes; sync runs both directions (marshal-time domain→oxml, ReplaceText-time oxml→domain) and each direction destroys the other's unsaved state (C1, C5). docx and xlsx have the same three tiers with different — also partial — sync rules. The alternative worth weighing: **one authoritative representation per part** (the typed model), with raw bytes only as an immutable fallback for untouched parts, and domain wrappers as *views* over the model rather than a parallel store. That's already xlsx's cell architecture — its bugs (C11 aside) are the shallowest of the three formats.

**T2 — Regeneration without lossless capture.** Any part that is ever re-marshaled needs the xlsx-workbook treatment (childOrder + UnknownChildren raw capture + original attrs), or it silently strips content. Today that kit exists in exactly one type. The treadmill alternative — modeling all of ECMA-376 — is unwinnable (this audit found dozens of missing elements after substantial modeling effort). Direction: make lossless-unknown-capture a *required pattern* for every type that participates in regeneration, and assert it with corpus round-trip tests per part type.

**T3 — Create-path and Open-path are two different libraries.** Nearly every public mutator works on Create() documents and fails some way on Open()ed ones (C2, C3, C12, C19, C26). The API surface offers no hint which world an object lives in. Either make the mutators work uniformly (dirty tracking + part writing in round-trip saves), or make the split explicit in the type system (e.g. a read-only `OpenedDocument` that must be explicitly converted). The current design — same methods, silently different semantics — is the worst point in the space.

**T4 — The production serializer is untested by construction.** Tests exercise `encoding/xml`; production exercises the Builder; the two disagree on bool encoding and marshaler dispatch (C106, C111). Either converge on one marshal path, or add an equivalence gate (every oxml type round-trips identically through both) — otherwise every new type is a fresh chance to ship a serializer path no test has ever run.

**T5 — Void-returning mutation API.** The API's ergonomic bet — setters return nothing, adders return objects — removes the channel through which half this report's bugs could have surfaced to users (C71, C87, C85). A v2 worth considering: mutators return errors; a `Validate()` pass before save reports dangling rels, duplicate IDs, schema-required-but-missing content. Several corrupt-output findings (C3, C13, C17) would become caught-at-save errors.

---

## 5. Expectation gaps (expected X, found Y)

| Expected | Found |
|---|---|
| README: "Byte-identical round-trip fidelity for unmodified parts across all formats" | Two xlsx fixtures fail byte-identity today (C6, C7); reading `Styles()` breaks it too (C72) |
| README: auto shapes "with solid/gradient fills" | Gradient/pattern fills silently dropped at save (C21) |
| README: "Support for placeholders and themes" | `Theme()` always nil; master/layout `Placeholders()` are stubs (C84) |
| README open-modify-save pptx example produces a valid file | Added slide has no layout relationship → repair prompt (C18) |
| `doc.AddParagraphWithText` on an opened docx adds a paragraph | Silently dropped (C2) |
| `AddHeading("x", 1)` renders as a heading | No styles.xml is written; Word renders plain text (C64) |
| `ReplaceText` works on any presentation | No-op on created decks; wipes unsaved shapes on opened decks (C19, C5) |
| A returned `*Cell` stays valid | Invalidated by the next `Cell()` call on the row (C11) |
| `Close()` then `Save()` errors or saves equivalently | Silently writes a gutted from-scratch file (C15) |
| `SetBold(false)` un-bolds | Means "inherit" (C59) |
| `xlsx` package doc | "This package is currently a placeholder" over 83 working exported symbols (C116) |
| `ExportFormatPDF`, `SaveOptions.Password`, `OpenOptions.ReadOnly` exist, so the features exist | All consumed by nothing (C83) |
| `make test` works from the docs | Works on fresh clone today, but hard-fails if any fixture URL dies; is red on machines that have the two unfetchable fixtures (C8, C115) |
| Tests passing ⇒ marshal correct | Tests exercise a different serializer than production (C111) |

---

## 6. Open questions (not resolvable from code alone)

1. **Is docx mutation-after-open in scope at all?** The code reads as if docx were designed create-only + preserve-only, and the mutation API grew later without the round-trip plumbing. Whether C2/C3/C26 are "implement the missing half" or "document and error" is a product decision.
2. **Provenance and licensing of `python-tests/` and `spec/gen_spec/`** (python-pptx material, MIT, unattributed): commit with attribution, or ignore and replace the fixture dependencies? Also: where did `fred_data.xlsx`/`abs_australia.xlsx` come from, and can minimized reproductions replace them?
3. **What is the compatibility target?** Byte-identity vs semantic-identity round-trip (C6's fix differs: preserve case vs normalize-and-accept-diff); which Office versions/strict-conformance modes matter (C58, C129).
4. **Is the three-way `[Content_Types].xml` strategy divergence intentional** (xlsx regenerate vs pptx/docx raw-preserve), or historical accident to converge?
5. **Versioning intent:** no tags, no releases, go.mod pinned to a patch version — is the module meant to be consumed externally yet? (Affects how urgently the API-shape findings — T5, C149 — matter, since fixes are breaking changes.)
6. **Concurrency contract:** is single-goroutine use the intended contract (then document it), or should the library be made save-concurrent (then C53/C142 are real work)?
