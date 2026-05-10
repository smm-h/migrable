package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

const migrableConfigTemplate = `# migrable configuration
# See: https://github.com/smm-h/migrable

# Directory containing migration files (required)
migrations_dir = "migrations"

# Target files to migrate (required)
# Each key is a name used in migration op paths
[files]
# config = "$HOME/.config/myapp/config.toml"

# For multi-file projects, specify which file holds _schema_version
# version_file = "config"
`

// Init scaffolds a new migrable project in the given directory. It creates
// migrable.toml, the migrations directory, and a next/ staging directory.
func Init(dir string) error {
	configPath := filepath.Join(dir, "migrable.toml")

	if fileExists(configPath) {
		return fmt.Errorf("already initialized: %s exists", configPath)
	}

	migrationsDir := filepath.Join(dir, "migrations")
	nextDir := filepath.Join(migrationsDir, "next")

	if err := os.MkdirAll(nextDir, 0o755); err != nil {
		return fmt.Errorf("failed to create migrations/next/ directory: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(migrableConfigTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write migrable.toml: %w", err)
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
