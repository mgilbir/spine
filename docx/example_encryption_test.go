package docx_test

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/opc"
)

// ExampleDocument_SaveEncryptedTo encrypts a document with a password and
// opens it again. Without opc.WithPassword the open fails with
// opc.ErrEncrypted; with the wrong password, crypto.ErrWrongPassword. The
// ciphertext differs on every save (the salt is random), so only the
// decrypted content is compared.
func ExampleDocument_SaveEncryptedTo() {
	doc := docx.Create()
	doc.AddParagraphWithText("Quarterly figures")

	var buf bytes.Buffer
	if err := doc.SaveEncryptedTo(&buf, "s3cret"); err != nil {
		panic(err)
	}
	data := buf.Bytes()
	open := func(opts ...opc.ReaderOption) (*docx.Document, error) {
		return docx.OpenReader(bytes.NewReader(data), int64(len(data)), opts...)
	}

	_, err := open()
	fmt.Println("no password:", errors.Is(err, opc.ErrEncrypted))

	_, err = open(opc.WithPassword("guess"))
	fmt.Println("wrong password:", errors.Is(err, crypto.ErrWrongPassword))

	opened, err := open(opc.WithPassword("s3cret"))
	if err != nil {
		panic(err)
	}
	fmt.Println("text:", opened.Paragraphs()[0].Text())
	// Output:
	// no password: true
	// wrong password: true
	// text: Quarterly figures
}
