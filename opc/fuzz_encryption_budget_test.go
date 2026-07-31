package opc

// A fuzz target that fails on legitimate input is worse than no target: it gets
// muted. These tests run the fuzz budgets from fuzz_encryption_test.go over real
// encrypted documents — every scheme this library writes, plus one large enough
// to need chained DIFAT sectors — and record the headroom, so a future change
// that tightens a budget too far is caught here rather than by a red fuzz run.

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzbound"
)

// budgetHeadroom runs fn under b, fails when the budget is exceeded, and logs
// the fraction of the budget the legitimate document actually used — the number
// that says how much room a future tightening has.
func budgetHeadroom(t *testing.T, b fuzzbound.Budget, n int, fn func()) {
	t.Helper()
	allocated, elapsed := fuzzbound.Measure(fn)
	maxBytes, maxTime := b.Allowance(n)
	t.Logf("%s over %d bytes: allocated %d/%d (%.1f%% of budget), took %v/%v (%.2f%% of budget)",
		b.What, n, allocated, maxBytes, 100*float64(allocated)/float64(maxBytes),
		elapsed, maxTime, 100*float64(elapsed)/float64(maxTime))
	if allocated > maxBytes {
		t.Errorf("%s allocated %d bytes for a legitimate %d-byte document, over the %d-byte budget", b.What, allocated, n, maxBytes)
	}
	if elapsed > maxTime {
		t.Errorf("%s took %v for a legitimate %d-byte document, over the %v budget", b.What, elapsed, n, maxTime)
	}
}

// exerciseBudgets runs all three encrypted-path budgets over one container.
func exerciseBudgets(t *testing.T, data []byte, password string) {
	t.Helper()
	opts := fuzzReaderOptions()

	var cfErr error
	budgetHeadroom(t, cfbBudget, len(data), func() {
		cf, err := readCFB(data)
		cfErr = err
		if err != nil {
			return
		}
		if _, err := cf.stream(cfbStreamEncryptionInfo); err != nil {
			cfErr = err
		}
		if _, err := cf.stream(cfbStreamEncryptedPackage); err != nil {
			cfErr = err
		}
	})
	if cfErr != nil {
		t.Fatalf("readCFB of a legitimate container: %v", cfErr)
	}

	var plain []byte
	var decErr error
	budgetHeadroom(t, decryptBudget, len(data), func() {
		plain, decErr = decryptCFBPackage(data, password, opts)
	})
	if decErr != nil {
		t.Fatalf("decrypting a legitimate container: %v", decErr)
	}
	if len(plain) == 0 {
		t.Fatal("decrypting a legitimate container produced no plaintext")
	}

	var openErr error
	budgetHeadroom(t, openBudget, len(data), func() {
		_, openErr = NewReader(bytes.NewReader(data), int64(len(data)), WithReaderOptions(opts), WithPassword(password))
	})
	if openErr != nil {
		t.Fatalf("opening a legitimate container: %v", openErr)
	}
}

// TestEncryptionFuzzBudgetsAllowLegitimateDocuments checks the budgets against
// every scheme SaveEncryptedWithOptions produces, at sizes from empty to
// multi-segment.
func TestEncryptionFuzzBudgetsAllowLegitimateDocuments(t *testing.T) {
	schemes := []struct {
		name string
		opts EncryptOptions
	}{
		{"agile", EncryptOptions{Scheme: SchemeAgile}},
		{"agile+dataspaces", EncryptOptions{Scheme: SchemeAgile, IncludeDataSpaces: true}},
		{"standard-256", EncryptOptions{Scheme: SchemeStandard}},
		{"standard-128", EncryptOptions{Scheme: SchemeStandard, StandardKeyBits: 128}},
	}
	for _, sc := range schemes {
		for _, bodyLen := range []int{0, 5000, 200000} {
			t.Run(sc.name, func(t *testing.T) {
				var enc bytes.Buffer
				if err := SaveEncryptedWithOptions(&enc, seedPlainPackage(bodyLen), seedPassword, sc.opts); err != nil {
					t.Fatalf("SaveEncryptedWithOptions: %v", err)
				}
				exerciseBudgets(t, enc.Bytes(), seedPassword)
			})
		}
	}
}

// TestEncryptionFuzzBudgetsAllowDIFATDocument is the large-input case the
// budgets have to survive: an encrypted document big enough that its FAT does
// not fit in the 109 header slots, so the reader walks chained DIFAT sectors —
// the same shape as TestEncryptedDIFATCrossValidateWithMsoffcrypto, which needs
// more than 7 MB of CFB payload. Its plaintext is incompressible, so the
// decrypted package is genuinely large rather than a compression artifact.
func TestEncryptionFuzzBudgetsAllowDIFATDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 12 MiB incompressible package")
	}
	plain := incompressibleZIP(t, 12<<20)
	var enc bytes.Buffer
	if err := SaveEncryptedWithOptions(&enc, plain, seedPassword, EncryptOptions{Scheme: SchemeAgile}); err != nil {
		t.Fatalf("SaveEncryptedWithOptions: %v", err)
	}
	if got := fatSectorsFor(enc.Len()); got <= cfbHeaderDIFAT {
		t.Fatalf("container needs only %d FAT sectors, at or below the %d in the header: the DIFAT path is not exercised", got, cfbHeaderDIFAT)
	}

	// The reader's decompression caps are what openBudget's floor is derived
	// from, and this package is deliberately larger than the fuzz caps; raise
	// them for this document so the open stage is measured rather than refused.
	data := enc.Bytes()
	opts := ReaderOptions{MaxDecompressedPartSize: 32 << 20, MaxDecompressedPackageSize: 64 << 20}

	var cfErr error
	budgetHeadroom(t, cfbBudget, len(data), func() {
		cf, err := readCFB(data)
		cfErr = err
		if err == nil {
			_, cfErr = cf.stream(cfbStreamEncryptedPackage)
		}
	})
	if cfErr != nil {
		t.Fatalf("readCFB of a DIFAT container: %v", cfErr)
	}

	var decErr error
	budgetHeadroom(t, decryptBudget, len(data), func() {
		_, decErr = decryptCFBPackage(data, seedPassword, opts)
	})
	if decErr != nil {
		t.Fatalf("decrypting a DIFAT container: %v", decErr)
	}

	// openBudget's floor is stated in terms of the configured package cap, so
	// scale it with the cap this document needs.
	scaled := openBudget
	scaled.Bytes = 64<<20 + 4<<20
	var openErr error
	budgetHeadroom(t, scaled, len(data), func() {
		_, openErr = NewReader(bytes.NewReader(data), int64(len(data)), WithReaderOptions(opts), WithPassword(seedPassword))
	})
	if openErr != nil {
		t.Fatalf("opening a DIFAT container: %v", openErr)
	}
}
