package agentvenv

import (
	"context"
	"errors"
	"os"
)

// Environment is a handle to a materialized profile directory.
type Environment struct {
	path         string
	envOverrides map[string]string
	spec         EnvironmentSpec
	log          *eventLog
	name         string // "" for ephemeral
	persistent   bool
	registry     *registry // nil for ephemeral; set for persistent
	destroyed    bool
}

// Path returns the absolute profile directory path.
func (e *Environment) Path() string { return e.path }

// EnvOverrides returns a defensive copy of the resolved env vars.
func (e *Environment) EnvOverrides() map[string]string {
	out := make(map[string]string, len(e.envOverrides))
	for k, v := range e.envOverrides {
		out[k] = v
	}
	return out
}

// AdapterID returns the spec's adapter_id (or "generic" if unset).
func (e *Environment) AdapterID() string { return e.spec.adapterIDOrDefault() }

// Name returns the persistent name; "" for ephemeral environments.
func (e *Environment) Name() string { return e.name }

// IsPersistent reports whether the environment is registered in the registry.
func (e *Environment) IsPersistent() bool { return e.persistent }

// Events returns a copy of the environment's event log.
func (e *Environment) Events() []Event { return e.log.all() }

// config holds option values; populated by Option functions.
type config struct {
	registryRoot string
	eventSink    EventSink
}

// Option configures NewEphemeral / CreateOrAttach / Attach / List / DestroyByName.
type Option func(*config)

// WithRegistryRoot overrides the default registry path.
func WithRegistryRoot(path string) Option { return func(c *config) { c.registryRoot = path } }

// WithEventSink registers an EventSink that receives every event emitted
// by this environment. Sink callbacks must not block.
func WithEventSink(sink EventSink) Option { return func(c *config) { c.eventSink = sink } }

func optionsToConfig(opts []Option) config {
	var c config
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// NewEphemeral creates an ephemeral environment from the given spec.
// Destroy(ctx) on the returned handle is idempotent and the only contract
// for cleanup; no Drop equivalent.
func NewEphemeral(ctx context.Context, spec EnvironmentSpec, opts ...Option) (*Environment, error) {
	if err := ctx.Err(); err != nil {
		return nil, newErr(ErrProfileSetupFailed, "context cancelled", err)
	}
	cfg := optionsToConfig(opts)
	log := newEventLog(cfg.eventSink)

	dir, err := os.MkdirTemp("", spec.prefixOrDefault())
	if err != nil {
		return nil, newErr(ErrProfileSetupFailed, "mkdtemp", err)
	}
	log.emit(EventEnvCreated, map[string]any{
		"name":       nil,
		"lifetime":   "ephemeral",
		"adapter_id": spec.adapterIDOrDefault(),
		"path":       dir,
	})

	eo, err := materialize(dir, spec, log, false)
	if err != nil {
		_ = os.RemoveAll(dir)
		log.emit(EventError, map[string]any{
			"error_kind": kindOf(err),
			"message":    err.Error(),
		})
		return nil, err
	}

	return &Environment{
		path:         dir,
		envOverrides: eo,
		spec:         spec,
		log:          log,
		persistent:   false,
	}, nil
}

// Destroy removes the profile directory (ephemeral) or both the directory
// and the registry entry (persistent). Idempotent.
func (e *Environment) Destroy(ctx context.Context) error {
	if e.destroyed {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return newErr(ErrCleanupFailed, "context cancelled", err)
	}
	if e.persistent && e.registry != nil && e.name != "" {
		ok, _, cerr := e.registry.remove(e.name)
		e.log.emit(EventRegistryWritten, map[string]any{"path": e.registry.indexPath})
		e.log.emit(EventEnvDestroyed, map[string]any{"path": e.path, "ok": ok})
		e.destroyed = true
		if !ok {
			return newErr(ErrCleanupFailed, "removing persistent env", cerr)
		}
		return nil
	}
	err := removeDir(e.path)
	ok := err == nil
	e.log.emit(EventEnvDestroyed, map[string]any{"path": e.path, "ok": ok})
	e.destroyed = true
	if !ok {
		return newErr(ErrCleanupFailed, "rmtree ephemeral profile", err)
	}
	return nil
}

// RefreshCredentials re-writes spec.Credentials into the profile directory.
// Useful when the host's credentials have rotated.
func (e *Environment) RefreshCredentials(ctx context.Context) error {
	if e.destroyed {
		return newErr(ErrInternalInvariantViolation, "RefreshCredentials on destroyed environment", nil)
	}
	if err := ctx.Err(); err != nil {
		return newErr(ErrProfileSetupFailed, "context cancelled", err)
	}
	if len(e.spec.Credentials) == 0 {
		return nil
	}
	count, _, err := writeFiles(e.path, e.spec.Credentials, e.spec.FileModes, 0o600)
	if err != nil {
		return err
	}
	e.log.emit(EventCredentialsRefresh, map[string]any{"file_count": count})
	return nil
}

// kindOf extracts the Kind string from an *Error chain; returns
// "InternalInvariantViolation" if the error is nil or not an *Error.
func kindOf(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return "InternalInvariantViolation"
}

