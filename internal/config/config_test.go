package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr string
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid single file",
			toml: `migrations_dir = "migrations"
[files]
app = "app.toml"
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.MigrationsDir != "migrations" {
					t.Errorf("MigrationsDir = %q, want %q", cfg.MigrationsDir, "migrations")
				}
				if len(cfg.Files) != 1 {
					t.Fatalf("len(Files) = %d, want 1", len(cfg.Files))
				}
				if cfg.Files["app"] != "app.toml" {
					t.Errorf("Files[app] = %q, want %q", cfg.Files["app"], "app.toml")
				}
				if cfg.VersionFile != "" {
					t.Errorf("VersionFile = %q, want empty", cfg.VersionFile)
				}
			},
		},
		{
			name: "valid multi file with version_file",
			toml: `migrations_dir = "migrations"
version_file = "primary"
[files]
primary = "primary.toml"
secondary = "secondary.toml"
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.VersionFile != "primary" {
					t.Errorf("VersionFile = %q, want %q", cfg.VersionFile, "primary")
				}
				if len(cfg.Files) != 2 {
					t.Fatalf("len(Files) = %d, want 2", len(cfg.Files))
				}
			},
		},
		{
			name:    "missing migrations_dir",
			toml:    "[files]\napp = \"app.toml\"\n",
			wantErr: "missing required key \"migrations_dir\"",
		},
		{
			name:    "missing files section",
			toml:    "migrations_dir = \"m\"\n",
			wantErr: "missing required section [files]",
		},
		{
			name:    "empty files section",
			toml:    "migrations_dir = \"m\"\n[files]\n",
			wantErr: "[files] must have at least one entry",
		},
		{
			name: "multi file missing version_file",
			toml: `migrations_dir = "m"
[files]
a = "a.toml"
b = "b.toml"
`,
			wantErr: "\"version_file\" is required when [files] has more than one entry",
		},
		{
			name: "version_file not in files",
			toml: `migrations_dir = "m"
version_file = "missing"
[files]
a = "a.toml"
b = "b.toml"
`,
			wantErr: "\"version_file\" value \"missing\" is not a key in [files]",
		},
		{
			name:    "invalid toml",
			toml:    "this is not valid toml [[[",
			wantErr: "failed to parse migrable.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parse([]byte(tt.toml), "/test/base")
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.BaseDir != "/test/base" {
				t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, "/test/base")
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestExpandEnvStrict(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		env     map[string]string
		want    string
		wantErr string
	}{
		{
			name:  "no variables",
			input: "/plain/path",
			want:  "/plain/path",
		},
		{
			name:  "simple variable",
			input: "$HOME/config",
			env:   map[string]string{"HOME": "/home/user"},
			want:  "/home/user/config",
		},
		{
			name:  "braced variable",
			input: "${HOME}/config",
			env:   map[string]string{"HOME": "/home/user"},
			want:  "/home/user/config",
		},
		{
			name:  "multiple variables",
			input: "$HOME/$APP/config",
			env:   map[string]string{"HOME": "/home/user", "APP": "myapp"},
			want:  "/home/user/myapp/config",
		},
		{
			name:    "unset variable",
			input:   "$MISSING/config",
			wantErr: "environment variable \"MISSING\" is not set",
		},
		{
			name:    "unset braced variable",
			input:   "${MISSING}/config",
			wantErr: "environment variable \"MISSING\" is not set",
		},
		{
			name:    "unterminated brace",
			input:   "${HOME",
			wantErr: "unterminated",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "dollar at end",
			input: "path$",
			want:  "path$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got, err := expandEnvStrict(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expandEnvStrict(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoad_Discovery(t *testing.T) {
	t.Run("found in current dir", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, configFileName), `migrations_dir = "m"
[files]
app = "app.toml"
`)
		origDir := chdir(t, dir)
		defer os.Chdir(origDir)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MigrationsDir != "m" {
			t.Errorf("MigrationsDir = %q, want %q", cfg.MigrationsDir, "m")
		}
	})

	t.Run("ignores .migrable subdir", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, ".migrable")
		os.Mkdir(subDir, 0o755)
		writeFile(t, filepath.Join(subDir, configFileName), `migrations_dir = "m"
[files]
app = "app.toml"
`)
		origDir := chdir(t, dir)
		defer os.Chdir(origDir)

		_, err := Load("")
		if err == nil {
			t.Fatal("expected not found error when config only in .migrable/")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), "not found")
		}
	})

	t.Run("current dir preferred over .migrable subdir", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, configFileName), `migrations_dir = "m"
[files]
app = "app.toml"
`)
		subDir := filepath.Join(dir, ".migrable")
		os.Mkdir(subDir, 0o755)
		writeFile(t, filepath.Join(subDir, configFileName), `migrations_dir = "other"
[files]
app = "other.toml"
`)
		origDir := chdir(t, dir)
		defer os.Chdir(origDir)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MigrationsDir != "m" {
			t.Errorf("MigrationsDir = %q, want %q (should load from current dir, not .migrable/)", cfg.MigrationsDir, "m")
		}
	})

	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		origDir := chdir(t, dir)
		defer os.Chdir(origDir)

		_, err := Load("")
		if err == nil {
			t.Fatal("expected not found error, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), "not found")
		}
	})

	t.Run("explicit config-dir", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, configFileName), `migrations_dir = "m"
[files]
app = "app.toml"
`)
		cfg, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MigrationsDir != "m" {
			t.Errorf("MigrationsDir = %q, want %q", cfg.MigrationsDir, "m")
		}
	})

	t.Run("explicit config-dir missing", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load(dir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), "does not exist")
		}
	})
}

func TestParse_EnvExpansionInFiles(t *testing.T) {
	t.Setenv("TEST_CONFIG_DIR", "/opt/configs")

	toml := `migrations_dir = "m"
[files]
app = "$TEST_CONFIG_DIR/app.toml"
`
	cfg, err := parse([]byte(toml), "/base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/opt/configs/app.toml"
	if cfg.Files["app"] != want {
		t.Errorf("Files[app] = %q, want %q", cfg.Files["app"], want)
	}
}

func TestParse_EnvExpansionUnsetError(t *testing.T) {
	os.Unsetenv("MIGRABLE_TEST_UNSET_VAR")

	toml := `migrations_dir = "m"
[files]
app = "$MIGRABLE_TEST_UNSET_VAR/app.toml"
`
	_, err := parse([]byte(toml), "/base")
	if err == nil {
		t.Fatal("expected error for unset env var, got nil")
	}
	if !strings.Contains(err.Error(), "MIGRABLE_TEST_UNSET_VAR") {
		t.Fatalf("error = %q, want it to mention the variable name", err.Error())
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func chdir(t *testing.T, dir string) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to %s: %v", dir, err)
	}
	return orig
}
