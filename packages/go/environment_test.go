package agentvenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewEphemeralCreatesDirAndReturnsHandle(t *testing.T) {
	ctx := context.Background()
	spec := EnvironmentSpec{
		EnvOverrides: map[string]string{"FOO": "$EPHEMERAL_HOME"},
		SeedFiles:    map[string]string{"hello.txt": "world"},
	}
	env, err := NewEphemeral(ctx, spec)
	if err != nil {
		t.Fatalf("NewEphemeral: %v", err)
	}
	defer env.Destroy(ctx)

	if env.Path() == "" {
		t.Fatal("Path empty")
	}
	if _, err := os.Stat(env.Path()); err != nil {
		t.Fatalf("profile dir missing: %v", err)
	}
	if env.IsPersistent() {
		t.Fatal("ephemeral env reported as persistent")
	}
	if env.Name() != "" {
		t.Fatalf("ephemeral name should be empty, got %q", env.Name())
	}
	if env.AdapterID() != "generic" {
		t.Fatalf("adapter_id default: %q", env.AdapterID())
	}
	if env.EnvOverrides()["FOO"] != env.Path() {
		t.Fatalf("FOO not resolved: %q (path=%q)", env.EnvOverrides()["FOO"], env.Path())
	}
	body, err := os.ReadFile(filepath.Join(env.Path(), "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "world" {
		t.Fatalf("hello.txt: %q", body)
	}
}

func TestEphemeralDestroyRemovesDir(t *testing.T) {
	ctx := context.Background()
	env, err := NewEphemeral(ctx, EnvironmentSpec{})
	if err != nil {
		t.Fatal(err)
	}
	path := env.Path()
	if err := env.Destroy(ctx); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dir still exists: %v", err)
	}
	if err := env.Destroy(ctx); err != nil {
		t.Fatalf("second Destroy: %v", err)
	}
}

func TestEphemeralEnvOverridesIsCopy(t *testing.T) {
	ctx := context.Background()
	env, err := NewEphemeral(ctx, EnvironmentSpec{
		EnvOverrides: map[string]string{"K": "v"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer env.Destroy(ctx)
	eo := env.EnvOverrides()
	eo["K"] = "MUTATED"
	if env.EnvOverrides()["K"] != "v" {
		t.Fatal("EnvOverrides() must return a defensive copy")
	}
}

func TestEphemeralEmitsLifecycleEvents(t *testing.T) {
	ctx := context.Background()
	env, err := NewEphemeral(ctx, EnvironmentSpec{
		SeedFiles: map[string]string{"a": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.Destroy(ctx); err != nil {
		t.Fatal(err)
	}
	var seen []EventKind
	for _, e := range env.Events() {
		seen = append(seen, e.Kind)
	}
	want := []EventKind{EventEnvCreated, EventProfileMaterialized, EventEnvDestroyed}
	for _, w := range want {
		found := false
		for _, s := range seen {
			if s == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("event %s missing in %v", w, seen)
		}
	}
}

func TestNewEphemeralRejectsInvalidSpec(t *testing.T) {
	ctx := context.Background()
	_, err := NewEphemeral(ctx, EnvironmentSpec{
		SeedFiles: map[string]string{"/abs": "x"},
	})
	if !errors.Is(err, ErrInvalidEnvironmentSpec) {
		t.Fatalf("expected InvalidEnvironmentSpec, got %v", err)
	}
}
