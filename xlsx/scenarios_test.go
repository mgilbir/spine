package xlsx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
)

// AddScenario writes a scenarios element from the typed model and it survives a
// Create -> Save -> Open round trip.
func TestAddScenarioRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	// A cell must exist so the sheet is non-empty on save.
	if _, err := s.Cell("B2"); err != nil {
		t.Fatal(err)
	}
	err := s.AddScenario(Scenario{
		Name:    "High",
		Comment: "best case",
		User:    "analyst",
		Inputs:  []ScenarioInput{{Cell: "B2", Value: "100"}, {Cell: "B3", Value: "200"}},
	})
	if err != nil {
		t.Fatalf("AddScenario: %v", err)
	}

	out, err := marshalWorksheetXML(s.ws())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	for _, want := range []string{
		`<scenarios current="0" show="0" sqref="B2 B3">`,
		`<scenario name="High" count="2" user="analyst" comment="best case">`,
		`<inputCells r="B2" val="100"/>`,
		`<inputCells r="B3" val="200"/>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("marshaled worksheet missing %q:\n%s", want, got)
		}
	}

	rs := firstSheet(t, reopen(t, w))
	scs := rs.Scenarios()
	if len(scs) != 1 {
		t.Fatalf("Scenarios() = %d, want 1", len(scs))
	}
	sc := scs[0]
	if sc.Name != "High" || sc.Comment != "best case" || sc.User != "analyst" {
		t.Errorf("scenario metadata = %+v", sc)
	}
	if len(sc.Inputs) != 2 || sc.Inputs[0].Cell != "B2" || sc.Inputs[0].Value != "100" {
		t.Errorf("scenario inputs = %+v", sc.Inputs)
	}
}

// A duplicate name and an empty input list are rejected.
func TestAddScenarioValidation(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	if err := s.AddScenario(Scenario{Name: "A", Inputs: []ScenarioInput{{Cell: "B2", Value: "1"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddScenario(Scenario{Name: "a", Inputs: []ScenarioInput{{Cell: "B2", Value: "2"}}}); err == nil {
		t.Error("duplicate scenario name accepted")
	}
	if err := s.AddScenario(Scenario{Name: "B"}); err == nil {
		t.Error("scenario with no inputs accepted")
	}
	if err := s.AddScenario(Scenario{Name: "", Inputs: []ScenarioInput{{Cell: "B2", Value: "1"}}}); err == nil {
		t.Error("empty scenario name accepted")
	}
}

// buildXLSXWithSheetInjection rebuilds minimal.xlsx, splicing extra XML in
// before </worksheet> of sheet1.
func buildXLSXWithSheetInjection(t *testing.T, inject string) []byte {
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
		s := content.String()
		if f.Name == "xl/worksheets/sheet1.xml" {
			s = strings.Replace(s, "</worksheet>", inject+"</worksheet>", 1)
		}
		wr, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if _, err := wr.Write([]byte(s)); err != nil {
			t.Fatalf("write %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// An existing scenarios element is read, and an unmodified round trip preserves
// its exact bytes (fidelity: preserved raw when untouched).
func TestScenariosReadAndPreserve(t *testing.T) {
	const scenarios = `<scenarios current="0" show="0" sqref="B2"><scenario name="Base" locked="1" count="1" user="u" comment="c"><inputCells r="B2" val="42"/></scenario></scenarios>`
	pkg := buildXLSXWithSheetInjection(t, scenarios)

	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s := firstSheet(t, wb)
	got := s.Scenarios()
	if len(got) != 1 || got[0].Name != "Base" || !got[0].Locked {
		t.Fatalf("Scenarios() = %+v", got)
	}
	if len(got[0].Inputs) != 1 || got[0].Inputs[0].Cell != "B2" || got[0].Inputs[0].Value != "42" {
		t.Fatalf("inputs = %+v", got[0].Inputs)
	}

	// Unmodified save must re-emit the scenarios element verbatim.
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheet1 := readZipEntry(t, out, "xl/worksheets/sheet1.xml")
	if !strings.Contains(string(sheet1), scenarios) {
		t.Errorf("scenarios element not preserved verbatim:\n%s", sheet1)
	}
}
