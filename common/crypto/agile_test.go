package crypto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestAgileRoundTripSizes(t *testing.T) {
	for _, n := range []int{0, 1, 15, 16, 17, 4095, 4096, 4097, 12345} {
		plain := make([]byte, n)
		for i := range plain {
			plain[i] = byte(i)
		}
		info, enc, err := Encrypt(plain, "P@ssw0rd")
		if err != nil {
			t.Fatalf("n=%d Encrypt: %v", n, err)
		}
		got, err := Decrypt(info, enc, "P@ssw0rd")
		if err != nil {
			t.Fatalf("n=%d Decrypt: %v", n, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("n=%d round-trip mismatch", n)
		}
	}
}

func TestAgileWrongPassword(t *testing.T) {
	info, enc, err := Encrypt([]byte("secret document"), "right")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(info, enc, "wrong"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("got %v, want ErrWrongPassword", err)
	}
	// Empty vs non-empty are distinct passwords.
	if _, err := Decrypt(info, enc, ""); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("empty password: got %v, want ErrWrongPassword", err)
	}
}

func TestAgileUnicodePassword(t *testing.T) {
	pw := "café-\U0001F510-密码" // includes a non-BMP rune (surrogate pair)
	info, enc, err := Encrypt([]byte("data"), pw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(info, enc, pw); err != nil {
		t.Fatalf("unicode password round-trip: %v", err)
	}
}

func TestDecryptUnsupportedSchemes(t *testing.T) {
	// Version headers for the legacy/standard schemes that must be rejected.
	cases := []struct {
		name         string
		major, minor uint16
	}{
		{"rc4-cryptoapi", 4, 1},
		{"rc4-standard", 2, 1},
		{"extensible-3.3", 3, 3},
		{"extensible-4.3", 4, 3},
		{"unknown", 9, 9},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hdr := make([]byte, 8)
			binary.LittleEndian.PutUint16(hdr[0:2], c.major)
			binary.LittleEndian.PutUint16(hdr[2:4], c.minor)
			_, err := Decrypt(hdr, nil, "pw")
			if !errors.Is(err, ErrUnsupportedEncryption) {
				t.Fatalf("got %v, want ErrUnsupportedEncryption", err)
			}
		})
	}
}

func TestDecryptMalformedInfo(t *testing.T) {
	if _, err := Decrypt([]byte{0x04, 0x00}, nil, "pw"); !errors.Is(err, ErrMalformedEncryptionInfo) {
		t.Fatalf("short info: got %v, want ErrMalformedEncryptionInfo", err)
	}
	// Valid agile version header but junk XML body.
	bad := append([]byte{0x04, 0x00, 0x04, 0x00, 0x40, 0x00, 0x00, 0x00}, []byte("<not-encryption/>")...)
	if _, err := Decrypt(bad, nil, "pw"); err == nil {
		t.Fatal("expected error for descriptor without a password keyEncryptor")
	}
}

func TestPasswordToUTF16LE(t *testing.T) {
	got := passwordToUTF16LE("AB")
	want := []byte{'A', 0x00, 'B', 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
