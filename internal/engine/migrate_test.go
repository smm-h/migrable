package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/internal/config"
)

// setupProject creates a temporary project directory with migrable.toml,
// a migrations dir, and optionally a target config file.
func setupProject(t *testing.T, targetContent string, migrations map[string]string) (*config.Config, string) {
	t.Helper()
	dir := t.TempDir()

	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.Mkdir(migrationsDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	targetPath := filepath.Join(dir, "config.toml")
	if targetContent != "" {
		if err := os.WriteFile(targetPath, []byte(targetContent), 0o644); err != nil {
			t.Fatalf("failed to write target file: %v", err)
		}
	}

	for name, content := range migrations {
		migPath := filepath.Join(migrationsDir, name)
		if err := os.WriteFile(migPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write migration %s: %v", name, err)
		}
	}

	cfg := &config.Config{
		MigrationsDir: "migrations",
		Files:         map[string]string{"config": "config.toml"},
		BaseDir:       dir,
	}

	return cfg, targetPath
}

func TestMigrate_SingleMigration(t *testing.T) {
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
`,
		},
	)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}
	if result.FromVersion.String() != "0.0.0" {
		t.Errorf("FromVersion = %s, want 0.0.0", result.FromVersion)
	}
	if result.ToVersion.String() != "1.0.0" {
		t.Errorf("ToVersion = %s, want 1.0.0", result.ToVersion)
	}

	// Verify the file was written.
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}

	doc, err := tomledit.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse target file: %v", err)
	}

	// Check _schema_version was set.
	verStr, ok := doc.GetString("_schema_version")
	if !ok {
		t.Fatal("_schema_version not found in target file")
	}
	if verStr != "1.0.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "1.0.0")
	}

	// Check the debug field was added.
	debugVal, ok := doc.GetBool("debug")
	if !ok {
		t.Fatal("debug field not found in target file")
	}
	if debugVal != false {
		t.Errorf("debug = %v, want false", debugVal)
	}
}

func TestMigrate_MultipleMigrationsInOrder(t *testing.T) {
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add debug"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`,
			"1.1.0.toml": `description = "Add log_level"

[[structure]]
op = "add_field"
path = "log_level"
type = "string"
default = "info"
`,
			"2.0.0.toml": `description = "Set value"

[[data]]
op = "set_value"
path = "debug"
value = true
`,
		},
	)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 3 {
		t.Errorf("Applied = %d, want 3", result.Applied)
	}
	if result.ToVersion.String() != "2.0.0" {
		t.Errorf("ToVersion = %s, want 2.0.0", result.ToVersion)
	}

	// Verify final state.
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}
	doc, err := tomledit.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse target file: %v", err)
	}

	verStr, _ := doc.GetString("_schema_version")
	if verStr != "2.0.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "2.0.0")
	}

	debugVal, ok := doc.GetBool("debug")
	if !ok {
		t.Fatal("debug field not found")
	}
	if debugVal != true {
		t.Errorf("debug = %v, want true", debugVal)
	}

	logLevel, ok := doc.GetString("log_level")
	if !ok {
		t.Fatal("log_level field not found")
	}
	if logLevel != "info" {
		t.Errorf("log_level = %q, want %q", logLevel, "info")
	}
}

func TestMigrate_SkipsAlreadyApplied(t *testing.T) {
	cfg, _ := setupProject(t,
		`_schema_version = "1.0.0"
title = "My App"
debug = false
`,
		map[string]string{
			"1.0.0.toml": `description = "Add debug"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`,
			"1.1.0.toml": `description = "Add log_level"

[[structure]]
op = "add_field"
path = "log_level"
type = "string"
default = "info"
`,
		},
	)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1 (only 1.1.0 should be applied)", result.Applied)
	}
	if result.FromVersion.String() != "1.0.0" {
		t.Errorf("FromVersion = %s, want 1.0.0", result.FromVersion)
	}
	if result.ToVersion.String() != "1.1.0" {
		t.Errorf("ToVersion = %s, want 1.1.0", result.ToVersion)
	}
}

