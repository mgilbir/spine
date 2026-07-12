# Spine Codebase Audit — 2026-07-11

Adversarial full-codebase re-audit of `github.com/mgilbir/spine` at HEAD `9fc882e` (~64.3k LOC of Go outside spec/, zero dependencies). Successor to `codebase-audit-2026-07-07.md` (findings C1–C169, §0 verification at `6d82c1c`) and `docs-audit-2026-07-07.md` (D1–D30). **IDs are continuous across audits**: C1–C169 retain their meaning; this report assigns **C170–C235** to new findings and records HEAD status for every prior ID (§7). A fixing agent can cite any C-number unambiguously.

**Method.** Eight parallel reviewers, one per area (opc; common/xml+oxml+docprops+enum; common/dml+chart+vml+omml+diagram; pptx public; pptx/internal/oxml; docx; xlsx; docs/DX), each read every file in its scope in full. Every prior finding re-verified at HEAD by code trace; every critical/high re-verified by runtime probe (open/create → mutate → save → unzip → inspect, multi-cycle where relevant; ~170 probe programs total). All eight README snippets and all three examples/ programs compiled and run. Candidates that did not survive an attempt at disproof were discarded (e.g. `sldMasterId id="0"` handling — save assigns spec-valid ids; Builder pointer-receiver dispatch on slice elements — works). Baseline at HEAD: `go build ./...` green, `go test ./...` green, `make lint` red with **24** issues (was 20), still **no CI**.

