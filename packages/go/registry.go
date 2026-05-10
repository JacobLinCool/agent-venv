package agentvenv

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/JacobLinCool/agent-venv/packages/go/internal/slug"
)

const registrySchemaVersion = 1

// DefaultRegistryRoot returns the path resolved per spec/registry.md:
// AGENT_VENV_REGISTRY_ROOT > $XDG_DATA_HOME/agent-venv/envs > ~/.local/share/agent-venv/envs.
func DefaultRegistryRoot() string {
	if v := os.Getenv("AGENT_VENV_REGISTRY_ROOT"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "agent-venv", "envs")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "agent-venv", "envs")
}

// metadata mirrors registry.md's metadata.json schema.
type metadata struct {
	SchemaVersion       int               `json:"schema_version"`
	Name                string            `json:"name"`
	AdapterID           string            `json:"adapter_id"`
	CreatedAt           string            `json:"created_at"`
	EnvOverrides        map[string]string `json:"env_overrides"`
	CredentialsLoaded   bool              `json:"credentials_loaded"`
	CredentialsLoadedAt string            `json:"credentials_loaded_at,omitempty"`
}

type indexFile struct {
	SchemaVersion int               `json:"schema_version"`
	Entries       map[string]string `json:"entries"`
}

type registry struct {
	root      string
	indexPath string
	envsDir   string
	lockPath  string
}

func newRegistry(root string) *registry {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &registry{
		root:      abs,
		indexPath: filepath.Join(abs, "index.json"),
		envsDir:   filepath.Join(abs, "envs"),
		lockPath:  filepath.Join(abs, ".lock"),
	}
}

// withLock executes fn while holding an exclusive lock on .lock. The lock
// file is created (with the registry root) if missing.
func (r *registry) withLock(fn func() error) error {
	if err := os.MkdirAll(r.root, 0o700); err != nil {
		return newErr(ErrRegistryUnavailable, "creating registry root", err)
	}
	f, err := os.OpenFile(r.lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return newErr(ErrRegistryUnavailable, "opening lock file", err)
	}
	defer f.Close()
	if err := acquireLock(f); err != nil {
		return err
	}
	defer releaseLock(f)
	return fn()
}

func (r *registry) readIndex() (map[string]string, error) {
	data, err := os.ReadFile(r.indexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, newErr(ErrRegistryUnavailable, "reading index.json", err)
	}
	var idx indexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, newErr(ErrRegistryUnavailable, "parsing index.json", err)
	}
	if idx.Entries == nil {
		idx.Entries = map[string]string{}
	}
	return idx.Entries, nil
}

func (r *registry) writeIndex(entries map[string]string) error {
	if err := os.MkdirAll(r.root, 0o700); err != nil {
		return newErr(ErrRegistryUnavailable, "creating registry root", err)
	}
	payload := indexFile{SchemaVersion: registrySchemaVersion, Entries: entries}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return newErr(ErrRegistryUnavailable, "marshalling index", err)
	}
	tmp := r.indexPath + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return newErr(ErrRegistryUnavailable, "writing index tmp", err)
	}
	if err := os.Rename(tmp, r.indexPath); err != nil {
		return newErr(ErrRegistryUnavailable, "renaming index", err)
	}
	return nil
}

func (r *registry) listNames() ([]string, error) {
	var names []string
	err := r.withLock(func() error {
		entries, err := r.readIndex()
		if err != nil {
			return err
		}
		names = make([]string, 0, len(entries))
		for k := range entries {
			names = append(names, k)
		}
		sort.Strings(names)
		return nil
	})
	return names, err
}

