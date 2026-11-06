package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/internal/testutil"
)

// isLimitError reports whether err is a decompression-limit violation: it
// must match ErrDecompressionLimit and still carry its specific message. The
// zip-bomb tests assert through it, so every limit site they reach — including
// the lying-header and streaming ones the declared-size checks never see — is
// held to the sentinel.
func isLimitError(err error, msg string) bool {
	return errors.Is(err, ErrDecompressionLimit) && strings.Contains(err.Error(), msg)
}

// propertiesPackage builds a package carrying all three property parts
// (core, extended, custom) next to a trivial main part.
func propertiesPackage(t *testing.T) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	w := NewWriter(buf)
	w.Properties = &CoreProperties{
		Title:    "T",
		Created:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Modified: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
	}
	w.ExtendedProperties = &ExtendedProperties{Application: "TestApp"}
	w.CustomProperties = &CustomProperties{}
	if err := w.CustomProperties.Set("Project", "spine"); err != nil {
		t.Fatal(err)
	}
	if err := w.WritePart("/ppt/presentation.xml", ContentTypePresentationMain, []byte("<presentation/>")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "ppt/presentation.xml", TargetModeInternal); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestNewReader_UnreadablePropertyPartFailsOpen: a property part that is
// present but cannot be read must fail the open with the read error, never be
// treated as absent. Treated as absent, the format packages saw no properties
// and the next save dropped or regenerated the part.
func TestNewReader_UnreadablePropertyPartFailsOpen(t *testing.T) {
	pkg := propertiesPackage(t)

	// The fixture must parse cleanly first, or the cases below prove nothing.
	r, err := NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("intact fixture: %v", err)
	}
	if r.Properties == nil || r.ExtendedProperties == nil || r.CustomProperties == nil {
		t.Fatalf("intact fixture: properties not all parsed: core=%v app=%v custom=%v",
			r.Properties != nil, r.ExtendedProperties != nil, r.CustomProperties != nil)
	}

	for _, part := range []string{"docProps/core.xml", "docProps/app.xml", "docProps/custom.xml"} {
		t.Run(part+"/oversized", func(t *testing.T) {
			bad := testutil.WithOversizedEntry(t, pkg, part)
			_, err := NewReader(bytes.NewReader(bad), int64(len(bad)))
			if err == nil {
				t.Fatal("NewReader succeeded with an unreadable property part")
			}
			if !errors.Is(err, ErrDecompressionLimit) {
				t.Errorf("error does not wrap ErrDecompressionLimit: %v", err)
			}
			if !strings.Contains(err.Error(), part) {
				t.Errorf("error does not name the part %s: %v", part, err)
			}
		})
		t.Run(part+"/badcrc", func(t *testing.T) {
			bad := testutil.WithBadCRC(t, pkg, part)
			_, err := NewReader(bytes.NewReader(bad), int64(len(bad)))
			if !errors.Is(err, zip.ErrChecksum) {
				t.Errorf("NewReader error = %v, want one wrapping zip.ErrChecksum", err)
			}
		})
	}
}

// TestNewReader_MissingPropertyPartStillOpens pins the leniency the fix must
// keep: a property relationship whose target is not in the package is an
// optional absence, not an error.
func TestNewReader_MissingPropertyPartStillOpens(t *testing.T) {
	pkg := propertiesPackage(t)
	for _, part := range []string{"docProps/core.xml", "docProps/app.xml", "docProps/custom.xml"} {
		t.Run(part, func(t *testing.T) {
			without := testutil.WithoutEntry(t, pkg, part)
			if _, err := NewReader(bytes.NewReader(without), int64(len(without))); err != nil {
				t.Fatalf("NewReader with %s absent: %v", part, err)
			}
		})
	}
}

// TestDecompressionLimitErrorsMatchSentinel: every decompression-limit
// violation matches ErrDecompressionLimit while keeping its specific message.
func TestDecompressionLimitErrorsMatchSentinel(t *testing.T) {
	big := bytes.Repeat([]byte("A"), 64*1024)
	data := createTestPackage(t, map[string][]byte{"/ppt/presentation.xml": big},
		map[string]string{"/ppt/presentation.xml": ContentTypePresentationMain})

	cases := []struct {
		name string
		opts []ReaderOption
		want string
	}{
		{"per-part", []ReaderOption{WithMaxDecompressedPartSize(1024)}, "per-part decompression limit"},
		{"package", []ReaderOption{WithMaxDecompressedPackageSize(4096)}, "package decompression limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewReader(bytes.NewReader(data), int64(len(data)), tc.opts...)
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}
			f := r.GetFile("/ppt/presentation.xml")
			if f == nil {
				t.Fatal("part not found")
			}
			_, err = f.ReadAll()
			if !errors.Is(err, ErrDecompressionLimit) {
				t.Errorf("ReadAll error = %v, want ErrDecompressionLimit", err)
			}
			if err != nil && !strings.Contains(err.Error(), tc.want) {
				t.Errorf("ReadAll error lost its message: %v", err)
			}
		})
	}
}

// TestVerifySignatures_UnreadableOriginRelsIsError: the origin relationships
// part names the signature parts, which may live outside /_xmlsignatures/.
// Skipping it when unreadable reported fewer signatures than the package holds.
func TestVerifySignatures_UnreadableOriginRelsIsError(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)
	if err := w.WritePart("/ppt/presentation.xml", ContentTypePresentationMain, []byte("<presentation/>")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "ppt/presentation.xml", TargetModeInternal); err != nil {
		t.Fatal(err)
	}
	if err := w.WritePart("/_xmlsignatures/origin.sigs", "application/vnd.openxmlformats-package.digital-signature-origin", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddRelationship(RelTypeDigitalSignatureOrigin, "_xmlsignatures/origin.sigs", TargetModeInternal); err != nil {
		t.Fatal(err)
	}
	originRels := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`)
	if err := w.WritePart("/_xmlsignatures/_rels/origin.sigs.rels", ContentTypeRelationships, originRels); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	pkg := buf.Bytes()

	r, err := NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("intact fixture: %v", err)
	}
	if _, err := r.VerifySignatures(); err != nil {
		t.Fatalf("intact fixture: VerifySignatures: %v", err)
	}

	bad := testutil.WithOversizedEntry(t, pkg, "_xmlsignatures/_rels/origin.sigs.rels")
	r, err = NewReader(bytes.NewReader(bad), int64(len(bad)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	if _, err := r.VerifySignatures(); !errors.Is(err, ErrDecompressionLimit) {
		t.Errorf("VerifySignatures error = %v, want ErrDecompressionLimit", err)
	}
}
