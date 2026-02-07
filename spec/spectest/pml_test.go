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
