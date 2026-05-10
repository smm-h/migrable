package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/internal/config"
)

// setupMultiFileProject creates a temporary multi-file project with migrations.
func setupMultiFileProject(t *testing.T, files map[string]string, fileContents map[string]string, migrations map[string]string) *config.Config {
	t.Helper()
	dir := t.TempDir()

	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.Mkdir(migrationsDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	for key, content := range fileContents {
		path := files[key]
		fullPath := filepath.Join(dir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	for name, content := range migrations {
		migPath := filepath.Join(migrationsDir, name)
		if err := os.WriteFile(migPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write migration %s: %v", name, err)
		}
	}

	// Determine version_file: if more than one file, use the first key alphabetically.
	var versionFile string
	if len(files) > 1 {
		keys := sortedFileKeys(files)
		versionFile = keys[0]
	}

	return &config.Config{
		MigrationsDir: "migrations",
		VersionFile:   versionFile,
		Files:         files,
		BaseDir:       dir,
	}
}

func TestMultiFile_BasicMigration(t *testing.T) {
	files := map[string]string{
		"config": "config.toml",
		"themes": "themes.toml",
	}
	contents := map[string]string{
		"config": "title = \"My App\"\n",
		"themes": "[dark]\nbackground = \"#000\"\n",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "Add debug to config and font to themes"

[[structure]]
op = "add_field"
path = "config.debug"
type = "bool"
default = false

[[structure]]
op = "add_field"
path = "themes.dark.font_size"
type = "integer"
default = 14
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)

	result, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}

	// Verify config.toml has debug field.
	configData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	configDoc, _ := tomledit.Parse(configData)
	debugVal, ok := configDoc.GetBool("debug")
	if !ok {
		t.Fatal("debug field not found in config.toml")
	}
	if debugVal != false {
		t.Errorf("debug = %v, want false", debugVal)
	}

	// Verify themes.toml has font_size field.
	themesData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))
	themesDoc, _ := tomledit.Parse(themesData)
	fontSize, ok := themesDoc.GetInt("dark.font_size")
	if !ok {
		t.Fatal("dark.font_size not found in themes.toml")
	}
	if fontSize != 14 {
		t.Errorf("dark.font_size = %d, want 14", fontSize)
	}
}

func TestMultiFile_FileKeyRouting(t *testing.T) {
	files := map[string]string{
		"config":      "config.toml",
		"keybindings": "keybindings.toml",
	}
	contents := map[string]string{
		"config":      "title = \"App\"\n",
		"keybindings": "",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "Add fields to each file"

[[structure]]
op = "add_field"
path = "config.log_level"
type = "string"
default = "info"

[[structure]]
op = "add_field"
path = "keybindings.copy"
type = "string"
default = "ctrl+c"
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// config.toml should have log_level, NOT copy.
	configData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	configDoc, _ := tomledit.Parse(configData)
	if _, ok := configDoc.GetString("log_level"); !ok {
		t.Error("log_level not found in config.toml")
	}
	if configDoc.Get("copy") != nil {
		t.Error("copy should not be in config.toml")
	}

	// keybindings.toml should have copy, NOT log_level.
	kbData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "keybindings.toml"))
	kbDoc, _ := tomledit.Parse(kbData)
	if _, ok := kbDoc.GetString("copy"); !ok {
		t.Error("copy not found in keybindings.toml")
	}
	if kbDoc.Get("log_level") != nil {
		t.Error("log_level should not be in keybindings.toml")
	}
}

func TestMultiFile_VersionFile(t *testing.T) {
	files := map[string]string{
		"config": "config.toml",
		"themes": "themes.toml",
	}
	contents := map[string]string{
		"config": "title = \"App\"\n",
		"themes": "",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "Add a field"

[[structure]]
op = "add_field"
path = "themes.font"
type = "string"
default = "monospace"
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)
	// version_file defaults to "config" (first alphabetically)

	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// _schema_version should be in config.toml (the version file).
	configData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	configDoc, _ := tomledit.Parse(configData)
	verStr, ok := configDoc.GetString("_schema_version")
	if !ok {
		t.Fatal("_schema_version not found in config.toml (the version file)")
	}
	if verStr != "1.0.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "1.0.0")
	}

	// _schema_version should NOT be in themes.toml.
	themesData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))
	themesDoc, _ := tomledit.Parse(themesData)
	if _, ok := themesDoc.GetString("_schema_version"); ok {
		t.Error("_schema_version should not be in themes.toml")
	}
}

