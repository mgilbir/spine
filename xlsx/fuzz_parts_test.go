package xlsx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzbound"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Targeted secondary-part fuzzers.
//
// FuzzOpenXlsx mutates raw archive bytes, so nearly every mutation breaks the
// zip and the parsers behind the container are barely reached. The targets in
// this file take one valid workbook and replace exactly one part inside it,
// leaving the archive and every other part intact — the shape
// FuzzXlsxWorksheetXML uses, extended to the parts it does not cover: styles,
// shared strings, theme, tables, comments (classic and threaded), the pivot
// cache and pivot table parts, and xl/workbook.xml itself.
//
// None of them is a "did not panic" target. Three oracles are applied.
//
//  1. Errors are honest. OpenReader must return exactly one of a workbook or
//     an error, never a partial success and never (nil, nil).
//  2. The read-back is a fixed point after one round trip. Sanitizing hostile
//     input on the first save is legitimate; a package that keeps changing on
//     every subsequent save is a parser and a serializer disagreeing, which
//     rewrites the user's file a little more each time it is opened.
//  3. Parts that cannot legitimately be affected still read the same. A
//     corrupt theme cannot change a cell value; a corrupt shared string table
//     cannot change a cell that does not reference it; a corrupt styles part
//     can retype a numeric cell as a date (that is what number formats do) but
//     cannot touch a string, boolean, error or formula cell.
//
// The whole cycle runs inside a fuzzbound.Budget, so a malformed count
// attribute that becomes an allocation size is reported as a finding rather
// than as an out-of-memory kill nobody attributes to a parser.

// Part names inside the hand-written fixture package.
const (
	partSharedStrings = "xl/sharedStrings.xml"
	partStyles        = "xl/styles.xml"
	partTheme         = "xl/theme/theme1.xml"
	partTable         = "xl/tables/table1.xml"
	partComments      = "xl/comments1.xml"
	partWorkbook      = "xl/workbook.xml"
	partSheet1        = "xl/worksheets/sheet1.xml"

	partThreadedComments = "xl/threadedComments/threadedComment1.xml"
	partPersons          = "xl/persons/person1.xml"
	partPivotCache       = "xl/pivotCache/pivotCacheDefinition1.xml"
	partPivotTable       = "xl/pivotTables/pivotTable1.xml"
)

// partsBudget bounds one open/save/reopen/save cycle over a mutated package.
// It catches the failure mode this package has actually shipped: a count or
// size attribute used unvalidated as an allocation length.
//
// The floor is calibrated against the most expensive input the target actually
// runs, not against the pristine fixture. That distinction is the whole reason
// the number moved: the fixture costs about 0.5 MiB, and 24 MiB looked like
// two orders of magnitude of headroom against it — but a seed in this very
// corpus declares 500 sheets, and opening and re-saving that costs 31.5 MiB,
// which is 0.995 of what the old floor allowed. It passed by 124 KiB.
//
// A margin that thin is not a bound, it is a coin flip. The nightly race job
// called it: the race detector adds about 4% on this path, so the seed measured
// 32.8 MiB there and failed every night while every plain run passed. A Go
// runtime change or an incidental allocation anywhere in open/save would have
// done the same.
//
// 48 MiB puts that seed at 0.55 of the allowance (0.58 under -race) and leaves
// every other seed below 0.35. Nothing is lost by the raise: the bug this
// guards against is an attribute-driven allocation, which overshoots by orders
// of magnitude — C360's was a 512-byte input asking for 16 GiB — not by the
// tens of percent this headroom absorbs.
var partsBudget = fuzzbound.Budget{
	What:              "opening and re-saving a workbook with one mutated part",
	Bytes:             48 << 20,
	BytesPerInputByte: 1024,
	Time:              5 * time.Second,
	TimePerMiB:        10 * time.Second,
}

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

const fixtureContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Default Extension="vml" ContentType="application/vnd.openxmlformats-officedocument.vmlDrawing"/>` +
	`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
	`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
	`<Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
	`<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>` +
	`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` +
	`<Override PartName="/xl/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
	`<Override PartName="/xl/tables/table1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.table+xml"/>` +
	`<Override PartName="/xl/comments1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.comments+xml"/>` +
	`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
	`</Types>`

const fixturePackageRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
	`</Relationships>`

const fixtureCoreProps = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
	`xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" ` +
	`xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
	`<dc:title>Parts fixture</dc:title>` +
	`<dcterms:modified xsi:type="dcterms:W3CDTF">2001-02-03T04:05:06Z</dcterms:modified></cp:coreProperties>`

const fixtureWorkbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
	`<sheets><sheet name="Data" sheetId="1" r:id="rId1"/><sheet name="Notes" sheetId="2" r:id="rId2"/></sheets>` +
	`<definedNames><definedName name="Anchor">Data!$A$1</definedName></definedNames>` +
	`</workbook>`

const fixtureWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>` +
	`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
	`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>` +
	`<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>` +
	`</Relationships>`

// fixtureSharedStrings holds five items, one of them rich text, so the
// concatenating branch of buildStringTable is live as well as the plain one.
const fixtureSharedStrings = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="8" uniqueCount="5">` +
	`<si><t>Alpha</t></si>` +
	`<si><t>Beta</t></si>` +
	`<si><r><rPr><b/></rPr><t>Gam</t></r><r><t>ma</t></r></si>` +
	`<si><t>Delta</t></si>` +
	`<si><t xml:space="preserve"> Epsilon </t></si>` +
	`</sst>`

const fixtureStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<numFmts count="2"><numFmt numFmtId="164" formatCode="yyyy\-mm\-dd"/><numFmt numFmtId="165" formatCode="0.000"/></numFmts>` +
	`<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="14"/><name val="Cambria"/></font></fonts>` +
	`<fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>` +
	`<fill><patternFill patternType="solid"><fgColor rgb="FFFFFF00"/><bgColor indexed="64"/></patternFill></fill></fills>` +
	`<borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border>` +
	`<border><left style="thin"><color indexed="64"/></left><right/><top/><bottom style="double"/><diagonal/></border></borders>` +
	`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
	`<cellXfs count="4">` +
	`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
	`<xf numFmtId="165" fontId="1" fillId="2" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1"/>` +
	`<xf numFmtId="14" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>` +
	`<xf numFmtId="9" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"><alignment horizontal="center" wrapText="1"/></xf>` +
	`</cellXfs>` +
	`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
	`<dxfs count="1"><dxf><font><color rgb="FF9C0006"/></font><fill><patternFill><bgColor rgb="FFFFC7CE"/></patternFill></fill></dxf></dxfs>` +
	`<tableStyles count="0" defaultTableStyle="TableStyleMedium2" defaultPivotStyle="PivotStyleLight16"/>` +
	`</styleSheet>`

const fixtureTheme = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">` +
	`<a:themeElements><a:clrScheme name="Office">` +
	`<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>` +
	`<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
	`<a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2>` +
	`<a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2>` +
	`<a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4>` +
	`<a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6>` +
	`<a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink>` +
	`</a:clrScheme><a:fontScheme name="Office">` +
	`<a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
	`<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>` +
	`</a:fontScheme><a:fmtScheme name="Office">` +
	`<a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>` +
	`<a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>` +
	`<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>` +
	`<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>` +
	`</a:fmtScheme></a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`

