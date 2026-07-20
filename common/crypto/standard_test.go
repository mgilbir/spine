package crypto

import (
	"bytes"
	"errors"
	"testing"
)

// TestStandardKeyDerivationKnownAnswer cross-checks standardDeriveKey against the
// published reference vector from msoffcrypto-tool
// (ECMA376Standard.makekey_from_password doctest): a fixed password/salt must
// derive a fixed AES-128 intermediate key. This pins the SHA-1 spin loop and the
// 0x36/0x5c key-material construction to an external reference.
func TestStandardKeyDerivationKnownAnswer(t *testing.T) {
	salt := []byte{0xe8, 0x82, 0x66, 0x49, 0x0c, 0x5b, 0xd1, 0xee, 0xbd, 0x2b, 0x43, 0x94, 0xe3, 0xf8, 0x30, 0xef}
	want := []byte{0x40, 0xb1, 0x3a, 0x71, 0xf9, 0x0b, 0x96, 0x6e, 0x37, 0x54, 0x08, 0xf2, 0xd1, 0x81, 0xa1, 0xaa}

	got := standardDeriveKey(passwordToUTF16LE("Password1234_"), salt, 16)
	if !bytes.Equal(got, want) {
		t.Fatalf("standard key derivation mismatch:\n got %x\nwant %x", got, want)
	}
}

func TestStandardRoundTrip(t *testing.T) {
	for _, keyBits := range []int{128, 192, 256} {
		for _, n := range []int{0, 1, 15, 16, 17, 4096, 100000} {
			plain := make([]byte, n)
			for i := range plain {
				plain[i] = byte(i * 7)
			}
			info, enc, err := EncryptStandard(plain, "s3cret!", keyBits)
			if err != nil {
				t.Fatalf("keyBits=%d n=%d EncryptStandard: %v", keyBits, n, err)
			}
			got, err := Decrypt(info, enc, "s3cret!")
			if err != nil {
				t.Fatalf("keyBits=%d n=%d Decrypt: %v", keyBits, n, err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatalf("keyBits=%d n=%d round-trip mismatch", keyBits, n)
			}
		}
	}
}

func TestStandardWrongPassword(t *testing.T) {
	info, enc, err := EncryptStandard([]byte("secret document"), "right", 256)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(info, enc, "wrong"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("got %v, want ErrWrongPassword", err)
	}
	if _, err := Decrypt(info, enc, ""); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("empty password: got %v, want ErrWrongPassword", err)
	}
}

func TestStandardUnicodePassword(t *testing.T) {
	pw := "café-\U0001F510-密码"
	info, enc, err := EncryptStandard([]byte("data spanning"), pw, 256)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(info, enc, pw); err != nil {
		t.Fatalf("unicode password round-trip: %v", err)
	}
}

func TestEncryptStandardBadKeyBits(t *testing.T) {
	if _, _, err := EncryptStandard([]byte("x"), "pw", 64); !errors.Is(err, ErrUnsupportedEncryption) {
		t.Fatalf("got %v, want ErrUnsupportedEncryption", err)
	}
}
