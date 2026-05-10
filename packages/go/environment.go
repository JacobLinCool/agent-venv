package agentvenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
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

// CreateOrAttach creates a new persistent environment or attaches to an
// existing one with the same name. Idempotent: same name + same registry
// root + same adapter_id always returns a handle backed by the same path.
func CreateOrAttach(ctx context.Context, name string, spec EnvironmentSpec, opts ...Option) (*Environment, error) {
	if err := ctx.Err(); err != nil {
		return nil, newErr(ErrProfileSetupFailed, "context cancelled", err)
	}
	cfg := optionsToConfig(opts)
	log := newEventLog(cfg.eventSink)
	root := cfg.registryRoot
	if root == "" {
		root = DefaultRegistryRoot()
	}
	reg := newRegistry(root)

	envDir, meta, created, err := reg.reserveOrGet(name, spec.adapterIDOrDefault())
	if err != nil {
		return nil, err
	}
	profileDir := filepath.Join(envDir, "profile")

	var eo map[string]string
	if created {
		log.emit(EventEnvCreated, map[string]any{
			"name":       name,
			"lifetime":   "persistent",
			"adapter_id": spec.adapterIDOrDefault(),
			"path":       profileDir,
		})
		eo, err = materialize(profileDir, spec, log, false)
		if err != nil {
			return nil, err
		}
		meta.EnvOverrides = eo
		if len(spec.Credentials) > 0 {
			meta.CredentialsLoaded = true
			meta.CredentialsLoadedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if err := reg.writeMetadata(envDir, meta); err != nil {
			return nil, err
		}
		log.emit(EventRegistryWritten, map[string]any{"path": filepath.Join(envDir, "metadata.json")})
	} else {
		log.emit(EventEnvAttached, map[string]any{
			"name":       name,
			"adapter_id": meta.AdapterID,
			"path":       profileDir,
		})
		log.emit(EventRegistryRead, map[string]any{"path": filepath.Join(envDir, "metadata.json")})
		if len(meta.EnvOverrides) > 0 {
			eo = make(map[string]string, len(meta.EnvOverrides))
			for k, v := range meta.EnvOverrides {
				eo[k] = v
			}
		} else {
			eo, err = materialize(profileDir, spec, log, true)
			if err != nil {
				return nil, err
			}
		}
	}

	return &Environment{
		path:         profileDir,
		envOverrides: eo,
		spec:         spec,
		log:          log,
		name:         name,
		persistent:   true,
		registry:     reg,
	}, nil
}

// Attach connects to an existing persistent environment by name. Returns
// ErrEnvironmentNotFound if the name is not in the registry.
func Attach(ctx context.Context, name string, opts ...Option) (*Environment, error) {
	if err := ctx.Err(); err != nil {
		return nil, newErr(ErrProfileSetupFailed, "context cancelled", err)
	}
	cfg := optionsToConfig(opts)
	log := newEventLog(cfg.eventSink)
	root := cfg.registryRoot
	if root == "" {
		root = DefaultRegistryRoot()
	}
	reg := newRegistry(root)
	envDir, meta, found, err := reg.lookup(name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, newErr(ErrEnvironmentNotFound, "no env named "+name, nil)
	}
	profileDir := filepath.Join(envDir, "profile")
	spec := EnvironmentSpec{
		AdapterID:    meta.AdapterID,
		EnvOverrides: meta.EnvOverrides,
	}
	log.emit(EventEnvAttached, map[string]any{
		"name":       name,
		"adapter_id": meta.AdapterID,
		"path":       profileDir,
	})
	log.emit(EventRegistryRead, map[string]any{"path": filepath.Join(envDir, "metadata.json")})
	eo := make(map[string]string, len(meta.EnvOverrides))
	for k, v := range meta.EnvOverrides {
		eo[k] = v
	}
	return &Environment{
		path:         profileDir,
		envOverrides: eo,
		spec:         spec,
		log:          log,
		name:         name,
		persistent:   true,
		registry:     reg,
	}, nil
}

// CreateOrAttachFor builds a spec from the adapter and calls CreateOrAttach.
func CreateOrAttachFor(ctx context.Context, name string, a AgentAdapter, opts ...Option) (*Environment, error) {
	spec, err := a.BuildSpec(true)
	if err != nil {
		return nil, err
	}
	return CreateOrAttach(ctx, name, spec, opts...)
}

// List returns persistent environment names sorted.
func List(ctx context.Context, opts ...Option) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, newErr(ErrRegistryUnavailable, "context cancelled", err)
	}
	cfg := optionsToConfig(opts)
	root := cfg.registryRoot
	if root == "" {
		root = DefaultRegistryRoot()
	}
	return newRegistry(root).listNames()
}

// DestroyByName removes the named environment from disk and registry.
// Returns ErrEnvironmentNotFound if no such name; ErrCleanupFailed if the
// dir removal failed (the registry entry is still removed for consistency).
func DestroyByName(ctx context.Context, name string, opts ...Option) error {
	if err := ctx.Err(); err != nil {
		return newErr(ErrCleanupFailed, "context cancelled", err)
	}
	cfg := optionsToConfig(opts)
	root := cfg.registryRoot
	if root == "" {
		root = DefaultRegistryRoot()
	}
	ok, _, err := newRegistry(root).remove(name)
	if err != nil {
		return err
	}
	if !ok {
		return newErr(ErrCleanupFailed, "removing env dir", nil)
	}
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

