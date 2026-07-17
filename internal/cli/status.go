package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/smm-h/migrable/config"
	"github.com/smm-h/migrable/engine"
	"github.com/smm-h/strictcli/go/strictcli"
)

func runStatus(ctx *strictcli.Context, kwargs map[string]interface{}) strictcli.Outcome {
	configDir := kwargs["config_dir"].(string)
	quiet := kwargs["quiet"].(bool)
	verbose := kwargs["verbose"].(bool)

	cfg, err := config.Load(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	migrations, err := engine.DiscoverMigrations(migrationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	versionFile := ""
	if cfg.VersionFile != "" {
		versionFile = cfg.VersionFile
	} else {
		for k := range cfg.Files {
			versionFile = k
			break
		}
	}
	versionFilePath := cfg.Files[versionFile]
	if !filepath.IsAbs(versionFilePath) {
		versionFilePath = filepath.Join(cfg.BaseDir, versionFilePath)
	}

	currentVersion, err := engine.ReadSchemaVersion(versionFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	var pending []engine.MigrationMeta
	for _, m := range migrations {
		if m.Version.GreaterThan(currentVersion) {
			pending = append(pending, m)
		}
	}

	if quiet {
		if len(pending) > 0 {
			return strictcli.Exit(ExitGeneralError)
		}
		return strictcli.Exit(ExitSuccess)
	}

	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Printf("Pending migrations: %d\n", len(pending))

	if len(pending) > 0 {
		fmt.Println()
		for _, m := range pending {
			fmt.Printf("  - %s\n", m.Version)
		}
	}

	if verbose {
		fmt.Println()
		fmt.Printf("All migrations (%d):\n", len(migrations))
		for _, m := range migrations {
			marker := " "
			if m.Version.GreaterThan(currentVersion) {
				marker = "*"
			}
			fmt.Printf("  %s %s\n", marker, m.Version)
		}

		fmt.Println()
		fmt.Println("Files:")
		for key, path := range cfg.Files {
			fmt.Printf("  %s = %s\n", key, path)
		}

		fmt.Printf("\nVersion file: %s\n", versionFile)
		fmt.Printf("Migrations dir: %s\n", migrationsDir)
	}

	return strictcli.Exit(ExitSuccess)
}
