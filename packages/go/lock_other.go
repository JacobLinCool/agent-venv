//go:build !unix

package agentvenv

import "os"

// v0: no locking on non-Unix. The registry is single-process-safe but not
// multi-process-safe on Windows; documented in the spec.

func acquireLock(_ *os.File) error { return nil }
func releaseLock(_ *os.File) error { return nil }
