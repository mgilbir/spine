//go:build unix

package main

import (
	"os"
	"syscall"
)

// lockExclusive takes an exclusive, non-blocking advisory lock on the open
// file (flock on Unix). It reports whether the lock is already held elsewhere,
// which serve reports as "file is locked".
func lockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
