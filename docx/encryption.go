package docx

import (
	"bytes"
	"io"
	"os"

	"github.com/mgilbir/spine/opc"
)

// OpenEncrypted opens a password-encrypted Word document from a file path. A
// modern encrypted .docx is not a zip but a CFB container holding the
// AES-encrypted package (see opc.OpenEncrypted); this decrypts it with the
// password and returns a Document exactly as Open would for a plain file.
//
// A wrong password returns crypto.ErrWrongPassword; an unsupported encryption
// scheme returns crypto.ErrUnsupportedEncryption.
func OpenEncrypted(path, password string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return OpenEncryptedReader(bytes.NewReader(data), int64(len(data)), password)
}

// OpenEncryptedReader opens a password-encrypted Word document from an
// in-memory reader.
func OpenEncryptedReader(r io.ReaderAt, size int64, password string) (*Document, error) {
	return OpenEncryptedReaderWithOptions(r, size, password, opc.ReaderOptions{})
}

// OpenEncryptedReaderWithOptions is OpenEncryptedReader with per-Reader
// options: the decompression bounds guarding the decrypted inner zip, and
// opc.ReaderOptions.AllowMissingDataIntegrity, the deliberate opt-in that
// accepts an agile package carrying no dataIntegrity block. The zero value
// keeps the strict default.
func OpenEncryptedReaderWithOptions(r io.ReaderAt, size int64, password string, opts opc.ReaderOptions) (*Document, error) {
	reader, err := opc.OpenEncryptedWithOptions(r, size, password, opts)
	if err != nil {
		return nil, err
	}
	return openFromReader(&opc.ReadCloser{Reader: *reader})
}

// SaveEncrypted saves the document to a file, encrypted with the supplied
// password using Office's agile encryption (AES-256, SHA-512). The password
// must not be empty. The resulting file opens in Word with the password and
// with OpenEncrypted here.
func (d *Document) SaveEncrypted(path, password string) error {
	var buf bytes.Buffer
	if err := d.SaveEncryptedTo(&buf, password); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// SaveEncryptedTo saves the document to an arbitrary writer, encrypted with the
// supplied password. It first serializes the document to plain package bytes
// (running the same validation as SaveTo), then wraps them in an encrypted CFB
// container.
func (d *Document) SaveEncryptedTo(w io.Writer, password string) error {
	data, err := d.SaveBytes()
	if err != nil {
		return err
	}
	return opc.SaveEncrypted(w, data, password)
}
