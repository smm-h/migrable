package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/smm-h/migrable/config"
	"github.com/smm-h/migrable/engine"
	"github.com/smm-h/strictcli/go/strictcli"
)

func runMerge(ctx *strictcli.Context, kwargs map[string]interface{}) strictcli.Outcome {
	configDir := kwargs["config_dir"].(string)
	quiet := ctx.Quiet()
	version := kwargs["version"].(string)

	// Validate semver.
	if _, err := semver.StrictNewVersion(version); err != nil {
		fmt.Fprintf(os.Stderr, "invalid version %q: %v\n", version, err)
		return strictcli.Exit(ExitValidationError)
	}

	cfg, err := config.Load(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	migrationsDir := filepath.Join(cfg.BaseDir, cfg.MigrationsDir)
	outPath, err := engine.Merge(migrationsDir, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return strictcli.Exit(ExitGeneralError)
	}

	if outPath == "" {
		if !quiet {
			fmt.Fprintln(os.Stderr, "Warning: no staging files in next/, nothing to merge")
		}
		return strictcli.Exit(ExitSuccess)
	}

	if quiet {
		return strictcli.Exit(ExitSuccess)
	}

	fmt.Printf("Merged staging files into %s\n", outPath)
	return strictcli.Exit(ExitSuccess)
}
