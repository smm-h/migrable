package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	t.Run("writes file correctly", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "output.toml")
		content := []byte("key = \"value\"\n")

		if err := WriteFileAtomic(path, content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("content = %q, want %q", got, content)
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "output.toml")
		os.WriteFile(path, []byte("old content"), 0o644)

		newContent := []byte("new content")
		if err := WriteFileAtomic(path, newContent); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(got) != string(newContent) {
			t.Errorf("content = %q, want %q", got, newContent)
		}
	})

	t.Run("no temp files left behind", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "output.toml")

		if err := WriteFileAtomic(path, []byte("data")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read dir: %v", err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".migrable-atomic-") {
				t.Errorf("temp file left behind: %s", e.Name())
			}
		}
	})

	t.Run("file has 0666 permissions before umask", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "output.toml")

		if err := WriteFileAtomic(path, []byte("data")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		// os.Chmod(0o666) is applied, then the umask may reduce it.
		// The typical umask is 0022, yielding 0644. We check that the
		// file is NOT the restrictive 0600 that os.CreateTemp would set.
		perm := info.Mode().Perm()
		if perm == 0o600 {
			t.Errorf("file permissions = %04o, want broader than 0600 (e.g. 0644 with typical umask)", perm)
		}
	})

	t.Run("fails for nonexistent directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent", "output.toml")
		err := WriteFileAtomic(path, []byte("data"))
		if err == nil {
			t.Fatal("expected error for nonexistent directory")
		}
	})
}
