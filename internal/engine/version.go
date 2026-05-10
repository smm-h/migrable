package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	tomledit "github.com/smm-h/go-toml-edit"
)

func ReadSchemaVersion(filePath string) (*semver.Version, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return semver.MustParse("0.0.0"), nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	doc, err := tomledit.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	verStr, ok := doc.GetString("_schema_version")
	if !ok {
		return semver.MustParse("0.0.0"), nil
	}

	v, err := semver.StrictNewVersion(verStr)
	if err != nil {
		return nil, fmt.Errorf("invalid _schema_version %q in %s: %w", verStr, filePath, err)
	}
	return v, nil
}

func WriteSchemaVersion(filePath string, version *semver.Version) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}
		data = nil
	}

	var doc *tomledit.DocumentNode
	if data != nil {
		doc, err = tomledit.Parse(data)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", filePath, err)
		}
	} else {
		doc = &tomledit.DocumentNode{}
	}

	if err := doc.SetCreate("_schema_version", version.String()); err != nil {
		return fmt.Errorf("failed to set _schema_version: %w", err)
	}

	dir := filepath.Dir(filePath)
	tmp, err := os.CreateTemp(dir, ".migrable-version-*.toml")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(doc.Bytes()); err != nil {
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
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}

	if err := os.Rename(tmpName, filePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to atomically replace %s: %w", filePath, err)
	}
	return nil
}
