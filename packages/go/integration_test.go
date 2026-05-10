//go:build integration

package agentvenv

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
)

// TestRegistryConcurrentGoroutines checks that within one process, two
// goroutines racing on create_or_attach for the same name resolve to the
// same path with no duplicate entries.
func TestRegistryConcurrentGoroutines(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	var wg sync.WaitGroup
	paths := make([]string, 8)
	var errs []error
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			env, err := CreateOrAttach(ctx, "race", EnvironmentSpec{}, WithRegistryRoot(root))
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			paths[i] = env.Path()
		}(i)
	}
	wg.Wait()
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	for i, p := range paths {
		if p != paths[0] {
			t.Fatalf("path[%d]=%q != path[0]=%q", i, p, paths[0])
		}
	}
	names, _ := List(ctx, WithRegistryRoot(root))
	if len(names) != 1 || names[0] != "race" {
		t.Fatalf("names=%v", names)
	}
}

// TestRegistryConcurrentSubprocesses spawns several helper subprocesses
// that each call create_or_attach against the same registry root,
// verifying the flock-protected index converges to one entry.
func TestRegistryConcurrentSubprocesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("flock-required test")
	}
	if os.Getenv("AGENT_VENV_HELPER") == "1" {
		ctx := context.Background()
		env, err := CreateOrAttach(ctx, "subproc", EnvironmentSpec{},
			WithRegistryRoot(os.Getenv("AGENT_VENV_TEST_ROOT")))
		if err != nil {
			os.Exit(2)
		}
		os.Stdout.WriteString(env.Path())
		os.Exit(0)
	}
	root := t.TempDir()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	out := make([]string, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(helper, "-test.run", "TestRegistryConcurrentSubprocesses")
			cmd.Env = append(os.Environ(),
				"AGENT_VENV_HELPER=1",
				"AGENT_VENV_TEST_ROOT="+root,
			)
			b, err := cmd.Output()
			if err != nil {
				t.Errorf("child %d: %v", i, err)
				return
			}
			out[i] = string(b)
		}(i)
	}
	wg.Wait()
	for i := 1; i < 4; i++ {
		if out[i] != out[0] {
			t.Fatalf("path mismatch: %q vs %q", out[0], out[i])
		}
	}
	names, _ := List(context.Background(), WithRegistryRoot(root))
	if len(names) != 1 {
		t.Fatalf("names=%v", names)
	}
}
