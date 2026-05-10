package agentvenv

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorKindMatchesSentinel(t *testing.T) {
	base := fmt.Errorf("wrap: %w", &Error{Kind: "EnvironmentNotFound", Msg: "x"})
	if !errors.Is(base, ErrEnvironmentNotFound) {
		t.Fatal("errors.Is should match by Kind")
	}
	if errors.Is(base, ErrAdapterMismatch) {
		t.Fatal("errors.Is should not match a different Kind")
	}
}

func TestErrorUnwrapsCause(t *testing.T) {
	inner := fmt.Errorf("inner")
	e := &Error{Kind: "ProfileSetupFailed", Msg: "outer", Cause: inner}
	if !errors.Is(e, inner) {
		t.Fatal("Unwrap chain should reach Cause")
	}
}

func TestErrorString(t *testing.T) {
	e := &Error{Kind: "EnvironmentNotFound", Msg: "no such env: foo"}
	if got := e.Error(); got != "EnvironmentNotFound: no such env: foo" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestAllSentinelsHaveKind(t *testing.T) {
	sentinels := []*Error{
		ErrEnvironmentNotFound, ErrEnvironmentAlreadyExists,
		ErrAdapterMismatch, ErrProfileSetupFailed, ErrRegistryUnavailable,
		ErrCredentialsMissing, ErrAdapterUnavailable, ErrCleanupFailed,
		ErrInvalidEnvironmentSpec, ErrInternalInvariantViolation,
	}
	for _, s := range sentinels {
		if s.Kind == "" {
			t.Fatalf("sentinel %v has empty Kind", s)
		}
	}
}
