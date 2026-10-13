package xlsxrender

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/xlsx"
)

// TestRenderBOMPrefixedParts is the #493 regression for workbooks: a UTF-8
// byte order mark at the start of a part (Office writes them) read as content
// outside the root, so rendering failed with "content outside XML root". With
// a BOM on every XML part, the range must render exactly as without one.
func TestRenderBOMPrefixedParts(t *testing.T) {
	w, _, opts := renderTestSheet(t)
	created, err := w.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// Excel writes a styles part and keeps strings in a shared string table;
	// the renderer checks both, so the fixture has both.
	sheet := string(fuzzseed.ZipEntry(created, "xl/worksheets/sheet1.xml"))
	rels := string(fuzzseed.ZipEntry(created, "xl/_rels/workbook.xml.rels"))
	types := string(fuzzseed.ZipEntry(created, "[Content_Types].xml"))
	const rel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	const ct = "application/vnd.openxmlformats-officedocument.spreadsheetml."
	plain := fuzzseed.EditZip(created, [][2]string{
		{"xl/worksheets/sheet1.xml", strings.Replace(sheet, `<c r="A1" t="inlineStr"><is><t>AA</t></is></c>`, `<c r="A1" t="s"><v>0</v></c>`, 1)},
		{"xl/_rels/workbook.xml.rels", strings.Replace(rels, "</Relationships>", `<Relationship Id="rId8" Type="`+rel+`styles" Target="styles.xml"/><Relationship Id="rId9" Type="`+rel+`sharedStrings" Target="sharedStrings.xml"/></Relationships>`, 1)},
		{"[Content_Types].xml", strings.Replace(types, "</Types>", `<Override PartName="/xl/styles.xml" ContentType="`+ct+`styles+xml"/><Override PartName="/xl/sharedStrings.xml" ContentType="`+ct+`sharedStrings+xml"/></Types>`, 1)},
		{"xl/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" + `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`},
		{"xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" + `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="1" uniqueCount="1"><si><t>AA</t></si></sst>`},
	})
	if plain == nil {
		t.Fatal("fixture")
	}
	withBOM, changed, err := testutil.WithBOM(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"xl/worksheets/sheet1.xml", "xl/styles.xml", "xl/sharedStrings.xml", "xl/workbook.xml"} {
		if !slices.Contains(changed, part) {
			t.Fatalf("fixture has no %s to mark: %v", part, changed)
		}
	}
	svg := func(data []byte) []byte {
		t.Helper()
		book, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = book.Close() }()
		page, err := PrepareRange(context.Background(), book.Sheets()[0], "A1:B2", opts)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err = page.WriteSVG(context.Background(), &out, 96); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	if want, got := svg(plain), svg(withBOM); !bytes.Equal(got, want) {
		t.Error("a workbook with BOM-prefixed parts renders differently")
	}
}
