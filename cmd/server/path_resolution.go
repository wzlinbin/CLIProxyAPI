package main

import (
	"os"
	"path/filepath"
)

var executablePathFunc = os.Executable

func resolveDefaultConfigPath() string {
	workingDir, err := os.Getwd()
	if err == nil {
		if existing := existingConfigPath(workingDir); existing != "" {
			return existing
		}
	}

	executableDir := resolveExecutableDir()
	if executableDir != "" {
		if existing := existingConfigPath(executableDir); existing != "" {
			return existing
		}
	}

	if workingDir != "" {
		return filepath.Join(workingDir, "config.yaml")
	}
	if executableDir != "" {
		return filepath.Join(executableDir, "config.yaml")
	}
	return "config.yaml"
}

func existingConfigPath(baseDir string) string {
	if baseDir == "" {
		return ""
	}

	candidate := filepath.Join(baseDir, "config.yaml")
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return ""
	}
	return candidate
}

func resolveExecutableDir() string {
	executablePath, err := executablePathFunc()
	if err != nil || executablePath == "" {
		return ""
	}

	if resolvedPath, errEval := filepath.EvalSymlinks(executablePath); errEval == nil && resolvedPath != "" {
		executablePath = resolvedPath
	}

	return filepath.Dir(executablePath)
}
