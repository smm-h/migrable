package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_ScaffoldsProject(t *testing.T) {
	dir := t.TempDir()

	if err := Init(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// migrable.toml should exist.
	configPath := filepath.Join(dir, "migrable.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("migrable.toml not created: %v", err)
	}
	if !strings.Contains(string(data), "migrations_dir") {
		t.Error("migrable.toml does not contain migrations_dir")
	}
	if !strings.Contains(string(data), "[files]") {
		t.Error("migrable.toml does not contain [files] section")
	}

	// migrations/ directory should exist.
	migrationsDir := filepath.Join(dir, "migrations")
	info, err := os.Stat(migrationsDir)
	if err != nil {
		t.Fatalf("migrations/ not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("migrations should be a directory")
	}

	// migrations/next/ directory should exist.
	nextDir := filepath.Join(migrationsDir, "next")
	info, err = os.Stat(nextDir)
	if err != nil {
		t.Fatalf("migrations/next/ not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("migrations/next should be a directory")
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	dir := t.TempDir()

	// Create migrable.toml first.
	os.WriteFile(filepath.Join(dir, "migrable.toml"), []byte("# existing\n"), 0o644)

	err := Init(dir)
	if err == nil {
		t.Fatal("expected error for already initialized project")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("error = %q, want it to contain 'already initialized'", err)
	}
}

func TestInit_IgnoresDotMigrableSubdir(t *testing.T) {
	dir := t.TempDir()

	// Create .migrable/migrable.toml -- Init should ignore it and proceed.
	dotDir := filepath.Join(dir, ".migrable")
	os.MkdirAll(dotDir, 0o755)
	os.WriteFile(filepath.Join(dotDir, "migrable.toml"), []byte("# existing\n"), 0o644)

	if err := Init(dir); err != nil {
		t.Fatalf("Init should succeed when only .migrable/migrable.toml exists: %v", err)
	}

	// migrable.toml should have been created at project root.
	configPath := filepath.Join(dir, "migrable.toml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("migrable.toml not created at project root: %v", err)
	}
}
