//go:build windows

package main

import (
	"errors"
	"os"
)

// lockExclusive is the Windows counterpart of the Unix flock. Advisory file
// locking needs LockFileEx, which is not in the standard library syscall
// package; adding golang.org/x/sys is the planned fix. Until then, serve
// refuses to start rather than pretending the single-writer guarantee holds.
func lockExclusive(f *os.File) error {
	return errors.New("spine serve is not supported on this platform yet (file locking)")
}
