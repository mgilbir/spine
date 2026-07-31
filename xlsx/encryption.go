package xlsx

import (
	"bytes"
	"io"
	"os"

	"github.com/mgilbir/spine/opc"
)

// Writing encrypted workbooks. Password encryption is format-generic — the
// same CFB wrapper carries any OOXML package — so these are the xlsx spelling
// of the docx and pptx pairs, not a separate mechanism.
//
// Reading one needs nothing here: an encrypted .xlsx is a CFB container rather
// than a zip, which Open and OpenReader detect from the file's leading bytes,
// so
//
//	xlsx.Open(path, opc.WithPassword("secret"))
//
// opens it. A wrong password returns crypto.ErrWrongPassword, an unsupported
// scheme crypto.ErrUnsupportedEncryption, and an agile package whose integrity
// HMAC is missing or does not verify crypto.ErrIntegrityCheckFailed — see
// opc.WithAllowMissingDataIntegrity for the deliberate opt-out.

// SaveEncrypted saves the workbook to a file, encrypted with the supplied
// password using Office's agile encryption (AES-256, SHA-512). The password
// must not be empty. The resulting file opens in Excel with the password, and
// here with Open and opc.WithPassword.
func (w *Workbook) SaveEncrypted(path, password string) error {
	var buf bytes.Buffer
	if err := w.SaveEncryptedTo(&buf, password); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// SaveEncryptedTo saves the workbook to an arbitrary writer, encrypted with the
// supplied password. It first serializes the workbook to plain package bytes
// (running the same validation as SaveTo), then wraps them in an encrypted CFB
// container.
func (w *Workbook) SaveEncryptedTo(dst io.Writer, password string) error {
	data, err := w.SaveBytes()
	if err != nil {
		return err
	}
	return opc.SaveEncrypted(dst, data, password)
}
