package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/smm-h/migrable/config"
	"github.com/smm-h/migrable/engine"
)

func runMerge(kwargs map[string]interface{}) int {
	configDir := kwargs["config_dir"].(string)
	quiet := kwargs["quiet"].(bool)
	version := kwargs["version"].(string)

	// Validate semver.
	if _, err := semver.StrictNewVersion(version); err != nil {
		fmt.Fprintf(os.Stderr, "invalid version %q: %v\n", version, err)
		return ExitValidationError
	}

	cfg, err := config.Load(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return ExitGeneralError
	}

	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	outPath, err := engine.Merge(migrationsDir, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return ExitGeneralError
	}

	if outPath == "" {
		if !quiet {
			fmt.Fprintln(os.Stderr, "Warning: no staging files in next/, nothing to merge")
		}
		return ExitSuccess
	}

	if quiet {
		return ExitSuccess
	}

	fmt.Printf("Merged staging files into %s\n", outPath)
	return ExitSuccess
}