func TestMigrate_DryRunDoesNotWrite(t *testing.T) {
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add debug"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`,
		},
	)

	// Read original content.
	originalData, _ := os.ReadFile(targetPath)

	result, err := Migrate(cfg, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}

	// File should NOT have been modified.
	afterData, _ := os.ReadFile(targetPath)
	if string(afterData) != string(originalData) {
		t.Error("dry-run modified the target file")
	}

	// FileChanges should have entries.
	changes, ok := result.FileChanges["config"]
	if !ok {
		t.Fatal("expected FileChanges for 'config'")
	}
	if len(changes) == 0 {
		t.Fatal("expected non-empty changes for dry-run")
	}

	// Should include the _schema_version addition and the debug field.
	var hasVersion, hasDebug bool
	for _, c := range changes {
		if c.Path == "_schema_version" && c.Kind == tomledit.Added {
			hasVersion = true
		}
		if c.Path == "debug" && c.Kind == tomledit.Added {
			hasDebug = true
		}
	}
	if !hasVersion {
		t.Error("expected _schema_version in dry-run changes")
	}
	if !hasDebug {
		t.Error("expected debug in dry-run changes")
	}
}

func TestMigrate_InvalidTargetTOML(t *testing.T) {
	cfg, _ := setupProject(t,
		`this is not valid toml [[[
`,
		map[string]string{
			"1.0.0.toml": `description = "test"

[[structure]]
op = "add_field"
path = "x"
type = "string"
default = "y"
`,
		},
	)

	_, err := Migrate(cfg, false)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
	if !strings.Contains(err.Error(), "invalid TOML") {
		t.Errorf("error = %q, want it to contain 'invalid TOML'", err.Error())
	}
}

func TestMigrate_FailedOpNoPartialWrite(t *testing.T) {
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "This will fail"

[[data]]
op = "remove_where"
path = "nonexistent_array"
match_mode = "all"
`,
		},
	)

	originalData, _ := os.ReadFile(targetPath)

	_, err := Migrate(cfg, false)
	if err == nil {
		t.Fatal("expected error for failed op, got nil")
	}

	// The file should NOT have been modified.
	afterData, _ := os.ReadFile(targetPath)
	if string(afterData) != string(originalData) {
		t.Error("failed migration modified the target file")
	}
}

func TestMigrate_FreshInstall(t *testing.T) {
	// Target file doesn't exist -- migrations applied from scratch.
	cfg, targetPath := setupProject(t,
		"", // empty string means don't create the file
		map[string]string{
			"1.0.0.toml": `description = "Initial schema"

[[structure]]
op = "add_field"
path = "title"
type = "string"
default = "Untitled"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`,
		},
	)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}
	if result.FromVersion.String() != "0.0.0" {
		t.Errorf("FromVersion = %s, want 0.0.0", result.FromVersion)
	}
	if result.ToVersion.String() != "1.0.0" {
		t.Errorf("ToVersion = %s, want 1.0.0", result.ToVersion)
	}

	// The file should now exist.
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("target file should exist after migration: %v", err)
	}

	doc, err := tomledit.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse target file: %v", err)
	}

	verStr, _ := doc.GetString("_schema_version")
	if verStr != "1.0.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "1.0.0")
	}

	title, ok := doc.GetString("title")
	if !ok {
		t.Fatal("title field not found")
	}
	if title != "Untitled" {
		t.Errorf("title = %q, want %q", title, "Untitled")
	}
}

func TestMigrate_AlreadyUpToDate(t *testing.T) {
	cfg, _ := setupProject(t,
		`_schema_version = "2.0.0"
title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Old"

[[structure]]
op = "add_field"
path = "x"
type = "string"
default = "y"
`,
			"2.0.0.toml": `description = "Current"

[[structure]]
op = "add_field"
path = "z"
type = "string"
default = "w"
`,
		},
	)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 0 {
		t.Errorf("Applied = %d, want 0", result.Applied)
	}
	if result.FromVersion.String() != "2.0.0" {
		t.Errorf("FromVersion = %s, want 2.0.0", result.FromVersion)
	}
	if result.ToVersion.String() != "2.0.0" {
		t.Errorf("ToVersion = %s, want 2.0.0", result.ToVersion)
	}
}

func TestMigrate_VersionTracking(t *testing.T) {
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "v1"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`,
			"1.1.0.toml": `description = "v1.1"

[[structure]]
op = "add_field"
path = "verbose"
type = "bool"
default = false
`,
		},
	)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 2 {
		t.Errorf("Applied = %d, want 2", result.Applied)
	}

	// Verify final _schema_version.
	data, _ := os.ReadFile(targetPath)
	doc, _ := tomledit.Parse(data)
	verStr, _ := doc.GetString("_schema_version")
	if verStr != "1.1.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "1.1.0")
	}

	// Run again -- should be no-op.
	result2, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error on second run: %v", err)
	}
	if result2.Applied != 0 {
		t.Errorf("second run Applied = %d, want 0", result2.Applied)
	}
	if result2.FromVersion.String() != "1.1.0" {
		t.Errorf("second run FromVersion = %s, want 1.1.0", result2.FromVersion)
	}
}