// fixtureSheet1 deliberately mixes cell kinds. A1/B1/B2 and the table header
// row read through the shared string table, C1 is an inline string, D1/E1/F1
// are boolean/error/formula, and the numeric cells carry style indices —
// including one whose number format is a date, the only documented way
// styles.xml can change a cell's Type.
const fixtureSheet1 = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
	`<dimension ref="A1:H6"/>` +
	`<sheetData>` +
	`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>2</v></c>` +
	`<c r="C1" t="inlineStr" s="3"><is><t>Inline</t></is></c>` +
	`<c r="D1" t="b" s="3"><v>1</v></c><c r="E1" t="e" s="3"><v>#DIV/0!</v></c>` +
	`<c r="F1" t="str" s="3"><f>CONCATENATE("a","b")</f><v>ab</v></c>` +
	`<c r="G1" s="1"><v>1234.5</v></c><c r="H1" s="2"><v>45000</v></c></row>` +
	`<row r="2"><c r="A2" s="3"><v>0.25</v></c><c r="B2" t="s" s="3"><v>4</v></c></row>` +
	`<row r="3"><c r="A3" t="s"><v>0</v></c><c r="B3" t="s"><v>1</v></c><c r="C3" t="s"><v>3</v></c></row>` +
	`<row r="4"><c r="A4"><v>1</v></c><c r="B4"><v>2</v></c><c r="C4"><v>3</v></c></row>` +
	`<row r="5"><c r="A5"><v>4</v></c><c r="B5"><v>5</v></c><c r="C5"><v>6</v></c></row>` +
	`<row r="6"><c r="A6"><v>7</v></c><c r="B6"><v>8</v></c><c r="C6"><v>9</v></c></row>` +
	`</sheetData>` +
	`<mergeCells count="1"><mergeCell ref="E3:E4"/></mergeCells>` +
	`<legacyDrawing r:id="rId3"/>` +
	`<tableParts count="1"><tablePart r:id="rId1"/></tableParts>` +
	`</worksheet>`

const fixtureSheet1Rels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/table" Target="../tables/table1.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="../comments1.xml"/>` +
	`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/vmlDrawing" Target="../drawings/vmlDrawing1.vml"/>` +
	`</Relationships>`

const fixtureSheet2 = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<sheetData><row r="1"><c r="A1" t="s"><v>1</v></c></row></sheetData></worksheet>`

const fixtureTable = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="1" name="SalesTable" ` +
	`displayName="SalesTable" ref="A3:C6" totalsRowShown="0">` +
	`<autoFilter ref="A3:C6"/>` +
	`<tableColumns count="3">` +
	`<tableColumn id="1" name="Alpha"/><tableColumn id="2" name="Beta"/><tableColumn id="3" name="Delta"/>` +
	`</tableColumns>` +
	`<tableStyleInfo name="TableStyleMedium2" showFirstColumn="0" showLastColumn="0" showRowStripes="1" showColumnStripes="0"/>` +
	`</table>`

const fixtureComments = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<authors><author>Reviewer</author><author>Second Reviewer</author></authors>` +
	`<commentList>` +
	`<comment ref="E1" authorId="0"><text><r><rPr><b/></rPr><t>Check </t></r><r><t>this cell</t></r></text></comment>` +
	`<comment ref="D2" authorId="1"><text><t>Plain note</t></text></comment>` +
	`</commentList></comments>`

const fixtureVML = `<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" ` +
	`xmlns:x="urn:schemas-microsoft-com:office:excel">` +
	`<o:shapelayout v:ext="edit"><o:idmap v:ext="edit" data="1"/></o:shapelayout>` +
	`<v:shapetype id="_x0000_t202" coordsize="21600,21600" o:spt="202" path="m,l,21600r21600,l21600,xe">` +
	`<v:stroke joinstyle="miter"/><v:path gradientshapeok="t" o:connecttype="rect"/></v:shapetype>` +
	`<v:shape id="_x0000_s1025" type="#_x0000_t202" style="position:absolute;visibility:hidden" fillcolor="#ffffe1">` +
	`<v:fill color2="#ffffe1"/><v:shadow on="t" color="black" obscured="t"/><v:path o:connecttype="none"/>` +
	`<x:ClientData ObjectType="Note"><x:MoveWithCells/><x:SizeWithCells/><x:AutoFill>False</x:AutoFill>` +
	`<x:Row>0</x:Row><x:Column>4</x:Column></x:ClientData>` +
	`</v:shape></xml>`

// buildXlsxPartsFixture assembles the workbook the styles, shared-string,
// theme, table and workbook targets mutate. It is hand-written rather than
// produced by Create() on purpose: a created workbook writes its strings
// inline and carries no xl/sharedStrings.xml, no xl/styles.xml and no
// xl/theme/theme1.xml at all, so it cannot exercise any of those parsers. A
// substitution made against one would match nothing and pass vacuously.
func buildXlsxPartsFixture() []byte {
	return fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", fixtureContentTypes},
		{"_rels/.rels", fixturePackageRels},
		{"docProps/core.xml", fixtureCoreProps},
		{partWorkbook, fixtureWorkbook},
		{"xl/_rels/workbook.xml.rels", fixtureWorkbookRels},
		{partSharedStrings, fixtureSharedStrings},
		{partStyles, fixtureStyles},
		{partTheme, fixtureTheme},
		{partSheet1, fixtureSheet1},
		{"xl/worksheets/_rels/sheet1.xml.rels", fixtureSheet1Rels},
		{"xl/worksheets/sheet2.xml", fixtureSheet2},
		{partTable, fixtureTable},
		{partComments, fixtureComments},
		{"xl/drawings/vmlDrawing1.vml", fixtureVML},
	})
}

// buildXlsxCommentsFixture builds a package carrying a threaded comment with a
// reply, which produces the classic comments part, the threadedComments part
// and the workbook-level persons part together. Those three are written by the
// library and are tedious and error-prone to hand-write consistently.
func buildXlsxCommentsFixture(tb testing.TB) []byte {
	tb.Helper()
	w := Create()
	s, err := w.AddSheet("Data")
	if err != nil {
		tb.Fatalf("building comments fixture: %v", err)
	}
	for _, kv := range []struct {
		ref string
		val any
	}{{"A1", "alpha"}, {"B1", 7}, {"C1", true}, {"A2", "beta"}} {
		if err := s.SetCellValue(kv.ref, kv.val); err != nil {
			tb.Fatalf("building comments fixture: %v", err)
		}
	}
	root := s.AddComment("B2", "Reviewer", "please check")
	root.Reply("Second Reviewer", "checked")
	s.AddComment("C3", "Third Reviewer", "unrelated note")

	// A save stamps dcterms:modified from the wall clock at second resolution,
	// so two builds of this fixture are byte-identical only when they land in
	// the same second — a coin flip weighted by how long a build takes. At
	// ~1.5ms locally that is invisible; under -race and coverage instrumentation
	// it is frequent enough to have failed the nightly of 2026-08-06 in both the
	// race job and the xlsx fuzz job, through the byte-stability assertion in
	// assertCommentsFixture.
	//
	// An explicit assignment is a property edit in its own right and stampModified
	// leaves it alone, which is what makes this the fix rather than rewriting the
	// saved core.xml afterwards. The three comment parts below are pinned for the
	// same reason: a fuzz fixture that is not byte-stable cannot be reproduced
	// from a crasher.
	//
	// synctest, which pins the clock for the modified-stamping tests and for
	// TestSaveBytesIsIdempotent, cannot be used here: synctest.Test takes a
	// *testing.T and this runs in fuzz-target setup holding a *testing.F.
	w.Properties.Created = fuzzseed.FixtureModified
	w.Properties.Modified = fuzzseed.FixtureModified

	out, err := w.SaveBytes()
	if err != nil {
		tb.Fatalf("building comments fixture: %v", err)
	}

	// The three comment parts are then rewritten with fixed content. Two
	// reasons, both necessary.
	//
	// The library stamps a fresh GUID and the wall-clock time into every
	// threaded comment and person it writes, so the package differs on every
	// run. A fuzz fixture that is not byte-stable cannot be reproduced from a
	// crasher, and the corpus entries it accumulates describe a package that no
	// longer exists.
	//
	// And a legacy note is only reported when no threaded comment covers the
	// same cell, so a package built purely through AddComment shadows every
	// legacy comment it writes: replacing xl/comments1.xml in one would have
	// no observable effect at all. D4 below is covered by no thread, which is
	// what puts the classic comment parser in front of the fuzzer.
	fixed := fuzzseed.EditZip(out, [][2]string{
		{partPersons, fixturePersons},
		{partThreadedComments, fixtureThreadedComments},
		{partComments, fixtureLegacyComments},
	})
	if fixed == nil {
		tb.Fatal("building comments fixture: the saved package is not a readable archive")
	}
	return fixed
}

