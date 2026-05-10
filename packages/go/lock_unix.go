//go:build unix

package agentvenv

import (
	"os"

	"golang.org/x/sys/unix"
)

func acquireLock(f *os.File) error {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return newErr(ErrRegistryUnavailable, "flock LOCK_EX failed", err)
	}
	return nil
}

func releaseLock(f *os.File) error {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
		return newErr(ErrRegistryUnavailable, "flock LOCK_UN failed", err)
	}
	return nil
}
