package agentvenv

import (
	"errors"
	"io/fs"
	"testing"
)

func TestSpecValidateAcceptsRelativePaths(t *testing.T) {
	s := EnvironmentSpec{
		SeedFiles: map[string]string{"a.txt": "x", "nested/b.txt": "y"},
	}
	if err := s.validate(); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestSpecValidateRejectsAbsolutePath(t *testing.T) {
	s := EnvironmentSpec{SeedFiles: map[string]string{"/etc/passwd": "x"}}
	err := s.validate()
	if !errors.Is(err, ErrInvalidEnvironmentSpec) {
		t.Fatalf("expected InvalidEnvironmentSpec, got %v", err)
	}
}

func TestSpecValidateRejectsParentTraversal(t *testing.T) {
	s := EnvironmentSpec{SeedFiles: map[string]string{"../escape": "x"}}
	err := s.validate()
	if !errors.Is(err, ErrInvalidEnvironmentSpec) {
		t.Fatalf("expected InvalidEnvironmentSpec, got %v", err)
	}
}

func TestSpecValidateChecksAllMaps(t *testing.T) {
	bad := "../bad"
	cases := []EnvironmentSpec{
		{Credentials: map[string]string{bad: "x"}},
		{FileModes: map[string]fs.FileMode{bad: 0o600}},
	}
	for _, s := range cases {
		if err := s.validate(); !errors.Is(err, ErrInvalidEnvironmentSpec) {
			t.Fatalf("expected InvalidEnvironmentSpec, got %v", err)
		}
	}
}

func TestSpecAdapterIDDefaultsToGeneric(t *testing.T) {
	s := EnvironmentSpec{}
	if got := s.adapterIDOrDefault(); got != "generic" {
		t.Fatalf("default adapter_id should be 'generic', got %q", got)
	}
}