const fixturePersons = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<personList xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments" ` +
	`xmlns:x="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<person displayName="Reviewer" id="{00000000-0000-0000-0000-0000000000A1}" providerId="None"/>` +
	`<person displayName="Second Reviewer" id="{00000000-0000-0000-0000-0000000000A2}" providerId="None"/>` +
	`</personList>`

const fixtureThreadedComments = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<ThreadedComments xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments">` +
	`<threadedComment ref="B2" dT="2001-02-03T04:05:06Z" personId="{00000000-0000-0000-0000-0000000000A1}" ` +
	`id="{00000000-0000-0000-0000-0000000000T1}"><text>please check</text></threadedComment>` +
	`<threadedComment ref="B2" dT="2001-02-03T04:05:07Z" personId="{00000000-0000-0000-0000-0000000000A2}" ` +
	`id="{00000000-0000-0000-0000-0000000000T2}" parentId="{00000000-0000-0000-0000-0000000000T1}">` +
	`<text>checked</text></threadedComment></ThreadedComments>`

// fixtureLegacyComments shadows B2 (a threaded comment covers it) and leaves
// D4 unshadowed, so the classic comment parser has one comment of its own to
// report.
const fixtureLegacyComments = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<authors><author>Reviewer</author><author>Second Reviewer</author><author>Third Reviewer</author></authors>` +
	`<commentList>` +
	`<comment ref="B2" authorId="0" shapeId="1025"><text><t>please check</t></text></comment>` +
	`<comment ref="D4" authorId="2" shapeId="1026"><text><r><rPr><b/></rPr><t>legacy </t></r><r><t>note</t></r></text></comment>` +
	`</commentList></comments>`

// buildXlsxPivotFixture builds a package with a real pivot cache definition,
// cache records and pivot table part over a small source range.
func buildXlsxPivotFixture(tb testing.TB) []byte {
	tb.Helper()
	w := Create()
	src, err := w.AddSheet("Data")
	if err != nil {
		tb.Fatalf("building pivot fixture: %v", err)
	}
	for c, header := range []string{"Region", "Sales", "Cost"} {
		if err := src.SetCellValue(FormatCellRef(1, c+1), header); err != nil {
			tb.Fatalf("building pivot fixture: %v", err)
		}
	}
	for r := 2; r <= 5; r++ {
		if err := src.SetCellValue(FormatCellRef(r, 1), fmt.Sprintf("R%d", r%2)); err != nil {
			tb.Fatalf("building pivot fixture: %v", err)
		}
		if err := src.SetCellValue(FormatCellRef(r, 2), r*10); err != nil {
			tb.Fatalf("building pivot fixture: %v", err)
		}
		if err := src.SetCellValue(FormatCellRef(r, 3), r*3); err != nil {
			tb.Fatalf("building pivot fixture: %v", err)
		}
	}
	dst, err := w.AddSheet("Pivot")
	if err != nil {
		tb.Fatalf("building pivot fixture: %v", err)
	}
	if _, err := dst.AddPivotTable("Data!A1:C5", "A1", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Sales"}},
	}); err != nil {
		tb.Fatalf("building pivot fixture: %v", err)
	}
	// Pinned so the fixture is byte-stable across builds; a fixture that moves
	// cannot be reproduced from a crasher. See fuzzseed.FixtureModified.
	w.Properties.Created = fuzzseed.FixtureModified
	w.Properties.Modified = fuzzseed.FixtureModified

	out, err := w.SaveBytes()
	if err != nil {
		tb.Fatalf("building pivot fixture: %v", err)
	}
	return out
}

// ---------------------------------------------------------------------------
// Fixture self-checks
// ---------------------------------------------------------------------------

// sharedStringCellIndex maps every fixture cell whose value is reached through
// xl/sharedStrings.xml to the index it holds, i.e. every t="s" cell in
// fixtureSheet1 and fixtureSheet2 and its <v>. The shared-string target
// excludes exactly these cells from its unchanged-parts oracle — they are the
// ones the mutated part is allowed to change — and holds them to the stricter
// oracle in assertSharedStringResolution instead.
var sharedStringCellIndex = map[string]int{
	"Data!A1": 0, "Data!B1": 2, "Data!B2": 4,
	"Data!A3": 0, "Data!B3": 1, "Data!C3": 3,
	"Notes!A1": 1,
}

func sharedStringCellRefs() []string {
	refs := make([]string, 0, len(sharedStringCellIndex))
	for ref := range sharedStringCellIndex {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}

// assertPartsFixture fails the fuzz setup unless the fixture really contains,
// and really reads back through, every construct the targets depend on.
//
// This exists because the obvious way to build a fixture — Create() plus
// SetCellValue — produces a workbook with no shared string table, no theme and
// no styles part. Probing shared-string handling against such a package
// substitutes into a part that is not there: every case passes and nothing has
// been tested. The assertions below fail loudly instead.
func assertPartsFixture(tb testing.TB, pkg []byte) {
	tb.Helper()

	for _, part := range []string{partSharedStrings, partStyles, partTheme, partTable, partComments, partSheet1, partWorkbook} {
		if fuzzseed.ZipEntry(pkg, part) == nil {
			tb.Fatalf("fixture has no %s", part)
		}
	}
	sheets := string(fuzzseed.ZipEntry(pkg, partSheet1)) + string(fuzzseed.ZipEntry(pkg, "xl/worksheets/sheet2.xml"))
	for _, construct := range []string{`t="s"`, `t="inlineStr"`, `t="b"`, `t="e"`, `<f>`, `s="1"`, `<tableParts`} {
		if !strings.Contains(sheets, construct) {
			tb.Fatalf("fixture worksheets do not contain %s", construct)
		}
	}
	// The exclusion list the shared-string oracle relies on must stay in step
	// with the fixture: one t="s" cell per listed reference, no more, no less.
	if got := strings.Count(sheets, `t="s"`); got != len(sharedStringCellIndex) {
		tb.Fatalf("fixture has %d t=\"s\" cells but sharedStringCellIndex lists %d", got, len(sharedStringCellIndex))
	}

	w, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		tb.Fatalf("fixture does not open: %v", err)
	}
	defer func() { _ = w.Close() }()

	if len(w.stringTable) != 5 {
		tb.Fatalf("fixture string table has %d entries, want 5", len(w.stringTable))
	}
	if w.stylesheet == nil {
		tb.Fatal("fixture stylesheet did not parse")
	}
	if w.Theme() == nil {
		tb.Fatal("fixture theme did not parse")
	}
	// The theme must reach the snapshot, or the theme target is looking at a
	// part nothing reads.
	if got := bookSnapshot(w)["theme"]; !strings.Contains(got, "Office Theme") || !strings.Contains(got, "Calibri") {
		tb.Fatalf("fixture theme reads back as %q, want the Office theme with its Calibri font scheme", got)
	}
	open := w.Sheets()
	if len(open) != 2 {
		tb.Fatalf("fixture has %d sheets, want 2", len(open))
	}
	// The shared-string indirection must actually resolve, through both the
	// plain and the rich-text branch of the string table.
	for _, want := range []struct{ ref, text string }{{"A1", "Alpha"}, {"B1", "Gamma"}, {"B2", " Epsilon "}} {
		c, err := open[0].Cell(want.ref)
		if err != nil {
			tb.Fatalf("fixture cell %s: %v", want.ref, err)
		}
		if got := c.String(); got != want.text {
			tb.Fatalf("fixture cell %s reads %q through the shared string table, want %q", want.ref, got, want.text)
		}
	}
	// The style index must actually reach a number format: H1 carries s="2",
	// whose xf sets numFmtId="14", so it must read back as a date and not as a
	// bare number. Without this the styles target could be pointed at a part
	// nothing consults.
	h1, err := open[0].Cell("H1")
	if err != nil {
		tb.Fatalf("fixture cell H1: %v", err)
	}
	if h1.Type() != CellTypeDate {
		tb.Fatalf("fixture cell H1 is %v, want %v: the style index does not reach a number format", h1.Type(), CellTypeDate)
	}
	// C1, D1, E1, F1 and B2 all carry a style index too (s="3", a percentage
	// format). They are the reason the styles target can catch a wrong-output
	// defect and not only a crash: a stylesheet that made a number format
	// retype a cell whatever its declared type would move these out of the
	// types they must keep, and they sit inside the unchanged-parts oracle.
	c1, err := open[0].Cell("C1")
	if err != nil {
		tb.Fatalf("fixture cell C1: %v", err)
	}
	if c1.Type() != CellTypeString {
		tb.Fatalf("fixture cell C1 is %v, want %v", c1.Type(), CellTypeString)
	}
	if got := len(open[0].Tables()); got != 1 {
		tb.Fatalf("fixture sheet has %d tables, want 1", got)
	}
	if got := len(open[0].Comments()); got != 2 {
		tb.Fatalf("fixture sheet has %d comments, want 2", got)
	}
	names := w.DefinedNames()
	if len(names) != 1 || names[0].Name != "Anchor" {
		tb.Fatalf("fixture workbook defined names = %+v, want one named Anchor", names)
	}
}

// assertCommentsFixture checks the API-built comments package really carries
// all three comment parts and that the threaded comment and its reply read
// back.
func assertCommentsFixture(tb testing.TB, pkg []byte) {
	tb.Helper()
	for _, part := range []string{partComments, partThreadedComments, partPersons} {
		if fuzzseed.ZipEntry(pkg, part) == nil {
			tb.Fatalf("comments fixture has no %s", part)
		}
	}
	w, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		tb.Fatalf("comments fixture does not open: %v", err)
	}
	defer func() { _ = w.Close() }()
	comments := w.Sheets()[0].Comments()
	if len(comments) != 2 {
		tb.Fatalf("comments fixture has %d top-level comments, want 2", len(comments))
	}
	byRef := map[string]*Comment{}
	for _, c := range comments {
		byRef[c.Ref()] = c
	}
	threaded, ok := byRef["B2"]
	if !ok || !threaded.Threaded() {
		tb.Fatalf("comments fixture has no threaded comment at B2 (%+v)", byRef)
	}
	if len(threaded.Replies()) != 1 {
		tb.Fatalf("the threaded comment at B2 has %d replies, want 1", len(threaded.Replies()))
	}
	// The legacy note is the reason the classic comments part is observable at
	// all: a threaded comment on the same cell would shadow it.
	legacy, ok := byRef["D4"]
	if !ok || legacy.Threaded() {
		tb.Fatalf("comments fixture has no classic (unthreaded) comment at D4 (%+v)", byRef)
	}
	if legacy.Author() != "Third Reviewer" || legacy.Text() != "legacy note" {
		tb.Fatalf("the classic comment at D4 reads %q/%q, want \"Third Reviewer\"/\"legacy note\"", legacy.Author(), legacy.Text())
	}
	if second := buildXlsxCommentsFixture(tb); !bytes.Equal(pkg, second) {
		tb.Fatal("the comments fixture is not byte-stable across two builds; a crasher found against it could not be reproduced")
	}
}

// assertPivotFixture checks the API-built pivot package really carries the
// cache definition and pivot table parts and that the pivot reads back.
func assertPivotFixture(tb testing.TB, pkg []byte) {
	tb.Helper()
	for _, part := range []string{partPivotCache, partPivotTable} {
		if fuzzseed.ZipEntry(pkg, part) == nil {
			tb.Fatalf("pivot fixture has no %s", part)
		}
	}
	w, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		tb.Fatalf("pivot fixture does not open: %v", err)
	}
	defer func() { _ = w.Close() }()
	pivots := w.PivotTables()
	if len(pivots) != 1 {
		tb.Fatalf("pivot fixture has %d pivot tables, want 1", len(pivots))
	}
	if got := pivots[0].RowFields(); len(got) != 1 || got[0] != "Region" {
		tb.Fatalf("pivot fixture row fields = %v, want [Region]", got)
	}
	if got := pivots[0].ValueFields(); len(got) != 1 || got[0].Field != "Sales" {
		tb.Fatalf("pivot fixture value fields = %+v, want one over Sales", got)
	}
}

// ---------------------------------------------------------------------------
// Oracles
// ---------------------------------------------------------------------------

// bookSnapshot renders everything the targets assert on: sheet names, per-cell
// type/value/formula, table and comment summaries and pivot table summaries.
// The grid walk is capped so a hostile dimension cannot turn the oracle itself
// into the slow path being measured.
func bookSnapshot(w *Workbook) map[string]string {
	snap := make(map[string]string)
	sheets := w.Sheets()
	names := make([]string, 0, len(sheets))
	for _, s := range sheets {
		names = append(names, s.Name())
	}
	snap["sheets"] = strings.Join(names, "|")

	for _, s := range sheets {
		prefix := s.Name() + "!"
		snap[prefix+"dims"] = fmt.Sprintf("%dx%d", s.Rows(), s.Cols())
		rows, cols := s.Rows(), s.Cols()
		if rows > 16 {
			rows = 16
		}
		if cols > 12 {
			cols = 12
		}
		for r := 1; r <= rows; r++ {
			for c := 1; c <= cols; c++ {
				key := prefix + FormatCellRef(r, c)
				cell, err := s.CellByRowCol(r, c)
				if err != nil {
					snap[key] = "error: " + err.Error()
					continue
				}
				snap[key] = fmt.Sprintf("t%d|%v|%s", cell.Type(), cell.Value(), cell.Formula())
			}
		}
		var tables []string
		for _, t := range s.Tables() {
			tables = append(tables, fmt.Sprintf("%s@%s%+v", t.Name(), t.Range(), t.Columns()))
		}
		snap[prefix+"tables"] = strings.Join(tables, ";")
		var comments []string
		for _, c := range s.Comments() {
			comments = append(comments, fmt.Sprintf("%s/%s/%s/%d", c.Ref(), c.Author(), c.Text(), len(c.Replies())))
		}
		snap[prefix+"comments"] = strings.Join(comments, ";")
	}

	// The theme is read here so that the theme target actually reaches the
	// theme parser: nothing on the open/save path parses xl/theme/theme1.xml
	// on its own — an untouched theme round-trips from its preserved bytes —
	// so without this a theme fuzzer would only ever confirm that opaque bytes
	// are copied through, which is not what it is for. Reading a theme must
	// also not mark it modified, so doing it between the two saves puts that
	// under the byte-equality oracle as well.
	if th := w.Theme(); th == nil {
		snap["theme"] = "<none>"
	} else {
		desc := "name=" + th.Name()
		if cs := th.ColorScheme(); cs != nil {
			desc += fmt.Sprintf(" clr=%s dk1=%+v lt1=%+v a1=%+v a6=%+v hl=%+v",
				cs.Name(), cs.Dark1(), cs.Light1(), cs.Accent1(), cs.Accent6(), cs.Hyperlink())
		}
		if fs := th.FontScheme(); fs != nil {
			desc += fmt.Sprintf(" font=%s major=%s", fs.Name(), fs.MajorLatin())
		}
		snap["theme"] = desc
	}

	var pivots []string
	for _, p := range w.PivotTables() {
		pivots = append(pivots, fmt.Sprintf("%s@%s r%v c%v v%+v", p.Name(), p.Location(), p.RowFields(), p.ColumnFields(), p.ValueFields()))
	}
	snap["pivots"] = strings.Join(pivots, ";")
	return snap
}

// roundTrip is one open/save/reopen/save cycle over pkg. It returns only
// values: nothing in here fails the test, so it is safe to repeat under
// fuzzbound.Budget.Check, which measures fn twice on a near miss.
type roundTrip struct {
	opened    bool
	dishonest string // non-empty when OpenReader broke its contract
	// refusedWrite records a first-save error. Save validates the model and
	// declines to write a workbook it believes is invalid (no sheets, duplicate
	// sheet ids, a defined name scoped to a sheet that does not exist), so for a
	// package assembled around a corrupt part this is a legitimate outcome —
	// the same class of outcome as refusing to open it.
	refusedWrite error
	// broken records a failure the library cannot excuse: its own output would
	// not reopen, or would not re-save.
	broken     error
	firstSave  []byte
	secondSave []byte
	firstSnap  map[string]string
	secondSnap map[string]string
}

func runRoundTrip(pkg []byte) roundTrip {
	var rt roundTrip

	w, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	switch {
	case err != nil && w != nil:
		rt.dishonest = fmt.Sprintf("OpenReader returned both a workbook and the error %v", err)
		_ = w.Close()
		return rt
	case err == nil && w == nil:
		rt.dishonest = "OpenReader returned a nil workbook and a nil error"
		return rt
	case err != nil:
		return rt
	}
	rt.opened = true
	defer func() { _ = w.Close() }()

	first, err := w.SaveBytes()
	if err != nil {
		rt.refusedWrite = err
		return rt
	}
	rt.firstSave = first

	once, err := OpenReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		rt.broken = fmt.Errorf("a package this library just wrote does not reopen: %w", err)
		return rt
	}
	defer func() { _ = once.Close() }()
	rt.firstSnap = bookSnapshot(once)

	second, err := once.SaveBytes()
	if err != nil {
		rt.broken = fmt.Errorf("re-saving a package this library just wrote and reopened failed: %w", err)
		return rt
	}
	rt.secondSave = second

	twice, err := OpenReader(bytes.NewReader(second), int64(len(second)))
	if err != nil {
		rt.broken = fmt.Errorf("the twice-saved package does not reopen: %w", err)
		return rt
	}
	defer func() { _ = twice.Close() }()
	rt.secondSnap = bookSnapshot(twice)

	return rt
}

// exercisePart applies the three oracles to a package with one part replaced.
// protected holds bookSnapshot keys whose values the mutated part cannot
// legitimately influence, mapped to their value in the pristine fixture; an
// empty map means the mutated part may change anything.
func exercisePart(t *testing.T, pkg []byte, protected map[string]string) map[string]string {
	t.Helper()

	var rt roundTrip
	partsBudget.Check(t, len(pkg), func() { rt = runRoundTrip(pkg) })

	if rt.dishonest != "" {
		t.Fatalf("%s", rt.dishonest)
	}
	if !rt.opened || rt.refusedWrite != nil {
		// Refusing to open, or opening and then declining to write, are both
		// legitimate outcomes for a package built around a corrupt part. What
		// is not legitimate is a silent partial success, which the dishonest
		// check above already covers.
		return nil
	}
	if rt.broken != nil {
		t.Fatalf("%v", rt.broken)
	}

	if !bytes.Equal(rt.firstSave, rt.secondSave) {
		t.Fatalf("the package is not a fixed point after one save: %s",
			describePackageDrift(rt.firstSave, rt.secondSave))
	}
	if diff := snapshotDiff(rt.firstSnap, rt.secondSnap); diff != "" {
		t.Fatalf("the read-back is not a fixed point after one round trip:\n%s", diff)
	}
	for key, want := range protected {
		got, ok := rt.firstSnap[key]
		if !ok {
			t.Fatalf("a corrupt part removed %s, which it cannot affect (was %q)", key, want)
		}
		if got != want {
			t.Fatalf("a corrupt part changed %s, which it cannot affect: got %q, want %q", key, got, want)
		}
	}
	return rt.firstSnap
}

// assertSharedStringResolution holds every t="s" cell in the fixture to the
// string table the mutated part actually declares.
//
// The unchanged-parts oracle cannot cover these cells: the mutated part is
// exactly what supplies their values, so they are excluded from it. That would
// leave the most interesting cells in the shared-string target checked only for
// "the value stopped changing", which a consistently wrong index satisfies. So
// the expected table is re-derived here from the same parsed model the library
// reads, by an independent eight-line projection: a defect in the
// index-to-string mapping (an off-by-one, a branch that prefers the runs over
// the text, a bounds check that lets a stale value through) shows up as a
// disagreement, while a defect in the XML parsing itself is left to the other
// oracles.
func assertSharedStringResolution(t *testing.T, part []byte, snap map[string]string) {
	t.Helper()
	if snap == nil {
		return
	}
	var sst oxml.CT_Sst
	if err := xmlb.Unmarshal(part, &sst); err != nil {
		// A part the library could not have parsed either; Open would have
		// failed and the round trip already returned.
		return
	}
	table := make([]string, len(sst.Si))
	for i := range sst.Si {
		si := &sst.Si[i]
		if si.T != nil {
			table[i] = *si.T
			continue
		}
		var sb strings.Builder
		for j := range si.R {
			sb.WriteString(si.R[j].T)
		}
		table[i] = sb.String()
	}

	for _, ref := range sharedStringCellRefs() {
		idx := sharedStringCellIndex[ref]
		want := ""
		if idx >= 0 && idx < len(table) {
			want = table[idx]
		}
		got, ok := snap[ref]
		if !ok {
			t.Fatalf("shared-string cell %s disappeared from the workbook", ref)
		}
		if expect := fmt.Sprintf("t%d|%s|", CellTypeString, want); got != expect {
			t.Fatalf("shared-string cell %s reads %q but index %d of the declared table is %q (table has %d items)",
				ref, got, idx, want, len(table))
		}
	}
}

// assertThemeMatchesPart holds the theme the library reports to the theme part
// it was told to read. The unchanged-parts oracle excludes the theme summary
// for this target — the mutated part is what defines it — so without this the
// theme target would check only that the summary stopped changing, which a
// theme read from the wrong part or from nothing at all satisfies.
func assertThemeMatchesPart(t *testing.T, part []byte, snap map[string]string) {
	t.Helper()
	if snap == nil {
		return
	}
	var theme dml.Theme
	if err := xmlb.Unmarshal(part, &theme); err != nil {
		// The library reads the part with the same call, so it could not have
		// parsed it either and reports no theme.
		return
	}
	got := snap["theme"]
	if got == "<none>" {
		t.Fatalf("the theme part parses as %q but the workbook reports no theme at all", theme.Name)
	}
	if want := "name=" + theme.Name; !strings.HasPrefix(got, want) {
		t.Fatalf("the theme reads back as %q but the part declares %q", got, want)
	}
	if theme.ThemeElements != nil && theme.ThemeElements.ClrScheme != nil {
		if want := "clr=" + theme.ThemeElements.ClrScheme.Name; !strings.Contains(got, want) {
			t.Fatalf("the theme reads back as %q but the part declares %q", got, want)
		}
	}
	if theme.ThemeElements != nil && theme.ThemeElements.FontScheme != nil {
		if want := "font=" + theme.ThemeElements.FontScheme.Name; !strings.Contains(got, want) {
			t.Fatalf("the theme reads back as %q but the part declares %q", got, want)
		}
	}
}

// assertTableColumnsMatchPart holds the table the library reports to the table
// part it was told to read. The unchanged-parts oracle excludes the table
// summary — the mutated part is what defines it — so without this the table
// target would check only that the summary stopped changing.
func assertTableColumnsMatchPart(t *testing.T, part []byte, snap map[string]string) {
	t.Helper()
	if snap == nil {
		return
	}
	model, err := oxml.ParseTable(part)
	if err != nil {
		// The library skips a table part it cannot parse, so no table is
		// reported and there is nothing to compare against.
		return
	}
	var columns []string
	for i := range model.Columns {
		columns = append(columns, model.Columns[i].Name)
	}
	got := snap["Data!tables"]
	if got == "" {
		// A parsed part the library still declined to attach is checked by the
		// other oracles; only a reported table is compared here.
		return
	}
	want := fmt.Sprintf("%s@%s", model.Name, model.Ref)
	if !strings.HasPrefix(got, want) {
		t.Fatalf("the table reads back as %q but the part declares name %q over %q", got, model.Name, model.Ref)
	}
	for _, name := range columns {
		if !strings.Contains(got, name) {
			t.Fatalf("the table reads back as %q but the part declares a column named %q", got, name)
		}
	}
}

// snapshotDiff renders the keys on which two snapshots disagree, sorted so the
// message is stable.
func snapshotDiff(a, b map[string]string) string {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		if a[k] != b[k] {
			sorted = append(sorted, k)
		}
	}
	if len(sorted) == 0 {
		return ""
	}
	sort.Strings(sorted)
	var sb strings.Builder
	for i, k := range sorted {
		if i == 8 {
			fmt.Fprintf(&sb, "  ... and %d more keys\n", len(sorted)-8)
			break
		}
		fmt.Fprintf(&sb, "  %s: first %q, second %q\n", k, a[k], b[k])
	}
	return sb.String()
}

// describePackageDrift names the parts on which two saved packages differ,
// which is far more useful in a fuzz failure than a byte count.
func describePackageDrift(a, b []byte) string {
	nameSet := func(pkg []byte) map[string][]byte {
		out := map[string][]byte{}
		for _, name := range zipEntryNames(pkg) {
			out[name] = fuzzseed.ZipEntry(pkg, name)
		}
		return out
	}
	first, second := nameSet(a), nameSet(b)
	var changed []string
	for name, data := range first {
		other, ok := second[name]
		switch {
		case !ok:
			changed = append(changed, name+" (dropped)")
		case !bytes.Equal(data, other):
			changed = append(changed, fmt.Sprintf("%s (%d -> %d bytes)", name, len(data), len(other)))
		}
	}
	for name := range second {
		if _, ok := first[name]; !ok {
			changed = append(changed, name+" (added)")
		}
	}
	sort.Strings(changed)
	if len(changed) == 0 {
		if orderA, orderB := zipEntryNames(a), zipEntryNames(b); !slices.Equal(orderA, orderB) {
			return fmt.Sprintf("every part is byte-identical but the entry order changed: %v then %v", orderA, orderB)
		}
		return fmt.Sprintf("no part differs, but the archives differ (%d vs %d bytes)", len(a), len(b))
	}
	return strings.Join(changed, ", ")
}

func zipEntryNames(pkg []byte) []string {
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

// protectedSnapshot returns the pristine fixture's snapshot with the given
// keys removed, i.e. the set of observations the mutated part may not change.
func protectedSnapshot(tb testing.TB, pkg []byte, drop ...string) map[string]string {
	tb.Helper()
	w, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		tb.Fatalf("snapshotting the fixture: %v", err)
	}
	defer func() { _ = w.Close() }()
	snap := bookSnapshot(w)
	for _, k := range drop {
		delete(snap, k)
	}
	return snap
}

// dropNumericCells removes every cell whose pristine type is a number or a
// date. Those are the only cells a number format can retype, so they are the
// only ones the styles part may legitimately change.
func dropNumericCells(snap map[string]string) map[string]string {
	for k, v := range snap {
		if strings.HasPrefix(v, fmt.Sprintf("t%d|", CellTypeNumber)) || strings.HasPrefix(v, fmt.Sprintf("t%d|", CellTypeDate)) {
			delete(snap, k)
		}
	}
	return snap
}

// ---------------------------------------------------------------------------
// Targets
// ---------------------------------------------------------------------------

// mutate replaces one part of a package, or skips the execution when the
// fixture has somehow stopped being a readable archive.
func mutate(t *testing.T, pkg []byte, part string, data []byte) []byte {
	t.Helper()
	out := fuzzseed.ReplaceZipEntry(pkg, part, data)
	if out == nil {
		t.Skip("fixture package unreadable")
	}
	return out
}

// FuzzXlsxStylesXML replaces xl/styles.xml. A stylesheet is pure presentation
// with exactly one documented exception — a number format can make a numeric
// cell read back as a date — so every non-numeric cell, every sheet name, and
// the tables and comments must survive whatever is put in the part.
func FuzzXlsxStylesXML(f *testing.F) {
	fixture := buildXlsxPartsFixture()
	assertPartsFixture(f, fixture)
	protected := dropNumericCells(protectedSnapshot(f, fixture))

	f.Add([]byte(fixtureStyles))
	f.Add([]byte{})
	f.Add([]byte("   "))
	f.Add([]byte("not xml"))
	f.Add([]byte(fixtureStyles[:len(fixtureStyles)/2]))
	// Counts that contradict the number of children, at the type boundary.
	f.Add([]byte(`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<numFmts count="4294967295"/><fonts count="4294967295"/><fills count="4294967295"/>` +
		`<borders count="4294967295"/><cellStyleXfs count="4294967295"/><cellXfs count="4294967295"/>` +
		`<dxfs count="4294967295"/></styleSheet>`))
	// Every index in an xf pointing past the end of the collection it indexes.
	f.Add([]byte(`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<cellXfs count="3"><xf numFmtId="4294967295" fontId="4294967295" fillId="4294967295" borderId="4294967295" xfId="4294967295"/>` +
		`<xf numFmtId="-1" fontId="-1" fillId="-1" borderId="-1" xfId="-1"/>` +
		`<xf numFmtId="99999999999999999999" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs></styleSheet>`))
	// Custom number formats at and around the reserved/custom boundary, one of
	// them a format code long enough to matter and one claiming to be a date.
	f.Add([]byte(`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<numFmts count="4"><numFmt numFmtId="0" formatCode="` + strings.Repeat("yyyy", 512) + `"/>` +
		`<numFmt numFmtId="163" formatCode="@"/><numFmt numFmtId="164" formatCode=""/>` +
		`<numFmt numFmtId="65535" formatCode="[$-409]dd/mm/yyyy"/></numFmts>` +
		`<cellXfs count="4"><xf numFmtId="0"/><xf numFmtId="65535"/><xf numFmtId="164"/><xf numFmtId="163"/></cellXfs></styleSheet>`))
	// A cellStyleXf chain that points at itself, plus an xfId cycle.
	f.Add([]byte(`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<cellStyleXfs count="2"><xf xfId="1"/><xf xfId="0"/></cellStyleXfs>` +
		`<cellXfs count="2"><xf xfId="1" numFmtId="14" applyNumberFormat="1"/><xf xfId="0"/></cellXfs>` +
		`<cellStyles count="2"><cellStyle name="A" xfId="1"/><cellStyle name="A" xfId="0"/></cellStyles></styleSheet>`))
	// Every cellXf a built-in date/time format, at every index the fixture's
	// cells use. A cell's declared type outranks its number format, so this
	// must retype the numeric cells and leave the string, boolean, error and
	// formula cells exactly as they are.
	f.Add([]byte(`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<cellXfs count="5"><xf numFmtId="14"/><xf numFmtId="18"/><xf numFmtId="22"/><xf numFmtId="45"/><xf numFmtId="47"/></cellXfs></styleSheet>`))
	// Deep nesting inside a legitimately-rooted stylesheet.
	f.Add([]byte(`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		strings.Repeat("<x>", 400) + strings.Repeat("</x>", 400) + `</styleSheet>`))
	// Wrong root element with the right namespace.
	f.Add([]byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		exercisePart(t, mutate(t, fixture, partStyles, data), protected)
	})
}

