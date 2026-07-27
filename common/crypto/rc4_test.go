package crypto

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strconv"
	"testing"
)

// TestRC4CipherKnownAnswers exercises the hand-rolled RC4 stream cipher (KSA +
// PRGA) against published known-answer vectors, independent of any Office
// wrapping: the canonical ASCII vectors and the RFC 6229 keystream. This is what
// gives confidence the cipher core is correct.
func TestRC4CipherKnownAnswers(t *testing.T) {
	// Canonical RC4 test vectors (widely published; e.g. Wikipedia "RC4").
	ascii := []struct {
		key, plain, cipherHex string
	}{
		{"Key", "Plaintext", "bbf316e8d940af0ad3"},
		{"Wiki", "pedia", "1021bf0420"},
		{"Secret", "Attack at dawn", "45a01f645fc35b383552544b9bf5"},
	}
	for _, v := range ascii {
		want, _ := hex.DecodeString(v.cipherHex)
		got := make([]byte, len(v.plain))
		newRC4Cipher([]byte(v.key)).xorKeyStream(got, []byte(v.plain))
		if !bytes.Equal(got, want) {
			t.Errorf("RC4(%q,%q) = %x, want %s", v.key, v.plain, got, v.cipherHex)
		}
		// Symmetric: re-encrypting the ciphertext recovers the plaintext.
		back := make([]byte, len(got))
		newRC4Cipher([]byte(v.key)).xorKeyStream(back, got)
		if string(back) != v.plain {
			t.Errorf("RC4 round-trip for %q = %q, want %q", v.key, back, v.plain)
		}
	}

	// RFC 6229 keystream: key 0x0102030405, first 16 output bytes.
	key, _ := hex.DecodeString("0102030405")
	wantKS, _ := hex.DecodeString("b2396305f03dc027ccc3524a0a1118a8")
	ks := make([]byte, len(wantKS))
	newRC4Cipher(key).xorKeyStream(ks, make([]byte, len(wantKS)))
	if !bytes.Equal(ks, wantKS) {
		t.Errorf("RFC 6229 keystream (40-bit) = %x, want %x", ks, wantKS)
	}
}

// TestRC4CryptoAPIRoundTrip encrypts a package with each supported key size and
// confirms Decrypt recovers it exactly, and that a wrong password is rejected
// with ErrWrongPassword rather than returning garbage.
func TestRC4CryptoAPIRoundTrip(t *testing.T) {
	const password = "RC4-Legacy!secret"
	// A payload larger than one 512-byte block so per-block rekeying is exercised.
	plain := make([]byte, 512*3+37)
	for i := range plain {
		plain[i] = byte(i*7 + 3)
	}

	for _, keyBits := range []int{40, 56, 128} {
		t.Run(keyBitsName(keyBits), func(t *testing.T) {
			info, pkg, err := EncryptRC4CryptoAPI(plain, password, keyBits)
			if err != nil {
				t.Fatalf("EncryptRC4CryptoAPI: %v", err)
			}
			got, err := Decrypt(info, pkg, password)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatalf("decrypted bytes differ (%d vs %d)", len(got), len(plain))
			}

			if _, err := Decrypt(info, pkg, "wrong-password"); !errors.Is(err, ErrWrongPassword) {
				t.Fatalf("wrong password: got %v, want ErrWrongPassword", err)
			}
		})
	}
}

// TestRC4CryptoAPIDefaultKeySize covers the header KeySize==0 path, which means
// the 40-bit default per [MS-OFFCRYPTO].
func TestRC4CryptoAPIDefaultKeySize(t *testing.T) {
	const password = "pw"
	plain := []byte("hello legacy world")
	info, pkg, err := EncryptRC4CryptoAPI(plain, password, 40)
	if err != nil {
		t.Fatal(err)
	}
	// Zero the KeySize field in the header (offset: 8 version + 8 info +
	// header[16:20] = 16..20 within the header that starts after the 8-byte
	// EncryptionInfo prefix and 8-byte version/flags/size fields).
	// version(4)+flags(4)+headerSize(4) = 12; header AlgID..KeySize follow.
	keySizeOff := 4 /*version*/ + 4 /*info flags*/ + 4 /*headerSize*/ + 16 /*header KeySize*/
	binary.LittleEndian.PutUint32(info[keySizeOff:keySizeOff+4], 0)
	got, err := Decrypt(info, pkg, password)
	if err != nil {
		t.Fatalf("Decrypt with KeySize=0: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypted bytes differ with KeySize=0")
	}
}

// TestRC4CryptoAPIDispatch confirms Decrypt routes an RC4 CryptoAPI stream to the
// RC4 path (not the AES standard path) purely from the version+AlgID.
func TestRC4CryptoAPIDispatch(t *testing.T) {
	info, pkg, err := EncryptRC4CryptoAPI([]byte("x"), "p", 128)
	if err != nil {
		t.Fatal(err)
	}
	if !isRC4CryptoAPI(info[4:]) {
		t.Fatal("isRC4CryptoAPI = false for an RC4 CryptoAPI stream")
	}
	if _, err := Decrypt(info, pkg, "p"); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
}

func keyBitsName(b int) string {
	return strconv.Itoa(b) + "-bit"
}
