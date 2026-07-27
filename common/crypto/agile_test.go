package crypto

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"strings"
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

// TestAgileSpinCountCeilingIsOfficeScale pins the ceiling near Office's own
// spinCount. The cap existed but was 100x Office's 100000, which left a few
// hundred descriptor bytes able to buy seconds of SHA-512 work per attempt —
// all of it spent before anything about the caller's password is known.
func TestAgileSpinCountCeilingIsOfficeScale(t *testing.T) {
	const officeSpinCount = 100000
	if maxSpinCount > 10*officeSpinCount {
		t.Fatalf("maxSpinCount = %d, more than 10x Office's %d: a hostile descriptor buys too much pre-password work", maxSpinCount, officeSpinCount)
	}
	if maxSpinCount < officeSpinCount {
		t.Fatalf("maxSpinCount = %d, below Office's own %d", maxSpinCount, officeSpinCount)
	}

	info, enc, err := Encrypt([]byte("secret"), "pw")
	if err != nil {
		t.Fatal(err)
	}
	// 5,000,000 is well inside the old ceiling and costs seconds of hashing.
	hostile := bytes.Replace(info, []byte(`spinCount="100000"`), []byte(`spinCount="5000000"`), 1)
	if bytes.Equal(hostile, info) {
		t.Fatal("failed to rewrite spinCount in descriptor")
	}
	start := time.Now()
	done := make(chan error, 1)
	go func() { _, e := Decrypt(hostile, enc, "pw"); done <- e }()
	select {
	case e := <-done:
		if !errors.Is(e, ErrMalformedEncryptionInfo) {
			t.Fatalf("spinCount 5000000: got %v, want ErrMalformedEncryptionInfo", e)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("rejecting an over-large spinCount took %v; the derivation ran first", elapsed)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Decrypt did not return for spinCount 5000000")
	}
}

// TestEncryptRejectsUnusablePasswords covers the empty-password guard (which
// previously lived only in opc.SaveEncryptedWithOptions, so the crypto entry
// points would happily "protect" a document with no password at all) and the
// two silent interop divergences alongside it: a password that is not valid
// UTF-8 hashes as U+FFFD replacement characters, and one longer than Office's
// 255-character limit cannot be typed on the other side.
func TestEncryptRejectsUnusablePasswords(t *testing.T) {
	longPassword := strings.Repeat("a", maxPasswordUTF16Units+1)
	cases := []struct {
		name, password string
	}{
		{"empty", ""},
		{"invalid-utf8", "pass\xffword"},
		{"too-long", longPassword},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := Encrypt([]byte("x"), c.password); !errors.Is(err, ErrInvalidPassword) {
				t.Fatalf("Encrypt: got %v, want ErrInvalidPassword", err)
			}
			if _, _, err := EncryptStandard([]byte("x"), c.password, 256); !errors.Is(err, ErrInvalidPassword) {
				t.Fatalf("EncryptStandard: got %v, want ErrInvalidPassword", err)
			}
			if _, _, err := EncryptRC4CryptoAPI([]byte("x"), c.password, 40); !errors.Is(err, ErrInvalidPassword) {
				t.Fatalf("EncryptRC4CryptoAPI: got %v, want ErrInvalidPassword", err)
			}
		})
	}

	// The boundary itself is accepted, and a non-BMP password still counts in
	// UTF-16 code units (a surrogate pair is two).
	atLimit := strings.Repeat("a", maxPasswordUTF16Units)
	if _, _, err := Encrypt([]byte("x"), atLimit); err != nil {
		t.Fatalf("password of exactly %d characters: %v", maxPasswordUTF16Units, err)
	}
	surrogateHeavy := strings.Repeat("\U0001F510", maxPasswordUTF16Units/2+1)
	if _, _, err := Encrypt([]byte("x"), surrogateHeavy); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("surrogate-pair password over the limit: got %v, want ErrInvalidPassword", err)
	}
}

func TestPasswordToUTF16LE(t *testing.T) {
	got := passwordToUTF16LE("AB")
	want := []byte{'A', 0x00, 'B', 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Non-BMP runes become surrogate pairs, as in the UTF-16 strings Office
	// hashes: U+1F510 is D83D DD10.
	got = passwordToUTF16LE("\U0001F510")
	want = []byte{0x3D, 0xD8, 0x10, 0xDD}
	if !bytes.Equal(got, want) {
		t.Fatalf("surrogate pair: got % x, want % x", got, want)
	}

	// Nothing is truncated: the read path must hash whatever it is given, so a
	// long password cannot collide with its own 255-character prefix.
	long := strings.Repeat("z", maxPasswordUTF16Units+10)
	if n := len(passwordToUTF16LE(long)); n != 2*len(long) {
		t.Fatalf("encoded %d bytes for a %d-character password, want %d", n, len(long), 2*len(long))
	}
}
