// Package faultio provides writers that fail part-way through, so a save can be
// tested against the failure it will eventually meet.
//
// Every save in this module ends in an io.Writer it does not control: a file on
// a disk that fills, a socket that closes, a caller's buffer with a cap. What
// happens on that path was untested in all three formats — not one test passed
// a failing writer to SaveTo — and it is not a path that can be reasoned about
// from the success case, because a save mutates the document as it goes. It
// stamps a modified timestamp, embeds pending media, allocates shape ids, and
// records baselines for the parts it has written. A save that fails half way
// through those has done some of them.
//
// The property that matters to a caller is not that the failure is clean, which
// nothing can promise once bytes have left the process, but that the *document*
// is: a save that fails must leave the model able to save correctly afterwards,
// to another writer or the same file retried. Otherwise the first disk-full
// error silently corrupts everything the program writes after it.
package faultio

import (
	"errors"
	"io"
)

// ErrFull is what a failing writer returns; it stands in for a full disk, a
// closed socket, or any other write that stops part way.
var ErrFull = errors.New("faultio: write failed")

// FailAfter returns a writer that accepts n bytes and then fails, writing the
// portion that fits and reporting a short write with ErrFull, as a real
// io.Writer must.
func FailAfter(n int) io.Writer { return &failAfter{limit: n} }

type failAfter struct {
	limit   int
	written int
}

func (f *failAfter) Write(p []byte) (int, error) {
	if f.written+len(p) <= f.limit {
		f.written += len(p)
		return len(p), nil
	}
	allowed := f.limit - f.written
	if allowed < 0 {
		allowed = 0
	}
	f.written += allowed
	return allowed, ErrFull
}

// Limits returns write limits to exercise: nothing, a few boundaries early
// enough to land inside the first parts, and points scaled to a known good
// output size, including one byte short of complete.
//
// The last is the interesting one. A save that streams a zip finishes with the
// central directory, so a writer that fails on the final byte has accepted
// every part and fails only at the end — the case most likely to leave a
// document that looks written and is not.
func Limits(cleanSize int) []int {
	var limits []int
	seen := map[int]bool{}
	add := func(n int) {
		// A limit at or above the output accepts every byte, so the writer
		// never fails and the case silently tests nothing.
		if n < 0 || n >= cleanSize || seen[n] {
			return
		}
		seen[n] = true
		limits = append(limits, n)
	}
	for _, n := range []int{0, 1, 64, 512, 4096} {
		add(n)
	}
	add(cleanSize / 4)
	add(cleanSize / 2)
	add(cleanSize - 1)
	return limits
}
