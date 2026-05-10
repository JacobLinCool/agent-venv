// Package agentvenv provides isolated profile directories for coding-agent
// CLIs (Claude Code, Codex, ...). It does not run the agent. The caller
// spawns the binary themselves with the env vars this package returns.
//
// See the spec at https://github.com/JacobLinCool/agent-venv/tree/main/spec
// for the cross-language contract this package implements.
package agentvenv

const (
	// Version is the semver of this package. The release workflow asserts
	// that the git tag (packages/go/vX.Y.Z) matches this constant.
	Version = "0.1.0"

	// SpecVersion is the spec/ contract version this implementation targets.
	SpecVersion = "0.1"
)
