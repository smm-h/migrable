package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type MigrationMeta struct {
	Version  *semver.Version
	FilePath string
}

func DiscoverMigrations(migrationsDir string) ([]MigrationMeta, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
	}

	var migrations []MigrationMeta
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".toml") {
			continue
		}
		base := strings.TrimSuffix(name, ".toml")
		v, err := semver.StrictNewVersion(base)
		if err != nil {
			return nil, fmt.Errorf("invalid migration filename %q: not a valid semver version: %w", name, err)
		}
		absPath, err := filepath.Abs(filepath.Join(migrationsDir, name))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path for %s: %w", name, err)
		}
		migrations = append(migrations, MigrationMeta{
			Version:  v,
			FilePath: absPath,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version.LessThan(migrations[j].Version)
	})

	return migrations, nil
}
