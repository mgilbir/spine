// Package fuzzbound bounds the resources one parse of attacker-controlled bytes
// may consume, so that a fuzz target fails on a resource blow-up instead of
// merely surviving it.
//
// It exists because "did it panic?" is not a strong enough oracle for the
// parsers that read encrypted documents. The worst bug this module has shipped
// (C360) was a CFB header field used unvalidated as an allocation size: a
// 512-byte file drove a 16 GiB allocation. That does not panic. On a large
// machine it succeeds; on a small one the kernel OOM-kills the process, which a
// human reads as infrastructure flake rather than as a finding — and Go's
// fuzzing engine cannot distinguish it from a machine falling over either.
// Turning it into an assertion failure with a message is the whole point.
//
// Two resources are bounded.
//
// Allocation volume is read from the runtime's cumulative allocation counter
// (/gc/heap/allocs:bytes), not from resident memory. That is deliberate: a
// make([]uint32, 0, 1<<32) is served by fresh mmap'd pages that are never
// touched, so RSS can stay flat while 16 GiB is charged to the heap. The
// counter records large allocations (>32 KiB, the only ones that matter here)
// exactly and at the moment they happen. Small allocations are accumulated at
// span granularity and so are approximate by at most a few tens of KiB, which
// is immaterial against the budgets below.
//
// Wall-clock time is bounded so that a quadratic or unbounded loop fails as a
// finding rather than as a package timeout nobody attributes to a parser.
//
// A budget is (fixed floor) + (rate x input size). The floor absorbs the
// per-call constants — decoder buffers, cipher key schedules, maps — that do
// not scale with the input; the rate expresses the structural claim that a
// container cannot cost materially more than a small multiple of its own size.
package fuzzbound

import (
	"fmt"
	"runtime/metrics"
	"testing"
	"time"
)

// allocsMetric is the runtime's cumulative "bytes allocated to the heap by the
// application" counter. Unlike runtime.ReadMemStats it needs no stop-the-world
// pass (measured here at ~0.4 us per read against ~11 us), which matters when it
// brackets every fuzz execution.
const allocsMetric = "/gc/heap/allocs:bytes"

// Measure runs fn and reports how many bytes it allocated and how long it took.
//
// The allocation figure is process-wide for the duration of the call: another
// goroutine allocating concurrently inflates it. Fuzz workers run one execution
// at a time, so in practice fn is the only allocator; Budget.Check re-measures
// before failing to keep a stray concurrent allocation from being reported as a
// finding.
func Measure(fn func()) (allocated uint64, elapsed time.Duration) {
	sample := []metrics.Sample{{Name: allocsMetric}}
	metrics.Read(sample)
	before := sample[0].Value.Uint64()

	start := time.Now()
	fn()
	elapsed = time.Since(start)

	metrics.Read(sample)
	after := sample[0].Value.Uint64()
	if after < before { // counter is cumulative; defensive only
		return 0, elapsed
	}
	return after - before, elapsed
}

// Budget is what one parse of an n-byte input is allowed to spend.
type Budget struct {
	// What names the operation in failure messages, e.g. "readCFB".
	What string

	// Bytes is the allocation allowed regardless of input size, and
	// BytesPerInputByte the allocation allowed per byte of input.
	Bytes             uint64
	BytesPerInputByte uint64

	// Time is the wall clock allowed regardless of input size, and TimePerMiB
	// the wall clock allowed per MiB of input.
	Time       time.Duration
	TimePerMiB time.Duration
}

// Allowance returns the budget for an input of n bytes.
func (b Budget) Allowance(n int) (bytes uint64, dur time.Duration) {
	if n < 0 {
		n = 0
	}
	return b.Bytes + b.BytesPerInputByte*uint64(n),
		b.Time + time.Duration(float64(b.TimePerMiB)*float64(n)/float64(1<<20))
}

// exceeded reports which limits a measurement broke, or "" when it is within
// budget.
func (b Budget) exceeded(n int, allocated uint64, elapsed time.Duration) string {
	maxBytes, maxTime := b.Allowance(n)
	switch {
	case allocated > maxBytes && elapsed > maxTime:
		return fmt.Sprintf("allocated %d bytes (budget %d) and took %v (budget %v)", allocated, maxBytes, elapsed, maxTime)
	case allocated > maxBytes:
		return fmt.Sprintf("allocated %d bytes (budget %d)", allocated, maxBytes)
	case elapsed > maxTime:
		return fmt.Sprintf("took %v (budget %v)", elapsed, maxTime)
	}
	return ""
}

// grossFactor is how far past its budget a measurement has to be before it is
// reported without a confirming re-run. A near-miss can be noise — a concurrent
// allocation, a scheduling stall — and re-running settles it. A measurement
// several times over budget cannot be noise, and re-running it is actively
// harmful: repeating the C360 case means asking for another 16 GiB, which can
// get the process killed before it reports anything.
const grossFactor = 4

// Check runs fn over an input of n bytes and fails tb when fn allocates or runs
// longer than the budget allows.
//
// fn must be repeatable: a near-miss is measured a second time and reported
// only if the second measurement also breaks the budget. That costs nothing on
// the overwhelmingly common in-budget path and keeps a stray concurrent
// allocation from being reported as a parser bug, while a blow-up (grossFactor
// times the budget or more) is reported from the first measurement.
func (b Budget) Check(tb testing.TB, n int, fn func()) {
	tb.Helper()
	allocated, elapsed := Measure(fn)
	why := b.exceeded(n, allocated, elapsed)
	if why == "" {
		return
	}
	if !b.gross(n, allocated, elapsed) {
		allocated, elapsed = Measure(fn)
		if why = b.exceeded(n, allocated, elapsed); why == "" {
			return
		}
	}
	tb.Fatalf("%s over budget for a %d-byte input: %s", b.What, n, why)
}

// gross reports whether a measurement is too far past the budget to be noise.
func (b Budget) gross(n int, allocated uint64, elapsed time.Duration) bool {
	maxBytes, maxTime := b.Allowance(n)
	return allocated/grossFactor > maxBytes || elapsed/grossFactor > maxTime
}
