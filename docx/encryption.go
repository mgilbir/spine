package docx

import (
	"bytes"
	"io"
	"os"

	"github.com/mgilbir/spine/opc"
)

// Writing encrypted documents. Reading one needs nothing here: an encrypted
// .docx is a CFB container rather than a zip, which Open and OpenReader detect
// from the file's leading bytes, so
//
//	docx.Open(path, opc.WithPassword("secret"))
//
// opens it. A wrong password returns crypto.ErrWrongPassword, an unsupported
// scheme crypto.ErrUnsupportedEncryption, and an agile package whose integrity
// HMAC is missing or does not verify crypto.ErrIntegrityCheckFailed — see
// opc.WithAllowMissingDataIntegrity for the deliberate opt-out.

// SaveEncrypted saves the document to a file, encrypted with the supplied
// password using Office's agile encryption (AES-256, SHA-512). The password
// must not be empty. The resulting file opens in Word with the password, and
// here with Open and opc.WithPassword.
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
