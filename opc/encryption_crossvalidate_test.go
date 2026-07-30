package opc

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// findMsoffcrypto locates the external msoffcrypto-tool used for cross-validation.
// It checks $MSOFFCRYPTO_TOOL and PATH; when absent the caller skips the test so
// the suite stays green in environments without the Python tool installed.
func findMsoffcrypto(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("MSOFFCRYPTO_TOOL"); p != "" {
		return p
	}
	if p, err := exec.LookPath("msoffcrypto-tool"); err == nil {
		return p
	}
	t.Skip("msoffcrypto-tool not available; set MSOFFCRYPTO_TOOL or add it to PATH to cross-validate encryption")
	return ""
}

// TestEncryptedCrossValidateWithMsoffcrypto encrypts a package with every scheme
// and DataSpaces variant this library produces, then confirms the independent
// msoffcrypto-tool decrypts each one back to the exact plaintext. This checks the
// CFB container, the agile/standard crypto, and the optional DataSpaces streams
// against a reference implementation rather than only against ourselves.
func TestEncryptedCrossValidateWithMsoffcrypto(t *testing.T) {
	tool := findMsoffcrypto(t)

	plain, err := os.ReadFile(filepath.Join("..", "docx", "testdata", "minimal.docx"))
	if err != nil {
		t.Fatalf("reading fixture package: %v", err)
	}
	const password = "Cross-Validate!123"

	cases := []struct {
		name string
		opts EncryptOptions
	}{
		{"agile", EncryptOptions{Scheme: SchemeAgile}},
		{"agile+dataspaces", EncryptOptions{Scheme: SchemeAgile, IncludeDataSpaces: true}},
		{"standard-256", EncryptOptions{Scheme: SchemeStandard, StandardKeyBits: 256}},
		{"standard-128", EncryptOptions{Scheme: SchemeStandard, StandardKeyBits: 128}},
		{"standard-192+dataspaces", EncryptOptions{Scheme: SchemeStandard, StandardKeyBits: 192, IncludeDataSpaces: true}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var enc bytes.Buffer
			if err := SaveEncryptedWithOptions(&enc, plain, password, c.opts); err != nil {
				t.Fatalf("SaveEncryptedWithOptions: %v", err)
			}

			dir := t.TempDir()
			in := filepath.Join(dir, "enc.bin")
			out := filepath.Join(dir, "dec.zip")
			if err := os.WriteFile(in, enc.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command(tool, "-p", password, in, out)
			if outBytes, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("msoffcrypto-tool failed: %v\n%s", err, outBytes)
			}
			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("reading msoffcrypto output: %v", err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatalf("%s: msoffcrypto-decrypted bytes differ from original (%d vs %d bytes)", c.name, len(got), len(plain))
			}

			// And confirm our own reader recovers the same package.
			r, err := OpenEncrypted(bytes.NewReader(enc.Bytes()), int64(enc.Len()), password)
			if err != nil {
				t.Fatalf("OpenEncrypted: %v", err)
			}
			if ct := r.GetFile("/word/document.xml"); ct == nil {
				t.Fatalf("%s: decrypted package missing /word/document.xml", c.name)
			}
		})
	}
}

// TestEncryptedDIFATCrossValidateWithMsoffcrypto covers the chained-DIFAT branch
// of the CFB writer against an independent implementation.
//
// A CFB file stores the locations of its first 109 FAT sectors in the header. A
// file needing more than that must spill the rest into chained DIFAT sectors,
// which writeDIFATSectors emits — and at 512-byte sectors with 128 FAT entries
// each, 109 FAT sectors describe only about 7 MB. An encrypted document with a
// few images passes that line easily, so this is an ordinary case rather than an
// exotic one, and a full coverage pass over the module showed
// writeDIFATSectors never executing in any test.
//
// Round-tripping through our own reader is not enough here. The DIFAT layout is
// a shared convention, so a symmetric misreading — writer and reader agreeing on
// something Office does not do — passes a self round trip and produces files no
// other implementation can open. That is exactly what a reference decryptor
// rules out, and why this case lives beside the cross-validation rather than in
// a plain round-trip test.
//
// The payload is incompressible on purpose. The first attempt at this test
// repeated one string, which deflated to a fraction of its size and never
// reached the threshold it was written to cross: the encrypted stream stayed at
// 34 FAT sectors while the test claimed to be exercising 136.
func TestEncryptedDIFATCrossValidateWithMsoffcrypto(t *testing.T) {
	tool := findMsoffcrypto(t)

	plain := incompressibleZIP(t, 12<<20)
	const password = "Cross-Validate!123"

	// Sanity-check that the fixture actually crosses the threshold; a payload
	// that quietly shrank would make this test pass while covering nothing.
	if got := fatSectorsFor(len(plain)); got <= cfbHeaderDIFAT {
		t.Fatalf("fixture needs only %d FAT sectors, at or below the %d that fit in the header: "+
			"this test would not reach writeDIFATSectors", got, cfbHeaderDIFAT)
	}

	var enc bytes.Buffer
	if err := SaveEncryptedWithOptions(&enc, plain, password, EncryptOptions{Scheme: SchemeAgile}); err != nil {
		t.Fatalf("SaveEncryptedWithOptions: %v", err)
	}
	if got := fatSectorsFor(enc.Len()); got <= cfbHeaderDIFAT {
		t.Fatalf("encrypted container needs only %d FAT sectors; DIFAT sectors were not emitted", got)
	}

	dir := t.TempDir()
	in := filepath.Join(dir, "enc.bin")
	out := filepath.Join(dir, "dec.zip")
	if err := os.WriteFile(in, enc.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(tool, "-p", password, in, out)
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("msoffcrypto-tool could not decrypt a container with chained DIFAT sectors: %v\n%s", err, outBytes)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading msoffcrypto output: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("msoffcrypto-decrypted bytes differ from original (%d vs %d bytes)", len(got), len(plain))
	}

	// And our own reader, so a regression in either direction is attributable.
	r, err := OpenEncrypted(bytes.NewReader(enc.Bytes()), int64(enc.Len()), password)
	if err != nil {
		t.Fatalf("OpenEncrypted on a chained-DIFAT container: %v", err)
	}
	if f := r.GetFile("/payload/blob0.bin"); f == nil {
		t.Fatal("decrypted package missing /payload/blob0.bin")
	}
}

// fatSectorsFor reports how many FAT sectors a CFB of n bytes needs, which is
// what decides whether chained DIFAT sectors are required.
func fatSectorsFor(n int) int {
	return (n / cfbWriteSectorSize) / cfbFATEntries
}

// incompressibleZIP builds a valid OPC package of at least size bytes whose
// parts are random, so deflate cannot shrink it below the DIFAT threshold.
func incompressibleZIP(t *testing.T, size int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	ct, err := zw.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="bin" ContentType="application/octet-stream"/></Types>`)); err != nil {
		t.Fatal(err)
	}

	const chunk = 1 << 20
	blob := make([]byte, chunk)
	for i := 0; buf.Len() < size; i++ {
		if _, err := rand.Read(blob); err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(fmt.Sprintf("payload/blob%d.bin", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(blob); err != nil {
			t.Fatal(err)
		}
		if err := zw.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
