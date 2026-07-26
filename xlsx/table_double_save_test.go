package xlsx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// countTableParts returns the number of <tablePart> entries and the number of
// distinct r:ids they reference in a saved worksheet part plus its .rels.
func assertSingleTablePart(t *testing.T, out []byte, sheetPart string) {
	t.Helper()
	ws := string(readZipPart(t, out, sheetPart))
	// Count the <tablePart r:id=.../> entries only; the <tableParts> container tag
	// shares the "<tablePart" prefix, so match on the entry's r:id attribute.
	if n := strings.Count(ws, "<tablePart r:id"); n != 1 {
		t.Fatalf("%s has %d <tablePart> entries, want 1:\n%s", sheetPart, n, ws)
	}
	relsPart := relsPartFor("/" + sheetPart)[1:] // strip leading '/'
	rels, err := opc.UnmarshalRelationships(readZipPart(t, out, relsPart))
	if err != nil {
		t.Fatalf("UnmarshalRelationships(%s): %v", relsPart, err)
	}
	seen := make(map[string]struct{})
	tableRels := 0
	for _, rel := range rels {
		if rel.Type != opc.RelTypeTable {
			continue
		}
		tableRels++
		if _, dup := seen[rel.ID]; dup {
			t.Errorf("duplicate table relationship id %q in %s", rel.ID, relsPart)
		}
		seen[rel.ID] = struct{}{}
	}
	if tableRels != 1 {
		t.Fatalf("%s has %d table relationships, want 1", relsPart, tableRels)
	}
}

// TestAddTableDoubleSaveCreated guards C257 on the create path: AddTable then two
// SaveBytes calls must both emit exactly one <tablePart>, and the durable
// worksheet model must not grow between saves.
func TestAddTableDoubleSaveCreated(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("Data")
	for i, h := range []string{"Name", "Age"} {
		c, _ := sh.Cell(FormatCellRef(1, i+1))
		c.SetString(h)
	}
	if _, err := sh.AddTable("A1:B2", TableOptions{}); err != nil {
		t.Fatalf("AddTable: %v", err)
	}

	if _, err := wb.SaveBytes(); err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	out2, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}

	assertSingleTablePart(t, out2, "xl/worksheets/sheet1.xml")
	if got := len(sh.ws().TableParts.TablePart); got != 1 {
		t.Errorf("durable model grew: TableParts = %d, want 1", got)
	}
}

// TestAddTableDoubleSaveOpened guards C257 on the round-trip path.
func TestAddTableDoubleSaveOpened(t *testing.T) {
	base := Create()
	sh := base.AddSheet("Sheet1")
	for i, h := range []string{"Product", "Price"} {
		c, _ := sh.Cell(FormatCellRef(1, i+1))
		c.SetString(h)
	}
	var baseBuf bytes.Buffer
	if err := base.SaveTo(&baseBuf); err != nil {
		t.Fatalf("save base: %v", err)
	}

	wb, err := OpenReader(bytes.NewReader(baseBuf.Bytes()), int64(baseBuf.Len()))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = wb.Close() }()
	osheet, err := wb.SheetByName("Sheet1")
	if err != nil {
		t.Fatalf("sheet: %v", err)
	}
	if _, err := osheet.AddTable("A1:B2", TableOptions{Name: "Products"}); err != nil {
		t.Fatalf("AddTable: %v", err)
	}

	if _, err := wb.SaveBytes(); err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	out2, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}

	assertSingleTablePart(t, out2, "xl/worksheets/sheet1.xml")
	if got := len(osheet.ws().TableParts.TablePart); got != 1 {
		t.Errorf("durable model grew: TableParts = %d, want 1", got)
	}
}
