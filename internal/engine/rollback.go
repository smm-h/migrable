package engine

import (
	"bytes"
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
	// Build sorted file keys for deterministic ordering and path routing.
	fileKeys := sortedFileKeys(cfg.Files)

	// Resolve all file paths and load documents.
	docs := make(map[string]*tomledit.DocumentNode, len(cfg.Files))
	filePaths := make(map[string]string, len(cfg.Files))
	for key, rel := range cfg.Files {
		p := rel
		if !filepath.IsAbs(p) {
			p = filepath.Join(cfg.BaseDir, p)
		}
		filePaths[key] = p

		data, err := os.ReadFile(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("target file %s does not exist, nothing to roll back", p)
			}
			return nil, fmt.Errorf("failed to read target file %s: %w", p, err)
		}

		doc, err := tomledit.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("target file %s contains invalid TOML: %w", p, err)
		}
		docs[key] = doc
	}

	// Determine the version file key.
	versionKey := cfg.VersionFile
	if versionKey == "" {
		if len(fileKeys) == 1 {
			versionKey = fileKeys[0]
		} else {
			return nil, fmt.Errorf("version_file must be set for multi-file projects")
		}
	}
	versionDoc := docs[versionKey]

	// Read _schema_version from the version document.
	currentVersion := semver.MustParse("0.0.0")
	if verStr, ok := versionDoc.GetString("_schema_version"); ok {
		v, parseErr := semver.StrictNewVersion(verStr)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid _schema_version %q in %s: %w", verStr, filePaths[versionKey], parseErr)
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

	// Save original documents for dry-run diff.
	originalDocs := make(map[string]*tomledit.DocumentNode, len(docs))
	for key, doc := range docs {
		origBytes := doc.Bytes()
		origDoc, _ := tomledit.Parse(origBytes)
		originalDocs[key] = origDoc
	}

	// Backup for rollback on failure.
	backups := backupDocs(docs)

	// Execute down ops in reverse order: data first (reversed), then structure (reversed).
	for i := len(migration.Data) - 1; i >= 0; i-- {
		op := migration.Data[i]
		if op.Down == nil {
			continue
		}
		if err := executeDownOpsMultiFile(docs, op.Down.Ops, fileKeys); err != nil {
			restoreDocs(docs, backups)
			return nil, fmt.Errorf("rollback %s: data[%d] down: %w", currentVersion, i, err)
		}
	}

	for i := len(migration.Structure) - 1; i >= 0; i-- {
		op := migration.Structure[i]
		if op.Down == nil {
			continue
		}
		if err := executeDownOpsMultiFile(docs, op.Down.Ops, fileKeys); err != nil {
			restoreDocs(docs, backups)
			return nil, fmt.Errorf("rollback %s: structure[%d] down: %w", currentVersion, i, err)
		}
	}

	// Update _schema_version to the previous version.
	if setErr := docs[versionKey].SetCreate("_schema_version", previousVersion.String()); setErr != nil {
		restoreDocs(docs, backups)
		return nil, fmt.Errorf("rollback %s: failed to update _schema_version: %w", currentVersion, setErr)
	}

	result.Applied = 1

	if !dryRun {
		fileData := make(map[string][]byte, len(docs))
		for key, doc := range docs {
			serialized := doc.Bytes()
			if !bytes.Equal(backups[key], serialized) {
				fileData[filePaths[key]] = serialized
			}
		}
		if len(fileData) > 0 {
			if writeErr := WriteFilesAtomic(fileData); writeErr != nil {
				return nil, fmt.Errorf("rollback %s: %w", currentVersion, writeErr)
			}
		}
	}

	if dryRun {
		for key, doc := range docs {
			changes := tomledit.Diff(originalDocs[key], doc)
			if len(changes) > 0 {
				result.FileChanges[key] = changes
			}
		}
	}

	return result, nil
}

// executeDownOpsMultiFile executes a slice of down ops in reverse order,
// routing each op to the correct file.
func executeDownOpsMultiFile(docs map[string]*tomledit.DocumentNode, downOps []ops.Op, fileKeys []string) error {
	for i := len(downOps) - 1; i >= 0; i-- {
		if err := executeMultiFileOp(docs, downOps[i], fileKeys); err != nil {
			return err
		}
	}
	return nil
}