func TestMultiFile_CrossFileMoveField(t *testing.T) {
	files := map[string]string{
		"config": "config.toml",
		"themes": "themes.toml",
	}
	contents := map[string]string{
		"config": "title = \"App\"\ntheme_color = \"#ff0000\"\n",
		"themes": "[dark]\nbackground = \"#000\"\n",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "Move theme_color from config to themes"

[[structure]]
op = "move_field"
from = "config.theme_color"
to = "themes.dark.foreground"
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// theme_color should be gone from config.toml.
	configData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	configDoc, _ := tomledit.Parse(configData)
	if configDoc.Get("theme_color") != nil {
		t.Error("theme_color should be removed from config.toml")
	}

	// dark.foreground should exist in themes.toml with value "#ff0000".
	themesData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))
	themesDoc, _ := tomledit.Parse(themesData)
	fg, ok := themesDoc.GetString("dark.foreground")
	if !ok {
		t.Fatal("dark.foreground not found in themes.toml")
	}
	if fg != "#ff0000" {
		t.Errorf("dark.foreground = %q, want %q", fg, "#ff0000")
	}
}

func TestMultiFile_TransactionalWrite(t *testing.T) {
	files := map[string]string{
		"config": "config.toml",
		"themes": "themes.toml",
	}
	contents := map[string]string{
		"config": "title = \"App\"\n",
		"themes": "[dark]\nbackground = \"#000\"\n",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "First op succeeds, second fails"

[[structure]]
op = "add_field"
path = "config.debug"
type = "bool"
default = false

[[data]]
op = "remove_where"
path = "themes.nonexistent_array"
match_mode = "all"
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)

	// Save original contents.
	origConfig, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	origThemes, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))

	_, err := Migrate(cfg, false)
	if err == nil {
		t.Fatal("expected error from failed op, got nil")
	}

	// Neither file should be modified.
	afterConfig, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	afterThemes, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))

	if string(afterConfig) != string(origConfig) {
		t.Error("config.toml was modified despite migration failure")
	}
	if string(afterThemes) != string(origThemes) {
		t.Error("themes.toml was modified despite migration failure")
	}
}

func TestMultiFile_MissingFile(t *testing.T) {
	files := map[string]string{
		"config": "config.toml",
		"themes": "themes.toml",
	}
	// Only create config.toml; themes.toml does not exist.
	contents := map[string]string{
		"config": "title = \"App\"\n",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "Add field to missing file"

[[structure]]
op = "add_field"
path = "themes.font"
type = "string"
default = "sans"
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// themes.toml should now exist with the font field.
	themesData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))
	themesDoc, _ := tomledit.Parse(themesData)
	font, ok := themesDoc.GetString("font")
	if !ok {
		t.Fatal("font not found in themes.toml")
	}
	if font != "sans" {
		t.Errorf("font = %q, want %q", font, "sans")
	}
}

func TestMultiFile_InvalidPathPrefix(t *testing.T) {
	files := map[string]string{
		"config": "config.toml",
		"themes": "themes.toml",
	}
	contents := map[string]string{
		"config": "title = \"App\"\n",
		"themes": "",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "Path with unknown file key"

[[structure]]
op = "add_field"
path = "unknown.field"
type = "string"
default = "x"
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)
	_, err := Migrate(cfg, false)
	if err == nil {
		t.Fatal("expected error for invalid path prefix, got nil")
	}
	if !strings.Contains(err.Error(), "known file key") {
		t.Errorf("error = %q, want it to mention known file key", err.Error())
	}
}

func TestMultiFile_Rollback(t *testing.T) {
	files := map[string]string{
		"config": "config.toml",
		"themes": "themes.toml",
	}
	contents := map[string]string{
		"config": "title = \"App\"\n",
		"themes": "[dark]\nbackground = \"#000\"\n",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "Add fields to both files"

[[structure]]
op = "add_field"
path = "config.debug"
type = "bool"
default = false
down = { op = "remove_field", path = "config.debug" }

[[structure]]
op = "add_field"
path = "themes.dark.font_size"
type = "integer"
default = 14
down = { op = "remove_field", path = "themes.dark.font_size" }
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)

	// Apply migration.
	_, err := Migrate(cfg, false)
	if err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}

	// Verify fields were added.
	configData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	configDoc, _ := tomledit.Parse(configData)
	if _, ok := configDoc.GetBool("debug"); !ok {
		t.Fatal("debug not found after migration")
	}

	themesData, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))
	themesDoc, _ := tomledit.Parse(themesData)
	if _, ok := themesDoc.GetInt("dark.font_size"); !ok {
		t.Fatal("dark.font_size not found after migration")
	}

	// Rollback.
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

	// Verify fields were removed from both files.
	configData, _ = os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	configDoc, _ = tomledit.Parse(configData)
	if _, ok := configDoc.GetBool("debug"); ok {
		t.Error("debug should be removed from config.toml after rollback")
	}

	themesData, _ = os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))
	themesDoc, _ = tomledit.Parse(themesData)
	if _, ok := themesDoc.GetInt("dark.font_size"); ok {
		t.Error("dark.font_size should be removed from themes.toml after rollback")
	}

	// Verify _schema_version was reset.
	verStr, _ := configDoc.GetString("_schema_version")
	if verStr != "0.0.0" {
		t.Errorf("_schema_version = %q, want %q", verStr, "0.0.0")
	}
}

