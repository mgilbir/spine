package crypto

import (
	"bytes"
	"encoding/binary"
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

// Byte offsets into a minor-2 EncryptionInfo stream, counted from the start of
// the stream: 4-byte version, 4-byte EncryptionInfo.Flags, 4-byte
// EncryptionHeaderSize, then the EncryptionHeader itself.
const (
	stdOffInfoFlags   = 4
	stdOffHeaderFlags = 12      // header[0:4]
	stdOffAlgID       = 12 + 8  // header[8:12]
	stdOffKeySize     = 12 + 16 // header[16:20]
)

func putU32(b []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(b[off:off+4], v)
}

// TestStandardAlgIDZeroIsAESWhenFAESSet covers [MS-OFFCRYPTO] §2.3.4.5's
// "AlgID = 0 means determined by Flags" encoding. Dispatching on AlgID alone
// routed such a file — a perfectly conformant AES document — into the RC4 path,
// where the verifier could never match, and reported crypto: wrong password:
// the one error that makes a user retry passwords forever on a file that
// decrypts fine.
func TestStandardAlgIDZeroIsAESWhenFAESSet(t *testing.T) {
	plain := []byte("AlgID=0 means look at the flags")
	const password = "flag-dispatch"

	for _, c := range []struct {
		name          string
		keyBits       int
		zeroKeySize   bool
		clearInfoFlag bool
	}{
		{name: "aes128", keyBits: 128},
		{name: "aes256", keyBits: 256},
		// KeySize 0 as well: both fields defer to the other, which resolves to
		// the smallest AES the scheme defines.
		{name: "aes128-keysize-0", keyBits: 128, zeroKeySize: true},
		// fAES only in the EncryptionHeader copy of the flags word.
		{name: "aes256-header-flag-only", keyBits: 256, clearInfoFlag: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			info, pkg, err := EncryptStandard(plain, password, c.keyBits)
			if err != nil {
				t.Fatal(err)
			}
			putU32(info, stdOffAlgID, 0)
			if c.zeroKeySize {
				putU32(info, stdOffKeySize, 0)
			}
			if c.clearInfoFlag {
				putU32(info, stdOffInfoFlags, binary.LittleEndian.Uint32(info[stdOffInfoFlags:stdOffInfoFlags+4])&^uint32(stdFlagAES))
			}

			if isRC4CryptoAPI(info[4:]) {
				t.Fatal("an AlgID=0 stream with fAES set was routed to the RC4 path")
			}
			got, err := Decrypt(info, pkg, password)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatal("decrypted bytes differ")
			}
		})
	}
}

// TestStandardKeySizeZeroTakesSizeFromAlgID covers the mirror-image encoding:
// KeySize 0 means "determined by AlgID".
func TestStandardKeySizeZeroTakesSizeFromAlgID(t *testing.T) {
	plain := []byte("KeySize=0 means look at the AlgID")
	for _, keyBits := range []int{128, 192, 256} {
		t.Run(keyBitsName(keyBits), func(t *testing.T) {
			info, pkg, err := EncryptStandard(plain, "keysize-zero", keyBits)
			if err != nil {
				t.Fatal(err)
			}
			putU32(info, stdOffKeySize, 0)
			got, err := Decrypt(info, pkg, "keysize-zero")
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatal("decrypted bytes differ")
			}
		})
	}
}

// TestStandardAlgIDZeroWithoutFAESStaysRC4 confirms the flag-driven dispatch did
// not steal the legacy path: AlgID 0 with fAES clear is still RC4.
func TestStandardAlgIDZeroWithoutFAESStaysRC4(t *testing.T) {
	plain := []byte("still RC4")
	info, pkg, err := EncryptRC4CryptoAPI(plain, "rc4-pw", 128)
	if err != nil {
		t.Fatal(err)
	}
	putU32(info, stdOffAlgID, 0)
	if !isRC4CryptoAPI(info[4:]) {
		t.Fatal("AlgID=0 with fAES clear was not routed to the RC4 path")
	}
	got, err := Decrypt(info, pkg, "rc4-pw")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("decrypted bytes differ")
	}

	// An RC4 AlgID with fAES set is contradictory; the explicit AlgID wins.
	putU32(info, stdOffHeaderFlags, binary.LittleEndian.Uint32(info[stdOffHeaderFlags:stdOffHeaderFlags+4])|stdFlagAES)
	putU32(info, stdOffAlgID, stdAlgRC4)
	if !isRC4CryptoAPI(info[4:]) {
		t.Fatal("an explicit RC4 AlgID was not routed to the RC4 path")
	}
}

func TestEncryptStandardBadKeyBits(t *testing.T) {
	if _, _, err := EncryptStandard([]byte("x"), "pw", 64); !errors.Is(err, ErrUnsupportedEncryption) {
		t.Fatalf("got %v, want ErrUnsupportedEncryption", err)
	}
}
