package spectest

import (
	"path/filepath"
	"runtime"
	"testing"
)

func mcTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "mc_examples.json")
}

// WrapMC wraps an MC XML fragment with common namespace declarations.
// MC examples use illustrative namespaces (Circles, v1, v2) that may not resolve.
func WrapMC(xmlStr string) string {
	return `<wrapper` +
		` xmlns:mc="` + NsMC + `"` +
		` xmlns:v1="http://schemas.openxmlformats.org/Circles/v1"` +
		` xmlns:v2="http://schemas.openxmlformats.org/Circles/v2"` +
		` xmlns:v3="http://schemas.openxmlformats.org/Circles/v3"` +
		`>` + xmlStr + `</wrapper>`
}

func TestMC_SpecExamples_WellFormed(t *testing.T) {
	examples := LoadExamples(t, mcTestdataPath())
	TestWellFormedExamples(t, examples, WrapMC)
}
