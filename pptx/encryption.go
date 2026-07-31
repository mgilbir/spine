package pptx

import (
	"bytes"
	"io"
	"os"

	"github.com/mgilbir/spine/opc"
)

// Encrypted-presentation support. Password encryption is format-generic — the
// same CFB wrapper carries any OOXML package — so these functions are the pptx
// spelling of the docx and xlsx pairs, not a separate mechanism. Before C386
// the pptx package had no encrypted entry point at all while Open's own godoc
// pointed at opc.OpenEncrypted, which returns an *opc.Reader that nothing
// public could turn into a Presentation.

// OpenEncrypted opens a password-encrypted PowerPoint presentation from a file
// path. A modern encrypted .pptx is not a zip but a CFB container holding the
// AES-encrypted package (see opc.OpenEncrypted); this decrypts it with the
// password and returns a Presentation exactly as Open would for a plain file.
//
// A wrong password returns crypto.ErrWrongPassword; an unsupported encryption
// scheme returns crypto.ErrUnsupportedEncryption; an agile package whose
// integrity HMAC is missing or does not verify returns
// crypto.ErrIntegrityCheckFailed (see OpenEncryptedReaderWithOptions for the
// explicit opt-out).
func OpenEncrypted(path, password string) (*Presentation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return OpenEncryptedReader(bytes.NewReader(data), int64(len(data)), password)
}

// OpenEncryptedReader opens a password-encrypted PowerPoint presentation from
// an in-memory reader.
func OpenEncryptedReader(r io.ReaderAt, size int64, password string) (*Presentation, error) {
	return OpenEncryptedReaderWithOptions(r, size, password)
}

// OpenEncryptedReaderWithOptions is OpenEncryptedReader with per-Reader
// options: the decompression bounds guarding the decrypted inner zip, and
// opc.WithAllowMissingDataIntegrity, the deliberate opt-in that
// accepts an agile package carrying no dataIntegrity block. The zero value
// keeps the strict default.
func OpenEncryptedReaderWithOptions(r io.ReaderAt, size int64, password string, opts ...opc.ReaderOption) (*Presentation, error) {
	reader, err := opc.OpenEncrypted(r, size, password, opts...)
	if err != nil {
		return nil, err
	}
	return openFromReader(&opc.ReadCloser{Reader: *reader})
}

// SaveEncrypted saves the presentation to a file, encrypted with the supplied
// password using Office's agile encryption (AES-256, SHA-512). The password
// must not be empty. The resulting file opens in PowerPoint with the password
// and with OpenEncrypted here.
func (p *Presentation) SaveEncrypted(path, password string) error {
	var buf bytes.Buffer
	if err := p.SaveEncryptedTo(&buf, password); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// SaveEncryptedTo saves the presentation to an arbitrary writer, encrypted with
// the supplied password. It first serializes the presentation to plain package
// bytes (running the same validation as SaveTo), then wraps them in an
// encrypted CFB container.
func (p *Presentation) SaveEncryptedTo(dst io.Writer, password string) error {
	data, err := p.SaveBytes()
	if err != nil {
		return err
	}
	return opc.SaveEncrypted(dst, data, password)
}