// FuzzXlsxSharedStringsXML replaces xl/sharedStrings.xml. The string table can
// only supply values to t="s" cells, so every other cell, the sheet names, the
// table and the comments must be unchanged no matter what the part contains.
func FuzzXlsxSharedStringsXML(f *testing.F) {
	fixture := buildXlsxPartsFixture()
	assertPartsFixture(f, fixture)
	protected := protectedSnapshot(f, fixture, sharedStringCellRefs()...)

	f.Add([]byte(fixtureSharedStrings))
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add([]byte(fixtureSharedStrings[:len(fixtureSharedStrings)/2]))
	// Counts that contradict the item list in both directions.
	f.Add([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="4294967295" uniqueCount="4294967295"/>`))
	f.Add([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="0" uniqueCount="0">` +
		`<si><t>one</t></si><si><t>two</t></si></sst>`))
	f.Add([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="-1" uniqueCount="99999999999999999999"><si><t>x</t></si></sst>`))
	// Items with neither text nor runs, empty items, and an item that is only
	// phonetic guides — every shape buildStringTable has to fall through.
	f.Add([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="4">` +
		`<si/><si><t/></si><si><r/></si>` +
		`<si><rPh sb="4294967295" eb="0"><t>reading</t></rPh><phoneticPr fontId="99999"/></si></sst>`))
	// An item list with holes: items that carry no text at all sit between
	// items that do. The empty items still occupy an index, so a table built
	// by appending only the non-empty ones would shift every later cell onto
	// the wrong string — the shape the resolution oracle exists to catch.
	f.Add([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="6" uniqueCount="6">` +
		`<si><t>zero</t></si><si/><si><t>two</t></si>` +
		`<si><rPh sb="0" eb="1"><t>reading</t></rPh></si><si><t>four</t></si><si><r><t>fi</t></r><r><t>ve</t></r></si></sst>`))
	// A single item made of many runs: the concatenating branch at scale.
	f.Add([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="1"><si>` +
		strings.Repeat(`<r><t>ab</t></r>`, 500) + `</si></sst>`))
	// Deep nesting under si, which the unknown-element skip has to unwind.
	f.Add([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si>` +
		strings.Repeat("<x>", 400) + strings.Repeat("</x>", 400) + `</si></sst>`))
	// Text that has to survive re-serialization intact.
	f.Add([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="3">` +
		`<si><t>&lt;&amp;&gt;&#xD;</t></si><si><t xml:space="preserve">  </t></si><si><t>` + strings.Repeat("z", 4096) + `</t></si></sst>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		snap := exercisePart(t, mutate(t, fixture, partSharedStrings, data), protected)
		assertSharedStringResolution(t, data, snap)
	})
}

// FuzzXlsxThemeXML replaces xl/theme/theme1.xml. A theme carries no cell data
// at all, so nothing observable about the sheets may change whatever is in it.
func FuzzXlsxThemeXML(f *testing.F) {
	fixture := buildXlsxPartsFixture()
	assertPartsFixture(f, fixture)
	protected := protectedSnapshot(f, fixture, "theme")

	f.Add([]byte(fixtureTheme))
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add([]byte(fixtureTheme[:len(fixtureTheme)/2]))
	// A theme with the scheme elements present but empty.
	f.Add([]byte(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="">` +
		`<a:themeElements><a:clrScheme name=""/><a:fontScheme name=""/><a:fmtScheme name=""/></a:themeElements></a:theme>`))
	// Colour slots repeated, out of order, and carrying nonsense values.
	f.Add([]byte(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="T">` +
		`<a:themeElements><a:clrScheme name="C">` +
		`<a:accent1><a:srgbClr val="ZZZZZZ"/></a:accent1><a:accent1><a:srgbClr val="00000000000000"/></a:accent1>` +
		`<a:dk1><a:sysClr val="" lastClr=""/></a:dk1><a:dk1><a:schemeClr val="phClr"/></a:dk1>` +
		`</a:clrScheme></a:themeElements></a:theme>`))
	// Deep nesting and a huge repeated child list.
	f.Add([]byte(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		strings.Repeat("<a:x>", 400) + strings.Repeat("</a:x>", 400) + `</a:theme>`))
	f.Add([]byte(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements><a:fmtScheme name="F"><a:lnStyleLst>` +
		strings.Repeat(`<a:ln w="2147483647"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>`, 200) +
		`</a:lnStyleLst></a:fmtScheme></a:themeElements></a:theme>`))
	// The right shape in the wrong namespace.
	f.Add([]byte(`<theme xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" name="Office Theme"><themeElements/></theme>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		snap := exercisePart(t, mutate(t, fixture, partTheme, data), protected)
		assertThemeMatchesPart(t, data, snap)
	})
}

// FuzzXlsxTableXML replaces xl/tables/table1.xml. A table part describes a
// region of an existing sheet; it owns no cell data, so every cell, every
// sheet name and every comment must be unchanged. The table summary itself is
// allowed to change, and is excluded.
func FuzzXlsxTableXML(f *testing.F) {
	fixture := buildXlsxPartsFixture()
	assertPartsFixture(f, fixture)
	protected := protectedSnapshot(f, fixture, "Data!tables", "Notes!tables")

	f.Add([]byte(fixtureTable))
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add([]byte(fixtureTable[:len(fixtureTable)/2]))
	// A ref that is reversed, degenerate, outside the sheet, or not a range.
	f.Add([]byte(`<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="1" name="T" displayName="T" ref="C6:A3">` +
		`<autoFilter ref=":"/><tableColumns count="3"><tableColumn id="1" name="a"/><tableColumn id="2" name="b"/><tableColumn id="3" name="c"/></tableColumns></table>`))
	f.Add([]byte(`<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="1" name="T" displayName="T" ref="A1:XFD1048576">` +
		`<tableColumns count="1"><tableColumn id="1" name="a"/></tableColumns></table>`))
	// Column count contradicting the column list, duplicate and boundary ids.
	f.Add([]byte(`<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="4294967295" name="T" displayName="T" ref="A3:C6" ` +
		`headerRowCount="4294967295" totalsRowCount="4294967295" totalsRowShown="1">` +
		`<tableColumns count="4294967295"><tableColumn id="0" name=""/><tableColumn id="0" name=""/>` +
		`<tableColumn id="4294967295" name="` + strings.Repeat("n", 4096) + `"/></tableColumns></table>`))
	// A calculated column that refers to the table it lives in.
	f.Add([]byte(`<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="1" name="SalesTable" displayName="SalesTable" ref="A3:C6">` +
		`<tableColumns count="3"><tableColumn id="1" name="Alpha"><calculatedColumnFormula>SalesTable[Alpha]</calculatedColumnFormula></tableColumn>` +
		`<tableColumn id="2" name="Beta" totalsRowFunction="custom"><totalsRowFormula>SUBTOTAL(109,SalesTable[Beta])</totalsRowFormula></tableColumn>` +
		`<tableColumn id="3" name="Delta" totalsRowLabel="` + strings.Repeat("t", 1024) + `"/></tableColumns></table>`))
	// Deep nesting.
	f.Add([]byte(`<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="1" name="T" displayName="T" ref="A3:C6">` +
		strings.Repeat("<x>", 400) + strings.Repeat("</x>", 400) + `</table>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		snap := exercisePart(t, mutate(t, fixture, partTable, data), protected)
		assertTableColumnsMatchPart(t, data, snap)
	})
}

