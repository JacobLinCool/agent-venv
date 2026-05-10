package agentvenv

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// EnvironmentSpec describes a profile directory to materialize.
// See spec/environment-spec.schema.json.
type EnvironmentSpec struct {
	AdapterID    string                 // "claude-code" / "codex" / "generic"
	EnvOverrides map[string]string      // values may contain "$EPHEMERAL_HOME"
	SeedFiles    map[string]string      // relative path -> contents
	FileModes    map[string]fs.FileMode // relative path -> mode bits (Unix)
	Credentials  map[string]string      // relative path -> contents (default mode 0600)
	Prefix       string                 // ephemeral tmpdir prefix
}

func (s *EnvironmentSpec) adapterIDOrDefault() string {
	if s.AdapterID == "" {
		return "generic"
	}
	return s.AdapterID
}

func (s *EnvironmentSpec) prefixOrDefault() string {
	if s.Prefix == "" {
		return "agent-venv-"
	}
	return s.Prefix
}

func (s *EnvironmentSpec) validate() error {
	for rel := range s.SeedFiles {
		if err := validateRelPath(rel); err != nil {
			return err
		}
	}
	for rel := range s.Credentials {
		if err := validateRelPath(rel); err != nil {
			return err
		}
	}
	for rel := range s.FileModes {
		if err := validateRelPath(rel); err != nil {
			return err
		}
	}
	return nil
}

func validateRelPath(rel string) error {
	if rel == "" {
		return newErr(ErrInvalidEnvironmentSpec, "empty path", nil)
	}
	if filepath.IsAbs(rel) {
		return newErr(ErrInvalidEnvironmentSpec, "absolute path: "+rel, nil)
	}
	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return newErr(ErrInvalidEnvironmentSpec, "path escapes profile: "+rel, nil)
	}
	return nil
}
