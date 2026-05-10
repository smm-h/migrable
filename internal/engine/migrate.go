package engine

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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

// Migrate applies all pending migrations to the target file(s).
func Migrate(cfg *config.Config, dryRun bool) (*MigrateResult, error) {
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
				data = []byte{}
			} else {
				return nil, fmt.Errorf("failed to read target file %s: %w", p, err)
			}
		}

		doc, err := tomledit.Parse(data)
		if err != nil {
			if len(data) > 0 {
				return nil, fmt.Errorf("target file %s contains invalid TOML: %w", p, err)
			}
			return nil, fmt.Errorf("failed to parse empty document: %w", err)
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

	// Save original documents for dry-run diff.
	originalDocs := make(map[string]*tomledit.DocumentNode, len(docs))
	for key, doc := range docs {
		origBytes := doc.Bytes()
		origDoc, _ := tomledit.Parse(origBytes)
		originalDocs[key] = origDoc
	}

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

		// Backup all documents before this migration.
		backups := backupDocs(docs)

		// Apply structure ops.
		for i, op := range migration.Structure {
			if execErr := executeMultiFileOp(docs, op, fileKeys); execErr != nil {
				restoreDocs(docs, backups)
				return nil, fmt.Errorf("migration %s: structure[%d] (%s): %w",
					meta.Version, i, op.Type, execErr)
			}
		}

		// Apply data ops.
		for i, op := range migration.Data {
			if execErr := executeMultiFileOp(docs, op, fileKeys); execErr != nil {
				restoreDocs(docs, backups)
				return nil, fmt.Errorf("migration %s: data[%d] (%s): %w",
					meta.Version, i, op.Type, execErr)
			}
		}

		// Update _schema_version in the version file's document.
		if setErr := docs[versionKey].SetCreate("_schema_version", meta.Version.String()); setErr != nil {
			restoreDocs(docs, backups)
			return nil, fmt.Errorf("migration %s: failed to update _schema_version: %w",
				meta.Version, setErr)
		}

		result.Applied++
		result.ToVersion = meta.Version

		// If not dry-run, write only files that changed after this migration.
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
					return nil, fmt.Errorf("migration %s: %w", meta.Version, writeErr)
				}
			}
		}
	}

	// For dry-run, compute diff between original and final state per file.
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

// executeMultiFileOp resolves the file key from the op's path fields and
// executes the op on the appropriate document(s).
func executeMultiFileOp(docs map[string]*tomledit.DocumentNode, op ops.Op, fileKeys []string) error {
	switch op.Type {
	case ops.OpMoveField:
		return executeMoveFieldMultiFile(docs, op, fileKeys)
	case ops.OpRenameField:
		return executeRenameFieldMultiFile(docs, op, fileKeys)
	default:
		return executePathOp(docs, op, fileKeys)
	}
}

// executePathOp handles ops that use op.Path for file routing.
func executePathOp(docs map[string]*tomledit.DocumentNode, op ops.Op, fileKeys []string) error {
	fileKey, innerPath, err := SplitFileKey(op.Path, fileKeys)
	if err != nil {
		return err
	}
	doc := docs[fileKey]
	modified := op
	modified.Path = innerPath
	return ops.Execute(doc, modified)
}

// executeRenameFieldMultiFile handles rename_field: both from and to must
// resolve to the same file.
func executeRenameFieldMultiFile(docs map[string]*tomledit.DocumentNode, op ops.Op, fileKeys []string) error {
	fromKey, fromInner, err := SplitFileKey(op.From, fileKeys)
	if err != nil {
		return fmt.Errorf("from path: %w", err)
	}
	toKey, _, err := SplitFileKey(op.To, fileKeys)
	if err != nil {
		return fmt.Errorf("to path: %w", err)
	}
	if fromKey != toKey {
		return fmt.Errorf("rename_field: from (%s) and to (%s) must target the same file", fromKey, toKey)
	}

	doc := docs[fromKey]
	modified := op
	// rename_field uses From for the path to the key being renamed
	modified.From = fromInner
	// For to, extract just the new key name (last segment after the last unescaped dot)
	_, toInner, _ := SplitFileKey(op.To, fileKeys)
	modified.To = toInner
	return ops.Execute(doc, modified)
}

// executeMoveFieldMultiFile handles move_field: source and destination can be
// in different files. When cross-file, it reads from source doc, writes to dest
// doc, and deletes from source doc.
func executeMoveFieldMultiFile(docs map[string]*tomledit.DocumentNode, op ops.Op, fileKeys []string) error {
	fromKey, fromInner, err := SplitFileKey(op.From, fileKeys)
	if err != nil {
		return fmt.Errorf("from path: %w", err)
	}
	toKey, toInner, err := SplitFileKey(op.To, fileKeys)
	if err != nil {
		return fmt.Errorf("to path: %w", err)
	}

	if fromKey == toKey {
		// Same file: use the standard move_field implementation.
		doc := docs[fromKey]
		modified := op
		modified.From = fromInner
		modified.To = toInner
		return ops.Execute(doc, modified)
	}

	// Cross-file move: read from source, write to dest, delete from source.
	srcDoc := docs[fromKey]
	dstDoc := docs[toKey]

	srcNode := srcDoc.Get(fromInner)
	if srcNode == nil {
		return nil
	}
	if dstDoc.Get(toInner) != nil {
		return fmt.Errorf("move_field: target path %q already exists in file %q", toInner, toKey)
	}
	val := srcNode.Value()
	if err := dstDoc.SetCreate(toInner, val); err != nil {
		return fmt.Errorf("move_field: failed to set %q in file %q: %w", toInner, toKey, err)
	}
	if err := srcDoc.Delete(fromInner); err != nil {
		return fmt.Errorf("move_field: failed to delete %q from file %q: %w", fromInner, fromKey, err)
	}
	return nil
}

func sortedFileKeys(files map[string]string) []string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func backupDocs(docs map[string]*tomledit.DocumentNode) map[string][]byte {
	backups := make(map[string][]byte, len(docs))
	for key, doc := range docs {
		backups[key] = doc.Bytes()
	}
	return backups
}

func restoreDocs(docs map[string]*tomledit.DocumentNode, backups map[string][]byte) {
	for key, data := range backups {
		restored, _ := tomledit.Parse(data)
		docs[key] = restored
	}
}
