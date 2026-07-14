package xlsx

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// retypedWorkbook builds a minimal workbook and rewrites its main-part
// content-type override to the given SpreadsheetML flavor.
func retypedWorkbook(t *testing.T, flavor string) []byte {
	t.Helper()
	w := Create()
	sheet := w.AddSheet("Sheet1")
	if err := sheet.SetCellValue("A1", "x"); err != nil {
		t.Fatal(err)
	}
	data, err := w.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	ct := string(readZipPart(t, data, "[Content_Types].xml"))
	if !strings.Contains(ct, opc.ContentTypeWorkbook) {
		t.Fatal("[Content_Types].xml has no workbook main-part override")
	}
	return replaceZipEntry(t, data, "[Content_Types].xml",
		strings.Replace(ct, opc.ContentTypeWorkbook, flavor, 1))
}

// TestOpenFlavorVariants opens each SpreadsheetML main-part flavor, checks
// the reported flavor, and verifies a zero-modification save keeps the flavor
// (an .xlsm retyped to .xlsx while carrying vbaProject.bin trips Excel's
// security checks).
func TestOpenFlavorVariants(t *testing.T) {
	flavors := []string{
		opc.ContentTypeWorkbook,
		opc.ContentTypeWorkbookTemplateMain,
		opc.ContentTypeWorkbookMacroMain,
		opc.ContentTypeWorkbookTemplateMacroMain,
		opc.ContentTypeWorkbookAddinMacroMain,
	}
	for _, flavor := range flavors {
		t.Run(flavor, func(t *testing.T) {
			data := retypedWorkbook(t, flavor)
			w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			defer func() { _ = w.Close() }()
			if got := w.Flavor(); got != flavor {
				t.Fatalf("Flavor() = %q, want %q", got, flavor)
			}

			saved, err := w.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			ct := string(readZipPart(t, saved, "[Content_Types].xml"))
			if !strings.Contains(ct, flavor) {
				t.Fatalf("saved [Content_Types].xml lost the %q flavor:\n%s", flavor, ct)
			}
			if flavor != opc.ContentTypeWorkbook && strings.Contains(ct, opc.ContentTypeWorkbook) {
				t.Fatalf("saved [Content_Types].xml retyped the main part to a regular workbook:\n%s", ct)
			}

			reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
			if err != nil {
				t.Fatalf("reopening saved package: %v", err)
			}
			defer func() { _ = reopened.Close() }()
			if got := reopened.Flavor(); got != flavor {
				t.Fatalf("reopened Flavor() = %q, want %q", got, flavor)
			}
		})
	}
}

// TestOpenRejectsUnknownMainPartContentType keeps the strict rejection for
// content types outside the SpreadsheetML family.
func TestOpenRejectsUnknownMainPartContentType(t *testing.T) {
	data := retypedWorkbook(t, "application/x-not-a-workbook")
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrNotXLSX) {
		t.Fatalf("OpenReader error = %v, want ErrNotXLSX", err)
	}
}

// TestCreatedWorkbookFlavor pins the default flavor for created workbooks.
func TestCreatedWorkbookFlavor(t *testing.T) {
	if got := Create().Flavor(); got != opc.ContentTypeWorkbook {
		t.Fatalf("Create().Flavor() = %q, want %q", got, opc.ContentTypeWorkbook)
	}
}
