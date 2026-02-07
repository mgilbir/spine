package spectest

import (
	"path/filepath"
	"runtime"
	"testing"
)

func wmlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "wml_examples.json")
}

func TestWML_SpecExamples_WellFormed(t *testing.T) {
	examples := LoadExamples(t, wmlTestdataPath())
	TestWellFormedExamples(t, examples, WrapWML)
}

func TestWML_SpecExamples_Unmarshal(t *testing.T) {
	examples := LoadExamples(t, wmlTestdataPath())
	TestUnmarshalExamples(t, examples, WMLTypeMap, WrapWML)
}

func TestWML_SpecExamples_RoundTrip(t *testing.T) {
	examples := LoadExamples(t, wmlTestdataPath())
	TestRoundTripExamples(t, examples, WMLTypeMap, WrapWML)
}
