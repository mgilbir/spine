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
//
// Tripped reports whether it ever refused anything, which the caller needs: a
// limit chosen from one save can exceed what a later save of the same document
// writes, because a timestamp that advanced between them compresses a byte
// differently. Such a run never truncates, and demanding an error from it is
// demanding one the writer had no reason to give.
func FailAfter(n int) *FailWriter { return &FailWriter{limit: n} }

// FailWriter accepts a fixed number of bytes and fails after them.
type FailWriter struct {
	limit   int
	written int
	tripped bool
}

// Tripped reports whether the writer ever returned ErrFull.
func (f *FailWriter) Tripped() bool { return f.tripped }

func (f *FailWriter) Write(p []byte) (int, error) {
	if f.written+len(p) <= f.limit {
		f.written += len(p)
		return len(p), nil
	}
	allowed := f.limit - f.written
	if allowed < 0 {
		allowed = 0
	}
	f.written += allowed
	f.tripped = true
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

// ReaderAt returns an io.ReaderAt over data that fails any read reaching at or
// past failFrom, as a truncated file or a disk with a bad sector does.
//
// The read side needs its own failure mode. A package is opened through an
// io.ReaderAt the library does not control either, and the failure there is
// more dangerous than on the write side: a short read that is not reported as
// an error becomes a part that looks complete and is not — a document silently
// missing half a sheet, rather than an error a caller can act on.
func ReaderAt(data []byte, failFrom int64) io.ReaderAt {
	return &faultyReaderAt{data: data, failFrom: failFrom}
}

type faultyReaderAt struct {
	data     []byte
	failFrom int64
}

func (f *faultyReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= f.failFrom {
		return 0, ErrFull
	}
	if off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(p, f.data[off:])
	// A read that starts before the fault and runs into it delivers the bytes
	// up to it and then fails, which is what a short read looks like.
	if off+int64(n) > f.failFrom {
		n = int(f.failFrom - off)
		return n, ErrFull
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// ReaderAtWindow returns an io.ReaderAt over data that fails only for reads
// overlapping [from, to).
//
// A fault that begins at an offset and continues to the end breaks the open
// itself: a zip is read from its central directory at the end and its content
// types near the beginning, so almost any such fault stops the package from
// opening and the part-reading paths are never reached. A window leaves the
// structure readable and corrupts one part's bytes, which is the case where a
// short read can be mistaken for a short part.
func ReaderAtWindow(data []byte, from, to int64) io.ReaderAt {
	return &windowReaderAt{data: data, from: from, to: to}
}

type windowReaderAt struct {
	data     []byte
	from, to int64
}

func (w *windowReaderAt) ReadAt(p []byte, off int64) (int, error) {
	end := off + int64(len(p))
	if off < w.to && end > w.from {
		return 0, ErrFull
	}
	if off >= int64(len(w.data)) {
		return 0, io.EOF
	}
	n := copy(p, w.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
