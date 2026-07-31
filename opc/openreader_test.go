package opc

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/internal/fuzzseed"
)

// OpenReader is the only entry point that touches the filesystem, and nothing
// exercised it.
//
// Every other test reaches the reader through NewReader over bytes, so the file
// half — opening, stat, and closing the handle again on each failure — had no
// coverage at all, in a function whose whole job is that half. The forms the
// documentation shows, including opening an encrypted package by path, ran
// nowhere.

// writePackage writes a package to a temporary file and returns its path.
func writePackage(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.pkg")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// openFDs counts this process's open descriptors, so a failed open that keeps
// its file can be told from one that does not. A leak here is invisible to
// every other kind of assertion and fatal in a long-running process: the
// handles accumulate one per failed open until the limit is reached.
func openFDs(t *testing.T) int {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("descriptor counting is /proc-specific")
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("cannot read /proc/self/fd: %v", err)
	}
	return len(entries)
}

func TestOpenReaderReadsAPackageFromDisk(t *testing.T) {
	path := writePackage(t, buildPlainPackage(t, 2048))

	rc, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	f := rc.GetFile("/ppt/body.bin")
	if f == nil {
		t.Fatal("package opened from disk is missing /ppt/body.bin")
	}
	body, err := f.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(body) != 2048 {
		t.Errorf("body is %d bytes, want 2048", len(body))
	}
	if err := rc.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// The bounds reach the reader through the path form too. Passing an option to
// OpenReader and having it silently ignored would leave every caller that opens
// by path — which is the ordinary way to open a file — running with defaults.
func TestOpenReaderAppliesOptions(t *testing.T) {
	path := writePackage(t, buildPlainPackage(t, 4096))

	// Above [Content_Types].xml, which is read during the open, and below the
	// body part, which is read after it: the bound has to reach both stages.
	rc, err := OpenReader(path, WithMaxDecompressedPartSize(2000))
	if err != nil {
		t.Fatalf("OpenReader with a small per-part bound: %v", err)
	}
	defer func() { _ = rc.Close() }()

	f := rc.GetFile("/ppt/body.bin")
	if f == nil {
		t.Fatal("package is missing /ppt/body.bin")
	}
	if _, err := f.ReadAll(); err == nil {
		t.Error("a 4096-byte part read under a 2000-byte per-part bound returned no error")
	} else if !strings.Contains(err.Error(), "WithMaxDecompressedPartSize") {
		t.Errorf("the error should name the option that raises the bound: %v", err)
	}

	// The open itself reads [Content_Types].xml, so a bound below that size has
	// to stop the open rather than be noticed later.
	if _, err := OpenReader(path, WithMaxDecompressedPartSize(16)); err == nil {
		t.Error("a bound smaller than [Content_Types].xml let the open succeed")
	}
}

// An encrypted package opens by path with a password, which is the form the
// documentation shows and nothing ran.
func TestOpenReaderOpensAnEncryptedPackage(t *testing.T) {
	const password = "hunter2"
	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, buildPlainPackage(t, 1024), password); err != nil {
		t.Fatal(err)
	}
	path := writePackage(t, enc.Bytes())

	rc, err := OpenReader(path, WithPassword(password))
	if err != nil {
		t.Fatalf("OpenReader with a password: %v", err)
	}
	if rc.GetFile("/ppt/body.bin") == nil {
		t.Error("decrypted package is missing /ppt/body.bin")
	}
	if err := rc.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	if _, err := OpenReader(path); !errors.Is(err, ErrEncrypted) {
		t.Errorf("OpenReader with no password: got %v, want ErrEncrypted", err)
	}
	if _, err := OpenReader(path, WithPassword("wrong")); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Errorf("OpenReader with a wrong password: got %v, want crypto.ErrWrongPassword", err)
	}
}

func TestOpenReaderMissingFile(t *testing.T) {
	_, err := OpenReader(filepath.Join(t.TempDir(), "absent.pkg"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("OpenReader on a missing file = %v, want fs.ErrNotExist", err)
	}
}

// Every failing open has to give the file back.
//
// OpenReader closes the handle on each error path, and nothing checked it: a
// missing Close reads as a perfectly ordinary error return, and only shows up
// as a process that has run out of descriptors much later, somewhere else.
func TestOpenReaderClosesTheFileOnEveryFailure(t *testing.T) {
	dir := t.TempDir()

	notAPackage := filepath.Join(dir, "not-a-package.pkg")
	if err := os.WriteFile(notAPackage, []byte("this is not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A zip with no [Content_Types].xml: it opens as an archive and then fails
	// deeper in, past the point where the handle is already held.
	noContentTypes := writePackage(t, fuzzseed.BuildZip([][2]string{
		{"ppt/presentation.xml", "<presentation/>"},
	}))
	encrypted := func() string {
		var enc bytes.Buffer
		if err := SaveEncrypted(&enc, buildPlainPackage(t, 64), "pw"); err != nil {
			t.Fatal(err)
		}
		return writePackage(t, enc.Bytes())
	}()

	cases := map[string]func() (*ReadCloser, error){
		"not a zip":              func() (*ReadCloser, error) { return OpenReader(notAPackage) },
		"no content types":       func() (*ReadCloser, error) { return OpenReader(noContentTypes) },
		"encrypted, no password": func() (*ReadCloser, error) { return OpenReader(encrypted) },
		"wrong password":         func() (*ReadCloser, error) { return OpenReader(encrypted, WithPassword("nope")) },
		"bound too low": func() (*ReadCloser, error) {
			return OpenReader(noContentTypes, WithMaxPackageEntries(1))
		},
	}

	for name, open := range cases {
		t.Run(name, func(t *testing.T) {
			before := openFDs(t)
			// Repeated so a single leaked handle is unambiguous rather than
			// lost in the noise of whatever else the runtime opened.
			const rounds = 16
			for i := 0; i < rounds; i++ {
				rc, err := open()
				if err == nil {
					_ = rc.Close()
					t.Fatalf("expected this open to fail")
				}
				if rc != nil {
					t.Fatalf("a failed open returned a non-nil ReadCloser alongside %v", err)
				}
			}
			if after := openFDs(t); after > before+2 {
				t.Errorf("%d failed opens leaked descriptors: %d open before, %d after",
					rounds, before, after)
			}
		})
	}
}
