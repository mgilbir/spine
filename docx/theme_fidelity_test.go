package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// themeWithExtensionsXML is an Office 2013+ style theme part: it carries the
// thm15:themeFamily extension every modern theme has plus a custom color list,
// both of which dml.Theme did not model before C374.
const themeWithExtensionsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme"><a:themeElements><a:clrScheme name="Office"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4><a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6><a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink></a:clrScheme><a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme><a:fmtScheme name="Office"><a:fillStyleLst><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"/></a:gs></a:gsLst></a:gradFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="100000"><a:schemeClr val="phClr"/></a:gs></a:gsLst></a:gradFill></a:fillStyleLst><a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/><a:custClrLst><a:custClr name="Brand Red"><a:srgbClr val="CC0000"/></a:custClr></a:custClrLst><a:extLst><a:ext uri="{05A4C25C-085E-4340-85A3-A5531E510DB2}"><thm15:themeFamily xmlns:thm15="http://schemas.microsoft.com/office/thememl/2012/main" name="Office Theme" id="{62F939B6-93AF-4DB8-9C6B-D6C7DFDC589F}" vid="{4A3C46E8-61CC-4603-A589-7422A47A8E4A}"/></a:ext></a:extLst></a:theme>`

// TestThemeEditKeepsExtensionsDocx is C374 through the public API: open a real
// document whose theme carries the extensions Office writes, rename the theme,
// save, and check the extensions are still in the saved part. Before the fix
// the one-line SetName deleted thm15:themeFamily and the whole custClrLst,
// because ThemeEditor.Marshal regenerates the part from a model that could not
// represent either.
func TestThemeEditKeepsExtensionsDocx(t *testing.T) {
	src := replaceZipEntry(t, "testdata/chart.docx", "word/theme/theme1.xml", []byte(themeWithExtensionsXML))

	doc, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	theme := doc.Theme()
	if theme == nil {
		t.Fatal("Theme() = nil")
	}
	theme.SetName("Renamed Theme")

	out, err := doc.SaveBytes()
	_ = doc.Close()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	saved := zipPart(t, out, "word/theme/theme1.xml")

	if !strings.Contains(saved, `name="Renamed Theme"`) {
		t.Errorf("the rename did not reach the saved part")
	}
	for _, want := range []string{"thm15:themeFamily", "a:custClrLst", `name="Brand Red"`, "CC0000"} {
		if !strings.Contains(saved, want) {
			t.Errorf("saved theme lost %s after SetName (C374)", want)
		}
	}
	// C401 through the same path: the fill style list is positional, so the
	// interleaved gradFill/solidFill/gradFill must not come back regrouped.
	i := strings.Index(saved, "<a:fillStyleLst>")
	j := strings.Index(saved, "</a:fillStyleLst>")
	if i < 0 || j < 0 {
		t.Fatal("no fillStyleLst in the saved theme")
	}
	fills := saved[i:j]
	wantOrder := []string{"<a:gradFill", "<a:solidFill", "<a:gradFill"}
	pos := 0
	for _, w := range wantOrder {
		k := strings.Index(fills[pos:], w)
		if k < 0 {
			t.Fatalf("fillStyleLst regrouped by kind (C401): %s", fills)
		}
		pos += k + len(w)
	}
}

// TestThemeUnmodifiedStillByteIdenticalDocx guards the other half of the
// promise: the extension modeling must not disturb the preserved-bytes path
// for a theme nobody touched.
func TestThemeUnmodifiedStillByteIdenticalDocx(t *testing.T) {
	src := replaceZipEntry(t, "testdata/chart.docx", "word/theme/theme1.xml", []byte(themeWithExtensionsXML))
	orig := zipPart(t, src, "word/theme/theme1.xml")

	doc, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer func() { _ = doc.Close() }()
	if theme := doc.Theme(); theme == nil {
		t.Fatal("Theme() = nil")
	} else if theme.Modified() {
		t.Fatal("read-only access marked the theme modified")
	}
	out, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := zipPart(t, out, "word/theme/theme1.xml"); got != orig {
		t.Errorf("unmodified theme part not byte-identical:\n got %s\nwant %s", got, orig)
	}
}

// replaceZipEntry returns the package at path with one entry's bytes replaced,
// so a fixture can exercise theme content the committed files do not carry.
func replaceZipEntry(t *testing.T, path, name string, data []byte) []byte {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	replaced := false
	for _, f := range zr.File {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: f.Method})
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == name {
			replaced = true
			if _, err := w.Write(data); err != nil {
				t.Fatal(err)
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(w, rc); err != nil {
			t.Fatal(err)
		}
		_ = rc.Close()
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if !replaced {
		t.Fatalf("entry %q not found in %s", name, path)
	}
	return buf.Bytes()
}