// FuzzXlsxCommentsXML replaces one of the three parts that make up a comment
// thread — the classic comments part, the threadedComments part, or the
// workbook-level persons part. Comments annotate cells but hold none of their
// data, so every cell and sheet name must be unchanged; the comment summary
// itself is excluded.
func FuzzXlsxCommentsXML(f *testing.F) {
	fixture := buildXlsxCommentsFixture(f)
	assertCommentsFixture(f, fixture)
	protected := protectedSnapshot(f, fixture, "Data!comments")

	parts := []string{partComments, partThreadedComments, partPersons}
	for i, part := range parts {
		orig := fuzzseed.ZipEntry(fixture, part)
		f.Add(uint8(i), orig)
		f.Add(uint8(i), []byte{})
		f.Add(uint8(i), []byte("not xml"))
		f.Add(uint8(i), orig[:len(orig)/2])
	}
	// An authorId pointing past the author list, and one that is not a number.
	f.Add(uint8(0), []byte(`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`+
		`<authors/><commentList><comment ref="D4" authorId="4294967295"><text><t>x</t></text></comment>`+
		`<comment ref="E5" authorId="-1"><text><t>y</t></text></comment>`+
		`<comment ref="!!" authorId="nope"><text><t>z</t></text></comment></commentList></comments>`))
	// An authorId exactly one past the end of the author list, the classic
	// off-by-one target, alongside the last valid one.
	f.Add(uint8(0), []byte(`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`+
		`<authors><author>only</author></authors><commentList>`+
		`<comment ref="D4" authorId="0"><text><t>in range</t></text></comment>`+
		`<comment ref="E5" authorId="1"><text><t>one past</t></text></comment></commentList></comments>`))
	// Many authors and no comments, then many comments on the same cell.
	f.Add(uint8(0), []byte(`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><authors>`+
		strings.Repeat(`<author>a</author>`, 500)+`</authors><commentList/></comments>`))
	f.Add(uint8(0), []byte(`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><authors><author>a</author></authors><commentList>`+
		strings.Repeat(`<comment ref="D4" authorId="0"><text><r><t>run</t></r></text></comment>`, 300)+`</commentList></comments>`))
	// A threaded comment whose parent id does not exist, and one that is its
	// own parent: the reply-threading walk must not follow the cycle.
	f.Add(uint8(1), []byte(`<ThreadedComments xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments">`+
		`<threadedComment ref="B2" dT="not-a-date" personId="{missing}" id="{1}" parentId="{nope}"><text>orphan</text></threadedComment>`+
		`<threadedComment ref="B2" dT="0001-01-01T00:00:00" personId="{p}" id="{2}" parentId="{2}"><text>self</text></threadedComment>`+
		`</ThreadedComments>`))
	// A person list with duplicate and empty ids.
	f.Add(uint8(2), []byte(`<personList xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments">`+
		`<person displayName="" id="" userId="" providerId=""/>`+
		`<person displayName="`+strings.Repeat("d", 2048)+`" id="{p}" userId="u" providerId="None"/>`+
		`<person displayName="dup" id="{p}" userId="u" providerId="None"/></personList>`))
	// Deep nesting in the comment text.
	f.Add(uint8(0), []byte(`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><authors><author>a</author></authors>`+
		`<commentList><comment ref="D4" authorId="0"><text>`+strings.Repeat("<x>", 400)+strings.Repeat("</x>", 400)+`</text></comment></commentList></comments>`))

	f.Fuzz(func(t *testing.T, which uint8, data []byte) {
		exercisePart(t, mutate(t, fixture, parts[int(which)%len(parts)], data), protected)
	})
}

