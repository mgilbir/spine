package schemavalid_test

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mgilbir/spine/internal/schemavalid"
)

// orderTablePath is the committed extract of the schemas' content models.
const orderTablePath = "testdata/child_order.tsv"

// orderedRoots are the part roots the three formats author from scratch, and
// therefore the ones whose child order nothing else can check. A root read from
// a file is covered by the fidelity tests instead: its bytes have to come back
// unchanged, which a reordering breaks.
var orderedRoots = []xml.Name{
	{Space: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Local: "document"},
	{Space: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Local: "styles"},
	{Space: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Local: "comments"},
	{Space: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Local: "settings"},
	{Space: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Local: "numbering"},
	{Space: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Local: "hdr"},
	{Space: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Local: "ftr"},
	{Space: "http://schemas.openxmlformats.org/spreadsheetml/2006/main", Local: "workbook"},
	{Space: "http://schemas.openxmlformats.org/spreadsheetml/2006/main", Local: "worksheet"},
	{Space: "http://schemas.openxmlformats.org/spreadsheetml/2006/main", Local: "styleSheet"},
	{Space: "http://schemas.openxmlformats.org/spreadsheetml/2006/main", Local: "sst"},
	{Space: "http://schemas.openxmlformats.org/presentationml/2006/main", Local: "presentation"},
	{Space: "http://schemas.openxmlformats.org/presentationml/2006/main", Local: "sld"},
	{Space: "http://schemas.openxmlformats.org/presentationml/2006/main", Local: "sldLayout"},
	{Space: "http://schemas.openxmlformats.org/presentationml/2006/main", Local: "sldMaster"},
	{Space: "http://schemas.openxmlformats.org/presentationml/2006/main", Local: "notes"},
	{Space: "http://schemas.openxmlformats.org/presentationml/2006/main", Local: "notesMaster"},
	{Space: "http://schemas.openxmlformats.org/drawingml/2006/main", Local: "theme"},
}

// TestUpdateChildOrderTable regenerates the committed table from the schemas.
// It is not a check — it writes a file — so it only runs when asked, on a
// machine that has the standard.
func TestUpdateChildOrderTable(t *testing.T) {
	if os.Getenv("SPINE_UPDATE_ORDER_TABLE") == "" {
		t.Skip("set SPINE_UPDATE_ORDER_TABLE=1 (with spec/part2 and spec/part4 present) to regenerate " + orderTablePath)
	}
	root := repoRoot(t)
	if !schemavalid.SchemasPresent(root) {
		t.Fatalf("SPINE_UPDATE_ORDER_TABLE is set but the schemas are absent; see spec/README.md")
	}

	orders := make([]schemavalid.ChildOrder, 0, len(orderedRoots))
	for _, name := range orderedRoots {
		order, err := schemavalid.LoadChildOrder(root, name)
		if err != nil {
			t.Fatalf("extracting %s: %v", name.Local, err)
		}
		if len(order.Positions) == 0 {
			t.Fatalf("extracting %s produced no content model", name.Local)
		}
		orders = append(orders, order)
	}
	if err := os.MkdirAll(filepath.Dir(orderTablePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orderTablePath, []byte(schemavalid.FormatOrderTable(orders)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s with %d content models", orderTablePath, len(orders))
}

// TestAuthoredPartsFollowTheContentModel is the order check that runs
// everywhere, schemas or not.
//
// Element order in OOXML is normative — the content models are xsd:sequence —
// and a part built by Create and the authoring APIs has nothing to be compared
// against: the corpus proves only that a part read from a file comes back
// unchanged. So the schemas' order is committed as a table and the parts this
// library writes are held to it.
func TestAuthoredPartsFollowTheContentModel(t *testing.T) {
	table := loadOrderTable(t)

	for name, pkg := range authoredPackages(t) {
		t.Run(name, func(t *testing.T) {
			zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
			if err != nil {
				t.Fatalf("saved package is not a zip: %v", err)
			}
			checked := 0
			for _, zf := range zr.File {
				rc, err := zf.Open()
				if err != nil {
					t.Fatalf("opening %s: %v", zf.Name, err)
				}
				data, err := io.ReadAll(rc)
				_ = rc.Close()
				if err != nil {
					t.Fatalf("reading %s: %v", zf.Name, err)
				}
				root, children := schemavalid.RootChildren(data)
				order, ok := table[root]
				if !ok {
					continue
				}
				checked++
				if why := schemavalid.CheckOrder(order, children); why != "" {
					t.Errorf("%s does not follow the content model of <%s>:\n\t%s\n\temitted: %v",
						zf.Name, root.Local, why, children)
				}
			}
			if checked == 0 {
				t.Errorf("no part of this package has an entry in %s, so nothing was checked", orderTablePath)
			}
		})
	}
}

func loadOrderTable(t *testing.T) map[xml.Name]schemavalid.ChildOrder {
	t.Helper()
	data, err := os.ReadFile(orderTablePath)
	if err != nil {
		t.Fatalf("reading %s: %v (regenerate with SPINE_UPDATE_ORDER_TABLE=1)", orderTablePath, err)
	}
	table, err := schemavalid.ParseOrderTable(string(data))
	if err != nil {
		t.Fatalf("parsing %s: %v", orderTablePath, err)
	}
	if len(table) < len(orderedRoots) {
		t.Fatalf("%s has %d content models, want %d — regenerate it", orderTablePath, len(table), len(orderedRoots))
	}
	return table
}
