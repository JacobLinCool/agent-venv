//go:build !darwin

package agentvenv

import (
	"context"
	"os"
	"path/filepath"
)

func loadHostClaudeCredentials(_ context.Context) (string, error) {
	return readFile(filepath.Join(homeDir(), ".claude", ".credentials.json"))
}

func loadHostCodexAuth() (string, error) {
	return readFile(filepath.Join(homeDir(), ".codex", "auth.json"))
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func readFile(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
