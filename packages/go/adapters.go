package agentvenv

import (
	"context"
	"io/fs"
	"os/exec"
)

// AgentAdapter is the Layer 2 contract: a small object that knows how to
// produce an EnvironmentSpec for a particular agent CLI. The library never
// runs the binary; an adapter is purely a spec factory.
type AgentAdapter interface {
	ID() string
	CLIBin() string
	ConfigEnvVar() string
	BuildSpec(loadCredentials bool) (EnvironmentSpec, error)
}

// ArgvBuilder is an optional capability. Detect with type assertion.
type ArgvBuilder interface {
	BuildArgv(prompt string, workspace string) []string
}

// AvailabilityChecker is an optional capability. Detect with type assertion.
type AvailabilityChecker interface {
	EnsureAvailable() error
}

// ---------------------------------------------------------------------------
// ClaudeCode
// ---------------------------------------------------------------------------

// ClaudeCode is the built-in adapter for the `claude` CLI.
type ClaudeCode struct {
	Model           string
	ReasoningEffort string
	ExtraArgv       []string
}

func (ClaudeCode) ID() string           { return "claude-code" }
func (ClaudeCode) CLIBin() string       { return "claude" }
func (ClaudeCode) ConfigEnvVar() string { return "CLAUDE_CONFIG_DIR" }

func (a ClaudeCode) BuildSpec(loadCredentials bool) (EnvironmentSpec, error) {
	spec := EnvironmentSpec{
		AdapterID:    a.ID(),
		EnvOverrides: map[string]string{a.ConfigEnvVar(): "$EPHEMERAL_HOME"},
		SeedFiles:    map[string]string{},
		Credentials:  map[string]string{},
		FileModes:    map[string]fs.FileMode{},
		Prefix:       "agent-venv-claude-",
	}
	if loadCredentials {
		body, err := loadHostClaudeCredentials(context.Background())
		if err != nil {
			return EnvironmentSpec{}, newErr(ErrCredentialsMissing, "loading Claude credentials", err)
		}
		spec.Credentials[".credentials.json"] = body
		spec.FileModes[".credentials.json"] = 0o600
	} else {
		spec.SeedFiles[".claude.json"] = `{"hasCompletedOnboarding": true}`
	}
	return spec, nil
}

func (a ClaudeCode) BuildArgv(prompt string, workspace string) []string {
	if a.Model == "" {
		return nil
	}
	argv := []string{
		a.CLIBin(),
		"--print",
		"--model", a.Model,
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
	}
	if a.ReasoningEffort != "" {
		argv = append(argv, "--effort", a.ReasoningEffort)
	}
	argv = append(argv, a.ExtraArgv...)
	return argv
}

func (a ClaudeCode) EnsureAvailable() error {
	if _, err := exec.LookPath(a.CLIBin()); err != nil {
		return newErr(ErrAdapterUnavailable, a.CLIBin()+" not on PATH", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Codex
// ---------------------------------------------------------------------------

// Codex is the built-in adapter for the `codex` CLI.
type Codex struct {
	Model           string
	ReasoningEffort string
	ExtraArgv       []string
}

func (Codex) ID() string           { return "codex" }
func (Codex) CLIBin() string       { return "codex" }
func (Codex) ConfigEnvVar() string { return "CODEX_HOME" }

func (a Codex) BuildSpec(loadCredentials bool) (EnvironmentSpec, error) {
	spec := EnvironmentSpec{
		AdapterID:    a.ID(),
		EnvOverrides: map[string]string{a.ConfigEnvVar(): "$EPHEMERAL_HOME"},
		Credentials:  map[string]string{},
		FileModes:    map[string]fs.FileMode{},
		Prefix:       "agent-venv-codex-",
	}
	if loadCredentials {
		body, err := loadHostCodexAuth()
		if err != nil {
			return EnvironmentSpec{}, newErr(ErrCredentialsMissing, "loading Codex auth.json", err)
		}
		spec.Credentials["auth.json"] = body
		spec.FileModes["auth.json"] = 0o600
	}
	return spec, nil
}

func (a Codex) BuildArgv(prompt string, workspace string) []string {
	if a.Model == "" {
		return nil
	}
	argv := []string{
		a.CLIBin(),
		"exec",
		"--model", a.Model,
	}
	if workspace != "" {
		argv = append(argv, "--cd", workspace)
	}
	argv = append(argv,
		"--json",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
	)
	if a.ReasoningEffort != "" {
		argv = append(argv, "-c", `model_reasoning_effort="`+a.ReasoningEffort+`"`)
	}
	argv = append(argv, a.ExtraArgv...)
	return argv
}

func (a Codex) EnsureAvailable() error {
	if _, err := exec.LookPath(a.CLIBin()); err != nil {
		return newErr(ErrAdapterUnavailable, a.CLIBin()+" not on PATH", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Layer 2 entry points
// ---------------------------------------------------------------------------

// NewEphemeralFor builds a spec from the adapter (with credentials loaded
// by default) and creates an ephemeral environment from it.
func NewEphemeralFor(ctx context.Context, a AgentAdapter, opts ...Option) (*Environment, error) {
	spec, err := a.BuildSpec(true)
	if err != nil {
		return nil, err
	}
	return NewEphemeral(ctx, spec, opts...)
}
