package agentvenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClaudeCodeWithoutCredentialsHasOnboardingMarker(t *testing.T) {
	a := ClaudeCode{}
	spec, err := a.BuildSpec(false)
	if err != nil {
		t.Fatalf("BuildSpec: %v", err)
	}
	if spec.AdapterID != "claude-code" {
		t.Fatalf("adapter_id: %q", spec.AdapterID)
	}
	if spec.EnvOverrides["CLAUDE_CONFIG_DIR"] != "$EPHEMERAL_HOME" {
		t.Fatalf("env_overrides: %v", spec.EnvOverrides)
	}
	if spec.SeedFiles[".claude.json"] == "" {
		t.Fatal("expected onboarding marker .claude.json")
	}
	if len(spec.Credentials) != 0 {
		t.Fatal("credentials should be empty when load=false")
	}
}

func TestClaudeCodeWithCredentialsFromHomeFile(t *testing.T) {
	if runtime.GOOS == "darwin" {
		// Keychain may shadow the file; this test only meaningful when file path is the source.
		t.Skip("darwin uses Keychain first; file-only test runs on linux CI")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), []byte(`{"oauth":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a := ClaudeCode{}
	spec, err := a.BuildSpec(true)
	if err != nil {
		t.Fatalf("BuildSpec: %v", err)
	}
	if spec.Credentials[".credentials.json"] != `{"oauth":"x"}` {
		t.Fatalf("credentials not loaded: %v", spec.Credentials)
	}
}

func TestClaudeCodeMissingCredentialsRaises(t *testing.T) {
	if runtime.GOOS == "darwin" {
		// On darwin the Keychain may have a real entry; we cannot guarantee miss.
		t.Skip("darwin Keychain may satisfy the lookup")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := ClaudeCode{}
	_, err := a.BuildSpec(true)
	if !errors.Is(err, ErrCredentialsMissing) {
		t.Fatalf("expected CredentialsMissing, got %v", err)
	}
}

func TestCodexWithCredentialsFromHomeFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "auth.json"), []byte(`{"k":"v"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a := Codex{}
	spec, err := a.BuildSpec(true)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Credentials["auth.json"] != `{"k":"v"}` {
		t.Fatalf("auth.json not loaded: %v", spec.Credentials)
	}
	if spec.AdapterID != "codex" {
		t.Fatalf("adapter_id: %q", spec.AdapterID)
	}
}

func TestNewEphemeralForUsesAdapterSpec(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("relies on file-based credential path, not Keychain")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), []byte(`x`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	env, err := NewEphemeralFor(ctx, ClaudeCode{})
	if err != nil {
		t.Fatal(err)
	}
	defer env.Destroy(ctx)
	if env.AdapterID() != "claude-code" {
		t.Fatalf("adapter_id: %q", env.AdapterID())
	}
	if env.EnvOverrides()["CLAUDE_CONFIG_DIR"] != env.Path() {
		t.Fatalf("CLAUDE_CONFIG_DIR=%q path=%q", env.EnvOverrides()["CLAUDE_CONFIG_DIR"], env.Path())
	}
}
