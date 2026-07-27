package crypto

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// requireMsoffcryptoEnv makes a missing reference implementation a failure
// rather than a skip. CI sets it (see .github/workflows/ci.yml, which installs
// msoffcrypto-tool) so that the cross-validation the documentation advertises
// actually runs somewhere instead of skipping everywhere.
const requireMsoffcryptoEnv = "SPINE_REQUIRE_MSOFFCRYPTO"

// pythonWithMsoffcrypto returns a Python interpreter that can import msoffcrypto.
// It honors $MSOFFCRYPTO_PY (a path to such an interpreter) and otherwise probes
// python3/python on PATH. When none is available it skips — unless
// $SPINE_REQUIRE_MSOFFCRYPTO is set, in which case the missing reference is a
// hard failure.
func pythonWithMsoffcrypto(t *testing.T) string {
	t.Helper()
	candidates := []string{os.Getenv("MSOFFCRYPTO_PY"), "python3", "python"}
	for _, py := range candidates {
		if py == "" {
			continue
		}
		if _, err := exec.LookPath(py); err != nil && !strings.Contains(py, "/") {
			continue
		}
		if err := exec.Command(py, "-c", "import msoffcrypto.method.rc4_cryptoapi").Run(); err == nil {
			return py
		}
	}
	if v := os.Getenv(requireMsoffcryptoEnv); v != "" && v != "0" {
		t.Fatalf("%s is set but no Python with msoffcrypto is available; install msoffcrypto-tool or point $MSOFFCRYPTO_PY at an interpreter that has it", requireMsoffcryptoEnv)
	}
	t.Skip("no Python with msoffcrypto available; set MSOFFCRYPTO_PY to cross-validate RC4 CryptoAPI")
	return ""
}

// rc4CrossCheckScript verifies the password and decrypts an RC4 CryptoAPI
// EncryptedPackage body using msoffcrypto's independent DocumentRC4CryptoAPI, then
// prints the recovered plaintext as hex. It reads, in order:
//
//	argv: password  keyBits  saltHex  encVerifierHex  encVerHashHex  ciphertextHex
const rc4CrossCheckScript = `
import sys, io, binascii
from msoffcrypto.method.rc4_cryptoapi import DocumentRC4CryptoAPI
pw, keyBits = sys.argv[1], int(sys.argv[2])
salt = binascii.unhexlify(sys.argv[3])
encVerifier = binascii.unhexlify(sys.argv[4])
encVerHash = binascii.unhexlify(sys.argv[5])
ciphertext = binascii.unhexlify(sys.argv[6])
if not DocumentRC4CryptoAPI.verifypw(pw, salt, keyBits, encVerifier, encVerHash):
    sys.stderr.write("verifypw failed\n"); sys.exit(2)
dec = DocumentRC4CryptoAPI.decrypt(pw, salt, keyBits, io.BytesIO(ciphertext)).read()
sys.stdout.write(binascii.hexlify(dec).decode())
`

// TestRC4CryptoAPICrossValidateWithMsoffcrypto confirms that RC4 CryptoAPI streams
// this package produces are decrypted, byte-for-byte, by the independent
// msoffcrypto reference implementation — validating the key derivation, the RC4
// cipher, and the per-512-byte block rekeying against a second implementation.
//
// NOTE: msoffcrypto-tool's *OOXML* path decrypts only the AES "standard" and
// "agile" schemes, so the CLI cannot open an RC4-CryptoAPI-wrapped OOXML package;
// this test therefore drives msoffcrypto's RC4 CryptoAPI method directly, which is
// the same primitive it uses for the legacy binary formats.
func TestRC4CryptoAPICrossValidateWithMsoffcrypto(t *testing.T) {
	py := pythonWithMsoffcrypto(t)

	const password = "Cross-Validate!RC4"
	plain := make([]byte, 512*2+123)
	for i := range plain {
		plain[i] = byte(i*11 + 5)
	}

	for _, keyBits := range []int{40, 128} {
		t.Run(keyBitsName(keyBits), func(t *testing.T) {
			info, pkg, err := EncryptRC4CryptoAPI(plain, password, keyBits)
			if err != nil {
				t.Fatalf("EncryptRC4CryptoAPI: %v", err)
			}
			parsed, err := parseRC4CryptoAPIInfo(info[4:])
			if err != nil {
				t.Fatalf("parseRC4CryptoAPIInfo: %v", err)
			}
			ciphertext := pkg[8:] // strip the 8-byte plaintext-size prefix

			cmd := exec.Command(py, "-c", rc4CrossCheckScript,
				password,
				strconv.Itoa(parsed.keyBits),
				hex.EncodeToString(parsed.salt),
				hex.EncodeToString(parsed.encVerifier),
				hex.EncodeToString(parsed.encVerHash),
				hex.EncodeToString(ciphertext),
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("msoffcrypto cross-check failed: %v\n%s", err, out)
			}
			got, err := hex.DecodeString(strings.TrimSpace(string(out)))
			if err != nil {
				t.Fatalf("decoding cross-check output: %v", err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatalf("msoffcrypto-decrypted bytes differ from original (%d vs %d)", len(got), len(plain))
			}
		})
	}
}
