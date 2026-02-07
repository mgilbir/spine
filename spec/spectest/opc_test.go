package spectest

import (
	"path/filepath"
	"runtime"
	"testing"
)

func opcTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "opc_examples.json")
}

// WrapOPC wraps an OPC XML fragment with common namespace declarations.
func WrapOPC(xmlStr string) string {
	return `<wrapper` +
		` xmlns="http://schemas.openxmlformats.org/package/2006/content-types"` +
		` xmlns:r="` + NsRelationship + `"` +
		` xmlns:coreProperties="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"` +
		` xmlns:dc="http://purl.org/dc/elements/1.1/"` +
		` xmlns:dcterms="http://purl.org/dc/terms/"` +
		` xmlns:Relationships="http://schemas.openxmlformats.org/package/2006/relationships"` +
		`>` + xmlStr + `</wrapper>`
}

func TestOPC_SpecExamples_WellFormed(t *testing.T) {
	examples := LoadExamples(t, opcTestdataPath())
	TestWellFormedExamples(t, examples, WrapOPC)
}