// reserveOrGet returns (envDir, metadata, created, err). created=true iff
// the entry did not exist before. If the name exists with a different
// adapter_id, returns ErrAdapterMismatch.
func (r *registry) reserveOrGet(name, adapterID string) (string, metadata, bool, error) {
	var envDir string
	var meta metadata
	var created bool
	err := r.withLock(func() error {
		entries, err := r.readIndex()
		if err != nil {
			return err
		}
		if rel, ok := entries[name]; ok {
			envDir = filepath.Join(r.root, rel)
			metaPath := filepath.Join(envDir, "metadata.json")
			data, err := os.ReadFile(metaPath)
			if err != nil {
				return newErr(ErrRegistryUnavailable, "reading metadata.json", err)
			}
			if err := json.Unmarshal(data, &meta); err != nil {
				return newErr(ErrRegistryUnavailable, "parsing metadata.json", err)
			}
			if meta.AdapterID != adapterID {
				return newErr(ErrAdapterMismatch,
					fmt.Sprintf("env %q created with adapter %q but %q requested",
						name, meta.AdapterID, adapterID), nil)
			}
			created = false
			return nil
		}
		s := slug.Of(name)
		existing := map[string]struct{}{}
		for _, p := range entries {
			existing[filepath.Base(p)] = struct{}{}
		}
		attempt := s
		for i := 1; ; i++ {
			if _, clash := existing[attempt]; !clash {
				break
			}
			attempt = fmt.Sprintf("%s-%d", s, i)
		}
		rel := filepath.Join("envs", attempt)
		envDir = filepath.Join(r.root, rel)
		if err := os.MkdirAll(filepath.Join(envDir, "profile"), 0o700); err != nil {
			return newErr(ErrRegistryUnavailable, "creating env dir", err)
		}
		meta = metadata{
			SchemaVersion: registrySchemaVersion,
			Name:          name,
			AdapterID:     adapterID,
			CreatedAt:     time.Now().UTC().Format(time.RFC3339),
			EnvOverrides:  map[string]string{},
		}
		if err := r.writeMetadata(envDir, meta); err != nil {
			return err
		}
		entries[name] = rel
		if err := r.writeIndex(entries); err != nil {
			return err
		}
		created = true
		return nil
	})
	return envDir, meta, created, err
}

func (r *registry) writeMetadata(envDir string, meta metadata) error {
	body, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return newErr(ErrRegistryUnavailable, "marshalling metadata", err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "metadata.json"), body, 0o644); err != nil {
		return newErr(ErrRegistryUnavailable, "writing metadata.json", err)
	}
	return nil
}

func (r *registry) lookup(name string) (string, metadata, bool, error) {
	var envDir string
	var meta metadata
	var found bool
	err := r.withLock(func() error {
		entries, err := r.readIndex()
		if err != nil {
			return err
		}
		rel, ok := entries[name]
		if !ok {
			return nil
		}
		envDir = filepath.Join(r.root, rel)
		data, err := os.ReadFile(filepath.Join(envDir, "metadata.json"))
		if err != nil {
			return newErr(ErrRegistryUnavailable, "reading metadata.json", err)
		}
		if err := json.Unmarshal(data, &meta); err != nil {
			return newErr(ErrRegistryUnavailable, "parsing metadata.json", err)
		}
		found = true
		return nil
	})
	return envDir, meta, found, err
}

// remove deletes the env dir and removes the index entry. Returns
// (ok, envDir, err). ok=false means the env dir removal failed but the
// index entry was still removed (consistent state); err carries the
// underlying os error.
func (r *registry) remove(name string) (bool, string, error) {
	var envDir string
	var rmErr error
	err := r.withLock(func() error {
		entries, err := r.readIndex()
		if err != nil {
			return err
		}
		rel, ok := entries[name]
		if !ok {
			return newErr(ErrEnvironmentNotFound, "no env named "+name, nil)
		}
		delete(entries, name)
		envDir = filepath.Join(r.root, rel)
		rmErr = os.RemoveAll(envDir)
		return r.writeIndex(entries)
	})
	if err != nil {
		return false, envDir, err
	}
	return rmErr == nil, envDir, rmErr
}
