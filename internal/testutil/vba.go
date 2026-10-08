package testutil

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// VBA signature fixture parts. Office stores a signed VBA project's signatures
// in parts the project's own relationships (vbaProject.bin.rels) target, and
// Word adds its macro data part (vbaData.xml) there too.
const (
	vbaRelsHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
	vbaSignatureRel = `<Relationship Id="rId1" Type="http://schemas.microsoft.com/office/2006/relationships/vbaProjectSignature" Target="vbaProjectSignature.bin"/>`
	vbaDataRel      = `<Relationship Id="rId2" Type="http://schemas.microsoft.com/office/2006/relationships/wordVbaData" Target="vbaData.xml"/>`
	vbaDataXML      = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<wne:vbaSuppData xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml"/>`
)

// AddVBASignature returns pkg with the parts a signed VBA project carries
// added next to its vbaProject.bin in dir ("word", "ppt" or "xl"): a
// vbaProject.bin.rels targeting vbaProjectSignature.bin and, with wordData, a
// vbaData.xml, each with its content-type override. The package must already
// carry dir/vbaProject.bin.
func AddVBASignature(t *testing.T, pkg []byte, dir string, wordData bool) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("AddVBASignature: open package: %v", err)
	}
	rels := vbaRelsHeader + vbaSignatureRel
	overrides := `<Override PartName="/` + dir + `/vbaProjectSignature.bin" ContentType="application/vnd.ms-office.vbaProjectSignature"/>`
	added := map[string]string{dir + "/vbaProjectSignature.bin": "signature"}
	if wordData {
		rels += vbaDataRel
		overrides += `<Override PartName="/` + dir + `/vbaData.xml" ContentType="application/vnd.ms-word.vbaData+xml"/>`
		added[dir+"/vbaData.xml"] = vbaDataXML
	}
	added[dir+"/_rels/vbaProject.bin.rels"] = rels + `</Relationships>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	sawProject := false
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("AddVBASignature: open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("AddVBASignature: read %s: %v", f.Name, err)
		}
		switch f.Name {
		case "[Content_Types].xml":
			data = []byte(strings.Replace(string(data), "</Types>", overrides+"</Types>", 1))
		case dir + "/vbaProject.bin":
			sawProject = true
		}
		writeZipEntry(t, zw, f.Name, data)
	}
	if !sawProject {
		t.Fatalf("AddVBASignature: package has no %s/vbaProject.bin", dir)
	}
	for _, name := range SortedKeys(stringMapBytes(added)) {
		writeZipEntry(t, zw, name, []byte(added[name]))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("AddVBASignature: close: %v", err)
	}
	return buf.Bytes()
}

func writeZipEntry(t *testing.T, zw *zip.Writer, name string, data []byte) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func stringMapBytes(m map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(m))
	for k, v := range m {
		out[k] = []byte(v)
	}
	return out
}

// UseBinDefaultForVBA returns pkg with the VBA project's content type declared
// the way Office often writes it: a Default for the "bin" extension instead of
// an Override for dir/vbaProject.bin.
func UseBinDefaultForVBA(t *testing.T, pkg []byte, dir string) []byte {
	t.Helper()
	const vbaCT = "application/vnd.ms-office.vbaProject"
	return rewriteEntry(t, pkg, "[Content_Types].xml", func(ct string) string {
		override := `<Override PartName="/` + dir + `/vbaProject.bin" ContentType="` + vbaCT + `"/>`
		if !strings.Contains(ct, override) {
			t.Fatalf("UseBinDefaultForVBA: no override for %s/vbaProject.bin in:\n%s", dir, ct)
		}
		if strings.Contains(strings.ToLower(ct), `extension="bin"`) {
			t.Fatalf("UseBinDefaultForVBA: the package already has a bin default:\n%s", ct)
		}
		ct = strings.Replace(ct, override, "", 1)
		i := strings.Index(ct, "<Default ")
		if i < 0 {
			t.Fatalf("UseBinDefaultForVBA: no Default entry in:\n%s", ct)
		}
		return ct[:i] + `<Default Extension="bin" ContentType="` + vbaCT + `"/>` + ct[i:]
	})
}

// rewriteEntry returns pkg with one entry's content passed through edit.
func rewriteEntry(t *testing.T, pkg []byte, name string, edit func(string) string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("rewriteEntry: open package: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("rewriteEntry: open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("rewriteEntry: read %s: %v", f.Name, err)
		}
		if f.Name == name {
			data = []byte(edit(string(data)))
		}
		writeZipEntry(t, zw, f.Name, data)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("rewriteEntry: close: %v", err)
	}
	return buf.Bytes()
}
