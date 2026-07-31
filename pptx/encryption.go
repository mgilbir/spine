package pptx

import (
	"bytes"
	"io"
	"os"

	"github.com/mgilbir/spine/opc"
)

// Writing encrypted presentations. Password encryption is format-generic — the
// same CFB wrapper carries any OOXML package — so these are the pptx spelling
// of the docx and xlsx pairs, not a separate mechanism.
//
// Reading one needs nothing here: an encrypted .pptx is a CFB container rather
// than a zip, which Open and OpenReader detect from the file's leading bytes,
// so
//
//	pptx.Open(path, opc.WithPassword("secret"))
//
// opens it. A wrong password returns crypto.ErrWrongPassword, an unsupported
// scheme crypto.ErrUnsupportedEncryption, and an agile package whose integrity
// HMAC is missing or does not verify crypto.ErrIntegrityCheckFailed — see
// opc.WithAllowMissingDataIntegrity for the deliberate opt-out.

// SaveEncrypted saves the presentation to a file, encrypted with the supplied
// password using Office's agile encryption (AES-256, SHA-512). The password
// must not be empty. The resulting file opens in PowerPoint with the password,
// and here with Open and opc.WithPassword.
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
