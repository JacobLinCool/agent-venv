package agentvenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateOrAttachIdempotent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	spec := EnvironmentSpec{
		SeedFiles:    map[string]string{"hello.txt": "world"},
		EnvOverrides: map[string]string{"FOO": "$EPHEMERAL_HOME"},
	}
	env1, err := CreateOrAttach(ctx, "idem", spec, WithRegistryRoot(root))
	if err != nil {
		t.Fatal(err)
	}
	env2, err := CreateOrAttach(ctx, "idem", spec, WithRegistryRoot(root))
	if err != nil {
		t.Fatal(err)
	}
	if env1.Path() != env2.Path() {
		t.Fatalf("paths differ: %q %q", env1.Path(), env2.Path())
	}
	if !env1.IsPersistent() || env1.Name() != "idem" {
		t.Fatalf("env1: name=%q persistent=%v", env1.Name(), env1.IsPersistent())
	}
	body, _ := os.ReadFile(filepath.Join(env2.Path(), "hello.txt"))
	if string(body) != "world" {
		t.Fatalf("hello.txt: %q", body)
	}
	var seenCreated, seenAttached bool
	for _, e := range env1.Events() {
		if e.Kind == EventEnvCreated {
			seenCreated = true
		}
	}
	for _, e := range env2.Events() {
		if e.Kind == EventEnvAttached {
			seenAttached = true
		}
	}
	if !seenCreated || !seenAttached {
		t.Fatalf("created=%v attached=%v", seenCreated, seenAttached)
	}
}

func TestAttachMissingReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_, err := Attach(ctx, "ghost", WithRegistryRoot(root))
	if !errors.Is(err, ErrEnvironmentNotFound) {
		t.Fatalf("expected EnvironmentNotFound, got %v", err)
	}
}

func TestListReturnsSortedNames(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for _, n := range []string{"c", "a", "b"} {
		if _, err := CreateOrAttach(ctx, n, EnvironmentSpec{}, WithRegistryRoot(root)); err != nil {
			t.Fatal(err)
		}
	}
	names, err := List(ctx, WithRegistryRoot(root))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	if len(names) != 3 {
		t.Fatalf("got %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v want %v", names, want)
		}
	}
}

func TestDestroyByName(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	env, err := CreateOrAttach(ctx, "dropme", EnvironmentSpec{}, WithRegistryRoot(root))
	if err != nil {
		t.Fatal(err)
	}
	p := env.Path()
	if err := DestroyByName(ctx, "dropme", WithRegistryRoot(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("path should be gone")
	}
	names, _ := List(ctx, WithRegistryRoot(root))
	for _, n := range names {
		if n == "dropme" {
			t.Fatal("name still listed")
		}
	}
}

func TestAdapterMismatchOnRecreate(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if _, err := CreateOrAttach(ctx, "a", EnvironmentSpec{AdapterID: "claude-code"}, WithRegistryRoot(root)); err != nil {
		t.Fatal(err)
	}
	_, err := CreateOrAttach(ctx, "a", EnvironmentSpec{AdapterID: "codex"}, WithRegistryRoot(root))
	if !errors.Is(err, ErrAdapterMismatch) {
		t.Fatalf("expected AdapterMismatch, got %v", err)
	}
}
