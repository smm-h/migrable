package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMigrations(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		wantVers []string
		wantErr  string
	}{
		{
			name:     "empty directory",
			files:    nil,
			wantVers: nil,
		},
		{
			name:     "single migration",
			files:    []string{"1.0.0.toml"},
			wantVers: []string{"1.0.0"},
		},
		{
			name:     "sorted by semver",
			files:    []string{"2.0.0.toml", "1.0.0.toml", "1.1.0.toml"},
			wantVers: []string{"1.0.0", "1.1.0", "2.0.0"},
		},
		{
			name:     "patch versions sorted",
			files:    []string{"1.0.2.toml", "1.0.0.toml", "1.0.1.toml"},
			wantVers: []string{"1.0.0", "1.0.1", "1.0.2"},
		},
		{
			name:    "invalid filename",
			files:   []string{"1.0.0.toml", "not-a-version.toml"},
			wantErr: "invalid migration filename",
		},
		{
			name:    "partial version",
			files:   []string{"1.0.toml"},
			wantErr: "invalid migration filename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte("# migration\n"), 0o644); err != nil {
					t.Fatalf("failed to write %s: %v", f, err)
				}
			}

			migs, err := DiscoverMigrations(dir)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !containsStr(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(migs) != len(tt.wantVers) {
				t.Fatalf("got %d migrations, want %d", len(migs), len(tt.wantVers))
			}
			for i, want := range tt.wantVers {
				if migs[i].Version.String() != want {
					t.Errorf("migration[%d].Version = %s, want %s", i, migs[i].Version, want)
				}
				if !filepath.IsAbs(migs[i].FilePath) {
					t.Errorf("migration[%d].FilePath = %q, want absolute path", i, migs[i].FilePath)
				}
			}
		})
	}
}

func TestDiscoverMigrations_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte("# ok\n"), 0o644)
	os.Mkdir(filepath.Join(dir, "next"), 0o755)
	os.WriteFile(filepath.Join(dir, "next", "2.0.0.toml"), []byte("# skip\n"), 0o644)

	migs, err := DiscoverMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migs) != 1 {
		t.Fatalf("got %d migrations, want 1", len(migs))
	}
	if migs[0].Version.String() != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", migs[0].Version)
	}
}

func TestDiscoverMigrations_SkipsNonToml(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "1.0.0.toml"), []byte("# ok\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# not a migration\n"), 0o644)

	migs, err := DiscoverMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migs) != 1 {
		t.Fatalf("got %d migrations, want 1", len(migs))
	}
}

func TestDiscoverMigrations_NonexistentDir(t *testing.T) {
	_, err := DiscoverMigrations("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
