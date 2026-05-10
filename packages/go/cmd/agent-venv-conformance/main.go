// agent-venv-conformance is the Go side of the cross-language conformance
// harness. It speaks the NDJSON protocol described in
// spec/conformance-protocol.md (version 2).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	agentvenv "github.com/JacobLinCool/agent-venv/packages/go"
)

// ---------------------------------------------------------------------------
// Wire types — kept private so the public API stays free of JSON tags.
// ---------------------------------------------------------------------------

type wireSpec struct {
	AdapterID    string            `json:"adapter_id,omitempty"`
	EnvOverrides map[string]string `json:"env_overrides,omitempty"`
	SeedFiles    map[string]string `json:"seed_files,omitempty"`
	FileModes    map[string]int    `json:"file_modes,omitempty"`
	Credentials  map[string]string `json:"credentials,omitempty"`
	Prefix       string            `json:"prefix,omitempty"`
}

func (w *wireSpec) toEnvironmentSpec() agentvenv.EnvironmentSpec {
	if w == nil {
		return agentvenv.EnvironmentSpec{}
	}
	fm := make(map[string]fs.FileMode, len(w.FileModes))
	for k, v := range w.FileModes {
		fm[k] = fs.FileMode(v)
	}
	return agentvenv.EnvironmentSpec{
		AdapterID:    w.AdapterID,
		EnvOverrides: w.EnvOverrides,
		SeedFiles:    w.SeedFiles,
		FileModes:    fm,
		Credentials:  w.Credentials,
		Prefix:       w.Prefix,
	}
}

type request struct {
	CaseID          string    `json:"case_id"`
	Op              string    `json:"op"`
	Spec            *wireSpec `json:"spec,omitempty"`
	FirstSpec       *wireSpec `json:"first_spec,omitempty"`
	SecondAdapterID string    `json:"second_adapter_id,omitempty"`
	Name            string    `json:"name,omitempty"`
	Names           []string  `json:"names,omitempty"`
	RegistryRoot    string    `json:"registry_root,omitempty"`
}

type errorPayload struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type inspection struct {
	Path         string            `json:"path"`
	Exists       bool              `json:"exists"`
	EnvOverrides map[string]string `json:"env_overrides"`
	FilesPresent []string          `json:"files_present"`
	FileModes    map[string]int    `json:"file_modes"`
}

type afterDestroy struct {
	PathExists bool `json:"path_exists"`
}

type response struct {
	CaseID                 string           `json:"case_id"`
	OK                     bool             `json:"ok"`
	Error                  *errorPayload    `json:"error,omitempty"`
	Events                 []map[string]any `json:"events,omitempty"`
	Inspection             *inspection      `json:"inspection,omitempty"`
	AfterDestroy           *afterDestroy    `json:"after_destroy,omitempty"`
	Paths                  []string         `json:"paths,omitempty"`
	Path                   string           `json:"path,omitempty"`
	SecondPathFilesPresent []string         `json:"second_path_files_present,omitempty"`
	NamesListed            []string         `json:"names_listed,omitempty"`
	CreatedPath            string           `json:"created_path,omitempty"`
	PathExistsAfter        bool             `json:"path_exists_after"`
	NameInIndexAfter       bool             `json:"name_in_index_after"`
	FilesPresent           []string         `json:"files_present,omitempty"`
}

// ---------------------------------------------------------------------------
// Dispatch
// ---------------------------------------------------------------------------

func dispatch(req request) response {
	ctx := context.Background()
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic in dispatch: %v\n", r)
		}
	}()
	switch req.Op {
	case "ephemeral_lifecycle":
		return opEphemeralLifecycle(ctx, req)
	default:
		return errorResp(req.CaseID, "InternalInvariantViolation", "unknown op: "+req.Op, false)
	}
}

func opEphemeralLifecycle(ctx context.Context, req request) response {
	spec := req.Spec.toEnvironmentSpec()
	env, err := agentvenv.NewEphemeral(ctx, spec)
	if err != nil {
		return errorResp(req.CaseID, kindOf(err), err.Error(), true)
	}
	insp := inspectEnv(env)
	derr := env.Destroy(ctx)
	resp := response{
		CaseID:       req.CaseID,
		OK:           true,
		Events:       eventsOf(env),
		Inspection:   &insp,
		AfterDestroy: &afterDestroy{PathExists: pathExists(env.Path())},
	}
	if derr != nil {
		resp.Error = &errorPayload{Kind: kindOf(derr), Message: derr.Error()}
	}
	return resp
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func eventsOf(env *agentvenv.Environment) []map[string]any {
	if env == nil {
		return nil
	}
	raw := env.Events()
	out := make([]map[string]any, 0, len(raw))
	for _, e := range raw {
		m := map[string]any{"ts_ms": e.TimestampMs, "kind": string(e.Kind)}
		for k, v := range e.Detail {
			// The 'kind' field on the event is the event kind itself; per
			// spec/events.schema.json we use 'error_kind' inside the error
			// event payload to avoid collision when serialized flat.
			if k == "kind" {
				k = "error_kind"
			}
			m[k] = v
		}
		out = append(out, m)
	}
	return out
}

func errorResp(caseID, kind, msg string, ok bool) response {
	return response{CaseID: caseID, OK: ok, Error: &errorPayload{Kind: kind, Message: msg}}
}

func kindOf(err error) string {
	var e *agentvenv.Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return "InternalInvariantViolation"
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func inspectEnv(env *agentvenv.Environment) inspection {
	insp := inspection{
		Path:         env.Path(),
		Exists:       pathExists(env.Path()),
		EnvOverrides: env.EnvOverrides(),
		FilesPresent: []string{},
		FileModes:    map[string]int{},
	}
	base := env.Path()
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(base, p)
		rel = filepath.ToSlash(rel)
		insp.FilesPresent = append(insp.FilesPresent, rel)
		if runtime.GOOS != "windows" {
			if st, err := os.Stat(p); err == nil {
				insp.FileModes[rel] = int(st.Mode().Perm())
			}
		}
		return nil
	})
	sort.Strings(insp.FilesPresent)
	return insp
}

func writeBanner(w io.Writer) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]any{
		"protocol":        "agent-venv.conformance",
		"version":         2,
		"language":        "go",
		"package_version": agentvenv.Version,
		"spec_version":    agentvenv.SpecVersion,
	})
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	writeBanner(os.Stdout)
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			_ = enc.Encode(response{
				OK:    false,
				Error: &errorPayload{Kind: "InternalInvariantViolation", Message: "bad request: " + err.Error()},
			})
			continue
		}
		resp := dispatch(req)
		if err := enc.Encode(resp); err != nil {
			return
		}
	}
}
