package xlsx

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

// buildXLSXWithExtraParts rebuilds minimal.xlsx adding extra named parts
// verbatim (used to inject connections / data-model / customXml parts).
func buildXLSXWithExtraParts(t *testing.T, extra map[string]string) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		wr, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if _, err := wr.Write(content.Bytes()); err != nil {
			t.Fatalf("write %s: %v", f.Name, err)
		}
	}
	for name, body := range extra {
		wr, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := wr.Write([]byte(body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestConnectionsRead(t *testing.T) {
	const conns = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<connections xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<connection id="2" name="Feed" type="4" description="a web query"><webPr url="https://example.com/data"/></connection>` +
		`<connection id="1" name="DB" type="1"><dbPr connection="Provider=SQLOLEDB;Data Source=srv" command="SELECT 1"/></connection>` +
		`</connections>`
	pkg := buildXLSXWithExtraParts(t, map[string]string{"xl/connections.xml": conns})

	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	got := wb.Connections()
	if len(got) != 2 {
		t.Fatalf("Connections() = %d, want 2", len(got))
	}
	// Sorted by id: DB (1) first.
	if got[0].ID != 1 || got[0].Name != "DB" || got[0].Type != 1 {
		t.Errorf("connection[0] = %+v", got[0])
	}
	if got[0].ConnectionString != "Provider=SQLOLEDB;Data Source=srv" || got[0].Command != "SELECT 1" {
		t.Errorf("connection[0] db props = %+v", got[0])
	}
	if got[1].ID != 2 || got[1].WebURL != "https://example.com/data" || got[1].Description != "a web query" {
		t.Errorf("connection[1] = %+v", got[1])
	}

	// Round trip preserves the connections part verbatim.
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := string(readZipEntry(t, out, "xl/connections.xml")); got != conns {
		t.Errorf("connections part not preserved verbatim:\n%s", got)
	}
}

func TestConnectionsNoneOnPlainWorkbook(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := wb.Connections(); got != nil {
		t.Errorf("plain workbook returned %d connections", len(got))
	}
	if info := wb.DataModel(); info.HasDataModel || info.HasPowerQuery {
		t.Errorf("plain workbook reports data model: %+v", info)
	}
}

func TestDataModelPresence(t *testing.T) {
	pkg := buildXLSXWithExtraParts(t, map[string]string{
		"xl/model/item.data":  "\x00\x01binary model",
		"customXml/item1.xml": `<DataMashup xmlns="http://schemas.microsoft.com/DataMashup">base64blob</DataMashup>`,
		"customXml/item2.xml": `<unrelated/>`,
	})
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	info := wb.DataModel()
	if !info.HasDataModel {
		t.Errorf("HasDataModel = false, want true")
	}
	if !info.HasPowerQuery {
		t.Errorf("HasPowerQuery = false, want true")
	}
	if len(info.ModelParts) != 1 || info.ModelParts[0] != "/xl/model/item.data" {
		t.Errorf("ModelParts = %v", info.ModelParts)
	}
	if len(info.CustomXMLParts) != 1 || info.CustomXMLParts[0] != "/customXml/item1.xml" {
		t.Errorf("CustomXMLParts = %v", info.CustomXMLParts)
	}
}
