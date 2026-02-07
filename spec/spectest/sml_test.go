package spectest

import (
	"path/filepath"
	"runtime"
	"testing"
)

func smlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "sml_examples.json")
}

func TestSML_SpecExamples_WellFormed(t *testing.T) {
	examples := LoadExamples(t, smlTestdataPath())
	TestWellFormedExamples(t, examples, WrapSML)
}

func TestSML_SpecExamples_Unmarshal(t *testing.T) {
	examples := LoadExamples(t, smlTestdataPath())
	TestUnmarshalExamples(t, examples, SMLTypeMap, WrapSML)
}

func TestSML_SpecExamples_RoundTrip(t *testing.T) {
	examples := LoadExamples(t, smlTestdataPath())
	TestRoundTripExamples(t, examples, SMLTypeMap, WrapSML)
}
