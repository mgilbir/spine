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

// vmlOutOfScope lists elements that appear in VML spec examples but are not
// VML content types.
var vmlOutOfScope = map[string]string{
	"object": "WML w:object element, not VML content",
	"":       "Example with no identifiable root element",
}

func TestVML_SpecExamples_WellFormed(t *testing.T) {
	examples := LoadExamples(t, vmlTestdataPath())
	TestWellFormedExamples(t, examples, WrapVML)
}

func TestVML_SpecExamples_Unmarshal(t *testing.T) {
	examples := LoadExamples(t, vmlTestdataPath())
	TestUnmarshalExamplesWithSkips(t, examples, VMLTypeMap, WrapVML, vmlOutOfScope)
}

func TestVML_SpecExamples_RoundTrip(t *testing.T) {
	examples := LoadExamples(t, vmlTestdataPath())
	TestRoundTripExamplesWithSkips(t, examples, VMLTypeMap, WrapVML, nil, vmlOutOfScope)
}
