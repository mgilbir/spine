package spectest

import (
	"path/filepath"
	"runtime"
	"testing"
)

func dmlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "dml_examples.json")
}

// WrapDML wraps a DML XML fragment with DrawingML namespace declarations.
func WrapDML(xmlStr string) string {
	return `<a:wrapper` +
		` xmlns:a="` + NsDML + `"` +
		` xmlns:r="` + NsRelationship + `"` +
		` xmlns:p="` + NsPML + `"` +
		` xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"` +
		` xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"` +
		` xmlns:mc="` + NsMC + `"` +
		`>` + xmlStr + `</a:wrapper>`
}

func TestDML_SpecExamples_WellFormed(t *testing.T) {
	examples := LoadExamples(t, dmlTestdataPath())
	TestWellFormedExamples(t, examples, WrapDML)
}

func TestDML_SpecExamples_Unmarshal(t *testing.T) {
	examples := LoadExamples(t, dmlTestdataPath())
	TestUnmarshalExamples(t, examples, DMLTypeMap, WrapDML)
}

func TestDML_SpecExamples_RoundTrip(t *testing.T) {
	examples := LoadExamples(t, dmlTestdataPath())
	TestRoundTripExamples(t, examples, DMLTypeMap, WrapDML)
}
