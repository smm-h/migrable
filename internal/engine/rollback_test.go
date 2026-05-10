package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/internal/config"
)

func TestRollback_Basic(t *testing.T) {
	// Apply a migration that adds a field, then roll it back.
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add debug field"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = { op = "remove_field", path = "debug" }
`,
		},
	)

	// First, apply the migration.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}

	// Verify field was added.
	data, _ := os.ReadFile(targetPath)
	doc, _ := tomledit.Parse(data)
	if _, ok := doc.GetBool("debug"); !ok {
		t.Fatal("debug field should exist after migration")
	}

	// Now roll back.
	result, err := Rollback(cfg, false)
	if err != nil {
		t.Fatalf("unexpected rollback error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}
	if result.FromVersion.String() != "1.0.0" {
		t.Errorf("FromVersion = %s, want 1.0.0", result.FromVersion)
	}
	if result.ToVersion.String() != "0.0.0" {
		t.Errorf("ToVersion = %s, want 0.0.0", result.ToVersion)
	}

	// Verify field was removed.
	data, _ = os.ReadFile(targetPath)
	doc, _ = tomledit.Parse(data)
	if _, ok := doc.GetBool("debug"); ok {
		t.Error("debug field should be removed after rollback")
	}

	// Verify version is 0.0.0.
	verStr, _ := doc.GetString("_schema_version")
	if verStr != "0.0.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "0.0.0")
	}
}

func TestRollback_ToPreviousVersion(t *testing.T) {
	// Apply two migrations, roll back only the last one.
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add debug field"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = { op = "remove_field", path = "debug" }
`,
			"1.1.0.toml": `description = "Add log_level field"

[[structure]]
op = "add_field"
path = "log_level"
type = "string"
default = "info"
down = { op = "remove_field", path = "log_level" }
`,
		},
	)

	// Apply both migrations.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// Roll back the last migration.
	result, err := Rollback(cfg, false)
	if err != nil {
		t.Fatalf("unexpected rollback error: %v", err)
	}

	if result.FromVersion.String() != "1.1.0" {
		t.Errorf("FromVersion = %s, want 1.1.0", result.FromVersion)
	}
	if result.ToVersion.String() != "1.0.0" {
		t.Errorf("ToVersion = %s, want 1.0.0", result.ToVersion)
	}

	// Verify log_level is removed but debug remains.
	data, _ := os.ReadFile(targetPath)
	doc, _ := tomledit.Parse(data)

	if _, ok := doc.GetString("log_level"); ok {
		t.Error("log_level should be removed after rollback")
	}
	if _, ok := doc.GetBool("debug"); !ok {
		t.Error("debug should still exist after rolling back only 1.1.0")
	}

	verStr, _ := doc.GetString("_schema_version")
	if verStr != "1.0.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "1.0.0")
	}
}

