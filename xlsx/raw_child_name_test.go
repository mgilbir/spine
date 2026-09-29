package xlsx

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzseed"
)

// The xlsx twin of the pptx slide case. An unknown worksheet child named ":"
// is well-formed XML that Go reads and Excel does not; in a sheet whose root
// binds SpreadsheetML as both the default and x:, an edited save rebuilt it as
// "<x::/>" and wrote a sheet nothing could reopen. Now, as in docx and pptx:
// the workbook opens, an untouched sheet passes through byte for byte, and an
// edited save refuses with ErrUnwritableName.
func TestWorksheetChildWithNonQNameRefusesAnEditedSave(t *testing.T) {
	const sml = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	const part = "xl/worksheets/sheet1.xml"
	w := Create()
	if err := addSheetT(w, "Sheet1").SetCellValue("A1", "a"); err != nil {
		t.Fatal(err)
	}
	base, err := w.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	sheet := string(fuzzseed.ZipEntry(base, part))
	if !strings.Contains(sheet, `xmlns="`+sml+`"`) || !strings.HasSuffix(sheet, "</worksheet>") {
		t.Fatalf("fixture sheet has an unexpected shape:\n%s", sheet)
	}
	sheet = strings.Replace(sheet, `xmlns="`+sml+`"`, `xmlns="`+sml+`" xmlns:x="`+sml+`"`, 1)
	sheet = strings.TrimSuffix(sheet, "</worksheet>") + `<:/></worksheet>`
	pkg := fuzzseed.ReplaceZipEntry(base, part, []byte(sheet))

	open := func() *Workbook {
		t.Helper()
		w, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
		if err != nil {
			t.Fatalf("a workbook with a non-QName sheet child no longer opens: %v", err)
		}
		return w
	}

	out, err := open().SaveBytes()
	if err != nil {
		t.Fatalf("an untouched workbook no longer saves: %v", err)
	}
	if got := string(fuzzseed.ZipEntry(out, part)); got != sheet {
		t.Errorf("untouched sheet changed on save\nwant: %s\n got: %s", sheet, got)
	}

	edited := open()
	if err := edited.Sheets()[0].SetCellValue("B2", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := edited.SaveBytes(); !errors.Is(err, xmlb.ErrUnwritableName) {
		t.Fatalf("edited save = %v, want ErrUnwritableName", err)
	}
}
