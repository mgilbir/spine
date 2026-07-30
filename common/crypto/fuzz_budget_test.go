package crypto

// Evidence that cryptoBudget does not fire on legitimate input: every scheme,
// at a size well past anything a fuzzer will generate, plus the slowest input
// the descriptor grammar permits at all (the spinCount ceiling). A budget that
// fires on a real document is a budget that gets muted, so it is checked here
// rather than discovered during a fuzz run.

import (
	"bytes"
	"errors"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzbound"
)

func budgetHeadroom(t *testing.T, n int, what string, fn func()) {
	t.Helper()
	allocated, elapsed := fuzzbound.Measure(fn)
	maxBytes, maxTime := cryptoBudget.Allowance(n)
	t.Logf("%s over %d bytes: allocated %d/%d (%.1f%% of budget), took %v/%v (%.2f%% of budget)",
		what, n, allocated, maxBytes, 100*float64(allocated)/float64(maxBytes),
		elapsed, maxTime, 100*float64(elapsed)/float64(maxTime))
	if allocated > maxBytes {
		t.Errorf("%s allocated %d bytes for a legitimate %d-byte input, over the %d-byte budget", what, allocated, n, maxBytes)
	}
	if elapsed > maxTime {
		t.Errorf("%s took %v for a legitimate %d-byte input, over the %v budget", what, elapsed, n, maxTime)
	}
}

// TestCryptoFuzzBudgetAllowsRealContainers runs the fuzz budget over containers
// produced by every scheme, from empty to 4 MiB.
func TestCryptoFuzzBudgetAllowsRealContainers(t *testing.T) {
	for _, size := range []int{0, 6000, 4 << 20} {
		plain := bytes.Repeat([]byte("payload!"), size/8)
		agileInfo, agilePkg, err := Encrypt(plain, fuzzPassword)
		if err != nil {
			t.Fatal(err)
		}
		stdInfo, stdPkg, err := EncryptStandard(plain, fuzzPassword, 256)
		if err != nil {
			t.Fatal(err)
		}
		rc4Info, rc4Pkg, err := EncryptRC4CryptoAPI(plain, fuzzPassword, 128)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			name      string
			info, pkg []byte
		}{
			{"agile", agileInfo, agilePkg},
			{"standard", stdInfo, stdPkg},
			{"rc4", rc4Info, rc4Pkg},
		} {
			n := len(c.info) + len(c.pkg)
			var res DecryptResult
			var derr error
			budgetHeadroom(t, n, c.name, func() {
				res, derr = DecryptWithOptions(c.info, c.pkg, fuzzPassword, DecryptOptions{})
			})
			if derr != nil {
				t.Fatalf("%s: decrypting a legitimate container: %v", c.name, derr)
			}
			if !bytes.Equal(res.Package, plain) {
				t.Fatalf("%s: round trip mismatch", c.name)
			}
		}
	}
}

// TestCryptoFuzzBudgetAllowsMaxSpinCount is the wall-clock case the time bound
// is sized against. spinCount is an attacker-controlled attribute capped at
// maxSpinCount, and the decrypt path runs a derivation of that many hash
// iterations up to three times, so a descriptor sitting exactly on the ceiling
// is the slowest input the grammar permits. Rewriting the attribute changes the
// derived key, so the decrypt ends in ErrWrongPassword — the cost, which is what
// the budget bounds, is paid in full before that is known.
func TestCryptoFuzzBudgetAllowsMaxSpinCount(t *testing.T) {
	info, pkg, err := Encrypt(fuzzPlaintext(), fuzzPassword)
	if err != nil {
		t.Fatal(err)
	}
	ceiling := bytes.Replace(info, []byte(`spinCount="100000"`), []byte(`spinCount="1000000"`), 1)
	if bytes.Equal(ceiling, info) {
		t.Fatal("descriptor has no spinCount attribute to raise")
	}
	if maxSpinCount != 1000000 {
		t.Fatalf("maxSpinCount is %d; this test pins the ceiling at 1000000", maxSpinCount)
	}

	var derr error
	budgetHeadroom(t, len(ceiling)+len(pkg), "spinCount=1000000", func() {
		_, derr = DecryptWithOptions(ceiling, pkg, fuzzPassword, DecryptOptions{})
	})
	if !errors.Is(derr, ErrWrongPassword) {
		t.Fatalf("decrypt at the spinCount ceiling: got %v, want ErrWrongPassword", derr)
	}

	// One over the ceiling is refused before any derivation runs, which is why
	// the bound above is the worst case rather than merely the measured case.
	over := bytes.Replace(info, []byte(`spinCount="100000"`), []byte(`spinCount="1000001"`), 1)
	if _, err := DecryptWithOptions(over, pkg, fuzzPassword, DecryptOptions{}); !errors.Is(err, ErrMalformedEncryptionInfo) {
		t.Fatalf("spinCount over the ceiling: got %v, want ErrMalformedEncryptionInfo", err)
	}
}