// FuzzXlsxPivotPartXML replaces either the pivot cache definition or the pivot
// table part. Both describe a summary of a source range and hold none of the
// source data, so the source sheet's cells must be unchanged; the pivot
// summary itself is excluded.
func FuzzXlsxPivotPartXML(f *testing.F) {
	fixture := buildXlsxPivotFixture(f)
	assertPivotFixture(f, fixture)
	protected := protectedSnapshot(f, fixture, "pivots")

	parts := []string{partPivotCache, partPivotTable}
	for i, part := range parts {
		orig := fuzzseed.ZipEntry(fixture, part)
		f.Add(uint8(i), orig)
		f.Add(uint8(i), []byte{})
		f.Add(uint8(i), []byte("not xml"))
		f.Add(uint8(i), orig[:len(orig)/2])
	}
	// A cache whose field count contradicts its field list and whose source
	// range is degenerate.
	f.Add(uint8(0), []byte(`<pivotCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" `+
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1" recordCount="4294967295">`+
		`<cacheSource type="worksheet"><worksheetSource ref=":" sheet=""/></cacheSource>`+
		`<cacheFields count="4294967295"/></pivotCacheDefinition>`))
	// Shared items with counts that do not match, and a field group that
	// refers to itself as its own base.
	f.Add(uint8(0), []byte(`<pivotCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" `+
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1" recordCount="0">`+
		`<cacheSource type="worksheet"><worksheetSource ref="A1:C5" sheet="Data"/></cacheSource>`+
		`<cacheFields count="1"><cacheField name="Region" numFmtId="0">`+
		`<sharedItems count="4294967295" containsSemiMixedTypes="1"><s v="R0"/><s v="R1"/></sharedItems>`+
		`<fieldGroup base="0" par="0"><rangePr groupBy="days" startDate="9999-99-99" endDate="0000-00-00"/>`+
		`<groupItems count="4294967295"/></fieldGroup></cacheField></cacheFields></pivotCacheDefinition>`))
	// A pivot table whose axis field indices all point past the cache fields.
	f.Add(uint8(1), []byte(`<pivotTableDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" `+
		`name="P" cacheId="4294967295" dataCaption="Values">`+
		`<location ref=":" firstHeaderRow="4294967295" firstDataRow="4294967295" firstDataCol="4294967295"/>`+
		`<pivotFields count="4294967295"><pivotField axis="axisRow" showAll="0"><items count="4294967295"/></pivotField></pivotFields>`+
		`<rowFields count="3"><field x="2147483647"/><field x="-2"/><field x="0"/></rowFields>`+
		`<colFields count="1"><field x="99999"/></colFields>`+
		`<dataFields count="1"><dataField name="" fld="4294967295" baseField="4294967295" baseItem="4294967295"/></dataFields>`+
		`</pivotTableDefinition>`))
	// A pivot table with a very long axis field list.
	f.Add(uint8(1), []byte(`<pivotTableDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" name="P" cacheId="1" dataCaption="V">`+
		`<location ref="A1:B2" firstHeaderRow="1" firstDataRow="1" firstDataCol="1"/>`+
		`<rowFields count="500">`+strings.Repeat(`<field x="0"/>`, 500)+`</rowFields></pivotTableDefinition>`))
	// Deep nesting in both parts.
	f.Add(uint8(0), []byte(`<pivotCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`+
		strings.Repeat("<x>", 400)+strings.Repeat("</x>", 400)+`</pivotCacheDefinition>`))
	f.Add(uint8(1), []byte(`<pivotTableDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`+
		strings.Repeat("<x>", 400)+strings.Repeat("</x>", 400)+`</pivotTableDefinition>`))

	f.Fuzz(func(t *testing.T, which uint8, data []byte) {
		exercisePart(t, mutate(t, fixture, parts[int(which)%len(parts)], data), protected)
	})
}

