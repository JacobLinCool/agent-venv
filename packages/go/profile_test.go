package agentvenv

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMaterializeWritesSeedFiles(t *testing.T) {
	dir := t.TempDir()
	log := newEventLog(nil)
	spec := EnvironmentSpec{
		SeedFiles: map[string]string{"a.txt": "1", "nested/b.txt": "2"},
	}
	eo, err := materialize(dir, spec, log, false)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(eo) != 0 {
		t.Fatalf("no env_overrides expected, got %v", eo)
	}
	for rel, want := range map[string]string{"a.txt": "1", "nested/b.txt": "2"} {
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(b) != want {
			t.Fatalf("%s: got %q want %q", rel, b, want)
		}
	}
}

func TestMaterializeReplacesEphemeralHome(t *testing.T) {
	dir := t.TempDir()
	log := newEventLog(nil)
	spec := EnvironmentSpec{
		EnvOverrides: map[string]string{"FOO": "$EPHEMERAL_HOME", "BAR": "$EPHEMERAL_HOME/sub"},
	}
	eo, err := materialize(dir, spec, log, false)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	abs, _ := filepath.Abs(dir)
	if eo["FOO"] != abs {
		t.Fatalf("FOO: got %q want %q", eo["FOO"], abs)
	}
	wantBar := filepath.Join(abs, "sub")
	if eo["BAR"] != wantBar {
		t.Fatalf("BAR: got %q want %q", eo["BAR"], wantBar)
	}
}

func TestMaterializeCredentialsDefaultMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are Unix-only")
	}
	dir := t.TempDir()
	log := newEventLog(nil)
	spec := EnvironmentSpec{
		Credentials: map[string]string{".credentials.json": `{"oauth":"x"}`},
	}
	if _, err := materialize(dir, spec, log, false); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, ".credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("default credentials mode: %v want 0600", info.Mode().Perm())
	}
}

func TestMaterializeAppliesExplicitFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are Unix-only")
	}
	dir := t.TempDir()
	log := newEventLog(nil)
	spec := EnvironmentSpec{
		SeedFiles: map[string]string{"data.txt": "x"},
		FileModes: map[string]fs.FileMode{"data.txt": 0o640},
	}
	if _, err := materialize(dir, spec, log, false); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode: %v want 0640", info.Mode().Perm())
	}
}

func TestMaterializeRejectsParentTraversal(t *testing.T) {
	dir := t.TempDir()
	log := newEventLog(nil)
	spec := EnvironmentSpec{SeedFiles: map[string]string{"../escape": "x"}}
	_, err := materialize(dir, spec, log, false)
	if !errors.Is(err, ErrInvalidEnvironmentSpec) {
		t.Fatalf("expected InvalidEnvironmentSpec, got %v", err)
	}
}

func TestMaterializeEmitsExpectedEvents(t *testing.T) {
	dir := t.TempDir()
	log := newEventLog(nil)
	spec := EnvironmentSpec{
		SeedFiles:   map[string]string{"a": "1"},
		Credentials: map[string]string{"c": "2"},
	}
	if _, err := materialize(dir, spec, log, false); err != nil {
		t.Fatal(err)
	}
	kinds := []EventKind{}
	for _, e := range log.all() {
		kinds = append(kinds, e.Kind)
	}
	has := func(k EventKind) bool {
		for _, x := range kinds {
			if x == k {
				return true
			}
		}
		return false
	}
	if !has(EventProfileMaterialized) {
		t.Fatalf("missing profile.materialized in %v", kinds)
	}
	if !has(EventCredentialsCopied) {
		t.Fatalf("missing credentials.copied in %v", kinds)
	}
}

func TestMaterializeSkipSeedIfExistsLeavesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	log := newEventLog(nil)
	spec := EnvironmentSpec{SeedFiles: map[string]string{"a.txt": "OVERWRITE"}}
	if _, err := materialize(dir, spec, log, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(b) != "kept" {
		t.Fatalf("skipSeedIfExists must not rewrite; got %q", b)
	}
}

func TestRemoveDirIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := removeDir(dir); err != nil {
		t.Fatalf("first remove: %v", err)
	}
	if err := removeDir(dir); err != nil {
		t.Fatalf("second remove (already gone): %v", err)
	}
}