func TestMigrate_StructureAndDataOps(t *testing.T) {
	cfg, targetPath := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Structure and data ops"

[[structure]]
op = "add_field"
path = "log_level"
type = "string"
default = "warn"

[[data]]
op = "set_value"
path = "title"
value = "Updated App"
`,
		},
	)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}

	data, _ := os.ReadFile(targetPath)
	doc, _ := tomledit.Parse(data)

	logLevel, ok := doc.GetString("log_level")
	if !ok {
		t.Fatal("log_level not found")
	}
	if logLevel != "warn" {
		t.Errorf("log_level = %q, want %q", logLevel, "warn")
	}

	title, ok := doc.GetString("title")
	if !ok {
		t.Fatal("title not found")
	}
	if title != "Updated App" {
		t.Errorf("title = %q, want %q", title, "Updated App")
	}
}

func TestMigrate_MultiFileRequiresVersionFile(t *testing.T) {
	dir := t.TempDir()
	migrationsDir := filepath.Join(dir, "migrations")
	os.Mkdir(migrationsDir, 0o755)

	cfg := &config.Config{
		MigrationsDir: "migrations",
		Files: map[string]string{
			"config": "config.toml",
			"users":  "users.toml",
		},
		BaseDir: dir,
	}

	_, err := Migrate(cfg, false)
	if err == nil {
		t.Fatal("expected error for multi-file project without version_file, got nil")
	}
	if !strings.Contains(err.Error(), "version_file") {
		t.Errorf("error = %q, want it to mention version_file", err.Error())
	}
}

func TestMigrate_Descriptions(t *testing.T) {
	cfg, _ := setupProject(t,
		`title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "Add debug mode"

[[structure]]
op = "add_field"
path = "debug"
type = "bool"
default = false
`,
		},
	)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	desc, ok := result.Descriptions["1.0.0"]
	if !ok {
		t.Fatal("description not found for 1.0.0")
	}
	if desc != "Add debug mode" {
		t.Errorf("description = %q, want %q", desc, "Add debug mode")
	}
}

func TestMigrate_InvalidSchemaVersion(t *testing.T) {
	cfg, _ := setupProject(t,
		`_schema_version = "not-a-version"
title = "My App"
`,
		map[string]string{
			"1.0.0.toml": `description = "test"

[[structure]]
op = "add_field"
path = "x"
type = "string"
default = "y"
`,
		},
	)

	_, err := Migrate(cfg, false)
	if err == nil {
		t.Fatal("expected error for invalid _schema_version, got nil")
	}
	if !strings.Contains(err.Error(), "invalid _schema_version") {
		t.Errorf("error = %q, want it to contain 'invalid _schema_version'", err.Error())
	}
}

// Verify that semver is imported correctly by using it in a non-trivial way.
var _ = semver.MustParse("0.0.0")
