package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEphemeralLifecycleHappy(t *testing.T) {
	req := request{
		CaseID: "test-001",
		Op:     "ephemeral_lifecycle",
		Spec: &wireSpec{
			AdapterID:    "generic",
			EnvOverrides: map[string]string{"FOO": "$EPHEMERAL_HOME"},
			SeedFiles:    map[string]string{"a.txt": "1"},
		},
	}
	resp := dispatch(req)
	if !resp.OK {
		t.Fatalf("ok=false: %v", resp.Error)
	}
	if resp.Inspection == nil || !resp.Inspection.Exists {
		t.Fatalf("inspection.exists false: %+v", resp.Inspection)
	}
	if got := resp.Inspection.EnvOverrides["FOO"]; got != resp.Inspection.Path {
		t.Fatalf("FOO=%q path=%q", got, resp.Inspection.Path)
	}
	found := false
	for _, f := range resp.Inspection.FilesPresent {
		if f == "a.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("a.txt not in files_present: %v", resp.Inspection.FilesPresent)
	}
	if resp.AfterDestroy == nil || resp.AfterDestroy.PathExists {
		t.Fatalf("after_destroy: %+v", resp.AfterDestroy)
	}
}

func TestUnknownOpReturnsError(t *testing.T) {
	resp := dispatch(request{CaseID: "x", Op: "no_such_op"})
	if resp.OK {
		t.Fatal("expected ok=false")
	}
	if resp.Error == nil || resp.Error.Kind != "InternalInvariantViolation" {
		t.Fatalf("error: %+v", resp.Error)
	}
}

func TestBannerJSONShape(t *testing.T) {
	var sb strings.Builder
	writeBanner(&sb)
	var b map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(sb.String())), &b); err != nil {
		t.Fatal(err)
	}
	if b["protocol"] != "agent-venv.conformance" {
		t.Fatalf("protocol: %v", b["protocol"])
	}
	if int(b["version"].(float64)) != 2 {
		t.Fatalf("version: %v", b["version"])
	}
	if b["language"] != "go" {
		t.Fatalf("language: %v", b["language"])
	}
}
