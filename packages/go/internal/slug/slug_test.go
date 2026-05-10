package slug

import "testing"

func TestOfDeterministic(t *testing.T) {
	a := Of("test-name")
	b := Of("test-name")
	if a != b {
		t.Fatalf("not deterministic: %q != %q", a, b)
	}
	if len(a) != 16 {
		t.Fatalf("expected 16 hex chars, got %d (%q)", len(a), a)
	}
}

func TestOfDifferentInputs(t *testing.T) {
	if Of("a") == Of("b") {
		t.Fatal("different inputs produced same slug")
	}
}
