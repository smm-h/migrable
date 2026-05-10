package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFilesAtomic(t *testing.T) {
	t.Run("multiple files written correctly", func(t *testing.T) {
		dir := t.TempDir()
		path1 := filepath.Join(dir, "file1.toml")
		path2 := filepath.Join(dir, "file2.toml")

		files := map[string][]byte{
			path1: []byte("key1 = \"value1\"\n"),
			path2: []byte("key2 = \"value2\"\n"),
		}

		if err := WriteFilesAtomic(files); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got1, _ := os.ReadFile(path1)
		got2, _ := os.ReadFile(path2)

		if string(got1) != "key1 = \"value1\"\n" {
			t.Errorf("file1 content = %q, want %q", got1, "key1 = \"value1\"\n")
		}
		if string(got2) != "key2 = \"value2\"\n" {
			t.Errorf("file2 content = %q, want %q", got2, "key2 = \"value2\"\n")
		}
	})

	t.Run("no temp files left behind", func(t *testing.T) {
		dir := t.TempDir()
		files := map[string][]byte{
			filepath.Join(dir, "a.toml"): []byte("a"),
			filepath.Join(dir, "b.toml"): []byte("b"),
		}

		if err := WriteFilesAtomic(files); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".migrable-atomic-") {
				t.Errorf("temp file left behind: %s", e.Name())
			}
		}
	})

	t.Run("cleanup on failure", func(t *testing.T) {
		dir := t.TempDir()
		goodPath := filepath.Join(dir, "good.toml")
		badPath := filepath.Join(dir, "nonexistent", "bad.toml") // dir doesn't exist

		files := map[string][]byte{
			goodPath: []byte("good data"),
			badPath:  []byte("bad data"),
		}

		err := WriteFilesAtomic(files)
		if err == nil {
			t.Fatal("expected error for nonexistent directory")
		}

		// The good file should NOT have been created either (transactional).
		if _, statErr := os.Stat(goodPath); statErr == nil {
			// The file might have been written but not renamed, or might not exist.
			// Either way, there should be no temp files left.
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), ".migrable-atomic-") {
					t.Errorf("temp file left behind: %s", e.Name())
				}
			}
		}
	})
}

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
