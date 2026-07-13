package opc

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
	"time"
)

// createTestPackage creates a minimal valid OPC package for testing.
func createTestPackage(t *testing.T, parts map[string][]byte, contentTypes map[string]string) []byte {
	t.Helper()

	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	for partName, content := range parts {
		ct := "application/xml"
		if c, ok := contentTypes[partName]; ok {
			ct = c
		}
		if err := w.WritePart(partName, ct, content); err != nil {
			t.Fatalf("Failed to write part %s: %v", partName, err)
		}
	}

	if _, err := w.AddRelationship(RelTypeOfficeDocument, "ppt/presentation.xml", TargetModeInternal); err != nil {
		t.Fatalf("AddRelationship() error = %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	return buf.Bytes()
}

func TestNewReader(t *testing.T) {
	parts := map[string][]byte{
		"/ppt/presentation.xml": []byte("<presentation/>"),
	}
	contentTypes := map[string]string{
		"/ppt/presentation.xml": ContentTypePresentationMain,
	}

	data := createTestPackage(t, parts, contentTypes)
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	if reader.ContentTypes == nil {
		t.Error("NewReader() ContentTypes is nil")
	}

	if len(reader.Files) == 0 {
		t.Error("NewReader() Files is empty")
	}
}

func TestReader_GetFile(t *testing.T) {
	parts := map[string][]byte{
		"/ppt/presentation.xml":  []byte("<presentation/>"),
		"/ppt/slides/slide1.xml": []byte("<slide/>"),
		"/ppt/slides/slide2.xml": []byte("<slide/>"),
	}

	data := createTestPackage(t, parts, nil)
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	// Test finding existing file
	f := reader.GetFile("/ppt/presentation.xml")
	if f == nil {
		t.Error("GetFile() returned nil for existing file")
	}

	// Test case-insensitive
	f = reader.GetFile("/PPT/PRESENTATION.XML")
	if f == nil {
		t.Error("GetFile() should be case-insensitive")
	}

	// Test non-existing file
	f = reader.GetFile("/nonexistent.xml")
	if f != nil {
		t.Error("GetFile() should return nil for non-existing file")
	}
}

func TestReader_GetRelationshipsByType(t *testing.T) {
	parts := map[string][]byte{
		"/ppt/presentation.xml": []byte("<presentation/>"),
	}

	data := createTestPackage(t, parts, nil)
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	rels := reader.GetRelationshipsByType(RelTypeOfficeDocument)
	if len(rels) != 1 {
		t.Errorf("GetRelationshipsByType() returned %d relationships, want 1", len(rels))
	}

	rels = reader.GetRelationshipsByType("http://nonexistent")
	if len(rels) != 0 {
		t.Errorf("GetRelationshipsByType() returned %d relationships for nonexistent type, want 0", len(rels))
	}
}

func TestFile_ReadAll(t *testing.T) {
	content := []byte("<test>Hello World</test>")
	parts := map[string][]byte{
		"/test.xml": content,
	}

	data := createTestPackage(t, parts, nil)
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	f := reader.GetFile("/test.xml")
	if f == nil {
		t.Fatal("GetFile() returned nil")
	}

	readContent, err := f.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("ReadAll() = %q, want %q", string(readContent), string(content))
	}
}

func TestFile_Open(t *testing.T) {
	content := []byte("<test>Streaming Content</test>")
	parts := map[string][]byte{
		"/test.xml": content,
	}

	data := createTestPackage(t, parts, nil)
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	f := reader.GetFile("/test.xml")
	rc, err := f.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = rc.Close() }()

	readContent, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("Read content = %q, want %q", string(readContent), string(content))
	}
}

func TestReader_ContentTypes(t *testing.T) {
	parts := map[string][]byte{
		"/ppt/presentation.xml":  []byte("<presentation/>"),
		"/ppt/slides/slide1.xml": []byte("<slide/>"),
	}
	contentTypes := map[string]string{
		"/ppt/presentation.xml":  ContentTypePresentationMain,
		"/ppt/slides/slide1.xml": ContentTypeSlide,
	}

	data := createTestPackage(t, parts, contentTypes)
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	// Check file content types
	presFile := reader.GetFile("/ppt/presentation.xml")
	if presFile.ContentType != ContentTypePresentationMain {
		t.Errorf("Presentation ContentType = %q, want %q", presFile.ContentType, ContentTypePresentationMain)
	}

	slideFile := reader.GetFile("/ppt/slides/slide1.xml")
	if slideFile.ContentType != ContentTypeSlide {
		t.Errorf("Slide ContentType = %q, want %q", slideFile.ContentType, ContentTypeSlide)
	}
}