func TestRollback_IrreversibleBlock(t *testing.T) {
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add then drop"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = "irreversible"
`,
		},
	)

	// Apply the migration.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}

	// Save the file state before rollback attempt.
	beforeData, _ := os.ReadFile(targetPath)

	// Attempt rollback.
	_, err = Rollback(cfg, false)
	if err == nil {
		t.Fatal("expected rollback to be blocked, got nil error")
	}

	var blocked *RollbackBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected RollbackBlockedError, got %T: %v", err, err)
	}
	if len(blocked.IrreversibleOps) != 1 {
		t.Errorf("IrreversibleOps count = %d, want 1", len(blocked.IrreversibleOps))
	}

	// Verify the file was NOT modified.
	afterData, _ := os.ReadFile(targetPath)
	if string(afterData) != string(beforeData) {
		t.Error("file should not be modified when rollback is blocked")
	}
}

func TestRollback_ArrayDownOps(t *testing.T) {
	// Migration adds two fields; the second has an array-valued down that removes both.
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add UI section"

[[structure]]
op = "add_field"
path = "ui_theme"
type = "string"
default = "dark"

[[structure]]
op = "add_field"
path = "ui_font_size"
type = "integer"
default = 14
down = [
  { op = "remove_field", path = "ui_font_size" },
  { op = "remove_field", path = "ui_theme" },
]
`,
		},
	)

	// Apply.
	migResult, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}
	if migResult.Applied != 1 {
		t.Fatalf("Migrate Applied = %d, want 1", migResult.Applied)
	}

	// Roll back.
	result, err := Rollback(cfg, false)
	if err != nil {
		t.Fatalf("unexpected rollback error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}

	// Both fields should be removed (array down ops executed in reverse).
	data, _ := os.ReadFile(targetPath)
	doc, _ := tomledit.Parse(data)

	if _, ok := doc.GetString("ui_theme"); ok {
		t.Error("ui_theme should be removed after rollback")
	}
	if _, ok := doc.GetInt("ui_font_size"); ok {
		t.Error("ui_font_size should be removed after rollback")
	}
}

func TestRollback_AtZeroVersion(t *testing.T) {
	cfg, _ := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "test"

[[structure]]
op = "add_field"
path = "x"
type = "string"
default = "y"
down = { op = "remove_field", path = "x" }
`,
		},
	)

	// No migrations applied, version is 0.0.0.
	_, err := Rollback(cfg, false)
	if err == nil {
		t.Fatal("expected error for rollback at 0.0.0, got nil")
	}
	if err.Error() != "nothing to roll back: _schema_version is 0.0.0" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRollback_MissingMigrationFile(t *testing.T) {
	// Set _schema_version to a version with no corresponding migration file.
	cfg, _ := setupProject(t,
		`_schema_version = "2.0.0"
title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "test"

[[structure]]
op = "add_field"
path = "x"
type = "string"
default = "y"
down = { op = "remove_field", path = "x" }
`,
		},
	)

	_, err := Rollback(cfg, false)
	if err == nil {
		t.Fatal("expected error for missing migration file, got nil")
	}
	if err.Error() != "migration file for current version 2.0.0 not found" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRollback_DryRun(t *testing.T) {
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add debug field"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = { op = "remove_field", path = "debug" }
`,
		},
	)

	// Apply the migration.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}

	// Save file state after migration.
	afterMigration, _ := os.ReadFile(targetPath)

	// Dry-run rollback.
	result, err := Rollback(cfg, true)
	if err != nil {
		t.Fatalf("unexpected rollback error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}

	// File should NOT have been modified.
	afterDryRun, _ := os.ReadFile(targetPath)
	if string(afterDryRun) != string(afterMigration) {
		t.Error("dry-run rollback modified the target file")
	}

	// FileChanges should have entries.
	changes, ok := result.FileChanges["config"]
	if !ok {
		t.Fatal("expected FileChanges for 'config'")
	}
	if len(changes) == 0 {
		t.Fatal("expected non-empty changes for dry-run rollback")
	}

	// Should include removal of debug field and version change.
	var hasDebugRemoved, hasVersionChanged bool
	for _, c := range changes {
		if c.Path == "debug" && c.Kind == tomledit.Removed {
			hasDebugRemoved = true
		}
		if c.Path == "_schema_version" && c.Kind == tomledit.Modified {
			hasVersionChanged = true
		}
	}
	if !hasDebugRemoved {
		t.Error("expected debug removal in dry-run changes")
	}
	if !hasVersionChanged {
		t.Error("expected _schema_version change in dry-run changes")
	}
}

func TestRollback_FailedDownOp(t *testing.T) {
	// Use a down op (transform) that will fail because the path doesn't exist.
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add debug field"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
down = { op = "transform", path = "nonexistent_path", expr = "value + 1" }
`,
		},
	)

	// Apply the migration.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}

	// Save file state after migration.
	afterMigration, _ := os.ReadFile(targetPath)

	// Attempt rollback -- the down op should fail because the path doesn't exist.
	_, err = Rollback(cfg, false)
	if err == nil {
		t.Fatal("expected rollback to fail, got nil error")
	}

	// File should NOT be modified (backup restored, then atomic write skipped).
	afterRollback, _ := os.ReadFile(targetPath)
	if string(afterRollback) != string(afterMigration) {
		t.Error("failed rollback should not modify the target file")
	}
}

func TestRollback_ReverseExecutionOrder(t *testing.T) {
	// Create a migration where structure adds a field, and data changes its value.
	// Rollback should reverse data first (restore original value), then structure
	// (remove the field). This verifies data down ops run before structure down ops.
	// If order were wrong (structure first), the field would be removed before the
	// data down op could set it back, and the data down op would create a new key
	// instead of cleaning up properly.
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add enabled flag and set it"

[[structure]]
op = "add_field"
path = "enabled"
type = "bool"
default = false
down = { op = "remove_field", path = "enabled" }

[[data]]
op = "set_value"
path = "enabled"
value = true
down = { op = "set_value", path = "enabled", value = false }
`,
		},
	)

	// Apply.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}

	// Verify state after migration.
	data, _ := os.ReadFile(targetPath)
	doc, _ := tomledit.Parse(data)
	val, ok := doc.GetBool("enabled")
	if !ok {
		t.Fatal("enabled should exist after migration")
	}
	if val != true {
		t.Errorf("enabled = %v, want true", val)
	}

	// Roll back. Data down ops run first (set_value back to false),
	// then structure down ops (remove_field enabled).
	result, err := Rollback(cfg, false)
	if err != nil {
		t.Fatalf("unexpected rollback error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}

	// The enabled field should be completely removed.
	data, _ = os.ReadFile(targetPath)
	doc, _ = tomledit.Parse(data)
	if _, ok := doc.GetBool("enabled"); ok {
		t.Error("enabled should not exist after rollback")
	}

	verStr, _ := doc.GetString("_schema_version")
	if verStr != "0.0.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "0.0.0")
	}
}

