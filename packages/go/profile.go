package agentvenv

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// materialize writes the spec into profileDir, returning the resolved
// env_overrides ($EPHEMERAL_HOME replaced with the absolute profile path).
//
// If skipSeedIfExists is true (used when reattaching to a pre-existing
// persistent env), seed_files and credentials are not rewritten — only
// env_overrides are recomputed.
func materialize(profileDir string, spec EnvironmentSpec, log *eventLog, skipSeedIfExists bool) (map[string]string, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(profileDir)
	if err != nil {
		return nil, newErr(ErrProfileSetupFailed, "resolving profile dir", err)
	}
	profileDir = abs
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, newErr(ErrProfileSetupFailed, "mkdir profile dir", err)
	}

	if !skipSeedIfExists && len(spec.SeedFiles) > 0 {
		count, total, err := writeFiles(profileDir, spec.SeedFiles, spec.FileModes, 0)
		if err != nil {
			return nil, err
		}
		log.emit(EventProfileMaterialized, map[string]any{
			"file_count":  count,
			"total_bytes": total,
		})
	}

	if !skipSeedIfExists && len(spec.Credentials) > 0 {
		count, _, err := writeFiles(profileDir, spec.Credentials, spec.FileModes, 0o600)
		if err != nil {
			return nil, err
		}
		log.emit(EventCredentialsCopied, map[string]any{"file_count": count})
	}

	eo := make(map[string]string, len(spec.EnvOverrides))
	for k, v := range spec.EnvOverrides {
		eo[k] = strings.ReplaceAll(v, "$EPHEMERAL_HOME", profileDir)
	}
	return eo, nil
}

func writeFiles(base string, files map[string]string, modes map[string]fs.FileMode, defaultMode fs.FileMode) (int, int, error) {
	count, total := 0, 0
	for rel, content := range files {
		if err := validateRelPath(rel); err != nil {
			return 0, 0, err
		}
		target := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return 0, 0, newErr(ErrProfileSetupFailed, "mkdir for "+rel, err)
		}
		b := []byte(content)
		if err := os.WriteFile(target, b, 0o644); err != nil {
			return 0, 0, newErr(ErrProfileSetupFailed, "writing "+rel, err)
		}
		if runtime.GOOS != "windows" {
			mode, ok := modes[rel]
			if !ok {
				mode = defaultMode
			}
			if mode != 0 {
				if err := os.Chmod(target, mode); err != nil {
					return 0, 0, newErr(ErrProfileSetupFailed, "chmod "+rel, err)
				}
			}
		}
		count++
		total += len(b)
	}
	return count, total, nil
}

func removeDir(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(path)
}
