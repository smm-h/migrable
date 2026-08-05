package engine

import (
	"github.com/smm-h/stricttest/go/hygiene"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestReadSchemaVersion(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	tests := []struct {
		name    string
		content string
		exists  bool
		want    string
		wantErr string
	}{
		{
			name:   "file does not exist",
			exists: false,
			want:   "0.0.0",
		},
		{
			name:    "empty file",
			exists:  true,
			content: "",
			want:    "0.0.0",
		},
		{
			name:    "file without _schema_version",
			exists:  true,
			content: "key = \"value\"\n",
			want:    "0.0.0",
		},
		{
			name:    "valid version",
			exists:  true,
			content: "_schema_version = \"1.2.3\"\nkey = \"value\"\n",
			want:    "1.2.3",
		},
		{
			name:    "invalid toml",
			exists:  true,
			content: "this is not [[[valid toml",
			wantErr: "failed to parse",
		},
		{
			name:    "invalid semver",
			exists:  true,
			content: "_schema_version = \"not-a-version\"\n",
			wantErr: "invalid _schema_version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			if tt.exists {
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatalf("failed to write: %v", err)
				}
			}

			v, err := ReadSchemaVersion(path)
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
			if v.String() != tt.want {
				t.Errorf("version = %s, want %s", v, tt.want)
			}
		})
	}
}

func TestWriteSchemaVersion(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("create new file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		v := semver.MustParse("1.0.0")

		if err := WriteSchemaVersion(path, v); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := ReadSchemaVersion(path)
		if err != nil {
			t.Fatalf("failed to read back: %v", err)
		}
		if !got.Equal(v) {
			t.Errorf("read back version = %s, want %s", got, v)
		}
	})

	t.Run("update existing file preserving content", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		original := "# important comment\nkey = \"value\"\n_schema_version = \"1.0.0\"\n"
		os.WriteFile(path, []byte(original), 0o644)

		v := semver.MustParse("2.0.0")
		if err := WriteSchemaVersion(path, v); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "# important comment") {
			t.Error("comment was not preserved")
		}
		if !strings.Contains(content, "key = \"value\"") {
			t.Error("existing key was not preserved")
		}

		got, err := ReadSchemaVersion(path)
		if err != nil {
			t.Fatalf("failed to read back: %v", err)
		}
		if !got.Equal(v) {
			t.Errorf("version = %s, want %s", got, v)
		}
	})

	t.Run("atomic write - no temp files left", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		v := semver.MustParse("1.0.0")

		if err := WriteSchemaVersion(path, v); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read dir: %v", err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".migrable-version-") {
				t.Errorf("temp file left behind: %s", e.Name())
			}
		}
	})

	t.Run("write to nonexistent directory fails", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent", "config.toml")
		v := semver.MustParse("1.0.0")

		err := WriteSchemaVersion(path, v)
		if err == nil {
			t.Fatal("expected error for nonexistent directory, got nil")
		}
	})
}
