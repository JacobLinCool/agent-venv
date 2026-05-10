package agentvenv

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestRegistryLockSerialisesGoroutines(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("flock is unix-only in v0")
	}
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")
	var counter int
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				t.Errorf("open: %v", err)
				return
			}
			defer f.Close()
			if err := acquireLock(f); err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			time.Sleep(20 * time.Millisecond)
			counter++
			_ = releaseLock(f)
		}()
	}
	wg.Wait()
	if counter != 5 {
		t.Fatalf("counter=%d (lock not exclusive)", counter)
	}
}
