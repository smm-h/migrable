package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/internal/config"
	"github.com/smm-h/migrable/internal/ops"
)

// MigrateResult holds the outcome of a Migrate call.
type MigrateResult struct {
	Applied     int
	FromVersion *semver.Version
	ToVersion   *semver.Version
	// Descriptions maps migration version to its description string.
	Descriptions map[string]string
	// FileChanges maps file key to changes (for dry-run output).
	FileChanges map[string][]tomledit.Change
}

// Migrate applies all pending migrations to the single target file.
func Migrate(cfg *config.Config, dryRun bool) (*MigrateResult, error) {
	if len(cfg.Files) != 1 {
		return nil, fmt.Errorf("migrate currently supports single-file projects only (found %d files)", len(cfg.Files))
	}

	// Resolve the single file entry.
	var fileKey string
	var filePath string
	for k, v := range cfg.Files {
		fileKey = k
		filePath = v
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(cfg.BaseDir, filePath)
	}

	// Read the target file.
	fileData, err := os.ReadFile(filePath)
	fileExists := true
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fileExists = false
			fileData = []byte{}
		} else {
			return nil, fmt.Errorf("failed to read target file %s: %w", filePath, err)
		}
	}

	// Parse the target file.
	doc, err := tomledit.Parse(fileData)
	if err != nil {
		if fileExists {
			return nil, fmt.Errorf("target file %s contains invalid TOML: %w", filePath, err)
		}
		return nil, fmt.Errorf("failed to parse empty document: %w", err)
	}

	// Read _schema_version from the document.
	currentVersion := semver.MustParse("0.0.0")
	if verStr, ok := doc.GetString("_schema_version"); ok {
		v, parseErr := semver.StrictNewVersion(verStr)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid _schema_version %q in %s: %w", verStr, filePath, parseErr)
		}
		currentVersion = v
	}

	// Discover migrations.
	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	allMigrations, err := DiscoverMigrations(migrationsDir)
	if err != nil {
		return nil, err
	}

	// Filter to pending migrations (version > current).
	var pending []MigrationMeta
	for _, m := range allMigrations {
		if m.Version.GreaterThan(currentVersion) {
			pending = append(pending, m)
		}
	}

	result := &MigrateResult{
		FromVersion:  currentVersion,
		ToVersion:    currentVersion,
		Descriptions: make(map[string]string),
		FileChanges:  make(map[string][]tomledit.Change),
	}

	if len(pending) == 0 {
		return result, nil
	}

	// Save the original document state for dry-run diff.
	originalBytes := doc.Bytes()
	originalDoc, _ := tomledit.Parse(originalBytes)

	// Apply each pending migration in order.
	for _, meta := range pending {
		migData, readErr := os.ReadFile(meta.FilePath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %w", meta.FilePath, readErr)
		}

		migration, parseErr := ops.ParseMigration(migData)
		if parseErr != nil {
			return nil, fmt.Errorf("migration %s: %w", meta.Version, parseErr)
		}

		result.Descriptions[meta.Version.String()] = migration.Description

		// Backup the document state before this migration (for rollback).
		backup := doc.Bytes()

		// Apply structure ops.
		for i, op := range migration.Structure {
			if execErr := ops.Execute(doc, op); execErr != nil {
				// Rollback: restore from backup.
				doc, _ = tomledit.Parse(backup)
				return nil, fmt.Errorf("migration %s: structure[%d] (%s): %w",
					meta.Version, i, op.Type, execErr)
			}
		}

		// Apply data ops.
		for i, op := range migration.Data {
			if execErr := ops.Execute(doc, op); execErr != nil {
				// Rollback: restore from backup.
				doc, _ = tomledit.Parse(backup)
				return nil, fmt.Errorf("migration %s: data[%d] (%s): %w",
					meta.Version, i, op.Type, execErr)
			}
		}

		// Update _schema_version to this migration's version.
		if setErr := doc.SetCreate("_schema_version", meta.Version.String()); setErr != nil {
			doc, _ = tomledit.Parse(backup)
			return nil, fmt.Errorf("migration %s: failed to update _schema_version: %w",
				meta.Version, setErr)
		}

		result.Applied++
		result.ToVersion = meta.Version

		// If not dry-run, write atomically after each migration.
		if !dryRun {
			if writeErr := WriteFileAtomic(filePath, doc.Bytes()); writeErr != nil {
				return nil, fmt.Errorf("migration %s: failed to write %s: %w",
					meta.Version, filePath, writeErr)
			}
		}
	}

	// For dry-run, compute diff between original and final state.
	if dryRun {
		changes := tomledit.Diff(originalDoc, doc)
		if len(changes) > 0 {
			result.FileChanges[fileKey] = changes
		}
	}

	return result, nil
}
