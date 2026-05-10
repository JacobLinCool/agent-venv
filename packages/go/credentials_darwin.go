//go:build darwin

package agentvenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func loadHostClaudeCredentials(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "security",
		"find-generic-password",
		"-s", "Claude Code-credentials",
		"-a", os.Getenv("USER"),
		"-w",
	)
	out, err := cmd.Output()
	if err == nil {
		s := strings.TrimRight(string(out), "\n") + "\n"
		if s != "\n" {
			return s, nil
		}
	}
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
