# Codebase audit — 2026-07-27

Fourth full adversarial pass over spine (Go library reading/writing OOXML —
docx/xlsx/pptx — over an OPC core), and the first pass over the tree with the
complete C236–C359 remediation stack (PRs #188–#220) merged to main. Method: 17
parallel area auditors (16 area-scoped + 1 cross-format coherence), each reading
its files in full and adversarially verifying every candidate with runtime
probes, deduped against the prior audits (C1–C235 on 2026-07-07/07-11, C236–C359
on 2026-07-26, D1–D49 docs). New IDs continue at **C360**. The four criticals
were re-verified independently by the coordinating auditor with fresh repros
(three executed end-to-end, one traced); those runs are quoted inline.

Baseline health at HEAD (56e03a3): `go build ./...`, `go vet ./...` and
`go test ./...` (all 24 packages) are green. Every finding below is a defect
*within* that green baseline.

Deliberately-deferred residuals from the prior audit (C329 schema-order
insertion, C298 bibliography customXml, C333 WARC digest impl, the pivot-cache
portion of xlsx DeleteSheet) were excluded from scope and are not re-reported.
One prior finding is re-opened: **C277 was recorded as remediated but was never
fixed** (see C400).

Status key: **CONFIRMED** = fully traced, most reproduced with a runtime probe;
**PLAUSIBLE** = mechanism confirmed, trigger not reproduced.
Novelty: all findings are **NEW** unless marked otherwise.

Area key: `opc`, `xml` (common/xml + common/oxml), `dml` (common/dml + diagram),
`crypto` (common/crypto), `omml`, `vml`, `docx`, `docx-oxml`, `xlsx`,
`xlsx-oxml`, `pptx`, `pml` (pptx/internal/oxml), `chart`, `xf` (cross-format),
`dx` (docs/tooling/harness).

---

## 1. Summary table

### Critical

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C360 | opc/crypto | `readCFB` sizes an allocation from the unvalidated `numFATSectors` header field: a **512-byte** file drives a **16 GiB** allocation through `OpenEncrypted` — process-fatal under any memory limit | opc/cfb.go:141,179 | CONFIRMED-runtime |
| C361 | crypto | Agile integrity is checked only if the attacker-supplied descriptor still contains `dataIntegrity`; deleting that unauthenticated element disables the HMAC and tampered ciphertext is returned as valid plaintext | common/crypto/agile.go:334-338 | CONFIRMED-runtime |
| C362 | opc | Signature verification counts manifest references from **every** `<Object>`, not only Objects covered by `SignedInfo` — `CoveredParts`/`Valid` are forgeable without the private key | opc/signature.go:341-352 | CONFIRMED-runtime |
| C363 | pptx | `AppendSlidesFrom` onto an *opened* destination allocates presentation rel-ids from two blind allocators: `p:sldId` entries end up bound to slideMaster/notesMaster rels, `Validate()` reports nothing | pptx/merge.go:659,937,1045,309 | CONFIRMED-runtime |

### High

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C364 | pptx | `RemoveSlide` leaves other slides' inbound slide-jump rels pointing at an unwritten part (OPC-invalid); `Validate` structurally cannot see it | pptx/presentation.go:2240 | CONFIRMED-runtime |
| C365 | pptx | `RemoveSlide` leaves `p:custShow` referencing the removed slide's r:id after the rel is dropped — schema-invalid ST_RelationshipId | pptx/presentation.go:2240 | CONFIRMED-runtime |
| C366 | xlsx | `DeleteSheet` on an opaque sheet (chartsheet/dialogsheet) deletes the part but re-emits the original workbook `.rels` verbatim, still targeting it | xlsx/workbook.go:1127-1174,893-901 | CONFIRMED-runtime |
| C367 | xlsx | `date1904` is parsed and round-tripped but never consulted: `Time()`/`SetTime()`/`Value()` are wrong by 1462 days on Mac-Excel-origin workbooks | xlsx/cell.go:499-534 | CONFIRMED-runtime |
| C368 | xlsx-oxml | Dirty save sorts cells by ref and unconditionally emits `r`: rows that legally omit cell refs get reordered to the row front with `r=""` | xlsx/internal/oxml/worksheet.go:977,1029 | CONFIRMED-runtime |
| C369 | xlsx-oxml | `RawSource` offset slicing bypasses the charset-transcoding guard — non-UTF-8 workbook.xml regenerates as malformed XML, deterministically | xlsx/workbook.go:169; internal/oxml/workbook.go:171-174,224-227 | CONFIRMED-runtime |
| C370 | docx | styles/comments/notes/header/footer parts are parsed **without** `UnmarshalWithSource`, so their capture kit is inert: any regeneration deletes every unmodeled property child in the part | docx/document.go:459-519 | CONFIRMED-runtime |
| C371 | docx-oxml | `commentRangeStart`/`End` (plus `proofErr`, row-level perms, body/cell `ins`/`del`) are dropped at body, table, row and cell level — comment anchors destroyed | docx/internal/oxml/document.go:196-210; table.go:343-350,460-472,713-724 | CONFIRMED-runtime |
| C372 | docx | `SetSize`/`SetAltText` on a *parsed* image rebuilds the whole drawing: `docPr id` becomes 0, floating position/wrap/rotation lost | docx/image.go:59-90; image_read.go:173-194 | CONFIRMED-runtime |
| C373 | docx-oxml | `w:ins`/`w:del`/`w:sdt`/perms inside `w:fldSimple` are silently deleted — tracked insertions lose their text, deletions are silently accepted | docx/internal/oxml/fields.go:135-136,153-154 | CONFIRMED-runtime |
| C374 | dml | Any theme edit deletes `a:extLst` (thm15:themeFamily), `a:custClrLst` and all nested theme extLsts — live docx/xlsx `Theme()` path | common/dml/xml_theme.go:12-17; theme_edit.go:69-83 | CONFIRMED-runtime |
| C375 | xml | `AlternateContent` marshal emits a hardcoded `mc:` prefix; Word-2007 files aliasing MC as `ve:` re-emit an **undeclared** prefix — malformed XML on a zero-modification save, and `Finish()` returns nil | common/oxml/alternate_content.go:138 | CONFIRMED-runtime |
| C376 | opc | Decompression budget is bypassable: opening a stream marks the entry charged before counting bytes, so abandoning it makes the whole part free | opc/reader.go:202-208,138-140 | CONFIRMED-runtime |
| C377 | opc | `Close` writes `docProps/core.xml` and its content type, then appends a relationship that `writeRelationships` silently discards when `.rels` was preserved — orphan part, properties unreadable | opc/writer.go:406-412,466-470 | CONFIRMED-runtime |
| C378 | pptx | Timing-id seeding misses `p:cmd`/`p:video`/`p:audio`, so `AddAnimation` allocates colliding `cTn` ids — including against the library's own autoplay tree | pptx/animation.go:630-674 | CONFIRMED-runtime |
| C379 | pptx | Removing a **group** containing autoplay media leaves timing targeting the deleted spid and leaks its media rels/parts (top-level case is fixed and tested) | pptx/media_embed.go:365-405; slide.go:477-496 | CONFIRMED-runtime |
| C380 | pptx | Editing a run's text rewrites its formatting to the modeled subset — `lumMod`/`lumOff` theme tints, `lang`, `spc`, `kern`, `cap` dropped; text visibly changes color | pptx/shape_sync.go:326-331; slide.go:1183-1244 | CONFIRMED-runtime |
| C381 | pml | `SetEmbedTrueTypeFonts(false)`/`SetSaveSubsetFonts(false)` are silently ignored on parsed decks — captured `"1"` replays | pptx/marshal.go:103-108 | CONFIRMED-runtime |
| C382 | pml | An empty/whitespace-only `<p:ext uri="{sectionLst}"/>` aborts `Open` with `EOF` | pptx/internal/oxml/extension.go:363-376 | CONFIRMED-runtime |
| C383 | xlsx | `editColumn` scans only `Cols[0]`, re-introducing the C127 overlapping-`<col>` bug for SetColumnHidden/OutlineLevel/Collapsed/GroupColumns (SetColWidth was fixed; its twin was not) | xlsx/sheet_view.go:563-608 | CONFIRMED-runtime, re-opens C127 |
| C384 | chart | The NaN blank-sentinel introduced by the C250 parse fix is serialized literally: `<c:v>NaN</c:v>` in chart.xml and invalid `<v>NaN</v>` in the embedded workbook | chart/serialize.go:677-689; data.go:221 | CONFIRMED-runtime |
| C385 | crypto | Scheme dispatch reads `AlgID` alone and ignores `fAES`, so a spec-legal AES file with `AlgID=0` is routed to RC4 and reported as **wrong password** | common/crypto/rc4.go:241-251; standard.go:68 | CONFIRMED-runtime |
| C386 | xf | Encrypted-open is docx-only, and the xlsx/pptx godocs plus `opc.ErrEncrypted`'s own message direct users to an API with no bridge to a Workbook/Presentation | xlsx/workbook.go:105; pptx/presentation.go:142 | CONFIRMED |
| C387 | xf | `AddPictureFromBytes(svg, "image/svg+xml")` in pptx emits a bare `<a:blip>` → .svg with no raster fallback; docx and xlsx auto-build the conformant dual-part structure | pptx/slide.go:289 | CONFIRMED-runtime |
| C388 | dx | The entire #188–#220 wave (173 commits since 0.1.0, incl. a breaking signature change) has zero CHANGELOG coverage | CHANGELOG.md:3 | CONFIRMED |
| C389 | dx | `harvest-batch` scope lacks `OOMPolicy=continue`, so a worker OOM kills the orchestrator before it ledgers — permanent resume-loop livelock on the offending file | Makefile:74-86 | CONFIRMED |

### Medium (selected — full list in §3)

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C390 | opc | `canonicalZipEntryName` doesn't clean `..`, so such entries are reachable under no name and unsavable — round-trip of the package fails | opc/reader.go:520-529 | CONFIRMED-runtime |
| C391 | opc | `SignPackage` writes raw part names into manifest URIs while verification percent-decodes them — spine fails to verify its own signature | opc/signature.go:933-950 | CONFIRMED-runtime |
| C392 | opc | `WriteRawFile` emits arbitrary zip entry names including `../..` traversal | opc/writer.go:250-256 | CONFIRMED-runtime |
| C393 | opc | `UnmarshalContentTypes` dispatches on local name only: a foreign-namespace `Override` steers every downstream part-kind decision | opc/content_types.go:761-805 | CONFIRMED-runtime |
| C394 | opc | `nextRelID` is independent of the exported `Relationships` slice; assigning it (as opc's own signer does) mints colliding rel ids | opc/writer.go:284-293 | CONFIRMED-runtime |
| C395 | opc | Duplicate zip entries yield duplicate `Files` entries with no dedup — any replay save fails `duplicate part` | opc/reader.go:467-499 | CONFIRMED-runtime |
| C396 | opc | Repeated known core.xml element is emitted twice with the last-parsed value (schema-invalid, silently changes data) | opc/package.go:287-311 | CONFIRMED-runtime |
| C397 | crypto | `maxSpinCount` is 100× Office's value: a few hundred bytes buys ~5 s of single-core SHA-512 before the password check | common/crypto/agile.go:136 | CONFIRMED-measured |
| C398 | vml | `xml.Marshal` of any `common/vml` type produces neither VML nor well-formed XML (Go type names as elements, unbound `w:` prefixes) | common/vml/vml.go:377-413 | CONFIRMED |
| C399 | vml | C343's presence-flag fix converted 4 of ~8 fields; `x:Visible`, `x:NoThreeD`, `x:VScroll` still drop silently | common/vml/vml.go:833,844-846 | CONFIRMED-runtime, re-opens C343 |
| C400 | dml | **C277 was marked remediated but never fixed**: transitional percent forms still hard-fail whole-part `Open` on the live blip path | common/dml/xml_effect.go:683-685,705-707; xml_3d.go:20-22; xml_line.go:52-55 | CONFIRMED-runtime, re-opens C277 |
| C401 | dml | FillStyleLst/BgFillStyleLst regroup by kind, destroying the positional style matrix `a:fillRef/@idx` targets | common/dml/xml_theme.go:74-81,102-109 | CONFIRMED-runtime |
| C402 | docx | Core-property edits silently dropped when the opened package has no core-properties part | docx/document.go:734-741 | CONFIRMED-runtime |
| C403 | docx | `Run.AddComment` ignores anchor failure — orphan comment with no document anchor (C296 fixed this for the range API only) | docx/comment.go:155-160 | CONFIRMED |
| C404 | docx | Reversed range endpoints emit end-before-start markers; `Validate()` passes, `AnchorText()` returns wrong text | docx/comment.go:168-182; bookmark.go:73-87 | CONFIRMED-runtime |
| C405 | docx | `ContentControls` misses SDTs in nested tables, hyperlinks and tracked-change blocks — godoc claims full coverage | docx/internal/oxml/sdtpr.go:624-648 | CONFIRMED-runtime |
| C406 | docx | Feature mutators never call `touch()` — edits into a reopened header/footer are masked while companion parts still regenerate (orphan footnote) | docx/revisions.go:52-148; field.go:33-66; comment.go:147,155; footnote.go:78-107 | CONFIRMED |
| C407 | docx | Hyperlink/image rels leak on `SetText`/`Clear`; no removal API exists to reclaim them | docx/paragraph.go:31-37; run.go:471-474 | CONFIRMED-runtime |
| C408 | docx | Bookmark id allocation and enumeration are body-only; header bookmarks collide and are invisible | docx/bookmark.go:31-47,61-65,89-92 | CONFIRMED |
| C409 | docx | Duplicate `wp:docPr` ids for text boxes across reopen (`nextShapeID` never seeded from the document) | docx/textbox.go:135-141; image.go:342-372 | CONFIRMED-runtime |
| C410 | docx | A malformed main-part `.rels` is swallowed at open and then **not written** at save — every image/hyperlink/styles rel severed | docx/document.go:1449-1465,605-629 | CONFIRMED |
| C411 | docx-oxml | Six `*Change`/`CellMerge` revision records lack `CapturedAttrs` — `w16du:dateUtc` dropped, asymmetric with `CT_RPrChange` | docx/internal/oxml/tracking.go:78-115 | CONFIRMED-runtime |
| C412 | docx-oxml | `AllParagraphs` misses SDT-wrapped rows/cells — partial re-occurrence of C330 in the walkers it didn't reach | docx/internal/oxml/traverse.go:112-133 | CONFIRMED-runtime |
| C413 | docx-oxml | `MaxRevisionID` misses row-level `w:ins`/`w:del` wrapper ids → new revisions collide | docx/internal/oxml/revisions.go:540-585 | CONFIRMED-runtime |
| C414 | pptx | `Slide.Duplicate` shares the comments part between original and duplicate; modern anchors still name the source slide | pptx/slide.go:1584-1639 | CONFIRMED |
| C415 | pptx | `CloneRow`/`CloneColumn`/`CloneShape` drop bullet color/size/font/autonumber, marL, indent, tabStops, autofit despite advertising full fidelity | pptx/clone.go:67-101 | CONFIRMED-runtime |
| C416 | pptx | Authored animation targeting a shape deleted before save emits a dangling spid | pptx/animation.go:449-455 | CONFIRMED-runtime |
| C417 | pptx | Connector line setters destroy arrowheads, caps and unmodeled `a:ln` content — arrowheads are the normal case | pptx/connector.go:232-251,331-349 | CONFIRMED-runtime |
| C418 | pptx | Table spans can be widened but never narrowed; stale `hMerge`/`vMerge` continuation flags with no spanning master | pptx/table.go:504-550 | CONFIRMED-runtime |
| C419 | pptx | Index-derived fallback ids can duplicate preserved `sldMasterId`/`sldLayoutId` values | pptx/presentation.go:2011; master.go:479 | PLAUSIBLE |
| C420 | pml | Explicit-zero booleans dropped on the always-remarshal paths (7 confirmed on one slide); masters/layouts hit this on every save | pptx/internal/oxml/transition.go:89,116; animation.go:604,783,789,796,917-919 | CONFIRMED-runtime |
| C421 | pml | Regenerated notes part can emit an unbound `mc:` prefix; `NotesSlide` is the only remarshaled root without `OriginalRootAttrs` | pptx/notes.go:230-247 | CONFIRMED-runtime |
| C422 | xlsx | `DeleteSheet` cascade omits OLE embeddings, ctrlProps, slicer/timeline parts — orphaned, permanent bloat | xlsx/workbook.go:1454-1460 | CONFIRMED |
| C423 | xlsx | Feature mutators silently no-op on opaque sheets and `AddImage` bypasses the `markDirty` guard — attachments silently dropped at save | xlsx/sheet.go:775-782; image.go:151,163 | CONFIRMED |
| C424 | xlsx | `DeleteSheet` doesn't rewrite name/formula **references** to the deleted sheet (Excel writes `#REF!`) | xlsx/workbook.go:1577-1603 | CONFIRMED-runtime |
| C425 | xlsx | `Cell()` is a mutating accessor: read-only lookups create phantom cells that serialize once anything else dirties the sheet | xlsx/sheet.go:148-191 | CONFIRMED-runtime |
| C426 | xlsx | Defined-name legality and duplication never validated (`AddDefinedName("A1", …)` saves) | xlsx/workbook.go:1746-1776 | CONFIRMED-runtime |
| C427 | xlsx | Two-cell-anchor images emit `editAs="oneCell"`, contradicting the documented resize-with-cells behavior | xlsx/image.go:253 | CONFIRMED |
| C428 | xlsx | `AddSparklineGroup` reallocates `Groups`, silently detaching earlier handles; mutations through them vanish | xlsx/sparkline.go:288-291 | CONFIRMED-runtime |
| C429 | xlsx-oxml | `CT_BookView` carrying an inline `xmlns` makes **every save fail** — the convention its siblings adopted was never applied here | xlsx/internal/oxml/workbook.go:658-661 | CONFIRMED-runtime |
| C430 | xlsx-oxml | Styles model omits border `vertical`/`horizontal`/`start`/`end` and `extLst` on xf/dxf/cellStyle — dropped on any style edit | xlsx/internal/oxml/styles.go:248-257,280-296 | CONFIRMED |
| C431 | xlsx-oxml | Worksheet model omits autoFilter/sortState/conditionalFormatting `extLst`, `sheetPr@syncRef`, dataValidations windows | xlsx/internal/oxml/worksheet.go:1465-1469,1626-1632 | CONFIRMED |
| C432 | xlsx-oxml | Adding one sparkline group regenerates the whole x14 extension, stripping `xr2:uid` and unmodeled content from pre-existing groups | xlsx/sparkline.go:256 | CONFIRMED |
| C433 | chart | Category chart with no categories emits `c:numRef` with no `c:f` — schema-invalid, refs and data diverge | chart/refs.go:77-90 | CONFIRMED-runtime |
| C434 | chart | Ref span sized by category count while caches/cells are sized per-series — Excel silently drops the tail on refresh | chart/refs.go:77-90 vs serialize.go:677-689 | CONFIRMED |
| C435 | omml | Parse binds every root-declared prefix, marshal registers a fixed eight — a math zone that parses cleanly can be un-writable through `MathZones`→`AddMath` | docx/math.go:86-118 vs 72-79 | CONFIRMED-runtime |
| C436 | omml | `m:t` silently discards every attribute except `xml:space`, with no raw-capture fallback | common/omml/run.go:30-44 | CONFIRMED-runtime |
| C437 | xml | `RawTokenBytes` aliases the registered source; every production caller retains it, pinning the whole part (defeats docx's explicit anti-double-memory design) | common/xml/emptystyle.go:184 | CONFIRMED |
| C438 | xml | `marshalReflect` silently drops interface-typed child fields while `hasStructChildren` counts them — parent emits empty, no error | common/xml/marshal.go:248 | CONFIRMED-runtime |
| C439 | xf | docx double-save is non-deterministic: `saveNew` draws furniture rel-ids from a durable counter on every save | docx/document.go:1331-1380 | CONFIRMED-runtime |
| C440 | xf | `AddSheet` silently rewrites invalid and duplicate names while `SetName` errors on the same input | xlsx/workbook.go:1299-1300,1367 | CONFIRMED-runtime |
| C441 | xf | Invalid image bytes: xlsx errors at add-time, docx and pptx never error — and pptx's godoc promises a save-time failure that never happens | xlsx/image.go:121-123; docx/image.go:257; pptx/slide.go:288 | CONFIRMED-runtime |
| C442 | xf | Alt-text write side three-way divergent: `SetAltText` (docx) / `SetDescription` (pptx) / impossible (xlsx), while the read side is deliberately harmonized | docx/image.go:67; pptx/media.go:164 | CONFIRMED |
| C443 | dx | `TestCCCorpus` passes vacuously on an empty or partially-fetched corpus | cctest/corpus_test.go:184-186 | CONFIRMED |
| C444 | dx | `make test-corpus` violates the project's own systemd-capping rule | Makefile:49-53 | CONFIRMED |
| C445 | dx | No spec coverage ratchet: xlsx-oxml already skips 139/207 unmarshal cases as "no Go type mapped", unmonitored | spec/spectest/spectest.go:171-174 | CONFIRMED-measured |
| C446 | dx | README and troubleshooting both claim a dangling `numPr` blocks saves; the code makes it a warning | README.md:205; docs/troubleshooting.md:23-26 | CONFIRMED |

### Low

| ID | Area | Issue | Location | Status |
|----|------|-------|----------|--------|
| C447 | opc | `writeCoreProperties`/`writeExtendedProperties` don't record parts in `w.parts` while `writeCustomProperties` does | opc/writer.go:436-509 | CONFIRMED |
| C448 | opc | Exported `ContentTypes.OriginalXMLSep` is inert for every parsed instance — its only reader is unreachable | opc/content_types.go:254-258,473 | CONFIRMED |
| C449 | opc | Comments in `[Content_Types].xml` dropped on regeneration; captured `RootEnd` never replayed | opc/content_types.go:735-749 | CONFIRMED-runtime |
| C450 | opc | `WritePartRelationships` returns success on a closed Writer for the empty-slice case | opc/writer.go:557-563 | CONFIRMED |
| C451 | opc | "missing coreProperties root element" fires only when no element was seen; any root name is accepted | opc/package.go:729-731 | CONFIRMED |
| C452 | opc | Mandatory `[Content_Types].xml` located by raw-name EqualFold only, defeating the C51 canonicalization | opc/reader.go:447-459 | CONFIRMED |
| C453 | opc | The two CFB writers disagree on the start sector of a zero-length stream; the flat writer produces a container `readCFB` rejects | opc/cfb_write.go:340-351 | PLAUSIBLE |
| C454 | opc | `ensureSelfContainedNS` injects only the element's own prefix, not descendants' — regenerated core.xml can be namespace-ill-formed | opc/package.go:543-571 | PLAUSIBLE |
| C455 | opc | `CustomProperties.Set`/`Marshal` panic on a nil receiver while eight sibling methods nil-guard — and nil is the documented common state | opc/custom_properties.go:177,286 | CONFIRMED |
| C456 | opc | Directory entries with backslash separators are recorded on read and silently skipped on write | opc/reader.go:481-483 vs writer.go:198-205 | CONFIRMED |
| C457 | opc | `GetFile` is a linear EqualFold scan called per part by every save/sign path — signing is O(n²) | opc/reader.go:639-647 | CONFIRMED |
| C458 | opc | Dead condition `HasSuffix(raw,"/>") && HasSuffix(raw," />")`; `ValidatePartName` godoc omits the part/directory collision rule and any length bound | opc/content_types.go:759; part.go:28-59 | CONFIRMED |
| C459 | opc | No entry-count bound alongside the byte bounds — a small zip can declare 200k entries | opc/reader.go | CONFIRMED |
| C460 | opc | `SignatureInfo` godoc names a `DigestMethod` field that doesn't exist; `doc.go` omits `writeCustomProperties`; SHA-1 signatures report `Valid` indistinguishably | opc/signature.go:112-113; doc.go:22-24 | CONFIRMED |
| C461 | crypto | The `1<<24` FAT-sector guard still permits an ~8 GiB allocation from a ~67 MB input | opc/cfb.go:175-179 | PLAUSIBLE |
| C462 | crypto | `LegacyPasswordHash` iterates UTF-8 bytes, not characters — non-ASCII protection passwords don't match Excel's | common/crypto/legacy.go:22-27 | CONFIRMED |
| C463 | crypto | `ecdsaDERToP1363` panics rather than errors on an out-of-range R/S from a misbehaving `crypto.Signer` | common/crypto/sign.go:209-219 | PLAUSIBLE |
| C464 | crypto | `passwordToUTF16LE` diverges from Office for invalid UTF-8 and >255-char passwords | common/crypto/agile.go:741-755 | CONFIRMED |
| C465 | crypto | Crypto godoc drift: `ErrUnsupportedEncryption` names RC4 as unimplemented (it is implemented), 2 of 4 sentinels documented, agile bullet describes write-only behavior, RC4 "read path only" contradicts an exported encryptor | common/crypto/agile.go:46-51; doc.go:6-8,15-17,51-56 | CONFIRMED |
| C466 | crypto | The msoffcrypto cross-validation the docs advertise as present-tense never runs (no Python in CI, skips locally) | common/crypto/rc4_crossvalidate_test.go:16-32 | CONFIRMED-executed |
| C467 | vml | `vml.Group` discards `o:lock`, `w10:wrap`, `x:ClientData`, `v:textbox` children; `Shape` drops `o:gfxdata` and friends | common/vml/vml.go:11-29 | CONFIRMED-runtime |
| C468 | vml | Child order normalized to Go struct-field order, so the package can never be byte-faithful | common/vml/vml.go:59-75 | CONFIRMED-runtime |
| C469 | omml | Four systematic parse/emit asymmetries invent attributes (`m:count m:val="0"` is out of its schema range) and normalize `<m:t></m:t>` → `<m:t/>` | common/omml/values.go:30-36; run.go:47-53 | CONFIRMED-runtime |
| C470 | omml | A single malformed numeric attribute aborts the whole math zone, while unknown *elements* are tolerated — inverted strictness | common/omml/values.go:39-52,328-343 | CONFIRMED-runtime |
| C471 | omml | Duplicate typed children last-wins silently; CharData between math items dropped | common/omml/raw.go:156-171 | CONFIRMED-runtime |
| C472 | xml | `MarshalRoot`'s `append(extraAttrs, attrs...)` writes into the caller's backing array | common/xml/marshal.go:182 | CONFIRMED-runtime |
| C473 | xml | `elemSeparator` mode injects whitespace inside expanded empty pairs, turning the separator into element content | common/xml/builder.go:516,774 | CONFIRMED-runtime |
| C474 | xml | Empty-tag-style capture collapses the whitespace-before-slash lexeme: `<leaf\n/>` replays `<leaf/>`, `<t\t/>` replays `<t />` | common/xml/emptystyle.go:82-87 | CONFIRMED-runtime |
| C475 | xml | `UnmarshalOrderedChildren` silently discards non-whitespace mixed content; its duplicated-singleton doc claims parity with encoding/xml but is first-wins vs last-wins | common/xml/children.go:192-216 | CONFIRMED |
| C476 | xml | AlternateContent Choice/Fallback attrs use `CaptureAttrs` not `CaptureAttrsSource` — quote style and spacing lost | common/oxml/alternate_content.go:88,107 | CONFIRMED |
| C477 | xml | Subset canonicalization omits inherited `xml:*` attributes required by inclusive C14N (safe direction: false negative) | common/xml/c14n.go:278 | CONFIRMED |
| C478 | xml | `StartElementWithRootAttrs` never registers a default xmlns for a non-root URI | common/xml/builder.go:352-375 | PLAUSIBLE |
| C479 | dml | `Color.WithTint` silently ignored for RGB colors across the whole convenience API | common/dml/fill.go:119-126; color.go:177-187 | CONFIRMED-runtime |
| C480 | dml | `EffectContainer.Blur`/`EffectDag.Cont` pointer replacement after parse silently discarded, contradicting "also settable programmatically" | common/dml/xml_effect.go:141-147,712-719 | CONFIRMED-runtime |
| C481 | dml | `TileXML` explicit `sx="0"`/`sy="0"` deleted, flipping 0% scale to the 100000 default | common/dml/xml_types.go:967-976 | CONFIRMED-runtime |
| C482 | dml | Five color types have no `MarshalXML`, so stdlib marshal drops captured duplicate transforms and reorders | common/dml/xml_color_order.go | CONFIRMED-runtime |
| C483 | dml | CxnSpLocks/GrpSpLocks/GraphicFrameLocks lack `CapturedAttrs` while SpLocks/PicLocks capture | common/dml/xml_shape.go:53-66,121-131,196-205 | CONFIRMED |
| C484 | dml | Out-of-range percentages hard-fail `Open` instead of degrading, contradicting the `roundInt64` policy for coordinates | common/dml/percentage.go:54-83 | CONFIRMED |
| C485 | dml | Diagram `PrSet` drops presLayoutVars/style children; `ColorList` models 2 of 6 color kinds and loses positional order | common/dml/diagram/diagram.go:50-80 | CONFIRMED |
| C486 | dml | `Wsp` claims dml-wordprocessingDrawing.xsd but the real `wps:wsp` is a different namespace — would parse to an empty struct | common/dml/xml_misc.go:5-19 | PLAUSIBLE |
| C487 | dml | `buildDataModel` returns bytes without checking `Finish()`/`Err()`; `Build()` has no error return | common/dml/diagram/build.go:75-145 | CONFIRMED |
| C488 | docx | `AddSectionBreak` leaves the new final section with an empty sectPr — page geometry and header/footer refs don't carry over | docx/document.go:1780-1804 | CONFIRMED |
| C489 | docx | `Append` imports source external rels unconditionally and aliases unresolvable internal rel ids | docx/merge.go:145-177 | PLAUSIBLE |
| C490 | docx | `TextBoxes()` header/footer sides walk only top-level paragraphs, contradicting the godoc | docx/textbox.go:432-455 | CONFIRMED |
| C491 | docx | docPr-id seeding depends on `id` being the first attribute of `wp:docPr` | docx/image.go:376-386 | PLAUSIBLE |
| C492 | docx | `AddHeader` over a preserved same-type header orphans the old part and its relationship | docx/headerfooter.go:96-123 | CONFIRMED |
| C493 | docx | `SetMargins` cannot set Header/Footer distance to 0; `Margins()` returns no ok-bool unlike its newer siblings | docx/section.go:95-109 | CONFIRMED |
| C494 | docx | `Revision.Accept`/`Reject` ignore the transform's bool — a stale handle "succeeds" and still flags the part modified | docx/revisions.go:264-312 | CONFIRMED |
| C495 | docx | Mid-document `sectPrChange` and SDT-wrapped-table revisions invisible to `Revisions()`, though `MaxRevisionID` walks them | docx/revisions.go:334-353 | CONFIRMED |
| C496 | docx | Revision-id seed scans the body only; authored ids can reuse header/footer ids | docx/revisions.go:25-34 | CONFIRMED |
| C497 | docx | `Charts()` iterates header/footer maps in map order — nondeterministic, contradicting "in document order" | docx/chart.go:241-256 | CONFIRMED |
| C498 | docx | `MergeFields`/`FormFields` miss fields inside SDT content and tracked changes; complex-field state resets at paragraph boundaries | docx/mailmerge.go:204-290; formfield.go:54-135 | CONFIRMED |
| C499 | docx | `nextParaID` uniqueness scan misses table-cell and header paragraphs (inconsistency left by the C267 fix) | docx/comment.go:418-441 | CONFIRMED |
| C500 | docx | Person `CapturedAttrs` captured but never replayed; `peopleModified` set even when nothing was added | docx/marshal.go:169-193; comment.go:260 | CONFIRMED |
| C501 | docx | No basedOn-cycle or self-reference guard on styles, in setters or in `Validate` | docx/styles.go:145-148 | PLAUSIBLE |
| C502 | docx | Metadata part names hardcoded; a nonstandard styles/numbering target yields a second orphaned part on edit | docx/document.go:1179-1210 | PLAUSIBLE |
| C503 | docx | `Append` doesn't remap bookmark ids/names or `w14:paraId` — duplicate `_GoBack`, foreign comments appearing threaded | docx/merge.go | PLAUSIBLE |
| C504 | docx | `hasAddedParts` forfeits `[Content_Types].xml` fidelity unnecessarily; its comment is stale | docx/document.go:787,1299-1306 | CONFIRMED |
| C505 | docx | Hyperlink godoc claims accessors are shared verbatim with xlsx/pptx, but no SetURL/SetAnchor exists in any format; `ownerPart` duplicated | docx/hyperlink.go:14-17 | CONFIRMED |
| C506 | docx-oxml | Session `w:num`/`w:abstractNum` emitted after a raw `numIdMacAtCleanup` — schema order violation introduced by the C352 rewrite | docx/internal/oxml/numbering.go:96-142 | CONFIRMED-runtime |
| C507 | docx-oxml | `MaxBookmarkID` scans only paragraph-level starts; table/row/cell/body bookmarks (column bookmarks) not counted | docx/internal/oxml/bookmark_anchor.go:44-60 | CONFIRMED |
| C508 | docx-oxml | `contentParagraphs` for SDT-wrapped cells reads only `tc.P` — nested tables/SDTs invisible | docx/internal/oxml/sdt.go:131-141 | CONFIRMED |
| C509 | docx-oxml | `CT_HeaderReference` emits `r:id` before `w:type`; `CT_HdrFtrRef` emits Word's order — two models of one element disagree | docx/internal/oxml/header_types.go:37-49 | CONFIRMED |
| C510 | docx-oxml | Dead code: `trChildCustomXmlCell` never produced/consumed; `WExtensionList`/`WExtension` referenced nowhere | docx/internal/oxml/table.go:333; extension.go:9-47 | CONFIRMED |
| C511 | docx-oxml | `atoiOK("-")` returns `(0,true)`; no overflow guard on id parsing | docx/internal/oxml/comments.go:152-172 | CONFIRMED |
| C512 | pptx | Source rel of type slide not in `sldIdLst` is silently dropped from presentation rels on save (the C118 unreferenced-slide case) | pptx/presentation.go:1417-1436 | PLAUSIBLE |
| C513 | pptx | `importMaster`/`addImportedLayout` swallow marshal failures — a failed master import silently becomes the default master | pptx/merge.go:580-586,683-690 | CONFIRMED |
| C514 | pptx | `AddShape` godoc omits `*Connector` which the switch accepts; `Theme` setters are silent no-ops on a documented read-only view | pptx/slide.go:195-203; theme.go | CONFIRMED |
| C515 | pptx | Dead code in both save paths; media dedup scans `otherParts` in map order, so byte-identical media under two names picks a nondeterministic target | pptx/presentation.go:1062-1066,1794-1800 | CONFIRMED |
| C516 | pptx | Morph transition read-back drops `p:sndAc` — read-modify-write deletes the sound the C312 fix added | pptx/transition.go:511-543 | CONFIRMED |
| C517 | pptx | Dirty-paragraph patch replaces the whole `a:pPr` with the modeled subset; `lvl="0"` always emitted once any property is set | pptx/shape_sync.go:320-325 | CONFIRMED |
| C518 | pptx | Run bool/zero collapse: explicit `b="0"` and zero spacing unrepresentable — editing a `b="0"` run inside a bold placeholder re-bolds it | pptx/slide.go:1202-1214 | CONFIRMED |
| C519 | pptx | `Animations()` handles for parsed effects are dead — documented chaining mutators silently no-op; `EffectUnknown` silently dropped | pptx/animation.go:157-164 | CONFIRMED |
| C520 | pptx | A leading With/After-Previous animation still waits for a click; entrance effects omit the `p:set` visibility PowerPoint pairs with them | pptx/animation.go:411-418 | PLAUSIBLE |
| C521 | pptx | One-way setters on the flush path: `SetName("")`, hyperlink removal, cell anchor and borders cannot be cleared | pptx/shape_sync.go:170-177,520-560 | CONFIRMED |
| C522 | pptx | Unreachable `partName == ""` guards in chart/smartart/ole; `Comment.hasPos` written never read; garbled `Comment.Position` godoc | pptx/chart.go:56-59; smartart.go:248-252 | CONFIRMED |
| C523 | pml | p14/p15 leaf extensions without `CapturedAttrs` double-declare xmlns on the child | pptx/internal/oxml/extension.go:461-490 | CONFIRMED-runtime |
| C524 | pml | `TimeNodeList` with directly-populated slices and empty childOrder marshals `<p:tnLst/>` — 13 exported slices, no guard, no doc warning | pptx/internal/oxml/animation.go:338-375 | CONFIRMED-runtime |
| C525 | pml | Modern-comment re-marshal drops empty `<p188:replyLst/>`, invents empty author attrs, fixes attr order | pptx/internal/oxml/moderncomments.go:200-203 | CONFIRMED-runtime |
| C526 | pml | Default-TRUE presProps/viewProps booleans modeled as plain `bool`+omitempty — a loaded gun for the next capability wave | pptx/internal/oxml/presprops.go:20-75; viewprops.go:11-108 | CONFIRMED |
| C527 | pml | Phantom types (`SlideProperties`, `SlideLayoutProperties`, `SlideMasterProperties`, `CommentText`) wired into spec coverage make two spec tests pass vacuously | pptx/internal/oxml/presprops.go:96-118; spec_test.go:76,82 | CONFIRMED |
| C528 | pml | Unknown root-level children of sld/sldLayout/sldMaster are `d.Skip()`ed, unlike the shape tree's raw capture; AC anchor not advanced | pptx/internal/oxml/root_marshal.go:153-157 | CONFIRMED |
| C529 | pml | `encodeRawChild` drops namespaces bound to ancestor-declared unknown prefixes and forces `/>` on empty children | pptx/internal/oxml/rawchild.go:56-104 | PLAUSIBLE |
| C530 | pml | Typed-URI ext with missing child fabricates `<p14:creationId val="0"/>`; extra sibling children discarded | pptx/internal/oxml/extension.go:268-306 | PLAUSIBLE |
| C531 | pml | `AnimVariantFloat` re-renders via `%g` — lexical drift for `val="1.0"`/`1E3` | pptx/internal/oxml/animation.go:859-862 | PLAUSIBLE |
| C532 | pml | `SlideID.MarshalXML` stdlib shadow drops `ExtLst` and writes a literal `r:id` — same shape C355 deleted for `SlideLayoutID` | pptx/internal/oxml/presentation.go:286-293 | CONFIRMED |
| C533 | xlsx | Stale `AddImage` godoc still documents the pre-C249 destructive drawing replacement | xlsx/image.go:90-95 | CONFIRMED |
| C534 | xlsx | `quoteSheetName` doesn't quote cell-reference-lookalike sheet names in print area/titles | xlsx/page_setup.go:394-409 | PLAUSIBLE |
| C535 | xlsx | Table-name validation is a subset of Excel's (interior chars, length, defined-name namespace collision); `AddPivotTable` validates syntax not at all | xlsx/table.go:410-424 | CONFIRMED |
| C536 | xlsx | `SortState()` reads the autoFilter-nested state but `RemoveSortState` cannot remove it | xlsx/filter.go:239-260 | CONFIRMED |
| C537 | xlsx | Chart mutated after `AddChart` shows in `Charts()` but is not saved | xlsx/chart.go:59-88 | CONFIRMED |
| C538 | xlsx | `SetAutoFilter`/`AddDataValidation` accept arbitrary strings as ranges while sibling APIs parse theirs | xlsx/sheet.go:661-669,722-769 | CONFIRMED |
| C539 | xlsx | `addComment` dereferences `s.workbook` without the nil guard its sibling writers have | xlsx/comment.go:402 | CONFIRMED |
| C540 | xlsx | Any comment mutation regenerates all note-box VML with default geometry — user-positioned/styled notes reset | xlsx/comment_vml.go:102-118 | CONFIRMED |
| C541 | xlsx | Totals row recorded in the table part but no SUBTOTAL formulas/labels written into the sheet cells | xlsx/table.go:256-268 | PLAUSIBLE |
| C542 | xlsx | iconSet: no threshold-count validation, and default percents are `0,33,66` where Excel writes `0,33,67` | xlsx/conditional_format_write.go:267-292 | CONFIRMED |
| C543 | xlsx | `AddPivotTable` has no anchor-vs-source overlap check; with refreshOnLoad Excel's rebuild clobbers source data | xlsx/pivot_table.go:369-419 | PLAUSIBLE |
| C544 | xlsx | `ApplyNamedStyle` dirties styles before its dedup check and can panic on a malformed cellStyles/cellStyleXfs pairing | xlsx/style.go:378-411 | CONFIRMED |
| C545 | xlsx | `SetName` over-dirties: rename regenerates the worksheet part if the model was ever materialized (behavior depends on read history) | xlsx/sheet.go:119-137 | CONFIRMED-runtime |
| C546 | xlsx | `SetRowHeight` missing the MaxRow bound its siblings enforce | xlsx/sheet.go:377-379 | CONFIRMED |
| C547 | xlsx | `ParseCellRef` accepts `+`-prefixed rows via `strconv.Atoi` — `"A+5"` silently addresses A5 | xlsx/workbook.go:1722-1729 | CONFIRMED-runtime |
| C548 | xlsx | `Value()` returns nil for error cells (indistinguishable from empty); `t="d"` cells misclassified as strings | xlsx/cell.go:39-72,119-148 | CONFIRMED |
| C549 | xlsx | `CopySheetFrom`: ranged col explosion, single-cell merges dropped, theme/indexed colors lost, half-populated sheet on error | xlsx/merge.go:53-107,236-277 | CONFIRMED |
| C550 | xlsx | `NewCellStyle` with NumberFormatID ≥164 emits a dangling numFmtId | xlsx/style.go:191-198 | CONFIRMED |
| C551 | xlsx-oxml | Expanded-empty `<definedName></definedName>` collapses to self-closing on a zero-mod save of an always-regenerated part | xlsx/internal/oxml/workbook.go:1053-1065 | CONFIRMED-runtime |
| C552 | xlsx-oxml | Numeric-parse leniency is three-way inconsistent: fabricate-zero (`si="abc"`→0, can merge shared-formula groups), skip, or fail-Open | xlsx/internal/oxml/worksheet.go:1095-1098; shared_strings.go:28-39 | CONFIRMED |
| C553 | xlsx-oxml | Duplicated singleton children parse last-wins but ChildOrder keeps both — the survivor is emitted twice | xlsx/internal/oxml/worksheet.go:192-196 | CONFIRMED |
| C554 | xlsx-oxml | Dead fidelity code: `CT_Sst.OriginalNSDecls` (no sst writer exists), `CT_Workbook.OriginalNSDecls` (never assigned), hand-rolled `ensureDrawingInChildOrder` duplicating `EnsureChildOrder` | xlsx/internal/oxml/shared_strings.go:18-20; xlsx/marshal.go:35-37 | CONFIRMED |
| C555 | xlsx-oxml | Removing a ChildOrder-gated element leaves its per-gap whitespace neighbors doubled | xlsx/marshal.go:70 | CONFIRMED |
| C556 | xlsx-oxml | `CT_Scenario` emits `user=""` unconditionally; reflection float attrs use `%g`, drifting Excel's E-notation tints | xlsx/internal/oxml/scenarios.go:146 | CONFIRMED |
| C557 | chart | Combo containing a horizontal-bar group parses but can never be re-marshaled | chart/parse.go:363-366 vs serialize.go:463-466 | CONFIRMED-runtime |
| C558 | chart | `quoteSheet` doesn't quote sheet names that lex as cell references or reserved words | chart/refs.go:153-170 | PLAUSIBLE |
| C559 | chart | `formatFloat` uses `%g` in caches while the data sheet uses `%f` — cache and sheet disagree textually for the same value | chart/serialize.go:771-773 | CONFIRMED-runtime |
| C560 | chart | `Parse` is heavily lossy in undocumented ways: stacked grouping collapses to clustered — a data-presentation mutation, not just formatting loss | chart/parse.go | CONFIRMED |
| C561 | chart | Pie-family drops all series after the first (including doughnut, where multiple rings are legitimate) while the workbook still holds their data | chart/serialize.go:160-196 | CONFIRMED |
| C562 | chart | `AddChart` mutates the caller's `*chart.Chart` via `SetDataRef` | xlsx/chart.go:62-63; docx/chart.go:74; pptx/chart.go:63 | CONFIRMED |
| C563 | chart | Mixed plot areas outside the bar/line/area triple silently truncated to one group | chart/parse.go:29-66 | CONFIRMED |
| C564 | chart | Charts on chartsheets invisible to `Sheet.Charts`/`Workbook.Charts`; workbook merge skips charts entirely | xlsx/chart_reader.go:46-52 | CONFIRMED |
| C565 | xf | Lookup/removal naming drift: `SheetByName` returns `(v, error)`, `GetLayoutByName` returns nil-on-miss; `DeleteSheet` vs `RemoveSlide`; `Get` prefix on exactly 4 methods | xlsx/workbook.go:1419; pptx/presentation.go:917 | CONFIRMED |
| C566 | xf | `AddChart` parameter order flips: chart last in xlsx, first in docx/pptx | xlsx/chart.go:46 vs pptx/chart.go:47 | CONFIRMED |
| C567 | xf | Vestigial aliases with no `Deprecated:` marker: pptx `SaveAs`, `AddSlideWithLayout`, xlsx `WriteToBuffer` | pptx/presentation.go:816,2229 | CONFIRMED |
| C568 | xf | Identical impossible-state handling differs three ways: docx panics with a diagnostic, xlsx and pptx silently return nil models (invisible data loss) | docx/document.go:238; xlsx/sheet.go:27; pptx/slide.go:99 | CONFIRMED |
| C569 | xf | Cross-file composition three names/granularities; xlsx alone has no whole-file merge | docx/merge.go:67; xlsx/merge.go:31 | CONFIRMED |
| C570 | xf | `Open` resource model differs: xlsx holds an OS handle until Close, docx/pptx slurp — an xlsx-only fd leak for users trained by the others | xlsx/workbook.go:107 | CONFIRMED |
| C571 | xf | `Theme()` returns a read-mostly pptx-local type in pptx but the shared `*dml.ThemeEditor` in docx/xlsx — the largest same-name-different-type collision in the public surface | pptx/theme.go:8 | CONFIRMED |
| C572 | dx | Symmetry guard frozen at 3 capabilities; charts/theme/page-print/protection unguarded, embedded-type methods a blind spot | internal/symmetry/symmetry_test.go:88-174 | CONFIRMED |
| C573 | dx | Part-map comparison collapses duplicate zip entries, so the fidelity harness is blind to duplicate-entry handling | internal/testutil/roundtrip.go:35-55 | CONFIRMED |
| C574 | dx | The batched harvest's only durable output is gitignored — the failure catalog of a 30k-reference harvest exists on one machine | .gitignore:41; Makefile:81 | CONFIRMED |
| C575 | dx | `spec/gen_spec` is simultaneously tracked and ignored — new files there are invisible to `git status` (the C182 loss class) | spec/.gitignore:8 | CONFIRMED |
| C576 | dx | `testdata/README.md` claims python-pptx's MIT notice applies, but no license text is vendored for 2.9 MB of verbatim source | python-tests/; testdata/README.md:14-21 | CONFIRMED |
| C577 | dx | DoH URL demoted from env to argv on the orchestrator→worker hop — a private resolver token becomes world-readable via `/proc` | tools/ccrun/main.go:349-354 | CONFIRMED |
| C578 | dx | CI claims to mirror the local merge gates but omits `make fetch`, `-race` and fuzz; every external-fixture test permanently skips | .github/workflows/ci.yml | CONFIRMED |
| C579 | dx | AppendSlidesFrom source-mutation, slide-jump drops, `Image.SVGData`, `DeleteSheet`, and typed `Cell.Value` are godoc-only — the guides never followed | docs/pptx.md; docs/xlsx.md | CONFIRMED |
| C580 | dx | README's public-surface justification names two packages no example imports and omits the one they do | README.md:297 | CONFIRMED |
| C581 | dx | Two examples write output to `os.TempDir()` while the other eight write to the working directory | examples/pptx_deck/main.go:64 | CONFIRMED |
| C582 | dx | 34 stale agent worktrees (1.1 GB) still registered in the checkout | .claude/worktrees/ | CONFIRMED |

---

## 2. System map

**Layering** (bottom-up): `common/xml` (Builder + reflection marshaler + the
capture kit; stdlib-only) → shared typed models (`common/dml` and its
`chart`/`diagram` children, `oxml`, `omml`, `enum`, `vml`) and `common/crypto`
→ `opc` (parts, relationships, `[Content_Types].xml`, signatures, CFB-wrapped
encrypted containers) → `common/validate` → the format packages
(`docx`/`xlsx`/`pptx`, each with an `internal/oxml` typed model) and the public
`chart` builder.

**Real execution paths.** Two lifecycles per format, and the defect density is
overwhelmingly on the *Open* one:

- **Create → Save (`saveNew`)**: every part generated from the model. The bugs
  here are id-allocation (C363, C409, C419) and explicit-vs-inherit emission.
- **Open → mutate → Save (`saveRoundTrip`)**: untouched parts copied verbatim,
  touched parts regenerated. This audit finds the dominant class has **moved
  down a level**. The previous wave closed regeneration-from-narrow-model at the
  *tree* level; it now reappears at *leaf* granularity — a dirty run, paragraph,
  line, drawing, theme, or sparkline group is rebuilt from the domain model and
  loses everything the model can't say (C374, C380, C372, C417, C432, C517).
  Alongside it sits the **inert-capture-kit** variant: parts whose fidelity
  machinery exists but was never armed because the parse path skipped
  `UnmarshalWithSource` (C370), or whose raw-child whitelist was never extended
  (C371, C373).

**A third class is now clearly dominant on the write side: deletion doesn't
sweep references.** Removing a node cleans the node and its own edges, never the
inbound ones — pptx slide-jump rels and custom shows (C364, C365), xlsx opaque
sheet rels and defined-name values (C366, C424), docx hyperlink rels (C407),
pptx group media (C379). In every case `Validate()` passes, because each
format's validator checks the *source* package's part set rather than the output
set — it is structurally incapable of catching deletion-induced dangling
references.

**Key invariants** (asserted, and where they break):
1. *Byte-identity for untouched content* — holds for never-accessed parts.
   Breaks for always-regenerated parts under charset transcoding (C369) and for
   any part whose capture kit is inert (C370).
2. *Relationship ids are unique per rels scope* — the per-part allocators are
   correct, but the pptx presentation part now has **three** allocators that
   cannot see each other's pending ids (C363), and `opc.Writer`'s counter is
   decoupled from its own exported slice (C394).
3. *Edits win over captured fidelity* — true for value changes, **false for
   clears**: any modeled zero suppressed by omitempty lets the captured original
   replay, so a setter that clears a parsed value is a silent no-op (C381, and
   the general form in Tension 4).
4. *Encrypted means authenticated* — **false**. Integrity is optional in the
   descriptor, unauthenticated, and trivially strippable (C361).
5. *A valid signature covers the parts it lists* — **false**. Coverage is read
   from unsigned Objects (C362).

---

## 3. Findings by category

Full per-finding detail for the criticals and highs; mediums and lows are
enumerated in §1 with location, mechanism and status, which is sufficient for a
fixing agent to act. Each entry below gives the concrete failure scenario.

### 3.1 Security and resource exhaustion

**C360 — CFB header-driven allocation (opc/cfb.go:141,179).** `readCFB` does
`fatSectorLocs := make([]uint32, 0, numFATSectors)` where `numFATSectors` is
`binary.LittleEndian.Uint32(data[44:48])`, unvalidated against the file size.
Verified independently: a 512-byte buffer with a valid CFB magic,
`sectorShift=9`, `miniSectorShift=6`, conformant `miniCutoff=4096` (so the C276
guard passes) and `numFATSectors=0xFFFFFFFF` produced

```
input=512 bytes  err=opc: corrupted package: CFB directory has no root entry
TotalAlloc delta = 17179900656 bytes (16.00 GiB)
```

`MaxEncryptedInputSize` (2 GiB) never engages — the input is 512 bytes. Under
`GOMEMLIMIT`, a container limit, `RLIMIT_AS` or 32-bit this is an immediate
unrecoverable OOM that kills the process, not the request. A second
amplification at line 179 is bounded by a `1<<24` check placed *after* the loop
that could already have exhausted memory (C461). **Direction:** bound every
`make` whose capacity derives from parsed bytes by `len(data)/sectorSize` before
allocating. Note C276 hardened two sites in this same file one commit ago and
left the largest one twelve lines away.

**C361 — Agile integrity is optional and unauthenticated
(common/crypto/agile.go:334-338).** The HMAC is verified only inside
`if info.DataIntegrity.EncryptedHmacKey != "" && info.DataIntegrity.EncryptedHmacValue != ""`.
Nothing binds the `dataIntegrity` element to the password: it is not covered by
any MAC, verifier, or key derivation. Verified independently:

```
[baseline]                        err=<nil> match=true
[bitflip, descriptor intact]      err=crypto: encrypted package failed integrity check
[bitflip, dataIntegrity deleted]  err=<nil>
  *** ACCEPTED TAMPERED DATA ***
  differs from original plaintext: true
```

Renaming one of the two attributes is equally sufficient — the `&&` makes a
half-present block skip the check rather than error. AES-CBC is malleable, so an
attacker with write access (mail relay, shared drive, object store) can flip
chosen bits in the decrypted zip — content types, part bytes, `vbaProject.bin` —
while the legitimate password still verifies. The decrypted bytes go straight
into `NewReaderWithOptions` and are trusted as a package. **Direction:**
`dataIntegrity` is mandatory for agile password encryptors; treat absent or
half-present as `ErrIntegrityCheckFailed`.

**C362 — Signature coverage is forgeable (opc/signature.go:341-352).**

```go
for _, obj := range sig.Objects {
    for _, ref := range obj.Manifest.References {
        res, part := r.verifyManifestReference(ref)
        info.References = append(info.References, res)
        if part != "" { info.CoveredParts = append(info.CoveredParts, part) }
    }
```

The loop walks *every* `<Object>` with no check that `obj.ID` is reachable from
a valid `SignedInfo` reference — the `SignedInfo → Object → Manifest` chain that
is the entire trust path is flattened into two independent lists that are ANDed.
Appending an `<Object>` with a Manifest listing extra parts and their correct
digests, leaving `SignedInfo` untouched, yields
`Valid=true SignedInfoValid=true` with the injected part in `CoveredParts`. The
API reports a part as signed by the certificate holder when it is not.
**Direction:** compute the covered set only from Objects whose digest was signed;
report unreferenced Objects as a `Problem`.

**C376 — Decompression budget bypass (opc/reader.go:202-208).** `openZipEntry`
sets `b.charged[zf] = true` before any bytes are counted; `charge` then
short-circuits on that boolean. Reading one byte from a part's stream and
abandoning it makes the rest of that part free. Measured with a 2500-byte
package budget over two 2000-byte parts: 4000 bytes decompressed, 2001 charged.
Repeat across N parts and `MaxDecompressedPackageSize` stops meaning anything.
**Direction:** track charged *bytes* per entry and charge the delta.

**C385 — AES file reported as wrong password (common/crypto/rc4.go:241-251).**
`isRC4CryptoAPI` returns `algID == stdAlgRC4 || algID == 0`. MS-OFFCRYPTO
§2.3.4.5 permits `AlgID = 0` meaning "determined by the flags"; with `fAES` set
it is AES-128. The disambiguating byte is in the same buffer and never
consulted. A conformant AES file with `AlgID=0` returns
`crypto: wrong password` — the worst possible error, since the natural response
is to retry passwords indefinitely on a file that is decryptable and supported.

### 3.2 Package corruption

**C363 — pptx merge rel-id collision.** Two allocators share one namespace:
`AddSlide` takes `rId{p.nextRelID}`, while `importMaster`/`importNotesMaster`/
`importHandoutMaster`/`importLegacyCommentAuthors` take
`nextRelationshipID(p.relationships["/ppt/presentation.xml"])`, which cannot see
pending slide relIDs (those enter the map only at save) and never bump
`p.nextRelID`. Verified independently — created deck saved and reopened, then
`AppendSlidesFrom` a corpus deck:

```
Validate() findings: 0
sldId  rel ids: rId2, rId7, rId8, rId9 …
master rel ids: rId1, rId8
  rId7 -> notesMasters/notesMaster1.xml
  rId8 -> slideMasters/slideMaster2.xml
```

Two `p:sldId` entries resolve to a **notesMaster** and a **slideMaster**, and
`rId8` is double-bound to a slide entry and a master entry. PowerPoint repairs
or refuses such a deck. `Validate()` reports zero findings, and the existing
`TestAppendOntoOpenedDestImportsMaster` passes because `SlideCount()` counts
`sldId` entries whose rels resolve to *any* parseable part — a slideMaster loads
as a "slide". **Direction:** one presentation-level allocator consulting both the
rels map max and `p.nextRelID` and bumping past the result (the pattern
`vba.go:118 nextPresentationRelID` already gets right). Tests must assert rel-id
uniqueness and that every `sldId`/`custShow` r:id resolves to a slide-type rel.

**C364/C365 — RemoveSlide leaves inbound references.** `RemoveSlide` cleans the
removed slide's own rels, owned parts and section membership, but never scans
other parts' rels for `RelTypeSlide` targets resolving to the removed part, nor
custom shows. Repro: slide 1 with `SetHyperlinkToSlide(2)`, remove slide 3, save
→ `slide1.xml.rels` still targets `slide3.xml`, absent from the zip, while
`slide1.xml` keeps its `ppaction://hlinksldjump`. Separately a custom show
emits `<p:sld r:id="rId4"/>` with no rId4 in the presentation rels — a
schema-invalid ST_RelationshipId. `Validate` cannot catch either: `partExists`
consults `p.reader`, where the removed part still exists.

**C366 — xlsx opaque-sheet deletion.** `DeleteSheet` removes the part, its rels
and its content-type override, but `rebuildWorksheetRelationships` filters only
`RelTypeWorksheet`, so a chartsheet-typed rel survives; the resulting rel set is
then set-equivalent to the original, so `writeWorkbookRelationships` re-emits the
original bytes verbatim, still pointing at the absent part.

**C369 — charset bypass corrupts workbook.xml.** `UnmarshalWithSource`
deliberately refuses to register a source when the declared charset transcodes,
because decoder offsets then index transcoded bytes. But `openFromReader` sets
`wb.RawSource = data` unconditionally, and `CT_Workbook.UnmarshalXML` slices it
directly. A windows-1252 workbook.xml with non-ASCII before
`<workbookProtection>` saves as
`...appName="Caféééééééééé"/>rotection lockStructure="1"/><xr:revisi<xr:revisionPtr ...`
— malformed, emitted silently because `WriteRaw` bypasses balance checking. Since
workbook.xml is *always* regenerated, the corruption is deterministic. The
replayed prolog also keeps `encoding="windows-1252"` over UTF-8 content.

**C368 — implicit cell refs destroyed.** `CT_Row.MarshalToBuilder` sorts cells by
`cellRefColIndex(c.R)` and emits `r` unconditionally. Cells legally omitting `r`
key to 0 and move ahead of every explicit cell:
`<c r="A1">11</c><c>22</c><c>33</c>` becomes
`<c r="">22</c><c r="">33</c><c r="A1">11</c>` — values shifted columns, plus
schema-invalid empty refs. Note `marshalSheetData` deliberately uses
`SliceStable` for exactly this reason; its sibling does not.

**C429 — inline xmlns makes every save fail.** `CT_BookView.UnmarshalXML` skips
namespace declarations while still capturing the namespaced attr, so a
`workbookView` carrying its own `xmlns:xr2` opens fine and then makes
`SaveBytes` return `no prefix registered for namespace`. `CT_Sheet` and
`CT_ExtensionList` solved this with `InlineNSDecls`; `CT_BookView`'s bespoke
`attrOrder` predates the convention.

### 3.3 Silent data loss

**C370 — inert capture kit on docx feature parts.** `UnmarshalOrderedChildren`
preserves unknown children *only when the decoder has a registered source*;
without one they are skipped. `loadAllParts` parses styles.xml, comments.xml,
footnotes.xml, endnotes.xml and every header/footer with plain `xmlb.Unmarshal`
— only document.xml uses `UnmarshalWithSource`. So every `CT_RPr`/`CT_PPr`/
`CT_TblPr`/`CT_TcPr` capture in those parts is empty, and any mutation that
flips the part to regenerate deletes the unmodeled children of *every* element in
it. Three repros confirmed: a `w14:cntxtAlts` in Normal's rPr vanishes on
`AddParagraphStyle` (along with `mc:Ignorable` and the root `xmlns:w14`); a
header run's `SetText` drops it from the header; `AddFootnote` drops it from an
existing note. The same mechanism hits `Append`, which re-parses the rewritten
body with plain `xml.Unmarshal`. This is the single highest-leverage fix in the
report: one parse-call change per part closes a loss affecting the most common
feature flows.

**C371/C373 — WML range markup and field content deleted.** The four raw-child
whitelists (`isRawPChild`, `isRawBodyChild`, `isRawRowChild`, CT_Tbl's inline
list) have drifted independently. `commentRangeStart`/`End` are schema-valid at
body, table, row and cell level (Word emits them for whole-row/cell comments) and
are in none of them, so the comment survives but its anchor is destroyed.
Separately, `CT_SimpleField` passes nil slots for `ins`/`del`/`sdt`/perms, and
the typed `case` arms match those names *before* the raw fallback, so the nil
branch calls `d.Skip()` — a tracked insertion inside a simple field loses its
text and a tracked deletion is silently accepted. **Direction:** the two
containers that already capture everything unknown (CT_R, CT_SectPr) have no
such holes; make that the family-wide default.

**C374 — theme edits delete extensions.** `dml.Theme` models four children of
CT_OfficeStyleSheet; `custClrLst` and `extLst` are unmodeled, as are the
`extLst` children of five nested types. `docx.Document.Theme()` and
`xlsx.Workbook.Theme()` parse the real part into this struct and, once
`Modified()`, regenerate the whole part from it. Every Office 2013+ theme carries
`thm15:themeFamily` in an extLst, so a one-line `SetName` deletes it plus any
custom color list.

**C380 — editing a run restyles it.** `patchParagraphsInPlace` replaces a dirty
run wholesale with `runToOxml(dr)`, and `oxmlToColor` keeps only the base color
value. A run styled "Accent1, Lighter 40%" (`schemeClr accent1` +
`lumMod 60000` + `lumOff 40000`) becomes bare `accent1` after nothing more than
`run.SetText("edited")` — the text visibly changes color on screen. `lang`,
`spc`, `kern`, `cap` and rPr effects go with it. The file header comment claims
"everything the domain model does not represent is left untouched", which is
true only for *untouched* runs.

**C372 — image edits rebuild the drawing.** `parseDrawingImage` captures only
relID/altText/extent/floating; `updateDrawingXML` then replaces the entire
`RawContent` with a canonical template. On a `wp:anchor` image, `SetSize` resets
position to (0,0), turns `wrapSquare` into `wrapNone`, drops rotation, and emits
`<wp:docPr id="0"` — ECMA-invalid and duplicated across multiple edited images.
`TestHeaderHandleEditsPersist` exercises this path but asserts only the descr
string, so the id=0 emission is already happening under test.

**C378/C379 — pptx timing and group removal.** `walkCTns` omits `p:cmd`,
`p:video` and `p:audio`, so `timingMaxIDs` under-reports on any tree with media
— including the library's own autoplay tree — and `AddAnimation` allocates
colliding `cTn` ids (verified: `id="6"` and `id="7"` each emitted twice in one
`p:timing`). Separately, `collectRemovedPicRefs` matches only `ChildPic`, so
removing a *group* containing autoplay media leaves `p:timing` targeting a
deleted spid and leaks the media rels and part — the identical top-level case is
fixed and tested one level up.

### 3.4 Silently-ignored API calls

**C381** — `SetEmbedTrueTypeFonts(false)` emits nothing (only `== true` emits),
so `ReplayCapturedAttrs`'s unmatched-captured rule replays the source's `"1"`.
The public setter has no effect on any parsed deck. **C423** — a dozen xlsx
feature mutators return success on opaque sheets while `markDirty` discards the
change, and `AddImage`/`AddChart` attachments are skipped at save with no error;
`AddImage` also sets `s.dirty` directly, bypassing the opaque guard. **C428** —
`AddSparklineGroup` reallocates the backing slice, so setters on previously
returned handles mutate detached memory and `Delete()` silently no-ops. **C406**
— feature mutators never call `touch()`, so an `AddFootnote` on a hyperlink run
in a preserved header updates footnotes.xml but not the header: orphan note,
reference lost. **C519** — parsed-effect animation handles are reconstructions;
their documented chaining mutators change nothing. **C402** — `Properties` edits
are dropped entirely when the opened package has no core-properties part, though
`Create` writes them unconditionally.

### 3.5 Everything else

Mediums C390–C446 and lows C447–C582 are fully specified in §1 (location,
mechanism, status). They cluster as: **capture-convention gaps** (C411, C420,
C430, C431, C476, C481, C483, C523), **walker-divergence blind spots** (C405,
C412, C413, C495, C498, C507, C508), **validation absent at the boundary**
(C426, C435, C440, C441, C535, C538), **dead or unreachable code** (C448, C510,
C515, C522, C527, C554), and **doc/godoc drift** (C446, C460, C465, C505, C514,
C533, C579, C580).

---

## 4. Design tensions

**T-A — The validator validates the input, not the output.** All three formats'
`Validate()` resolve part existence against the *source* reader. Every
deletion-induced dangling reference in this report (C364, C365, C366, C424,
C407, C379) is therefore invisible to the pre-save gate by construction, and the
xlsx case additionally skips `validatePackage` entirely after `Close()`. The
gate's own contract says it "mirrors what the save path actually writes". *The
alternative:* have Validate run against the computed output set — the save path
already knows which parts it will write and which rels it will emit. This is one
change that closes an entire finding class across three packages, and it would
have caught C363 and C366 before they shipped.

**T-B — Regeneration moved from trees to leaves, and the fidelity contract
didn't follow.** The wave established "untouched content survives" and proved it
at part and tree level. But the unit of regeneration is now the *dirty leaf*: a
run, a paragraph's pPr, a connector's `a:ln`, a theme, a sparkline group, a
drawing. Each is rebuilt from a narrow domain model, so "touched content keeps
its unmodeled parts" is false — and several file-header comments claim the
stronger property (C380 quotes one). *The alternative:* patch-in-place rather
than regenerate — set the modeled fields on the existing parsed node and leave
its siblings alone. This is more invasive than adding capture fields, but adding
capture fields is what the last three audits did and the class keeps returning
in a new location (C374 is C244's shape one layer over).

**T-C — Capture-convention coverage is symptom-driven, so the gaps are
invisible until a wild file finds them.** `CapturedAttrs` landed on
`CT_RPrChange` but not its five siblings (C411); on `SequenceTimeNode` but not
`Iterate` (C420); on `SpLocks` but not `GrpSpLocks` (C483); on `cfRule` but not
its parent `conditionalFormatting` (C431); on worksheet types but on no styles
type at all (C430). Each was applied where a specific finding pointed. *The
alternative:* a per-type XSD diff — mechanically enumerate every modeled type's
attributes and children against the schema and assert either modeled-or-captured.
This is a generatable test, not a manual sweep, and it subsumes T-C, most of
C400's class, and the leniency inconsistencies (C552).

**T-D — Fidelity replay and model authority are in direct conflict, and replay
wins.** `ReplayCapturedAttrs`'s rule "a captured attribute with no modeled match
replays verbatim" is what makes explicit zeros survive — and simultaneously makes
*clearing* a parsed value impossible for any modeled zero suppressed by
omitempty. C381 is the API-reachable instance; `Placeholder.Idx→0`,
`CommonSlideData.Name→""` and `SlideLayout.Preserve→false` are the latent ones.
Every new capture adoption widens the trap. *The alternative:* make setters
invalidate the corresponding captured attribute (the machinery for this exists —
`RawAttrListOverride` already resets staleness), so "modeled wins" means modeled
wins including when the modeled value is a zero.

**T-E — Remediation bookkeeping became the source of truth and it failed.** C277
is listed in the prior audit's medium block and in its "all critical/high/medium
fixed" rollup, appears in no Findings→PR entry of §9, and the code is unchanged
— transitional percent forms still hard-fail `Open` on the live blip path
(C400). C343's presence-flag fix converted 4 of ~8 fields and pinned only the 4
it converted (C399). C127's overlap fix was applied to `SetColWidth` and not to
its 90%-duplicated twin `editColumn` (C383). *The alternative:* a fix is not
closed by a rollup row but by a test that fails on the unfixed code, and
class-shaped findings ("field X has the wrong type for its XML shape") need
table-driven tests enumerating the whole class — otherwise the CHANGELOG records
a class fix that was an instance fix.

---

## 5. Expectation gaps

**Affordance.** Expected `Encrypt`/`Decrypt` to mean "authenticated"; found
integrity optional, strippable, and never reported to the caller — `Decrypt`
never tells you which of agile-with-HMAC, agile-without, AES-ECB/SHA-1 or broken
RC4 produced the bytes. Expected `SignatureInfo.CoveredParts` to mean "parts this
signature protects"; found "parts any `<Object>` claims". Expected
`MaxDecompressedPackageSize` to bound decompression; found it bounds *charged*
bytes, which a caller can decouple. Expected a setter that returns no error to
have taken effect; found ~six APIs where it silently does not (§3.4). Expected
`Validate()` to gate what `Save` writes; found it gates what `Open` read.

**Missing capability the API shape implies.** No row/column insert/delete in
xlsx — yet `SheetProtection` exposes `InsertRows`/`DeleteRows` *lock flags* for
an operation the library cannot perform, and seven feature stores hold absolute
refs with no central registry should it ever land. No deletion APIs across the
board (no RemoveComment/RemoveFootnote/RemoveStyle/RemoveParagraph/RemoveRow/
RemoveHyperlink; sparklines' `Delete` and bibliography's `RemoveSource` are the
lone exceptions), so every replace-style edit accretes orphans. No numbering
`lvlOverride`/`startOverride` API, so "restart this list at 1" — the most-asked
numbering feature — is unexpressible though the model carries it. Content
controls and `CustomXMLParts` both exist but cannot be connected (no
`w:dataBinding` authoring). Filter predicates don't hide rows.

**Cross-format betrayal.** A user who learns one format is misled by the next:
encrypted open exists only in docx while xlsx/pptx godocs point at it (C386);
SVG auto-fallback exists in docx/xlsx but not pptx (C387); bad image bytes fail
fast in xlsx, never in docx/pptx — and pptx's godoc promises the xlsx behavior
(C441); alt-text is `SetAltText`/`SetDescription`/impossible (C442); `Theme()`
returns a different type in pptx (C571); the same impossible state panics in
docx and silently returns an empty model in xlsx and pptx (C568).

**Docs and DX.** The wave's behavior caveats all landed in godoc and none in the
guides (C579), the CHANGELOG has no record of 173 commits including a breaking
signature change (C388), and README/troubleshooting both describe a save-blocking
validation that is a warning (C446). CI advertises that it mirrors the local
merge gates while skipping fixtures, `-race` and fuzz entirely (C578). "All
green" carries little information: absent corpus skips, *empty* corpus passes
vacuously (C443), unmapped spec elements skip with 139/207 already skipping in
one suite and no ratchet (C445).

---

## 6. Open questions

1. **Is `dataIntegrity` genuinely optional in the wild?** C361's fix is
   straightforward if every real agile producer emits it. Office does; whether
   any third-party encryptor omits it determines whether the fix should hard-fail
   or require an explicit opt-out. Needs a corpus of non-Office encrypted files.
2. **Does PowerPoint repair or refuse the C363 output?** The deck is
   OPC-inconsistent (a `sldId` bound to a master rel), but PowerPoint's repair
   heuristics are undocumented. This changes the severity framing from "silent
   corruption" to "data loss on repair" — not the fix, which is required either
   way.
3. **Excel's tolerance for the C368 output.** Cells reordered with `r=""`: does
   Excel infer position from order, repair, or refuse? Determines whether this is
   corruption or drift.
4. **Whether Word tolerates the C371 anchor loss silently.** A comment whose
   range markers are gone but whose `commentReference` run survives — does Word
   drop the comment, anchor it to the reference run, or prompt?
5. **`AlgID=0` prevalence (C385).** The spec permits it; no fixture in the tree
   uses it. Whether any real producer emits it decides medium vs high.
6. **Theme `fillRef` matrix reordering (C401)** — confirmed to change which fill
   a shape resolves, but the visual impact depends on how different the
   interleaved fills are in real themes. Needs a rendered comparison.
7. **Is `common/vml` meant to be a writable model?** C398/C467/C468 are only
   defects if it is. Nothing outside spectest imports it, and xlsx builds VML
   from string templates — the honest resolution may be to mark it read/spec-only
   and delete its marshal path rather than fix three findings.
8. **Whether the harvest OOM design (C389) was ever exercised end-to-end.** The
   livelock is traced from systemd 255 semantics, not observed; a single capped
   run with a deliberately memory-hungry worker would settle it.
