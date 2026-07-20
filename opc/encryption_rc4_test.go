package opc

import (
	"bytes"
	"errors"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
)

// TestOpenEncryptedRC4CryptoAPI wraps a plain package as a legacy RC4 CryptoAPI
// CFB container and confirms OpenEncrypted auto-detects the scheme, decrypts it,
// and exposes the inner parts. The RC4 crypto itself is validated against
// msoffcrypto in common/crypto; this test covers the opc container wiring.
func TestOpenEncryptedRC4CryptoAPI(t *testing.T) {
	for _, bodyLen := range []int{0, 100, 512, 4097, 20000} {
		plain := buildPlainPackage(t, bodyLen)

		info, pkg, err := crypto.EncryptRC4CryptoAPI(plain, "rc4-pw", 40)
		if err != nil {
			t.Fatalf("bodyLen=%d EncryptRC4CryptoAPI: %v", bodyLen, err)
		}
		var enc bytes.Buffer
		if err := writeCFBWithStorages(&enc, []cfbStream{
			{name: cfbStreamEncryptionInfo, data: info},
			{name: cfbStreamEncryptedPackage, data: pkg},
		}, nil); err != nil {
			t.Fatalf("bodyLen=%d writeCFB: %v", bodyLen, err)
		}

		r, err := OpenEncrypted(bytes.NewReader(enc.Bytes()), int64(enc.Len()), "rc4-pw")
		if err != nil {
			t.Fatalf("bodyLen=%d OpenEncrypted: %v", bodyLen, err)
		}
		if f := r.GetFile("/ppt/body.bin"); f == nil {
			t.Fatalf("bodyLen=%d decrypted package missing body part", bodyLen)
		}

		if _, err := decryptCFBPackage(enc.Bytes(), "wrong"); !errors.Is(err, crypto.ErrWrongPassword) {
			t.Fatalf("bodyLen=%d wrong password: got %v, want ErrWrongPassword", bodyLen, err)
		}
	}
}
