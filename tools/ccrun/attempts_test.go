package main

import (
	"path/filepath"
	"testing"
)

// TestAttemptsRetryCap proves a reference that keeps failing transiently is
// deferred up to maxFetchAttempts and then reported exhausted on the next
// selection — the 4th selection for a cap of 3 — and that the counter survives
// a process restart so the cap is durable.
func TestAttemptsRetryCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempts.tsv")

	a, err := openAttempts(path)
	if err != nil {
		t.Fatal(err)
	}
	const digest = "D1"

	// Simulate three consecutive batches, each of which transient-fails the
	// reference. Before each fetch the selection loop must NOT consider it
	// exhausted; after the fetch it bumps the counter.
	for sel := 1; sel <= maxFetchAttempts; sel++ {
		if a.Exhausted(digest) {
			t.Fatalf("selection %d: exhausted too early (count=%d)", sel, a.Count(digest))
		}
		if _, err := a.Bump(digest); err != nil {
			t.Fatal(err)
		}
	}
	// After maxFetchAttempts defers, the next selection must retire it.
	if !a.Exhausted(digest) {
		t.Fatalf("selection %d: not exhausted after %d transient failures (count=%d)",
			maxFetchAttempts+1, maxFetchAttempts, a.Count(digest))
	}
	if a.Count(digest) != maxFetchAttempts {
		t.Errorf("count = %d, want %d", a.Count(digest), maxFetchAttempts)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	// Restart: a fresh process must still see the reference as exhausted, and a
	// never-seen reference must be fresh.
	a2, err := openAttempts(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a2.Close() }()
	if !a2.Exhausted(digest) {
		t.Errorf("after restart: %s not exhausted (count=%d)", digest, a2.Count(digest))
	}
	if a2.Count("D2") != 0 || a2.Exhausted("D2") {
		t.Errorf("unseen digest should be fresh, got count=%d", a2.Count("D2"))
	}
}