**Headline.** The remediation wave since the last audit (16 commits, PRs #46–#53, every commit citing C-IDs) is the real thing: **19 of the 21 targeted findings verified FIXED at runtime**, including all three criticals (C150/C151/C152), and — unlike the previous wave — the fixes survive adversarial multi-cycle probing (remove→save→remove→save; edit→save→edit→save; media+MoveSlide across both save paths). Only two regressions were introduced (C181 data race in the new decompression budget; C174 table-styling wipe surfaced by the new dirty-flush), versus twenty last time. The docx childOrder-backfill (PR #48) and part-naming (PR #53) fixes were airtight under a full mutator matrix.

The new findings therefore shift character: they are mostly **pre-existing losses in quadrants nobody had probed yet** — group-shape interiors, tracked changes, equations, OLE objects, shared formulas, CR characters — all of the same two families the prior audit named as design tensions: *regeneration without lossless capture* (a part that is always re-marshaled silently strips whatever the typed model doesn't know) and *partial sync between representations*. Notably, pml re-marshals **every slide/layout/master on every save of an opened deck** (no raw-byte preservation for those parts), so every pml modeling gap is live data loss, not latent.

**Severity counts (new findings): 0 critical, 13 high, 22 medium, 31 low (C170–C235).**
Prior findings: of C1–C169, roughly **65 remain open** and **~25 partial** at HEAD (§7); four §0 verdicts from the previous report were found to be wrong and are corrected below (C40, C74, C104, C146).

---

## 0. Verification of the remediation wave (PRs #46–#53)

Every fix commit's diff read; every cited finding re-tested at HEAD with runtime probes for criticals/highs.

**FIXED — verified at runtime (19):** C17 (AddLayout rel-id allocation scans all master rels; survives reopen), C150 (shapeRefs re-indexed after surgical removal — three-cycle remove test passes, group shape survives unrenumbered), C151/C152 (slides get stable part names at creation; media rels survive save→MoveSlide/RemoveSlide→save and pre-save ReplaceText/Duplicate on both save paths), C153/C154/C159 (childOrder backfill before first tracked append — airtight under the full SetText/AddRun/Clear/AddTable matrix at paragraph, run, and cell level, both lifecycles, across save cycles; also fixed C23/C24/C43 as side effects), C155/C161 (added part names derived from existing package parts — gap-filling, 11-image numbering, multi-cycle collision-free; C57 fixed as a side effect), C156 (dirty-flush persists in-place edits to parsed shapes), C157 (worksheet mutators emit XSD-ordered children on sheets that lacked them; ranked-unknown insertion correct; multi-cycle correct), C158 (appended shapes tracked in shapeRefs; add→save→remove→save surgical), C160 (`w:type` prefixed), C162/C167 (no more lnSpc/underline/strike inheritance clobber), C163 (timing tree regenerated when ids change — but see C191/C193 for edges the fix missed), C164 (media data/content-type validated, sniffed, CT-registered), C165 (sync bookkeeping survives ReplaceText re-materialization; held pointers stay live), C166 (AddPicture native-size default).

**PARTIAL (2):**
- **C168** — media dedup is now content-type-aware, but the **image/poster** dedup path (`pptx/image_replace.go:51-57`) is still bytes-only: identical bytes as png and gif share one `image1.png`.
- **C169** — total package cap + per-Reader snapshot added and the arithmetic is correct (declared-size pre-check, limit+1 re-check, charge-once). But the "unsynchronized global" half was documented rather than fixed, the fix introduced a data race (**C181**), and `File.Open` bypasses both caps (**C183**).

**Regressions introduced by the wave (2):** C181 (PR #46's shared budget makes concurrent `ReadAll` a fatal-crash data race — the commit title says "race-free"), C174 (PR #50's dirty-flush turned "edit on a loaded table is dropped" into "edit on a loaded table wipes the table's styling").

**§0-verdict corrections to the 2026-07-07 report:** C40 was marked FIXED but only the gradient/pattern legs were fixed — the shadow leg still emits black for theme colors (now **C179**); C104's Builder error-detection was marked FIXED but has zero production call sites (now **C187**); C146 was marked FIXED but most of the dead/fabricated dml surface remains; conversely **C74** (worksheet root attrs) was marked NOT ADDRESSED but is actually FIXED at HEAD (mc:Ignorable/xr:uid/x14ac:dyDescent survive dirty regeneration — probed).

**Escalations of prior findings:** **C143** low/PLAUSIBLE → **high/CONFIRMED-runtime**: ReplaceText splits runs at *byte* offsets — runs `["αα","x"]` with replacement `αx`→`γ` emit `<a:t>α\xce</a:t><a:t>\xb3</a:t>`: invalid UTF-8, malformed slide XML (pptx/template.go:489-516,446-475). **C88** PLAUSIBLE → CONFIRMED-runtime and aggravated: RemoveSlide leaves an orphan notesSlide whose back-reference then points at the unrelated slide that reuses the freed part name. **C36** and **C32** re-confirmed by runtime probe at HEAD.

---

## 1. Summary table — new findings C170–C235

Severity-ordered. Area key as before: `opc`, `pptx` (public), `pml` (pptx/internal/oxml + pptx/marshal.go), `docx`, `xlsx`, `dml` (common/dml+chart+vml+omml+diagram), `common` (common/xml+oxml+docprops+enum), `dx`.

### High

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C170 | pml | p14/p15 boolean ext attrs typed `*int32` + Open swallows slide parse errors: schema-valid `val="true"` makes the slide silently vanish (Slides() empty, saved deck loses the part and its sldIdLst entry) | pptx/internal/oxml/extension.go:59,69,107; pptx/presentation.go:326-328 | CONFIRMED-runtime |
| C171 | docx | Run-level WML children silently deleted on every regenerated document.xml: `w:delText` (tracked-deletion text irreversibly lost), `w:pict`, `w:object`, `w:commentReference`, `w:ptab` | docx/internal/oxml/run.go:250-254 | CONFIRMED-runtime |
| C172 | pml | `AGraphicData` has no unknown-URI fallback: OLE objects re-emit as empty `<a:graphicData uri=".../ole"/>` on any save — including zero-modification saves | pptx/internal/oxml/table.go:50-55 | CONFIRMED-runtime |
| C173 | docx | oMath/oMathPara re-emitted as **unprefixed** `<oMath>` (WML builder never registers the math namespace): equations destroyed on every open+save; malformed XML when `m:` was declared inline | docx/internal/oxml/paragraph.go:407-418; common/xml/builder.go:657-666,398-404 | CONFIRMED-runtime |
| C174 | pptx | Any cell edit on a **loaded** table regenerates the whole `a:tbl` from the lossy domain model — one `SetText` wipes cell borders/margins/gradients table-wide (regression surfaced by PR #50's dirty-flush; before, the edit was dropped but styling survived) | pptx/shape_sync.go:276-278; pptx/oxml_to_domain.go:493-516 | CONFIRMED-runtime |
| C175 | pptx | Edits to shapes **inside groups** silently dropped: `ShapeByName` recurses into groups and hands out handles that `shapeDirty`/`updateGroupNode` never flush; `GroupShape.AddChild/RemoveChild` also no-ops | pptx/shape_sync.go:29-49; pptx/slide.go:225-237; pptx/shape.go:203-215 | CONFIRMED-runtime |
| C176 | xlsx | Overwriting a shared-formula master cell orphans the whole shared group: followers keep `<f t="shared" si/>` with no master anywhere — ordinary `SetCellValue` yields a spec-invalid, repair-prompt sheet | xlsx/cell.go:161-199 | CONFIRMED-runtime |
| C177 | common | `textEscaper` doesn't escape `\r` in text content; XML EOL normalization silently converts user CRs to `\n` on every save/reopen across docx/xlsx/pptx text | common/xml/builder.go:538-542 | CONFIRMED-runtime |
| C178 | dml | Single-color-choice containers (glow, innerShdw, prstShdw, clrRepl, alphaInv, buClr, tcTxStyle) model only srgbClr+schemeClr: hsl/sys/prst/scrgb colors parse to nothing and re-emit as **schema-invalid empty elements** on the live pptx re-marshal path | common/dml/xml_effect.go:36-192; xml_text.go:443-446; xml_table.go:131-138 | CONFIRMED-runtime |
| C179 | dml | `Shadow.ApplyToSpPr` (public via pptx `SetShadow`) still routes theme colors through `colorToSrgbClr` → literal black; negative Angle yields invalid negative `dir` — the un-fixed shadow leg of C40 (§0 overcredited) | common/dml/effect.go:34; fill.go:111-115 | CONFIRMED-runtime |
| C180 | docx | AddImage on a header/footer paragraph registers the rel on document.xml.rels while the drawing lives in headerN.xml (which gets no .rels part) → dangling `r:embed`, broken image — newly reachable since C3's fix writes header parts | docx/image.go:134-176; headerfooter.go:174-199 | CONFIRMED-runtime |
| C181 | opc | PR #46's shared `decompressionBudget` (unsynchronized `total` + `charged` map) makes concurrent `File.ReadAll` on one Reader a data race / fatal map-crash; this concurrency was safe before the PR whose title claims "race-free" | opc/reader.go:42-59,73-81,109-112 | CONFIRMED-runtime (`-race`) |
| C182 | dx | `docs/` (both audits — the entire C1–C169 record ~20 commit messages cite), `python-tests/` (2.9M unattributed python-pptx suite that schema_test.go depends on), and `spec/gen_spec/` are untracked and not gitignored: the project's documentation, provenance, and remediation record exist on one machine; one `git clean -fd` destroys them | repo root; spec/README.md:119; pptx/schema_test.go | CONFIRMED |

### Medium

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C183 | opc | `File.Open` bypasses both decompression limits and charges no budget — silent unbounded exit next to a bounded `ReadAll` | opc/reader.go:139-141 | CONFIRMED |
| C184 | opc | Foreign-namespace core-property elements matched by local name: `<evil:creator xmlns:evil="urn:evil">` is captured into `cp.Creator` and re-emitted as genuine `<dc:creator>` — metadata laundering | opc/package.go:396-415 | CONFIRMED-runtime |
| C185 | opc | Empty content type + no Default ⇒ part silently absent from [Content_Types].xml; Close succeeds, package OPC-invalid (C45 residual; dead `ErrInvalidContentType` available) | opc/writer.go:88-95 | CONFIRMED-runtime |
| C186 | common | Inline xmlns declarations recorded globally, not lexically scoped: the second sibling subtree in the same namespace emits an unbound prefix (latent: every builder registers more namespaces than its root declares) | common/xml/builder.go:338-395 | CONFIRMED-runtime |
| C187 | common | The C104 balance-check machinery (`Finish()`/`Err()`) has **zero production call sites** — unbalanced Builder output still ships silently into packages (C104 → PARTIAL) | common/xml/builder.go:52-67 | CONFIRMED |
| C188 | common | Scalar field kinds bypass `prependNamespaceDecls`: an element in an undeclared namespace is emitted with no declaration anywhere (struct case prepends; String/Int/Uint/Bool don't) | common/xml/marshal.go:215-229 | CONFIRMED-runtime |
| C189 | dml | Known-URI `a14:imgProps` typed dispatch is lossier than the unknown-URI raw fallback: artistic effects (4 of ~30 children modeled) deleted, empty `<a14:imgEffect>` re-emitted | common/dml/xml_extension.go:135-140,301-308 | CONFIRMED-runtime |
| C190 | dml | `extLst` missing from seven CTs that have it in the XSD (Ln, BodyPr, PPr, RPr, TblPr, TcPr, TableStyle) — any `<a:extLst>` inside them stripped on re-marshal | common/dml/xml_line.go:4-21; xml_text.go; xml_table.go | CONFIRMED-runtime (Ln) |
| C191 | pptx | Removing autoplay media on a loaded slide (surgical path) leaves the generated `p:timing` tree targeting the deleted spid; media rels linger (edge the C163 fix missed) | pptx/media_timing.go:34-56; slide.go:103-129 | CONFIRMED-runtime |
| C192 | pptx | Video/Audio added to a **loaded** slide without SetSize collapses to `<a:ext cx="0" cy="0"/>` after save + any later SetPosition/SetName — the 4×3 default exists only in XML and the dirty-flush writes the domain zeros back (create-path immune) | pptx/media_embed.go:212-215 vs shape_sync.go:160-168 | CONFIRMED-runtime |
| C193 | pptx | `Duplicate()` before the first save loses autoplay on the duplicate: timing is built at save, after Duplicate snapshots the XML; bookkeeping not copied | pptx/slide.go:977-1008 | CONFIRMED-runtime |
| C194 | pptx | `SetAnchor`/`SetWordWrap`/`SetMargins` on a parsed shape rewrite `<a:bodyPr/>` with explicit `lIns="0" tIns="0" rIns="0" bIns="0"` — inherited default insets replaced by zeros (same clobbering class PR #52 fixed for lnSpc/u/strike) | pptx/shape_sync.go:195-207; oxml_to_domain.go:541-570 | CONFIRMED-runtime |
| C195 | pml | `P14Media` keeps only `r:embed`; `p14:trim`/`p14:fade`/`p14:bmkLst` children deleted — video trim points silently reset on save | pptx/internal/oxml/extension.go:53-55,143-152,245-247 | CONFIRMED-runtime |
| C196 | pml | Full shape-tree rebuild renumbers everything from 2 but never clears or renumbers `CxnSp` → duplicate `cNvPr` ids (ST_DrawingElementId uniqueness violated); also asymmetric: groups dropped, connectors kept | pptx/slide.go:330-345 | CONFIRMED-runtime |
| C197 | docx | `AddHeader(HeaderEven)`/`AddFooter(FooterEven)` never set `w:evenAndOddHeaders` in settings.xml (created docs have no settings.xml at all) — the even header silently never renders | docx/headerfooter.go:95-98,145-148 | CONFIRMED-runtime |
| C198 | xlsx | Stale calcChain.xml preserved after formulas are removed — references cells with no `<f>`; known Excel repair-prompt class | xlsx/workbook.go:378-400; cell.go setters | CONFIRMED-runtime (staleness) |
| C199 | xlsx | styles.xml regeneration drops `mc:Ignorable` and demotes `x14ac:knownFonts` to unprefixed `knownFonts` (CT_Stylesheet lacks the OriginalRootAttrs kit that fixed the same class for worksheets) | xlsx/internal/oxml/styles.go:24-43,137-139; xlsx/marshal.go:145-196 | CONFIRMED-runtime |
| C200 | xlsx | `AddImage` guards on `reader != nil` instead of the durable `opened` flag: Open→Close→AddImage silently drops the image — directly contradicting AddImage's own doc comment (C15-fix follow-through miss) | xlsx/image.go:63-66 vs workbook.go:295-330 | CONFIRMED-runtime |
| C201 | xlsx | Unknown-child re-encoding resolves prefixes from root attrs only: an element with its own inline `xmlns:foo` re-emits with element/attr prefixes stripped — content silently moves into the default sml namespace (xlsx sibling of C36) | xlsx/internal/oxml/workbook.go:222-268 | CONFIRMED-runtime |
| C202 | dx | Lint debt grew 20→24 with no gate; four of the new issues were introduced by the remediation wave itself; `.golangci.yml` is 13 bytes with no linter config and an undocumented v2-binary requirement (extends C9) | .golangci.yml; Makefile:9 | CONFIRMED-runtime |
| C203 | dx | ~20 competing package comments in common/dml — `go doc` surfaces one arbitrarily (one candidate calls the production package "lightweight PML shape types for spec test validation") | common/dml/*.go | CONFIRMED |
| C204 | dx | `oldMiddleEnd = oldMiddleStart` clamp in ReplaceText's overlap guard is assigned but never read (ineffassign) — dead code or a live splice bug adjacent to the C143 byte-splitting family; invisible because lint is red-baseline | pptx/template.go:426 | PLAUSIBLE |

### Low

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C205 | opc | MarshalRelationships escapes attrs with `xml.EscapeText` contra the project policy (`'`→`&#39;` etc.) — byte drift on regenerated .rels with quotes in Targets | opc/relationship.go:141-145 | CONFIRMED |
| C206 | opc | WriteRawFile writes the entry name verbatim: a leading-slash name yields an absolute-path zip entry | opc/writer.go:114-142 | CONFIRMED-runtime |
| C207 | opc | AddRelationship after Close silently accepted and never written → dangling r:id | opc/writer.go:145-155 | CONFIRMED-runtime |
| C208 | opc | ResolvePartName does no percent-decoding or fragment stripping of rel-target URIs — `Some%20Image.png` treated as missing part | opc/part.go:63-74 | PLAUSIBLE |
| C209 | opc | Present-but-empty date elements dropped on core.xml re-marshal (inconsistent with the empty-string-element branch) | opc/package.go:101-117,198-203 | CONFIRMED |
| C210 | opc | testutil.ReadZipParts collapses duplicate zip entries (map, last-wins) — round-trip tests blind to the duplicate-entry defect class C44 guarded against | internal/testutil/roundtrip.go:22-38 | CONFIRMED |
| C211 | opc | Case-variant duplicate overrides resolved nondeterministically (map iteration); first-wins for case-variant [Content_Types].xml entries | opc/content_types.go:117-121; reader.go:218-232 | CONFIRMED |
| C212 | common | Nil pointer attr without omitempty → `attr=""` where encoding/xml omits (latent serializer divergence) | common/xml/marshal.go:251-260 | CONFIRMED-runtime |
| C213 | common | `stripInvalidXMLChars` filters only C0 bytes: U+FFFE/U+FFFF (and invalid UTF-8) still yield unparseable parts | common/xml/builder.go:563-595 | CONFIRMED-runtime |
| C214 | common | Builder balance check compares local names only — `<p:sp></a:sp>` passes `Finish()` | common/xml/builder.go:33-48 | CONFIRMED-runtime |
| C215 | common | AlternateContent marshal resets `declaredNamespaces` for namespaces it did not declare — later elements re-declare inline; byte drift + desynced IsNamespaceDeclared | common/oxml/alternate_content.go:91-133 | CONFIRMED |
| C216 | common | AlternateContent unmarshal matches Choice/Fallback by local name in any namespace | common/oxml/alternate_content.go:44-60 | CONFIRMED |
| C217 | dml | `GradFill.RotWithShape`/`Lin.Scaled` plain bool+omitempty with no XSD default — explicit `"0"` deleted | common/dml/xml_types.go:368,396 | PLAUSIBLE |
| C218 | dml | `Fld.Id` (XSD required GUID) is omitempty — API-constructed fields emit schema-invalid `<a:fld>` | common/dml/xml_text.go:377 | CONFIRMED |
| C219 | dml | `CNvGraphicFramePr` typed `*CNvGrpSpPr`; real child `a:graphicFrameLocks` parses to nothing and is dropped (no type exists for it) | common/dml/xml_wpdrawing.go:30,44 | CONFIRMED (latent) |
| C220 | pptx | `SetPlayMode`/`SetPoster` after the shape is synced are silent no-ops (no dirty flag; flush can't apply them) — works on created decks, another lifecycle asymmetry | pptx/media.go:236-257 | CONFIRMED |
| C221 | pptx | RemoveShape leaves the removed shape's media/image rels and parts in the package forever | pptx/slide.go:103-129 | CONFIRMED |
| C222 | pml | Decks without `defaultTextStyle` gain a fabricated 9-level one on every save (hard-coded fonts/margins/sz=1800) — invented document defaults | pptx/marshal.go:180-184,399-447 | CONFIRMED |
| C223 | pml | Single `AlternateContent` pointer per slide root: multiple root-level AC siblings collapse to the last, re-emitted at a fixed position | pptx/internal/oxml/slide.go:22,40,54 | PLAUSIBLE |
| C224 | pml | `CommonTimeNode.Display`/`AutoRev`/`AfterEffect`/`NodePh` plain bool+omitempty on the always-remarshal timing path — explicit `="0"` dropped | pptx/internal/oxml/animation.go:362-374 | PLAUSIBLE |
| C225 | pml | `sldMasterId`/`sldId`/`sldLayoutId` UnmarshalXML skips the optional `extLst` child; PhotoAlbum also lacks extLst | pptx/internal/oxml/presentation.go:177-234; slide.go:85-101 | CONFIRMED |
| C226 | docx | Repeated AddHeader/AddFooter of the same type appends duplicate same-type references to one sectPr | docx/headerfooter.go:90-93,140-143 | CONFIRMED-runtime |
| C227 | docx | `AddSectionBreak` attaches the old sectPr to the last element of `body.P`, not the last body child — wrong boundary when the doc ends with a table/SDT | docx/document.go:746-755 | PLAUSIBLE |
| C228 | docx | `Body()`/`Paragraphs()` omit body-level-SDT paragraphs (read API under-reports; round-trip unaffected); `marshalHdrFtrContent` ignores childOrder | docx/document.go:519-528; headerfooter.go:214-222 | CONFIRMED |
| C229 | docx | `nextImageNumber` prefix match is case-sensitive while OPC part names are case-insensitive — `IMAGE1.PNG` in a package makes AddImage pick a colliding name and fail Save | docx/document.go:651-668 | PLAUSIBLE |
| C230 | xlsx | SetRowHeight bypasses `rowNumberOf` (C73 fix incomplete): duplicate row emitted for r-less rows | xlsx/sheet.go:233-238 | CONFIRMED-runtime |
| C231 | xlsx | Non-standard main-part name: save emits both the stale original workbook part (edits lost) and an orphan regenerated /xl/workbook.xml (xlsx analog of docx C61) | xlsx/workbook.go:66 vs 376-457 | CONFIRMED-runtime |
| C232 | xlsx | `xfEqual` ignores QuotePrefix/PivotButton/Protection: NewCellStyle can dedupe onto a parsed xf carrying locked/hidden protection | xlsx/style.go:570-582 | PLAUSIBLE |
| C233 | dx | README Testing section documents none of: best-effort fetch/`--strict`, silent test skips, four permanently unfetchable fixtures, the python-tests tranche — contributor suite ≠ maintainer suite, both green | README.md:303-325 | CONFIRMED |
| C234 | dx | No tags, no releases, no CHANGELOG; README `go get` resolves a moving pseudo-version of main | repo | CONFIRMED |
| C235 | dx | Makefile has only fetch/test/lint — no build/vet target, no strict-fetch target for CI; `--strict` undocumented | Makefile | CONFIRMED |

---

## 2. System map (updated)

### Layering (as before, with the new machinery)

1. **`opc/`** — zip layer. New since last audit: `decompressionBudget` per Reader (total-package cap + per-part cap, snapshot of two mutable package globals). The skip-if-already-written contract on `Close()` is unchanged and still load-bearing/double-edged (C10, C46).
2. **`common/xml`** — the Builder, production serializer for every regenerated part. Now has balance-check machinery (`Err`/`Finish`) — which nothing calls (C187). Escaping is context-aware and spec-correct except `\r` in text (C177).
3. **`common/*` models** — unchanged since 6d82c1c (zero commits). Only pptx consumes dml on a live path; chart/diagram/vml/omml remain spec-test-only; docprops remains dead (C109).
4. **`{pptx,docx,xlsx}/internal/oxml`** — typed models. Fidelity-kit maturity is still wildly uneven: xlsx CT_Workbook and (new) CT_Worksheet have the full kit; xlsx CT_Stylesheet (C199), all of docx run-level (C171), and all of pml (C172, C195, C32, C33, C36) lack unknown-capture.
5. **Public packages** — pptx's third representation (domain shapes) now syncs via **per-shape dirty flags + shapeRefs bookkeeping** (shape_sync.go, new) instead of clear-and-rebuild in the common cases. The bookkeeping now survives multiple save cycles (C150/C156/C158/C165 all verified fixed). Where it still falls back to full rebuild (removing never-synced shapes, AddShape of unsupported types) the old lossy behavior returns (C196, C85).

### Real execution paths (revised where changed)

- **pptx save on an opened deck** re-marshals every slide, layout, master, and presentation.xml from typed models — **no raw preservation for these parts** (probed: zero-modification save rewrites slide XML). presProps/viewProps/tableStyles/notesSlides are raw-preserved. Consequence: every pml modeling gap is live on every save (C172, C178, C190, C195, C32, C33, C34, C92).
- **Slides now get stable part names at AddSlide** (`nextAvailableSlidePartName`), and `saveNew` no longer re-derives names by index — the fix that closed C151/C152.
- **docx** now writes new parts (images/headers/footers) on the round-trip path and regenerates [Content_Types].xml when parts were added (C3 closed); document.xml is still always regenerated, so every model gap in WML types is live (C171, C27, C28).
- **xlsx** worksheet marshal now inserts new child kinds at the XSD-correct position via a ranked schema sequence (C157 closed); **workbook.xml marshal is still childOrder-gated with no such kit** — C12 is precisely "works iff the original file already had that element".

### Key invariants and where they hold (updated)

- "Byte-identical round-trip for unmodified parts": now holds across the full available corpus including the two formerly-red xlsx fixtures (C6/C7 fixed) — but fresh clones still can't verify 4 fixtures (C8) — and does **not** hold for pptx slides in the strict sense (they are re-marshaled; equality holds only for content the model captures).
- "Sync bookkeeping survives save cycles": now true for top-level shapes on slides (verified through three-cycle probes); **false inside groups** (C175) and for tables (C174).
- "Mutations on opened documents persist": now true for docx body content and xlsx sheet-level mutators; still false for Properties (C10), xlsx workbook-level elements absent from the original (C12), pptx group interiors (C175), media property setters post-sync (C220).
- "Errors surface": still violated at every load path (opc C120, xlsx C60/C78, pptx C170's `continue` — the most expensive single instance found this round).

---

## 3. Findings by category (new findings; severity order within category)

### 3.1 Correctness — silent data loss & wrong output

**C170 — a valid attribute value deletes a slide.** `val="true"` is legal xsd:boolean lexical space; PowerPoint usually writes `1`, other producers write `true`. Parse of the p14/p15 extension fails (`*int32`), `Open`'s slide loop does `continue`, and the deck opens "successfully" minus one slide; the next save makes the loss permanent (no part, no sldId). Two independent defects compound: wrong type, swallowed error. Fix both — the swallow converts every future slide-level parse bug into silent data destruction.

**C171 — the tracked-changes trap.** `w:delText` is how WML stores *deleted-but-tracked* text. Because run marshal has no case for it (nor pict/object/commentReference/ptab) and document.xml is always regenerated, opening and saving any document with tracked deletions destroys the deleted text — rejecting the change in Word can no longer restore it. This is the run-level sibling of C28 and arguably the single worst data-loss scenario in docx today.

**C172 / C195 / C178 / C190 / C189 — the "regeneration strips what the model doesn't know" family, pptx edition.** OLE objects (C172), video trim/fade points (C195), non-srgb/scheme colors in seven effect containers (C178 — schema-invalid output, not just loss), `a:extLst` in seven dml CTs (C190), and artistic image effects (C189) are all silently deleted by a zero-modification open+save of a pptx. All are CONFIRMED-runtime. The common direction: every type on the always-remarshal path needs either full modeling or a raw-capture fallback; C189 adds the sharper invariant that *typed dispatch must never lose what the default raw branch would have preserved*.

**C174 — the dirty-flush's blind spot regenerates tables.** The new sync flushes most shape edits surgically, but a Table edit routes through `buildTableGraphicFrame` — a full regeneration from the domain model, which doesn't carry cell borders/margins/fills. One cell's `SetText` on a loaded deck wipes the styling of the whole table. Regression relative to the (also wrong) pre-PR-#50 behavior of dropping the edit. Direction: patch parsed `ATc` nodes in place, as the text path now does.

**C176 — shared formulas.** `SetCellValue` on a master cell nils its `<f t="shared" ref>` while followers keep `si`-only stubs → spec-invalid workbook from an ordinary write to one cell. The API gives no hint the cell was load-bearing for others.

**C177 — `\r` normalization.** Any user text containing CR round-trips as LF everywhere (all three formats). One replacer entry (`"\r"`→`"&#xD;"`) fixes it; note MEMORY.md's escaping guidance enshrines the bug as design.

**C179 — theme shadows are black.** The public `SetShadow` degrades any theme color to `srgbClr 000000`. §0 marked C40 fixed; only the gradient/pattern legs were.

**C196, C209, C213, C217, C218, C219, C222, C224, C225, C230 (low/medium tail)** — duplicate shape ids via the CxnSp asymmetry in full rebuilds; empty core-props dates dropped; U+FFFF passes the char filter; two no-default bools; required `Fld.Id` omitted; fabricated defaultTextStyle injected into decks that had none; timing display flags; sldId extLst; r-less row duplication. Each has a one-line fix noted in §1.

### 3.2 Unintended paths — lifecycle, concurrency, second calls

**C181 — the "race-free" commit added a race.** Concurrent `ReadAll` on distinct parts of one opened Reader — a natural pattern for a read-only object, and safe before PR #46 — now races on the budget map (`-race` confirmed; fatal `concurrent map writes` possible). One mutex fixes it. **C183** is the same feature's other edge: `File.Open` is an unbounded, unbudgeted bypass sitting next to the carefully-capped `ReadAll`.

**C175 — groups are a read-write trap.** `ShapeByName` happily recurses into groups (so the API *affords* editing group children), but no sync path flushes those edits, and `GroupShape.AddChild/RemoveChild` are silent no-ops. Either flush per-child refs or make group interiors explicitly read-only.

**C191/C192/C193/C220 — media lifecycle edges around the C163/C164 fixes.** Removing autoplay media leaves a timing tree pointing at a dead spid; media added to a *loaded* slide silently becomes 0×0 if any later setter runs; `Duplicate()` before first save yields a non-autoplaying duplicate; `SetPlayMode`/`SetPoster` post-sync are no-ops. All are create-vs-open or order-of-operations asymmetries in the new media machinery.

**C200 — Open→Close→AddImage.** The C15 fix made Close-then-Save a supported flow; AddImage's guard still tests `reader != nil` and silently drops the image — contradicting its own doc comment.

**C207, C211, C221, C226, C227, C229, C231 (tail)** — post-Close rel writes accepted; nondeterministic case-variant CT resolution; leaked media parts on RemoveShape; duplicate header references; sectPr anchored to the wrong element; case-sensitive image numbering vs case-insensitive OPC; non-standard workbook part names silently forking into two workbook parts.

### 3.3 Incoherences

**C187 — a safety mechanism with no callers.** The Builder can now detect unbalanced output, and no production marshal consults it; §0 counted C104 as fixed. Mechanism ≠ closed failure mode. Wire `Finish()` into every `marshal*` entry point and surface through Save.

**C184 — two truths about core properties.** The parser maps unknown-namespace elements onto dc/cp fields by local name, then Marshal re-labels them into the standard namespaces — the library actively rewrites provenance it doesn't understand (worse than C48's plain dropping).

**C203 — ~20 competing package comments in common/dml**; which one is "the" package doc depends on filename sort. **C205** — opc .rels escapes attributes through `xml.EscapeText` while everything else uses `EscapeAttrValue`: two escaping policies in one writer. **C210** — the round-trip test helper is structurally blind to duplicate zip entries, the exact class opc's writer defends against. **C232** — style dedup compares a subset of xf fields and can silently adopt protection flags.

### 3.4 Affordance mismatches

**C185** — `WritePart` with an empty content type succeeds all the way through Close and produces an OPC-invalid package; the error variable for exactly this exists and is never returned. **C197** — `AddHeader(HeaderEven)` compiles, saves, and never renders (missing `evenAndOddHeaders` setting); the API surface implies even/odd support it only half-delivers. **C228** — `Paragraphs()` under-reports content that round-trips fine (SDT-wrapped paragraphs), so read-modify-write code operates on a subset without knowing. **C204** — ReplaceText's overlap clamp assigns a variable nothing reads; either dead code to delete or a real splice bug hiding behind the red lint baseline (the C143 escalation shows this exact code path already emits invalid UTF-8; treat the whole prefix/suffix splice as suspect and rewrite rune-safe).

### 3.5 Missing functionality

The void-returning mutator pattern (T5) persists unchanged: `SetName` on sheets accepts invalid names AddSheet would reject (C71 residual), MergeCells accepts garbage (C128), zero-sheet workbooks save without error (C130). New instances this round: media property setters that can't report "too late" (C220), AddImage that can't report "closed" (C200), WritePart that can't report "no content type" (C185).

### 3.6 Boundary & safety

The hostile-file surface improved (per-part + total decompression caps with correct arithmetic; part-name traversal still blocked), but the new budget is the concurrency hazard (C181) and `File.Open` the bypass (C183). C184 is the one place hostile input rewrites rather than merely survives. C186/C188/C201 form a namespace-scoping family where the Builder or re-encoders can emit unbound prefixes or silently re-namespace content — all producing malformed or semantically altered XML from valid input. C213: char-level validity filtering stops at C0 controls.

### 3.7 Documentation

Carried from the docs audit, still true: the Create-vs-Open behavioral split is documented nowhere (D9) while it remains the top silent-loss source (C10, C12); README teaches `dml.Points(12)` for a plain-points API (D7 — a 152,400pt font if followed); AddHeading produces documents whose styles don't exist (D8); shipped features (SVG, SaveBytes, examples/) undocumented while unshipped ones (themes, placeholders, passwords — C83/C84/D25) are advertised; `SVG_PPTX_EMBEDDING_PLAN.md` still opens the repo with a stale "what is missing" for a shipped feature (C114). New: **C233** (the Testing docs don't say the suite silently thins itself on machines without fixtures).

### 3.8 Developer experience

**C182 is the structural one**: the audit record that ~20 commit messages cite, the python-pptx-derived test corpus that schema_test.go consumes, and the codegen tooling documented in spec/README.md are all untracked. The project's institutional memory is one `git clean` from gone, and no contributor can follow the remediation trail. **C202**: lint debt grew during the remediation wave because nothing gates it; the config doesn't even pin the required linter major version. **C234/C235**: no tags/releases/changelog; Makefile lacks build/vet/strict-fetch. C9 (no CI) remains the root cause under all of these — verified worse, not better, since the last audit.

---

## 4. Design tensions

**T1 — Three representations, one owner *emerging*.** The prior audit's headline tension — domain shapes ↔ typed oxml ↔ raw bytes, mutually destructive syncs — has genuinely narrowed: the new dirty-flag + shapeRefs machinery survives multi-cycle probing, and re-materialization now adopts instead of orphaning. What remains is the *frontier* of the synced set: group interiors (C175), tables (C174), media property setters (C220), and the full-rebuild fallback (C196). The pattern of the residual bugs is consistent: **every shape family that lacks a surgical patch path falls back to regeneration from a model that can't represent it.** The endgame the codebase is implicitly walking toward — domain wrappers as views over the parsed tree, no rebuild path at all — should be named and finished rather than approached one finding at a time.

**T2 — Regeneration without lossless capture is now the dominant source of highs.** 9 of the 13 new highs (C171, C172, C173, C176, C177, C178, C179, and the C190/C195 mediums behind them) are this one tension. The lossless kit (childOrder + unknown-capture + original attrs + inline-ns capture) exists and is proven in exactly two types (xlsx CT_Workbook, CT_Worksheet). pml — where **every slide/layout/master is re-marshaled on every save** — has none of it, and docx run-level has none of it. Either port the kit to every type on an always-remarshal path, or stop always-remarshaling (dirty-part tracking would let untouched slides ship as raw bytes, which would eliminate the entire zero-modification-loss class in one move).

**T3 — Create-vs-Open: narrowed in docx, alive elsewhere.** PR #48/#53 made docx body mutation genuinely uniform across lifecycles — the first format to get there. xlsx workbook-level (C12: "the file's history determines whether the API works" — literally, the element must pre-exist), pptx media (C192/C220), Properties everywhere (C10), and xlsx AddImage-after-Close (C200) still make the same method mean different things depending on how the object was born. The docx work proves the split is fixable; it's now a coverage problem, not a design mystery.

**T4 — The production serializer is still tested by exception.** spec_test.go (pml) and the new common/xml robustness tests are the only suites exercising the Builder; 209 encoding/xml assertions in pml tests and 10+ files elsewhere validate a serializer that never ships. C177 (the `\r` bug) lived in the Builder's escaper for the project's whole life precisely because no test round-trips *through the production path and back*. An equivalence gate (every oxml type: Builder output parses back equal, and matches encoding/xml where intended) is still the cheapest systemic fix on the table.

**T5 — Void-returning mutators.** Unchanged, and this round added instances rather than subtracting them (C185, C200, C220). Worth restating the concrete v2 shape: mutators return errors; `Validate()` before save reports dangling rels, duplicate ids, orphaned shared formulas, missing content types. C170, C176, C180, C185 would all become caught-at-save errors.

**T6 — The remediation process itself (new).** Fix-by-finding-ID produces exactly what it targets: 19/21 fixed, verified — and the same disease untouched one type away (C157 fixed for CT_Worksheet, C12 still open in CT_Workbook via the identical mechanism; C167 fixed for underline/strike, C194 the same clobber for insets; C168 fixed for media, open for images). Meanwhile the audit trail driving all of it is untracked (C182) and the lint/CI substrate that would hold fixes steady is absent (C9/C202). The alternative worth weighing: after each finding, spend the marginal hour asking "where else does this exact pattern occur?" — this audit's grep-level sweeps (C178's seven containers, C190's seven CTs) are what that looks like, and they were cheap.

---

## 5. Expectation gaps (expected X, found Y)

| Expected | Found |
|---|---|
| Open+Save with zero modifications is lossless (README fidelity claim) | pptx: OLE objects (C172), video trim points (C195), non-srgb effect colors (C178), spTree AlternateContent (C32), ActiveX (C33) all deleted; docx: tracked-deletion text, VML, OLE refs deleted (C171); equations destroyed (C173) |
| A schema-valid file opens with all its slides | `val="true"` in a p14 extension silently deletes the slide (C170) |
| `ShapeByName` finding a shape inside a group means you can edit it | Edits to group children are silently dropped (C175) |
| Editing one table cell touches one table cell | Wipes borders/margins/fills of the whole table on loaded decks (C174) |
| `SetCellValue` on a cell affects that cell | Orphans every follower of a shared-formula group (C176) |
| Text I write is the text I read back | `\r` becomes `\n` on every round-trip (C177) |
| A commit titled "make limits race-free" removes races | It introduced the package's only known data race (C181) |
| `AddHeader(HeaderEven)` produces an even-page header | Never renders; the activating setting is never written (C197) |
| AddImage returning nil error means the image is in the file | Not after Close (C200), not in headers (C180 — dangling rel instead) |
| Commit messages citing C-IDs are traceable | The cited documents are untracked, local-only files (C182) |
| `make lint` reflects my changes | 24 pre-existing failures, 4 added by the fix wave itself (C202) |
| The Builder's new `Finish()` error check protects output | Zero production callers (C187) |

---

## 6. Open questions (not resolvable from code alone)

1. **Is always-remarshal for pptx slides a policy or an accident?** T2's cheapest fix (raw-preserve untouched slides) contradicts it; everything in C172/C178/C190/C195 hangs on this choice.
2. **Provenance/licensing of `python-tests/` and `spec/gen_spec/`** (python-pptx material, MIT, unattributed) — now compounded by C182: commit with attribution, slim to the 18 fixtures actually used, or delete and de-reference?
3. **Should the audit record be tracked?** Commit messages already treat C-IDs as the project's issue tracker; either commit docs/audits/ or move findings to a tracked system.
4. **Concurrency contract for opc:** the format packages now document single-goroutine use, but a read-only opened `opc.Reader` is a plausible concurrent object and C181 makes it crash-prone. Is concurrent read intended?
5. **Tracked-changes support level:** C171 makes open+save destructive for revision-bearing documents. Is "preserve tracked changes" in scope (model delText et al.) or should Open refuse/warn on revision marks it can't carry?
6. **Versioning:** still no tags. At what point does the API surface (T5 redesign, C149 verb asymmetries) get frozen by external consumers?

---

## 7. Prior-finding status rollup at HEAD (C1–C169)

Statuses re-verified this audit (code trace at minimum; runtime probes for criticals/highs and everything marked "probed" in the area reports). One-liners only for OPEN/PARTIAL; FIXED IDs listed bare.

**opc** — FIXED: C6, C7, C44, C47, C49, C50, C52, C122. PARTIAL: C45 (residual → C185), C120 (root element presence checked, name/DecodeElement errors still ignored), C121 (three dead error vars remain), C169 (→ C181/C183). OPEN: C8 (fixtures still commented out of external.txt; sting reduced — nothing red is currently masked), C10 (Properties-after-Open silently dropped, docx+xlsx, re-probed), C46 (raw-CT freeze; docx path defused by C3 fix, mechanism intact), C48 (unknown core-props children dropped), C51 (non-canonical zip entry names unreachable/droppable), C53 (ContentTypes aliasing reader↔writer), C54 (Close finalizes on error path; no Abort), C55 (app.xml hardcoded zeros/falses), C119 (part-name validation gaps: backslash/space/%/controls), C123 (writer lifecycle traps).

**common/xml + oxml + docprops + enum** — FIXED: C41, C102, C105. PARTIAL: C104 (→ C187), C111 (Builder now tested here; everywhere else unchanged). OPEN: C103 (StartElementInlineNS still writes the URI raw, builder.go:454), C106 (xml.Marshaler ignored), C109 (docprops dead+wrong), C147 (itoa MinInt64 → "-", writeQName silent-unprefixed, RelsPathToSourcePart mangling, ExtensionPrefixToNS gaps, dead constants, NSPresentationRels misname — all re-verified), C148 (FontStyleBold bit-0 dead; UnderlineStyle 12/18; TextAlign gaps; vml MiniGo/FaceAt/SignatureLine mismaps).

**dml / chart / vml / omml / diagram** (zero commits since 6d82c1c; all re-verified) — FIXED: C37, C39 (for Gs/OuterShdw only — class not propagated → C178), C94, C96, C97, C98, C145. PARTIAL: C29 (dml legs done; vml presence flags latent), C38 (duotone cross-kind order still inverts — probed), C40 (**§0 overcredited** — shadow leg → C179), C99 (WithAlpha(0) still opaque — probed), C146 (**§0 overcredited** — Blip/Tile/RelRect duplicates, fabricated types, Wsp wrong-ns children, negative-angle gradients, misnamed constructor docs all remain). OPEN: C31 (chart extLst = dml.ExtLst → chart extensions deleted, probed), C36 (unknown-ext inline xmlns lost → unbound prefixes, probed), C42 (OMath broken both directions, probed), C93 (BlipXML no effect children — live via slide backgrounds), C95 (17/28 color transforms; bogus val="0" on empty transforms, probed), C100, C101, C107 (vml textbox content mangled, probed), C108, C110.

**pptx public** — FIXED (re-verified, runtime for the starred): C1* (headline; residuals split out as C174/C175), C5*, C16*, C17*, C18*, C19, C20*, C21, C22, C79*, C80, C81*, C82, C86, C135, C136, C137, C138*, C140 (mostly — AdvanceOnClick representable), C142 (resolved by documented contract), C150*, C151*, C152*, C156*, C158*, C162*, C163* (edges → C191/C193), C164*, C165*, C166*, C167*; C141 closed-superseded. PARTIAL: C4 (all attrs emitted; custShow still emits schema-invalid empty element — probed), C29-pptx (AdvClick done; AdvTm live gap), C168 (image path → see §0). OPEN: C30/C31 (pml extLst types), C32 (spTree AlternateContent/contentPart deleted — re-probed), C34 (graphicEl/bldSub/progress wrong ns+types), C83 (CreateWithOptions ignores its options; dead option types), C84 (Theme/Placeholders stubs), C85 (AddShape accepts what it can't serialize — incl. GroupShape silent no-op), C87 (br/fld paragraphs revert replacements), C88 (**escalated** CONFIRMED-runtime: orphan notesSlide back-references the wrong slide after part-name reuse), C118 (unreferenced slide parts dropped), C139 (4:3 vs layout mismatch), C143 (**escalated** high: byte-offset run splitting emits invalid UTF-8).

**pml (pptx/internal/oxml)** — FIXED: C35 (holds), C150-internal machinery (verified). PARTIAL: C33 (roots done; Slide flags, custDataLst/controls/ActiveX, Shape.useBgFill, Picture/CxnSp extLst, GrpSpPr fills all still deleted — live), C144 (p14 fabrications gone; comments/tags/SlideProperties scaffolding remains), C89 (GridSpacing x/y vs cx/cy — latent). OPEN: C34, C36, C90, C91 (presentation.xml writer drops srgb solid fills; Tint/Shade as child tags), C92 (table model: no rtl/fills/tableStyle/cell gradients/diagonals/id; noChangeAspect deleted), C93, C106/C111-pml (209 encoding/xml calls, 0 Builder in oxml tests).

**docx** — FIXED (all probed): C2, C3, C23, C24, C41, C43, C56, C57, C58, C59, C62, C153, C154, C155, C159, C160, C161. PARTIAL: C46-docx (defused), C63 (SetText still leaves stale hyperlink/SDT text), C124 (unreachable r:id branches remain). OPEN: C10, C25 (second image in a run overwrites the first's drawing — probed), C26 (numbering.xml never written on opened docs — probed), C27 (eight sectPr children stripped — probed), C28 (altChunk/customXml/smartTag/moveTo/tblPrEx dropped; moveTo = moved text lost — probed), C60, C61 (non-standard main-part name forks the document — probed), C64 (AddHeading with no styles.xml — probed), C65 (cell AddImage errors on nil backref — probed), C125.

**xlsx** — FIXED (probed where starred): C11*, C13*, C14, C15*, C66, C67, C68, C69, C70, C72*, C74 (**§0 stale** — actually fixed), C77, C157*. PARTIAL: C12 (**reconciled**: AddDefinedName/SetActiveSheet persist iff the original file already had the element — childOrder-gated; no CT_Workbook EnsureChildOrder analog), C71 (AddSheet validates; SetName doesn't — probed), C73 (→ C230), C75 (part removed from preserved set; orphan CT override + orphan sheet .rels remain — probed), C116 (ErrInvalidRange still dead), C126 (case normalized; A01 still forks a second cell — probed). OPEN: C76 (ShowDropDown inverted/inert — probed), C78 (corrupt sheet silently replaced by fabricated empty sheet after one edit — probed), C117 (stale dimension — probed), C127 (overlapping col entries — probed), C128 (garbage merge refs accepted — probed), C129, C130 (zero-sheet save — probed), C131, C132, C133 (FreezePanes half-built panes — probed), C134.

**dx / docs** — RESOLVED: C115, D1, D2, D3, D5 (mostly), D6, C58/D17-part, D19-part, D22 (format packages). PARTIAL: C8, C111, C149, D11, D12, D23. OPEN/PERSIST: C9 (**worse** — 24 lint issues), C112, C113, C114, D4, D7, D8, D9 (**the most dangerous doc omission**: the Create-vs-Open split is still documented nowhere), D13, D14, D15, D18, D20, D21, D24, D25, D26–D28, D29, D30.

---

## 8. What held up under adversarial probing

Recorded so future audits don't re-litigate: the C150 reindex algorithm is correct for multi-kind, multi-removal, sentinel-bearing batches and composes with the placeholder-image remap; stable slide part-naming holds across save→move→save→reopen→save; the docx childOrder backfill could not be broken by any mutator combination tried (~50 probes); the C157 worksheet schema-sequence insertion is correct at both edges and across cycles, and its tests probe their own edge cases; PR #46's cap arithmetic defeats lying zip headers (single-threaded); C6/C7 byte-identity holds on the full available corpus; zero-modification docx round-trip is part-content-identical including SDTs, AlternateContent, w:ins, and hyperlink attributes; all eight README snippets and all three examples compile and run; the deterministic-output property (byte-identical parts across process runs) held everywhere it was checked. Disproved this round and discarded: sldMasterId zero-id emission, Builder pointer-receiver dispatch on slice elements, r:id prefix resolution in reflection marshal, `writeIndent` trailingWS corruption via WriteRaw interleaving.