func TestMultiFile_DryRun(t *testing.T) {
	files := map[string]string{
		"config": "config.toml",
		"themes": "themes.toml",
	}
	contents := map[string]string{
		"config": "title = \"App\"\n",
		"themes": "[dark]\nbackground = \"#000\"\n",
	}
	migrations := map[string]string{
		"1.0.0.toml": `description = "Add fields to both files"

[[structure]]
op = "add_field"
path = "config.debug"
type = "bool"
default = false

[[structure]]
op = "add_field"
path = "themes.dark.font_size"
type = "integer"
default = 14
`,
	}

	cfg := setupMultiFileProject(t, files, contents, migrations)

	origConfig, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	origThemes, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))

	result, err := Migrate(cfg, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("Applied = %d, want 1", result.Applied)
	}

	// Files should NOT be modified.
	afterConfig, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "config.toml"))
	afterThemes, _ := os.ReadFile(filepath.Join(cfg.BaseDir, "themes.toml"))
	if string(afterConfig) != string(origConfig) {
		t.Error("config.toml was modified during dry-run")
	}
	if string(afterThemes) != string(origThemes) {
		t.Error("themes.toml was modified during dry-run")
	}

	// FileChanges should have entries for both files.
	configChanges, ok := result.FileChanges["config"]
	if !ok {
		t.Fatal("expected FileChanges for 'config'")
	}
	var hasDebug, hasVersion bool
	for _, c := range configChanges {
		if c.Path == "debug" && c.Kind == tomledit.Added {
			hasDebug = true
		}
		if c.Path == "_schema_version" && c.Kind == tomledit.Added {
			hasVersion = true
		}
	}
	if !hasDebug {
		t.Error("expected debug in config dry-run changes")
	}
	if !hasVersion {
		t.Error("expected _schema_version in config dry-run changes")
	}

	themesChanges, ok := result.FileChanges["themes"]
	if !ok {
		t.Fatal("expected FileChanges for 'themes'")
	}
	var hasFontSize bool
	for _, c := range themesChanges {
		if c.Path == "dark.font_size" && c.Kind == tomledit.Added {
			hasFontSize = true
		}
	}
	if !hasFontSize {
		t.Error("expected dark.font_size in themes dry-run changes")
	}
}

func TestSplitFileKey(t *testing.T) {
	t.Run("single file returns full path unchanged", func(t *testing.T) {
		key, inner, err := SplitFileKey("ui.theme", []string{"config"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "config" {
			t.Errorf("fileKey = %q, want %q", key, "config")
		}
		if inner != "ui.theme" {
			t.Errorf("innerPath = %q, want %q", inner, "ui.theme")
		}
	})

	t.Run("multi file splits on first segment", func(t *testing.T) {
		key, inner, err := SplitFileKey("themes.dark.background", []string{"config", "themes"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "themes" {
			t.Errorf("fileKey = %q, want %q", key, "themes")
		}
		if inner != "dark.background" {
			t.Errorf("innerPath = %q, want %q", inner, "dark.background")
		}
	})

	t.Run("escaped dot not treated as separator", func(t *testing.T) {
		key, inner, err := SplitFileKey(`my\.file.field`, []string{`my\.file`, "other"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != `my\.file` {
			t.Errorf("fileKey = %q, want %q", key, `my\.file`)
		}
		if inner != "field" {
			t.Errorf("innerPath = %q, want %q", inner, "field")
		}
	})

	t.Run("no match returns error", func(t *testing.T) {
		_, _, err := SplitFileKey("unknown.field", []string{"config", "themes"})
		if err == nil {
			t.Fatal("expected error for unknown file key, got nil")
		}
	})

	t.Run("path is just file key with no inner path", func(t *testing.T) {
		_, _, err := SplitFileKey("config", []string{"config", "themes"})
		if err == nil {
			t.Fatal("expected error for path with no inner path, got nil")
		}
	})

	t.Run("no file keys returns error", func(t *testing.T) {
		_, _, err := SplitFileKey("anything", []string{})
		if err == nil {
			t.Fatal("expected error for empty file keys, got nil")
		}
	})
}
