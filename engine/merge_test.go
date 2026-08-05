package engine

import (
	"github.com/smm-h/stricttest/go/hygiene"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smm-h/migrable/ops"
)

func TestMerge_TwoStagingFiles(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	file1 := `description = "add theme"

[[structure]]
op = "add_field"
path = "ui.theme"
type = "string"
default = "dark"
down = {op = "remove_field", path = "ui.theme"}
`
	file2 := `description = "add timeout"

[[data]]
op = "set_value"
path = "server.timeout"
value = 30
down = {op = "remove_field", path = "server.timeout"}
`
	os.WriteFile(filepath.Join(nextDir, "01-add-theme.toml"), []byte(file1), 0o644)
	os.WriteFile(filepath.Join(nextDir, "02-add-timeout.toml"), []byte(file2), 0o644)

	outPath, err := Merge(dir, "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outPath != filepath.Join(dir, "1.0.0.toml") {
		t.Errorf("outPath = %q, want %q", outPath, filepath.Join(dir, "1.0.0.toml"))
	}

	// Verify the merged file is valid and parseable.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	migration, err := ops.ParseMigration(data)
	if err != nil {
		t.Fatalf("failed to parse merged migration: %v", err)
	}

	if migration.Description != "add theme; add timeout" {
		t.Errorf("description = %q, want %q", migration.Description, "add theme; add timeout")
	}
	if len(migration.Structure) != 1 {
		t.Errorf("structure ops = %d, want 1", len(migration.Structure))
	}
	if len(migration.Data) != 1 {
		t.Errorf("data ops = %d, want 1", len(migration.Data))
	}
}

func TestMerge_EmptyNext(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	outPath, err := Merge(dir, "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outPath != "" {
		t.Errorf("outPath = %q, want empty string", outPath)
	}
}

func TestMerge_NextNotExist(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()

	_, err := Merge(dir, "1.0.0")
	if err == nil {
		t.Fatal("expected error when next/ does not exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want it to contain 'does not exist'", err)
	}
}

func TestMerge_VersionCollision(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	// Create existing migration file.
	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte("description = \"existing\"\n"), 0o644)

	// Create a staging file.
	os.WriteFile(filepath.Join(nextDir, "a.toml"), []byte("description = \"staging\"\n"), 0o644)

	_, err := Merge(dir, "1.0.0")
	if err == nil {
		t.Fatal("expected error for version collision")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to contain 'already exists'", err)
	}
}

func TestMerge_InvalidStagingFile(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	os.WriteFile(filepath.Join(nextDir, "bad.toml"), []byte("not valid { toml"), 0o644)

	_, err := Merge(dir, "1.0.0")
	if err == nil {
		t.Fatal("expected error for invalid staging file")
	}
}

func TestMerge_AlphabeticalOrder(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	fileB := `description = "second"
[[structure]]
op = "add_field"
path = "b"
type = "string"
default = "bval"
down = {op = "remove_field", path = "b"}
`
	fileA := `description = "first"
[[structure]]
op = "add_field"
path = "a"
type = "string"
default = "aval"
down = {op = "remove_field", path = "a"}
`
	os.WriteFile(filepath.Join(nextDir, "b-second.toml"), []byte(fileB), 0o644)
	os.WriteFile(filepath.Join(nextDir, "a-first.toml"), []byte(fileA), 0o644)

	outPath, err := Merge(dir, "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	migration, err := ops.ParseMigration(data)
	if err != nil {
		t.Fatalf("failed to parse merged: %v", err)
	}

	// a-first.toml should come before b-second.toml alphabetically.
	if migration.Description != "first; second" {
		t.Errorf("description = %q, want %q", migration.Description, "first; second")
	}
	if len(migration.Structure) != 2 {
		t.Fatalf("structure ops = %d, want 2", len(migration.Structure))
	}
	if migration.Structure[0].Path != "a" {
		t.Errorf("structure[0].Path = %q, want %q", migration.Structure[0].Path, "a")
	}
	if migration.Structure[1].Path != "b" {
		t.Errorf("structure[1].Path = %q, want %q", migration.Structure[1].Path, "b")
	}
}

func TestMerge_NextCleanedAfterMerge(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	staging := `description = "cleanup test"
[[structure]]
op = "add_field"
path = "x"
type = "string"
default = "val"
down = {op = "remove_field", path = "x"}
`
	os.WriteFile(filepath.Join(nextDir, "a.toml"), []byte(staging), 0o644)
	os.WriteFile(filepath.Join(nextDir, "b.toml"), []byte(staging), 0o644)

	_, err := Merge(dir, "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// next/ directory should still exist but be empty.
	entries, err := os.ReadDir(nextDir)
	if err != nil {
		t.Fatalf("next/ directory should still exist: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("next/ should be empty after merge, but has %d entries", len(entries))
	}
}

func TestMerge_LocalDateDefault(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	staging := `description = "add birthday field"

[[structure]]
op = "add_field"
path = "user.birthday"
type = "local_date"
default = 1979-05-27
down = {op = "remove_field", path = "user.birthday"}
`
	os.WriteFile(filepath.Join(nextDir, "add-birthday.toml"), []byte(staging), 0o644)

	outPath, err := Merge(dir, "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	content := string(data)

	// The merged output must contain a valid TOML date literal, not garbage like {1979 5 27}.
	if !strings.Contains(content, "1979-05-27") {
		t.Errorf("merged output does not contain valid date literal '1979-05-27':\n%s", content)
	}
	if strings.Contains(content, "{") && strings.Contains(content, "1979") {
		// Check it's not the garbage struct format.
		if strings.Contains(content, "{1979") {
			t.Errorf("merged output contains garbage struct format instead of date literal:\n%s", content)
		}
	}

	// Verify the output is valid parseable TOML.
	_, err = ops.ParseMigration(data)
	if err != nil {
		t.Fatalf("merged output is not valid TOML: %v", err)
	}
}

func TestMerge_StructureAndDataFromDifferentFiles(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	dir := t.TempDir()
	nextDir := filepath.Join(dir, "next")
	os.MkdirAll(nextDir, 0o755)

	// File 1 has structure ops.
	file1 := `description = "structure changes"
[[structure]]
op = "add_field"
path = "x"
type = "string"
default = "xval"
down = {op = "remove_field", path = "x"}

[[structure]]
op = "add_field"
path = "y"
type = "integer"
default = 10
down = {op = "remove_field", path = "y"}
`
	// File 2 has data ops.
	file2 := `description = "data changes"
[[data]]
op = "set_value"
path = "z"
value = true
down = {op = "remove_field", path = "z"}
`
	os.WriteFile(filepath.Join(nextDir, "01-structure.toml"), []byte(file1), 0o644)
	os.WriteFile(filepath.Join(nextDir, "02-data.toml"), []byte(file2), 0o644)

	outPath, err := Merge(dir, "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	migration, err := ops.ParseMigration(data)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(migration.Structure) != 2 {
		t.Errorf("structure ops = %d, want 2", len(migration.Structure))
	}
	if len(migration.Data) != 1 {
		t.Errorf("data ops = %d, want 1", len(migration.Data))
	}
}