// FuzzXlsxWorkbookXML replaces xl/workbook.xml itself. The main part decides
// which sheets exist and in what order, so nothing about the sheets is
// protected here; what is asserted is that a package the library accepts is a
// fixed point and that a package it rejects is rejected with an error rather
// than opened half-way.
func FuzzXlsxWorkbookXML(f *testing.F) {
	fixture := buildXlsxPartsFixture()
	assertPartsFixture(f, fixture)

	f.Add([]byte(fixtureWorkbook))
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add([]byte(fixtureWorkbook[:len(fixtureWorkbook)/2]))
	// Sheets naming a relationship that does not exist, sharing an r:id,
	// sharing a name, and sharing a sheetId.
	f.Add([]byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` +
		`<sheet name="Data" sheetId="1" r:id="rId1"/><sheet name="Data" sheetId="1" r:id="rId1"/>` +
		`<sheet name="" sheetId="4294967295" r:id="rId99"/></sheets></workbook>`))
	// Both sheets pointing at the same worksheet part.
	f.Add([]byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` +
		`<sheet name="A" sheetId="1" r:id="rId1"/><sheet name="B" sheetId="2" r:id="rId1"/></sheets></workbook>`))
	// A sheet claiming to be a shared-strings part, i.e. an r:id of the wrong
	// relationship type.
	f.Add([]byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` +
		`<sheet name="S" sheetId="1" r:id="rId4"/></sheets></workbook>`))
	// No sheets at all, and numeric fields at their boundaries.
	f.Add([]byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheets/>` +
		`<workbookPr date1904="maybe" backupFile="2"/><calcPr calcId="4294967295" iterateCount="4294967295" iterateDelta="1e999"/>` +
		`<bookViews><workbookView activeTab="4294967295" firstSheet="-1" windowWidth="4294967295"/></bookViews></workbook>`))
	// Defined names that are self-referential, malformed, and duplicated.
	f.Add([]byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` +
		`<sheet name="Data" sheetId="1" r:id="rId1"/><sheet name="Notes" sheetId="2" r:id="rId2"/></sheets><definedNames>` +
		`<definedName name="A">A</definedName><definedName name="A" localSheetId="4294967295">Data!$A$1</definedName>` +
		`<definedName name="">` + strings.Repeat("Data!$A$1,", 500) + `</definedName></definedNames></workbook>`))
	// Deep nesting and a very long sheet list.
	f.Add([]byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		strings.Repeat("<x>", 400) + strings.Repeat("</x>", 400) + `</workbook>`))
	f.Add([]byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` +
		strings.Repeat(`<sheet name="Data" sheetId="1" r:id="rId1"/>`, 500) + `</sheets></workbook>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		exercisePart(t, mutate(t, fixture, partWorkbook, data), nil)
	})
}