func TestReader_PartRelationships(t *testing.T) {
	// Create a package with part-level relationships
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	if err := w.WritePart("/ppt/presentation.xml", ContentTypePresentationMain, []byte("<presentation/>")); err != nil {
		t.Fatalf("WritePart error: %v", err)
	}
	if err := w.WritePart("/ppt/slides/slide1.xml", ContentTypeSlide, []byte("<slide/>")); err != nil {
		t.Fatalf("WritePart error: %v", err)
	}

	// Add package-level relationship
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "ppt/presentation.xml", TargetModeInternal); err != nil {
		t.Fatalf("AddRelationship() error = %v", err)
	}

	// Add part-level relationships
	partRels := []*Relationship{
		{ID: "rId1", Type: "http://slide", Target: "slides/slide1.xml", TargetMode: TargetModeInternal},
	}
	if err := w.WritePartRelationships("/ppt/presentation.xml", partRels); err != nil {
		t.Fatalf("WritePartRelationships error: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// Read the package
	data := buf.Bytes()
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	// Get part relationships
	rels, err := reader.GetPartRelationships("/ppt/presentation.xml")
	if err != nil {
		t.Fatalf("GetPartRelationships() error = %v", err)
	}

	if len(rels) != 1 {
		t.Fatalf("GetPartRelationships() returned %d relationships, want 1", len(rels))
	}

	if rels[0].ID != "rId1" {
		t.Errorf("Relationship ID = %q, want %q", rels[0].ID, "rId1")
	}
	if rels[0].Target != "slides/slide1.xml" {
		t.Errorf("Relationship Target = %q, want %q", rels[0].Target, "slides/slide1.xml")
	}
}

func TestReader_GetPartRelationships_NoRels(t *testing.T) {
	parts := map[string][]byte{
		"/test.xml": []byte("<test/>"),
	}

	data := createTestPackage(t, parts, nil)
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	// Part without relationships should return nil, nil
	rels, err := reader.GetPartRelationships("/test.xml")
	if err != nil {
		t.Fatalf("GetPartRelationships() error = %v", err)
	}
	if rels != nil {
		t.Error("GetPartRelationships() should return nil for part without relationships")
	}
}

func TestReader_CoreProperties(t *testing.T) {
	// Create a package with core properties
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	w.Properties = &CoreProperties{
		Title:    "Test Title",
		Creator:  "Test Author",
		Created:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Modified: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
	}

	if err := w.WritePart("/ppt/presentation.xml", ContentTypePresentationMain, []byte("<presentation/>")); err != nil {
		t.Fatalf("WritePart error: %v", err)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "ppt/presentation.xml", TargetModeInternal); err != nil {
		t.Fatalf("AddRelationship() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// Read the package
	data := buf.Bytes()
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	if reader.Properties == nil {
		t.Fatal("Properties is nil")
	}

	if reader.Properties.Title != "Test Title" {
		t.Errorf("Properties.Title = %q, want %q", reader.Properties.Title, "Test Title")
	}
	if reader.Properties.Creator != "Test Author" {
		t.Errorf("Properties.Creator = %q, want %q", reader.Properties.Creator, "Test Author")
	}
}

func TestNewReader_NoContentTypes(t *testing.T) {
	// Create a zip without [Content_Types].xml
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("test.xml")
	if _, err := w.Write([]byte("<test/>")); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	data := buf.Bytes()
	_, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != ErrCorruptedPackage {
		t.Errorf("NewReader() error = %v, want %v", err, ErrCorruptedPackage)
	}
}

func TestNewReader_InvalidZip(t *testing.T) {
	data := []byte("not a zip file")
	_, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Error("NewReader() should return error for invalid zip")
	}
}

func TestReader_SkipsDirectories(t *testing.T) {
	parts := map[string][]byte{
		"/ppt/presentation.xml": []byte("<presentation/>"),
	}

	data := createTestPackage(t, parts, nil)
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	// Files should not include directory entries
	for _, f := range reader.Files {
		if f.Name[len(f.Name)-1] == '/' {
			t.Errorf("Files includes directory: %s", f.Name)
		}
	}
}

// TestNewReader_HostileEntryNames verifies that zip entries whose names are
// not in canonical form ("./"-prefixed, backslash separators, doubled
// slashes) are still reachable through GetFile under their canonical part
// names, and that the raw entries remain accessible under their original
// names (C51).
func TestNewReader_HostileEntryNames(t *testing.T) {
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="xml" ContentType="application/xml"/></Types>`

	entries := map[string]string{
		"[Content_Types].xml":    contentTypes,
		"./ppt/presentation.xml": "<presentation/>",
		`word\document.xml`:      "<document/>",
		"a//b.xml":               "<b/>",
	}

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range entries {
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatalf("CreateHeader(%q) error = %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close error = %v", err)
	}

	reader, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	cases := []struct {
		rawName   string
		canonical string
	}{
		{"./ppt/presentation.xml", "/ppt/presentation.xml"},
		{`word\document.xml`, "/word/document.xml"},
		{"a//b.xml", "/a/b.xml"},
	}
	for _, tc := range cases {
		f := reader.GetFile(tc.canonical)
		if f == nil {
			t.Errorf("GetFile(%q) = nil; raw entry %q unreachable", tc.canonical, tc.rawName)
			continue
		}
		if f.Name != tc.canonical {
			t.Errorf("File.Name = %q, want canonical %q", f.Name, tc.canonical)
		}
		got, err := f.ReadAll()
		if err != nil {
			t.Errorf("ReadAll(%q) error = %v", tc.canonical, err)
			continue
		}
		if string(got) != entries[tc.rawName] {
			t.Errorf("ReadAll(%q) = %q, want %q", tc.canonical, got, entries[tc.rawName])
		}

		// The raw entry keeps its original name for byte-preserving paths.
		raw, err := reader.GetRawZipFile(tc.rawName)
		if err != nil {
			t.Errorf("GetRawZipFile(%q) error = %v", tc.rawName, err)
			continue
		}
		if string(raw) != entries[tc.rawName] {
			t.Errorf("GetRawZipFile(%q) = %q, want %q", tc.rawName, raw, entries[tc.rawName])
		}
	}
}

func TestCanonicalZipEntryName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"ppt/presentation.xml", "/ppt/presentation.xml"},
		{"./ppt/presentation.xml", "/ppt/presentation.xml"},
		{"././a.xml", "/a.xml"},
		{`word\document.xml`, "/word/document.xml"},
		{"a//b.xml", "/a/b.xml"},
		{"/leading.xml", "/leading.xml"},
		{"dir/", "/dir/"},
	}
	for _, tt := range tests {
		if got := canonicalZipEntryName(tt.in); got != tt.want {
			t.Errorf("canonicalZipEntryName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
