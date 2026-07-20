package xlsx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/opc"
)

var testVBABytes = []byte("fake vbaProject.bin blob \x00\x01\x02MSVBA")

func TestVBAInjectRoundTrip(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if wb.HasMacros() {
		t.Fatal("plain workbook should report no macros")
	}
	if wb.Flavor() != opc.ContentTypeWorkbook {
		t.Fatalf("flavor = %q, want workbook", wb.Flavor())
	}

	wb.SetVBAProject(testVBABytes)
	if !wb.HasMacros() {
		t.Fatal("HasMacros false after SetVBAProject")
	}
	if wb.Flavor() != opc.ContentTypeWorkbookMacroMain {
		t.Fatalf("flavor = %q, want macro-enabled after inject", wb.Flavor())
	}

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if !re.HasMacros() {
		t.Fatal("reopened workbook reports no macros")
	}
	if re.Flavor() != opc.ContentTypeWorkbookMacroMain {
		t.Fatalf("reopened flavor = %q, want macro-enabled", re.Flavor())
	}
	if got := re.VBAProject(); !bytes.Equal(got, testVBABytes) {
		t.Fatalf("VBAProject = %q, want %q", got, testVBABytes)
	}
}

func TestVBAUnmodifiedRoundTripByteIdentical(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	wb.SetVBAProject(testVBABytes)
	macroWb, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(macroWb), int64(len(macroWb)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	before := append([]byte(nil), re.VBAProject()...)
	again, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes 2: %v", err)
	}
	re2, err := OpenReader(bytes.NewReader(again), int64(len(again)))
	if err != nil {
		t.Fatalf("OpenReader 2: %v", err)
	}
	if !bytes.Equal(re2.VBAProject(), before) {
		t.Fatal("vbaProject bytes drifted across an unmodified round-trip")
	}
	if !re2.HasMacros() || re2.Flavor() != opc.ContentTypeWorkbookMacroMain {
		t.Fatal("macro flavor not preserved across unmodified round-trip")
	}
}

func TestVBARemove(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	wb.SetVBAProject(testVBABytes)
	macroWb, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(macroWb), int64(len(macroWb)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	re.RemoveVBAProject()
	if re.HasMacros() {
		t.Fatal("HasMacros true after RemoveVBAProject")
	}
	if re.Flavor() != opc.ContentTypeWorkbook {
		t.Fatalf("flavor = %q, want plain workbook after removal", re.Flavor())
	}

	out, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after remove: %v", err)
	}
	re2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("OpenReader after remove: %v", err)
	}
	if re2.HasMacros() {
		t.Fatal("reopened workbook still reports macros after removal")
	}
	if re2.Flavor() != opc.ContentTypeWorkbook {
		t.Fatalf("reopened flavor = %q, want plain workbook", re2.Flavor())
	}
}
