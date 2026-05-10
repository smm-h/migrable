package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_ValidMigration(t *testing.T) {
	dir := t.TempDir()
	migration := `description = "add theme field"

[[structure]]
op = "add_field"
path = "ui.theme"
type = "string"
default = "dark"
down = {op = "remove_field", path = "ui.theme"}
`
	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte(migration), 0o644)

	result, err := Validate(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", result.FileCount)
	}
}

func TestValidate_MissingDownOp(t *testing.T) {
	dir := t.TempDir()
	migration := `description = "missing down"

[[structure]]
op = "add_field"
path = "ui.theme"
type = "string"
default = "dark"
`
	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte(migration), 0o644)

	result, err := Validate(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(result.Errors), result.Errors)
	}
	if !strings.Contains(result.Errors[0].Message, "missing down op") {
		t.Errorf("error message = %q, want it to contain 'missing down op'", result.Errors[0].Message)
	}
}

func TestValidate_TypeMismatch(t *testing.T) {
	dir := t.TempDir()
	// Declare type as "string" but provide integer default.
	migration := `description = "type mismatch"

[[structure]]
op = "add_field"
path = "ui.count"
type = "string"
default = 42
down = {op = "remove_field", path = "ui.count"}
`
	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte(migration), 0o644)

	result, err := Validate(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(result.Errors), result.Errors)
	}
	if !strings.Contains(result.Errors[0].Message, "expected string") {
		t.Errorf("error message = %q, want it to contain 'expected string'", result.Errors[0].Message)
	}
}

func TestValidate_MissingDescription(t *testing.T) {
	dir := t.TempDir()
	migration := `[[structure]]
op = "add_field"
path = "ui.theme"
type = "string"
default = "dark"
down = {op = "remove_field", path = "ui.theme"}
`
	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte(migration), 0o644)

	result, err := Validate(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
	}
	if !strings.Contains(result.Warnings[0].Message, "missing description") {
		t.Errorf("warning message = %q, want 'missing description'", result.Warnings[0].Message)
	}
}

func TestValidate_MultipleFilesCollectsAll(t *testing.T) {
	dir := t.TempDir()

	// File 1: missing down op
	f1 := `description = "first"
[[structure]]
op = "add_field"
path = "a"
type = "string"
default = "x"
`
	// File 2: valid
	f2 := `description = "second"
[[structure]]
op = "add_field"
path = "b"
type = "string"
default = "y"
down = {op = "remove_field", path = "b"}
`
	// File 3: type mismatch
	f3 := `description = "third"
[[structure]]
op = "add_field"
path = "c"
type = "boolean"
default = "not a bool"
down = {op = "remove_field", path = "c"}
`

	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte(f1), 0o644)
	os.WriteFile(filepath.Join(dir, "1.1.0.toml"), []byte(f2), 0o644)
	os.WriteFile(filepath.Join(dir, "1.2.0.toml"), []byte(f3), 0o644)

	result, err := Validate(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", result.FileCount)
	}
	// File 1: missing down op, File 3: type mismatch
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestValidate_StagingFilesInNext(t *testing.T) {
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	staging := `description = "staging"
[[structure]]
op = "add_field"
path = "x"
type = "string"
`
	os.WriteFile(filepath.Join(nextDir, "add-x.toml"), []byte(staging), 0o644)

	result, err := Validate(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", result.FileCount)
	}
	// Missing down op
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(result.Errors), result.Errors)
	}
	if !strings.Contains(result.Errors[0].File, "next") {
		t.Errorf("error file = %q, want it to reference next/", result.Errors[0].File)
	}
}

func TestValidate_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	result, err := Validate(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(result.Warnings))
	}
	if result.FileCount != 0 {
		t.Errorf("FileCount = %d, want 0", result.FileCount)
	}
}

func TestValidate_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte("this is not { valid toml"), 0o644)

	result, err := Validate(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(result.Errors), result.Errors)
	}
	if !strings.Contains(result.Errors[0].Message, "parse error") {
		t.Errorf("error message = %q, want it to contain 'parse error'", result.Errors[0].Message)
	}
}
