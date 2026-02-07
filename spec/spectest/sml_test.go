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
