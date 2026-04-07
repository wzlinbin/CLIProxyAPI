package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDefaultConfigPathPrefersWorkingDirectory(t *testing.T) {
	originalExecutablePathFunc := executablePathFunc
	executablePathFunc = func() (string, error) {
		return filepath.Join(t.TempDir(), "cli-proxy-api.exe"), nil
	}
	defer func() {
		executablePathFunc = originalExecutablePathFunc
	}()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	workingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workingDir, "config.yaml"), []byte("port: 8317\n"), 0o644); err != nil {
		t.Fatalf("failed to write working directory config: %v", err)
	}

	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("failed to switch working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}()

	got := resolveDefaultConfigPath()
	want := filepath.Join(workingDir, "config.yaml")
	if got != want {
		t.Fatalf("resolveDefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestResolveDefaultConfigPathFallsBackToExecutableDirectory(t *testing.T) {
	executableDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(executableDir, "config.yaml"), []byte("port: 8317\n"), 0o644); err != nil {
		t.Fatalf("failed to write executable directory config: %v", err)
	}

	originalExecutablePathFunc := executablePathFunc
	executablePathFunc = func() (string, error) {
		return filepath.Join(executableDir, "cli-proxy-api.exe"), nil
	}
	defer func() {
		executablePathFunc = originalExecutablePathFunc
	}()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	workingDir := t.TempDir()
	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("failed to switch working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}()

	got := resolveDefaultConfigPath()
	want := filepath.Join(executableDir, "config.yaml")
	if got != want {
		t.Fatalf("resolveDefaultConfigPath() = %q, want %q", got, want)
	}
}
