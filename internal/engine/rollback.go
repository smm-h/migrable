package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/internal/config"
	"github.com/smm-h/migrable/internal/ops"
)

// RollbackBlockedError is returned when a rollback cannot proceed because
// one or more ops are marked as irreversible.
type RollbackBlockedError struct {
	IrreversibleOps []string
}

func (e *RollbackBlockedError) Error() string {
	return fmt.Sprintf("rollback blocked: irreversible op(s): %s", strings.Join(e.IrreversibleOps, ", "))
}

// Rollback reverses the most recently applied migration.
func Rollback(cfg *config.Config, dryRun bool) (*MigrateResult, error) {
	if len(cfg.Files) != 1 {
		return nil, fmt.Errorf("rollback currently supports single-file projects only (found %d files)", len(cfg.Files))
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
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("target file %s does not exist, nothing to roll back", filePath)
		}
		return nil, fmt.Errorf("failed to read target file %s: %w", filePath, err)
	}

	// Parse the target file.
	doc, err := tomledit.Parse(fileData)
	if err != nil {
		return nil, fmt.Errorf("target file %s contains invalid TOML: %w", filePath, err)
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

	// Nothing to roll back at 0.0.0.
	if currentVersion.Equal(semver.MustParse("0.0.0")) {
		return nil, fmt.Errorf("nothing to roll back: _schema_version is 0.0.0")
	}

	// Discover migrations.
	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	allMigrations, err := DiscoverMigrations(migrationsDir)
	if err != nil {
		return nil, err
	}

	// Find the migration whose version matches the current _schema_version.
	targetIdx := -1
	for i, m := range allMigrations {
		if m.Version.Equal(currentVersion) {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return nil, fmt.Errorf("migration file for current version %s not found", currentVersion)
	}

	// Parse the migration file.
	migData, err := os.ReadFile(allMigrations[targetIdx].FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file %s: %w", allMigrations[targetIdx].FilePath, err)
	}
	migration, err := ops.ParseMigration(migData)
	if err != nil {
		return nil, fmt.Errorf("migration %s: %w", currentVersion, err)
	}

	// Check for irreversible ops BEFORE executing anything.
	var irreversibleDescs []string
	for i, op := range migration.Structure {
		if op.Down != nil && op.Down.Irreversible {
			irreversibleDescs = append(irreversibleDescs, fmt.Sprintf("structure[%d] (%s)", i, op.Type))
		}
	}
	for i, op := range migration.Data {
		if op.Down != nil && op.Down.Irreversible {
			irreversibleDescs = append(irreversibleDescs, fmt.Sprintf("data[%d] (%s)", i, op.Type))
		}
	}
	if len(irreversibleDescs) > 0 {
		return nil, &RollbackBlockedError{IrreversibleOps: irreversibleDescs}
	}

	// Determine the previous version.
	previousVersion := semver.MustParse("0.0.0")
	if targetIdx > 0 {
		previousVersion = allMigrations[targetIdx-1].Version
	}

	result := &MigrateResult{
		FromVersion:  currentVersion,
		ToVersion:    previousVersion,
		Descriptions: map[string]string{currentVersion.String(): migration.Description},
		FileChanges:  make(map[string][]tomledit.Change),
	}

	// Save original state for dry-run diff.
	originalBytes := doc.Bytes()
	originalDoc, _ := tomledit.Parse(originalBytes)

	// Backup for rollback on failure.
	backup := doc.Bytes()

	// Execute down ops in reverse order: data first (reversed), then structure (reversed).
	// Reverse data section.
	for i := len(migration.Data) - 1; i >= 0; i-- {
		op := migration.Data[i]
		if op.Down == nil {
			continue
		}
		if err := executeDownOps(doc, op.Down.Ops); err != nil {
			doc, _ = tomledit.Parse(backup)
			return nil, fmt.Errorf("rollback %s: data[%d] down: %w", currentVersion, i, err)
		}
	}

	// Reverse structure section.
	for i := len(migration.Structure) - 1; i >= 0; i-- {
		op := migration.Structure[i]
		if op.Down == nil {
			continue
		}
		if err := executeDownOps(doc, op.Down.Ops); err != nil {
			doc, _ = tomledit.Parse(backup)
			return nil, fmt.Errorf("rollback %s: structure[%d] down: %w", currentVersion, i, err)
		}
	}

	// Update _schema_version to the previous version.
	if setErr := doc.SetCreate("_schema_version", previousVersion.String()); setErr != nil {
		doc, _ = tomledit.Parse(backup)
		return nil, fmt.Errorf("rollback %s: failed to update _schema_version: %w", currentVersion, setErr)
	}

	result.Applied = 1

	if !dryRun {
		if writeErr := WriteFileAtomic(filePath, doc.Bytes()); writeErr != nil {
			return nil, fmt.Errorf("rollback %s: failed to write %s: %w", currentVersion, filePath, writeErr)
		}
	}

	if dryRun {
		changes := tomledit.Diff(originalDoc, doc)
		if len(changes) > 0 {
			result.FileChanges[fileKey] = changes
		}
	}

	return result, nil
}

// executeDownOps executes a slice of down ops in reverse order.
func executeDownOps(doc *tomledit.DocumentNode, downOps []ops.Op) error {
	for i := len(downOps) - 1; i >= 0; i-- {
		if err := ops.Execute(doc, downOps[i]); err != nil {
			return err
		}
	}
	return nil
}
