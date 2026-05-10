package agentvenv

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRegistryRootHonoursEnv(t *testing.T) {
	t.Setenv("AGENT_VENV_REGISTRY_ROOT", "/tmp/agent-venv-test-x")
	if got := DefaultRegistryRoot(); got != "/tmp/agent-venv-test-x" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultRegistryRootXDG(t *testing.T) {
	t.Setenv("AGENT_VENV_REGISTRY_ROOT", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-test")
	want := filepath.Join("/tmp/xdg-test", "agent-venv", "envs")
	if got := DefaultRegistryRoot(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRegistryReserveOrGetCreatesEntry(t *testing.T) {
	root := t.TempDir()
	r := newRegistry(root)
	envDir, meta, created, err := r.reserveOrGet("alice", "generic")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if meta.Name != "alice" || meta.AdapterID != "generic" {
		t.Fatalf("meta: %+v", meta)
	}
	if _, err := os.Stat(filepath.Join(envDir, "metadata.json")); err != nil {
		t.Fatalf("metadata.json missing: %v", err)
	}
	envDir2, _, created2, err := r.reserveOrGet("alice", "generic")
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("second call should not create")
	}
	if envDir2 != envDir {
		t.Fatalf("path differs: %q vs %q", envDir, envDir2)
	}
}

func TestRegistryAdapterMismatch(t *testing.T) {
	root := t.TempDir()
	r := newRegistry(root)
	if _, _, _, err := r.reserveOrGet("a", "claude-code"); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := r.reserveOrGet("a", "codex")
	if !errors.Is(err, ErrAdapterMismatch) {
		t.Fatalf("expected AdapterMismatch, got %v", err)
	}
}

func TestRegistryListNamesSorted(t *testing.T) {
	root := t.TempDir()
	r := newRegistry(root)
	for _, n := range []string{"c", "a", "b"} {
		if _, _, _, err := r.reserveOrGet(n, "generic"); err != nil {
			t.Fatal(err)
		}
	}
	names, err := r.listNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 || names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Fatalf("names: %v", names)
	}
}

func TestRegistryRemoveOK(t *testing.T) {
	root := t.TempDir()
	r := newRegistry(root)
	envDir, _, _, err := r.reserveOrGet("rm-me", "generic")
	if err != nil {
		t.Fatal(err)
	}
	ok, _, cerr := r.remove("rm-me")
	if !ok {
		t.Fatalf("remove failed: %v", cerr)
	}
	if _, err := os.Stat(envDir); !os.IsNotExist(err) {
		t.Fatal("envDir should be gone")
	}
	names, _ := r.listNames()
	for _, n := range names {
		if n == "rm-me" {
			t.Fatal("name still in index")
		}
	}
}

func TestRegistryRemoveMissing(t *testing.T) {
	root := t.TempDir()
	r := newRegistry(root)
	_, _, err := r.remove("missing")
	if !errors.Is(err, ErrEnvironmentNotFound) {
		t.Fatalf("expected EnvironmentNotFound, got %v", err)
	}
}

func TestRegistryLookupReturnsMetadata(t *testing.T) {
	root := t.TempDir()
	r := newRegistry(root)
	if _, _, _, err := r.reserveOrGet("look", "generic"); err != nil {
		t.Fatal(err)
	}
	envDir, meta, ok, err := r.lookup("look")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("lookup miss")
	}
	if meta.Name != "look" {
		t.Fatalf("meta.Name=%q", meta.Name)
	}
	if envDir == "" {
		t.Fatal("envDir empty")
	}
}
