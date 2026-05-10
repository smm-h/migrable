package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFilesAtomic writes multiple files transactionally. All files are first
// written to temp files in the same directories as their targets. Only after all
// temp files are written successfully are they renamed to their final paths.
// On any failure, all temp files are cleaned up.
func WriteFilesAtomic(files map[string][]byte) error {
	// Phase 1: write all temp files.
	tmpPaths := make(map[string]string, len(files)) // final path -> temp path
	for path, data := range files {
		dir := filepath.Dir(path)
		tmp, err := os.CreateTemp(dir, ".migrable-atomic-*.tmp")
		if err != nil {
			cleanupTempFiles(tmpPaths)
			return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
		}
		tmpName := tmp.Name()
		tmpPaths[path] = tmpName

		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			cleanupTempFiles(tmpPaths)
			return fmt.Errorf("failed to write temp file for %s: %w", path, err)
		}
		if err := tmp.Close(); err != nil {
			cleanupTempFiles(tmpPaths)
			return fmt.Errorf("failed to close temp file for %s: %w", path, err)
		}
		if err := os.Chmod(tmpName, 0o666); err != nil {
			cleanupTempFiles(tmpPaths)
			return fmt.Errorf("failed to set permissions on temp file for %s: %w", path, err)
		}
	}

	// Phase 2: rename all temp files to final paths.
	for path, tmpName := range tmpPaths {
		if err := os.Rename(tmpName, path); err != nil {
			cleanupTempFiles(tmpPaths)
			return fmt.Errorf("failed to atomically replace %s: %w", path, err)
		}
		// Remove from map so cleanup doesn't try to delete the successfully renamed file.
		delete(tmpPaths, path)
	}

	return nil
}

func cleanupTempFiles(tmpPaths map[string]string) {
	for _, tmpName := range tmpPaths {
		os.Remove(tmpName)
	}
}

func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".migrable-atomic-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpName, 0o666); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to set permissions on temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to atomically replace %s: %w", path, err)
	}
	return nil
}
