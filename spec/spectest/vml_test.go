package spectest

import (
	"path/filepath"
	"runtime"
	"testing"
)

func vmlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "vml_examples.json")
}

// WrapVML wraps a VML XML fragment with VML namespace declarations.
func WrapVML(xmlStr string) string {
	return `<v:wrapper` +
		` xmlns:v="` + NsVML + `"` +
		` xmlns:o="` + NsOffice + `"` +
		` xmlns:r="` + NsRelationship + `"` +
		` xmlns:w10="urn:schemas-microsoft-com:office:word"` +
		` xmlns:x="urn:schemas-microsoft-com:office:excel"` +
		` xmlns:p="urn:schemas-microsoft-com:office:powerpoint"` +
		`>` + xmlStr + `</v:wrapper>`
}

func TestVML_SpecExamples_WellFormed(t *testing.T) {
	examples := LoadExamples(t, vmlTestdataPath())
	TestWellFormedExamples(t, examples, WrapVML)
}

func TestVML_SpecExamples_Unmarshal(t *testing.T) {
	examples := LoadExamples(t, vmlTestdataPath())
	// VML types are in common/vml/ - add to VMLTypeMap when accessible
	TestUnmarshalExamples(t, examples, VMLTypeMap, WrapVML)
}

func TestVML_SpecExamples_RoundTrip(t *testing.T) {
	examples := LoadExamples(t, vmlTestdataPath())
	TestRoundTripExamples(t, examples, VMLTypeMap, WrapVML)
}
