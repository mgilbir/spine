package opc

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNewWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	if w == nil {
		t.Fatal("NewWriter() returned nil")
	}
	if w.ContentTypes == nil {
		t.Error("NewWriter() ContentTypes is nil")
	}
	if w.Relationships == nil {
		t.Error("NewWriter() Relationships is nil")
	}
	if w.closed {
		t.Error("NewWriter() returned closed writer")
	}
}

func TestWriter_CreatePart(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	writer, err := w.CreatePart("/test/document.xml", "application/xml", CompressionDeflate)
	if err != nil {
		t.Fatalf("CreatePart() error = %v", err)
	}

	content := []byte("<test>Hello World</test>")
	n, err := writer.Write(content)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(content) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(content))
	}

	// Verify content type was registered
	ct := w.ContentTypes.GetContentType("/test/document.xml")
	if ct != "application/xml" {
		t.Errorf("ContentType = %q, want %q", ct, "application/xml")
	}

	w.Close()
}

func TestWriter_CreatePart_InvalidName(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	_, err := w.CreatePart("no-leading-slash.xml", "application/xml", CompressionDeflate)
	if err != ErrInvalidPartName {
		t.Errorf("CreatePart() error = %v, want %v", err, ErrInvalidPartName)
	}
}

func TestWriter_CreatePart_Duplicate(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	_, err := w.CreatePart("/test.xml", "application/xml", CompressionDeflate)
	if err != nil {
		t.Fatalf("First CreatePart() error = %v", err)
	}

	_, err = w.CreatePart("/test.xml", "application/xml", CompressionDeflate)
	if err != ErrDuplicatePart {
		t.Errorf("Second CreatePart() error = %v, want %v", err, ErrDuplicatePart)
	}
}

func TestWriter_CreatePart_CaseInsensitiveDuplicate(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	_, err := w.CreatePart("/Test.xml", "application/xml", CompressionDeflate)
	if err != nil {
		t.Fatalf("First CreatePart() error = %v", err)
	}

	_, err = w.CreatePart("/TEST.xml", "application/xml", CompressionDeflate)
	if err != ErrDuplicatePart {
		t.Errorf("Case-different CreatePart() error = %v, want %v", err, ErrDuplicatePart)
	}
}

func TestWriter_WritePart(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	content := []byte("<document>Test Content</document>")
	err := w.WritePart("/document.xml", "application/xml", content)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}

	w.Close()

	// Verify the part exists in the zip
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	found := false
	for _, f := range reader.File {
		if f.Name == "document.xml" {
			found = true
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			if string(data) != string(content) {
				t.Errorf("Part content = %q, want %q", string(data), string(content))
			}
		}
	}
	if !found {
		t.Error("Part not found in zip")
	}
}

func TestWriter_AddRelationship(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	rel1 := w.AddRelationship(RelTypeOfficeDocument, "ppt/presentation.xml", TargetModeInternal)
	rel2 := w.AddRelationship(RelTypeCore, "docProps/core.xml", TargetModeInternal)
	rel3 := w.AddRelationship("http://external", "https://example.com", TargetModeExternal)

	if rel1.ID != "rId1" {
		t.Errorf("First relationship ID = %q, want %q", rel1.ID, "rId1")
	}
	if rel2.ID != "rId2" {
		t.Errorf("Second relationship ID = %q, want %q", rel2.ID, "rId2")
	}
	if rel3.ID != "rId3" {
		t.Errorf("Third relationship ID = %q, want %q", rel3.ID, "rId3")
	}

	if len(w.Relationships) != 3 {
		t.Errorf("Relationships count = %d, want 3", len(w.Relationships))
	}
}

func TestWriter_Close(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	w.WritePart("/test.xml", "application/xml", []byte("<test/>"))
	w.AddRelationship(RelTypeOfficeDocument, "test.xml", TargetModeInternal)

	err := w.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify the zip is valid
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	// Check required files exist
	requiredFiles := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"test.xml",
	}

	fileMap := make(map[string]bool)
	for _, f := range reader.File {
		fileMap[f.Name] = true
	}

	for _, required := range requiredFiles {
		if !fileMap[required] {
			t.Errorf("Missing required file: %s", required)
		}
	}
}

func TestWriter_Close_Twice(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	err := w.Close()
	if err != nil {
		t.Fatalf("First Close() error = %v", err)
	}

	err = w.Close()
	if err != ErrPackageClosed {
		t.Errorf("Second Close() error = %v, want %v", err, ErrPackageClosed)
	}
}

func TestWriter_CreatePart_AfterClose(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)
	w.Close()

	_, err := w.CreatePart("/test.xml", "application/xml", CompressionDeflate)
	if err != ErrPackageClosed {
		t.Errorf("CreatePart() after Close() error = %v, want %v", err, ErrPackageClosed)
	}
}

func TestWriter_WithCoreProperties(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	w.Properties = &CoreProperties{
		Title:    "Test Document",
		Creator:  "Test Author",
		Created:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Modified: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
	}

	w.WritePart("/test.xml", "application/xml", []byte("<test/>"))
	w.Close()

	// Verify the zip contains core properties
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	found := false
	for _, f := range reader.File {
		if f.Name == "docProps/core.xml" {
			found = true
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()

			content := string(data)
			if !strings.Contains(content, "Test Document") {
				t.Error("Core properties missing title")
			}
			if !strings.Contains(content, "Test Author") {
				t.Error("Core properties missing creator")
			}
		}
	}
	if !found {
		t.Error("Core properties file not found in zip")
	}
}

func TestWriter_WritePartRelationships(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	w.WritePart("/ppt/presentation.xml", ContentTypePresentationMain, []byte("<presentation/>"))

	rels := []*Relationship{
		{ID: "rId1", Type: "http://slide", Target: "slides/slide1.xml", TargetMode: TargetModeInternal},
		{ID: "rId2", Type: "http://slide", Target: "slides/slide2.xml", TargetMode: TargetModeInternal},
	}

	err := w.WritePartRelationships("/ppt/presentation.xml", rels)
	if err != nil {
		t.Fatalf("WritePartRelationships() error = %v", err)
	}

	w.Close()

	// Verify the relationships file exists
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	found := false
	for _, f := range reader.File {
		if f.Name == "ppt/_rels/presentation.xml.rels" {
			found = true
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()

			content := string(data)
			if !strings.Contains(content, "rId1") {
				t.Error("Part relationships missing rId1")
			}
			if !strings.Contains(content, "rId2") {
				t.Error("Part relationships missing rId2")
			}
		}
	}
	if !found {
		t.Error("Part relationships file not found in zip")
	}
}

func TestWriter_CompressionNone(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	content := []byte("<uncompressed>This content should not be compressed</uncompressed>")
	writer, err := w.CreatePart("/uncompressed.xml", "application/xml", CompressionNone)
	if err != nil {
		t.Fatalf("CreatePart() error = %v", err)
	}
	writer.Write(content)
	w.Close()

	// Verify the file is stored (not deflated)
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	for _, f := range reader.File {
		if f.Name == "uncompressed.xml" {
			if f.Method != zip.Store {
				t.Errorf("File method = %d, want %d (Store)", f.Method, zip.Store)
			}
		}
	}
}
