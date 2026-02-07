package spectest

import (
	"path/filepath"
	"runtime"
	"testing"
)

func pmlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "pml_examples.json")
}

func TestPML_SpecExamples_WellFormed(t *testing.T) {
	examples := LoadExamples(t, pmlTestdataPath())
	TestWellFormedExamples(t, examples, WrapPML)
}

func TestPML_SpecExamples_Unmarshal(t *testing.T) {
	examples := LoadExamples(t, pmlTestdataPath())
	TestUnmarshalExamples(t, examples, PMLTypeMap, WrapPML)
}

func TestPML_SpecExamples_RoundTrip(t *testing.T) {
	examples := LoadExamples(t, pmlTestdataPath())
	TestRoundTripExamples(t, examples, PMLTypeMap, WrapPML)
}
