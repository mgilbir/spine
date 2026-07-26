package crypto

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"testing"
	"time"
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

// TestAgileSpinCountCapped confirms an attacker-supplied spinCount above the
// ceiling is rejected before any key derivation runs, so a hostile
// EncryptionInfo cannot pin a core doing SHA-512 work. It must return promptly
// (the derivation loop would otherwise run billions of iterations).
func TestAgileSpinCountCapped(t *testing.T) {
	// Direct deriveKey guard: a value past the ceiling returns immediately.
	p := agileParams{newHash: sha512.New, hashSize: 64, keyBytes: 32, blockSize: 16}
	if _, err := deriveKey(p, make([]byte, 16), passwordToUTF16LE("pw"), maxSpinCount+1, blockKeyVerifierHashInput); !errors.Is(err, ErrMalformedEncryptionInfo) {
		t.Fatalf("deriveKey over ceiling: got %v, want ErrMalformedEncryptionInfo", err)
	}
	if _, err := deriveKey(p, make([]byte, 16), passwordToUTF16LE("pw"), -1, blockKeyVerifierHashInput); !errors.Is(err, ErrMalformedEncryptionInfo) {
		t.Fatalf("deriveKey negative: got %v, want ErrMalformedEncryptionInfo", err)
	}
	// A legitimate value still derives a key.
	if _, err := deriveKey(p, make([]byte, 16), passwordToUTF16LE("pw"), 100000, blockKeyVerifierHashInput); err != nil {
		t.Fatalf("deriveKey legitimate spinCount: %v", err)
	}

	// End-to-end: a real descriptor whose spinCount attribute is rewritten to a
	// hostile value is rejected by agileDecrypt before the derivation loops.
	info, enc, err := Encrypt([]byte("secret"), "pw")
	if err != nil {
		t.Fatal(err)
	}
	hostile := bytes.Replace(info, []byte(`spinCount="100000"`), []byte(`spinCount="2000000000"`), 1)
	if bytes.Equal(hostile, info) {
		t.Fatal("failed to rewrite spinCount in descriptor")
	}
	done := make(chan error, 1)
	go func() { _, e := Decrypt(hostile, enc, "pw"); done <- e }()
	select {
	case e := <-done:
		if !errors.Is(e, ErrMalformedEncryptionInfo) {
			t.Fatalf("hostile spinCount: got %v, want ErrMalformedEncryptionInfo", e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Decrypt did not return promptly for a hostile spinCount")
	}
}

func TestPasswordToUTF16LE(t *testing.T) {
	got := passwordToUTF16LE("AB")
	want := []byte{'A', 0x00, 'B', 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
