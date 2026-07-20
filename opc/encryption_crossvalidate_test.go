package opc

import (
	"bytes"
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
