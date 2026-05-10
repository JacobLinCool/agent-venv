package agentvenv

import "testing"

func TestVersionConstants(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must be set")
	}
	if SpecVersion == "" {
		t.Fatal("SpecVersion must be set")
	}
}