func TestRollback_MultiFileRequiresVersionFile(t *testing.T) {
	dir := t.TempDir()
	migrationsDir := filepath.Join(dir, "migrations")
	os.Mkdir(migrationsDir, 0o755)

	// Create the target files so the file-loading step succeeds.
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("title = \"test\"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "users.toml"), []byte(""), 0o644)

	cfg := &config.Config{
		MigrationsDir: "migrations",
		Files: map[string]string{
			"config": "config.toml",
			"users":  "users.toml",
		},
		BaseDir: dir,
	}

	_, err := Rollback(cfg, false)
	if err == nil {
		t.Fatal("expected error for multi-file project without version_file, got nil")
	}
	if !strings.Contains(err.Error(), "version_file") {
		t.Errorf("error = %q, want it to mention version_file", err.Error())
	}
}

func TestRollback_IrreversibleMultipleOps(t *testing.T) {
	// Multiple ops marked irreversible should all be listed.
	cfg, _ := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Multiple irreversible"

[[structure]]
op = "add_field"
path = "a"
type = "string"
default = "x"
down = "irreversible"

[[data]]
op = "set_value"
path = "title"
value = "New Title"
down = "irreversible"
`,
		},
	)

	// Apply the migration.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}

	// Attempt rollback.
	_, err = Rollback(cfg, false)
	if err == nil {
		t.Fatal("expected rollback to be blocked, got nil error")
	}

	var blocked *RollbackBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected RollbackBlockedError, got %T: %v", err, err)
	}
	if len(blocked.IrreversibleOps) != 2 {
		t.Errorf("IrreversibleOps count = %d, want 2", len(blocked.IrreversibleOps))
	}
}

func TestRollback_OpsWithoutDown(t *testing.T) {
	// Ops without down fields should be skipped during rollback.
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Mixed ops"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false

[[structure]]
op = "add_field"
path = "verbose"
type = "bool"
default = false
down = { op = "remove_field", path = "verbose" }
`,
		},
	)

	// Apply.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}

	// Roll back.
	result, err := Rollback(cfg, false)
	if err != nil {
		t.Fatalf("unexpected rollback error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}

	// verbose should be removed (has down op), debug should remain (no down op).
	data, _ := os.ReadFile(targetPath)
	doc, _ := tomledit.Parse(data)

	if _, ok := doc.GetBool("verbose"); ok {
		t.Error("verbose should be removed after rollback")
	}
	if _, ok := doc.GetBool("debug"); !ok {
		t.Error("debug should remain after rollback (no down op)")
	}
}
